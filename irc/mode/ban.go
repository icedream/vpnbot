package mode

import "time"

type Ban struct {
	User
	Src       string
	Hostmask  string
	Timestamp time.Time
}
