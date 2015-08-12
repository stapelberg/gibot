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
	return repo{
		Url:      url,
		IssuesRe: regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)#([1-9][0-9]*)`),
		PullsRe:  regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)!([1-9][0-9]*)`),
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
		orig := line.Args[1]
		for _, p := range repos {
			for _, issue := range p.IssuesRe.FindAllStringSubmatch(orig, -1) {
				n := issue[2]
				message := fmt.Sprintf("%s/issues/%s", p.Url, n)
				go func() {
					w.notc <- notice{
						channel: channel,
						message: message,
					}
				}()
			}
			for _, issue := range p.PullsRe.FindAllStringSubmatch(orig, -1) {
				n := issue[2]
				message := fmt.Sprintf("%s/merge_requests/%s", p.Url, n)
				go func() {
					w.notc <- notice{
						channel: channel,
						message: message,
					}
				}()
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
