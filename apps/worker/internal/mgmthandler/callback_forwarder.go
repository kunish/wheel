package mgmthandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	log "github.com/sirupsen/logrus"
)

// callbackForwarder is a lightweight HTTP server that binds a provider's
// expected OAuth redirect port and 302-redirects every request to the
// management server's own callback endpoint.
type callbackForwarder struct {
	provider string
	server   *http.Server
	done     chan struct{}
}

var (
	callbackForwardersMu sync.Mutex
	callbackForwarders   = make(map[int]*callbackForwarder)
)

// startCallbackForwarder spins up a redirect-only HTTP server on the
// given port. Any previous forwarder on the same port is stopped first.
// It listens on both IPv4 (127.0.0.1) and IPv6 ([::1]) loopback addresses
// to handle browsers that resolve "localhost" to either address.
func startCallbackForwarder(port int, provider, targetBase string) (*callbackForwarder, error) {
	callbackForwardersMu.Lock()
	prev := callbackForwarders[port]
	if prev != nil {
		delete(callbackForwarders, port)
	}
	callbackForwardersMu.Unlock()

	if prev != nil {
		stopForwarderInstance(port, prev)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := targetBase
		if raw := r.URL.RawQuery; raw != "" {
			if strings.Contains(target, "?") {
				target = target + "&" + raw
			} else {
				target = target + "?" + raw
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusFound)
	})

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}
	done := make(chan struct{})

	// Listen on IPv4 loopback.
	ipv4Addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln4, err4 := net.Listen("tcp4", ipv4Addr)

	// Also try IPv6 loopback so that browsers resolving "localhost" to ::1 can connect.
	ipv6Addr := fmt.Sprintf("[::1]:%d", port)
	ln6, err6 := net.Listen("tcp6", ipv6Addr)

	if err4 != nil && err6 != nil {
		return nil, fmt.Errorf("failed to listen on %s and %s: %w", ipv4Addr, ipv6Addr, err4)
	}

	// Combine both listeners into a single logical listener.
	combinedDone := make(chan struct{})
	var serveWg sync.WaitGroup

	if ln4 != nil {
		serveWg.Add(1)
		go func() {
			defer serveWg.Done()
			if errServe := srv.Serve(ln4); errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
				log.Warnf("callback forwarder for %s (ipv4) stopped unexpectedly: %v", provider, errServe)
			}
		}()
	}
	if ln6 != nil {
		serveWg.Add(1)
		go func() {
			defer serveWg.Done()
			if errServe := srv.Serve(ln6); errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
				log.Warnf("callback forwarder for %s (ipv6) stopped unexpectedly: %v", provider, errServe)
			}
		}()
	}

	go func() {
		serveWg.Wait()
		close(combinedDone)
		close(done)
	}()

	forwarder := &callbackForwarder{
		provider: provider,
		server:   srv,
		done:     done,
	}

	callbackForwardersMu.Lock()
	callbackForwarders[port] = forwarder
	callbackForwardersMu.Unlock()

	var listenAddrs []string
	if ln4 != nil {
		listenAddrs = append(listenAddrs, ipv4Addr)
	}
	if ln6 != nil {
		listenAddrs = append(listenAddrs, ipv6Addr)
	}
	log.Infof("callback forwarder for %s listening on %s", provider, strings.Join(listenAddrs, " and "))
	return forwarder, nil
}

// stopCallbackForwarderInstance removes the forwarder from the global map
// and gracefully shuts it down.
func stopCallbackForwarderInstance(port int, forwarder *callbackForwarder) {
	if forwarder == nil {
		return
	}
	callbackForwardersMu.Lock()
	if current := callbackForwarders[port]; current == forwarder {
		delete(callbackForwarders, port)
	}
	callbackForwardersMu.Unlock()

	stopForwarderInstance(port, forwarder)
}

func stopForwarderInstance(port int, forwarder *callbackForwarder) {
	if forwarder == nil || forwarder.server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := forwarder.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Warnf("failed to shut down callback forwarder on port %d: %v", port, err)
	}

	select {
	case <-forwarder.done:
	case <-time.After(2 * time.Second):
	}

	log.Infof("callback forwarder on port %d stopped", port)
}

// isWebUIRequest checks whether the request includes the is_webui query
// parameter set to a truthy value.
func isWebUIRequest(c interface{ Query(string) string }) bool {
	raw := strings.TrimSpace(c.Query("is_webui"))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// managementCallbackURL constructs the full callback URL for the
// management server given a path like "/codex/callback".
func managementCallbackURL(port int, tlsEnabled bool, path string) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("server port is not configured")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return fmt.Sprintf("%s://localhost:%d%s", scheme, port, path), nil
}

// callbackFileResult holds the parsed contents of an OAuth callback file.
type callbackFileResult struct {
	Code  string
	State string
	Error string
}

// pollForCallbackFile waits for the OAuth callback file to appear on disk,
// polling every 500ms with a 5-minute timeout. Returns the parsed result,
// or an error if the session expires, times out, or the callback reports an error.
func pollForCallbackFile(authDir, provider, state string) (*callbackFileResult, error) {
	waitFile := filepath.Join(authDir, fmt.Sprintf(".oauth-%s-%s.oauth", provider, state))
	deadline := time.Now().Add(5 * time.Minute)

	for {
		if !sdkcliproxy.IsOAuthSessionPending(state, provider) {
			return nil, fmt.Errorf("oauth session cancelled")
		}
		if time.Now().After(deadline) {
			sdkcliproxy.SetOAuthSessionError(state, "Timeout waiting for OAuth callback")
			return nil, fmt.Errorf("timeout waiting for OAuth callback")
		}
		if data, err := os.ReadFile(waitFile); err == nil {
			var m map[string]string
			_ = json.Unmarshal(data, &m)
			_ = os.Remove(waitFile)

			if errStr := m["error"]; errStr != "" {
				sdkcliproxy.SetOAuthSessionError(state, "Bad Request")
				return nil, fmt.Errorf("oauth error: %s", errStr)
			}
			if m["state"] != state {
				sdkcliproxy.SetOAuthSessionError(state, "State code error")
				return nil, fmt.Errorf("state mismatch: expected %s, got %s", state, m["state"])
			}
			return &callbackFileResult{
				Code:  m["code"],
				State: m["state"],
			}, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}
