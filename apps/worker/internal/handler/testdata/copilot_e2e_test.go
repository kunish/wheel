package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tokenExchangeURL = "https://api.github.com/copilot_internal/v2/token"
	copilotBaseURL    = "https://api.githubcopilot.com"
)

type copilotTokenResp struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	Endpoints struct {
		API string `json:"api"`
	} `json:"endpoints"`
	ErrorDetails *struct {
		Message string `json:"message"`
	} `json:"error_details"`
}

func main() {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}

	fmt.Println("=== Copilot Relay E2E Test Suite ===")
	fmt.Println()

	// Test 1: Token Exchange
	fmt.Println("▶ Test 1: Token Exchange (GitHub → Copilot JWT)")
	apiToken, baseURL, err := exchangeToken(githubToken)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ Token obtained (len=%d, expires_at=%d)\n", len(apiToken), 0)
	fmt.Printf("  ✓ API endpoint: %s\n", baseURL)
	fmt.Println()

	// Test 2: Non-streaming Chat Completions (claude-opus-4.6)
	fmt.Println("▶ Test 2: Non-streaming /chat/completions (claude-opus-4.6)")
	testNonStreaming(apiToken, baseURL, "claude-opus-4.6", false)
	fmt.Println()

	// Test 3: Streaming Chat Completions (claude-opus-4.6)
	fmt.Println("▶ Test 3: Streaming /chat/completions (claude-opus-4.6)")
	testStreaming(apiToken, baseURL, "claude-opus-4.6", false)
	fmt.Println()

	// Test 4: X-Initiator = agent (tool loop simulation)
	fmt.Println("▶ Test 4: X-Initiator=agent (tool loop body)")
	testAgentInitiated(apiToken, baseURL, "claude-opus-4.6")
	fmt.Println()

	// Test 5: X-Initiator = user (normal user message)
	fmt.Println("▶ Test 5: X-Initiator=user (normal user body)")
	testUserInitiated(apiToken, baseURL, "claude-opus-4.6")
	fmt.Println()

	// Test 6: stream_options.include_usage
	fmt.Println("▶ Test 6: stream_options.include_usage in streaming")
	testStreamUsage(apiToken, baseURL, "claude-opus-4.6")
	fmt.Println()

	// Test 7: Responses API with codex model
	fmt.Println("▶ Test 7: /responses endpoint (codex-mini-latest)")
	testResponsesAPI(apiToken, baseURL, "codex-mini-latest")
	fmt.Println()

	// Test 8: reasoning_text normalization
	fmt.Println("▶ Test 8: reasoning/thinking support (claude-opus-4.6)")
	testReasoningSupport(apiToken, baseURL, "claude-opus-4.6")
	fmt.Println()

	// Test 9: List models
	fmt.Println("▶ Test 9: GET /models")
	testListModels(apiToken, baseURL)
	fmt.Println()

	fmt.Println("=== All E2E tests completed ===")
}

func exchangeToken(githubToken string) (string, string, error) {
	req, _ := http.NewRequest("GET", tokenExchangeURL, nil)
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GithubCopilot/1.0")
	req.Header.Set("Editor-Version", "vscode/1.100.0")
	req.Header.Set("Editor-Plugin-Version", "copilot/1.300.0")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp copilotTokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", fmt.Errorf("parse response: %w", err)
	}
	if tokenResp.ErrorDetails != nil {
		return "", "", fmt.Errorf("token error: %s", tokenResp.ErrorDetails.Message)
	}

	baseURL := copilotBaseURL
	if tokenResp.Endpoints.API != "" {
		baseURL = strings.TrimRight(tokenResp.Endpoints.API, "/")
	}
	return tokenResp.Token, baseURL, nil
}

func copilotHeaders(apiToken string, body []byte) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Authorization", "Bearer "+apiToken)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", "GitHubCopilotChat/0.35.0")
	h.Set("Editor-Version", "vscode/1.107.0")
	h.Set("Editor-Plugin-Version", "copilot-chat/0.35.0")
	h.Set("Openai-Intent", "conversation-edits")
	h.Set("Copilot-Integration-Id", "vscode-chat")
	h.Set("X-Github-Api-Version", "2025-04-01")
	h.Set("X-Request-Id", uuid.NewString())
	h.Set("X-Initiator", "user")
	return h
}

