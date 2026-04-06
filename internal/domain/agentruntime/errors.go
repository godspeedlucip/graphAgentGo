package agentruntime

import "errors"

var (
	ErrInvalidInput = errors.New("invalid runtime input")
	ErrBuildFailed  = errors.New("build runtime failed")
)