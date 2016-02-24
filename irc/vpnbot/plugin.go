package vpnbot

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/dustin/go-humanize"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"

	"github.com/icedream/vpnbot/irc/isupport"
	"github.com/icedream/vpnbot/irc/nickserv"
	"github.com/icedream/vpnbot/irc/tempban"
	"github.com/icedream/vpnbot/irc/util"
	"github.com/icedream/vpnbot/irc/whois"
)

var nonWordCharsRegex = regexp.MustCompile("\\W")
var maskedAddrRegex = regexp.MustCompile("Rizon\\-[A-F0-9]{8}\\..+\\.[A-Za-z]|[0-9A-F]{8}\\.[0-9A-F]{8}\\.[0-9A-F]{8}\\.IP")

type Plugin struct {
	bot            bot.Bot
	whois          *whois.Plugin
	isupport       *isupport.Plugin
	tempban        *tempban.Plugin
	nickserv       *nickserv.Plugin
	lastCheckNicks map[string]time.Time
	Admins         []string
}

func New(b bot.Bot, whoisPlugin *whois.Plugin, isupportPlugin *isupport.Plugin,
	tempbanPlugin *tempban.Plugin, nickservPlugin *nickserv.Plugin) *Plugin {
	if whoisPlugin == nil {
		panic("whoisPlugin must not be nil")
	}
	if isupportPlugin == nil {
		panic("isupportPlugin must not be nil")
	}
	if tempbanPlugin == nil {
		panic("tempbanPlugin must not be nil")
	}
	if nickservPlugin == nil {
		panic("nickservPlugin must not be nil")
	}

	plugin := &Plugin{
		bot:            b,
		whois:          whoisPlugin,
		isupport:       isupportPlugin,
		tempban:        tempbanPlugin,
		nickserv:       nickservPlugin,
		lastCheckNicks: map[string]time.Time{},
		Admins:         []string{},
	}

	b.Conn().HandleFunc("join", plugin.OnJoin)

	b.Commands().Add("listbans", bot.Command{
		Hidden: true,
		Pub:    true,
		Help:   "(only admins) Lists bans the bot set in a channel.",
		Handler: func(e *bot.Event) {
			if !plugin.isAdmin(e.Line.Src) {
				return
			}

			channel := e.Args
			if len(channel) < 1 {
				b.Privmsg(e.Target, "Need a channel to query.")
				return
			}

			bans := tempbanPlugin.Bans(channel)
			if len(bans) == 0 {
				b.Privmsg(e.Target, fmt.Sprintf("No bans set for \x02%v\x02.",
					channel))
			} else {
				for i, ban := range bans {
					b.Privmsg(e.Target,
						fmt.Sprintf("%4v. \x02%-41v\x02 (\x02%v\x02, expires \x02%v\x02)",
							i+1, ban.Hostmask, ban.Reason,
							humanize.Time(ban.ExpirationTime)))
				}
			}
		},
	})

	b.Commands().Add("globalban", bot.Command{
		Hidden: true,
		Pub:    true,
		Help:   "(only admins) Globally bans a specific nickname or hostmask with given duration and reason.",
		Handler: func(e *bot.Event) {
			if !plugin.isAdmin(e.Line.Src) {
				return
			}

			split := strings.SplitN(e.Args, " ", 3)
			if len(split) < 3 {
				b.Privmsg(e.Target, "Need a nickname or hostmask, duration and reason to ban, in this order.")
				return
			}
			nick, durationStr, reason := split[0], split[1], split[2]
			reason = fmt.Sprintf("Manual global ban: %v", reason)
			duration, err := time.ParseDuration(durationStr)
			var banmask string
			if !strings.Contains(nick, "@") && !strings.Contains(nick, "!") {
				if err != nil {
					b.Privmsg(e.Target, fmt.Sprintf("Failed to parse duration: %v", err))
					return
				}

				// Generate the ban mask from WHOIS information
				info, err := whoisPlugin.WhoIs(nick)
				if err != nil {
					b.Privmsg(e.Target, fmt.Sprintf("Can't get information about this nick: %v", err))
					return
				}

				banmask = fmt.Sprintf("%v!%v@%v", "*", "*", info.Host)
			} else {
				banmask = nick
				nick = ""
			}

			b.Privmsg(e.Target,
				fmt.Sprintf("Banning \x02%v\x02 until \x02%v\x02 with reason \x02%v\x02.",
					banmask, humanize.Time(time.Now().Add(duration)), reason))
			plugin.banGlobal(plugin.generateBan(nick, banmask, reason, duration))
		},
	})

	b.Commands().Add("globalunban", bot.Command{
		Hidden: true,
		Pub:    true,
		Help:   "(only admins) Globally unbans a specific nickname or hostmask.",
		Handler: func(e *bot.Event) {
			if !plugin.isAdmin(e.Line.Src) {
				return
			}

			if len(e.Args) <= 0 {
				b.Privmsg(e.Target, "Need a nickname or hostmask.")
				return
			}

			nick := e.Args
			var banmask string
			if !strings.Contains(nick, "@") && !strings.Contains(nick, "!") {

				// Generate the ban mask from WHOIS information
				info, err := whoisPlugin.WhoIs(nick)
				if err != nil {
					b.Privmsg(e.Target, fmt.Sprintf("Can't get information about this nick: %v", err))
					return
				}

				banmask = fmt.Sprintf("%v!%v@%v", "*", "*", info.Host)
			} else {
				banmask = nick
			}

			b.Privmsg(e.Target, fmt.Sprintf("Unbanning \x02%v\x02.", banmask))
			plugin.unbanGlobal(banmask)
		},
	})

	return plugin
}

