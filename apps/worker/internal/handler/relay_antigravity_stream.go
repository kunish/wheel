package handler

// relay_antigravity_stream.go implements the Gemini → Anthropic streaming SSE
// response conversion for the Antigravity relay. This is modeled after sub2api's
// stream_transformer.go and includes a full state machine with proper block
// lifecycle management, signature_delta events, grounding text at stream end,
// and correct stop_reason mapping.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// BlockType tracks the type of the currently open content block.
type BlockType int

const (
	BlockNone     BlockType = iota // No block open
	BlockText                      // Text content block
	BlockThinking                  // Thinking content block
	BlockFunction                  // Function call (tool_use) block
)

// StreamingProcessor manages the state machine for converting Gemini streaming
// responses to Anthropic SSE events.
type StreamingProcessor struct {
	w       http.ResponseWriter
	flusher http.Flusher
	model   string

	// State tracking.
	currentBlock     BlockType
	blockIndex       int
	messageStartSent bool
	messageStopSent  bool
	usedTool         bool

	// Signature handling.
	pendingSignature  string
	trailingSignature string

	// Grounding metadata accumulation.
	webSearchQueries []string
	groundingChunks  []GeminiGroundingChunk

	// Content accumulation for callback.
	accText     string
	accThinking string

	// Token counts.
	inputTokens  int
	outputTokens int
}

// NewStreamingProcessor creates a new streaming processor for the given writer.
func NewStreamingProcessor(w http.ResponseWriter, model string) *StreamingProcessor {
	flusher, _ := w.(http.Flusher)
	return &StreamingProcessor{
		w:       w,
		flusher: flusher,
		model:   model,
	}
}

// SendMessageStart sends the initial message_start event.
func (sp *StreamingProcessor) SendMessageStart() {
	if sp.messageStartSent {
		return
	}
	sp.messageStartSent = true

	sp.writeSSE("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      "msg_" + generateSecureID(),
			"type":    "message",
			"role":    "assistant",
			"model":   sp.model,
			"content": []any{},
			"usage":   map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	})
}

// ProcessChunk processes a single Gemini SSE chunk and emits corresponding
// Anthropic SSE events.
func (sp *StreamingProcessor) ProcessChunk(chunk []byte) {
	// Try V1InternalResponse first, then direct GeminiResponse.
	geminiResp := unwrapGeminiStreamChunk(chunk)
	if geminiResp == nil {
		return
	}

	// Ensure message_start is sent.
	sp.SendMessageStart()

	// Extract usage metadata.
	if geminiResp.UsageMetadata != nil {
		sp.inputTokens, sp.outputTokens, _ = extractUsageFromGemini(geminiResp.UsageMetadata)
	}

	// Process candidates.
	if len(geminiResp.Candidates) == 0 {
		return
	}
	candidate := geminiResp.Candidates[0]

	// Accumulate grounding metadata.
	if candidate.GroundingMetadata != nil {
		if len(candidate.GroundingMetadata.WebSearchQueries) > 0 {
			sp.webSearchQueries = candidate.GroundingMetadata.WebSearchQueries
		}
		if len(candidate.GroundingMetadata.GroundingChunks) > 0 {
			sp.groundingChunks = candidate.GroundingMetadata.GroundingChunks
		}
	}

	if candidate.Content == nil {
		return
	}

	isGemini := !strings.Contains(sp.model, "claude")

	for _, part := range candidate.Content.Parts {
		sp.processPart(part, isGemini)
	}
}

