package relay

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildAzureOpenAIRequest builds a request for Azure OpenAI Service.
// Azure uses a different URL pattern: {base}/openai/deployments/{model}/chat/completions?api-version=...
func buildAzureOpenAIRequest(baseUrl, key string, body map[string]any, inboundPath, model string, channel ChannelConfig) UpstreamRequest {
	apiVersion := "2024-12-01-preview"

	// Allow overriding api-version via custom headers
	headers := map[string]string{
		"Content-Type": "application/json",
		"api-key":      key,
	}
	for _, h := range channel.CustomHeader {
		if strings.EqualFold(h.Key, "api-version") {
			apiVersion = h.Value
			continue
		}
		headers[h.Key] = h.Value
	}

	// Determine endpoint type from inbound path
	endpoint := "chat/completions"
	if strings.Contains(inboundPath, "/chat/completions") {
		endpoint = "chat/completions"
	} else if strings.Contains(inboundPath, "/completions") {
		endpoint = "completions"
	} else if strings.Contains(inboundPath, "/embeddings") {
		endpoint = "embeddings"
	} else if strings.Contains(inboundPath, "/images/generations") {
		endpoint = "images/generations"
	} else if strings.Contains(inboundPath, "/audio/speech") {
		endpoint = "audio/speech"
	} else if strings.Contains(inboundPath, "/audio/transcriptions") {
		endpoint = "audio/transcriptions"
	} else if strings.Contains(inboundPath, "/audio/translations") {
		endpoint = "audio/translations"
	} else if strings.Contains(inboundPath, "/moderations") {
		endpoint = "moderations"
	} else if strings.Contains(inboundPath, "/responses") {
		endpoint = "responses"
	}

	// Azure deployment name is typically the model name
	deployment := model
	url := fmt.Sprintf("%s/openai/deployments/%s/%s?api-version=%s", baseUrl, deployment, endpoint, apiVersion)

	outBody := copyBody(body)
	// Azure doesn't need model in body since it's in the URL
	delete(outBody, "model")
	applyParamOverrides(outBody, channel.ParamOverride)

	bodyJSON, _ := json.Marshal(outBody)
	return UpstreamRequest{
		URL:     url,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}
