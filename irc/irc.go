// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package irc

import (
	"strings"
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

	server = "irc.freenode.net:7000"
	ssl    = true
)

type Event struct {
	Cmd  string
	Args []string
}

func EventFromLine(line *client.Line) Event {
	return Event{
		Cmd:  strings.ToUpper(line.Cmd),
		Args: line.Args,
	}
}

type Notice struct {
	Channel string
	Message string
}

type Client struct {
	conn *client.Conn

	chans map[string]struct{}

	In  chan Event
	Out chan Notice
}

func toSet(list []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, s := range list {
		m[s] = struct{}{}
	}
	return m
}

func Connect(nick string, chans []string) (*Client, error) {
	c := &Client{
		chans: toSet(chans),
		In:    make(chan Event),
		Out:   make(chan Notice),
	}

	c.conn = client.Client(&client.Config{
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

	c.conn.HandleFunc(client.CONNECTED, func(conn *client.Conn, line *client.Line) {
		c.In <- EventFromLine(line)
		for channel := range c.chans {
			conn.Join(channel)
		}
	})
	c.conn.HandleFunc(client.DISCONNECTED, func(_ *client.Conn, line *client.Line) {
		c.In <- EventFromLine(line)
	})
	c.conn.HandleFunc(client.PRIVMSG, func(_ *client.Conn, line *client.Line) {
		channel := line.Args[0]
		if _, e := c.chans[channel]; !e {
			return
		}
		c.In <- EventFromLine(line)
	})
	if err := c.conn.Connect(); err != nil {
		return nil, err
	}
	go c.Work()
	return c, nil
}

func (c *Client) Work() {
	for {
		not := <-c.Out
		c.conn.Notice(not.Channel, not.Message)
		time.Sleep(wait)
	}
}

func (c *Client) Quit() {
	c.conn.Quit()
}