// processPart processes a single Gemini part and emits SSE events.
func (sp *StreamingProcessor) processPart(part GeminiPart, isGemini bool) {
	// Handle trailing signature (empty text with signature only).
	if part.ThoughtSignature != "" && part.Text == "" && part.FunctionCall == nil && part.FunctionResponse == nil {
		sp.trailingSignature = part.ThoughtSignature
		return
	}

	// Handle thought (thinking) parts.
	if part.Thought != nil && *part.Thought {
		if part.Text != "" {
			sp.startBlock(BlockThinking)
			sp.writeSSE("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": sp.blockIndex,
				"delta": map[string]any{"type": "thinking_delta", "thinking": part.Text},
			})
			sp.accThinking += part.Text
		}
		// Handle signature on thinking block end.
		if part.ThoughtSignature != "" {
			sp.pendingSignature = part.ThoughtSignature
		}
		return
	}

	// Handle text parts.
	if part.Text != "" {
		// If transitioning from thinking to text, close thinking block with signature.
		if sp.currentBlock == BlockThinking {
			sp.endBlockWithSignature(isGemini)
		}

		sp.startBlock(BlockText)
		sp.writeSSE("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": sp.blockIndex,
			"delta": map[string]any{"type": "text_delta", "text": part.Text},
		})
		sp.accText += part.Text
		return
	}

	// Handle inline data (images).
	if part.InlineData != nil {
		imgMarkdown := fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
		sp.startBlock(BlockText)
		sp.writeSSE("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": sp.blockIndex,
			"delta": map[string]any{"type": "text_delta", "text": imgMarkdown},
		})
		sp.accText += imgMarkdown
		return
	}

	// Handle function calls.
	if part.FunctionCall != nil {
		sp.endCurrentBlock(isGemini)
		sp.usedTool = true

		toolID := "toolu_" + generateSecureID()
		sp.startBlock(BlockFunction)
		sp.writeSSE("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": sp.blockIndex,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    toolID,
				"name":  part.FunctionCall.Name,
				"input": map[string]any{},
			},
		})

		argsJSON, _ := json.Marshal(part.FunctionCall.Args)
		sp.writeSSE("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": sp.blockIndex,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": string(argsJSON),
			},
		})

		// Handle signature on tool_use blocks.
		if part.ThoughtSignature != "" {
			sig := resolveSignature(part.ThoughtSignature, isGemini)
			if sig != "" {
				sp.writeSSE("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": sp.blockIndex,
					"delta": map[string]any{"type": "signature_delta", "signature": sig},
				})
			}
		}

		sp.writeSSE("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": sp.blockIndex,
		})
		sp.blockIndex++
		sp.currentBlock = BlockNone
	}
}

// startBlock opens a new content block, closing any existing one first.
func (sp *StreamingProcessor) startBlock(bt BlockType) {
	if sp.currentBlock == bt {
		return // Same block type, continue appending.
	}
	if sp.currentBlock != BlockNone {
		isGemini := !strings.Contains(sp.model, "claude")
		sp.endCurrentBlock(isGemini)
	}

	sp.currentBlock = bt

	switch bt {
	case BlockThinking:
		sp.writeSSE("content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         sp.blockIndex,
			"content_block": map[string]any{"type": "thinking", "thinking": ""},
		})
	case BlockText:
		sp.writeSSE("content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         sp.blockIndex,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
	}
}

// endCurrentBlock closes the current content block.
func (sp *StreamingProcessor) endCurrentBlock(isGemini bool) {
	if sp.currentBlock == BlockNone {
		return
	}

	// Emit signature_delta if there's a pending signature.
	if sp.currentBlock == BlockThinking && sp.pendingSignature != "" {
		sig := resolveSignature(sp.pendingSignature, isGemini)
		if sig != "" {
			sp.writeSSE("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": sp.blockIndex,
				"delta": map[string]any{"type": "signature_delta", "signature": sig},
			})
		}
		sp.pendingSignature = ""
	}

	sp.writeSSE("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": sp.blockIndex,
	})
	sp.blockIndex++
	sp.currentBlock = BlockNone
}

// endBlockWithSignature closes a thinking block and emits the signature.
func (sp *StreamingProcessor) endBlockWithSignature(isGemini bool) {
	if sp.currentBlock != BlockThinking {
		return
	}

	// Emit signature_delta.
	if sp.pendingSignature != "" {
		sig := resolveSignature(sp.pendingSignature, isGemini)
		if sig != "" {
			sp.writeSSE("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": sp.blockIndex,
				"delta": map[string]any{"type": "signature_delta", "signature": sig},
			})
		}
		sp.pendingSignature = ""
	}

	sp.writeSSE("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": sp.blockIndex,
	})
	sp.blockIndex++
	sp.currentBlock = BlockNone
}

