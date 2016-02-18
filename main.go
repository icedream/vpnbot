package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	// YAML

	// IRC bot
	"github.com/StalkR/goircbot/bot"

	// Logs
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/logging"
	glogging "github.com/fluffle/goirc/logging/glog"

	// Plugins
	adminplugin "github.com/StalkR/goircbot/plugins/admin"
	"github.com/icedream/vpnbot/irc/autojoin"
	"github.com/icedream/vpnbot/irc/bots"
	"github.com/icedream/vpnbot/irc/dumptempban"
	"github.com/icedream/vpnbot/irc/isupport"
	"github.com/icedream/vpnbot/irc/mode"
	"github.com/icedream/vpnbot/irc/nickserv"
	"github.com/icedream/vpnbot/irc/tempban"
	"github.com/icedream/vpnbot/irc/vpnbot"
	"github.com/icedream/vpnbot/irc/whois"
)

// Program version, build server changes this at compile time to represent
// the `git describe` output of the current commit.
var version = "dev"

// Loaded configuration
var loadedConfiguration = defaultConfiguration

// Command line flags
var configPath = flag.String("config", "config.yml",
	"Path to the configuration file. Configuration file must be in YAML format.")
var migratePath = flag.String("migrate", "",
	"If given with a path, will migrate from version 1 of VPN bot to the new configuration format.")
var generateDefault = flag.Bool("generate", false,
	"Generates a default configuration and saves it at the path given via -config.")
var makeTempBans = flag.Bool("maketempbans", false,
	"Causes the bot to only connect shortly to dump all its previously set bans to temporary bans. This is useful for V1 migration.")

// Logger
var logger glogging.GLogger

