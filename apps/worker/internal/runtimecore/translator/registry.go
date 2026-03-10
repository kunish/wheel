package translator

import (
	"context"
	"sync"
)

// registry manages translation functions across schemas.
type registry struct {
	mu        sync.RWMutex
	requests  map[Format]map[Format]requestTransform
	responses map[Format]map[Format]responseTransform
}

// newRegistry constructs an empty translator registry.
func newRegistry() *registry {
	return &registry{
		requests:  make(map[Format]map[Format]requestTransform),
		responses: make(map[Format]map[Format]responseTransform),
	}
}

// register stores request/response transforms between two formats.
func (r *registry) register(from, to Format, request requestTransform, response responseTransform) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.requests[from]; !ok {
		r.requests[from] = make(map[Format]requestTransform)
	}
	if request != nil {
		r.requests[from][to] = request
	}

	if _, ok := r.responses[from]; !ok {
		r.responses[from] = make(map[Format]responseTransform)
	}
	r.responses[from][to] = response
}

// translateRequest converts a payload between schemas, returning the original payload
// if no translator is registered.
func (r *registry) translateRequest(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.requests[from]; ok {
		if fn, isOk := byTarget[to]; isOk && fn != nil {
			return fn(model, rawJSON, stream)
		}
	}
	return rawJSON
}

// hasResponseTransformer indicates whether a response translator exists.
func (r *registry) hasResponseTransformer(from, to Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[from]; ok {
		if _, isOk := byTarget[to]; isOk {
			return true
		}
	}
	return false
}

// translateStream applies the registered streaming response translator.
func (r *registry) translateStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.Stream != nil {
			return fn.Stream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		}
	}
	return []string{string(rawJSON)}
}

// translateNonStream applies the registered non-stream response translator.
func (r *registry) translateNonStream(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.NonStream != nil {
			return fn.NonStream(ctx, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
		}
	}
	return string(rawJSON)
}

// translateTokenCount applies the registered token count response translator.
func (r *registry) translateTokenCount(ctx context.Context, from, to Format, count int64, rawJSON []byte) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byTarget, ok := r.responses[to]; ok {
		if fn, isOk := byTarget[from]; isOk && fn.TokenCount != nil {
			return fn.TokenCount(ctx, count)
		}
	}
	return string(rawJSON)
}

var defaultRegistry = newRegistry()

// defaultReg exposes the package-level registry for shared use.
func defaultReg() *registry {
	return defaultRegistry
}

// registerDefault attaches transforms to the default registry.
func registerDefault(from, to Format, request requestTransform, response responseTransform) {
	defaultRegistry.register(from, to, request, response)
}

// translateRequestDefault is a helper on the default registry.
func translateRequestDefault(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	return defaultRegistry.translateRequest(from, to, model, rawJSON, stream)
}

// hasResponseTransformerDefault inspects the default registry.
func hasResponseTransformerDefault(from, to Format) bool {
	return defaultRegistry.hasResponseTransformer(from, to)
}

// translateStreamDefault is a helper on the default registry.
func translateStreamDefault(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return defaultRegistry.translateStream(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// translateNonStreamDefault is a helper on the default registry.
func translateNonStreamDefault(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return defaultRegistry.translateNonStream(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// translateTokenCountDefault is a helper on the default registry.
func translateTokenCountDefault(ctx context.Context, from, to Format, count int64, rawJSON []byte) string {
	return defaultRegistry.translateTokenCount(ctx, from, to, count, rawJSON)
}
