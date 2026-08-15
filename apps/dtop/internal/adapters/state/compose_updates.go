package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

const composeUpdatesVersion = 1

type ComposeUpdates struct {
	mu             sync.RWMutex
	path           string
	projects       map[string]domain.ComposeUpdateProject
	disabled       error
	mutationUnlock func()
}

type composeUpdatesFile struct {
	Version  int                             `json:"version"`
	Projects map[string]composeUpdateProject `json:"projects"`
}

type composeUpdateProject struct {
	Name                    string                          `json:"name"`
	RegistrationFingerprint string                          `json:"registration_fingerprint"`
	ConfigFingerprint       string                          `json:"config_fingerprint"`
	Services                map[string]composeUpdateService `json:"services"`
}

type composeUpdateService struct {
	Reference         string `json:"reference"`
	DownloadedDigest  string `json:"downloaded_digest"`
	DownloadedImageID string `json:"downloaded_image_id"`
	AppliedDigest     string `json:"applied_digest"`
	AppliedImageID    string `json:"applied_image_id"`
	PendingUnknown    bool   `json:"pending_unknown"`
}

var _ ports.ComposeUpdateStore = (*ComposeUpdates)(nil)

func DefaultComposeUpdatesPath() string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "dtop", "compose-updates.json")
}

func NewComposeUpdates(path string) (*ComposeUpdates, error) {
	store := &ComposeUpdates{path: path, projects: make(map[string]domain.ComposeUpdateProject)}
	if path == "" {
		store.disabled = errors.New("compose updates path is empty")
		return store, store.disabled
	}

	if err := store.reloadFromDisk(); err != nil {
		store.disable(err)
		return store, store.disabled
	}
	return store, nil
}

func (s *ComposeUpdates) Get(ctx context.Context, project string) (domain.ComposeUpdateProject, bool) {
	if s == nil {
		return domain.ComposeUpdateProject{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return domain.ComposeUpdateProject{}, false
	}
	release, err := s.lockForOperation(ctx)
	if err != nil {
		return domain.ComposeUpdateProject{}, false
	}
	defer release()
	if err := s.reloadFromDisk(); err != nil {
		s.disable(err)
		return domain.ComposeUpdateProject{}, false
	}
	value, ok := s.projects[project]
	if !ok {
		return domain.ComposeUpdateProject{}, false
	}
	return cloneProject(value), true
}

func (s *ComposeUpdates) Put(ctx context.Context, project domain.ComposeUpdateProject) error {
	if s == nil {
		return errors.New("compose updates store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}
	release, err := s.lockForOperation(ctx)
	if err != nil {
		return fmt.Errorf("lock compose updates: %w", err)
	}
	defer release()
	if err := s.reloadFromDisk(); err != nil {
		s.disable(err)
		return fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}
	if s.disabled != nil {
		return fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}
	if err := validateProject(project.Name, project); err != nil {
		return err
	}

	projects := cloneProjects(s.projects)
	next := cloneProject(project)
	projects[project.Name] = next
	data, err := json.MarshalIndent(fileFromProjects(projects), "", "  ")
	if err != nil {
		return fmt.Errorf("encode compose updates: %w", err)
	}
	data = append(data, '\n')
	if err := replaceFile(s.path, data); err != nil {
		return err
	}
	s.projects = projects
	return nil
}

func (s *ComposeUpdates) Health(ctx context.Context) error {
	if s == nil {
		return errors.New("compose updates store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return s.disabled
	}
	release, err := s.lockForOperation(ctx)
	if err != nil {
		return fmt.Errorf("lock compose updates: %w", err)
	}
	defer release()
	if err := s.reloadFromDisk(); err != nil {
		s.disable(err)
	}
	return s.disabled
}

func (s *ComposeUpdates) BeginMutation(ctx context.Context) (func(), error) {
	if s == nil {
		return nil, errors.New("compose updates store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil, fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}
	if s.mutationUnlock != nil {
		return nil, errors.New("compose updates mutation is already in progress")
	}

	release, err := acquireInterprocessLock(ctx, s.path)
	if err != nil {
		return nil, fmt.Errorf("lock compose updates: %w", err)
	}
	if err := s.reloadFromDisk(); err != nil {
		release()
		s.disable(err)
		return nil, fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}
	if s.disabled != nil {
		release()
		return nil, fmt.Errorf("compose updates store is disabled: %w", s.disabled)
	}

	s.mutationUnlock = release
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			release := s.mutationUnlock
			s.mutationUnlock = nil
			if release != nil {
				release()
			}
			s.mu.Unlock()
		})
	}, nil
}

// lockForOperation is called with s.mu held, so mutation release cannot race it.
func (s *ComposeUpdates) lockForOperation(ctx context.Context) (func(), error) {
	if s.mutationUnlock != nil {
		return func() {}, nil
	}
	return acquireInterprocessLock(ctx, s.path)
}

func (s *ComposeUpdates) reloadFromDisk() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.projects = make(map[string]domain.ComposeUpdateProject)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read compose updates: %w", err)
	}

	var file composeUpdatesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("decode compose updates: %w", err)
	}
	if file.Version != composeUpdatesVersion {
		return fmt.Errorf("unsupported compose updates version %d", file.Version)
	}

	projects := projectsFromFile(file.Projects)
	if err := validateProjects(projects); err != nil {
		return fmt.Errorf("validate compose updates: %w", err)
	}
	s.projects = projects
	return nil
}

