// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/mvdan/gibot/irc"
	"github.com/mvdan/gibot/site/gitlab"
)

var config struct {
	Nick  string   `json:"nick"`
	Chans []string `json:"chans"`
	Repos []struct {
		Name    string   `json:"name"`
		Url     string   `json:"url"`
		Aliases []string `json:"aliases"`
	} `json:"repos"`
}

func main() {
	configFile, err := os.Open("gibot.json")
	if err != nil {
		log.Fatalf("Could not open config: %v", err)
	}

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	log.Printf("nick  = %s", config.Nick)
	log.Printf("chans = %s", strings.Join(config.Chans, ", "))

	var repos []gitlab.Repo
	aliases := make(map[string]struct{})
	for _, r := range config.Repos {
		repos = append(repos, gitlab.NewRepo(r.Name,
			r.Url, r.Aliases...))
		for _, a := range r.Aliases {
			if _, e := aliases[a]; e {
				log.Fatalf("Alias '%s' is not unique!", a)
			}
			aliases[a] = struct{}{}
		}
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
