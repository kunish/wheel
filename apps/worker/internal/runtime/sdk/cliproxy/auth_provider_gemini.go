package cliproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	managementHandlers "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/api/handlers/management"
	geminiAuth "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/auth/gemini"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/util"

	"github.com/tidwall/gjson"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// --- Gemini CLI Auth Provider Seam ---

// GeminiTokenStorage is the persistent token storage for Gemini credentials.
type GeminiTokenStorage = geminiAuth.GeminiTokenStorage

// GeminiAuthProvider wraps the Gemini CLI auth flow for first-party use.
type GeminiAuthProvider struct {
	cfg *config.Config
}

// NewGeminiAuthProvider creates a new Gemini auth provider.
func NewGeminiAuthProvider(cfg *config.Config) *GeminiAuthProvider {
	return &GeminiAuthProvider{cfg: cfg}
}

// GeminiCallbackPort is the port the Gemini CLI expects for OAuth redirect.
const GeminiCallbackPort = geminiAuth.DefaultCallbackPort

// GeminiOAuthConfig returns a configured oauth2.Config for Gemini CLI auth.
func (p *GeminiAuthProvider) GeminiOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     geminiAuth.ClientID,
		ClientSecret: geminiAuth.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/oauth2callback", geminiAuth.DefaultCallbackPort),
		Scopes:       geminiAuth.Scopes,
		Endpoint:     google.Endpoint,
	}
}

// PrepareOAuthContext creates a context with the proxy HTTP client for OAuth2 operations.
func (p *GeminiAuthProvider) PrepareOAuthContext(ctx context.Context) context.Context {
	proxyHTTPClient := util.SetProxy(&p.cfg.SDKConfig, &http.Client{})
	return context.WithValue(ctx, oauth2.HTTPClient, proxyHTTPClient)
}

// GeminiExchangeResult contains the result of the Gemini token exchange and setup.
type GeminiExchangeResult struct {
	Storage  *GeminiTokenStorage
	FileName string
	Metadata map[string]any
}

// ExchangeAndSetup performs the entire Gemini CLI auth flow after receiving an auth code:
// 1. Exchanges code for token via OAuth2
// 2. Fetches user email from Google API
// 3. Builds GeminiTokenStorage with enriched token data
// 4. Performs GCP project onboarding (loadCodeAssist / onboardUser)
// 5. Verifies Cloud AI API is enabled
//
// The requestedProjectID parameter controls project selection:
// - "" or specific ID: auto-discover or use specific project
// - "ALL": onboard all available projects
// - "GOOGLE_ONE": Google One auto-discovery
func (p *GeminiAuthProvider) ExchangeAndSetup(ctx context.Context, conf *oauth2.Config, authCode, requestedProjectID string) (*GeminiExchangeResult, error) {
	// Exchange authorization code for token
	token, err := conf.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	requestedProjectID = strings.TrimSpace(requestedProjectID)

	// Fetch user email
	authHTTPClient := conf.Client(ctx, token)
	email, errEmail := fetchGeminiUserEmail(ctx, authHTTPClient, token.AccessToken)
	if errEmail != nil {
		return nil, errEmail
	}

	// Build token map (oauth2.Token -> map[string]any with enriched fields)
	var ifToken map[string]any
	jsonData, _ := json.Marshal(token)
	if errUnmarshal := json.Unmarshal(jsonData, &ifToken); errUnmarshal != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", errUnmarshal)
	}
	ifToken["token_uri"] = "https://oauth2.googleapis.com/token"
	ifToken["client_id"] = geminiAuth.ClientID
	ifToken["client_secret"] = geminiAuth.ClientSecret
	ifToken["scopes"] = geminiAuth.Scopes
	ifToken["universe_domain"] = "googleapis.com"

	ts := &geminiAuth.GeminiTokenStorage{
		Token:     ifToken,
		ProjectID: requestedProjectID,
		Email:     email,
		Auto:      requestedProjectID == "",
	}

	// Get authenticated client for GCP API calls
	gemAuth := geminiAuth.NewGeminiAuth()
	gemClient, errClient := gemAuth.GetAuthenticatedClient(ctx, ts, p.cfg, &geminiAuth.WebLoginOptions{
		NoBrowser: true,
	})
	if errClient != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", errClient)
	}

	// Perform GCP project setup via the vendored management helper
	if errSetup := performGeminiSetup(ctx, gemClient, ts, requestedProjectID); errSetup != nil {
		return nil, errSetup
	}

	fileName := geminiAuth.CredentialFileName(ts.Email, ts.ProjectID, true)
	metadata := map[string]any{
		"email":      ts.Email,
		"project_id": ts.ProjectID,
		"auto":       ts.Auto,
		"checked":    ts.Checked,
	}

	return &GeminiExchangeResult{
		Storage:  ts,
		FileName: fileName,
		Metadata: metadata,
	}, nil
}

// fetchGeminiUserEmail retrieves the user's email from the Google user info endpoint.
func fetchGeminiUserEmail(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v1/userinfo?alt=json", nil)
	if err != nil {
		return "", fmt.Errorf("could not get user info: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get user info request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	email := gjson.GetBytes(bodyBytes, "email").String()
	return email, nil
}

// performGeminiSetup delegates to the vendored management package's GCP project setup functions.
// It directly calls the exported GeminiSetupFunc() from the management package, which wraps
// the unexported helpers (ensureGeminiProjectAndOnboard, onboardAllGeminiProjects, etc.).
func performGeminiSetup(ctx context.Context, client *http.Client, ts *geminiAuth.GeminiTokenStorage, requestedProjectID string) error {
	setupFn := managementHandlers.GeminiSetupFunc()
	return setupFn(ctx, client, ts, requestedProjectID)
}
