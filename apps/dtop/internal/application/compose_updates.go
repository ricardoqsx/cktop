package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

type composeUpdateCoordinator struct {
	mu        sync.RWMutex
	store     ports.ComposeUpdateStore
	overrides map[string]domain.ComposeUpdateProject
	cache     map[string]composeUpdateCache
	revision  uint64
}

type composeUpdateCache struct {
	project  domain.ComposeUpdateProject
	eligible bool
	reason   string
}

type resolvedComposeProject struct {
	project  domain.ComposeUpdateProject
	services []string
}

type localComposeImage struct {
	digest  string
	imageID string
}

type configuredComposeService struct {
	reference  string
	pullable   bool
	pullPolicy string
	build      bool
}

func newComposeUpdateCoordinator(store ports.ComposeUpdateStore) *composeUpdateCoordinator {
	return &composeUpdateCoordinator{
		store:     store,
		overrides: make(map[string]domain.ComposeUpdateProject),
		cache:     make(map[string]composeUpdateCache),
	}
}

func (state *composeUpdateCoordinator) get(project string) (domain.ComposeUpdateProject, bool) {
	if state == nil {
		return domain.ComposeUpdateProject{}, false
	}
	state.mu.RLock()
	value, found := state.overrides[project]
	state.mu.RUnlock()
	if found {
		return cloneComposeUpdateProject(value), true
	}
	if state.store == nil {
		return domain.ComposeUpdateProject{}, false
	}
	return state.store.Get(project)
}

func (state *composeUpdateCoordinator) put(project domain.ComposeUpdateProject) error {
	if state == nil {
		return errors.New("Compose update state is unavailable")
	}
	if state.store == nil {
		state.mu.Lock()
		state.overrides[project.Name] = cloneComposeUpdateProject(project)
		state.revision++
		state.mu.Unlock()
		return nil
	}
	err := state.store.Put(project)
	state.mu.Lock()
	if err != nil {
		state.overrides[project.Name] = cloneComposeUpdateProject(project)
	} else {
		delete(state.overrides, project.Name)
	}
	state.revision++
	state.mu.Unlock()
	return err
}

func (state *composeUpdateCoordinator) health() error {
	if state == nil || state.store == nil {
		return nil
	}
	return state.store.Health()
}

func (state *composeUpdateCoordinator) beginMutation() (func(), error) {
	if state == nil || state.store == nil {
		return func() {}, nil
	}
	release, err := state.store.BeginMutation()
	if err != nil {
		return nil, err
	}
	state.mu.Lock()
	state.overrides = make(map[string]domain.ComposeUpdateProject)
	state.revision++
	state.mu.Unlock()
	return release, nil
}

func (state *composeUpdateCoordinator) currentRevision() uint64 {
	if state == nil {
		return 0
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.revision
}

func (state *composeUpdateCoordinator) setCache(project string, cache composeUpdateCache) {
	if state == nil {
		return
	}
	state.mu.Lock()
	state.cache[project] = composeUpdateCache{project: cloneComposeUpdateProject(cache.project), eligible: cache.eligible, reason: cache.reason}
	state.mu.Unlock()
}

func (state *composeUpdateCoordinator) setCacheAtRevision(project string, cache composeUpdateCache, revision uint64) {
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.revision != revision {
		return
	}
	state.cache[project] = composeUpdateCache{project: cloneComposeUpdateProject(cache.project), eligible: cache.eligible, reason: cache.reason}
}

func (state *composeUpdateCoordinator) cached(project string) (composeUpdateCache, bool) {
	if state == nil {
		return composeUpdateCache{}, false
	}
	state.mu.RLock()
	cache, found := state.cache[project]
	state.mu.RUnlock()
	cache.project = cloneComposeUpdateProject(cache.project)
	return cache, found
}

func cloneComposeUpdateProject(project domain.ComposeUpdateProject) domain.ComposeUpdateProject {
	copy := project
	copy.Services = make(map[string]domain.ComposeUpdateService, len(project.Services))
	for name, service := range project.Services {
		copy.Services[name] = service
	}
	return copy
}

func registrationFingerprint(stack domain.Stack) string {
	parts := []string{stack.Name, filepath.Clean(stack.WorkingDir)}
	for _, file := range stack.Files {
		parts = append(parts, filepath.Clean(file))
	}
	return fingerprint(parts...)
}

func fingerprint(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil))
}

