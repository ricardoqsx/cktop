package domain

type ComposeUpdateProject struct {
	Name                    string
	RegistrationFingerprint string
	ConfigFingerprint       string
	Services                map[string]ComposeUpdateService
}

type ComposeUpdateService struct {
	Reference         string
	DownloadedDigest  string
	DownloadedImageID string
	AppliedDigest     string
	AppliedImageID    string
	PendingUnknown    bool
}

func (project ComposeUpdateProject) Pending() bool {
	for _, service := range project.Services {
		if service.PendingUnknown || service.DownloadedDigest != "" && service.DownloadedDigest != service.AppliedDigest {
			return true
		}
	}
	return false
}

func (project ComposeUpdateProject) PendingUnknown() bool {
	for _, service := range project.Services {
		if service.PendingUnknown {
			return true
		}
	}
	return false
}
