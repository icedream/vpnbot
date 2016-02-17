package whois

import "errors"

var ErrNoSuchNick = errors.New("No such nick/channel")
var ErrInvalidNick = errors.New("Invalid or no nickname given")
