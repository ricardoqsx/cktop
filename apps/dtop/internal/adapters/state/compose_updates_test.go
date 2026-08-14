package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

func TestDefaultComposeUpdatesPath(t *testing.T) {
	t.Run("XDG state home takes precedence", func(t *testing.T) {
		t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
		xdg := filepath.Join(t.TempDir(), "state")
		t.Setenv("XDG_STATE_HOME", xdg)
		want := filepath.Join(xdg, "dtop", "compose-updates.json")
		if got := DefaultComposeUpdatesPath(); got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
	})

	t.Run("home fallback", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", "")
		want := filepath.Join(home, ".local", "state", "dtop", "compose-updates.json")
		if got := DefaultComposeUpdatesPath(); got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
	})
}

func TestComposeUpdatesMissingFileIsEnabledAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "compose-updates.json")
	store, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatalf("NewComposeUpdates() error = %v", err)
	}
	if _, ok := store.Get("missing"); ok {
		t.Fatal("missing project was found")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("constructor created state file: %v", err)
	}
}

func TestComposeUpdatesRoundTripCopiesAndPreservesProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	store, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	project := completeProject("active")
	if err := store.Put(project); err != nil {
		t.Fatal(err)
	}

	project.Services["web"] = domain.ComposeUpdateService{Reference: "mutated:input"}
	got, ok := store.Get("active")
	if !ok || got.Services["web"].Reference != "registry.example/web:latest" {
		t.Fatalf("Put retained caller map: %#v", got)
	}
	got.Services["web"] = domain.ComposeUpdateService{Reference: "mutated:output"}
	again, _ := store.Get("active")
	if again.Services["web"].Reference != "registry.example/web:latest" {
		t.Fatalf("Get returned store map: %#v", again)
	}

	ineligible := completeProject("ineligible")
	ineligible.Services["web"] = domain.ComposeUpdateService{Reference: "registry.example/old:latest"}
	if err := store.Put(ineligible); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(completeProject("active")); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	want := completeProject("active")
	got, ok = reopened.Get("active")
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
	preserved, ok := reopened.Get("ineligible")
	if !ok || preserved.Services["web"].Reference != "registry.example/old:latest" {
		t.Fatalf("unrelated project was pruned: %#v", preserved)
	}
}

func TestComposeUpdatesStoresReloadEachOthersMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	first, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}

	project := completeProject("shared")
	project.ConfigFingerprint = "written:first"
	if err := first.Put(project); err != nil {
		t.Fatal(err)
	}
	got, ok := second.Get(project.Name)
	if !ok || got.ConfigFingerprint != "written:first" {
		t.Fatalf("second store did not reload first marker: %#v", got)
	}

	project.ConfigFingerprint = "written:second"
	if err := second.Put(project); err != nil {
		t.Fatal(err)
	}
	got, ok = first.Get(project.Name)
	if !ok || got.ConfigFingerprint != "written:second" {
		t.Fatalf("first store did not reload second marker: %#v", got)
	}
}

func TestComposeUpdatesInterleavedPutMergesLatestDiskProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	first, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := first.Put(completeProject("first")); err != nil {
		t.Fatal(err)
	}
	if err := second.Put(completeProject("second")); err != nil {
		t.Fatal(err)
	}
	updated := completeProject("first")
	updated.ConfigFingerprint = "updated:first"
	if err := first.Put(updated); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"first", "second"} {
		if _, ok := second.Get(name); !ok {
			t.Fatalf("project %q was erased by an interleaved Put", name)
		}
	}
}

