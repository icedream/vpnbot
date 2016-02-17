package autojoin

import (
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
)

type Plugin struct {
	bot bot.Bot
}

// Creates a new plugin instance.
func New(b bot.Bot) *Plugin {
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
			joinChan := make(chan interface{})

			go func() {
				defer conn.HandleFunc("join",
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
					}).Remove()

				select {
				case <-time.After(10 * time.Second):
					// Oops, we timed out
					logging.Warn("Timed out while waiting for us to join %v",
						channel)
					return
				case <-joinChan:
				}

				// We have joined successfully, let's send our hello message!
				b.Privmsg(channel, "Hi, I'm vpn, I automatically get rid of bad "+
					"IP-changing ban evading bots! I need half-op (+h/%) to do "+
					"this properly, thank you!")
			}()

			// Join and wait until joined
			b.Join(channel)
		})

	return plugin
}

// Registers this plugin with the bot.
func Register(b bot.Bot) *Plugin {
	return New(b)
}
