package isupport

import (
	"strings"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
)

const (
	// ":<prefix> 005 <menick> *( <key> ( "=" <value> ) " " )"
	ircISupport = "005"
)

// Some known channel modes.
var stdChannelModes = []rune("+%@&~")

type Plugin struct {
	bot      bot.Bot
	supports *Support
}

// Creates a new plugin instance.
func New(b bot.Bot) *Plugin {
	plugin := &Plugin{
		bot: b,
		supports: &Support{
			Data: map[string]string{},
		},
	}

	// Handle 005/ISUPPORT
	b.HandleFunc(ircISupport,
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 ||
				!strings.HasSuffix(
					line.Args[len(line.Args)-1],
					"are supported by this server") {
				return
			}

			// First arg is our name, last arg is "are supported by this server"
			for _, support := range line.Args[1 : len(line.Args)-1] {
				name := ""
				value := ""
				if strings.Contains(support, "=") {
					spl := strings.SplitN(support, "=", 2)
					name, value = strings.ToUpper(spl[0]), spl[1]
				} else {
					name = strings.ToUpper(support)
				}

				// Set in supports table
				plugin.supports.Data[name] = value
			}

		})

	return plugin
}

// Returns the table of things the server reported to the bot as supported.
func (plugin *Plugin) Supports() *Support {
	return plugin.supports
}

// Returns whether the target is a channel.
func (p *Plugin) IsChannel(target string) (ok bool, prefixes []rune, name string) {
	chantypes, _ := p.supports.ChanTypes()
	prefixes, name = SplitIrcPrefix(target, chantypes)
	ok = len(prefixes) > 0
	return
}

// Registers this plugin with the bot.
func Register(b bot.Bot) *Plugin {
	return New(b)
}
