// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/mvdan/gibot/irc"
	"github.com/mvdan/gibot/site/gitlab"
)

const (
	nick = "[gibot]"
)

func main() {
	chans := []string{
		"#fdroid",
		"#fdroid-dev",
	}
	c, err := irc.Connect(nick, chans)
	if err != nil {
		log.Fatalf("Could not connect to IRC: %v", err)
	}

	repos := []gitlab.Repo{
		gitlab.NewRepo("https://gitlab.com/fdroid/fdroidclient",
			"", "c", "client", "fdroidclient"),
		gitlab.NewRepo("https://gitlab.com/fdroid/fdroidserver",
			"s", "server", "fdroidserver"),
		gitlab.NewRepo("https://gitlab.com/fdroid/fdroiddata",
			"d", "data", "fdroiddata"),
	}

	l := &listener{
		repos:  repos,
		allRe:  joinRegexes(repos),
		client: c,
	}
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
		case "PRIVMSG":
			go l.onPrivmsg(ev)
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
				message := r.IssueURL(n)
				go func() {
					l.client.Out <- irc.Notice{
						Channel: channel,
						Message: message,
					}
				}()
			}
			if s := r.PullsRe.FindStringSubmatch(m); s != nil && s[0] == m {
				n := s[2]
				message := r.PullURL(n)
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
