// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/mvdan/gibot/site"
	"github.com/mvdan/gibot/site/gitlab"

	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
)

var (
	configPath = flag.String("c", "gibot.json", "path to json config file")

	repos []gitlab.Repo
	allRe *regexp.Regexp

	pathRegex = regexp.MustCompile("[a-zA-Z0-9]+/[a-zA-Z0-9]+")
)

var config struct {
	Nick   string
	Server string
	Pass   string
	TLS    bool
	Chans  []string
	Repos  []site.Repo
}

func main() {
	flag.Parse()
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	log.Printf("nick   = %s", config.Nick)
	log.Printf("server = %s", config.Server)
	log.Printf("tls    = %t", config.TLS)
	log.Printf("chans  = %s", strings.Join(config.Chans, ", "))

	for i := range config.Repos {
		r := &config.Repos[i]
		r.Aliases = append(r.Aliases, r.Name)
		repos = append(repos, *gitlab.NewRepo(r))
	}
	allRe = joinRegexes(repos)

	bot := getBot()
	log.Printf("Connecting...")
	if err := bot.Connect(); err != nil {
		log.Fatalf("Unable to dial IRC Server: %v", err)
	}
	registerHandlers(bot)
	bot.CallbackLoop()
}

func getBot() *ircx.Bot {
	if !config.TLS {
		if config.Pass == "" {
			return ircx.Classic(config.Server, config.Nick)
		}
		return ircx.WithLogin(config.Server, config.Nick, config.Nick, config.Pass)
	}
	if config.Pass == "" {
		return ircx.WithTLS(config.Server, config.Nick, nil)
	}
	return ircx.WithLoginTLS(config.Server, config.Nick, config.Nick, config.Pass, nil)
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
	if len(config.Chans) < 1 {
		return fmt.Errorf("no channels specified")
	}
	for _, r := range config.Repos {
		if r.Name == "" {
			return fmt.Errorf("repo without name")
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

func joinRegexes(repos []gitlab.Repo) *regexp.Regexp {
	var all []string
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
	bot.AddCallback(irc.RPL_WELCOME, ircx.Callback{Handler: ircx.HandlerFunc(onWelcome)})
	bot.AddCallback(irc.PING, ircx.Callback{Handler: ircx.HandlerFunc(onPing)})
	bot.AddCallback(irc.PRIVMSG, ircx.Callback{Handler: ircx.HandlerFunc(onPrivmsg)})
}
