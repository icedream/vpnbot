package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type V1NetworkConfig struct {
	Host string `host`
	Port uint16 `port`
}

type V1IRCConfig struct {
	Nick          string   `nick`
	Pass          string   `pass`
	Ident         string   `ident`
	Realname      string   `realname`
	Channels      []string `channels`
	Owner         string   `owner`
	AcceptInvites *bool    `accept-invites`
}

type V1NSConfig struct {
	User string `user`
	Pass string `pass`
}

type V1BlacklistConfig struct {
	Domains       []string `domains`
	EnableJoewein bool     `enable-joewein`
}

// This configuration struct represents the configuration layout from the old
// bot (that was written in Node.js).
type V1Config struct {
	Network   V1NetworkConfig   `network`
	IRC       V1IRCConfig       `irc`
	NickServ  V1NSConfig        `nickserv`
	Blacklist V1BlacklistConfig `blacklist`
}

func LoadV1Config(configPath string) (c V1Config, err error) {
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		return
	}
	err = json.Unmarshal(contents, &c)
	return
}

func (c V1Config) Migrate() (cfg Config) {
	// Admins
	if c.IRC.Owner != "" {
		cfg.Admins = []string{c.IRC.Owner}
	}

	// AutoJoinOnInvite
	switch {
	case c.IRC.AcceptInvites == nil:
		cfg.AutoJoinOnInvite = InviteBehaviorOnce
	case *c.IRC.AcceptInvites:
		cfg.AutoJoinOnInvite = InviteBehaviorAlways
	case !*c.IRC.AcceptInvites:
		cfg.AutoJoinOnInvite = InviteBehaviorIgnore
	}

	// TODO - Blacklist

	// Channels
	cfg.Channels = c.IRC.Channels

	// Ident
	cfg.Ident = c.IRC.Ident

	// Name
	cfg.Name = c.IRC.Realname

	// Nick
	cfg.Nick = c.IRC.Nick

	// NickServ
	cfg.NickServ.Username = c.NickServ.User
	cfg.NickServ.Password = c.NickServ.Pass

	// Server
	cfg.Server.Address = fmt.Sprintf("%v:%v", c.Network.Host, c.Network.Port)

	return
}
