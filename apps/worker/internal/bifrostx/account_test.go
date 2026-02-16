package bifrostx

import (
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/types"
	schemas "github.com/maximhq/bifrost/core/schemas"
)

func TestProviderKeyRoundtrip(t *testing.T) {
	key := ProviderKeyForChannelID(42)
	if key != "wheel-ch-42" {
		t.Fatalf("unexpected provider key: %s", key)
	}
	id, err := ParseChannelIDFromProviderKey(key)
	if err != nil {
		t.Fatalf("parse provider key: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id=42, got %d", id)
	}
}

func TestParseChannelIDFromProviderKey_Invalid(t *testing.T) {
	cases := []schemas.ModelProvider{
		"openai",
		"wheel-ch-",
		"wheel-ch-a",
		"wheel-ch-0",
	}
	for _, c := range cases {
		if _, err := ParseChannelIDFromProviderKey(c); err == nil {
			t.Fatalf("expected parse error for %q", c)
		}
	}
}

func TestBaseProviderForChannelType(t *testing.T) {
	tests := []struct {
		name string
		in   types.OutboundType
		want schemas.ModelProvider
	}{
		{"anthropic", types.OutboundAnthropic, schemas.Anthropic},
		{"gemini", types.OutboundGemini, schemas.Gemini},
		{"openai chat", types.OutboundOpenAIChat, schemas.OpenAI},
		{"openai", types.OutboundOpenAI, schemas.OpenAI},
		{"openai responses", types.OutboundOpenAIResponses, schemas.OpenAI},
		{"openai embedding", types.OutboundOpenAIEmbedding, schemas.OpenAI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseProviderForChannelType(tt.in); got != tt.want {
				t.Fatalf("baseProviderForChannelType(%v)=%s want=%s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRequestPathOverrides(t *testing.T) {
	openai := requestPathOverrides(schemas.OpenAI)
	if openai[schemas.ChatCompletionRequest] != "/v1/chat/completions" {
		t.Fatalf("unexpected openai chat path override")
	}
	if openai[schemas.ResponsesRequest] != "/v1/responses" {
		t.Fatalf("unexpected openai responses path override")
	}

	anth := requestPathOverrides(schemas.Anthropic)
	if anth[schemas.ChatCompletionRequest] != "/v1/messages" {
		t.Fatalf("unexpected anthropic chat path override")
	}

	if gem := requestPathOverrides(schemas.Gemini); gem != nil {
		t.Fatalf("expected nil gemini path overrides, got: %v", gem)
	}
}

func TestSelectBaseURL(t *testing.T) {
	base := selectBaseURL([]types.BaseUrl{
		{URL: "https://a.example.com/", Delay: 120},
		{URL: "https://b.example.com", Delay: 40},
		{URL: "https://c.example.com", Delay: 60},
	})
	if base != "https://b.example.com" {
		t.Fatalf("unexpected selected base url: %s", base)
	}
}

func TestBuildProxyConfig(t *testing.T) {
	envChannel := &types.Channel{
		Proxy:        true,
		ChannelProxy: ptr("environment"),
	}
	envProxy := buildProxyConfig(envChannel)
	if envProxy == nil || envProxy.Type != schemas.EnvProxy {
		t.Fatalf("expected env proxy config")
	}

	socksChannel := &types.Channel{
		Proxy:        true,
		ChannelProxy: ptr("socks5://user:pass@127.0.0.1:1080"),
	}
	socksProxy := buildProxyConfig(socksChannel)
	if socksProxy == nil || socksProxy.Type != schemas.Socks5Proxy {
		t.Fatalf("expected socks5 proxy config")
	}
	if socksProxy.Username != "user" || socksProxy.Password != "pass" {
		t.Fatalf("expected socks creds")
	}

	httpChannel := &types.Channel{
		Proxy:        true,
		ChannelProxy: ptr("http://127.0.0.1:8080"),
	}
	httpProxy := buildProxyConfig(httpChannel)
	if httpProxy == nil || httpProxy.Type != schemas.HTTPProxy {
		t.Fatalf("expected http proxy config")
	}
}

func ptr[T any](v T) *T {
	return &v
}
