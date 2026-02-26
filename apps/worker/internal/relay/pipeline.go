package relay

import "log"

// PluginPipeline manages an ordered list of RelayPlugins and executes
// them in onion-model order: PreHooks forward, PostHooks reverse.
type PluginPipeline struct {
	plugins []RelayPlugin
}

// NewPluginPipeline creates a pipeline with the given plugins.
func NewPluginPipeline(plugins ...RelayPlugin) *PluginPipeline {
	return &PluginPipeline{plugins: plugins}
}

// AddPlugin appends a plugin to the pipeline.
func (p *PluginPipeline) AddPlugin(plugin RelayPlugin) {
	p.plugins = append(p.plugins, plugin)
}

// Plugins returns the registered plugins (read-only use).
func (p *PluginPipeline) Plugins() []RelayPlugin {
	return p.plugins
}

// PreHookResult holds the outcome of running all PreHooks.
type PreHookResult struct {
	// ShortCircuit is non-nil if a plugin short-circuited.
	ShortCircuit *ShortCircuit
	// ExecutedCount is how many PreHooks ran (for symmetric PostHook).
	ExecutedCount int
}

// RunPreHooks executes PreHooks in registration order.
// Stops early if a plugin returns a ShortCircuit.
func (p *PluginPipeline) RunPreHooks(ctx *RelayContext) PreHookResult {
	for i, plugin := range p.plugins {
		sc := plugin.PreHook(ctx)
		if sc != nil {
			log.Printf("[plugin] %s short-circuited request", plugin.Name())
			return PreHookResult{
				ShortCircuit:  sc,
				ExecutedCount: i + 1,
			}
		}
	}
	return PreHookResult{ExecutedCount: len(p.plugins)}
}

// RunPostHooks executes PostHooks in REVERSE order, only for the
// first `executedCount` plugins (symmetry guarantee).
func (p *PluginPipeline) RunPostHooks(ctx *RelayContext, resp *RelayPluginResponse, executedCount int) {
	if executedCount > len(p.plugins) {
		executedCount = len(p.plugins)
	}
	for i := executedCount - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[plugin] %s PostHook panicked: %v", p.plugins[i].Name(), r)
				}
			}()
			p.plugins[i].PostHook(ctx, resp)
		}()
	}
}