func TestComposeUpdatesMutationScopeAllowsOwnerOperationsAndBlocksOtherStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	owner, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	waiter, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}

	releaseOwner, err := owner.BeginMutation()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOwner()
	if _, err := owner.BeginMutation(); err == nil {
		t.Fatal("nested mutation on the same store succeeded")
	}

	ownerOperation := make(chan error, 1)
	go func() {
		project := completeProject("held")
		if err := owner.Put(project); err != nil {
			ownerOperation <- err
			return
		}
		if got, ok := owner.Get(project.Name); !ok || !reflect.DeepEqual(got, project) {
			ownerOperation <- fmt.Errorf("Get while held = %#v, found %t", got, ok)
			return
		}
		ownerOperation <- owner.Health()
	}()
	select {
	case err := <-ownerOperation:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("owner Get/Put deadlocked while mutation lock was held")
	}

	type mutationResult struct {
		release func()
		err     error
	}
	waiterStarted := make(chan struct{})
	waiterResult := make(chan mutationResult, 1)
	go func() {
		close(waiterStarted)
		release, err := waiter.BeginMutation()
		waiterResult <- mutationResult{release: release, err: err}
	}()
	<-waiterStarted
	select {
	case result := <-waiterResult:
		if result.release != nil {
			result.release()
		}
		t.Fatalf("second store did not wait for mutation release: %v", result.err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseOwner()
	select {
	case result := <-waiterResult:
		if result.err != nil {
			t.Fatal(result.err)
		}
		defer result.release()
		if _, ok := waiter.Get("held"); !ok {
			t.Fatal("waiting store did not reload mutation state while holding lock")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second store remained blocked after mutation release")
	}
}

func TestComposeUpdatesMutationReleaseIsIdempotent(t *testing.T) {
	store, err := NewComposeUpdates(filepath.Join(t.TempDir(), "compose-updates.json"))
	if err != nil {
		t.Fatal(err)
	}
	release, err := store.BeginMutation()
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()

	nextRelease, err := store.BeginMutation()
	if err != nil {
		t.Fatalf("BeginMutation() after repeated release = %v", err)
	}
	release()
	nextRelease()
}

func TestComposeUpdatesReloadNoticesMalformedReplacementAndDisablesWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	getStore, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := getStore.Put(completeProject("existing")); err != nil {
		t.Fatal(err)
	}
	healthStore, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}

	malformed := []byte(`{"version":1,"projects":`)
	if err := os.WriteFile(path, malformed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := getStore.Get("existing"); ok {
		t.Fatal("Get returned stale state after malformed replacement")
	}
	if err := healthStore.Health(); err == nil {
		t.Fatal("Health did not report malformed replacement")
	}
	if err := getStore.Put(completeProject("blocked")); err == nil {
		t.Fatal("Put succeeded after malformed replacement")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, malformed) {
		t.Fatalf("disabled store replaced malformed file with %q", after)
	}
}

func TestComposeUpdatesJSONContainsFingerprintAndServiceFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	store, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(completeProject("media")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"registration_fingerprint": "registration:media"`,
		`"config_fingerprint": "config:media"`,
		`"downloaded_digest": "sha256:downloaded"`,
		`"downloaded_image_id": "sha256:downloaded-image"`,
		`"applied_digest": "sha256:applied"`,
		`"applied_image_id": "sha256:applied-image"`,
		`"pending_unknown": true`,
	} {
		if !strings.Contains(string(data), field) {
			t.Errorf("state file does not contain %s:\n%s", field, data)
		}
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document["version"] != float64(1) {
		t.Fatalf("version = %#v", document["version"])
	}
}

func TestComposeUpdatesMalformedAndFutureFilesDisableWritesAndRemainUntouched(t *testing.T) {
	for name, contents := range map[string]string{
		"malformed": `{not json`,
		"future":    `{"version":2,"projects":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "compose-updates.json")
			original := []byte(contents)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := NewComposeUpdates(path)
			if store == nil || err == nil {
				t.Fatalf("store = %#v, error = %v", store, err)
			}
			if err := store.Put(completeProject("blocked")); err == nil {
				t.Fatal("Put succeeded on disabled store")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, original) {
				t.Fatalf("disabled Put changed file to %q", after)
			}
		})
	}
}

func TestComposeUpdatesInvalidDataDisablesStore(t *testing.T) {
	for name, projectJSON := range map[string]string{
		"project key mismatch": `"wrong":{"name":"project","services":{}}`,
		"empty project name":   `"":{"name":"","services":{}}`,
		"empty service name":   `"project":{"name":"project","services":{"":{"reference":"image:tag"}}}`,
		"empty reference":      `"project":{"name":"project","services":{"web":{"reference":""}}}`,
		"invalid digest":       `"project":{"name":"project","services":{"web":{"reference":"image:tag","downloaded_digest":"sha256"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "compose-updates.json")
			original := []byte(`{"version":1,"projects":{` + projectJSON + `}}`)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			store, err := NewComposeUpdates(path)
			if store == nil || err == nil {
				t.Fatalf("store = %#v, error = %v", store, err)
			}
			if err := store.Put(completeProject("blocked")); err == nil {
				t.Fatal("Put succeeded on disabled store")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, original) {
				t.Fatal("disabled Put changed invalid file")
			}
		})
	}
}

func TestComposeUpdatesPutRejectsInvalidProjectWithoutChangingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compose-updates.json")
	store, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(completeProject("valid")); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	invalid := completeProject("invalid")
	invalid.Services["web"] = domain.ComposeUpdateService{Reference: "image:tag", AppliedDigest: "missing-colon"}
	if err := store.Put(invalid); err == nil {
		t.Fatal("invalid Put succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, original) {
		t.Fatal("invalid Put changed state file")
	}
}

func TestComposeUpdatesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not portable to Windows")
	}
	root := t.TempDir()
	directory := filepath.Join(root, "private")
	path := filepath.Join(directory, "compose-updates.json")
	store, err := NewComposeUpdates(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(completeProject("project")); err != nil {
		t.Fatal(err)
	}
	assertPermissions(t, directory, 0o700)
	assertPermissions(t, path, 0o600)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := map[string]bool{
		filepath.Base(path):           true,
		filepath.Base(path) + ".lock": true,
	}
	if len(entries) != len(wantEntries) {
		t.Fatalf("unexpected files remain after write: %#v", entries)
	}
	for _, entry := range entries {
		if !wantEntries[entry.Name()] {
			t.Fatalf("unexpected file remains after write: %q", entry.Name())
		}
	}
	assertPermissions(t, path+".lock", 0o600)
}

func completeProject(name string) domain.ComposeUpdateProject {
	return domain.ComposeUpdateProject{
		Name:                    name,
		RegistrationFingerprint: "registration:" + name,
		ConfigFingerprint:       "config:" + name,
		Services: map[string]domain.ComposeUpdateService{
			"web": {
				Reference:         "registry.example/web:latest",
				DownloadedDigest:  "sha256:downloaded",
				DownloadedImageID: "sha256:downloaded-image",
				AppliedDigest:     "sha256:applied",
				AppliedImageID:    "sha256:applied-image",
				PendingUnknown:    true,
			},
		},
	}
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s permissions = %04o, want %04o", path, got, want)
	}
}
