package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

type ManifestExecutor interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type CommandExecutor func(context.Context, string, ...string) ([]byte, error)

func (f CommandExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

type UpdateOptions struct {
	Enabled     bool
	Interval    time.Duration
	Concurrency int
}

type ImageUpdateService struct {
	executor ManifestExecutor
	options  UpdateOptions
	mu       sync.Mutex
	cache    map[string]cachedUpdate
}

type cachedUpdate struct {
	digest string
	err    error
	at     time.Time
}

const DockerHubLoginRequiredReason = "docker hub login required"

var errDockerHubLoginRequired = errors.New(DockerHubLoginRequiredReason)

func NewImageUpdateService(executor ManifestExecutor, options UpdateOptions) *ImageUpdateService {
	if options.Interval < time.Minute {
		options.Interval = 15 * time.Minute
	}
	if options.Concurrency < 1 || options.Concurrency > 16 {
		options.Concurrency = 4
	}
	return &ImageUpdateService{executor: executor, options: options, cache: make(map[string]cachedUpdate)}
}

func (s *ImageUpdateService) Interval() time.Duration {
	if s == nil {
		return 15 * time.Minute
	}
	return s.options.Interval
}
func (s *ImageUpdateService) Enabled() bool { return s != nil && s.options.Enabled }

func (s *ImageUpdateService) Invalidate(references ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, reference := range references {
		if normalized, ok := NormalizeImageReference(reference); ok {
			delete(s.cache, normalized)
		}
	}
}

// NormalizeImageReference creates an explicit Docker Hub name while preserving tags and digests.
func NormalizeImageReference(reference string) (string, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" || reference == "<none>:<none>" || strings.HasPrefix(reference, "sha256:") {
		return "", false
	}
	name := reference
	if at := strings.IndexByte(name, '@'); at >= 0 {
		if at == 0 || !strings.HasPrefix(name[at+1:], "sha256:") {
			return "", false
		}
		name = name[:at] + "@" + strings.ToLower(name[at+1:])
	}
	path := name
	if at := strings.IndexByte(path, '@'); at >= 0 {
		path = path[:at]
	}
	first := strings.Split(path, "/")[0]
	host := strings.Split(first, ":")[0]
	hasRegistry := strings.Contains(host, ".") || host == "localhost" || (strings.Contains(first, ":") && strings.Contains(path, "/"))
	if !hasRegistry {
		if !strings.Contains(path, "/") {
			name = "docker.io/library/" + name
		} else {
			name = "docker.io/" + name
		}
	}
	return name, true
}

func IsPinnedReference(reference string) bool { return strings.Contains(reference, "@sha256:") }

func (s *ImageUpdateService) Scan(ctx context.Context, snapshot domain.Snapshot, images []domain.Image) []domain.ImageUpdate {
	if s == nil || !s.options.Enabled {
		return nil
	}
	byID := make(map[string]domain.Image, len(images))
	for _, image := range images {
		byID[trimImageID(image.ID)] = image
	}
	type scanTarget struct {
		containerID string
		imageID     string
	}
	refs := make(map[string][]scanTarget)
	for _, container := range snapshot.Containers {
		if container.State != "running" {
			continue
		}
		reference, ok := NormalizeImageReference(container.Image)
		if !ok {
			continue
		}
		id := trimImageID(container.ImageID)
		if id == "" {
			id = imageIDForReference(reference, images)
		}
		if id != "" {
			refs[reference] = append(refs[reference], scanTarget{containerID: container.ID, imageID: id})
		}
	}
	results := make(map[string]domain.ImageUpdate)
	var mu sync.Mutex
	jobs := make(chan string)
	var wait sync.WaitGroup
	for range s.options.Concurrency {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for ref := range jobs {
				targets := refs[ref]
				taggedID := imageIDForReference(ref, images)
				updates := make([]domain.ImageUpdate, 0, len(targets))
				for _, target := range targets {
					status, reason := s.check(ctx, ref, byID[target.imageID])
					if taggedID != "" && taggedID != target.imageID {
						status, reason = s.check(ctx, ref, byID[taggedID])
						if status == domain.UpdateCurrent {
							status, reason = domain.UpdatePulledPendingRecreate, "pulled image differs from running image"
						}
					}
					updates = append(updates, domain.ImageUpdate{ContainerID: target.containerID, ImageID: target.imageID, Reference: ref, Status: status, Reason: reason})
				}
				mu.Lock()
				for _, update := range updates {
					key := update.ContainerID
					if key == "" {
						key = update.Reference + "|" + update.ImageID
					}
					results[key] = update
				}
				mu.Unlock()
			}
		}()
	}
