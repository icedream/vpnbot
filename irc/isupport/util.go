package isupport

func SplitIrcPrefix(s string, availablePrefixes []rune) (prefixes []rune, rest string) {
	rest = s
	gotPrefix := true
	for gotPrefix {
		gotPrefix = false
		for _, availablePrefix := range availablePrefixes {
			if rune(rest[0]) == availablePrefix {
				gotPrefix = true
				rest = rest[1:]
				prefixes = append(prefixes, availablePrefix)
				break
			}
		}
	}
	return
}
