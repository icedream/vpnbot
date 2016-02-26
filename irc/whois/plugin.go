package whois

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/icedream/vpnbot/irc/isupport"
)

const (
	// ":<prefix> 301 <menick> <nick> :<away message>"
	ircReplyAway = "301"
	// ":<prefix> 311 <menick> <nick> <user> <host> * :<real name>"
	ircReplyWhoIsUser = "311"
	// ":<prefix> 312 <menick> <nick> <server> :<server info>"
	ircReplyWhoIsServer = "312"
	// ":<prefix> 313 <menick> <nick> :is an IRC operator"
	ircReplyWhoIsOperator = "313"
	// ":<prefix> 317 <menick> <nick> <integer> :seconds idle"
	ircReplyWhoIsIdle = "317"
	// ":<prefix> 318 <menick> <nick> :End of WHOIS list"
	ircReplyEndOfWhoIs = "318"
	// ":<prefix> 319 <menick> <nick> :*( ( "@" / "+" ) <channel> " " )"
	ircReplyWhoIsChannels = "319"
	// ":<prefix> 401 <menick> <nickname> :No such nick/channel"
	ircErrNoSuchNick = "401"
	// ":<prefix> 431 <menick> <nickname> :No nickname given"
	ircErrNoNickGiven = "431"
	// ":<prefix> 263 <menick> :Server load is temporarily too heavy. Please wait a while and try again."
	ircErrUnderHeavyLoad = "263"
)

// Some known channel modes.
var stdChannelModes = []rune("+%@&~")

type Plugin struct {
	bot            bot.Bot
	isupportPlugin *isupport.Plugin
}

func New(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	if isupportPlugin == nil {
		panic("isupportPlugin must not be nil")
	}
	return &Plugin{
		bot:            b,
		isupportPlugin: isupportPlugin,
	}
}

func (p *Plugin) WhoIs(nick string) (resp *WhoIsResponse, err error) {
	newResp := new(WhoIsResponse)
	whoisCompletionChan := make(chan interface{})

	availablePrefixes, _ := p.isupportPlugin.Supports().Prefix()

	defer p.bot.HandleFunc(ircReplyAway,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 3 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			newResp.AwayMessage = line.Args[2]
			newResp.IsAway = true
		}).Remove()
	defer p.bot.HandleFunc(ircReplyWhoIsChannels,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 3 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			if newResp.Channels == nil {
				newResp.Channels = map[string]WhoIsChannelMode{}
			}
			for _, prefixedChannel := range strings.Split(line.Args[2], " ") {
				prefixes, channel := isupport.SplitIrcPrefix(prefixedChannel, availablePrefixes.Symbols)
				var prefix WhoIsChannelMode = WhoIsChannelModeNone
				if len(prefixes) > 0 {
					prefix = WhoIsChannelMode(prefixes[0])
				}
				newResp.Channels[channel] = prefix
			}
		}).Remove()
	defer p.bot.HandleFunc(ircReplyWhoIsIdle,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 3 {
				return
			}
			if line.Args[1] != nick {
				return
			}

			// Idle duration
			v, err := strconv.ParseInt(line.Args[2], 10, 64)
			if err != nil {
				return
			}
			newResp.IdleDuration = time.Duration(v) * time.Second

			if len(line.Args) > 3 {
				// Sign on time
				v, err := strconv.ParseInt(line.Args[3], 10, 64)
				if err != nil {
					return
				}
				newResp.SignOnTime = time.Unix(v, 0)
			}
		}).Remove()
	defer p.bot.HandleFunc(ircReplyWhoIsOperator,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			newResp.IsOperator = true
		}).Remove()
	defer p.bot.HandleFunc(ircReplyWhoIsServer,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 4 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			newResp.Server = WhoIsServerInfo{
				Host: line.Args[2],
				Info: line.Args[3],
			}
		}).Remove()
	defer p.bot.HandleFunc(ircReplyWhoIsUser,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 6 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			newResp.Nick = line.Args[1]
			newResp.Ident = line.Args[2]
			newResp.Host = line.Args[3]
			// line.Args[4] == "*"
			newResp.Realname = line.Args[5]
		}).Remove()
	defer p.bot.HandleFunc(ircReplyEndOfWhoIs,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return
			}
			if line.Args[1] != nick {
				return
			}
			close(whoisCompletionChan)
		}).Remove()
	defer p.bot.HandleFunc(ircErrNoNickGiven,
		func(conn *client.Conn, line *client.Line) {
			if line.Nick != nick {
				return
			}
			err = ErrInvalidNick
			close(whoisCompletionChan)
		}).Remove()
	defer p.bot.HandleFunc(ircErrNoSuchNick,
		func(conn *client.Conn, line *client.Line) {
			if line.Nick != nick {
				return
			}
			err = ErrNoSuchNick
			close(whoisCompletionChan)
		}).Remove()
	defer p.bot.HandleFunc(ircErrUnderHeavyLoad,
		func(conn *client.Conn, line *client.Line) {
			err = ErrServerUnderHeavyLoad
			close(whoisCompletionChan)
		})

	p.bot.Conn().Whois(nick)
	select {
	case <-time.After(30 * time.Second):
		err = errors.New("Request timed out")
	case _, _ = <-whoisCompletionChan:
	}

	if err == nil {
		resp = newResp
	}

	return
}

func Register(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	return New(b, isupportPlugin)
}
