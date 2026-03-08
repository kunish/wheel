package openai

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

func TestResolveEndpointOverrideExported(t *testing.T) {
	reg := registry.GetGlobalRegistry()
	reg.RegisterClient("test-endpoint-override", "openai", []*registry.ModelInfo{{
		ID:                 "test-model",
		SupportedEndpoints: []string{OpenAIResponsesEndpoint},
	}})
	t.Cleanup(func() {
		reg.UnregisterClient("test-endpoint-override")
	})

	got, ok := ResolveEndpointOverride("test-model", OpenAIChatEndpoint)
	if !ok {
		t.Fatal("expected override")
	}
	if got != OpenAIResponsesEndpoint {
		t.Fatalf("override = %q, want %q", got, OpenAIResponsesEndpoint)
	}
}