enqueue:
	for reference := range refs {
		select {
		case jobs <- reference:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(jobs)
	wait.Wait()
	output := make([]domain.ImageUpdate, 0, len(results))
	for _, result := range results {
		output = append(output, result)
	}
	return output
}

func (s *ImageUpdateService) check(ctx context.Context, reference string, image domain.Image) (domain.UpdateStatus, string) {
	if IsPinnedReference(reference) {
		return domain.UpdatePinned, "digest reference"
	}
	if len(image.RepoDigests) == 0 {
		return domain.UpdateUnknown, "local digest unavailable"
	}
	digest, err := s.remoteDigest(ctx, reference)
	if err != nil {
		if errors.Is(err, errDockerHubLoginRequired) {
			return domain.UpdateUnknown, DockerHubLoginRequiredReason
		}
		return domain.UpdateUnknown, "manifest unavailable"
	}
	for _, local := range image.RepoDigests {
		if digestOf(local) == digest {
			return domain.UpdateCurrent, ""
		}
	}
	return domain.UpdateAvailable, ""
}

func (s *ImageUpdateService) remoteDigest(ctx context.Context, reference string) (string, error) {
	s.mu.Lock()
	cached, ok := s.cache[reference]
	s.mu.Unlock()
	if ok && time.Since(cached.at) < s.options.Interval {
		return cached.digest, cached.err
	}
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := s.executor.Run(queryCtx, "docker", "buildx", "imagetools", "inspect", reference, "--format", "{{json .Manifest}}")
	if err != nil && queryCtx.Err() == nil {
		output, err = s.executor.Run(queryCtx, "docker", "manifest", "inspect", "--verbose", reference)
	}
	digest := ""
	if err == nil {
		digest, err = ParseManifestDigest(output)
	}
	if err != nil {
		message := strings.ToLower(string(output))
		if strings.HasPrefix(reference, "docker.io/") && (strings.Contains(message, "toomanyrequests") || strings.Contains(message, "rate limit") || strings.Contains(message, "unauthorized") || strings.Contains(message, "authentication required")) {
			err = errDockerHubLoginRequired
		} else {
			err = errors.New("manifest unavailable")
		}
	}
	if ctx.Err() == nil {
		s.mu.Lock()
		s.cache[reference] = cachedUpdate{digest: digest, err: err, at: time.Now()}
		s.mu.Unlock()
	}
	return digest, err
}

func ParseManifestDigest(data []byte) (string, error) {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return "", fmt.Errorf("decode manifest: %w", err)
	}
	node, ok := document.(map[string]any)
	if !ok {
		return "", errors.New("top-level manifest digest unavailable")
	}
	if digest, ok := node["digest"].(string); ok && strings.HasPrefix(strings.ToLower(digest), "sha256:") {
		return strings.ToLower(digest), nil
	}
	if descriptor, ok := node["Descriptor"].(map[string]any); ok {
		if digest, ok := descriptor["digest"].(string); ok && strings.HasPrefix(strings.ToLower(digest), "sha256:") {
			return strings.ToLower(digest), nil
		}
	}
	return "", errors.New("manifest digest missing")
}

func trimImageID(id string) string { return strings.TrimPrefix(strings.ToLower(id), "sha256:") }
func digestOf(reference string) string {
	if at := strings.LastIndexByte(reference, '@'); at >= 0 {
		return strings.ToLower(reference[at+1:])
	}
	return ""
}
func imageIDForReference(reference string, images []domain.Image) string {
	for _, image := range images {
		for _, tag := range image.Tags {
			if normalized, ok := NormalizeImageReference(tag); ok && normalized == reference {
				return trimImageID(image.ID)
			}
		}
	}
	return ""
}
