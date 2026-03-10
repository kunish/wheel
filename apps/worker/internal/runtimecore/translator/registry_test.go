package translator

import (
	"context"
	"testing"
)

func TestRegistryTranslatesRequestAndResponse(t *testing.T) {
	t.Parallel()

	r := newRegistry()
	r.register(FromString("openai"), FromString("claude"), func(_ string, rawJSON []byte, _ bool) []byte {
		return append([]byte("req:"), rawJSON...)
	}, responseTransform{
		NonStream: func(context.Context, string, []byte, []byte, []byte, *any) string {
			return "resp:ok"
		},
	})

	if got := string(r.translateRequest(FromString("openai"), FromString("claude"), "m", []byte("body"), false)); got != "req:body" {
		t.Fatalf("translateRequest() = %q, want req:body", got)
	}
	if got := r.translateNonStream(context.Background(), FromString("claude"), FromString("openai"), "m", nil, nil, []byte("body"), nil); got != "resp:ok" {
		t.Fatalf("translateNonStream() = %q, want resp:ok", got)
	}
}