func canonicalImageReference(reference string) (string, bool) {
	normalized, ok := NormalizeImageReference(reference)
	if !ok || strings.Contains(normalized, "@") {
		return normalized, ok
	}
	lastSlash := strings.LastIndexByte(normalized, '/')
	lastColon := strings.LastIndexByte(normalized, ':')
	if lastColon <= lastSlash {
		normalized += ":latest"
	}
	return normalized, true
}

func imageRepository(reference string) string {
	if at := strings.IndexByte(reference, '@'); at >= 0 {
		return reference[:at]
	}
	lastSlash := strings.LastIndexByte(reference, '/')
	if lastColon := strings.LastIndexByte(reference, ':'); lastColon > lastSlash {
		return reference[:lastColon]
	}
	return reference
}

func normalizeComposeConfig(config []ports.ComposeServiceImage) (map[string]configuredComposeService, string, error) {
	services := make(map[string]configuredComposeService, len(config))
	names := make([]string, 0, len(config))
	for _, item := range config {
		name := strings.TrimSpace(item.Service)
		if name == "" {
			return nil, "", errors.New("Compose config contains an empty service name")
		}
		if _, found := services[name]; found {
			return nil, "", fmt.Errorf("Compose config contains duplicate service %q", name)
		}
		reference := strings.TrimSpace(item.Reference)
		if reference != "" {
			var ok bool
			reference, ok = canonicalImageReference(reference)
			if !ok {
				return nil, "", fmt.Errorf("Compose service %q has an unsupported image reference", name)
			}
		}
		pullPolicy := strings.ToLower(strings.TrimSpace(item.PullPolicy))
		pullablePolicy := pullPolicy == "" || pullPolicy == "always"
		services[name] = configuredComposeService{reference: reference, pullable: reference != "" && pullablePolicy && (!item.Build || pullPolicy == "always"), pullPolicy: pullPolicy, build: item.Build}
		names = append(names, name)
	}
	sort.Strings(names)
	fingerprintParts := make([]string, 0, len(names)*2)
	for _, name := range names {
		service := services[name]
		fingerprintParts = append(fingerprintParts, name, service.reference, fmt.Sprint(service.build), service.pullPolicy)
	}
	return services, fingerprint(fingerprintParts...), nil
}

