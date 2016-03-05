package tempban

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
	"github.com/icedream/vpnbot/irc/isupport"
	"github.com/icedream/vpnbot/irc/mode"
	"github.com/icedream/vpnbot/irc/util"

	"github.com/dustin/go-humanize"
)

type Plugin struct {
	bot          bot.Bot
	tbmgr        map[string]*TemporaryBanManager
	isupport     *isupport.Plugin
	mode         *mode.Plugin
	OldHostmasks []string
}

func New(b bot.Bot, isupportPlugin *isupport.Plugin, modePlugin *mode.Plugin) *Plugin {
	plugin := &Plugin{
		bot:          b,
		tbmgr:        map[string]*TemporaryBanManager{},
		isupport:     isupportPlugin,
		mode:         modePlugin,
		OldHostmasks: []string{},
	}

	modePlugin.HandleFunc("-b", func(e *mode.ModeChangeEvent) {
		if e.Nick == b.Me().Nick {
			return // this is us
		}

		if ok, _, _ := isupportPlugin.IsChannel(e.Target); !ok {
			return // not a channel
		}

		hostmask := e.Argument
		tbmgr := plugin.ensureTemporaryBanManager(e.Target)
		if ban, ok := tbmgr.Remove(hostmask); ok {
			logging.Debug("%v: %v removed the temporary ban for %v",
				e.Target, e.Nick, ban.Hostmask)
			plugin.syncBans(e.Target)
		}
	})

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
	hash.Write([]byte(strings.ToLower(target)))
	return fmt.Sprintf("target_%x.tempban", string(hash.Sum([]byte{})))
}

func (p *Plugin) ensureTemporaryBanManager(target string) (tbmgr *TemporaryBanManager) {
	target = strings.ToLower(target)
	tbmgr, ok := p.tbmgr[target]
	if !ok {
		tbmgr = NewTemporaryBanManager()
		tbmgr.BanExpiredFunc = func(ban TemporaryBan) {
			banSetChan := make(chan error)
			go func() {
				defer p.bot.HandleFunc("482", // ERR_CHANOPRIVSNEEDED
					func(conn *client.Conn, line *client.Line) {
						if banSetChan == nil {
							return
						}
						if line.Args[0] != conn.Me().Nick ||
							line.Args[1] != target {
							return
						}
						banSetChan <- errors.New("Missing channel operator privileges")
					}).Remove()
				defer p.mode.HandleFunc("-b",
					func(e *mode.ModeChangeEvent) {
						if banSetChan == nil {
							return
						}
						if strings.EqualFold(e.Target, target) {
							banSetChan <- nil
						}
					}).Remove()

				// +b-b will definitely trigger MODE -b if successful
				p.bot.Mode(target, "+b-b", ban.Hostmask, ban.Hostmask)
				if err := <-banSetChan; err != nil {
					// TODO - Make this cleaner somehow
					// Right now we just requeue the ban to expire again in a minute
					// to allow operators to give the bot permissions in that time.
					logging.Warn("Could not remove expired ban %v, expanding ban by a minute",
						ban.Hostmask)
					tbmgr.Add(NewTemporaryBan(ban.Nick, ban.Hostmask, ban.Source,
						ban.Reason, 60*time.Second))
				}

				p.syncBans(target)
			}()
		}
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
		logging.Warn("%v: Could not fetch ban list, old bans won't get handled (%v)", target, err)
		return
	}

	tbmgr := p.ensureTemporaryBanManager(target)

	// Save only bans from us
	for _, ban := range banlist {
		if ban.Nick != p.bot.Me().Nick {
			// Not a ban from us (going by the nickname at least)
			isOldHostmask := false
			for _, hostmask := range p.OldHostmasks {
				// TODO - Test this implementation
				hostmask = regexp.QuoteMeta(hostmask)
				hostmask = strings.Replace(hostmask, "\\*", ".*", -1)
				hostmask = strings.Replace(hostmask, "\\?", ".?", -1)
				if matched, err := regexp.MatchString(hostmask, ban.Src); matched {
					isOldHostmask = true
					break
				} else if err != nil {
					logging.Error("vpnbot.Plugin: dumpBans regular expression failed: %v",
						err)
					break
				}
			}
			if !isOldHostmask {
				// Not a ban from an old hostmask either
				continue
			}
		}

		if _, ok := tbmgr.Get(ban.Hostmask); ok {
			// We already have this ban saved
			continue
		}

		if err := tbmgr.Add(NewTemporaryBan(
			ban.Nick,
			ban.Hostmask,
			ban.Src,
			"Migrated old ban",
			48*time.Hour+ban.Timestamp.Sub(time.Now()))); err != nil {
			logging.Warn("Could not migrate ban on %v: %v", ban.Hostmask, err)
		}

		num++
	}

	if num > 0 {
		p.syncBans(target)
		logging.Info("Migrated %v bans", num)
	}
}

