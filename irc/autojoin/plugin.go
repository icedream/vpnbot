package autojoin

import (
	"time"

	"strings"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
	"github.com/icedream/vpnbot/irc/mode"
)

type Plugin struct {
	bot bot.Bot
}

// Creates a new plugin instance.
func New(b bot.Bot, modePlugin *mode.Plugin) *Plugin {
	plugin := &Plugin{
		bot: b,
	}

	// Handle INVITE
	b.HandleFunc("invite",
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return
			}
			channel := line.Args[1]
			joinChan := make(chan interface{}, 1)
			modeChan := make(chan interface{}, 1)

			go func() {
				joinHandler := conn.HandleFunc(client.JOIN,
					func(conn *client.Conn, line *client.Line) {
						// Is this us joining somewhere?
						if line.Nick != conn.Me().Nick {
							return
						}

						// JOIN message should always have a channel sent with it
						if len(line.Args) < 1 {
							return
						}

						// Is this the channel we got invited to?
						if line.Args[0] != channel {
							return
						}

						// Yup, we're done here
						joinChan <- struct{}{}
					})
				modeHandler := modePlugin.HandleFunc("*",
					func(e *mode.ModeChangeEvent) {
						if e.Action != mode.ModeChangeAction_Added {
							return
						}

						switch e.Mode {
						case 'h', 'o', 'a', 'q':
							if !strings.EqualFold(e.Target, channel) {
								return // Not wanted channel
							}
							if e.Argument != conn.Me().Nick {
								return // Not for us
							}
							modeChan <- struct{}{}
						}
					})

				select {
				case <-time.After(30 * time.Second):
					joinHandler.Remove()

					// Oops, we timed out
					logging.Warn("Timed out while waiting for us to join %v",
						channel)
					return
				case <-joinChan:
					joinHandler.Remove()
				}

				// We have joined successfully, let's send our hello message!
				b.Privmsg(channel, "Hi, I'm vpn, I automatically get rid of bad "+
					"IP-changing ban evading bots! I need half-op (+h/%) to do "+
					"this properly, thank you!")

				select {
				case <-time.After(30 * time.Minute):
					modeHandler.Remove()

					// We didn't get usable permissions in 30 minutes, leave channel
					b.Part(channel, "Got no permissions, leaving.")
					return
				case <-modeChan:
					modeHandler.Remove()
				}
			}()

			// Join and wait until joined
			b.Join(channel)
		})

	return plugin
}

// Registers this plugin with the bot.
func Register(b bot.Bot, modePlugin *mode.Plugin) *Plugin {
	return New(b, modePlugin)
}
