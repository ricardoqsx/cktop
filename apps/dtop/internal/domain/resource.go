package domain

import "time"

type Network struct {
	ID         string
	ShortID    string
	Name       string
	Driver     string
	Scope      string
	Created    time.Time
	Containers int
	UsageKnown bool
	Internal   bool
	Attachable bool
}

type NetworkDetails struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Created    time.Time
	Internal   bool
	Attachable bool
	Containers []string
}

type Volume struct {
	Name       string
	Driver     string
	Scope      string
	Mountpoint string
	Created    time.Time
	Containers int
	UsageKnown bool
}

type VolumeDetails struct {
	Name       string
	Driver     string
	Scope      string
	Mountpoint string
	Created    time.Time
	Options    map[string]string
	Labels     map[string]string
}
