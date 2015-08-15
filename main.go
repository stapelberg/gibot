// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/mvdan/gibot/irc"
	"github.com/mvdan/gibot/site"
	"github.com/mvdan/gibot/site/gitlab"
)

var (
	configPath = flag.String("c", "gibot.json", "path to json config file")
)

var config struct {
	Nick  string
	Chans []string
	Repos []site.Repo
}

func main() {
	flag.Parse()
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("Could not load config: %v", err)
	}
	log.Printf("nick  = %s", config.Nick)
	log.Printf("chans = %s", strings.Join(config.Chans, ", "))

	var repos []gitlab.Repo
	for i := range config.Repos {
		r := &config.Repos[i]
		r.Aliases = append(r.Aliases, r.Name)
		repos = append(repos, *gitlab.NewRepo(r))
	}
	l := &listener{
		repos: repos,
		allRe: joinRegexes(repos),
	}

	log.Printf("Connecting...")
	c, err := irc.Connect(config.Nick, config.Chans)
	if err != nil {
		log.Fatalf("Could not connect to IRC: %v", err)
	}

	l.client = c
	go l.listen()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-sigc
	log.Printf("Quitting.")
	c.Quit()
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
		all = append(all, r.IssuesRe.String())
		all = append(all, r.PullsRe.String())
	}
	allRe := regexp.MustCompile(`(` + strings.Join(all, `|`) + `)`)
	allRe.Longest()
	return allRe
}

type listener struct {
	repos  []gitlab.Repo
	allRe  *regexp.Regexp
	client *irc.Client
}

func (l *listener) listen() {
	for {
		ev := <-l.client.In
		switch ev.Cmd {
		case "CONNECTED":
			log.Printf("Connected.")
		case "PRIVMSG":
			go l.onPrivmsg(ev)
		case "DISCONNECTED":
			log.Fatalf("Disconnected!")
		}
	}
}

func (l *listener) onPrivmsg(ev irc.Event) {
	channel := ev.Args[0]
	line := ev.Args[1]
	for _, m := range l.allRe.FindAllString(line, -1) {
		for _, r := range l.repos {
			if s := r.IssuesRe.FindStringSubmatch(m); s != nil && s[0] == m {
				n := s[2]
				message := fmt.Sprintf("[%s] %s", r.Name, r.IssueURL(n))
				go func() {
					l.client.Out <- irc.Notice{
						Channel: channel,
						Message: message,
					}
				}()
			}
			if s := r.PullsRe.FindStringSubmatch(m); s != nil && s[0] == m {
				n := s[2]
				message := fmt.Sprintf("[%s] %s", r.Name, r.PullURL(n))
				go func() {
					l.client.Out <- irc.Notice{
						Channel: channel,
						Message: message,
					}
				}()
			}
		}
	}
}
