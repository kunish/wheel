package config

// AmpModelMapping defines a model name mapping for Amp CLI requests.
// When Amp requests a model that isn't available locally, this mapping
// allows routing to an alternative model that IS available.
type AmpModelMapping struct {
	// From is the model name that Amp CLI requests (e.g., "claude-opus-4.5").
	From string `yaml:"from" json:"from"`

	// To is the target model name to route to (e.g., "claude-sonnet-4").
	// The target model must have available providers in the registry.
	To string `yaml:"to" json:"to"`

	// Regex indicates whether the 'from' field should be interpreted as a regular
	// expression for matching model names. When true, this mapping is evaluated
	// after exact matches and in the order provided. Defaults to false (exact match).
	Regex bool `yaml:"regex,omitempty" json:"regex,omitempty"`
}

// AmpCode groups Amp CLI integration settings including upstream routing,
// optional overrides, management route restrictions, and model fallback mappings.
type AmpCode struct {
	// UpstreamURL defines the upstream Amp control plane used for non-provider calls.
	UpstreamURL string `yaml:"upstream-url" json:"upstream-url"`

	// UpstreamAPIKey optionally overrides the Authorization header when proxying Amp upstream calls.
	UpstreamAPIKey string `yaml:"upstream-api-key" json:"upstream-api-key"`

	// UpstreamAPIKeys maps client API keys (from top-level api-keys) to upstream API keys.
	// When a client authenticates with a key that matches an entry, that upstream key is used.
	// If no match is found, falls back to UpstreamAPIKey (default behavior).
	UpstreamAPIKeys []AmpUpstreamAPIKeyEntry `yaml:"upstream-api-keys,omitempty" json:"upstream-api-keys,omitempty"`

	// RestrictManagementToLocalhost restricts Amp management routes (/api/user, /api/threads, etc.)
	// to only accept connections from localhost (127.0.0.1, ::1). When true, prevents drive-by
	// browser attacks and remote access to management endpoints. Default: false (API key auth is sufficient).
	RestrictManagementToLocalhost bool `yaml:"restrict-management-to-localhost" json:"restrict-management-to-localhost"`

	// ModelMappings defines model name mappings for Amp CLI requests.
	// When Amp requests a model that isn't available locally, these mappings
	// allow routing to an alternative model that IS available.
	ModelMappings []AmpModelMapping `yaml:"model-mappings" json:"model-mappings"`

	// ForceModelMappings when true, model mappings take precedence over local API keys.
	// When false (default), local API keys are used first if available.
	ForceModelMappings bool `yaml:"force-model-mappings" json:"force-model-mappings"`
}

// AmpUpstreamAPIKeyEntry maps a set of client API keys to a specific upstream API key.
// When a request is authenticated with one of the APIKeys, the corresponding UpstreamAPIKey
// is used for the upstream Amp request.
type AmpUpstreamAPIKeyEntry struct {
	// UpstreamAPIKey is the API key to use when proxying to the Amp upstream.
	UpstreamAPIKey string `yaml:"upstream-api-key" json:"upstream-api-key"`

	// APIKeys are the client API keys (from top-level api-keys) that map to this upstream key.
	APIKeys []string `yaml:"api-keys" json:"api-keys"`
}

// PayloadConfig defines default and override parameter rules applied to provider payloads.
type PayloadConfig struct {
	// Default defines rules that only set parameters when they are missing in the payload.
	Default []PayloadRule `yaml:"default" json:"default"`
	// DefaultRaw defines rules that set raw JSON values only when they are missing.
	DefaultRaw []PayloadRule `yaml:"default-raw" json:"default-raw"`
	// Override defines rules that always set parameters, overwriting any existing values.
	Override []PayloadRule `yaml:"override" json:"override"`
	// OverrideRaw defines rules that always set raw JSON values, overwriting any existing values.
	OverrideRaw []PayloadRule `yaml:"override-raw" json:"override-raw"`
	// Filter defines rules that remove parameters from the payload by JSON path.
	Filter []PayloadFilterRule `yaml:"filter" json:"filter"`
}

// PayloadFilterRule describes a rule to remove specific JSON paths from matching model payloads.
type PayloadFilterRule struct {
	// Models lists model entries with name pattern and protocol constraint.
	Models []PayloadModelRule `yaml:"models" json:"models"`
	// Params lists JSON paths (gjson/sjson syntax) to remove from the payload.
	Params []string `yaml:"params" json:"params"`
}

