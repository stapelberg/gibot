// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fluffle/goirc/client"
	log_glog "github.com/fluffle/goirc/logging/glog"
	"github.com/fluffle/goirc/state"
)

const (
	name    = "gibot"
	version = name + " v0.0"
	quit    = name + " exited"

	wait    = 2 * time.Second
	timeout = 20 * time.Second
	ping    = 2 * time.Minute
	split   = 100

	nick   = "[" + name + "]"
	server = "irc.freenode.net:7000"
	ssl    = true
)

type repo struct {
	Url      string
	IssuesRe *regexp.Regexp
	PullsRe  *regexp.Regexp
}

func newRepo(url string, aliases ...string) repo {
	issuesRe := regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)#([1-9][0-9]*)`)
	issuesRe.Longest()
	pullsRe := regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)!([1-9][0-9]*)`)
	pullsRe.Longest()
	return repo{
		Url:      url,
		IssuesRe: issuesRe,
		PullsRe:  pullsRe,
	}
}

type notice struct {
	channel string
	message string
}

type writer struct {
	notc chan notice
	conn *client.Conn
}

func (w *writer) work() {
	for {
		not := <-w.notc
		w.conn.Notice(not.channel, not.message)
		time.Sleep(wait)
	}
}

func main() {
	repos := []repo{
		newRepo("https://gitlab.com/fdroid/fdroidclient",
			"", "c", "client", "fdroidclient"),
		newRepo("https://gitlab.com/fdroid/fdroidserver",
			"s", "server", "fdroidserver"),
		newRepo("https://gitlab.com/fdroid/fdroiddata",
			"d", "data", "fdroiddata"),
	}

	var all []string
	for _, r := range repos {
		all = append(all, r.IssuesRe.String())
		all = append(all, r.PullsRe.String())
	}
	allRe := regexp.MustCompile(`(` + strings.Join(all, `|`) + `)`)
	allRe.Longest()

	channels := map[string]struct{}{
		"#fdroid":     struct{}{},
		"#fdroid-dev": struct{}{},
	}

	c := client.Client(&client.Config{
		Me: &state.Nick{
			Nick:  nick,
			Ident: name,
			Name:  name,
		},
		PingFreq:    ping,
		NewNick:     func(s string) string { return s + "_" },
		Recover:     (*client.Conn).LogPanic,
		SplitLen:    split,
		Timeout:     timeout,
		Server:      server,
		SSL:         ssl,
		Version:     version,
		QuitMessage: quit,
	})

	log_glog.Init()

	w := writer{
		notc: make(chan notice),
		conn: c,
	}

	c.HandleFunc(client.CONNECTED, func(conn *client.Conn, line *client.Line) {
		log.Printf("Connected.")
		for channel := range channels {
			conn.Join(channel)
		}
	})
	c.HandleFunc(client.DISCONNECTED, func(_ *client.Conn, line *client.Line) {
		log.Fatalf("Disconnected!")
	})
	c.HandleFunc(client.PRIVMSG, func(_ *client.Conn, line *client.Line) {
		channel := line.Args[0]
		if _, e := channels[channel]; !e {
			return
		}
		origmsg := line.Args[1]
		for _, m := range allRe.FindAllString(origmsg, -1) {
			for _, r := range repos {
				if s := r.IssuesRe.FindStringSubmatch(m); s != nil && s[0] == m {
					n := s[2]
					message := fmt.Sprintf("%s/issues/%s", r.Url, n)
					go func() {
						w.notc <- notice{
							channel: channel,
							message: message,
						}
					}()
				}
				if s := r.PullsRe.FindStringSubmatch(m); s != nil && s[0] == m {
					n := s[2]
					message := fmt.Sprintf("%s/merge_requests/%s", r.Url, n)
					go func() {
						w.notc <- notice{
							channel: channel,
							message: message,
						}
					}()
				}
			}
		}
	})

	log.Printf("Connecting...")
	if err := c.Connect(); err != nil {
		log.Fatalf("Connection error: %v", err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go w.work()

	<-sigc
	log.Printf("Quitting.")
	c.Quit()
}
