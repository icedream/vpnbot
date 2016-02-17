package tempban

import "errors"

var ErrHostmaskAlreadyBanned = errors.New("Hostmask already banned.")
var ErrNotAChannel = errors.New("Not a channel.")
