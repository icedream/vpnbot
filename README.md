# Icedream's Anti-VPN bot

This bot automatically sets bans when it detects clients with specific patterns known from bots that evade bans and (probably) log channels.

The pattern is roughly as follows:

- Nickname, Ident and Real name are the same (Ident shortened down to first 9 characters)
- Client joins a very high amount of channels.
- Client is not identified.
- Client generates nicknames from a preset of words and random numbers.
- Client connects from various IP addresses (botnet)
- It optionally quits very soon with an "Excess Flood" message due to it spamming channel joins.

# How to install this

```
go get -v github.com/icedream/vpnbot
```

Make sure your GOPATH is set properly and that you have PATH also pointing at your GOPATH binary folder.

# How to run this

First, generate a default configuration in a new folder:

```
mkdir vpnbot
cd vpnbot
vpnbot -generate
```

This will generate `config.yml` - the file path can be changed by appending `-config "<your own path here>"`. Edit this file to your needs, then rerun the bot without -generate.

```
vpnbot
```

Again, optionally add `-config "<your own path here>"` if you use your own path to the configuration file.

# License

This work is licensed under the GNU General Public License Version 3. For more information check [LICENSE.txt](LICENSE.txt).