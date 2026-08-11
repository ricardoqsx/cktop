package domain

import "time"

type Image struct {
	ID          string
	ShortID     string
	Name        string
	Tags        []string
	Size        uint64
	Created     time.Time
	Containers  int64
	UsageKnown  bool
	Dangling    bool
	RepoDigests []string
	Update      UpdateStatus
}

type UpdateStatus string

const (
	UpdateChecking              UpdateStatus = "checking"
	UpdateCurrent               UpdateStatus = "current"
	UpdateAvailable             UpdateStatus = "available"
	UpdateUnknown               UpdateStatus = "unknown"
	UpdatePinned                UpdateStatus = "pinned"
	UpdatePulledPendingRecreate UpdateStatus = "pulled_pending_recreate"
)

type ImageUpdate struct {
	ImageID string
	Status  UpdateStatus
	Reason  string
}

type ImageDetails struct {
	ID           string
	Tags         []string
	Digests      []string
	Size         uint64
	Created      time.Time
	Architecture string
	OS           string
}
