package domain

import "errors"

// Sentinel errors shared across layers.
var (
	// ErrNotFound means a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrUnknown means a name did not match any known value.
	ErrUnknown = errors.New("unknown")
	// ErrInvalid means input broke a rule or was malformed.
	ErrInvalid = errors.New("invalid")
	// ErrNotYourTurn means an action was attempted out of turn.
	ErrNotYourTurn = errors.New("not your turn")
	// ErrWrongMode means the action does not fit the current game mode.
	ErrWrongMode = errors.New("wrong game mode")
	// ErrBusy means the table is still resolving a previous action.
	ErrBusy = errors.New("the table is busy")
)
