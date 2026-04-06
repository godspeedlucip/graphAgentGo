package runlifecycle

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid run lifecycle input")
	ErrInvalidTransition = errors.New("invalid run status transition")
	ErrRunNotFound       = errors.New("run not found")
	ErrRunAlreadyExists  = errors.New("run already exists")
	ErrRunBusy           = errors.New("run is already in progress")
)
