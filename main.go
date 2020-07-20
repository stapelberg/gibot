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

	"github.com/nickvanw/ircx/v2"
	"gopkg.in/sorcix/irc.v2"
	"mvdan.cc/xurls/v2"
)

func defaultConfigPath() string {
	if os.Getenv("GOKRAZY_FIRST_START") == "1" {
		// Make it easy for gokrazy users to run this main without specifying
		// any flags (which is cumbersome in gokrazy).
		return "/perm/gibot.json"
	}
	return "gibot.json"
}

var (
	configPath = flag.String("c", defaultConfigPath(), "path to json config file")
	listenAddr = flag.String("listen", ":9990", "[host]:port to listen on")

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
		log.Printf("Could not load config: %v", err)
		// gokrazy.org treats exit code 125 as a signal to not restart the
		// process anymore. This way, two use-cases are covered:
		//
		// 1. Users who need a single instance of gibot can create
		//    /perm/gibot.json and add mvdan.cc/gibot to their
		//    gokr-packer command line.
		//
		// 2. Users who need multiple instances of gibot can create their own
		//    wrappers which run /user/gibot and add mvdan.cc/gibot to
		//    their gokr-packer command line. The auto-generated wrapper will
		//    start gibot only once (as opposed to crashlooping forever).
		os.Exit(125)
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
		repo, err := gitlab.NewRepo(r)
		if err != nil {
			log.Fatalf("Unable to create gitlab client: %v", err)
		}
		repos[url] = repo
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
	log.Printf("Receiving webhooks on %s", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
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
