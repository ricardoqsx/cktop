package domain

import "time"

type EngineInfo struct {
	Name            string
	Endpoint        string
	Transport       string
	Remote          bool
	Secure          bool
	Source          string
	ServerVersion   string
	APIVersion      string
	OperatingSystem string
	NCPU            int
	MemoryTotal     uint64
}

type Container struct {
	ID                 string
	ShortID            string
	Name               string
	Image              string
	ImageID            string
	State              string
	Status             string
	Health             string
	Created            time.Time
	StartedAt          time.Time
	CPUPercent         float64
	CPUAvailable       bool
	MemoryUsage        uint64
	MemoryLimit        uint64
	MemoryPercent      float64
	MemoryAvailable    bool
	SampledAt          time.Time
	ComposeProject     string
	ComposeService     string
	ComposeWorkingDir  string
	ComposeConfigFiles string
}

type Snapshot struct {
	Engine               EngineInfo
	Containers           []Container
	ContainerCPUPercent  float64
	CPUAvailable         bool
	ContainerMemoryUsage uint64
	MemoryAvailable      bool
	SampledAt            time.Time
}

type ContainerDetails struct {
	ID        string
	Name      string
	Image     string
	State     string
	Health    string
	StartedAt time.Time
	Ports     []string
	Networks  []string
}
