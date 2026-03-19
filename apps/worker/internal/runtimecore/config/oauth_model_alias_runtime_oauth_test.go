package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkconfig "github.com/kunish/wheel/apps/worker/internal/runtime/corelib/config"
)

func TestLoadConfig_RuntimeOAuthAliasParity(t *testing.T) {
	t.Run("defaults stay aligned", func(t *testing.T) {
		configPath := writeRuntimeOAuthAliasConfig(t, "remote-management:\n  secret-key: local-secret\n")

		runtimeCfg, sdkCfg := loadRuntimeOAuthConfigs(t, configPath)

		assertEquivalentRuntimeOAuthAliases(t, runtimeCfg, sdkCfg)
	})

	t.Run("explicit deletion stays deleted", func(t *testing.T) {
		configPath := writeRuntimeOAuthAliasConfig(t, strings.Join([]string{
			"remote-management:",
			"  secret-key: local-secret",
			"oauth-model-alias:",
			"  github-copilot: []",
			"  antigravity: null",
		}, "\n")+"\n")

		runtimeCfg, sdkCfg := loadRuntimeOAuthConfigs(t, configPath)

		assertEquivalentRuntimeOAuthAliases(t, runtimeCfg, sdkCfg)
	})
}

func TestLoadConfig_RuntimeOAuthInvalidChannelVariantFallsBackToDefaults(t *testing.T) {
	cfg := &Config{OAuthModelAlias: map[string][]OAuthModelAlias{
		"  antigravity  ": {
			{Name: "", Alias: ""},
		},
	}}

	cfg.sanitizeOAuthModelAlias()
	aliases := cfg.OAuthModelAlias["antigravity"]
	if len(aliases) == 0 {
		t.Fatal("expected antigravity defaults after invalid variant was normalized away")
	}
	for _, alias := range aliases {
		if alias.Name == "" || alias.Alias == "" {
			t.Fatalf("unexpected empty antigravity alias entry: %+v", alias)
		}
	}
}

func writeRuntimeOAuthAliasConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func loadRuntimeOAuthConfigs(t *testing.T, configPath string) (*Config, *sdkconfig.Config) {
	t.Helper()
	runtimeCfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("runtimecore LoadConfig() error = %v", err)
	}
	sdkCfg, err := sdkconfig.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("corelib LoadConfig() error = %v", err)
	}
	return runtimeCfg, sdkCfg
}

func assertEquivalentRuntimeOAuthAliases(t *testing.T, runtimeCfg *Config, want *sdkconfig.Config) {
	t.Helper()
	got := runtimeCfg.OAuthModelAlias
	wantAliases := want.OAuthModelAlias
	if len(got) != len(wantAliases) {
		t.Fatalf("alias channel count mismatch: runtimecore=%d corelib=%d", len(got), len(wantAliases))
	}
	for _, channel := range []string{"kiro", "github-copilot", "antigravity"} {
		gotAliases, gotOK := got[channel]
		wantChannelAliases, wantOK := wantAliases[channel]
		if gotOK != wantOK {
			t.Fatalf("channel %s presence mismatch: runtimecore=%v corelib=%v", channel, gotOK, wantOK)
		}
		if len(gotAliases) != len(wantChannelAliases) {
			t.Fatalf("channel %s alias count mismatch: runtimecore=%d corelib=%d", channel, len(gotAliases), len(wantChannelAliases))
		}
		for i := range gotAliases {
			if gotAliases[i].Name != wantChannelAliases[i].Name || gotAliases[i].Alias != wantChannelAliases[i].Alias || gotAliases[i].Fork != wantChannelAliases[i].Fork {
				t.Fatalf("channel %s alias %d mismatch: runtimecore=%+v corelib=%+v", channel, i, gotAliases[i], wantChannelAliases[i])
			}
		}
	}

	gotSDK := make(map[string][]sdkconfig.OAuthModelAlias, len(got))
	for channel, aliases := range got {
		if aliases == nil {
			gotSDK[channel] = nil
			continue
		}
		converted := make([]sdkconfig.OAuthModelAlias, len(aliases))
		for i, alias := range aliases {
			converted[i] = sdkconfig.OAuthModelAlias{Name: alias.Name, Alias: alias.Alias, Fork: alias.Fork}
		}
		gotSDK[channel] = converted
	}
	if !reflect.DeepEqual(gotSDK, wantAliases) {
		t.Fatalf("runtimecore/corelib alias maps differ:\nruntimecore=%#v\ncorelib=%#v", gotSDK, wantAliases)
	}

	convertedCfg, err := runtimeCfg.ToSDKConfig()
	if err != nil {
		t.Fatalf("ToSDKConfig() error = %v", err)
	}
	if !reflect.DeepEqual(convertedCfg.OAuthModelAlias, wantAliases) {
		t.Fatalf("runtimecore ToSDKConfig alias maps differ from direct corelib load:\nconverted=%#v\ncorelib=%#v", convertedCfg.OAuthModelAlias, wantAliases)
	}
}
