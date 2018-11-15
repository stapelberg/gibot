// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"mvdan.cc/gibot/site"
	"mvdan.cc/gibot/site/gitlab"

	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"mvdan.cc/xurls/v2"
)

const listenAddr = ":9990"

var (
	configPath = flag.String("c", "gibot.json", "path to json config file")

	repos          map[string]*gitlab.Repo // by url
	prejoinedChans = make(map[string]bool) // by channel name, e.g. #i3
	allRe          *regexp.Regexp

	pathRegex = regexp.MustCompile("[a-zA-Z0-9]+/[a-zA-Z0-9]+")

	config struct {
		Nick         string
		Server       string
		User         string
		Pass         string
		TLS          bool
		Chans        []string
		Feeds        []string
		Repos        []site.Repo
		GithubSecret string
	}

	throttle throttler
)

func main() {
	flag.Parse()
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	log.Printf("nick   = %s", config.Nick)
	log.Printf("server = %s", config.Server)
	log.Printf("tls    = %t", config.TLS)
	log.Printf("chans  = %s", strings.Join(config.Chans, ", "))
	log.Printf("feeds  = %s", strings.Join(config.Feeds, ", "))

	repos = make(map[string]*gitlab.Repo, len(config.Repos))
	for i := range config.Repos {
		r := &config.Repos[i]
		url := r.Prefix + "/" + r.Path
		r.Aliases = append(r.Aliases, r.Name)
		if _, e := repos[url]; e {
			log.Fatalf("Duplicate repo url found: %s", url)
		}
		repos[url] = gitlab.NewRepo(r)
	}
	allRe = joinRegexes(repos)

	ircConfig := ircx.Config{
		User:       config.User,
		Password:   config.Pass,
		MaxRetries: 100,
	}
	if config.TLS {
		ircConfig.TLSConfig = &tls.Config{}
	}
	bot := ircx.New(config.Server, config.Nick, ircConfig)
	bot.SetLogger(bot.Logger())

	log.Printf("Connecting...")
	if err := bot.Connect(); err != nil {
		log.Fatalf("Unable to dial IRC Server: %v", err)
	}
	registerHandlers(bot)
	throttle = throttler{
		bot:   bot,
		sendc: make(chan *irc.Message),
	}
	go throttle.Loop()
	go bot.HandleLoop()
	http.HandleFunc("/gibot", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/gibot/gitlab", gitlabHandler)
	http.HandleFunc("/gibot/discourse", discourseHandler)
	http.Handle("/gibot/github", githubHandler(config.GithubSecret))
	log.Printf("Receiving webhooks on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func loadConfig(p string) error {
	configFile, err := os.Open(p)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return err
	}
	knownAliases := make(map[string]struct{})
	if config.Nick == "" {
		return fmt.Errorf("no nick specified")
	}
	if config.Server == "" {
		return fmt.Errorf("no server specified")
	}
	if len(config.Chans) < 1 && len(config.Feeds) < 1 {
		return fmt.Errorf("no channels specified (need either chans or feeds)")
	}
	for _, c := range config.Chans {
		prejoinedChans[c] = true
	}
	for _, r := range config.Repos {
		if r.Name == "" {
			return fmt.Errorf("repo without name")
		}
		if r.Prefix == "" {
			return fmt.Errorf("repo without prefix")
		}
		if r.Path == "" {
			return fmt.Errorf("repo without path")
		}
		if !pathRegex.MatchString(r.Path) {
			return fmt.Errorf("incorrect repo path - should be like foo/bar")
		}
		aliases := append(r.Aliases, r.Name)
		for _, a := range aliases {
			if _, e := knownAliases[a]; e {
				return fmt.Errorf("alias '%s' is not unique", a)
			}
			knownAliases[a] = struct{}{}
		}
	}
	return nil
}

func joinRegexes(repos map[string]*gitlab.Repo) *regexp.Regexp {
	all := []string{xurls.Strict().String()}
	for _, r := range repos {
		all = append(all, r.IssueRe.String())
		all = append(all, r.PullRe.String())
		all = append(all, r.CommitRe.String())
	}
	allRe := regexp.MustCompile(`(` + strings.Join(all, `|`) + `)`)
	allRe.Longest()
	return allRe
}

func registerHandlers(bot *ircx.Bot) {
	bot.HandleFunc(irc.RPL_WELCOME, onWelcome)
	bot.HandleFunc(irc.PING, onPing)
	bot.HandleFunc(irc.PRIVMSG, onPrivmsg)
}
