package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDockerHubLoginConfiguredAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write := func(contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"auths":{"https://index.docker.io/v1/":{"auth":"configured"}}}`)
	if !DockerHubLoginConfiguredAt(context.Background(), path, nil) {
		t.Fatal("direct Docker Hub auth was not detected")
	}

	write(`{"credsStore":"desktop"}`)
	if !DockerHubLoginConfiguredAt(context.Background(), path, func(context.Context, string) ([]byte, error) {
		return []byte(`{"https://index.docker.io/v1/":"user"}`), nil
	}) {
		t.Fatal("Docker Hub credential helper was not detected")
	}

	write(`{"auths":{"ghcr.io":{"auth":"configured"}}}`)
	if DockerHubLoginConfiguredAt(context.Background(), path, nil) {
		t.Fatal("non-Docker Hub auth must not hide the login guidance")
	}

	write(`{"auths":{"https://index.docker.io/v1/":{}}}`)
	if DockerHubLoginConfiguredAt(context.Background(), path, nil) {
		t.Fatal("empty Docker Hub auth must not be accepted")
	}

	write(`{"credsStore":"desktop"}`)
	if DockerHubLoginConfiguredAt(context.Background(), path, func(context.Context, string) ([]byte, error) {
		return nil, errors.New("helper unavailable")
	}) {
		t.Fatal("unavailable credential helper must not be accepted")
	}
}
