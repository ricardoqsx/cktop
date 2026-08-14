package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/ports"
)

func TestComposeConfigArgsUseExactStackMetadata(t *testing.T) {
	stack := domain.Stack{
		Name:       "app",
		WorkingDir: "/srv/app",
		Files:      []string{"/srv/app/compose.yaml", "/srv/app/compose.prod.yaml"},
	}
	want := []string{
		"compose", "--project-name", "app", "--project-directory", "/srv/app",
		"-f", "/srv/app/compose.yaml", "-f", "/srv/app/compose.prod.yaml",
		"config", "--format", "json",
	}
	if got := composeConfigArgs(stack); !reflect.DeepEqual(got, want) {
		t.Fatalf("composeConfigArgs() = %v, want %v", got, want)
	}
}

func TestParseComposeConfigSortsServicesAndRetainsEmptyImages(t *testing.T) {
	data := []byte(`{
		"name": "app",
		"services": {
			"worker": {"image": "registry.example/worker:v2", "pull_policy": "always", "environment": {"TOKEN": "ignored"}},
			"api": {"image": ""},
			"build-only": {"build": {"context": "."}}
		}
	}`)
	want := []ports.ComposeServiceImage{
		{Service: "api", Reference: ""},
		{Service: "build-only", Reference: "", Build: true},
		{Service: "worker", Reference: "registry.example/worker:v2", PullPolicy: "always"},
	}
	got, err := parseComposeConfig(data)
	if err != nil {
		t.Fatalf("parseComposeConfig() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseComposeConfig() = %#v, want %#v", got, want)
	}
}

func TestParseComposeConfigRejectsMalformedJSON(t *testing.T) {
	if _, err := parseComposeConfig([]byte(`{"services":`)); err == nil {
		t.Fatal("parseComposeConfig() accepted malformed JSON")
	}
}

func TestParseComposeConfigAllowsNoServices(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{}`), []byte(`{"services": {}}`)} {
		got, err := parseComposeConfig(data)
		if err != nil {
			t.Fatalf("parseComposeConfig(%s) error = %v", data, err)
		}
		if len(got) != 0 {
			t.Fatalf("parseComposeConfig(%s) = %#v, want no services", data, got)
		}
	}
}

func TestParseComposeConfigRejectsInvalidServiceNames(t *testing.T) {
	tests := map[string]string{
		"empty":     `{"services":{"":{"image":"one"}}}`,
		"duplicate": `{"services":{"web":{"image":"one"},"web":{"image":"two"}}}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseComposeConfig([]byte(data)); err == nil {
				t.Fatal("parseComposeConfig() accepted an invalid service name")
			}
		})
	}
}

func TestParseComposeConfigErrorsDoNotExposeRawJSON(t *testing.T) {
	const secret = "do-not-expose-this-secret"
	raw := `{"services":{"web":{"image":{"password":"` + secret + `"}}}}`
	_, err := parseComposeConfig([]byte(raw))
	if err == nil {
		t.Fatal("parseComposeConfig() accepted a non-string image")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), raw) {
		t.Fatalf("parser error exposed config JSON: %v", err)
	}
}

func TestBoundedOutputRejectsOversizedData(t *testing.T) {
	output := boundedOutput{limit: 4}
	written, err := output.Write([]byte("secret"))
	if err != nil || written != len("secret") {
		t.Fatalf("Write() = %d, %v", written, err)
	}
	if !output.exceeded || output.String() != "secr" {
		t.Fatalf("bounded output = %q, exceeded = %t", output.String(), output.exceeded)
	}
}

func TestComposeConfigRejectsRemoteEndpoint(t *testing.T) {
	runtime := NewRuntime(ResolverOptions{Spec: ConnectionSpec{Host: "ssh://docker@example.test"}})
	stack := domain.Stack{Name: "app", WorkingDir: "/srv/app", Files: []string{"/srv/app/compose.yaml"}}
	images, err := runtime.ComposeConfig(context.Background(), stack)
	if images != nil || !errors.Is(err, ErrRemoteUnsupported) {
		t.Fatalf("ComposeConfig(remote) = %#v, %v", images, err)
	}
}

func TestComposeLifecycleExecutesExactArgumentsAndWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-only")
	}
	workingDir := t.TempDir()
	captureDir := t.TempDir()
	script := writeComposeFixture(t, captureDir, ``)
	stubComposeClient(t, ConnectionInfo{})
	runtime := NewRuntime(ResolverOptions{})
	runtime.command = script
	stack := domain.Stack{Name: "app", WorkingDir: workingDir, Files: []string{filepath.Join(workingDir, "compose.yaml"), filepath.Join(workingDir, "override.yaml")}}

	if err := runtime.PullStackServices(context.Background(), stack, []string{"api", "worker"}); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(filepath.Join(captureDir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"compose", "--project-name", "app", "--project-directory", workingDir, "-f", stack.Files[0], "-f", stack.Files[1], "pull", "api", "worker", ""}, "\n")
	if string(args) != want {
		t.Fatalf("arguments = %q, want %q", args, want)
	}
	cwd, err := os.ReadFile(filepath.Join(captureDir, "cwd"))
	if err != nil || strings.TrimSpace(string(cwd)) != workingDir {
		t.Fatalf("working directory = %q, %v", cwd, err)
	}
}

func TestComposeConfigExecutesAndReturnsOnlyServiceImages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-only")
	}
	workingDir := t.TempDir()
	captureDir := t.TempDir()
	output := `{"services":{"web":{"image":"app:latest","environment":{"TOKEN":"secret"}},"builder":{"build":{"context":"."}}}}`
	script := writeComposeFixture(t, captureDir, output)
	stubComposeClient(t, ConnectionInfo{})
	runtime := NewRuntime(ResolverOptions{})
	runtime.command = script
	stack := domain.Stack{Name: "app", WorkingDir: workingDir, Files: []string{filepath.Join(workingDir, "compose.yaml")}}

	images, err := runtime.ComposeConfig(context.Background(), stack)
	if err != nil {
		t.Fatal(err)
	}
	want := []ports.ComposeServiceImage{{Service: "builder", Build: true}, {Service: "web", Reference: "app:latest"}}
	if !reflect.DeepEqual(images, want) {
		t.Fatalf("ComposeConfig() = %#v, want %#v", images, want)
	}
}

func TestComposeLifecycleRejectsRemoteBeforeExecutingCommand(t *testing.T) {
	stubComposeClient(t, ConnectionInfo{Remote: true})
	runtime := NewRuntime(ResolverOptions{})
	runtime.command = filepath.Join(t.TempDir(), "must-not-run")
	stack := domain.Stack{Name: "app", WorkingDir: t.TempDir(), Files: []string{"compose.yaml"}}
	if err := runtime.Up(context.Background(), stack); !errors.Is(err, ErrRemoteUnsupported) {
		t.Fatalf("Up(remote) = %v", err)
	}
}

func stubComposeClient(t *testing.T, info ConnectionInfo) {
	t.Helper()
	original := composeClient
	composeClient = func(context.Context, ResolverOptions) (*Client, ConnectionInfo, error) {
		return &Client{}, info, nil
	}
	t.Cleanup(func() { composeClient = original })
}

func writeComposeFixture(t *testing.T, captureDir, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker-fixture")
	script := "#!/bin/sh\n" +
		"pwd > \"" + filepath.Join(captureDir, "cwd") + "\"\n" +
		"printf '%s\\n' \"$@\" > \"" + filepath.Join(captureDir, "args") + "\"\n"
	if output != "" {
		script += "printf '%s' '" + output + "'\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
