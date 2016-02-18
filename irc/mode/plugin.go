package mode

import (
	"strconv"
	"strings"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/icedream/vpnbot/irc/isupport"
)

type ModeChangeAction byte

const (
	ModeChangeAction_Added ModeChangeAction = iota
	ModeChangeAction_Removed
)

type ModeChange struct {
	Action      ModeChangeAction
	Mode        rune
	HasArgument bool
	Argument    string
}

type ModeChangeEvent struct {
	Host, Ident, Nick, Src string
	Tags                   map[string]string

	Target string
	ModeChange
}

type Plugin struct {
	bot      bot.Bot
	isupport *isupport.Plugin

	// Handlers
	intHandlers *hSet
	fgHandlers  *hSet
	bgHandlers  *hSet
}

// Creates a new plugin instance.
func New(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	plugin := &Plugin{
		bot:         b,
		isupport:    isupportPlugin,
		intHandlers: handlerSet(),
		fgHandlers:  handlerSet(),
		bgHandlers:  handlerSet(),
	}

	// Handle MODE
	b.HandleFunc("mode",
		func(conn *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return
			}

			isChannel, _, _ := isupportPlugin.IsChannel(line.Args[0])

			modes := line.Args[1]
			var action ModeChangeAction
			hasAction := false
			paramIndex := 2

			getParam := func() (retval string, ok bool) {
				if len(line.Args) <= paramIndex {
					return
				}
				retval = line.Args[paramIndex]
				ok = true
				paramIndex++
				return
			}

			knownModes, ok := isupportPlugin.Supports().ChanModes()
			if !ok {
				return // TODO use some decent defaults
			}
			for _, mode := range modes {
				switch mode {
				case '-':
					hasAction = true
					action = ModeChangeAction_Removed
				case '+':
					hasAction = true
					action = ModeChangeAction_Added
				default:
					if !hasAction {
						return // + or - must come first!
					}

					modeChange := ModeChange{
						Mode:        mode,
						Action:      action,
						HasArgument: false,
					}

					if isChannel {
						// Find the mode in ISupports
						for _, knownMode := range knownModes {
							if knownMode.Mode != mode {
								continue
							}

							switch knownMode.Type {
							case isupport.ChanModeType_List,
								isupport.ChanModeType_Setting:
								// Always has a parameter
								arg, ok := getParam()
								if !ok {
									return // invalid syntax
								}
								modeChange.Argument = arg
							case isupport.ChanModeType_Setting_ParamWhenSet:
								// Only has parameter when set
								if action == ModeChangeAction_Added {
									arg, ok := getParam()
									if !ok {
										return // invalid syntax
									}
									modeChange.Argument = arg
								}
							case isupport.ChanModeType_Setting_NoParam:
								// No parameter
							}
						}
					}

					e := &ModeChangeEvent{
						ModeChange: modeChange,

						Host:  line.Host,
						Ident: line.Ident,
						Nick:  line.Nick,
						Src:   line.Src,
						Tags:  line.Tags,

						Target: line.Target(),
					}

					// Pass event to handlers
					plugin.dispatch("*", e)
					switch action {
					case ModeChangeAction_Added:
						plugin.dispatch(string([]rune{rune('+'), mode}), e)
					case ModeChangeAction_Removed:
						plugin.dispatch(string([]rune{rune('-'), mode}), e)
					}
				}
			}
		})

	return plugin
}

// Fetches the ban list for a channel.
func (p *Plugin) Bans(target string) (banlist []Ban, err error) {
	if ok, _, _ := p.isupport.IsChannel(target); !ok {
		err = ErrNotAChannel
		return
	}

	doneChan := make(chan error)

	// NOTE - Rizon doesn't limit who can see the ban list based on my own few tests...

	defer p.bot.HandleFunc("367",
		func(c *client.Conn, line *client.Line) {
			if len(line.Args) < 5 {
				return // Invalid response
			}
			if line.Args[0] != c.Me().Nick {
				return // Not for us
			}
			if !strings.EqualFold(line.Args[1], target) {
				return // Not the channel we searched for
			}
			hostmask, src, timestampStr := line.Args[2], line.Args[3], line.Args[4]
			timestampUnix, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				return // Invalid timestamp string
			}
			banlist = append(banlist, Ban{
				Hostmask:  hostmask,
				Src:       src,
				User:      splitHostmask(src),
				Timestamp: time.Unix(timestampUnix, 0),
			})
		}).Remove()

	defer p.bot.HandleFunc("368",
		func(c *client.Conn, line *client.Line) {
			if len(line.Args) < 2 {
				return // Invalid response
			}
			if line.Args[0] != c.Me().Nick {
				return // Not for us
			}
			if !strings.EqualFold(line.Args[1], target) {
				return // Not the channel we searched for
			}

			// End of channel ban list, we're done
			doneChan <- nil
		}).Remove()

	p.bot.Conn().Mode(target, "+b")
	err = <-doneChan
	return
}

// Registers this plugin with the bot.
func Register(b bot.Bot, isupportPlugin *isupport.Plugin) *Plugin {
	return New(b, isupportPlugin)
}
