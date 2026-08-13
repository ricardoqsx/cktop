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

func (s Status) messageID() string {
	switch s {
	case StatusLoading:
		return MessageShellStatusLoading
	case StatusEmpty:
		return MessageShellStatusEmpty
	case StatusWarning:
		return MessageShellStatusWarning
	case StatusError:
		return MessageShellStatusError
	case StatusUnavailable:
		return MessageShellStatusUnavailable
	default:
		return MessageShellStatusReady
	}
}
