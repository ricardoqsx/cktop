package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

func TestNormalizeImageReference(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"nginx:1.27", "docker.io/library/nginx:1.27"}, {"linuxserver/jellyfin:10", "docker.io/linuxserver/jellyfin:10"}, {"ghcr.io/acme/app:2", "ghcr.io/acme/app:2"}, {"registry:5000/acme/app:2", "registry:5000/acme/app:2"}, {"redis@sha256:ABC", "docker.io/library/redis@sha256:abc"},
	} {
		if got, ok := NormalizeImageReference(test.in); !ok || got != test.want {
			t.Fatalf("%q: got %q ok=%v", test.in, got, ok)
		}
	}
}

func TestParseManifestDigest(t *testing.T) {
	digest, err := ParseManifestDigest([]byte(`{"Descriptor":{"digest":"sha256:ABC"},"SchemaV2Manifest":{"config":{"digest":"sha256:def"}}}`))
	if err != nil || digest != "sha256:abc" {
		t.Fatalf("got %q, %v", digest, err)
	}
}

func TestParseManifestDigestUsesTopLevelIndexAndRejectsPlatformArray(t *testing.T) {
	digest, err := ParseManifestDigest([]byte(`{"digest":"sha256:INDEX","manifests":[{"digest":"sha256:platform"}]}`))
	if err != nil || digest != "sha256:index" {
		t.Fatalf("top-level digest: got %q, %v", digest, err)
	}
	if _, err := ParseManifestDigest([]byte(`[{"Descriptor":{"digest":"sha256:platform"}}]`)); err == nil {
		t.Fatal("platform array must not be compared with a local index digest")
	}
}

func TestScanUsesVersionTagsCachesAndLimitsConcurrency(t *testing.T) {
	var calls, active, maximum atomic.Int32
	executor := CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		calls.Add(1)
		value := active.Add(1)
		for {
			current := maximum.Load()
			if value <= current || maximum.CompareAndSwap(current, value) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
		return []byte(`{"Descriptor":{"digest":"sha256:remote"}}`), nil
	})
	service := NewImageUpdateService(executor, UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	images := []domain.Image{{ID: "sha256:a", Tags: []string{"nginx:1.27"}, RepoDigests: []string{"docker.io/library/nginx@sha256:local"}}, {ID: "sha256:b", Tags: []string{"redis:7.4"}, RepoDigests: []string{"docker.io/library/redis@sha256:remote"}}}
	snapshot := domain.Snapshot{Containers: []domain.Container{{State: "running", Image: "nginx:1.27", ImageID: "sha256:a"}, {State: "running", Image: "redis:7.4", ImageID: "sha256:b"}}}
	results := service.Scan(context.Background(), snapshot, images)
	if len(results) != 2 || calls.Load() != 2 || maximum.Load() != 1 {
		t.Fatalf("unexpected scan results=%#v calls=%d max=%d", results, calls.Load(), maximum.Load())
	}
	service.Scan(context.Background(), snapshot, images)
	if calls.Load() != 2 {
		t.Fatalf("cached scan called executor %d times", calls.Load())
	}
}

func TestScanMarksDigestReferencePinned(t *testing.T) {
	service := NewImageUpdateService(CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("pinned reference queried")
		return nil, nil
	}), UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	results := service.Scan(context.Background(), domain.Snapshot{Containers: []domain.Container{{State: "running", Image: "nginx@sha256:abc", ImageID: "sha256:a"}}}, []domain.Image{{ID: "sha256:a"}})
	if len(results) != 1 || results[0].Status != domain.UpdatePinned {
		t.Fatalf("got %#v", results)
	}
}

func TestScanComparesSharedReferenceAgainstEachImage(t *testing.T) {
	service := NewImageUpdateService(CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"Descriptor":{"digest":"sha256:remote"}}`), nil
	}), UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	images := []domain.Image{{ID: "sha256:old", RepoDigests: []string{"app@sha256:old"}}, {ID: "sha256:current", RepoDigests: []string{"app@sha256:remote"}}}
	snapshot := domain.Snapshot{Containers: []domain.Container{{State: "running", Image: "app:latest", ImageID: "sha256:old"}, {State: "running", Image: "app:latest", ImageID: "sha256:current"}}}

	results := service.Scan(context.Background(), snapshot, images)
	statuses := make(map[string]domain.UpdateStatus, len(results))
	for _, result := range results {
		statuses[result.ImageID] = result.Status
	}
	if statuses["old"] != domain.UpdateAvailable || statuses["current"] != domain.UpdateCurrent {
		t.Fatalf("shared reference statuses: %#v", statuses)
	}
}

func TestInvalidateForcesManifestRefresh(t *testing.T) {
	var calls atomic.Int32
	service := NewImageUpdateService(CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		calls.Add(1)
		return []byte(`{"Descriptor":{"digest":"sha256:remote"}}`), nil
	}), UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	images := []domain.Image{{ID: "sha256:image", RepoDigests: []string{"app@sha256:local"}}}
	snapshot := domain.Snapshot{Containers: []domain.Container{{State: "running", Image: "app:latest", ImageID: "sha256:image"}}}

	service.Scan(context.Background(), snapshot, images)
	service.Scan(context.Background(), snapshot, images)
	service.Invalidate("app:latest")
	service.Scan(context.Background(), snapshot, images)
	if got := calls.Load(); got != 2 {
		t.Fatalf("manifest calls = %d, want 2", got)
	}
}

func TestScanDetectsPulledImagePendingRecreate(t *testing.T) {
	service := NewImageUpdateService(CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"Descriptor":{"digest":"sha256:new"}}`), nil
	}), UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	images := []domain.Image{
		{ID: "sha256:old", Tags: []string{"registry.example/app:old"}, RepoDigests: []string{"registry.example/app@sha256:old"}},
		{ID: "sha256:new", Tags: []string{"registry.example/app:latest"}, RepoDigests: []string{"registry.example/app@sha256:new"}},
	}
	snapshot := domain.Snapshot{Containers: []domain.Container{{ID: "container", State: "running", Image: "registry.example/app:latest", ImageID: "sha256:old"}}}

	results := service.Scan(context.Background(), snapshot, images)
	if len(results) != 1 || results[0].Status != domain.UpdatePulledPendingRecreate {
		t.Fatalf("pending recreate scan result: %#v", results)
	}
}

func TestScanClassifiesDockerHubRateLimitAsLoginRequired(t *testing.T) {
	service := NewImageUpdateService(CommandExecutor(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("toomanyrequests: unauthenticated pull rate limit"), errors.New("exit status 1")
	}), UpdateOptions{Enabled: true, Interval: time.Minute, Concurrency: 1})
	images := []domain.Image{{ID: "sha256:image", RepoDigests: []string{"busybox@sha256:local"}}}
	snapshot := domain.Snapshot{Containers: []domain.Container{{State: "running", Image: "busybox:latest", ImageID: "sha256:image"}}}

	results := service.Scan(context.Background(), snapshot, images)
	if len(results) != 1 || results[0].Reason != DockerHubLoginRequiredReason {
		t.Fatalf("rate-limit result: %#v", results)
	}
}
