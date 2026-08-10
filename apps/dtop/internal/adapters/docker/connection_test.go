package docker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEndpointUsesDockerHostFirst(t *testing.T) {
	resolved, err := ResolveEndpoint(ResolverOptions{
		Env: map[string]string{
			"DOCKER_HOST":    "unix:///tmp/docker.sock",
			"DOCKER_CONTEXT": "desktop-linux",
		},
		HomeDir: t.TempDir(),
		GOOS:    "linux",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}

	if resolved.host != "unix:///tmp/docker.sock" {
		t.Fatalf("expected DOCKER_HOST endpoint, got %q", resolved.host)
	}
	if resolved.source != "DOCKER_HOST" {
		t.Fatalf("expected DOCKER_HOST source, got %q", resolved.source)
	}
}

func TestResolveEndpointRejectsRemoteBeforeD1R(t *testing.T) {
	_, err := ResolveEndpoint(ResolverOptions{
		Env: map[string]string{
			"DOCKER_HOST": "ssh://docker@host.example.net",
		},
		HomeDir: t.TempDir(),
		GOOS:    "linux",
	})
	if !errors.Is(err, ErrRemoteUnsupported) {
		t.Fatalf("expected remote unsupported error, got %v", err)
	}
}

func TestResolveEndpointUsesCurrentContext(t *testing.T) {
	homeDir := t.TempDir()
	writeFile(t, filepath.Join(homeDir, ".docker", "config.json"), `{"currentContext":"desktop-linux"}`)
	writeFile(t, filepath.Join(homeDir, ".docker", "contexts", "meta", "abc", "meta.json"), `{
  "Name": "desktop-linux",
  "Endpoints": {
    "docker": {
      "Host": "unix:///Users/test/.docker/run/docker.sock"
    }
  }
}`)

	resolved, err := ResolveEndpoint(ResolverOptions{
		Env:     map[string]string{},
		HomeDir: homeDir,
		GOOS:    "darwin",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}

	if resolved.name != "desktop-linux" {
		t.Fatalf("expected desktop-linux context, got %q", resolved.name)
	}
	if resolved.host != "unix:///Users/test/.docker/run/docker.sock" {
		t.Fatalf("expected context host, got %q", resolved.host)
	}
}

func TestResolveEndpointFallsBackToDockerDesktopSocket(t *testing.T) {
	homeDir := t.TempDir()
	desktopSocket := filepath.Join(homeDir, ".docker", "run", "docker.sock")
	writeFile(t, desktopSocket, "")

	resolved, err := ResolveEndpoint(ResolverOptions{
		Env:     map[string]string{},
		HomeDir: homeDir,
		GOOS:    "darwin",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}

	if resolved.host != "unix://"+desktopSocket {
		t.Fatalf("expected Docker Desktop socket, got %q", resolved.host)
	}
}

func TestSanitizeEndpointRedactsUser(t *testing.T) {
	got := sanitizeEndpoint("ssh://docker-user@example.net")
	want := "ssh://%3Cuser%3E@example.net"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
