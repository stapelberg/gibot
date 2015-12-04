// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"log"
	"time"

	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
)

const timeBetweenMessages = 2 * time.Second

type throttler struct {
	sender ircx.Sender
	sendc  chan *irc.Message
}

func (t *throttler) Loop() {
	for {
		m := <-t.sendc
		for {
			err := t.sender.Send(m)
			if err != nil {
				time.Sleep(timeBetweenMessages * 2)
				log.Printf("Error sending message: %v", err)
			} else {
				time.Sleep(timeBetweenMessages)
				break
			}
		}
	}
}

func (t *throttler) Send(m *irc.Message) {
	t.sendc <- m
}
