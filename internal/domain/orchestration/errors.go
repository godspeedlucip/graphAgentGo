package orchestration

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid orchestration input")
	ErrInvalidTransition = errors.New("invalid orchestration transition")
	ErrStepLimitExceeded = errors.New("step limit exceeded")
)
