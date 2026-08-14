package application

import (
	"context"
	"reflect"
	"testing"
)

func TestPruneCommandsAreExplicitAndSystemExcludesVolumes(t *testing.T) {
	tests := map[PruneKind][]string{
		PruneContainers: {"docker", "container", "prune", "--force"},
		PruneImages:     {"docker", "image", "prune", "--all", "--force"},
		PruneNetworks:   {"docker", "network", "prune", "--force"},
		PruneVolumes:    {"docker", "volume", "prune", "--force"},
		PruneSystem:     {"docker", "system", "prune", "--all", "--force"},
	}
	for kind, want := range tests {
		got, ok := PruneCommand(kind)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("PruneCommand(%q) = %v, %v; want %v", kind, got, ok, want)
		}
		for _, argument := range got {
			if kind == PruneSystem && argument == "--volumes" {
				t.Fatal("system prune must not include --volumes")
			}
		}
	}
}

func TestPruneDelegatesOnlyValidatedDockerArguments(t *testing.T) {
	runtime := &fakeRuntime{}
	result := NewContainerService(runtime).Prune(context.Background(), PruneVolumes)
	if result.Err != nil || !reflect.DeepEqual(result.Command, []string{"docker", "volume", "prune", "--force"}) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !reflect.DeepEqual(runtime.actions, []string{"prune:volume:prune:--force"}) {
		t.Fatalf("runtime actions = %v", runtime.actions)
	}

	result = NewContainerService(runtime).Prune(context.Background(), PruneKind("invalid"))
	if result.Err == nil || len(runtime.actions) != 1 {
		t.Fatalf("invalid kind reached runtime: result=%#v actions=%v", result, runtime.actions)
	}
}
