package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLIParsesConfigAndVersion(t *testing.T) {
	options, err := parseCLI([]string{"--config", "/tmp/dtop.conf", "--version"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if options.configPath != "/tmp/dtop.conf" || !options.version {
		t.Fatalf("parseCLI() = %#v", options)
	}
}

func TestParseCLIPrintsHelpWithoutStartingApplication(t *testing.T) {
	var output bytes.Buffer
	_, err := parseCLI([]string{"--help"}, &output)
	if !isHelp(err) {
		t.Fatalf("parseCLI(--help) error = %v", err)
	}
	for _, expected := range []string{"Usage: dtop", "config", "version"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("help does not contain %q:\n%s", expected, output.String())
		}
	}
}

func TestParseCLIRejectsPositionalArguments(t *testing.T) {
	if _, err := parseCLI([]string{"unexpected"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("parseCLI() error = %v", err)
	}
}

func TestParseCLIRejectsEmptyConfigPath(t *testing.T) {
	if _, err := parseCLI([]string{"--config="}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "nonempty path") {
		t.Fatalf("parseCLI() error = %v", err)
	}
}

func TestVersionTextContainsBuildMetadata(t *testing.T) {
	previousVersion, previousCommit, previousDate := version, commit, buildDate
	t.Cleanup(func() {
		version, commit, buildDate = previousVersion, previousCommit, previousDate
	})
	version, commit, buildDate = "0.4.0", "abc123", "2026-08-15T12:00:00Z"
	if got, want := versionText(), "dtop 0.4.0 (commit abc123, built 2026-08-15T12:00:00Z)"; got != want {
		t.Fatalf("versionText() = %q, want %q", got, want)
	}
}
