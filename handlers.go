// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
)

func onWelcome(s ircx.Sender, m *irc.Message) {
	log.Printf("Connected.")
	s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  []string{strings.Join(config.Chans, ",")},
	})
}

func onPing(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}

func onPrivmsg(s ircx.Sender, m *irc.Message) {
	channel := m.Params[0]
	line := m.Trailing
	for _, m := range allRe.FindAllString(line, -1) {
		for _, r := range repos {
			if ss := r.IssueRe.FindStringSubmatch(m); ss != nil && ss[0] == m {
				body, err := r.IssueInfo(ss[2])
				if err != nil {
					log.Printf("#%s: %v", ss[2], err)
					continue
				}
				go sendNotice(s, channel, r.Name, body)
			}
			if ss := r.PullRe.FindStringSubmatch(m); ss != nil && ss[0] == m {
				body, err := r.PullInfo(ss[2])
				if err != nil {
					log.Printf("!%s: %v", ss[2], err)
					continue
				}
				go sendNotice(s, channel, r.Name, body)
			}
			if ss := r.CommitRe.FindString(m); ss == m {
				body, err := r.CommitInfo(ss)
				if err != nil {
					log.Printf("%s: %v", ss, err)
					continue
				}
				go sendNotice(s, channel, r.Name, body)
			}
		}
	}
}

func sendNotice(s ircx.Sender, channel, categ, body string) error {
	message := fmt.Sprintf("[%s] %s", categ, body)
	return s.Send(&irc.Message{
		Command:  irc.NOTICE,
		Params:   []string{channel},
		Trailing: message,
	})
}