// PayloadRule describes a single rule targeting a list of models with parameter updates.
type PayloadRule struct {
	// Models lists model entries with name pattern and protocol constraint.
	Models []PayloadModelRule `yaml:"models" json:"models"`
	// Params maps JSON paths (gjson/sjson syntax) to values written into the payload.
	// For *-raw rules, values are treated as raw JSON fragments (strings are used as-is).
	Params map[string]any `yaml:"params" json:"params"`
}

// PayloadModelRule ties a model name pattern to a specific translator protocol.
type PayloadModelRule struct {
	// Name is the model name or wildcard pattern (e.g., "gpt-*", "*-5", "gemini-*-pro").
	Name string `yaml:"name" json:"name"`
	// Protocol restricts the rule to a specific translator format (e.g., "gemini", "responses").
	Protocol string `yaml:"protocol" json:"protocol"`
}

// CloakConfig configures request cloaking for non-Claude-Code clients.
// Cloaking disguises API requests to appear as originating from the official Claude Code CLI.
type CloakConfig struct {
	// Mode controls cloaking behavior: "auto" (default), "always", or "never".
	// - "auto": cloak only when client is not Claude Code (based on User-Agent)
	// - "always": always apply cloaking regardless of client
	// - "never": never apply cloaking
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

	// StrictMode controls how system prompts are handled when cloaking.
	// - false (default): prepend Claude Code prompt to user system messages
	// - true: strip all user system messages, keep only Claude Code prompt
	StrictMode bool `yaml:"strict-mode,omitempty" json:"strict-mode,omitempty"`

	// SensitiveWords is a list of words to obfuscate with zero-width characters.
	// This can help bypass certain content filters.
	SensitiveWords []string `yaml:"sensitive-words,omitempty" json:"sensitive-words,omitempty"`

	// CacheUserID controls whether Claude user_id values are cached per API key.
	// When false, a fresh random user_id is generated for every request.
	CacheUserID *bool `yaml:"cache-user-id,omitempty" json:"cache-user-id,omitempty"`
}

// ClaudeKey represents the configuration for a Claude API key,
// including the API key itself and an optional base URL for the API endpoint.
type ClaudeKey struct {
	// APIKey is the authentication key for accessing Claude API services.
	APIKey string `yaml:"api-key" json:"api-key"`

	// Priority controls selection preference when multiple credentials match.
	// Higher values are preferred; defaults to 0.
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Prefix optionally namespaces models for this credential (e.g., "teamA/claude-sonnet-4").
	Prefix string `yaml:"prefix,omitempty" json:"prefix,omitempty"`

	// BaseURL is the base URL for the Claude API endpoint.
	// If empty, the default Claude API URL will be used.
	BaseURL string `yaml:"base-url" json:"base-url"`

	// ProxyURL overrides the global proxy setting for this API key if provided.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// Models defines upstream model names and aliases for request routing.
	Models []ClaudeModel `yaml:"models" json:"models"`

	// Headers optionally adds extra HTTP headers for requests sent with this key.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// ExcludedModels lists model IDs that should be excluded for this provider.
	ExcludedModels []string `yaml:"excluded-models,omitempty" json:"excluded-models,omitempty"`

	// Cloak configures request cloaking for non-Claude-Code clients.
	Cloak *CloakConfig `yaml:"cloak,omitempty" json:"cloak,omitempty"`
}

func (k ClaudeKey) GetAPIKey() string  { return k.APIKey }
func (k ClaudeKey) GetBaseURL() string { return k.BaseURL }

// ClaudeModel describes a mapping between an alias and the actual upstream model name.
type ClaudeModel struct {
	// Name is the upstream model identifier used when issuing requests.
	Name string `yaml:"name" json:"name"`

	// Alias is the client-facing model name that maps to Name.
	Alias string `yaml:"alias" json:"alias"`
}

func (m ClaudeModel) GetName() string  { return m.Name }
func (m ClaudeModel) GetAlias() string { return m.Alias }

