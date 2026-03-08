package translatorbridge

import (
	"context"
	"testing"

	runtimetranslator "github.com/kunish/wheel/apps/worker/internal/runtimecore/translator"
	"github.com/tidwall/gjson"
)

func TestDefaultAdapterTranslatesRequest(t *testing.T) {
	got := Default().TranslateRequest(runtimetranslator.FormatOpenAI, runtimetranslator.FormatOpenAI, "model", []byte(`{"x":1}`), false)
	if v := gjson.GetBytes(got, "model").String(); v != "model" {
		t.Fatalf("TranslateRequest() model = %q", v)
	}
}

func TestTranslateRequestDelegatesToUnderlyingTranslator(t *testing.T) {
	got := TranslateRequest("openai", "openai", "model", []byte(`{"x":1}`), false)
	if v := gjson.GetBytes(got, "model").String(); v != "model" {
		t.Fatalf("TranslateRequest() model = %q", v)
	}
}

func TestTranslateNonStreamDelegatesToUnderlyingTranslator(t *testing.T) {
	got := TranslateNonStream(context.Background(), "openai", "openai", "model", nil, nil, []byte(`{"y":2}`), nil)
	if got != `{"y":2}` {
		t.Fatalf("TranslateNonStream() = %q", got)
	}
}

func TestTranslateStreamDelegatesToUnderlyingTranslator(t *testing.T) {
	got := TranslateStream(context.Background(), "openai", "openai", "model", nil, nil, []byte("data: hi"), nil)
	if len(got) != 1 || got[0] != "hi" {
		t.Fatalf("TranslateStream() = %#v", got)
	}
}

func TestDefaultAdapterTranslatesNonStream(t *testing.T) {
	got := Default().TranslateNonStream(context.Background(), runtimetranslator.FormatOpenAI, runtimetranslator.FormatOpenAI, "model", nil, nil, []byte(`{"y":2}`), nil)
	if got != `{"y":2}` {
		t.Fatalf("TranslateNonStream() = %q", got)
	}
}

func TestDefaultAdapterTranslatesTokenCount(t *testing.T) {
	got := Default().TranslateTokenCount(context.Background(), runtimetranslator.FormatOpenAI, runtimetranslator.FormatOpenAI, 7, []byte(`{"usage":{"total_tokens":7}}`))
	if gjson.Get(got, "usage.total_tokens").Int() != 7 {
		t.Fatalf("TranslateTokenCount() = %q", got)
	}
}