func (plugin *Plugin) isAdmin(mask string) bool {
	for _, adminmask := range plugin.Admins {
		// TODO - Test this implementation
		adminmask = regexp.QuoteMeta(adminmask)
		adminmask = strings.Replace(adminmask, "\\*", ".*", -1)
		adminmask = strings.Replace(adminmask, "\\?", ".?", -1)
		if matched, err := regexp.MatchString(adminmask, mask); matched {
			return true
		} else if err != nil {
			logging.Error("vpnbot.Plugin: isAdmin regular expression failed: %v",
				err)
			break
		}
	}

	return false
}

func (plugin *Plugin) OnJoin(conn *client.Conn, line *client.Line) {
	logging.Info("vpnbot.Plugin: %v joined %v", line.Src, line.Target())

	if lastCheck, ok := plugin.lastCheckNicks[line.Nick]; ok && time.Now().Sub(lastCheck) < 15*time.Minute {
		// There is a check in progress or another one has been done earlier
		logging.Debug("vpnbot.Plugin: Not checking %v, last check was %v",
			line.Nick, humanize.Time(plugin.lastCheckNicks[line.Nick]))
		return
	}
	logging.Debug("vpnbot.Plugin: Checking %v...", line.Nick)
	plugin.lastCheckNicks[line.Nick] = time.Now()

	// Is this me?
	if line.Nick == conn.Me().Nick {
		logging.Debug("vpnbot.Plugin: %v is actually me, skipping.", line.Nick)
		return
	}

	// Nickname == Ident? (9 chars cut)
	if !strings.HasPrefix(nonWordCharsRegex.ReplaceAllString(line.Nick, ""),
		strings.TrimLeft(line.Ident, "~")) {
		logging.Debug("vpnbot.Plugin: %v's nick doesn't match the ident, skipping.", line.Nick)
		return
	}

	// Hostname is masked RDNS vhost/IP?
	// TODO - Use regex to avoid banning due to similar vhosts
	if !maskedAddrRegex.MatchString(line.Host) {
		// Detected custom vHost
		logging.Debug("vpnbot.Plugin: %v has a custom vhost, skipping.", line.Nick)
		return
	}

	go func() {
		botNick := line.Nick

		nobotActivityChan := make(chan string)
		defer plugin.bot.HandleFunc(client.PRIVMSG,
			func(conn *client.Conn, line *client.Line) {
				if line.Nick != botNick {
					return
				}

				nobotActivityChan <- "User sent a message"
			}).Remove()
		defer plugin.bot.HandleFunc(client.NOTICE,
			func(conn *client.Conn, line *client.Line) {
				if line.Nick != botNick {
					return
				}

				nobotActivityChan <- "User sent a notice"
			}).Remove()
		defer plugin.bot.HandleFunc(client.PART,
			func(conn *client.Conn, line *client.Line) {
				if line.Nick != botNick {
					return
				}

				nobotActivityChan <- "User left"
			}).Remove()
		defer plugin.bot.HandleFunc(client.QUIT,
			func(conn *client.Conn, line *client.Line) {
				if line.Nick != botNick {
					return
				}

				if len(line.Args) > 0 {
					switch line.Args[0] {
					case "Excess flood":
						// If this was an excess flood, consider it spam that should
						// be good to ban anyways
						nobotActivityChan <- "Excess flood, banning early"
						banmask := fmt.Sprintf("%v!%v@%v", "*", "*", line.Host)
						// TODO - Ramp up/down the duration with increasing/decreasing activity of the bots
						plugin.banGlobal(plugin.generateBan(line.Nick, banmask,
							"Instant excess flood", 2*24*time.Hour))
						nobotActivityChan <- "Bot excess flooded"
					default:
						nobotActivityChan <- "User quit normally"
					}
				}
			}).Remove()

		// Give nobotActivityChan some time to prove this is not a bot
		select {
		case reason := <-nobotActivityChan:
			logging.Info("vpnbot.Plugin: %v, skipping.", reason)
			return
		case <-time.After(1 * time.Second):
		}

		// Get WHOIS info
		info, err := plugin.whois.WhoIs(line.Nick)
		if err != nil && err != whois.ErrNoSuchNick {
			logging.Warn("vpnbot.Plugin: Can't get WHOIS info for %v, skipping: %v",
				line.Nick, err.Error())
			return
		}

		// Not an oper?
		if info.IsOperator {
			logging.Debug("vpnbot.Plugin: %v is operator, skipping.",
				line.Nick)
			return
		}

		// Not away
		if info.IsAway {
			logging.Debug("vpnbot.Plugin: %v is away, skipping.", line.Nick)
			return
		}

		// Realname == Nickname?
		if info.Realname != line.Nick {
			logging.Debug(
				"vpnbot.Plugin: %v's nick doesn't match the realname, skipping.",
				line.Nick)
			return
		}

		// Count of channels at least 48
		if len(info.Channels) < 48 {
			logging.Debug(
				"vpnbot.Plugin: %v is only in %v channels, skipping.",
				line.Nick, len(info.Channels))
			return
		}

		// Not halfop, op, aop or owner in any channel
		for _, prefix := range info.Channels {
			if prefix == '%' || prefix == '@' ||
				prefix == '&' || prefix == '~' {
				logging.Debug(
					"vpnbot.Plugin: %v is opped in a channel, skipping.",
					line.Nick)
				return
			}
		}

		// Give nobotActivityChan some time to prove this is not a bot
		select {
		case reason := <-nobotActivityChan:
			logging.Info("vpnbot.Plugin: %v, skipping.", reason)
			return
		case <-time.After(250 * time.Millisecond):
		}

		// More expensive tests below...

		// Make sure we deal with an unregistered client

		status := plugin.nickserv.Status(line.Nick)[line.Nick]
		if status.Error != nil {
			logging.Warn("vpnbot.Plugin: Can't get auth status for %v, skipping: %v",
				line.Nick, status.Error)
			return
		}
		if status.Level >= nickserv.Status_IdentifiedViaPassword {
			logging.Debug("vpnbot.Plugin: %v is identified, skipping.",
				line.Nick)
			return
		}

		// Give nobotActivityChan some time to prove this is not a bot
		select {
		case reason := <-nobotActivityChan:
			logging.Info("vpnbot.Plugin: %v, skipping.", reason)
			return
		case <-time.After(250 * time.Millisecond):
		}

		// This is a bot we need to ban!
		banmask := fmt.Sprintf("%v!%v@%v", "*", "*", line.Host)
		// TODO - Ramp up/down the duration with increasing/decreasing activity of the bots
		plugin.banGlobal(plugin.generateBan(line.Nick, banmask,
			"Known pattern of ban-evading/logging bot", 2*24*time.Hour))
	}()
}

