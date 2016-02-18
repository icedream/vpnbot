package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type V1NetworkConfig struct {
	Host string `json:"host"`
	Port uint16 `json:"port"`
}

type V1IRCConfig struct {
	Nick          string   `json:"nick"`
	Pass          string   `json:"pass"`
	Ident         string   `json:"ident"`
	Realname      string   `json:"realname"`
	Channels      []string `json:"channels"`
	Owner         string   `json:"owner"`
	AcceptInvites *bool    `json:"accept-invites"`
}

type V1NSConfig struct {
	User string `json:"user`
	Pass string `json:"pass`
}

type V1BlacklistConfig struct {
	Domains       []string `json:"domains"`
	EnableJoewein bool     `json:"enable-joewein"`
}

// This configuration struct represents the configuration layout from the old
// bot (that was written in Node.js).
type V1Config struct {
	Network   V1NetworkConfig   `json:"network"`
	IRC       V1IRCConfig       `json:"irc"`
	NickServ  V1NSConfig        `json:"nickserv"`
	Blacklist V1BlacklistConfig `json:"blacklist"`
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
	cfg.Server.Password = c.IRC.Pass

	return
}
