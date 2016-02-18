package mode

import "strings"

type User struct {
	Nick, Ident, Host string
}

func splitHostmask(hostmask string) (retval User) {
	if strings.Contains(hostmask, "!") {
		split := strings.SplitN(hostmask, "!", 2)
		retval.Nick, hostmask = split[0], split[1]
	}
	if strings.Contains(hostmask, "@") {
		split := strings.SplitN(hostmask, "@", 2)
		retval.Ident, hostmask = split[0], split[1]
	}
	retval.Host = hostmask
	return
}