func (s *ComposeUpdates) disable(err error) {
	if s.disabled == nil {
		s.disabled = err
	}
}

func validateProjects(projects map[string]domain.ComposeUpdateProject) error {
	for name, project := range projects {
		if err := validateProject(name, project); err != nil {
			return err
		}
	}
	return nil
}

func validateProject(key string, project domain.ComposeUpdateProject) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(project.Name) == "" {
		return errors.New("compose update project name must not be empty")
	}
	if key != project.Name {
		return fmt.Errorf("compose update project key %q does not match name %q", key, project.Name)
	}
	for name, service := range project.Services {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("compose update project %q has an empty service name", project.Name)
		}
		if strings.TrimSpace(service.Reference) == "" {
			return fmt.Errorf("compose update service %q in project %q has an empty reference", name, project.Name)
		}
		if !validDigest(service.DownloadedDigest) {
			return fmt.Errorf("compose update service %q in project %q has an invalid downloaded digest", name, project.Name)
		}
		if !validDigest(service.AppliedDigest) {
			return fmt.Errorf("compose update service %q in project %q has an invalid applied digest", name, project.Name)
		}
	}
	return nil
}

func validDigest(digest string) bool {
	if digest == "" {
		return true
	}
	algorithm, value, found := strings.Cut(digest, ":")
	if !found || algorithm == "" || value == "" || strings.Contains(value, ":") {
		return false
	}
	for index, char := range algorithm {
		letter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		if letter || index > 0 && (char >= '0' && char <= '9' || strings.ContainsRune("+._-", char)) {
			continue
		}
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("=_-", char) {
			continue
		}
		return false
	}
	return true
}

func cloneProjects(projects map[string]domain.ComposeUpdateProject) map[string]domain.ComposeUpdateProject {
	copy := make(map[string]domain.ComposeUpdateProject, len(projects))
	for name, project := range projects {
		copy[name] = cloneProject(project)
	}
	return copy
}

func cloneProject(project domain.ComposeUpdateProject) domain.ComposeUpdateProject {
	copy := project
	copy.Services = make(map[string]domain.ComposeUpdateService, len(project.Services))
	for name, service := range project.Services {
		copy.Services[name] = service
	}
	return copy
}

func projectsFromFile(projects map[string]composeUpdateProject) map[string]domain.ComposeUpdateProject {
	result := make(map[string]domain.ComposeUpdateProject, len(projects))
	for name, project := range projects {
		services := make(map[string]domain.ComposeUpdateService, len(project.Services))
		for serviceName, service := range project.Services {
			services[serviceName] = domain.ComposeUpdateService{
				Reference:         service.Reference,
				DownloadedDigest:  service.DownloadedDigest,
				DownloadedImageID: service.DownloadedImageID,
				AppliedDigest:     service.AppliedDigest,
				AppliedImageID:    service.AppliedImageID,
				PendingUnknown:    service.PendingUnknown,
			}
		}
		result[name] = domain.ComposeUpdateProject{
			Name:                    project.Name,
			RegistrationFingerprint: project.RegistrationFingerprint,
			ConfigFingerprint:       project.ConfigFingerprint,
			Services:                services,
		}
	}
	return result
}

func fileFromProjects(projects map[string]domain.ComposeUpdateProject) composeUpdatesFile {
	file := composeUpdatesFile{Version: composeUpdatesVersion, Projects: make(map[string]composeUpdateProject, len(projects))}
	for name, project := range projects {
		services := make(map[string]composeUpdateService, len(project.Services))
		for serviceName, service := range project.Services {
			services[serviceName] = composeUpdateService{
				Reference:         service.Reference,
				DownloadedDigest:  service.DownloadedDigest,
				DownloadedImageID: service.DownloadedImageID,
				AppliedDigest:     service.AppliedDigest,
				AppliedImageID:    service.AppliedImageID,
				PendingUnknown:    service.PendingUnknown,
			}
		}
		file.Projects[name] = composeUpdateProject{
			Name:                    project.Name,
			RegistrationFingerprint: project.RegistrationFingerprint,
			ConfigFingerprint:       project.ConfigFingerprint,
			Services:                services,
		}
	}
	return file
}

func replaceFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create compose updates directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".compose-updates-*")
	if err != nil {
		return fmt.Errorf("create compose updates temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set compose updates temporary file permissions: %w", err)
	}
	if written, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write compose updates temporary file: %w", err)
	} else if written != len(data) {
		return errors.New("write compose updates temporary file: short write")
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync compose updates temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close compose updates temporary file: %w", err)
	}
	closed = true
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace compose updates file: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync compose updates directory: %w", err)
	}
	return nil
}

func acquireInterprocessLock(ctx context.Context, path string) (func(), error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create compose updates directory: %w", err)
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open compose updates lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("set compose updates lock permissions: %w", err)
	}
	unlock, err := lockFile(ctx, file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = unlock()
		_ = file.Close()
	}, nil
}
