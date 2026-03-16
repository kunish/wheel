package relay

import (
	"sync"
)

// RequestPool provides pooled RelayContext and RelayPluginResponse objects
// to reduce GC pressure on the hot request path.
var RequestPool = &requestPool{}

type requestPool struct {
	ctxPool  sync.Pool
	respPool sync.Pool
	bodyPool sync.Pool
}

func init() {
	RequestPool.ctxPool = sync.Pool{
		New: func() any {
			return &RelayContext{
				Values: make(map[string]any, 8),
			}
		},
	}
	RequestPool.respPool = sync.Pool{
		New: func() any {
			return &RelayPluginResponse{}
		},
	}
	RequestPool.bodyPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 16)
		},
	}
}

// AcquireContext gets a RelayContext from the pool.
func (p *requestPool) AcquireContext() *RelayContext {
	return p.ctxPool.Get().(*RelayContext)
}

// ReleaseContext returns a RelayContext to the pool after clearing it.
func (p *requestPool) ReleaseContext(ctx *RelayContext) {
	if ctx == nil {
		return
	}
	ctx.GinCtx = nil
	ctx.RequestModel = ""
	ctx.TargetModel = ""
	ctx.Body = nil
	ctx.BodyBytes = nil
	ctx.ApiKeyID = 0
	ctx.IsStream = false
	ctx.IsAnthropicInbound = false
	ctx.RequestType = ""
	ctx.Channel = nil
	ctx.SelectedKey = nil
	ctx.Group = nil
	clear(ctx.Values)
	p.ctxPool.Put(ctx)
}

// AcquireResponse gets a RelayPluginResponse from the pool.
func (p *requestPool) AcquireResponse() *RelayPluginResponse {
	return p.respPool.Get().(*RelayPluginResponse)
}

// ReleaseResponse returns a RelayPluginResponse to the pool after clearing it.
func (p *requestPool) ReleaseResponse(resp *RelayPluginResponse) {
	if resp == nil {
		return
	}
	resp.Success = false
	resp.StatusCode = 0
	resp.Body = nil
	resp.InputTokens = 0
	resp.OutputTokens = 0
	resp.CacheReadTokens = 0
	resp.CacheCreationTokens = 0
	resp.Cost = 0
	resp.Error = nil
	resp.IsStream = false
	resp.StreamContent = ""
	resp.ThinkingContent = ""
	p.respPool.Put(resp)
}

// AcquireBody gets a map[string]any from the pool for request body parsing.
func (p *requestPool) AcquireBody() map[string]any {
	return p.bodyPool.Get().(map[string]any)
}

// ReleaseBody returns a body map to the pool.
func (p *requestPool) ReleaseBody(body map[string]any) {
	if body == nil {
		return
	}
	clear(body)
	p.bodyPool.Put(body)
}

// ── Pipeline Pool ──

var pipelinePool = sync.Pool{
	New: func() any {
		return &PluginPipeline{
			plugins: make([]RelayPlugin, 0, 8),
		}
	},
}

// AcquirePipeline gets a PluginPipeline from the pool.
func AcquirePipeline() *PluginPipeline {
	return pipelinePool.Get().(*PluginPipeline)
}

// ReleasePipeline returns a PluginPipeline to the pool.
func ReleasePipeline(p *PluginPipeline) {
	if p == nil {
		return
	}
	p.plugins = p.plugins[:0]
	pipelinePool.Put(p)
}