func (s ContainerService) resolveComposeProject(ctx context.Context, stack domain.Stack, requested []string, expected map[string]string) (resolvedComposeProject, error) {
	if !stack.Registered {
		return resolvedComposeProject{}, fmt.Errorf("Compose project %q is not registered", stack.Name)
	}
	config, err := s.runtime.ComposeConfig(ctx, stack)
	if err != nil {
		return resolvedComposeProject{}, err
	}
	configured, configFingerprint, err := normalizeComposeConfig(config)
	if err != nil {
		return resolvedComposeProject{}, err
	}
	services := append([]string(nil), requested...)
	if len(services) == 0 {
		for name, service := range configured {
			if service.pullable && !IsPinnedReference(service.reference) {
				services = append(services, name)
			}
		}
	}
	sort.Strings(services)
	services = compactStrings(services)
	if len(services) == 0 {
		return resolvedComposeProject{}, fmt.Errorf("Compose project %q has no image-based services", stack.Name)
	}
	for _, service := range services {
		configuredService, found := configured[service]
		if !found {
			return resolvedComposeProject{}, fmt.Errorf("Compose service %q no longer exists in project %q", service, stack.Name)
		}
		if configuredService.reference == "" {
			return resolvedComposeProject{}, fmt.Errorf("Compose service %q does not define an image", service)
		}
		if !configuredService.pullable {
			return resolvedComposeProject{}, fmt.Errorf("Compose service %q is build-managed and cannot use registry update pull", service)
		}
		reference := configuredService.reference
		if IsPinnedReference(reference) {
			return resolvedComposeProject{}, fmt.Errorf("Compose service %q uses a pinned image and does not support update pull", service)
		}
		if value := expected[service]; value != "" {
			normalized, ok := canonicalImageReference(value)
			if !ok || normalized != reference {
				return resolvedComposeProject{}, fmt.Errorf("Compose service %q image changed from the selected container", service)
			}
		}
	}
	registration := registrationFingerprint(stack)
	project, found := s.composeUpdates.get(stack.Name)
	if !found || project.RegistrationFingerprint != registration || project.ConfigFingerprint != configFingerprint {
		next := domain.ComposeUpdateProject{
			Name:                    stack.Name,
			RegistrationFingerprint: registration,
			ConfigFingerprint:       configFingerprint,
			Services:                make(map[string]domain.ComposeUpdateService),
		}
		if found {
			for service, value := range project.Services {
				if configured[service].reference == value.Reference {
					next.Services[service] = value
				}
			}
		}
		project = next
	}
	if project.Services == nil {
		project.Services = make(map[string]domain.ComposeUpdateService)
	}
	for _, service := range services {
		value := project.Services[service]
		value.Reference = configured[service].reference
		project.Services[service] = value
	}
	return resolvedComposeProject{project: project, services: services}, nil
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func (s ContainerService) pullComposeProject(ctx context.Context, stack domain.Stack, services []string, expected map[string]string, pull func([]string) error) (resolvedComposeProject, bool, error, error) {
	resolved, err := s.resolveComposeProject(ctx, stack, services, expected)
	if err != nil {
		return resolvedComposeProject{}, false, err, nil
	}
	s.captureAppliedComposeBaseline(ctx, stack, &resolved)
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		value.PendingUnknown = true
		resolved.project.Services[service] = value
	}
	if err := s.saveResolvedComposeProject(resolved); err != nil {
		return resolved, false, fmt.Errorf("prepare Compose update state: %w", err), nil
	}
	if err := pull(resolved.services); err != nil {
		for _, service := range resolved.services {
			value := resolved.project.Services[service]
			value.PendingUnknown = true
			resolved.project.Services[service] = value
		}
		warning := s.saveResolvedComposeProject(resolved)
		return resolved, false, err, warning
	}
	images, err := s.runtime.Images(ctx)
	if err != nil {
		for _, service := range resolved.services {
			value := resolved.project.Services[service]
			value.PendingUnknown = true
			resolved.project.Services[service] = value
		}
		warning := s.saveResolvedComposeProject(resolved)
		return resolved, true, fmt.Errorf("verify pulled Compose images: %w", err), warning
	}
	var verificationErrors []error
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		local, err := localImageForReference(images, value.Reference)
		if err != nil {
			value.PendingUnknown = true
			verificationErrors = append(verificationErrors, fmt.Errorf("service %s: %w", service, err))
		} else {
			value.DownloadedDigest = local.digest
			value.DownloadedImageID = local.imageID
			value.PendingUnknown = false
		}
		resolved.project.Services[service] = value
	}
	warning := s.saveResolvedComposeProject(resolved)
	if len(verificationErrors) > 0 {
		return resolved, true, fmt.Errorf("verify pulled Compose images: %w", errors.Join(verificationErrors...)), warning
	}
	return resolved, true, nil, warning
}

