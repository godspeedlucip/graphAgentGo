package runlifecycle

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusInit: {
		StatusRunning:  {},
		StatusFailed:   {},
		StatusCanceled: {},
		StatusTimeout:  {},
	},
	StatusRunning: {
		StatusDone:     {},
		StatusFailed:   {},
		StatusCanceled: {},
		StatusTimeout:  {},
	},
}

func CanTransit(from Status, to Status) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func IsTerminal(s Status) bool {
	switch s {
	case StatusDone, StatusFailed, StatusCanceled, StatusTimeout:
		return true
	default:
		return false
	}
}
