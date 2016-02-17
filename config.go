package main

import (
	"errors"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type InviteBehavior string

const (
	// Only accept invite and automatically join the channel once. When this
	// value is set, the configuration will fall back to "Ignore" once a channel
	// is joined.
	InviteBehaviorOnce InviteBehavior = "once"

	// Ignore all invitations.
	InviteBehaviorIgnore = "ignore"

	// Always automatically join channels on invite.
	InviteBehaviorAlways = "always"
)

type ServerConfig struct {
	Address string "Address"
	SSL     bool   "SSL"
}

type NickServConfig struct {
	// The username to use when authenticating with NickServ.
	Username string "Username,omitempty"

	// The password to use when authenticating with NickServ.
	Password string "Password,omitempty"
}

type Config struct {
	// The main nickname the bot uses.
	Nick string "Nick"

	// The ident (username) the bot uses.
	Ident string "Ident"

	// The full exact hostmask of admins that can control the bot.
	Admins []string "Admins"

	// Determines how the bot should handle invitations.
	AutoJoinOnInvite InviteBehavior "AutoJoinOnInvite"

	// The channels to join automatically. This field will be changed by the
	// bot when it joins channels on invitation.
	Channels []string "Channels"

	// The server to connect to.
	Server ServerConfig "Server"

	// Details regarding NickServ authentication
	NickServ NickServConfig "NickServ"
}

func Load(configPath string) (c Config, err error) {
	logger.Debug("Reading from %v...", configPath)
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		return
	}

	logger.Debug("Parsing configuration...")
	c = defaultConfiguration
	err = yaml.Unmarshal(contents, &c)
	return
}

func (c Config) Save(configPath string) error {
	contents, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configPath, contents, 0750)
}

func (c Config) Validate() error {
	if c.Nick == "" {
		return errors.New("You need to set a nickname in the configuration.")
	}
	if c.Ident == "" {
		return errors.New("You need to set an ident in the configuration.")
	}
	if c.Server.Address == "" {
		return errors.New("You need to set a server address in the configuration.")
	}

	// AutoJoinOnInvite
	c.AutoJoinOnInvite = InviteBehavior(strings.ToLower(string(c.AutoJoinOnInvite)))
	switch c.AutoJoinOnInvite {
	case InviteBehaviorAlways:
	case InviteBehaviorIgnore:
	case InviteBehaviorOnce:
	case "":
		c.AutoJoinOnInvite = InviteBehaviorOnce
	default:
		return errors.New("AutoJoinOnInvite must be set to either always, ignore or once (default).")
	}

	return nil
}

// Default configuration
var defaultConfiguration = Config{
	Nick:             "vpn",
	Ident:            "vpn",
	Admins:           []string{},
	AutoJoinOnInvite: InviteBehaviorOnce,
	Channels:         []string{"#vpnbot"},
	Server: ServerConfig{
		Address: "",
		SSL:     false,
	},
	NickServ: NickServConfig{
		Username: "",
		Password: "",
	},
}