func localImageForReference(images []domain.Image, reference string) (localComposeImage, error) {
	reference, ok := canonicalImageReference(reference)
	if !ok {
		return localComposeImage{}, errors.New("invalid image reference")
	}
	for _, image := range images {
		matched := false
		for _, tag := range image.Tags {
			if normalized, ok := canonicalImageReference(tag); ok && normalized == reference {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		repository := imageRepository(reference)
		digests := append([]string(nil), image.RepoDigests...)
		sort.Strings(digests)
		for _, repoDigest := range digests {
			at := strings.LastIndexByte(repoDigest, '@')
			if at < 0 {
				continue
			}
			repo, ok := canonicalImageReference(repoDigest)
			if !ok || imageRepository(repo) != repository {
				continue
			}
			digest := digestOf(repoDigest)
			if digest != "" {
				return localComposeImage{digest: digest, imageID: strings.ToLower(image.ID)}, nil
			}
		}
		return localComposeImage{}, errors.New("local RepoDigest is unavailable")
	}
	return localComposeImage{}, errors.New("pulled image is unavailable locally")
}

func localImageForID(images []domain.Image, imageID, reference string) (localComposeImage, error) {
	reference, ok := canonicalImageReference(reference)
	if !ok {
		return localComposeImage{}, errors.New("invalid image reference")
	}
	for _, image := range images {
		if trimImageID(image.ID) != trimImageID(imageID) {
			continue
		}
		repository := imageRepository(reference)
		for _, repoDigest := range image.RepoDigests {
			at := strings.LastIndexByte(repoDigest, '@')
			if at < 0 {
				continue
			}
			repo, ok := canonicalImageReference(repoDigest)
			if !ok || imageRepository(repo) != repository {
				continue
			}
			if digest := digestOf(repoDigest); digest != "" {
				return localComposeImage{digest: digest, imageID: strings.ToLower(image.ID)}, nil
			}
		}
		return localComposeImage{}, errors.New("running image RepoDigest is unavailable")
	}
	return localComposeImage{}, errors.New("running image is unavailable locally")
}

func (s ContainerService) captureAppliedComposeBaseline(ctx context.Context, stack domain.Stack, resolved *resolvedComposeProject) {
	images, imagesErr := s.runtime.Images(ctx)
	snapshot, snapshotErr := s.runtime.Snapshot(ctx)
	if imagesErr != nil || snapshotErr != nil {
		return
	}
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		if value.AppliedDigest != "" && value.AppliedImageID != "" {
			continue
		}
		var baseline localComposeImage
		consistent := true
		found := false
		for _, container := range snapshot.Containers {
			if container.ComposeProject != stack.Name || container.ComposeService != service || container.ComposeOneOff {
				continue
			}
			local, err := localImageForID(images, container.ImageID, value.Reference)
			if err != nil {
				consistent = false
				break
			}
			if !found {
				baseline = local
				found = true
				continue
			}
			if local.digest != baseline.digest || trimImageID(local.imageID) != trimImageID(baseline.imageID) {
				consistent = false
				break
			}
		}
		if found && consistent {
			value.AppliedDigest = baseline.digest
			value.AppliedImageID = baseline.imageID
			resolved.project.Services[service] = value
		}
	}
}

func (s ContainerService) saveResolvedComposeProject(resolved resolvedComposeProject) error {
	err := s.composeUpdates.put(resolved.project)
	s.composeUpdates.setCache(resolved.project.Name, composeUpdateCache{project: resolved.project, eligible: true})
	if err != nil {
		return fmt.Errorf("Compose update state was not saved: %w", err)
	}
	return nil
}

func (s ContainerService) applyComposeProject(ctx context.Context, stack domain.Stack, services []string, expected map[string]string, up func() error) (resolvedComposeProject, error, error) {
	if len(services) == 0 {
		if project, found := s.composeUpdates.get(stack.Name); found && project.RegistrationFingerprint == registrationFingerprint(stack) {
			for service, value := range project.Services {
				if value.PendingUnknown || value.DownloadedDigest != "" && value.DownloadedDigest != value.AppliedDigest {
					services = append(services, service)
				}
			}
		}
		if len(services) == 0 {
			return resolvedComposeProject{}, fmt.Errorf("Compose project %q has no verified downloaded update pending", stack.Name), nil
		}
	}
	resolved, err := s.resolveComposeProject(ctx, stack, services, expected)
	if err != nil {
		return resolvedComposeProject{}, err, nil
	}
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		if value.PendingUnknown || value.DownloadedDigest == "" || value.DownloadedImageID == "" {
			return resolvedComposeProject{}, fmt.Errorf("Compose service %q has no verified downloaded update", service), nil
		}
	}
	images, err := s.runtime.Images(ctx)
	if err != nil {
		return resolvedComposeProject{}, fmt.Errorf("verify downloaded Compose images: %w", err), nil
	}
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		local, err := localImageForReference(images, value.Reference)
		if err != nil || local.digest != value.DownloadedDigest || trimImageID(local.imageID) != trimImageID(value.DownloadedImageID) {
			return resolvedComposeProject{}, fmt.Errorf("downloaded image for Compose service %q changed; pull it again", service), nil
		}
	}
	if err := up(); err != nil {
		return resolved, err, nil
	}
	if err := s.verifyAppliedComposeProject(ctx, stack, resolved); err != nil {
		return resolved, err, nil
	}
	for _, service := range resolved.services {
		value := resolved.project.Services[service]
		value.AppliedDigest = value.DownloadedDigest
		value.AppliedImageID = value.DownloadedImageID
		value.PendingUnknown = false
		resolved.project.Services[service] = value
	}
	warning := s.saveResolvedComposeProject(resolved)
	return resolved, nil, warning
}

func (s ContainerService) verifyAppliedComposeProject(ctx context.Context, stack domain.Stack, resolved resolvedComposeProject) error {
	snapshot, err := s.runtime.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("verify applied Compose update: %w", err)
	}
	for _, service := range resolved.services {
		expected := resolved.project.Services[service]
		found := false
		for _, container := range snapshot.Containers {
			if container.ComposeProject != stack.Name || container.ComposeService != service || container.ComposeOneOff {
				continue
			}
			found = true
			if trimImageID(container.ImageID) != trimImageID(expected.DownloadedImageID) {
				return fmt.Errorf("Compose service %q did not apply the verified downloaded image", service)
			}
		}
		if !found {
			return fmt.Errorf("Compose service %q has no container to verify", service)
		}
	}
	return nil
}