// emitEmptyThinkingWithSignature creates an empty thinking block with just a signature.
// Used for trailing signatures at stream end.
func (sp *StreamingProcessor) emitEmptyThinkingWithSignature(sig string, isGemini bool) {
	resolvedSig := resolveSignature(sig, isGemini)
	if resolvedSig == "" {
		return
	}

	sp.writeSSE("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         sp.blockIndex,
		"content_block": map[string]any{"type": "thinking", "thinking": ""},
	})
	sp.writeSSE("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": sp.blockIndex,
		"delta": map[string]any{"type": "thinking_delta", "thinking": ""},
	})
	sp.writeSSE("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": sp.blockIndex,
		"delta": map[string]any{"type": "signature_delta", "signature": resolvedSig},
	})
	sp.writeSSE("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": sp.blockIndex,
	})
	sp.blockIndex++
}

// Finish handles stream end: emits grounding text, trailing signatures,
// message_delta with stop_reason, and message_stop.
// Returns the accumulated usage.
func (sp *StreamingProcessor) Finish() (inputTokens, outputTokens int) {
	if !sp.messageStartSent {
		// No data was ever received; don't emit orphaned events.
		return 0, 0
	}

	isGemini := !strings.Contains(sp.model, "claude")

	// Close any open block.
	sp.endCurrentBlock(isGemini)

	// Emit grounding text as a final text block.
	groundingText := sp.buildStreamGroundingText()
	if groundingText != "" {
		sp.startBlock(BlockText)
		sp.writeSSE("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": sp.blockIndex,
			"delta": map[string]any{"type": "text_delta", "text": groundingText},
		})
		sp.accText += groundingText
		sp.endCurrentBlock(isGemini)
	}

	// Handle trailing signature.
	if sp.trailingSignature != "" {
		sp.emitEmptyThinkingWithSignature(sp.trailingSignature, isGemini)
		sp.trailingSignature = ""
	}

	// Determine stop reason.
	stopReason := "end_turn"
	if sp.usedTool {
		stopReason = "tool_use"
	}

	// Send message_delta with stop_reason.
	sp.writeSSE("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason},
		"usage": map[string]any{"output_tokens": sp.outputTokens},
	})

	// Send message_stop.
	sp.writeSSE("message_stop", map[string]any{
		"type": "message_stop",
	})

	sp.messageStopSent = true
	return sp.inputTokens, sp.outputTokens
}

// MessageStartSent returns whether the initial message_start was sent.
func (sp *StreamingProcessor) MessageStartSent() bool {
	return sp.messageStartSent
}

// AccText returns accumulated text content.
func (sp *StreamingProcessor) AccText() string {
	return sp.accText
}

// AccThinking returns accumulated thinking content.
func (sp *StreamingProcessor) AccThinking() string {
	return sp.accThinking
}

// buildStreamGroundingText builds grounding text from accumulated metadata.
func (sp *StreamingProcessor) buildStreamGroundingText() string {
	if len(sp.webSearchQueries) == 0 && len(sp.groundingChunks) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "\n\n---\n**Sources:**")

	for _, chunk := range sp.groundingChunks {
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

// writeSSE writes a single SSE event.
func (sp *StreamingProcessor) writeSSE(eventType string, data any) {
	writeAnthropicSSE(sp.w, sp.flusher, eventType, data)
}

// ──────────────────────────────────────────────────────────────
// Stream chunk parsing
// ──────────────────────────────────────────────────────────────

// unwrapGeminiStreamChunk parses a streaming chunk, handling both
// V1InternalResponse envelope and direct GeminiResponse formats.
func unwrapGeminiStreamChunk(data []byte) *GeminiResponse {
	// Try V1InternalResponse envelope.
	var v1 V1InternalResponse
	if json.Unmarshal(data, &v1) == nil && v1.Response != nil {
		return v1.Response
	}

	// Try direct GeminiResponse.
	var gemini GeminiResponse
	if json.Unmarshal(data, &gemini) == nil {
		return &gemini
	}

	// Fallback map-based parsing.
	var raw map[string]any
	if json.Unmarshal(data, &raw) == nil {
		if resp, ok := raw["response"]; ok {
			respBytes, _ := json.Marshal(resp)
			var g GeminiResponse
			if json.Unmarshal(respBytes, &g) == nil {
				return &g
			}
		}
	}

	return nil
}
