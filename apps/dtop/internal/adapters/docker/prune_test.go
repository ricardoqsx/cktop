package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPruneRejectsMissingArgumentsBeforeConnecting(t *testing.T) {
	output, err := NewRuntime(ResolverOptions{}).Prune(context.Background())
	if output != "" || err == nil || !strings.Contains(err.Error(), "missing arguments") {
		t.Fatalf("Prune() = %q, %v", output, err)
	}
}

func TestPruneRejectsRemoteEndpointBeforeExecutingDocker(t *testing.T) {
	runtime := NewRuntime(ResolverOptions{Spec: ConnectionSpec{Host: "ssh://docker@example.test"}})
	output, err := runtime.Prune(context.Background(), "system", "prune", "--all", "--force")
	if output != "" || !errors.Is(err, ErrRemoteUnsupported) {
		t.Fatalf("Prune(remote) = %q, %v", output, err)
	}
}
