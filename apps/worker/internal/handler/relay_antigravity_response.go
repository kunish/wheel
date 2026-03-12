package handler

// relay_antigravity_response.go implements the Gemini → Anthropic non-streaming
// response conversion for the Antigravity relay. This is modeled after sub2api's
// response_transformer.go and includes: V1InternalResponse unwrapping, grounding
// metadata conversion, inline data to markdown images, proper stop_reason mapping,
// signature handling, and accurate usage calculation.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// transformGeminiToClaudeResponse converts a Gemini response to Anthropic Messages format.
// Returns the Anthropic response, input tokens, and output tokens.
func transformGeminiToClaudeResponse(respBytes []byte, model string) (map[string]any, int, int) {
	geminiResp := unwrapGeminiResponse(respBytes)
	if geminiResp == nil {
		return buildEmptyClaudeResponse(model), 0, 0
	}

	// Extract usage.
	inputTokens, outputTokens, cacheReadTokens := extractUsageFromGemini(geminiResp.UsageMetadata)

	// Process candidates.
	var contentBlocks []any
	var usedTool bool
	var stopReason string

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]

		// Determine stop reason from finish reason.
		stopReason = mapFinishReason(candidate.FinishReason)

		if candidate.Content != nil {
			contentBlocks, usedTool = processResponseParts(candidate.Content.Parts, model)
		}

		// Append grounding text if present.
		if candidate.GroundingMetadata != nil {
			groundingText := buildGroundingText(candidate.GroundingMetadata)
			if groundingText != "" {
				contentBlocks = append(contentBlocks, map[string]any{
					"type": "text",
					"text": groundingText,
				})
			}
		}
	}

	// Override stop_reason if tool_use was detected.
	if usedTool {
		stopReason = "tool_use"
	}
	if stopReason == "" {
		stopReason = "end_turn"
	}

	if len(contentBlocks) == 0 {
		contentBlocks = []any{map[string]any{"type": "text", "text": ""}}
	}

	usage := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}
	if cacheReadTokens > 0 {
		usage["cache_read_input_tokens"] = cacheReadTokens
	}

	return map[string]any{
		"id":            "msg_" + generateSecureID(),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         usage,
	}, inputTokens, outputTokens
}

// processResponseParts converts Gemini response parts to Anthropic content blocks.
// Returns the content blocks and whether any tool_use was detected.
func processResponseParts(parts []GeminiPart, model string) ([]any, bool) {
	isGemini := !strings.Contains(model, "claude")
	var blocks []any
	var usedTool bool
	var pendingSignature string

	for _, part := range parts {
		// Handle trailing signature (empty text with signature only).
		if part.ThoughtSignature != "" && part.Text == "" && part.FunctionCall == nil {
			pendingSignature = part.ThoughtSignature
			continue
		}

		// Handle thought (thinking) parts.
		if part.Thought != nil && *part.Thought {
			sig := resolveSignature(part.ThoughtSignature, isGemini)
			block := map[string]any{
				"type":     "thinking",
				"thinking": part.Text,
			}
			if sig != "" {
				block["signature"] = sig
			}
			blocks = append(blocks, block)
			continue
		}

		// Handle text parts.
		if part.Text != "" {
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": part.Text,
			})
			continue
		}

		// Handle inline data (images).
		if part.InlineData != nil {
			imgMarkdown := fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": imgMarkdown,
			})
			continue
		}

		// Handle function calls.
		if part.FunctionCall != nil {
			usedTool = true
			toolID := "toolu_" + generateSecureID()
			block := map[string]any{
				"type":  "tool_use",
				"id":    toolID,
				"name":  part.FunctionCall.Name,
				"input": part.FunctionCall.Args,
			}
			// tool_use blocks can also carry signatures.
			if part.ThoughtSignature != "" {
				sig := resolveSignature(part.ThoughtSignature, isGemini)
				if sig != "" {
					block["signature"] = sig
				}
			}
			blocks = append(blocks, block)
		}
	}

	// Handle trailing signature: attach to a new empty thinking block.
	if pendingSignature != "" {
		sig := resolveSignature(pendingSignature, isGemini)
		if sig != "" {
			blocks = append(blocks, map[string]any{
				"type":      "thinking",
				"thinking":  "",
				"signature": sig,
			})
		}
	}

	return blocks, usedTool
}

// ──────────────────────────────────────────────────────────────
// Gemini response parsing
// ──────────────────────────────────────────────────────────────

