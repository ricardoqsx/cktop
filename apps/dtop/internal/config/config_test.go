package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPathsUsesDefaultsWhenFilesAreMissing(t *testing.T) {
	config, err := LoadPaths(Paths{System: "/missing/system.conf", User: "/missing/user.conf"})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Display.MemoryMode != MemoryBoth || config.Display.AccentColor != "63" || config.Display.FocusColor != "15" || !config.Updates.Enabled || config.Updates.Scope != "running" || config.Updates.Interval.String() != "15m0s" || config.Updates.Concurrency != 4 {
		t.Fatalf("unexpected display defaults: %#v", config.Display)
	}
}

func TestLoadPathsParsesUpdatesAndRejectsUnsafeValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[updates]\nenabled = false\nscope = running\ninterval = 20m\nconcurrency = 8\n")
	loaded, err := LoadPaths(Paths{User: path})
	if err != nil || loaded.Updates.Enabled || loaded.Updates.Interval.String() != "20m0s" || loaded.Updates.Concurrency != 8 {
		t.Fatalf("updates=%#v err=%v", loaded.Updates, err)
	}
	writeConfig(t, path, "[updates]\ninterval = 30s\n")
	if _, err := LoadPaths(Paths{User: path}); err == nil {
		t.Fatal("expected unsafe interval rejection")
	}
}

func TestLoadPathsParsesDisplayColors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[display]\naccent_color = 12\nfocus_color = 231\n")

	config, err := LoadPaths(Paths{User: path})
	if err != nil || config.Display.AccentColor != "12" || config.Display.FocusColor != "231" {
		t.Fatalf("unexpected display colors: %#v, %v", config.Display, err)
	}
}

func TestLoadPathsRejectsInvalidDisplayColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[display]\nfocus_color = 256\n")
	if _, err := LoadPaths(Paths{User: path}); err == nil || !strings.Contains(err.Error(), path+":2") {
		t.Fatalf("expected controlled color config error, got %v", err)
	}
}

func TestLoadPathsAppliesUserOverride(t *testing.T) {
	dir := t.TempDir()
	system := filepath.Join(dir, "system.conf")
	user := filepath.Join(dir, "user.conf")
	writeConfig(t, system, "[display]\nmemory_mode = usage\n")
	writeConfig(t, user, "[display]\nmemory_mode = percent\n")

	config, err := LoadPaths(Paths{System: system, User: user})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Display.MemoryMode != MemoryPercent {
		t.Fatalf("expected user override percent, got %q", config.Display.MemoryMode)
	}
}

func TestLoadPathsReportsFileAndLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[display]\nmemory_mode = unknown\n")

	_, err := LoadPaths(Paths{User: path})
	if err == nil {
		t.Fatal("expected config error")
	}
	if !strings.Contains(err.Error(), path+":2") {
		t.Fatalf("expected file and line in error, got %v", err)
	}
}

func TestLoadPathsRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[display]\nunknown = value\n")

	_, err := LoadPaths(Paths{User: path})
	if err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestLoadPathsRegistersComposeProjectAndResolvesFiles(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "compose.yaml")
	writeConfig(t, manifest, "services: {}\n")
	path := filepath.Join(dir, "dtop.conf")
	writeConfig(t, path, "[compose \"Demo\"]\nworking_dir = "+dir+"\nfiles = compose.yaml, override.yaml\n")

	config, err := LoadPaths(Paths{User: path})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(config.ComposeProjects) != 1 || config.ComposeProjects[0].Name != "Demo" {
		t.Fatalf("unexpected projects: %#v", config.ComposeProjects)
	}
	project := config.ComposeProjects[0]
	if project.WorkingDir != dir || project.Files[0] != manifest || project.MissingFiles[0] != filepath.Join(dir, "override.yaml") {
		t.Fatalf("unexpected resolved registration: %#v", project)
	}
}

func TestLoadPathsReportsMalformedComposeRegistrationWithoutFailing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dtop.conf")
	writeConfig(t, path, "[compose \"broken\"]\nworking_dir = \nfiles = compose.yaml\n")

	config, err := LoadPaths(Paths{User: path})
	if err != nil {
		t.Fatalf("malformed registration must not prevent startup: %v", err)
	}
	if len(config.ComposeProjects) != 0 || len(config.ComposeDiagnostics) != 1 || !strings.Contains(config.ComposeDiagnostics[0], "requires a nonempty") {
		t.Fatalf("expected controlled diagnostic, got projects=%#v diagnostics=%#v", config.ComposeProjects, config.ComposeDiagnostics)
	}
}

func TestLoadPathsDoesNotRegisterComposeSectionWithUnsupportedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	writeConfig(t, path, "[compose \"broken\"]\nworking_dir = "+dir+"\nfiles = compose.yaml\nunknown = value\n")

	config, err := LoadPaths(Paths{User: path})
	if err != nil {
		t.Fatalf("malformed registration must not prevent startup: %v", err)
	}
	if len(config.ComposeProjects) != 0 || len(config.ComposeDiagnostics) != 1 || !strings.Contains(config.ComposeDiagnostics[0], "unsupported key") {
		t.Fatalf("expected rejected registration diagnostic, got projects=%#v diagnostics=%#v", config.ComposeProjects, config.ComposeDiagnostics)
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
