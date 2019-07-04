// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/nickvanw/ircx/v2"
	"gopkg.in/sorcix/irc.v2"
)

func onWelcome(s ircx.Sender, m *irc.Message) {
	log.Printf("Connected.")
	err := s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  []string{strings.Join(config.Chans, ",")},
	})
	if err != nil {
		log.Printf("Could not send JOIN: %v", err)
	}
}

func onPing(s ircx.Sender, m *irc.Message) {
	err := s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
	})
	if err != nil {
		log.Printf("Could not reply to PING: %v", err)
	}
}

func onPrivmsg(s ircx.Sender, m *irc.Message) {
	channel := m.Params[0]
	line := m.Trailing()
	for _, m := range allRe.FindAllString(line, -1) {
		for _, r := range repos {
			if ss := r.IssueRe.FindStringSubmatch(m); ss != nil && ss[0] == m {
				id, err := strconv.Atoi(ss[2])
				if err != nil {
					continue
				}
				body, err := r.IssueInfo(id)
				if err != nil {
					log.Printf("#%d: %v", id, err)
					continue
				}
				go sendNotice(channel, r.Name, body)
			}
			if ss := r.PullRe.FindStringSubmatch(m); ss != nil && ss[0] == m {
				id, err := strconv.Atoi(ss[2])
				if err != nil {
					continue
				}
				body, err := r.PullInfo(id)
				if err != nil {
					log.Printf("!%d: %v", id, err)
					continue
				}
				go sendNotice(channel, r.Name, body)
			}
			if ss := r.CommitRe.FindString(m); ss == m {
				body, err := r.CommitInfo(ss)
				if err != nil {
					log.Printf("%s: %v", ss, err)
					continue
				}
				go sendNotice(channel, r.Name, body)
			}
		}
	}
}

func sendNotice(channel, categ, body string) {
	message := fmt.Sprintf("[%s] %s", categ, body)
	throttle.Send(&irc.Message{
		Command:  irc.NOTICE,
		Params:   []string{channel, message},
	})
}

func sendNotices(chans []string, categ string, body ...string) {
	for _, channel := range chans {
		if !prejoinedChans[channel] {
			throttle.Send(&irc.Message{
				Command: irc.JOIN,
				Params:  []string{channel},
			})
		}
	}

	for _, channel := range chans {
		for _, body := range body {
			message := fmt.Sprintf("[%s] %s", categ, body)
			throttle.Send(&irc.Message{
				Command:  irc.NOTICE,
				Params:   []string{channel, message},
			})
		}
	}

	for _, channel := range chans {
		if !prejoinedChans[channel] {
			throttle.Send(&irc.Message{
				Command: irc.PART,
				Params:  []string{channel},
			})
		}
	}
}