// The main program logic.
func main() {
	// Load configuration path from flags
	flag.Parse()

	// Initialize the logger
	logger = glogging.GLogger{}
	logging.SetLogger(logger)

	// Check if we're supposed to generate a default config
	if *generateDefault {
		logger.Debug("Saving default configuration...")
		if err := defaultConfiguration.Save(*configPath); err != nil {
			logger.Error("Failed at saving default configuration: %v", err)
			os.Exit(1)
		}
		logger.Info("Saved default configuration.")
		os.Exit(0)
	}

	// Check if we're supposed to migrate an old config
	if *migratePath != "" {
		logger.Debug("Migrating old configuration...")
		if c, err := LoadV1Config(*migratePath); err != nil {
			logger.Error("Failed to load old configuration: %v", err)
			os.Exit(1)
		} else {
			newC := c.Migrate()
			if err := newC.Save(*configPath); err != nil {
				logger.Error("Migration failed: %v", err)
				os.Exit(1)
			}
			if err := newC.Validate(); err != nil {
				logger.Warn("Migration successful but found errors while "+
					"validating the new configuration, you should fix this before "+
					"running the bot: %v", err)
				os.Exit(2)
			}
		}
		logger.Info("Migration successful.")
		os.Exit(0)
	}

	// Load configuration from configuration path
	if c, err := Load(*configPath); err != nil {
		logger.Error("Can't load configuration from %v: %v\n", *configPath, err)
		os.Exit(1)
	} else {
		loadedConfiguration = c
	}

	logger.Debug("Loaded configuration will be printed below.")
	logger.Debug("%#v", loadedConfiguration)

	// Validate configuration
	if err := loadedConfiguration.Validate(); err != nil {
		logger.Error("The configuration is invalid: %v\n", err)
		os.Exit(2)
	}

	// Now initialize the bot
	logger.Info("Initializing vpnbot %v...", version)
	b := bot.NewBot(
		loadedConfiguration.Server.Address,
		loadedConfiguration.Server.SSL,
		loadedConfiguration.Nick,
		loadedConfiguration.Ident,
		[]string{})
	b.Conn().Config().Version = fmt.Sprintf("vpnbot/%v", version)
	b.Conn().Config().Recover = func(conn *client.Conn, line *client.Line) {
		if err := recover(); err != nil {
			logging.Error("An internal error occurred: %v\n%v",
				err, string(debug.Stack()))
		}
	}
	b.Conn().Config().Pass = loadedConfiguration.Server.Password
	if loadedConfiguration.Name != "" {
		b.Conn().Config().Me.Name = loadedConfiguration.Name
	}

	// Load plugins
	// TODO - Move this into its own little intelligent loader struct, maybe.
	isupportPlugin := isupport.Register(b)
	modePlugin := mode.Register(b, isupportPlugin)
	nickservPlugin := nickserv.Register(b, modePlugin)
	nickservPlugin.Username = loadedConfiguration.NickServ.Username
	nickservPlugin.Password = loadedConfiguration.NickServ.Password
	nickservPlugin.Channels = loadedConfiguration.Channels

	switch {
	case *makeTempBans: // Run in tempban dumping mode
		// Prepare channels to let us know about dumped bans
		doneChan := make(map[string]chan interface{})
		for _, channel := range loadedConfiguration.Channels {
			doneChan[strings.ToLower(channel)] = make(chan interface{}, 1)
		}

		// Load the tempban dumping plugin
		dumptempbanPlugin := dumptempban.Register(b, isupportPlugin, modePlugin)
		dumptempbanPlugin.DumpedBansFunc = func(target string, num int, err error) {
			if err != nil {
				logging.Error("Failed to dump bans for %v: %v", target, err)
			} else {
				logging.Info("Dumped %v bans for %v successfully.", num, target)
			}
			if done, ok := doneChan[strings.ToLower(target)]; ok {
				done <- nil
			}
		}

		// Start up the bot asynchronously
		go b.Run()

		// Wait for all channels to be done
		for _, done := range doneChan {
			<-done
		}
		b.Quit("Ban dumping done.")

	default: // Run normally
		// Load plugins
		autojoin.Register(b)
		adminplugin.Register(b, loadedConfiguration.Admins)
		bots.Register(b, isupportPlugin)
		tempbanPlugin := tempban.Register(b, isupportPlugin, modePlugin)
		whoisPlugin := whois.Register(b, isupportPlugin)
		vpnbotPlugin := vpnbot.Register(b, whoisPlugin, tempbanPlugin, nickservPlugin)
		vpnbotPlugin.Admins = loadedConfiguration.Admins

		// This is to update the configuration when the bot joins channels
		b.HandleFunc("join",
			func(c *client.Conn, line *client.Line) {
				// Arguments: [ <channel> ]

				// Make sure this is about us
				if line.Nick != c.Me().Nick {
					return
				}

				// I don't think method calls are a good idea in a loop
				channel := line.Target()

				// See if we already had this channel saved
				for _, savedChannel := range loadedConfiguration.Channels {
					if strings.EqualFold(savedChannel, channel) {
						return // Channel already saved
					}
				}

				// Store this channel
				logger.Info("Adding %v to configured channels", channel)
				loadedConfiguration.Channels = append(
					loadedConfiguration.Channels, channel)

				// And save to configuration file!
				loadedConfiguration.Save(*configPath)
			})
		b.HandleFunc("kick",
			func(c *client.Conn, line *client.Line) {
				// Arguments: [ <channel>, <nick>, <reason> ]

				// Make sure this is about us
				if line.Args[1] != c.Me().Nick {
					return
				}

				// I don't think method calls are a good idea in a loop
				channel := line.Target()

				for index, savedChannel := range loadedConfiguration.Channels {
					if strings.EqualFold(savedChannel, channel) {
						// Delete the channel
						logger.Info("Removing %v from configured channels", savedChannel)
						loadedConfiguration.Channels = append(
							loadedConfiguration.Channels[0:index],
							loadedConfiguration.Channels[index+1:]...)

						// And save to configuration file!
						loadedConfiguration.Save(*configPath)
						return
					}
				}
			})
		b.HandleFunc("part",
			func(c *client.Conn, line *client.Line) {
				// Arguments: [ <channel> (, <reason>) ]

				// Make sure this is about us
				if line.Nick != c.Me().Nick {
					return
				}

				// I don't think method calls are a good idea in a loop
				channel := line.Target()

				for index, savedChannel := range loadedConfiguration.Channels {
					if strings.EqualFold(savedChannel, channel) {
						// Delete the channel
						logger.Info("Removing %v from configured channels", savedChannel)
						loadedConfiguration.Channels = append(
							loadedConfiguration.Channels[0:index],
							loadedConfiguration.Channels[index+1:]...)

						// And save to configuration file!
						loadedConfiguration.Save(*configPath)
						return
					}
				}
			})

		// Run the bot
		b.Run()
	}
}