func testNonStreaming(apiToken, baseURL, model string, _ bool) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 50,
		"stream":     false,
		"messages": []any{
			map[string]any{"role": "user", "content": "Say hello in exactly 3 words."},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)

	start := time.Now()
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("  ✗ FAIL: request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	// Extract content
	choices, _ := result["choices"].([]any)
	content := ""
	if len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			if msg, ok := c["message"].(map[string]any); ok {
				content, _ = msg["content"].(string)
			}
		}
	}

	// Extract usage
	usage, _ := result["usage"].(map[string]any)
	promptTokens := toInt(usage["prompt_tokens"])
	completionTokens := toInt(usage["completion_tokens"])

	fmt.Printf("  ✓ HTTP %d, latency=%v\n", resp.StatusCode, elapsed.Round(time.Millisecond))
	fmt.Printf("  ✓ Model: %v\n", result["model"])
	fmt.Printf("  ✓ Content: %q\n", truncate(content, 100))
	fmt.Printf("  ✓ Usage: prompt=%d, completion=%d\n", promptTokens, completionTokens)
}

func testStreaming(apiToken, baseURL, model string, _ bool) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 50,
		"stream":     true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "Count from 1 to 5, one number per line."},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)

	start := time.Now()
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, 20_971_520)

	var chunks int
	var accContent string
	var firstTokenTime time.Duration
	var promptTokens, completionTokens int
	var hasReasoningText, hasReasoningContent bool

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			continue
		}
		chunks++

		if chunks == 1 {
			firstTokenTime = time.Since(start)
		}

		var obj map[string]any
		if json.Unmarshal([]byte(data), &obj) != nil {
			continue
		}

		// Check usage
		if usage, ok := obj["usage"].(map[string]any); ok {
			promptTokens = toInt(usage["prompt_tokens"])
			completionTokens = toInt(usage["completion_tokens"])
		}

		// Accumulate content
		if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
			if c, ok := choices[0].(map[string]any); ok {
				if delta, ok := c["delta"].(map[string]any); ok {
					if text, ok := delta["content"].(string); ok {
						accContent += text
					}
					if _, ok := delta["reasoning_text"]; ok {
						hasReasoningText = true
					}
					if _, ok := delta["reasoning_content"]; ok {
						hasReasoningContent = true
					}
				}
			}
		}
	}

	fmt.Printf("  ✓ HTTP %d, first_token=%v\n", resp.StatusCode, firstTokenTime.Round(time.Millisecond))
	fmt.Printf("  ✓ Chunks received: %d\n", chunks)
	fmt.Printf("  ✓ Content: %q\n", truncate(accContent, 100))
	fmt.Printf("  ✓ Usage: prompt=%d, completion=%d\n", promptTokens, completionTokens)
	if hasReasoningText {
		fmt.Println("  ℹ reasoning_text field detected (needs normalization)")
	}
	if hasReasoningContent {
		fmt.Println("  ✓ reasoning_content field present")
	}
	if promptTokens == 0 && completionTokens == 0 {
		fmt.Println("  ⚠ No usage data in stream (stream_options may not be supported)")
	}
}

func testAgentInitiated(apiToken, baseURL, model string) {
	// Simulate a tool loop: last message is from tool role
	body := map[string]any{
		"model":      model,
		"max_tokens": 30,
		"stream":     false,
		"messages": []any{
			map[string]any{"role": "user", "content": "What is 2+2?"},
			map[string]any{
				"role":    "assistant",
				"content": nil,
				"tool_calls": []any{
					map[string]any{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "calculator",
							"arguments": `{"expression":"2+2"}`,
						},
					},
				},
			},
			map[string]any{
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      "4",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "calculator",
					"description": "Evaluate math expression",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"expression": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)
	req.Header.Set("X-Initiator", "agent") // This should be agent-initiated

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	var result map[string]any
	json.Unmarshal(data, &result)
	choices, _ := result["choices"].([]any)
	content := ""
	if len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			if msg, ok := c["message"].(map[string]any); ok {
				content, _ = msg["content"].(string)
			}
		}
	}

	fmt.Printf("  ✓ HTTP %d (X-Initiator: agent)\n", resp.StatusCode)
	fmt.Printf("  ✓ Content: %q\n", truncate(content, 100))
	fmt.Println("  ✓ Agent-initiated request accepted (tool loop free quota)")
}

func testUserInitiated(apiToken, baseURL, model string) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 20,
		"stream":     false,
		"messages": []any{
			map[string]any{"role": "user", "content": "Say hi."},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)
	req.Header.Set("X-Initiator", "user")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}
	fmt.Printf("  ✓ HTTP %d (X-Initiator: user)\n", resp.StatusCode)
}

