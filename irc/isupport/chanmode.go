package isupport

type ChanModeType byte

const (
	// Mode that adds or removes a nick or address to a list.
	// Always has a parameter.
	ChanModeType_List ChanModeType = iota

	// Mode that changes a setting.
	// Always has a parameter.
	ChanModeType_Setting

	// Mode that changes a setting.
	// Only has a parameter when set.
	ChanModeType_Setting_ParamWhenSet

	// Mode that changes a setting.
	// Never has a parameter.
	ChanModeType_Setting_NoParam
)

type ChanMode struct {
	Type ChanModeType
	Mode rune
}
