package cliproxy

import (
	"context"
	"net/http"

	"github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/corelib/wsrelay"
)

type vendoredWebsocketGateway struct {
	manager *wsrelay.Manager
}

func (g *vendoredWebsocketGateway) Path() string {
	if g == nil || g.manager == nil {
		return "/v1/ws"
	}
	return g.manager.Path()
}

func (g *vendoredWebsocketGateway) Handler() http.Handler {
	if g == nil || g.manager == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	return g.manager.Handler()
}

func (g *vendoredWebsocketGateway) DisconnectAll(ctx context.Context) error {
	if g == nil || g.manager == nil {
		return nil
	}
	return g.manager.Stop(ctx)
}

func (g *vendoredWebsocketGateway) NonStream(ctx context.Context, provider string, req *wsrelay.HTTPRequest) (*wsrelay.HTTPResponse, error) {
	if g == nil || g.manager == nil {
		return nil, nil
	}
	return g.manager.NonStream(ctx, provider, req)
}

func (g *vendoredWebsocketGateway) Stream(ctx context.Context, provider string, req *wsrelay.HTTPRequest) (<-chan wsrelay.StreamEvent, error) {
	if g == nil || g.manager == nil {
		return nil, nil
	}
	return g.manager.Stream(ctx, provider, req)
}

func defaultWebsocketGatewayFactory(opts WebsocketGatewayOptions) WebsocketGateway {
	return &vendoredWebsocketGateway{manager: wsrelay.NewManager(wsrelay.Options{
		Path:           opts.Path,
		OnConnected:    opts.OnConnected,
		OnDisconnected: opts.OnDisconnected,
		LogDebugf:      opts.LogDebugf,
		LogInfof:       opts.LogInfof,
		LogWarnf:       opts.LogWarnf,
	})}
}

// DefaultWebsocketGatewayFactory exposes the SDK default websocket gateway factory for host-owned seams.
func DefaultWebsocketGatewayFactory(opts WebsocketGatewayOptions) WebsocketGateway {
	return defaultWebsocketGatewayFactory(opts)
}
