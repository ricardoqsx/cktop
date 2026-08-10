package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Stack struct {
	Name            string
	State           string
	Services        []StackService
	ContainerItems  []Container
	Containers      int
	WorkingDir      string
	Files           []string
	MetadataReason  string
	CPUPercent      float64
	CPUAvailable    bool
	MemoryUsage     uint64
	MemoryLimit     uint64
	MemoryAvailable bool
}

type StackService struct {
	Name       string
	State      string
	Containers int
}

func (s Stack) DownUnavailableReason() string {
	if s.MetadataReason != "" {
		return s.MetadataReason
	}
	if strings.TrimSpace(s.WorkingDir) == "" {
		return "Compose working directory is unavailable"
	}
	if len(s.Files) == 0 {
		return "Compose config files are unavailable"
	}
	return ""
}

func NormalizeComposeMetadata(workingDir, configFiles string) (string, []string, string) {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		return "", nil, "Compose working directory label is missing"
	}
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return "", nil, fmt.Sprintf("invalid Compose working directory: %v", err)
	}
	absWorkingDir = filepath.Clean(absWorkingDir)
	if strings.TrimSpace(configFiles) == "" {
		return absWorkingDir, nil, "Compose config files label is missing"
	}
	files := make([]string, 0)
	for _, value := range strings.Split(configFiles, ",") {
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value == "" {
			return absWorkingDir, nil, "Compose config files label has an empty entry"
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(absWorkingDir, value)
		}
		files = append(files, filepath.Clean(value))
	}
	return absWorkingDir, files, ""
}
