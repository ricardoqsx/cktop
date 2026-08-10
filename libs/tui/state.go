package tui

// Status describes a neutral UI state. Product-specific meaning belongs in each app.
type Status int

const (
	StatusReady Status = iota
	StatusLoading
	StatusEmpty
	StatusWarning
	StatusError
	StatusUnavailable
)

func (s Status) label() string {
	switch s {
	case StatusLoading:
		return "loading"
	case StatusEmpty:
		return "empty"
	case StatusWarning:
		return "warning"
	case StatusError:
		return "error"
	case StatusUnavailable:
		return "unavailable"
	default:
		return "ready"
	}
}
