package bots

import (
	"fmt"
	"strings"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/icedream/vpnbot/irc/isupport"
)

type Plugin struct {
	bot      bot.Bot
	isupport *isupport.Plugin
}

// Creates a new plugin instance.
func New(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	plugin := &Plugin{
		bot:      b,
		isupport: isupportPlugin,
	}

	// ("."|"!"|"+"|"@")"bots"
	b.HandleFunc("privmsg",
		func(conn *client.Conn, line *client.Line) {
			// Check if this came from a channel named "bots" or "vpnbot"
			chanPrefixes, _ := plugin.isupport.Supports().ChanTypes()
			prefixes, channelName := isupport.SplitIrcPrefix(line.Args[0], chanPrefixes)
			if len(prefixes) <= 0 {
				return // target not a channel
			}
			if !strings.EqualFold(channelName, "bots") &&
				!strings.EqualFold(channelName, "vpnbot") {
				return
			}

			// Check if this is the "bots" command (with either prefix below)
			words := strings.Split(line.Args[1], " ")
			prefixes, firstWord := isupport.SplitIrcPrefix(words[0], []rune(".!+@"))
			if len(prefixes) <= 0 {
				return // not a command
			}
			if !strings.EqualFold(firstWord, "bots") {
				return // not the bots command
			}

			// Report in!
			plugin.bot.Privmsg(line.Args[0],
				fmt.Sprintf(
					"Reporting in! [Go] See %v12%vhttps://github.com/icedream/vpnbot%v%v",
					"\x03", "\x1F", "\x1F", "\x03"))
		})

	return plugin
}

// Registers this plugin with the bot.
func Register(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	return New(b, isupportPlugin)
}
