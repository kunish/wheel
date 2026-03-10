// Package config provides configuration management for the CLI Proxy API server.
// It handles loading and parsing YAML configuration files, and provides structured
// access to application settings including server port, authentication directory,
// debug settings, proxy configuration, and API keys.
package config

import (
	"fmt"

	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
	"gopkg.in/yaml.v3"
)

const (
	DefaultPanelGitHubRepository = "https://github.com/router-for-me/Cli-Proxy-API-Management-Center"
	DefaultPprofAddr             = "127.0.0.1:8316"
)

// Config represents the application's configuration, loaded from a YAML file.
type Config struct {
	SDKConfig `yaml:",inline"`
	// Host is the network host/interface on which the API server will bind.
	// Default is empty ("") to bind all interfaces (IPv4 + IPv6). Use "127.0.0.1" or "localhost" for local-only access.
	Host string `yaml:"host" json:"-"`
	// Port is the network port on which the API server will listen.
	Port int `yaml:"port" json:"-"`

	// TLS config controls HTTPS server settings.
	TLS TLSConfig `yaml:"tls" json:"tls"`

	// RemoteManagement nests management-related options under 'remote-management'.
	RemoteManagement RemoteManagement `yaml:"remote-management" json:"-"`

	// AuthDir is the directory where authentication token files are stored.
	AuthDir string `yaml:"auth-dir" json:"-"`

	// Debug enables or disables debug-level logging and other debug features.
	Debug bool `yaml:"debug" json:"debug"`

	// Pprof config controls the optional pprof HTTP debug server.
	Pprof PprofConfig `yaml:"pprof" json:"pprof"`

	// CommercialMode disables high-overhead HTTP middleware features to minimize per-request memory usage.
	CommercialMode bool `yaml:"commercial-mode" json:"commercial-mode"`

	// LoggingToFile controls whether application logs are written to rotating files or stdout.
	LoggingToFile bool `yaml:"logging-to-file" json:"logging-to-file"`

	// LogsMaxTotalSizeMB limits the total size (in MB) of log files under the logs directory.
	// When exceeded, the oldest log files are deleted until within the limit. Set to 0 to disable.
	LogsMaxTotalSizeMB int `yaml:"logs-max-total-size-mb" json:"logs-max-total-size-mb"`

	// ErrorLogsMaxFiles limits the number of error log files retained when request logging is disabled.
	// When exceeded, the oldest error log files are deleted. Default is 10. Set to 0 to disable cleanup.
	ErrorLogsMaxFiles int `yaml:"error-logs-max-files" json:"error-logs-max-files"`

	// UsageStatisticsEnabled toggles in-memory usage aggregation; when false, usage data is discarded.
	UsageStatisticsEnabled bool `yaml:"usage-statistics-enabled" json:"usage-statistics-enabled"`

	// DisableCooling disables quota cooldown scheduling when true.
	DisableCooling bool `yaml:"disable-cooling" json:"disable-cooling"`

	// RequestRetry defines the retry times when the request failed.
	RequestRetry int `yaml:"request-retry" json:"request-retry"`
	// MaxRetryCredentials defines the maximum number of credentials to try for a failed request.
	// Set to 0 or a negative value to keep trying all available credentials (legacy behavior).
	MaxRetryCredentials int `yaml:"max-retry-credentials" json:"max-retry-credentials"`
	// MaxRetryInterval defines the maximum wait time in seconds before retrying a cooled-down credential.
	MaxRetryInterval int `yaml:"max-retry-interval" json:"max-retry-interval"`

	// QuotaExceeded defines the behavior when a quota is exceeded.
	QuotaExceeded QuotaExceeded `yaml:"quota-exceeded" json:"quota-exceeded"`

	// Routing controls credential selection behavior.
	Routing RoutingConfig `yaml:"routing" json:"routing"`

	// WebsocketAuth enables or disables authentication for the WebSocket API.
	WebsocketAuth bool `yaml:"ws-auth" json:"ws-auth"`

	// GeminiKey defines Gemini API key configurations with optional routing overrides.
	GeminiKey []GeminiKey `yaml:"gemini-api-key" json:"gemini-api-key"`

	// KiroKey defines a list of Kiro (AWS CodeWhisperer) configurations.
	KiroKey []KiroKey `yaml:"kiro" json:"kiro"`

	// KiroFingerprint defines a global fingerprint configuration for all Kiro requests.
	// When set, all Kiro requests will use this fixed fingerprint instead of random generation.
	KiroFingerprint *KiroFingerprintConfig `yaml:"kiro-fingerprint,omitempty" json:"kiro-fingerprint,omitempty"`

	// KiroPreferredEndpoint sets the global default preferred endpoint for all Kiro providers.
	// Values: "ide" (default, CodeWhisperer) or "cli" (Amazon Q).
	KiroPreferredEndpoint string `yaml:"kiro-preferred-endpoint" json:"kiro-preferred-endpoint"`

	// Codex defines a list of Codex API key configurations as specified in the YAML configuration file.
	CodexKey []CodexKey `yaml:"codex-api-key" json:"codex-api-key"`

	// ClaudeKey defines a list of Claude API key configurations as specified in the YAML configuration file.
	ClaudeKey []ClaudeKey `yaml:"claude-api-key" json:"claude-api-key"`

	// ClaudeHeaderDefaults configures default header values for Claude API requests.
	// These are used as fallbacks when the client does not send its own headers.
	ClaudeHeaderDefaults ClaudeHeaderDefaults `yaml:"claude-header-defaults" json:"claude-header-defaults"`

	// OpenAICompatibility defines OpenAI API compatibility configurations for external providers.
	OpenAICompatibility []OpenAICompatibility `yaml:"openai-compatibility" json:"openai-compatibility"`

	// VertexCompatAPIKey defines Vertex AI-compatible API key configurations for third-party providers.
	// Used for services that use Vertex AI-style paths but with simple API key authentication.
	VertexCompatAPIKey []VertexCompatKey `yaml:"vertex-api-key" json:"vertex-api-key"`

	// AmpCode contains Amp CLI upstream configuration, management restrictions, and model mappings.
	AmpCode AmpCode `yaml:"ampcode" json:"ampcode"`

	// OAuthExcludedModels defines per-provider global model exclusions applied to OAuth/file-backed auth entries.
	// Supported channels: gemini-cli, vertex, aistudio, antigravity, claude, codex, qwen, iflow, kiro, github-copilot.
	OAuthExcludedModels map[string][]string `yaml:"oauth-excluded-models,omitempty" json:"oauth-excluded-models,omitempty"`

	// OAuthModelAlias defines global model name aliases for OAuth/file-backed auth channels.
	// These aliases affect both model listing and model routing for supported channels:
	// gemini-cli, vertex, aistudio, antigravity, claude, codex, qwen, iflow, kiro, github-copilot.
	//
	// NOTE: This does not apply to existing per-credential model alias features under:
	// gemini-api-key, codex-api-key, claude-api-key, openai-compatibility, vertex-api-key, and ampcode.
	OAuthModelAlias map[string][]OAuthModelAlias `yaml:"oauth-model-alias,omitempty" json:"oauth-model-alias,omitempty"`

	// Payload defines default and override rules for provider payload parameters.
	Payload PayloadConfig `yaml:"payload" json:"payload"`

	// IncognitoBrowser enables opening OAuth URLs in incognito/private browsing mode.
	// This is useful when you want to login with a different account without logging out
	// from your current session. Default: false.
	IncognitoBrowser bool `yaml:"incognito-browser" json:"incognito-browser"`
}