// CodexKey represents the configuration for a Codex API key,
// including the API key itself and an optional base URL for the API endpoint.
type CodexKey struct {
	// APIKey is the authentication key for accessing Codex API services.
	APIKey string `yaml:"api-key" json:"api-key"`

	// Priority controls selection preference when multiple credentials match.
	// Higher values are preferred; defaults to 0.
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Prefix optionally namespaces models for this credential (e.g., "teamA/gpt-5-codex").
	Prefix string `yaml:"prefix,omitempty" json:"prefix,omitempty"`

	// BaseURL is the base URL for the Codex API endpoint.
	// If empty, the default Codex API URL will be used.
	BaseURL string `yaml:"base-url" json:"base-url"`

	// Websockets enables the Responses API websocket transport for this credential.
	Websockets bool `yaml:"websockets,omitempty" json:"websockets,omitempty"`

	// ProxyURL overrides the global proxy setting for this API key if provided.
	ProxyURL string `yaml:"proxy-url" json:"proxy-url"`

	// Models defines upstream model names and aliases for request routing.
	Models []CodexModel `yaml:"models" json:"models"`

	// Headers optionally adds extra HTTP headers for requests sent with this key.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// ExcludedModels lists model IDs that should be excluded for this provider.
	ExcludedModels []string `yaml:"excluded-models,omitempty" json:"excluded-models,omitempty"`
}

func (k CodexKey) GetAPIKey() string  { return k.APIKey }
func (k CodexKey) GetBaseURL() string { return k.BaseURL }

// CodexModel describes a mapping between an alias and the actual upstream model name.
type CodexModel struct {
	// Name is the upstream model identifier used when issuing requests.
	Name string `yaml:"name" json:"name"`

	// Alias is the client-facing model name that maps to Name.
	Alias string `yaml:"alias" json:"alias"`
}

func (m CodexModel) GetName() string  { return m.Name }
func (m CodexModel) GetAlias() string { return m.Alias }

// GeminiKey represents the configuration for a Gemini API key,
// including optional overrides for upstream base URL, proxy routing, and headers.
type GeminiKey struct {
	// APIKey is the authentication key for accessing Gemini API services.
	APIKey string `yaml:"api-key" json:"api-key"`

	// Priority controls selection preference when multiple credentials match.
	// Higher values are preferred; defaults to 0.
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Prefix optionally namespaces models for this credential (e.g., "teamA/gemini-3-pro-preview").
	Prefix string `yaml:"prefix,omitempty" json:"prefix,omitempty"`

	// BaseURL optionally overrides the Gemini API endpoint.
	BaseURL string `yaml:"base-url,omitempty" json:"base-url,omitempty"`

	// ProxyURL optionally overrides the global proxy for this API key.
	ProxyURL string `yaml:"proxy-url,omitempty" json:"proxy-url,omitempty"`

	// Models defines upstream model names and aliases for request routing.
	Models []GeminiModel `yaml:"models,omitempty" json:"models,omitempty"`

	// Headers optionally adds extra HTTP headers for requests sent with this key.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// ExcludedModels lists model IDs that should be excluded for this provider.
	ExcludedModels []string `yaml:"excluded-models,omitempty" json:"excluded-models,omitempty"`
}

func (k GeminiKey) GetAPIKey() string  { return k.APIKey }
func (k GeminiKey) GetBaseURL() string { return k.BaseURL }

// GeminiModel describes a mapping between an alias and the actual upstream model name.
type GeminiModel struct {
	// Name is the upstream model identifier used when issuing requests.
	Name string `yaml:"name" json:"name"`

	// Alias is the client-facing model name that maps to Name.
	Alias string `yaml:"alias" json:"alias"`
}

func (m GeminiModel) GetName() string  { return m.Name }
func (m GeminiModel) GetAlias() string { return m.Alias }

