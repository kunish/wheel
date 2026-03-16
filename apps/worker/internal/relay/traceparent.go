package relay

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// InjectTraceparent adds W3C traceparent and tracestate headers to outgoing requests
// so that distributed traces propagate through to upstream LLM providers.
func InjectTraceparent(ctx context.Context, headers map[string]string) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	sc := span.SpanContext()
	tp := fmt.Sprintf("00-%s-%s-%02x", sc.TraceID().String(), sc.SpanID().String(), sc.TraceFlags())
	headers["traceparent"] = tp

	if sc.TraceState().Len() > 0 {
		headers["tracestate"] = sc.TraceState().String()
	}
}

// InjectTraceparentHTTP adds W3C traceparent header to a standard http.Request.
func InjectTraceparentHTTP(ctx context.Context, req *http.Request) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	sc := span.SpanContext()
	tp := fmt.Sprintf("00-%s-%s-%02x", sc.TraceID().String(), sc.SpanID().String(), sc.TraceFlags())
	req.Header.Set("traceparent", tp)

	if sc.TraceState().Len() > 0 {
		req.Header.Set("tracestate", sc.TraceState().String())
	}
}

// GenerateSpanID produces a random 16-hex span ID for cases without OTEL.
func GenerateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%016x", b)
}
