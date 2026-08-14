package ports

import (
	"context"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

type LogStream struct {
	Lines  <-chan string
	Errors <-chan error
}

// ResourceLoad keeps independently recoverable resource results outside the domain.
type ResourceLoad struct {
	Stacks      []domain.Stack
	StacksErr   error
	Images      []domain.Image
	ImagesErr   error
	Networks    []domain.Network
	NetworksErr error
	Volumes     []domain.Volume
	VolumesErr  error
}

type ComposeServiceImage struct {
	Service    string
	Reference  string
	Build      bool
	PullPolicy string
}

type ComposeUpdateStore interface {
	Get(project string) (domain.ComposeUpdateProject, bool)
	Put(project domain.ComposeUpdateProject) error
	Health() error
	BeginMutation() (func(), error)
}

type Runtime interface {
	Snapshot(ctx context.Context) (domain.Snapshot, error)
	LoadResources(ctx context.Context) (ResourceLoad, error)
	Stacks(ctx context.Context) ([]domain.Stack, error)
	Images(ctx context.Context) ([]domain.Image, error)
	ImageDetails(ctx context.Context, id string) (domain.ImageDetails, error)
	Networks(ctx context.Context) ([]domain.Network, error)
	NetworkDetails(ctx context.Context, id string) (domain.NetworkDetails, error)
	Volumes(ctx context.Context) ([]domain.Volume, error)
	VolumeDetails(ctx context.Context, name string) (domain.VolumeDetails, error)
	Details(ctx context.Context, id string) (domain.ContainerDetails, error)
	Logs(ctx context.Context, id string, tail int) (LogStream, error)
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Remove(ctx context.Context, id string, force bool) error
	RemoveImage(ctx context.Context, id string, force bool) error
	PullImage(ctx context.Context, reference string) error
	RecreateContainer(ctx context.Context, id string, reference string) error
	RemoveNetwork(ctx context.Context, id string) error
	RemoveVolume(ctx context.Context, name string) error
	Up(ctx context.Context, stack domain.Stack) error
	StopStack(ctx context.Context, stack domain.Stack) error
	RestartStack(ctx context.Context, stack domain.Stack) error
	PullStack(ctx context.Context, stack domain.Stack) error
	PullStackServices(ctx context.Context, stack domain.Stack, services []string) error
	UpStackServices(ctx context.Context, stack domain.Stack, services []string) error
	Down(ctx context.Context, stack domain.Stack) error
	ComposeLogs(ctx context.Context, stack domain.Stack, tail int) (LogStream, error)
	ComposeConfig(ctx context.Context, stack domain.Stack) ([]ComposeServiceImage, error)
	Prune(ctx context.Context, args ...string) (string, error)
}
