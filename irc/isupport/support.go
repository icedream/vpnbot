package isupport

import (
	"strconv"
	"strings"
)

var DefaultChanTypes = []rune("#&")
var DefaultPrefixLetters = []rune("vo")
var DefaultPrefixSymbols = []rune("+@")

type Support struct {
	Data map[string]string
}

// Returns a list of channel modes a person can get and the respective prefix a
// channel or nickname will get in case the person has it. The order of the
// modes goes from most powerful to least powerful. Those prefixes are shown in
// the output of the WHOIS, WHO and NAMES command.
//
// Note: Some servers only show the most powerful, others may show all of them.
func (s *Support) Prefix() (retval *Prefixes, ok bool) {
	retval = &Prefixes{
		Letters: DefaultPrefixLetters,
		Symbols: DefaultPrefixSymbols,
	}

	v, ok := s.Data["PREFIX"]
	if !ok {
		return // No prefix given by the server
	}

	if !strings.HasPrefix(v, "(") {
		return // Missing start bracket
	}

	// Look where the letters end
	endIndex := strings.IndexRune(v[1:], ')') + 1
	if endIndex < 1 {
		return // Missing end bracket
	}

	// Check if there is enough symbols for all letters
	count := endIndex - 1
	if len(v) < (count+1)*2 {
		return // Not enough symbols
	}

	// (modes)prefixes
	letters := []rune(v[1:endIndex])
	symbols := []rune(v[endIndex+1 : endIndex+1+count])

	retval = &Prefixes{
		Letters: letters,
		Symbols: symbols,
	}
	ok = true

	return
}

// Returns the supported channel prefixes.
func (s *Support) ChanTypes() (retval []rune, ok bool) {
	retval = DefaultChanTypes

	v, ok := s.Data["CHANTYPES"]
	if !ok {
		return
	}

	retval = []rune(v)
	ok = true
	return
}

// Returns the list of channel modes according to 4 types.
// They are documented with the ChanModeType constants.
func (s *Support) ChanModes() (retval []ChanMode, ok bool) {
	retval = []ChanMode{}

	v, ok := s.Data["CHANMODES"]
	if !ok {
		return
	}

	modes := strings.Split(v, ",")
	if len(modes) != 4 {
		return // Wrong syntax
	}

	order := []ChanModeType{
		ChanModeType_List,
		ChanModeType_Setting,
		ChanModeType_Setting_ParamWhenSet,
		ChanModeType_Setting_NoParam,
	}

	for index, modesString := range modes {
		modeType := order[index]
		for _, modeRune := range []rune(modesString) {
			retval = append(retval, ChanMode{Mode: modeRune, Type: modeType})
		}
	}

	ok = true

	return
}

// Returns maximum number of channel modes with parameter allowed per MODE
// command.
func (s *Support) Modes() (retval uint64, ok bool) {
	v, ok := s.Data["MODES"]
	if !ok {
		return
	}

	retval, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return
	}
	ok = true
	return
}

// Maximum number of channels allowed to join.
//
// Note: This has been replaced by CHANLIMIT.
func (s *Support) MaxChannels() (retval uint64, ok bool) {
	v, ok := s.Data["MAXCHANNELS"]
	if !ok {
		return
	}

	retval, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return
	}
	ok = true
	return
}

// Maximum number of channels allowed to join by channel prefix.
func (s *Support) ChanLimit() (retval map[rune]uint64, ok bool) {
	v, ok := s.Data["CHANLIMIT"]
	if !ok {
		return
	}

	retvalNew := map[rune]uint64{}
	for _, def := range strings.Split(v, ",") {
		defSplit := strings.Split(def, ":")
		if len(defSplit) != 2 {
			return
		}

		num, err := strconv.ParseUint(defSplit[1], 10, 32)
		if err != nil {
			return
		}

		for _, prefix := range []rune(defSplit[0]) {
			retvalNew[prefix] = num
		}
	}

	retval = retvalNew
	ok = true

	return
}

// Maximum number of bans per channel.
//
// Note: This has been replaced by MAXLIST.
func (s *Support) MaxBans() (retval uint64, ok bool) {
	v, ok := s.Data["MAXBANS"]
	if !ok {
		return
	}

	retval, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return
	}
	ok = true
	return
}

// Maximum number entries in the list per mode.
func (s *Support) MaxList() (retval map[rune]uint64, ok bool) {
	v, ok := s.Data["MAXLIST"]
	if !ok {
		return
	}

	retvalNew := map[rune]uint64{}
	for _, def := range strings.Split(v, ",") {
		defSplit := strings.Split(def, ":")
		if len(defSplit) != 2 {
			return
		}

		num, err := strconv.ParseUint(defSplit[1], 10, 64)
		if err != nil {
			return
		}

		for _, mode := range []rune(defSplit[0]) {
			retvalNew[mode] = num
		}
	}

	retval = retvalNew
	ok = true

	return
}

// The server supports ban exceptions (e mode).
// See RFC 2811 for more information.
func (s *Support) Excepts() (retval rune, ok bool) {
	v, ok := s.Data["EXCEPTS"]
	if !ok || len(v) != 1 {
		ok = false
		return
	}

	retval = rune(v[0])
	ok = true
	return
}

// The server supports invite exceptions (+I mode).
// See RFC 2811 for more information.
func (s *Support) InviteExcepts() (retval rune, ok bool) {
	v, ok := s.Data["INVEX"]
	if !ok || len(v) != 1 {
		ok = false
		return
	}

	ok = true
	retval = rune(v[0])
	return
}

// The server supports messaging channel member who have a certain status or
// higher. The status is one of the letters from PREFIX.
func (s *Support) StatusMsg() (retval []rune, ok bool) {
	v, ok := s.Data["STATUSMSG"]
	if !ok {
		return
	}

	retval = []rune(v)
	ok = true
	return
}

// Case mapping used for nick- and channel name comparing.
//
// Current possible values:
//   - ascii: The chars [a-z] are lowercase of [A-Z].
//   - rfc1459: ascii with additional {}|~ the lowercase of []\^.
//   - strict-rfc1459: ascii with additional {}| the lowercase of []\.
//
// Note: RFC1459 forgot to mention the ~ and ^ although in all known
// implementations those are considered equivalent too.
func (s *Support) CaseMapping() (retval string, ok bool) {
	v, ok := s.Data["CASEMAPPING"]
	if !ok || len(v) <= 0 {
		ok = false
		return
	}

	ok = true
	retval = v
	return
}
