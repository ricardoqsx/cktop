package domain

import "time"

type Image struct {
	ID         string
	ShortID    string
	Name       string
	Tags       []string
	Size       uint64
	Created    time.Time
	Containers int64
	UsageKnown bool
	Dangling   bool
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
