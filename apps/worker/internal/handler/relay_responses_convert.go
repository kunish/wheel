package handler

import "github.com/kunish/wheel/apps/worker/internal/types"

// needsResponsesConversion returns true for channel types that read body["messages"]
// directly and therefore require Responses API input to be converted to Chat Completions format.
// OpenAI-compatible channels (including Azure) pass the body through as-is and handle
// the Responses API format natively, so they don't need conversion.
func needsResponsesConversion(channelType types.OutboundType) bool {
	switch channelType {
	case types.OutboundAnthropic,
		types.OutboundGemini,
		types.OutboundBedrock,
		types.OutboundVertex,
		types.OutboundCohere:
		return true
	default:
		return false
	}
}

// convertResponsesBodyToChatCompletions converts an OpenAI Responses API request body
// (which uses "input" field) into an OpenAI Chat Completions format (which uses "messages").
// This allows non-OpenAI adapters (Anthropic, Gemini, Bedrock, etc.) to process the request
// correctly since they all expect the Chat Completions message schema.
func convertResponsesBodyToChatCompletions(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))

	// Copy passthrough fields
	for k, v := range body {
		switch k {
		case "input", "instructions":
			// handled below
		default:
			out[k] = v
		}
	}

	var messages []any

	// instructions → system message
	if instr, ok := body["instructions"].(string); ok && instr != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": instr,
		})
	}

	input := body["input"]
	switch v := input.(type) {
	case string:
		// Simple string input → single user message
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": v,
		})

	case []any:
		// Array of input items
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}

			role, _ := entry["role"].(string)
			typ, _ := entry["type"].(string)

			// If no explicit type but has role, treat as "message"
			if typ == "" && role != "" {
				typ = "message"
			}

			switch typ {
			case "message":
				msg := convertResponsesMessage(entry, role)
				if msg != nil {
					// Extract system messages for the instructions if not already present
					if role == "system" && !hasSystemMessage(messages) {
						messages = append(messages, msg)
					} else if role != "system" {
						messages = append(messages, msg)
					}
				}

			case "function_call":
				// → assistant message with tool_calls
				callID, _ := entry["call_id"].(string)
				name, _ := entry["name"].(string)
				args, _ := entry["arguments"].(string)

				toolCall := map[string]any{
					"id":   callID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": args,
					},
				}
				messages = append(messages, map[string]any{
					"role":       "assistant",
					"content":    nil,
					"tool_calls": []any{toolCall},
				})

			case "function_call_output":
				// → tool message
				callID, _ := entry["call_id"].(string)
				output, _ := entry["output"].(string)
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": callID,
					"content":      output,
				})
			}
		}
	}

	if len(messages) > 0 {
		out["messages"] = messages
	}

	return out
}

// convertResponsesMessage converts a Responses API message item to a Chat Completions message.
func convertResponsesMessage(entry map[string]any, role string) map[string]any {
	content := entry["content"]

	switch c := content.(type) {
	case string:
		// Simple string content
		if role == "" {
			role = "user"
		}
		return map[string]any{
			"role":    role,
			"content": c,
		}

	case []any:
		// Array of content parts — convert input_text/output_text to text
		var parts []any
		inferredRole := ""

		for _, part := range c {
			p, ok := part.(map[string]any)
			if !ok {
				continue
			}
			ptype, _ := p["type"].(string)
			switch ptype {
			case "input_text":
				text, _ := p["text"].(string)
				parts = append(parts, map[string]any{
					"type": "text",
					"text": text,
				})
				inferredRole = "user"

			case "output_text":
				text, _ := p["text"].(string)
				parts = append(parts, map[string]any{
					"type": "text",
					"text": text,
				})
				inferredRole = "assistant"

			case "input_image":
				url := getImageURL(p)
				if url != "" {
					parts = append(parts, map[string]any{
						"type":      "image_url",
						"image_url": map[string]any{"url": url},
					})
					inferredRole = "user"
				}

			default:
				// Pass through unrecognized content parts
				parts = append(parts, p)
			}
		}

		if role == "" {
			role = inferredRole
		}
		if role == "" {
			role = "user"
		}

		// If single text part, use simple string content
		if len(parts) == 1 {
			if text, ok := parts[0].(map[string]any); ok && text["type"] == "text" {
				return map[string]any{
					"role":    role,
					"content": text["text"],
				}
			}
		}

		if len(parts) > 0 {
			return map[string]any{
				"role":    role,
				"content": parts,
			}
		}

		return nil

	default:
		if role == "" {
			role = "user"
		}
		return map[string]any{
			"role":    role,
			"content": "",
		}
	}
}

// getImageURL extracts the image URL from a Responses API input_image part.
func getImageURL(p map[string]any) string {
	if url, ok := p["image_url"].(string); ok && url != "" {
		return url
	}
	if url, ok := p["url"].(string); ok && url != "" {
		return url
	}
	return ""
}

// hasSystemMessage checks if a system message already exists in the messages slice.
func hasSystemMessage(messages []any) bool {
	for _, m := range messages {
		if msg, ok := m.(map[string]any); ok {
			if role, _ := msg["role"].(string); role == "system" {
				return true
			}
		}
	}
	return false
}
