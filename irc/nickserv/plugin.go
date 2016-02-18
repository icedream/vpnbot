package nickserv

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"

	"github.com/icedream/vpnbot/irc/mode"
)

type Plugin struct {
	bot  bot.Bot
	mode *mode.Plugin

	Password string
	Username string
	Channels []string
}

// Creates a new plugin instance.
func New(b bot.Bot, modePlugin *mode.Plugin) *Plugin {
	plugin := &Plugin{
		bot:      b,
		mode:     modePlugin,
		Username: "",
		Password: "",
		Channels: []string{},
	}

	// Handle PRIVMSG
	b.HandleFunc(client.CONNECTED,
		func(conn *client.Conn, line *client.Line) {
			// Try to log in via NickServ
			if plugin.Password != "" {
				if plugin.Username != "" {
					b.Privmsg("NickServ", fmt.Sprintf("IDENTIFY %v %v",
						plugin.Username, plugin.Password))
				} else {
					b.Privmsg("NickServ", fmt.Sprintf("IDENTIFY %v",
						plugin.Password))
				}
			}
		})

	// TODO - Handle invalid password
	// TODO - Handle nickname not registered

	/*b.HandleFunc("privmsg",
	func(conn *client.Conn, line *client.Line) {
		if len(line.Args) < 2 {
			return
		}

		if line.Nick != "NickServ" {
			return
		}

	})*/

	modePlugin.HandleFunc("+r",
		func(e *mode.ModeChangeEvent) {
			// Is this mode change for us?
			if e.Nick != b.Me().Nick {
				return
			}

			// We have logged in
			// TODO - Allow for channels to have a password
			b.Conn().Raw(client.JOIN + " " + strings.Join(plugin.Channels, ","))
		})

	return plugin
}

// Fetches the authentication status for the given nicknames.
func (p *Plugin) Status(nicks ...string) (responses map[string]NickServStatusResponse) {
	if len(nicks) > 16 {
		panic("Too many nicks")
	}

	responses = map[string]NickServStatusResponse{}
	for _, nick := range nicks {
		responses[nick] = NickServStatusResponse{
			Nick:  nick,
			Error: ErrTimedOut,
		}
	}

	done := []string{}
	doneChan := make(chan interface{})

	defer p.bot.HandleFunc("notice",
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return
			}
			if line.Args[0] != conn.Me().Nick {
				logging.Debug("nickserv.Status(): Not for me")
				return
			}
			if line.Nick != "NickServ" {
				logging.Debug("nickserv.Status(): Not from NickServ")
				return
			}
			words := strings.Split(line.Args[1], " ")
			if strings.ToUpper(words[0]) != "STATUS" {
				logging.Debug("nickserv.Status(): Not a status")
				return
			}
			if _, ok := responses[words[1]]; !ok {
				logging.Debug("nickserv.Status(): Not a nick I searched for")
				return
			}
			level, err := strconv.ParseUint(words[2], 10, 8)
			if err != nil {
				logging.Debug("Failed to parse level integer: %v", err)
				responses[words[1]] = NickServStatusResponse{
					Nick:  words[1],
					Error: err,
				}
				return
			}
			logging.Debug("nickserv.Status(): Got response for %v", words[1])
			responses[words[1]] = NickServStatusResponse{
				Nick:  words[1],
				Level: NickServStatusLevel(byte(level)),
				Error: nil,
			}
			done = append(done, words[1])
			if len(done) == len(nicks) {
				close(doneChan)
			}
		}).Remove()

	p.bot.Privmsg("NickServ", fmt.Sprintf("STATUS %v", strings.Join(nicks, " ")))
	select {
	case _, _ = <-doneChan:
		logging.Debug("Done waiting for NickServ to return status codes")
		break
	case <-time.After(30 * time.Second):
		logging.Warn("Timed out waiting for NickServ to return status codes")
	}

	return responses
}

// Registers this plugin with the bot.
func Register(b bot.Bot, modePlugin *mode.Plugin) *Plugin {
	return New(b, modePlugin)
}
