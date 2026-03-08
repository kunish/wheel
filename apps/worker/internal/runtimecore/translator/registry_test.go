package translator

import (
	"context"
	"testing"
)

func TestRegistryTranslatesRequestAndResponse(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(FromString("openai"), FromString("claude"), func(_ string, rawJSON []byte, _ bool) []byte {
		return append([]byte("req:"), rawJSON...)
	}, ResponseTransform{
		NonStream: func(context.Context, string, []byte, []byte, []byte, *any) string {
			return "resp:ok"
		},
	})

	if got := string(r.TranslateRequest(FromString("openai"), FromString("claude"), "m", []byte("body"), false)); got != "req:body" {
		t.Fatalf("TranslateRequest() = %q, want req:body", got)
	}
	if got := r.TranslateNonStream(context.Background(), FromString("claude"), FromString("openai"), "m", nil, nil, []byte("body"), nil); got != "resp:ok" {
		t.Fatalf("TranslateNonStream() = %q, want resp:ok", got)
	}
}
