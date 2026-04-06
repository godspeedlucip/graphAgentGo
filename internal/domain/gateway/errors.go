package gateway

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid gateway input")
	ErrNoRouteMatched = errors.New("no route matched")
)