func (plugin *Plugin) generateBan(nick string, hostmask string, reason string,
	duration time.Duration) tempban.TemporaryBan {
	return tempban.NewTemporaryBan(
		nick,
		hostmask,
		plugin.bot.Me().Name,
		reason,
		duration)
}

func (plugin *Plugin) banGlobal(bans ...tempban.TemporaryBan) {
	for _, channel := range plugin.bot.Channels() {
		go func(channel string) {
			var errs []error
			errs = plugin.tempban.Kickban(channel, bans...)
			for i, err := range errs {
				ban := bans[i]
				if err != nil {
					logging.Warn("Couldn't ban %v from %v: %v", ban.Nick, channel,
						err.Error())
				}
			}
		}(channel)
	}
}

func (plugin *Plugin) unbanGlobal(hostmask string) {
	modesNative, ok := plugin.isupport.Supports().Modes()
	if !ok {
		modesNative = 1
	}
	modeBuf := util.NewModeChangeBuffer(plugin.bot.Conn(), modesNative)

	for _, channel := range plugin.bot.Channels() {
		// -b will get sent back by the server and then picked up by our mode
		// changing listener that we hooked in the New() method.
		modeBuf.Mode(channel, "-b", hostmask)
	}

	modeBuf.Flush()
}

func Register(b bot.Bot, whoisPlugin *whois.Plugin,
	isupportPlugin *isupport.Plugin, tempbanPlugin *tempban.Plugin,
	nickservPlugin *nickserv.Plugin) *Plugin {
	return New(b, whoisPlugin, isupportPlugin, tempbanPlugin, nickservPlugin)
}