func (p *Plugin) Bans(target string) []TemporaryBan {
	return p.ensureTemporaryBanManager(target).GetAll()
}

// Sets multiple bans in a target channel.
//
// Returns an error slice of the same amount of elements as bans
// passed in, each error corresponding to the respective ban by index.
//
// All errors will be ErrNotAChannel if the target is not a valid channel.
func (p *Plugin) Ban(target string, bans ...TemporaryBan) (retval []error) {
	retval = make([]error, len(bans))

	if ok, _, _ := p.isupport.IsChannel(target); !ok {
		for index, _ := range retval {
			retval[index] = ErrNotAChannel
		}
		return // not a channel
	}

	modesNative, ok := p.isupport.Supports().Modes()
	if !ok {
		modesNative = 1
	}
	modeBuf := util.NewModeChangeBuffer(p.bot.Conn(), modesNative)

	// NOTE - We assume that responses to bans get returned in the same order
	// as we send out the ban requests... this may very well go wrong!

	index := 0
	banSetChans := []chan error{}
	banSetMutex := &sync.Mutex{}

	// Populate banSetChans with channels to send to
	// TODO - There probably is a more efficient way to do this
	for _, _ = range bans {
		banSetChans = append(banSetChans, make(chan error))
	}

	// TODO - This will very well fail on multiple async calls to this method
	// on the same target while both are waiting for replies as the index will
	// get bigger than the length of bans.
	// We also can not use a map by hostmask as some of these replies are far
	// too vague and don't provide anything but our nickname and the error
	// message.
	// Sometimes I really hate the IRC protocol because of things like that...

	defer p.bot.HandleFunc("482", // ERR_CHANOPRIVSNEEDED
		func(conn *client.Conn, line *client.Line) {
			if line.Args[0] != conn.Me().Nick ||
				line.Args[1] != target {
				return
			}

			banSetMutex.Lock()
			defer banSetMutex.Unlock()
			for i := index; i < int(math.Min(float64(len(banSetChans)), float64(uint64(index)+modesNative))); i++ {
				banSetChans[index] <- errors.New("Missing channel operator privileges")
			}

			// This error message counts for all bans sent in the same request
			// line.
			index += int(modesNative)
		}).Remove()
	defer p.bot.HandleFunc("478", // ERR_BANLISTFULL
		func(conn *client.Conn, line *client.Line) {
			if line.Args[0] != conn.Me().Nick ||
				line.Args[1] != target {
				return
			}

			banSetMutex.Lock()
			defer banSetMutex.Unlock()
			for i := index; i < int(math.Min(float64(len(banSetChans)), float64(uint64(index)+modesNative))); i++ {
				banSetChans[index] <- errors.New("The ban list is full")
			}

			// This error message counts for all bans sent in the same request
			// line.
			index += int(modesNative)
		}).Remove()
	defer p.mode.HandleFunc("+b",
		func(e *mode.ModeChangeEvent) {
			if !strings.EqualFold(e.Target, target) {
				return
			}

			banSetMutex.Lock()
			defer banSetMutex.Unlock()

			banSetChans[index] <- nil

			index++
		}).Remove()

	// -b+b will definitely trigger a MODE +b response if the ban can be set
	for _, ban := range bans {
		modeBuf.Mode(target, "-b", ban.Hostmask)
		modeBuf.Mode(target, "+b", ban.Hostmask)
	}
	modeBuf.Flush()

	// Now wait for the responses
	anyBanAppliedCorrectly := false
	for i, ch := range banSetChans {
		select {
		case err := <-ch:
			close(ch)
			retval[i] = err
			if err == nil {
				p.ensureTemporaryBanManager(target).Add(bans[i])
				anyBanAppliedCorrectly = true
			}
		}
	}

	if anyBanAppliedCorrectly {
		p.syncBans(target)
	}

	return
}

func (p *Plugin) Kickban(target string, bans ...TemporaryBan) (errs []error) {
	errs = p.Ban(target, bans...)
	for index, ban := range bans {
		err := errs[index]
		reason := ban.Reason
		if err == nil {
			reason = fmt.Sprintf("Banned until %v (%v)",
				humanize.Time(ban.ExpirationTime),
				ban.Reason)
		}
		if ban.Nick != "" && err != ErrNotAChannel {
			p.bot.Conn().Kick(target, ban.Nick, reason)
		}
	}
	return
}

func Register(b bot.Bot, isupportPlugin *isupport.Plugin, modePlugin *mode.Plugin) *Plugin {
	return New(b, isupportPlugin, modePlugin)
}
