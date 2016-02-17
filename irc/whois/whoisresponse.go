package whois

import "time"

const (
	WhoIsChannelModeNone = WhoIsChannelMode(0)
)

type WhoIsChannelMode rune

type WhoIsServerInfo struct {
	// The server's hostname
	Host string

	// The information associated with this server
	Info string
}

type WhoIsResponse struct {
	// TODO 311 RPL_WHOISUSER "<nick> <user> <host> * :<real name>"

	// The user's nickname.
	Nick     string
	Ident    string
	Host     string
	Realname string

	// TODO 312 RPL_WHOISSERVER "<nick> <server> :<server info>"
	Server WhoIsServerInfo

	// TODO 313 RPL_WHOISOPERATOR "<nick> :is an IRC operator"
	IsOperator bool

	// TODO 301 RPL_AWAY "<nick> :<away message>"
	IsAway      bool
	AwayMessage string

	// TODO 317 RPL_WHOISIDLE "<nick> <integer> :seconds idle"
	IdleDuration time.Duration
	SignOnTime   time.Time

	// TODO 319 RPL_WHOISCHANNELS "<nick> :*( ( "@" / "+" ) <channel> " " )"
	Channels map[string]WhoIsChannelMode

	// TODO 318 RPL_ENDOFWHOIS "<nick> :End of WHOIS list"
	// TODO 401 ERR_NOSUCHNICK "<nickname> :No such nick/channel"
	// TODO 431 ERR_NONICKNAMEGIVEN
}