// unwrapGeminiResponse tries to parse the response as V1InternalResponse first,
// then falls back to a direct GeminiResponse.
func unwrapGeminiResponse(data []byte) *GeminiResponse {
	// Try V1InternalResponse envelope.
	var v1 V1InternalResponse
	if json.Unmarshal(data, &v1) == nil && v1.Response != nil {
		return v1.Response
	}

	// Try direct GeminiResponse.
	var gemini GeminiResponse
	if json.Unmarshal(data, &gemini) == nil && (len(gemini.Candidates) > 0 || gemini.UsageMetadata != nil) {
		return &gemini
	}

	// Try map-based parsing as last resort.
	var raw map[string]any
	if json.Unmarshal(data, &raw) == nil {
		if resp, ok := raw["response"].(map[string]any); ok {
			respBytes, _ := json.Marshal(resp)
			var g GeminiResponse
			if json.Unmarshal(respBytes, &g) == nil {
				return &g
			}
		}
		// Try parsing the raw map directly as a GeminiResponse.
		var g GeminiResponse
		if json.Unmarshal(data, &g) == nil {
			return &g
		}
	}

	return nil
}

// extractUsageFromGemini extracts input, output, and cache-read tokens from Gemini usage metadata.
func extractUsageFromGemini(usage *GeminiUsageMetadata) (inputTokens, outputTokens, cacheReadTokens int) {
	if usage == nil {
		return 0, 0, 0
	}
	// input_tokens = promptTokenCount - cachedContentTokenCount
	inputTokens = usage.PromptTokenCount - usage.CachedContentTokenCount
	if inputTokens < 0 {
		inputTokens = usage.PromptTokenCount
	}
	// output_tokens = candidatesTokenCount + thoughtsTokenCount
	outputTokens = usage.CandidatesTokenCount + usage.ThoughtsTokenCount
	// cache_read_input_tokens = cachedContentTokenCount
	cacheReadTokens = usage.CachedContentTokenCount
	return
}

// mapFinishReason maps Gemini finish reasons to Anthropic stop reasons.
func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "end_turn"
	case "MAX_TOKENS":
		return "max_tokens"
	case "SAFETY":
		return "end_turn"
	case "MALFORMED_FUNCTION_CALL":
		// Log this but treat as end_turn.
		return "end_turn"
	default:
		return "end_turn"
	}
}

// ──────────────────────────────────────────────────────────────
// Grounding metadata
// ──────────────────────────────────────────────────────────────

// buildGroundingText converts grounding metadata to markdown text with source links.
func buildGroundingText(gm *GeminiGroundingMetadata) string {
	if gm == nil {
		return ""
	}

	var parts []string

	// Add search queries.
	if len(gm.WebSearchQueries) > 0 {
		parts = append(parts, "\n\n---\n**Sources:**")
	}

	// Add grounding chunks as markdown links.
	for _, chunk := range gm.GroundingChunks {
		if chunk.Web != nil && chunk.Web.URI != "" {
			title := chunk.Web.Title
			if title == "" {
				title = chunk.Web.URI
			}
			parts = append(parts, fmt.Sprintf("- [%s](%s)", title, chunk.Web.URI))
		}
	}

	return strings.Join(parts, "\n")
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// resolveSignature handles the Gemini/Claude signature distinction.
// Gemini models use dummy signatures; Claude models need real ones.
func resolveSignature(sig string, isGemini bool) string {
	if sig == "" {
		return ""
	}
	if isGemini {
		return DummyThoughtSignature
	}
	return sig
}

// generateSecureID generates a cryptographically secure random ID (12 bytes hex).
func generateSecureID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use a simple counter-based approach.
		return fmt.Sprintf("%012x", fastRandUint64())
	}
	return hex.EncodeToString(b)
}

// fastRandUint64 provides a quick non-crypto random number using xorshift.
// Only used as fallback when crypto/rand fails.
func fastRandUint64() uint64 {
	// Simple xorshift64 seeded from runtime.
	x := uint64(0x1234567890abcdef) // Fixed seed for fallback
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	return x
}

// buildEmptyClaudeResponse creates a minimal valid Claude response.
func buildEmptyClaudeResponse(model string) map[string]any {
	return map[string]any{
		"id":            "msg_" + generateSecureID(),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       []any{map[string]any{"type": "text", "text": ""}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
	}
}
