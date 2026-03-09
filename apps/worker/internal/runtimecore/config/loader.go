package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads a YAML configuration file from the given path,
// unmarshals it into a Config struct, applies environment variable overrides,
// and returns it.
//
// Parameters:
//   - configFile: The path to the YAML configuration file
//
// Returns:
//   - *Config: The loaded configuration
//   - error: An error if the configuration could not be loaded
func LoadConfig(configFile string) (*Config, error) {
	return LoadConfigOptional(configFile, false)
}

// LoadConfigOptional reads YAML from configFile.
// If optional is true and the file is missing, it returns an empty Config.
// If optional is true and the file is empty or invalid, it returns an empty Config.
func LoadConfigOptional(configFile string, optional bool) (*Config, error) {
	// Read the entire configuration file into memory.
	data, err := os.ReadFile(configFile)
	if err != nil {
		if optional {
			if os.IsNotExist(err) || errors.Is(err, syscall.EISDIR) {
				// Missing and optional: return empty config (cloud deploy standby).
				return &Config{}, nil
			}
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// In cloud deploy mode (optional=true), if file is empty or contains only whitespace, return empty config.
	if optional && len(data) == 0 {
		return &Config{}, nil
	}

	// Unmarshal the YAML data into the Config struct.
	var cfg Config
	// Set defaults before unmarshal so that absent keys keep defaults.
	cfg.Host = "" // Default empty: binds to all interfaces (IPv4 + IPv6)
	cfg.LoggingToFile = false
	cfg.LogsMaxTotalSizeMB = 0
	cfg.ErrorLogsMaxFiles = 10
	cfg.UsageStatisticsEnabled = false
	cfg.DisableCooling = false
	cfg.Pprof.Enable = false
	cfg.Pprof.Addr = DefaultPprofAddr
	cfg.AmpCode.RestrictManagementToLocalhost = false // Default to false: API key auth is sufficient
	cfg.RemoteManagement.PanelGitHubRepository = DefaultPanelGitHubRepository
	cfg.IncognitoBrowser = false // Default to normal browser (AWS uses incognito by force)
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		if optional {
			// In cloud deploy mode, if YAML parsing fails, return empty config instead of error.
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// NOTE: Startup legacy key migration is intentionally disabled.
	// Reason: avoid mutating config.yaml during server startup.
	// Re-enable the block below if automatic startup migration is needed again.
	// var legacy legacyConfigData
	// if errLegacy := yaml.Unmarshal(data, &legacy); errLegacy == nil {
	// 	if cfg.migrateLegacyGeminiKeys(legacy.LegacyGeminiKeys) {
	// 		cfg.legacyMigrationPending = true
	// 	}
	// 	if cfg.migrateLegacyOpenAICompatibilityKeys(legacy.OpenAICompat) {
	// 		cfg.legacyMigrationPending = true
	// 	}
	// 	if cfg.migrateLegacyAmpConfig(&legacy) {
	// 		cfg.legacyMigrationPending = true
	// 	}
	// }

	// Hash remote management key if plaintext is detected (nested)
	// We consider a value to be already hashed if it looks like a bcrypt hash ($2a$, $2b$, or $2y$ prefix).
	if cfg.RemoteManagement.SecretKey != "" && !looksLikeBcrypt(cfg.RemoteManagement.SecretKey) {
		hashed, errHash := hashSecret(cfg.RemoteManagement.SecretKey)
		if errHash != nil {
			return nil, fmt.Errorf("failed to hash remote management key: %w", errHash)
		}
		cfg.RemoteManagement.SecretKey = hashed

		// Persist the hashed value back to the config file to avoid re-hashing on next startup.
		// Preserve YAML comments and ordering; update only the nested key.
		_ = SaveConfigPreserveCommentsUpdateNestedScalar(configFile, []string{"remote-management", "secret-key"}, hashed)
	}

	cfg.RemoteManagement.PanelGitHubRepository = strings.TrimSpace(cfg.RemoteManagement.PanelGitHubRepository)
	if cfg.RemoteManagement.PanelGitHubRepository == "" {
		cfg.RemoteManagement.PanelGitHubRepository = DefaultPanelGitHubRepository
	}

	cfg.Pprof.Addr = strings.TrimSpace(cfg.Pprof.Addr)
	if cfg.Pprof.Addr == "" {
		cfg.Pprof.Addr = DefaultPprofAddr
	}

	if cfg.LogsMaxTotalSizeMB < 0 {
		cfg.LogsMaxTotalSizeMB = 0
	}

	if cfg.ErrorLogsMaxFiles < 0 {
		cfg.ErrorLogsMaxFiles = 10
	}

	if cfg.MaxRetryCredentials < 0 {
		cfg.MaxRetryCredentials = 0
	}

	// Sanitize Gemini API key configuration and migrate legacy entries.
	cfg.SanitizeGeminiKeys()

	// Sanitize Vertex-compatible API keys: drop entries without base-url
	cfg.SanitizeVertexCompatKeys()

	// Sanitize Codex keys: drop entries without base-url
	cfg.SanitizeCodexKeys()

	// Sanitize Claude key headers
	cfg.SanitizeClaudeKeys()

	// Sanitize Kiro keys: trim whitespace from credential fields
	cfg.SanitizeKiroKeys()

	// Sanitize OpenAI compatibility providers: drop entries without base-url
	cfg.SanitizeOpenAICompatibility()

	// Normalize OAuth provider model exclusion map.
	cfg.OAuthExcludedModels = NormalizeOAuthExcludedModels(cfg.OAuthExcludedModels)

	// Normalize global OAuth model name aliases.
	cfg.SanitizeOAuthModelAlias()

	// Validate raw payload rules and drop invalid entries.
	cfg.SanitizePayloadRules()

	// NOTE: Legacy migration persistence is intentionally disabled together with
	// startup legacy migration to keep startup read-only for config.yaml.
	// Re-enable the block below if automatic startup migration is needed again.
	// if cfg.legacyMigrationPending {
	// 	fmt.Println("Detected legacy configuration keys, attempting to persist the normalized config...")
	// 	if !optional && configFile != "" {
	// 		if err := SaveConfigPreserveComments(configFile, &cfg); err != nil {
	// 			return nil, fmt.Errorf("failed to persist migrated legacy config: %w", err)
	// 		}
	// 		fmt.Println("Legacy configuration normalized and persisted.")
	// 	} else {
	// 		fmt.Println("Legacy configuration normalized in memory; persistence skipped.")
	// 	}
	// }

	// Return the populated configuration struct.
	return &cfg, nil
}
