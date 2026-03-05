package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ──── OAuth Token ────

// OAuthToken holds a cached OAuth access token with expiry information.
type OAuthToken struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
	IsStatic    bool // True for pre-configured access tokens that never expire
}

// IsExpired returns true if the token is expired or about to expire (within 30s buffer).
// Static tokens (pre-configured access tokens) never expire.
func (t *OAuthToken) IsExpired() bool {
	if t == nil {
		return true
	}
	if t.IsStatic {
		return false
	}
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// ──── OAuth Metadata Discovery ────

// OAuthServerMetadata represents the OAuth 2.0 Authorization Server Metadata (RFC 8414).
type OAuthServerMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint,omitempty"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
	GrantTypesSupported   []string `json:"grant_types_supported,omitempty"`
}

// ProtectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC 9728).
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

// OAuthDiscoveryResult holds the combined result of OAuth metadata discovery.
type OAuthDiscoveryResult struct {
	ServerMetadata   *OAuthServerMetadata       `json:"serverMetadata,omitempty"`
	ResourceMetadata *ProtectedResourceMetadata `json:"resourceMetadata,omitempty"`
	TokenURL         string                     `json:"tokenUrl,omitempty"`
	AuthorizationURL string                     `json:"authorizationUrl,omitempty"`
	RegistrationURL  string                     `json:"registrationUrl,omitempty"`
	Scopes           []string                   `json:"scopes,omitempty"`
}

var discoveryHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		// Validate redirect target is not an internal address
		if err := validateExternalURL(req.URL.String()); err != nil {
			return err
		}
		return nil
	},
}

// validateExternalURL checks that a URL does not point to internal/private network addresses.
func validateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow http and https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	host := parsed.Hostname()

	// Block localhost variants
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("internal addresses are not allowed: %s", host)
	}

	// Resolve and check for private IP ranges
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("internal addresses are not allowed: %s resolves to %s", host, ipStr)
		}
	}

	return nil
}

// DiscoverOAuthMetadata discovers OAuth metadata for a given MCP server URL.
// It first tries RFC 9728 (Protected Resource Metadata), then RFC 8414 (Authorization Server Metadata).
func DiscoverOAuthMetadata(serverURL string) (*OAuthDiscoveryResult, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Validate the target URL is not an internal address (SSRF prevention)
	if err := validateExternalURL(serverURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	result := &OAuthDiscoveryResult{}

	// 1. Try RFC 9728: /.well-known/oauth-protected-resource
	resourceURL := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource", parsed.Scheme, parsed.Host)
	resourceMeta, err := fetchJSON[ProtectedResourceMetadata](resourceURL)
	if err == nil && resourceMeta != nil {
		result.ResourceMetadata = resourceMeta
		if len(resourceMeta.ScopesSupported) > 0 {
			result.Scopes = resourceMeta.ScopesSupported
		}
		// Use the first authorization server to fetch server metadata
		if len(resourceMeta.AuthorizationServers) > 0 {
			authServerURL := resourceMeta.AuthorizationServers[0]
			serverMeta, err := fetchAuthServerMetadata(authServerURL)
			if err == nil && serverMeta != nil {
				result.ServerMetadata = serverMeta
				result.TokenURL = serverMeta.TokenEndpoint
				result.AuthorizationURL = serverMeta.AuthorizationEndpoint
				result.RegistrationURL = serverMeta.RegistrationEndpoint
				if len(result.Scopes) == 0 && len(serverMeta.ScopesSupported) > 0 {
					result.Scopes = serverMeta.ScopesSupported
				}
			}
		}
	}

	// 2. If no token URL yet, try RFC 8414 directly on the server
	if result.TokenURL == "" {
		serverMeta, err := fetchAuthServerMetadata(fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host))
		if err == nil && serverMeta != nil {
			result.ServerMetadata = serverMeta
			result.TokenURL = serverMeta.TokenEndpoint
			result.AuthorizationURL = serverMeta.AuthorizationEndpoint
			result.RegistrationURL = serverMeta.RegistrationEndpoint
			if len(result.Scopes) == 0 && len(serverMeta.ScopesSupported) > 0 {
				result.Scopes = serverMeta.ScopesSupported
			}
		}
	}

	if result.TokenURL == "" {
		return nil, fmt.Errorf("no OAuth metadata found at %s", serverURL)
	}

	return result, nil
}

