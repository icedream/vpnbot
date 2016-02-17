package nickserv

import "errors"

type NickServStatusLevel byte

var ErrTooManyNicks error = errors.New("Too many nicknames")
var ErrTimedOut error = errors.New("Timed out")

const (
	// No such user online or nickname not registered.
	Status_NoSuchUser NickServStatusLevel = 0

	// User not recognized as nickname's owner.
	Status_NotRecognizedAsOwner = 1

	// User recognized as owner of nickname via access list only.
	Status_IdentifiedViaAccessList = 2

	// User recognized as owner of nickname via password identification.
	Status_IdentifiedViaPassword = 3
)

type NickServStatusResponse struct {
	Nick  string
	Level NickServStatusLevel
	Error error
}
