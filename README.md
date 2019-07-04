# gibot

[![Travis](https://travis-ci.org/mvdan/gibot.svg?branch=master)](https://travis-ci.org/mvdan/gibot)

Simple IRC bot that helps software projects. Requires Go 1.12 or later.

### Setup

	go get -u mvdan.cc/gibot

It will read a config file from `config.json` by default. See the
[example config](confs/fdroid.json) that the F-Droid project uses, for
instance.

`user`, `nick` and `server` are self-explanatory - it configures how the
bot will connect to IRC.

`chans` are the channels that the bot will join and listen for messages
on.

`feeds` are the channels that the bot will post activity feed items on,
such as when a new issue is created. `feeds` channels which are not
listed in `chans` will be joined temporarily for posting.

Finally, `repos` are the repos that the bot will use. The `token` has to
be populated with your Gitlab API token.

For each of the repos, its feed will be posted to the channels in
`feeds`. On top of that, the bot will listen for messages mentioning
issues, pull requests and commits and it will link them.

For example, in the example config, the gitlab setup for F-Droid will
result in the following behaviour:

* `#10`, `c#10`, `client#10` and `fdroidclient#10` will make the bot
  link the issue number 10 on fdroidclient
* `s!20`, `server!20` and `fdroidserver!20` will make the bot link the
  merge request number 20 on fdroidclient
* `abcdef12`, `abcdef12345678` and any valid commit hash at least 8
  characters in length will make the bot link the commit (it will try
  all the repos and return the first result)

### Systemd

Below is a sample systemd service file to run it as a daemon. The automatic
restarts will take care of failures to reconnect and crashes, if there are any.

```
[Unit]
Description=Gibot

[Service]
ExecStart=/opt/gibot/gibot -c /opt/gibot/config.json
Restart=on-failure
RestartSec=20s

[Install]
WantedBy=default.target
```
