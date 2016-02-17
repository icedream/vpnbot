package tempban

import (
	"strings"
	"time"
)

type TemporaryBan struct {
	Nick           string
	Hostmask       string
	Source         string
	Reason         string
	BanTime        time.Time
	ExpirationTime time.Time
}

func NewTemporaryBan(nick string, hostmask string, source string, reason string,
	duration time.Duration) TemporaryBan {
	now := time.Now()
	return TemporaryBan{
		Nick:           nick,
		Hostmask:       hostmask,
		Source:         source,
		Reason:         reason,
		BanTime:        now,
		ExpirationTime: now.Add(duration),
	}
}

func (t TemporaryBan) TotalDuration() time.Duration {
	return t.ExpirationTime.Sub(t.BanTime)
}

func (t TemporaryBan) RemainingDuration() time.Duration {
	return t.ExpirationTime.Sub(time.Now())
}

func (t TemporaryBan) WaitUntilExpired() <-chan time.Time {
	remaining := t.RemainingDuration()
	if remaining > 0 {
		// Wait for expiration via time.After
		return time.After(remaining)
	}

	// Already expired!
	retchan := make(chan time.Time, 1)
	retchan <- time.Now()
	return retchan
}

func (t TemporaryBan) TargetUser() (nick, ident, host string) {
	nick, ident, host = splitHostmask(t.Hostmask)
	return
}

func splitHostmask(hostmask string) (nick, ident, host string) {
	split := strings.SplitN(hostmask, "!", 2)
	nick, hostmask = split[0], split[1]
	split = strings.SplitN(hostmask, "@", 2)
	ident, host = split[0], split[1]
	return
}