// fetchAuthServerMetadata fetches RFC 8414 metadata from a base URL.
func fetchAuthServerMetadata(baseURL string) (*OAuthServerMetadata, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	metaURL := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsed.Scheme, parsed.Host)
	return fetchJSON[OAuthServerMetadata](metaURL)
}

// fetchJSON fetches a URL and decodes the response as JSON.
func fetchJSON[T any](targetURL string) (*T, error) {
	resp, err := discoveryHTTPClient.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, targetURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON decode error from %s: %w", targetURL, err)
	}
	return &result, nil
}

// ──── Token Acquisition ────

// tokenResponse is the OAuth 2.0 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // Seconds until expiry
	Scope       string `json:"scope,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// AcquireToken obtains an OAuth token using client_credentials grant.
// If the config has a pre-configured AccessToken, it returns that directly.
func AcquireToken(cfg *MCPOAuthConfig) (*OAuthToken, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OAuth config is nil")
	}

	// If a static access token is pre-configured, use it directly
	if cfg.AccessToken != "" {
		return &OAuthToken{
			AccessToken: cfg.AccessToken,
			TokenType:   "Bearer",
			IsStatic:    true,
		}, nil
	}

	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("token URL is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	// Build client_credentials request
	data := url.Values{
		"grant_type": {"client_credentials"},
		"client_id":  {cfg.ClientID},
	}
	if cfg.ClientSecret != "" {
		data.Set("client_secret", cfg.ClientSecret)
	}
	if cfg.Scopes != "" {
		data.Set("scope", cfg.Scopes)
	}

	resp, err := discoveryHTTPClient.PostForm(cfg.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("token error: %s — %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // Default 1 hour if not specified
	}

	tokenType := tokenResp.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	} else {
		// Normalize: "bearer" -> "Bearer", "BEARER" -> "Bearer"
		lower := strings.ToLower(tokenType)
		tokenType = strings.ToUpper(lower[:1]) + lower[1:]
	}

	return &OAuthToken{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenType,
		ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

// ──── Token Manager ────

// TokenManager provides thread-safe token caching and auto-refresh.
type TokenManager struct {
	mu     sync.RWMutex
	tokens map[int]*OAuthToken // keyed by MCPClient ID
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager() *TokenManager {
	return &TokenManager{
		tokens: make(map[int]*OAuthToken),
	}
}

// GetToken returns a valid token for the given client, acquiring or refreshing as needed.
func (tm *TokenManager) GetToken(clientID int, cfg *MCPOAuthConfig) (*OAuthToken, error) {
	tm.mu.RLock()
	token, ok := tm.tokens[clientID]
	tm.mu.RUnlock()

	if ok && !token.IsExpired() {
		return token, nil
	}

	// Need to acquire/refresh
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if token, ok := tm.tokens[clientID]; ok && !token.IsExpired() {
		return token, nil
	}

	newToken, err := AcquireToken(cfg)
	if err != nil {
		return nil, err
	}

	tm.tokens[clientID] = newToken
	return newToken, nil
}

// InvalidateToken removes the cached token for a client.
func (tm *TokenManager) InvalidateToken(clientID int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, clientID)
}

// GetBearerToken returns the Authorization header value ("<tokenType> <accessToken>").
// Callers should only use this for OAuth-enabled clients.
func (tm *TokenManager) GetBearerToken(clientID int, cfg *MCPOAuthConfig) (string, error) {
	token, err := tm.GetToken(clientID, cfg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", token.TokenType, token.AccessToken), nil
}