// KiroKey represents the configuration for Kiro (AWS CodeWhisperer) authentication.
type KiroKey struct {
	// TokenFile is the path to the Kiro token file (default: ~/.aws/sso/cache/kiro-auth-token.json)
	TokenFile string `yaml:"token-file,omitempty" json:"token-file,omitempty"`

	// AccessToken is the OAuth access token for direct configuration.
	AccessToken string `yaml:"access-token,omitempty" json:"access-token,omitempty"`

	// RefreshToken is the OAuth refresh token for token renewal.
	RefreshToken string `yaml:"refresh-token,omitempty" json:"refresh-token,omitempty"`

	// ProfileArn is the AWS CodeWhisperer profile ARN.
	ProfileArn string `yaml:"profile-arn,omitempty" json:"profile-arn,omitempty"`

	// Region is the AWS region (default: us-east-1).
	Region string `yaml:"region,omitempty" json:"region,omitempty"`

	// StartURL is the IAM Identity Center (IDC) start URL for SSO login.
	StartURL string `yaml:"start-url,omitempty" json:"start-url,omitempty"`

	// ProxyURL optionally overrides the global proxy for this configuration.
	ProxyURL string `yaml:"proxy-url,omitempty" json:"proxy-url,omitempty"`

	// AgentTaskType sets the Kiro API task type. Known values: "vibe", "dev", "chat".
	// Leave empty to let API use defaults. Different values may inject different system prompts.
	AgentTaskType string `yaml:"agent-task-type,omitempty" json:"agent-task-type,omitempty"`

	// PreferredEndpoint sets the preferred Kiro API endpoint/quota.
	// Values: "codewhisperer" (default, IDE quota) or "amazonq" (CLI quota).
	PreferredEndpoint string `yaml:"preferred-endpoint,omitempty" json:"preferred-endpoint,omitempty"`
}

// KiroFingerprintConfig defines a global fingerprint configuration for Kiro requests.
// When configured, all Kiro requests will use this fixed fingerprint instead of random generation.
// Empty fields will fall back to random selection from built-in pools.
type KiroFingerprintConfig struct {
	OIDCSDKVersion      string `yaml:"oidc-sdk-version,omitempty" json:"oidc-sdk-version,omitempty"`
	RuntimeSDKVersion   string `yaml:"runtime-sdk-version,omitempty" json:"runtime-sdk-version,omitempty"`
	StreamingSDKVersion string `yaml:"streaming-sdk-version,omitempty" json:"streaming-sdk-version,omitempty"`
	OSType              string `yaml:"os-type,omitempty" json:"os-type,omitempty"`
	OSVersion           string `yaml:"os-version,omitempty" json:"os-version,omitempty"`
	NodeVersion         string `yaml:"node-version,omitempty" json:"node-version,omitempty"`
	KiroVersion         string `yaml:"kiro-version,omitempty" json:"kiro-version,omitempty"`
	KiroHash            string `yaml:"kiro-hash,omitempty" json:"kiro-hash,omitempty"`
}

// OpenAICompatibility represents the configuration for OpenAI API compatibility
// with external providers, allowing model aliases to be routed through OpenAI API format.
type OpenAICompatibility struct {
	// Name is the identifier for this OpenAI compatibility configuration.
	Name string `yaml:"name" json:"name"`

	// Priority controls selection preference when multiple providers or credentials match.
	// Higher values are preferred; defaults to 0.
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Prefix optionally namespaces model aliases for this provider (e.g., "teamA/kimi-k2").
	Prefix string `yaml:"prefix,omitempty" json:"prefix,omitempty"`

	// BaseURL is the base URL for the external OpenAI-compatible API endpoint.
	BaseURL string `yaml:"base-url" json:"base-url"`

	// APIKeyEntries defines API keys with optional per-key proxy configuration.
	APIKeyEntries []OpenAICompatibilityAPIKey `yaml:"api-key-entries,omitempty" json:"api-key-entries,omitempty"`

	// Models defines the model configurations including aliases for routing.
	Models []OpenAICompatibilityModel `yaml:"models" json:"models"`

	// Headers optionally adds extra HTTP headers for requests sent to this provider.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// OpenAICompatibilityAPIKey represents an API key configuration with optional proxy setting.
type OpenAICompatibilityAPIKey struct {
	// APIKey is the authentication key for accessing the external API services.
	APIKey string `yaml:"api-key" json:"api-key"`

	// ProxyURL overrides the global proxy setting for this API key if provided.
	ProxyURL string `yaml:"proxy-url,omitempty" json:"proxy-url,omitempty"`
}

// OpenAICompatibilityModel represents a model configuration for OpenAI compatibility,
// including the actual model name and its alias for API routing.
type OpenAICompatibilityModel struct {
	// Name is the actual model name used by the external provider.
	Name string `yaml:"name" json:"name"`

	// Alias is the model name alias that clients will use to reference this model.
	Alias string `yaml:"alias" json:"alias"`
}

func (m OpenAICompatibilityModel) GetName() string  { return m.Name }
func (m OpenAICompatibilityModel) GetAlias() string { return m.Alias }