func (s ContainerService) composeUpBlocked(ctx context.Context, stack domain.Stack) error {
	if err := s.composeUpdates.health(); err != nil {
		return fmt.Errorf("Compose update state is unavailable; refusing Up: %w", err)
	}
	project, found := s.composeUpdates.get(stack.Name)
	if !found || !project.Pending() {
		return nil
	}
	resolved, err := s.resolveComposeProject(ctx, stack, nil, nil)
	if err != nil {
		return fmt.Errorf("verify pending Compose update before Up: %w", err)
	}
	if resolved.project.Pending() {
		return fmt.Errorf("Compose project %q has a downloaded update pending; use Apply downloaded update", stack.Name)
	}
	return nil
}

func (s ContainerService) refreshComposeUpdateState(ctx context.Context, stacks []domain.Stack) {
	for _, stack := range stacks {
		if !stack.Registered {
			continue
		}
		revision := s.composeUpdates.currentRevision()
		if err := s.composeUpdates.health(); err != nil {
			s.composeUpdates.setCacheAtRevision(stack.Name, composeUpdateCache{project: domain.ComposeUpdateProject{Name: stack.Name}, eligible: true, reason: err.Error()}, revision)
			continue
		}
		project, found := s.composeUpdates.get(stack.Name)
		if !found || !project.Pending() {
			s.composeUpdates.setCacheAtRevision(stack.Name, composeUpdateCache{}, revision)
			continue
		}
		resolved, err := s.resolveComposeProject(ctx, stack, nil, nil)
		if err != nil {
			s.composeUpdates.setCacheAtRevision(stack.Name, composeUpdateCache{project: project, eligible: true, reason: err.Error()}, revision)
			continue
		}
		if !resolved.project.Pending() {
			s.composeUpdates.setCacheAtRevision(stack.Name, composeUpdateCache{}, revision)
			continue
		}
		s.composeUpdates.setCacheAtRevision(stack.Name, composeUpdateCache{project: resolved.project, eligible: true}, revision)
	}
}

func (s ContainerService) decorateComposeStacks(stacks []domain.Stack) []domain.Stack {
	result := append([]domain.Stack(nil), stacks...)
	for index := range result {
		stack := &result[index]
		cache, found := s.composeUpdates.cached(stack.Name)
		if !found || !cache.eligible || !cache.project.Pending() && cache.reason == "" {
			continue
		}
		stack.UpdatePending = true
		stack.UpdateUnknown = cache.project.PendingUnknown() || cache.reason != ""
		stack.UpdateReason = cache.reason
		if stack.UpdateUnknown {
			stack.Update = domain.UpdateUnknown
		} else {
			stack.Update = domain.UpdatePulledPendingRecreate
		}
		for containerIndex := range stack.ContainerItems {
			container := &stack.ContainerItems[containerIndex]
			service, ok := cache.project.Services[container.ComposeService]
			if !ok || !service.PendingUnknown && (service.DownloadedDigest == "" || service.DownloadedDigest == service.AppliedDigest) {
				continue
			}
			if service.PendingUnknown {
				container.Update = domain.UpdateUnknown
				container.UpdatePending = true
				container.UpdateUnknown = true
			} else {
				container.Update = domain.UpdatePulledPendingRecreate
				container.UpdatePending = true
			}
		}
	}
	return result
}

func (s ContainerService) DecorateComposeSnapshot(snapshot domain.Snapshot) domain.Snapshot {
	result := snapshot
	result.Containers = append([]domain.Container(nil), snapshot.Containers...)
	for index := range result.Containers {
		container := &result.Containers[index]
		container.UpdatePending = false
		container.UpdateUnknown = false
		cache, found := s.composeUpdates.cached(container.ComposeProject)
		if !found || !cache.eligible {
			continue
		}
		service, found := cache.project.Services[container.ComposeService]
		if !found {
			continue
		}
		if service.PendingUnknown {
			container.Update = domain.UpdateUnknown
			container.UpdatePending = true
			container.UpdateUnknown = true
		} else if service.DownloadedDigest != "" && service.DownloadedDigest != service.AppliedDigest {
			container.Update = domain.UpdatePulledPendingRecreate
			container.UpdatePending = true
		}
	}
	return result
}