func (cfg *Config) ToSDKConfig() (*sdkconfig.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil runtime config")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime config: %w", err)
	}

	var out sdkconfig.Config
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal runtime config for builder: %w", err)
	}

	return &out, nil
}

// ClaudeHeaderDefaults configures default header values injected into Claude API requests
// when the client does not send them. Update these when Claude Code releases a new version.
type ClaudeHeaderDefaults struct {
	UserAgent      string `yaml:"user-agent" json:"user-agent"`
	PackageVersion string `yaml:"package-version" json:"package-version"`
	RuntimeVersion string `yaml:"runtime-version" json:"runtime-version"`
	Timeout        string `yaml:"timeout" json:"timeout"`
}

// TLSConfig holds HTTPS server settings.
type TLSConfig struct {
	// Enable toggles HTTPS server mode.
	Enable bool `yaml:"enable" json:"enable"`
	// Cert is the path to the TLS certificate file.
	Cert string `yaml:"cert" json:"cert"`
	// Key is the path to the TLS private key file.
	Key string `yaml:"key" json:"key"`
}

// PprofConfig holds pprof HTTP server settings.
type PprofConfig struct {
	// Enable toggles the pprof HTTP debug server.
	Enable bool `yaml:"enable" json:"enable"`
	// Addr is the host:port address for the pprof HTTP server.
	Addr string `yaml:"addr" json:"addr"`
}

// RemoteManagement holds management API configuration under 'remote-management'.
type RemoteManagement struct {
	// AllowRemote toggles remote (non-localhost) access to management API.
	AllowRemote bool `yaml:"allow-remote"`
	// SecretKey is the management key (plaintext or bcrypt hashed). YAML key intentionally 'secret-key'.
	SecretKey string `yaml:"secret-key"`
	// DisableControlPanel skips serving and syncing the bundled management UI when true.
	DisableControlPanel bool `yaml:"disable-control-panel"`
	// PanelGitHubRepository overrides the GitHub repository used to fetch the management panel asset.
	// Accepts either a repository URL (https://github.com/org/repo) or an API releases endpoint.
	PanelGitHubRepository string `yaml:"panel-github-repository"`
}

// QuotaExceeded defines the behavior when API quota limits are exceeded.
// It provides configuration options for automatic failover mechanisms.
type QuotaExceeded struct {
	// SwitchProject indicates whether to automatically switch to another project when a quota is exceeded.
	SwitchProject bool `yaml:"switch-project" json:"switch-project"`

	// SwitchPreviewModel indicates whether to automatically switch to a preview model when a quota is exceeded.
	SwitchPreviewModel bool `yaml:"switch-preview-model" json:"switch-preview-model"`
}

// RoutingConfig configures how credentials are selected for requests.
type RoutingConfig struct {
	// Strategy selects the credential selection strategy.
	// Supported values: "round-robin" (default), "fill-first".
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
}

// OAuthModelAlias defines a model ID alias for a specific channel.
// It maps the upstream model name (Name) to the client-visible alias (Alias).
// When Fork is true, the alias is added as an additional model in listings while
// keeping the original model ID available.
type OAuthModelAlias struct {
	Name  string `yaml:"name" json:"name"`
	Alias string `yaml:"alias" json:"alias"`
	Fork  bool   `yaml:"fork,omitempty" json:"fork,omitempty"`
}
