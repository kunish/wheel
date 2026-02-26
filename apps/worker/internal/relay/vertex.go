package relay

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildVertexRequest builds a request for Google Vertex AI.
// Vertex AI uses the same Gemini format but with a different URL pattern and OAuth auth.
func buildVertexRequest(baseUrl, key string, body map[string]any, model string, channel ChannelConfig) UpstreamRequest {
	// Vertex AI URL pattern: {base}/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:streamGenerateContent
	// If baseUrl already contains the full path, use it directly.
	// Otherwise, build from custom headers (project, location).

	project := ""
	location := "us-central1"
	for _, h := range channel.CustomHeader {
		switch strings.ToLower(h.Key) {
		case "x-vertex-project":
			project = h.Value
		case "x-vertex-location":
			location = h.Value
		}
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	// Vertex uses OAuth2 Bearer token
	if key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	for _, h := range channel.CustomHeader {
		if strings.HasPrefix(strings.ToLower(h.Key), "x-vertex-") {
			continue // skip our custom config headers
		}
		headers[h.Key] = h.Value
	}

	// Build Gemini-format body (reuse Gemini conversion logic)
	outBody := copyBody(body)
	delete(outBody, "model")

	messages, _ := outBody["messages"].([]any)
	delete(outBody, "messages")

	var systemParts []any
	var contents []any

	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system", "developer":
			systemParts = append(systemParts, contentToGeminiParts(msg["content"]))
		case "assistant":
			parts := assistantToGeminiParts(msg)
			contents = append(contents, map[string]any{"role": "model", "parts": parts})
		case "tool":
			contents = append(contents, toolResultToGeminiContent(msg))
		default:
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": contentToGeminiParts(msg["content"]),
			})
		}
	}

	geminiBody := map[string]any{"contents": contents}
	if len(systemParts) > 0 {
		var allParts []any
		for _, sp := range systemParts {
			if parts, ok := sp.([]any); ok {
				allParts = append(allParts, parts...)
			}
		}
		geminiBody["systemInstruction"] = map[string]any{"parts": allParts}
	}

	genConfig := map[string]any{}
	if t, ok := outBody["temperature"]; ok {
		genConfig["temperature"] = t
	}
	if tp, ok := outBody["top_p"]; ok {
		genConfig["topP"] = tp
	}
	if mt, ok := outBody["max_tokens"].(float64); ok && mt > 0 {
		genConfig["maxOutputTokens"] = int(mt)
	}
	if len(genConfig) > 0 {
		geminiBody["generationConfig"] = genConfig
	}

	if tools, ok := outBody["tools"].([]any); ok {
		geminiBody["tools"] = convertOpenAIToolsToGemini(tools)
	}

	applyParamOverrides(geminiBody, channel.ParamOverride)

	stream, _ := body["stream"].(bool)
	var url string
	if project != "" {
		// Full Vertex AI path
		action := "generateContent"
		if stream {
			action = "streamGenerateContent?alt=sse"
		}
		url = fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
			baseUrl, project, location, model, action)
	} else {
		// Fallback: treat as Gemini-style endpoint
		if stream {
			url = fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseUrl, model)
		} else {
			url = fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseUrl, model)
		}
	}

	bodyJSON, _ := json.Marshal(geminiBody)
	return UpstreamRequest{
		URL:     url,
		Headers: headers,
		Body:    string(bodyJSON),
	}
}
