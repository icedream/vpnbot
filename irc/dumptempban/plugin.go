package dumptempban

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
	"github.com/icedream/vpnbot/irc/isupport"
	"github.com/icedream/vpnbot/irc/mode"
	"github.com/icedream/vpnbot/irc/tempban"
)

type Plugin struct {
	bot      bot.Bot
	tbmgr    map[string]*tempban.TemporaryBanManager
	isupport *isupport.Plugin
	mode     *mode.Plugin

	DumpedBansFunc func(target string, num int, err error)
}

func New(b bot.Bot, isupportPlugin *isupport.Plugin, modePlugin *mode.Plugin) *Plugin {
	plugin := &Plugin{
		bot:      b,
		tbmgr:    map[string]*tempban.TemporaryBanManager{},
		isupport: isupportPlugin,
		mode:     modePlugin,
	}

	b.HandleFunc("join",
		func(conn *client.Conn, line *client.Line) {
			if line.Nick != conn.Me().Nick {
				return
			}

			plugin.loadBans(line.Args[0])
			go plugin.dumpBans(line.Args[0])
		})

	return plugin
}

func (p *Plugin) getTempbansFilename(target string) string {
	hash := sha256.New()
	hash.Write([]byte(p.bot.Conn().Config().Server))
	if p.bot.Conn().Config().SSL {
		hash.Write([]byte{1})
	} else {
		hash.Write([]byte{0})
	}
	hash.Write([]byte(strings.ToLower(target)))
	return fmt.Sprintf("target_%x.tempban", string(hash.Sum([]byte{})))
}

func (p *Plugin) ensureTemporaryBanManager(target string) (tbmgr *tempban.TemporaryBanManager) {
	target = strings.ToLower(target)
	tbmgr, ok := p.tbmgr[target]
	if !ok {
		tbmgr = tempban.NewTemporaryBanManager()
		tbmgr.DisableExpiry = true
		p.tbmgr[target] = tbmgr
	}
	return
}

func (p *Plugin) loadBans(target string) {
	logging.Debug("Loading temporary bans for %v from disk...", target)

	// Check if file exists
	fn := p.getTempbansFilename(target)
	f, err := os.Open(fn)
	switch {
	case os.IsNotExist(err):
		return
	case err == nil:
	default:
		logging.Warn("Could not load temporary bans for %v: %v",
			fn, target, err.Error())
	}
	defer f.Close()

	// Load temporary bans from this file
	if err := p.ensureTemporaryBanManager(target).Import(f); err != nil {
		logging.Warn("Could not load temporary bans: %v", err.Error())
	}
}

func (p *Plugin) syncBans(target string) {
	// Synchronize bans to file
	logging.Debug("Synchronizing temporary bans for %v to disk...", target)
	fn := p.getTempbansFilename(target)
	f, err := os.Create(fn)
	if err != nil {
		logging.Warn("Could not save temporary bans for %v: %v",
			fn, target, err.Error())
	}
	defer f.Close()

	// Load temporary bans from this file
	if err := p.ensureTemporaryBanManager(target).Export(f); err != nil {
		logging.Warn("Could not save temporary bans: %v", err.Error())
	}
}

func (p *Plugin) dumpBans(target string) {
	num := 0

	// Fetch ban list
	banlist, err := p.mode.Bans(target)
	if err != nil {
		p.onDumpedBans(target, num, err)
		return
	}

	tbmgr := p.ensureTemporaryBanManager(target)

	// Save only bans from us
	for _, ban := range banlist {
		if ban.Nick != p.bot.Me().Nick {
			// Not a ban from us (going by the nickname at least)
			continue
		}

		if _, ok := tbmgr.Get(ban.Hostmask); ok {
			// We already have this ban saved
			continue
		}

		if err := tbmgr.Add(tempban.NewTemporaryBan(
			ban.Nick,
			ban.Hostmask,
			ban.Src,
			"Migrated old ban",
			48*time.Hour+ban.Timestamp.Sub(time.Now()))); err != nil {
			p.onDumpedBans(target, num, err)
			return
		}

		num++
	}

	p.syncBans(target)
	p.onDumpedBans(target, num, nil)
}

func (p *Plugin) onDumpedBans(target string, num int, err error) {
	f := p.DumpedBansFunc
	if f != nil {
		f(target, num, err)
	}
}

func Register(b bot.Bot, isupportPlugin *isupport.Plugin, modePlugin *mode.Plugin) *Plugin {
	return New(b, isupportPlugin, modePlugin)
}
