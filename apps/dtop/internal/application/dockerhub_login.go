package application

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CredentialHelperLister func(context.Context, string) ([]byte, error)

type dockerConfig struct {
	Auths       map[string]json.RawMessage `json:"auths"`
	CredHelpers map[string]string          `json:"credHelpers"`
	CredsStore  string                     `json:"credsStore"`
}

// DockerHubLoginConfigured reports whether Docker CLI has a configured Docker
// Hub credential without reading or exposing the credential itself.
func DockerHubLoginConfigured(ctx context.Context) bool {
	configDir := os.Getenv("DOCKER_CONFIG")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		configDir = filepath.Join(home, ".docker")
	}
	return DockerHubLoginConfiguredAt(ctx, filepath.Join(configDir, "config.json"), listCredentialHelper)
}

func DockerHubLoginConfiguredAt(ctx context.Context, path string, list CredentialHelperLister) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config dockerConfig
	if json.Unmarshal(contents, &config) != nil {
		return false
	}
	for registry, auth := range config.Auths {
		if dockerHubRegistry(registry) && dockerAuthConfigured(auth) {
			return true
		}
	}
	for registry, helper := range config.CredHelpers {
		if dockerHubRegistry(registry) && helperHasDockerHub(ctx, helper, list) {
			return true
		}
	}
	return config.CredsStore != "" && helperHasDockerHub(ctx, config.CredsStore, list)
}

func dockerAuthConfigured(auth json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(auth, &fields) != nil {
		return false
	}
	for _, key := range []string{"auth", "identitytoken"} {
		value, found := fields[key]
		if found && string(value) != `""` && string(value) != "null" {
			return true
		}
	}
	return false
}

func helperHasDockerHub(ctx context.Context, helper string, list CredentialHelperLister) bool {
	if helper == "" || list == nil {
		return false
	}
	contents, err := list(ctx, helper)
	if err != nil {
		return false
	}
	var credentials map[string]string
	if json.Unmarshal(contents, &credentials) != nil {
		return false
	}
	for registry := range credentials {
		if dockerHubRegistry(registry) {
			return true
		}
	}
	return false
}

func listCredentialHelper(ctx context.Context, helper string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker-credential-"+helper, "list").Output()
}

func dockerHubRegistry(registry string) bool {
	registry = strings.Trim(strings.ToLower(registry), "/")
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimSuffix(registry, "/v1")
	registry = strings.TrimSuffix(registry, "/v2")
	return registry == "docker.io" || registry == "index.docker.io" || registry == "registry-1.docker.io"
}