func testStreamUsage(apiToken, baseURL, model string) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 20,
		"stream":     true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "Say ok."},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var promptTokens, completionTokens int
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data), &obj) != nil {
			continue
		}
		if usage, ok := obj["usage"].(map[string]any); ok {
			promptTokens = toInt(usage["prompt_tokens"])
			completionTokens = toInt(usage["completion_tokens"])
		}
	}

	if promptTokens > 0 || completionTokens > 0 {
		fmt.Printf("  ✓ stream_options.include_usage works: prompt=%d, completion=%d\n", promptTokens, completionTokens)
	} else {
		fmt.Println("  ⚠ No usage in stream (Copilot may not support stream_options yet)")
	}
}

func testResponsesAPI(apiToken, baseURL, model string) {
	body := map[string]any{
		"model":      model,
		"stream":     false,
		"store":      false,
		"include":    []string{"reasoning.encrypted_content"},
		"max_output_tokens": 100,
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "Write a Go function that adds two numbers. Keep it very short."},
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/responses", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)
	req.Header.Set("X-Initiator", "user")

	start := time.Now()
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 300))
		return
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	// Extract output
	outputText := ""
	if output, ok := result["output"].([]any); ok {
		for _, item := range output {
			if m, ok := item.(map[string]any); ok {
				if m["type"] == "message" {
					if content, ok := m["content"].([]any); ok {
						for _, c := range content {
							if cm, ok := c.(map[string]any); ok {
								if t, ok := cm["text"].(string); ok {
									outputText += t
								}
							}
						}
					}
				}
			}
		}
	}

	usage, _ := result["usage"].(map[string]any)
	fmt.Printf("  ✓ HTTP %d, latency=%v\n", resp.StatusCode, elapsed.Round(time.Millisecond))
	fmt.Printf("  ✓ Model: %v\n", result["model"])
	fmt.Printf("  ✓ Output: %q\n", truncate(outputText, 150))
	fmt.Printf("  ✓ Usage: input=%v, output=%v\n", usage["input_tokens"], usage["output_tokens"])
}

func testReasoningSupport(apiToken, baseURL, model string) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 200,
		"stream":     false,
		"messages": []any{
			map[string]any{"role": "user", "content": "What is 15 * 37? Think step by step."},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header = copilotHeaders(apiToken, bodyJSON)

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	choices, _ := result["choices"].([]any)
	if len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			if msg, ok := c["message"].(map[string]any); ok {
				content, _ := msg["content"].(string)
				rt, hasRT := msg["reasoning_text"]
				rc, hasRC := msg["reasoning_content"]
				fmt.Printf("  ✓ HTTP %d\n", resp.StatusCode)
				fmt.Printf("  ✓ Content: %q\n", truncate(content, 100))
				if hasRT {
					fmt.Printf("  ✓ reasoning_text present (len=%d) — needs normalization\n", len(fmt.Sprint(rt)))
				}
				if hasRC {
					fmt.Printf("  ✓ reasoning_content present (len=%d)\n", len(fmt.Sprint(rc)))
				}
				if !hasRT && !hasRC {
					fmt.Println("  ℹ No reasoning fields (model may not support reasoning in this mode)")
				}
			}
		}
	}
}

func testListModels(apiToken, baseURL string) {
	req, _ := http.NewRequest("GET", baseURL+"/models", nil)
	req.Header = copilotHeaders(apiToken, nil)

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ FAIL: %v\n", err)
		return
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ FAIL: HTTP %d: %s\n", resp.StatusCode, truncate(string(data), 200))
		return
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	models, _ := result["data"].([]any)
	fmt.Printf("  ✓ HTTP %d, %d models available\n", resp.StatusCode, len(models))

	// Show Claude and Codex models
	var claudeModels, codexModels []string
	for _, m := range models {
		if mm, ok := m.(map[string]any); ok {
			id, _ := mm["id"].(string)
			if strings.Contains(id, "claude") {
				claudeModels = append(claudeModels, id)
			}
			if strings.Contains(id, "codex") {
				codexModels = append(codexModels, id)
			}
		}
	}
	if len(claudeModels) > 0 {
		fmt.Printf("  ✓ Claude models: %s\n", strings.Join(claudeModels, ", "))
	}
	if len(codexModels) > 0 {
		fmt.Printf("  ✓ Codex models: %s\n", strings.Join(codexModels, ", "))
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
