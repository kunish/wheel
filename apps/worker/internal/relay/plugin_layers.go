package relay

// HTTPTransportPlugin extends RelayPlugin with HTTP-layer interception.
// Plugins implementing this interface can modify/intercept at the HTTP transport layer,
// including per-chunk interception for streaming responses.
type HTTPTransportPlugin interface {
	RelayPlugin

	// TransportPreHook runs at the HTTP layer before the request enters the relay pipeline.
	// Return a *ShortCircuit to skip the request entirely.
	TransportPreHook(ctx *RelayContext, headers map[string]string, path string) *ShortCircuit

	// TransportPostHook runs at the HTTP layer after the response is ready.
	// Can modify response headers or body.
	TransportPostHook(ctx *RelayContext, resp *RelayPluginResponse, headers map[string]string)
}

// StreamChunkInterceptor is an optional interface for plugins that need to
// inspect or modify individual SSE chunks during streaming responses.
type StreamChunkInterceptor interface {
	// InterceptStreamChunk is called for each SSE chunk before it's written to the client.
	// Return the (possibly modified) chunk data, or nil to skip this chunk.
	// Return an error to abort the stream.
	InterceptStreamChunk(ctx *RelayContext, chunkData []byte) ([]byte, error)
}

// ObservabilityPlugin is an interface for plugins that receive completed request
// traces asynchronously after the response has been sent to the client.
// This ensures observability processing doesn't add latency to responses.
type ObservabilityPlugin interface {
	RelayPlugin

	// OnRequestComplete is called asynchronously after a relay request completes.
	// It receives the full request context and response for forwarding to
	// observability backends (OTEL collectors, Datadog, etc.).
	OnRequestComplete(ctx *RelayContext, resp *RelayPluginResponse)
}

// RunTransportPreHooks runs TransportPreHook for all plugins that implement HTTPTransportPlugin.
func (p *PluginPipeline) RunTransportPreHooks(ctx *RelayContext, headers map[string]string, path string) *ShortCircuit {
	for _, plugin := range p.plugins {
		if tp, ok := plugin.(HTTPTransportPlugin); ok {
			if sc := tp.TransportPreHook(ctx, headers, path); sc != nil {
				return sc
			}
		}
	}
	return nil
}

// RunTransportPostHooks runs TransportPostHook for all plugins that implement HTTPTransportPlugin (reverse order).
func (p *PluginPipeline) RunTransportPostHooks(ctx *RelayContext, resp *RelayPluginResponse, headers map[string]string) {
	for i := len(p.plugins) - 1; i >= 0; i-- {
		if tp, ok := p.plugins[i].(HTTPTransportPlugin); ok {
			tp.TransportPostHook(ctx, resp, headers)
		}
	}
}

// RunStreamChunkInterceptors runs InterceptStreamChunk for all plugins that implement StreamChunkInterceptor.
// Returns the (possibly modified) chunk data, or nil to skip, or error to abort.
func (p *PluginPipeline) RunStreamChunkInterceptors(ctx *RelayContext, chunkData []byte) ([]byte, error) {
	data := chunkData
	for i := len(p.plugins) - 1; i >= 0; i-- {
		if sci, ok := p.plugins[i].(StreamChunkInterceptor); ok {
			var err error
			data, err = sci.InterceptStreamChunk(ctx, data)
			if err != nil {
				return nil, err
			}
			if data == nil {
				return nil, nil
			}
		}
	}
	return data, nil
}

// NotifyObservability sends completed request data to all ObservabilityPlugin implementations
// asynchronously so it doesn't block the response.
func (p *PluginPipeline) NotifyObservability(ctx *RelayContext, resp *RelayPluginResponse) {
	for _, plugin := range p.plugins {
		if op, ok := plugin.(ObservabilityPlugin); ok {
			go op.OnRequestComplete(ctx, resp)
		}
	}
}
