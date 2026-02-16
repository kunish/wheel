package bifrostx

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	schemas "github.com/maximhq/bifrost/core/schemas"
	"github.com/uptrace/bun"
)

const providerKeyPrefix = "wheel-ch-"

type accountContextKey string

const (
	contextKeyChannelID     accountContextKey = "wheel.channel_id"
	contextKeySelectedKeyID accountContextKey = "wheel.selected_key_id"
	contextKeySelectedKey   accountContextKey = "wheel.selected_key"
	contextKeySelectedModel accountContextKey = "wheel.selected_model"
)

type Account struct {
	db          *bun.DB
	sendBackRaw bool
}

func NewAccount(db *bun.DB, sendBackRaw bool) *Account {
	return &Account{
		db:          db,
		sendBackRaw: sendBackRaw,
	}
}

func ProviderKeyForChannelID(channelID int) schemas.ModelProvider {
	return schemas.ModelProvider(fmt.Sprintf("%s%d", providerKeyPrefix, channelID))
}

func ParseChannelIDFromProviderKey(providerKey schemas.ModelProvider) (int, error) {
	raw := string(providerKey)
	if !strings.HasPrefix(raw, providerKeyPrefix) {
		return 0, fmt.Errorf("invalid provider key: %s", providerKey)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(raw, providerKeyPrefix))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid provider key: %s", providerKey)
	}
	return id, nil
}

func (a *Account) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	channels, err := dal.ListChannels(context.Background(), a.db)
	if err != nil {
		return nil, err
	}

	providers := make([]schemas.ModelProvider, 0, len(channels))
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		providers = append(providers, ProviderKeyForChannelID(channel.ID))
	}
	return providers, nil
}

func (a *Account) GetConfigForProvider(providerKey schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	channelID, err := ParseChannelIDFromProviderKey(providerKey)
	if err != nil {
		return nil, err
	}

	channel, err := dal.GetChannel(context.Background(), a.db, channelID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel %d not found", channelID)
	}
	if !channel.Enabled {
		return nil, fmt.Errorf("channel %d is disabled", channelID)
	}

	baseProvider := baseProviderForChannelType(channel.Type)
	config := &schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        selectBaseURL(channel.BaseUrls),
			ExtraHeaders:                   customHeadersToMap(channel.CustomHeader),
			DefaultRequestTimeoutInSeconds: 60,
			MaxRetries:                     0,
		},
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{
			Concurrency: 128,
			BufferSize:  1024,
		},
		ProxyConfig:         buildProxyConfig(channel),
		SendBackRawRequest:  a.sendBackRaw,
		SendBackRawResponse: a.sendBackRaw,
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType:     baseProvider,
			RequestPathOverrides: requestPathOverrides(baseProvider),
		},
	}
	config.CheckAndSetDefaults()
	config.NetworkConfig.MaxRetries = 0
	return config, nil
}

func selectBaseURL(baseURLs []types.BaseUrl) string {
	if len(baseURLs) == 0 {
		return "https://api.openai.com"
	}
	best := baseURLs[0]
	for i := 1; i < len(baseURLs); i++ {
		if baseURLs[i].Delay < best.Delay {
			best = baseURLs[i]
		}
	}
	return strings.TrimRight(best.URL, "/")
}

func (a *Account) GetKeysForProvider(ctx context.Context, providerKey schemas.ModelProvider) ([]schemas.Key, error) {
	channelID, err := ParseChannelIDFromProviderKey(providerKey)
	if err != nil {
		return nil, err
	}

	var selectedKeyValue string
	if v, ok := ctx.Value(contextKeySelectedKey).(string); ok {
		selectedKeyValue = strings.TrimSpace(v)
	}

	selectedKeyID := 0
	if v, ok := ctx.Value(contextKeySelectedKeyID).(int); ok {
		selectedKeyID = v
	}

	selectedModel := ""
	if v, ok := ctx.Value(contextKeySelectedModel).(string); ok {
		selectedModel = strings.TrimSpace(v)
	}

	if selectedKeyValue == "" {
		channel, getErr := dal.GetChannel(context.Background(), a.db, channelID)
		if getErr != nil {
			return nil, getErr
		}
		if channel == nil {
			return nil, fmt.Errorf("channel %d not found", channelID)
		}
		for _, key := range channel.Keys {
			if selectedKeyID > 0 && key.ID != selectedKeyID {
				continue
			}
			if !key.Enabled {
				continue
			}
			selectedKeyID = key.ID
			selectedKeyValue = key.ChannelKey
			break
		}
	}

	if selectedKeyValue == "" {
		return nil, fmt.Errorf("no selected key for provider %s", providerKey)
	}

	if selectedKeyID == 0 {
		selectedKeyID = -1
	}
	keyID := strconv.Itoa(selectedKeyID)

	var models []string
	if selectedModel != "" {
		models = []string{selectedModel}
	}

	return []schemas.Key{
		{
			ID:     keyID,
			Name:   fmt.Sprintf("wheel-ch-%d-key-%s", channelID, keyID),
			Value:  *schemas.NewEnvVar(selectedKeyValue),
			Models: models,
			Weight: 1,
		},
	}, nil
}

func requestPathOverrides(baseProvider schemas.ModelProvider) map[schemas.RequestType]string {
	switch baseProvider {
	case schemas.Anthropic:
		return map[schemas.RequestType]string{
			schemas.ChatCompletionRequest:       "/v1/messages",
			schemas.ChatCompletionStreamRequest: "/v1/messages",
		}
	case schemas.OpenAI:
		return map[schemas.RequestType]string{
			schemas.ChatCompletionRequest:       "/v1/chat/completions",
			schemas.ChatCompletionStreamRequest: "/v1/chat/completions",
			schemas.ResponsesRequest:            "/v1/responses",
			schemas.ResponsesStreamRequest:      "/v1/responses",
		}
	default:
		return nil
	}
}

func baseProviderForChannelType(channelType types.OutboundType) schemas.ModelProvider {
	switch channelType {
	case types.OutboundAnthropic:
		return schemas.Anthropic
	case types.OutboundGemini:
		return schemas.Gemini
	default:
		return schemas.OpenAI
	}
}

func customHeadersToMap(headers []types.CustomHeader) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for _, header := range headers {
		if header.Key == "" {
			continue
		}
		result[header.Key] = header.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildProxyConfig(channel *types.Channel) *schemas.ProxyConfig {
	if channel == nil || !channel.Proxy || channel.ChannelProxy == nil {
		return nil
	}
	raw := strings.TrimSpace(*channel.ChannelProxy)
	if raw == "" {
		return nil
	}
	if strings.EqualFold(raw, "environment") {
		return &schemas.ProxyConfig{Type: schemas.EnvProxy}
	}

	proxyType := schemas.HTTPProxy
	parsed, err := url.Parse(raw)
	if err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "socks5", "socks5h":
			proxyType = schemas.Socks5Proxy
		}
	}

	config := &schemas.ProxyConfig{
		Type: proxyType,
		URL:  raw,
	}
	if parsed != nil && parsed.User != nil {
		config.Username = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			config.Password = password
		}
	}
	return config
}
