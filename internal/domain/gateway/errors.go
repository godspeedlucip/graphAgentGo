package gateway

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid gateway input")
	ErrNoRouteMatched = errors.New("no route matched")
	ErrBodyTooLarge   = errors.New("gateway request body too large for replay")
)
