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

	timeout = 20 * time.Second
	ping    = 2 * time.Minute
	split   = 100

	nick    = "[" + name + "]"
	server  = "irc.freenode.net:7000"
	ssl     = true
	channel = "#fdroid-dev"
)

type project struct {
	Url   string
	Issue *regexp.Regexp
}

func newProject(url string, aliases ...string) project {
	return project{
		Url:   url,
		Issue: regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)#([1-9][0-9]*)`),
	}
}

func main() {
	projects := []project{
		newProject("https://gitlab.com/fdroid/fdroidclient",
			"", "c", "client", "fdroidclient"),
		newProject("https://gitlab.com/fdroid/fdroidserver",
			"s", "server", "fdroidserver"),
		newProject("https://gitlab.com/fdroid/fdroiddata",
			"d", "data", "fdroiddata"),
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
	c.HandleFunc(client.CONNECTED, func(conn *client.Conn, line *client.Line) {
		log.Printf("Connected.")
		conn.Join(channel)
	})
	c.HandleFunc(client.DISCONNECTED, func(conn *client.Conn, line *client.Line) {
		log.Fatalf("Disconnected!")
	})
	c.HandleFunc(client.PRIVMSG, func(conn *client.Conn, line *client.Line) {
		target := line.Args[0]
		if target != channel {
			return
		}
		message := line.Args[1]
		for _, p := range projects {
			for _, issue := range p.Issue.FindAllStringSubmatch(message, -1) {
				n := issue[2]
				notice := fmt.Sprintf("%s/issues/%s", p.Url, n)
				conn.Notice(target, notice)
			}
		}
	})

	log.Printf("Connecting...")
	if err := c.Connect(); err != nil {
		log.Fatalf("Connection error: %v", err)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigc
	log.Printf("Quitting.")
	c.Quit()
}
