package handler

// relay_antigravity_request.go implements the Anthropic → Gemini request conversion
// for the Antigravity relay. This is modeled after sub2api's request_transformer.go
// and includes all advanced features: identity patch injection, MCP XML protocol,
// OpenCode prompt filtering, stable session IDs, signature handling, web_search
// detection, schema cleaning, safety settings, and generation config defaults.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/util"
)

// transformClaudeToGemini performs the full Anthropic → Gemini request conversion.
// It takes the raw request body (map[string]any) and returns a V1InternalRequest envelope
// ready to send to the Antigravity upstream API.
func transformClaudeToGemini(body map[string]any, model string, projectID string) V1InternalRequest {
	// Antigravity upstream registers Claude models with the "-thinking" suffix.
	// The bare name (e.g. "claude-opus-4-6") returns 404, while the suffixed
	// name (e.g. "claude-opus-4-6-thinking") is recognized. Ensure the suffix
	// is always present for Claude models.
	if strings.Contains(model, "claude") && !strings.HasSuffix(model, "-thinking") {
		model = model + "-thinking"
	}

	// Parse the body into structured types for safer manipulation.
	messages := extractMessages(body)
	systemText := extractSystemText(body["system"])
	tools := extractTools(body)

	// Detect features.
	hasWebSearch := detectWebSearchTool(tools)
	hasMCPTools := detectMCPTools(tools)

	// Apply model fallback for web search.
	if hasWebSearch {
		model = WebSearchFallbackModel
	}

	// Build tool ID→name mapping from assistant messages.
	toolIDToName := buildToolIDToNameMap(messages)

	// Convert messages to Gemini contents.
	contents := convertMessagesToContents(messages, model, toolIDToName)

	// Build system instruction with patches.
	sysInstruction := buildSystemInstruction(systemText, hasMCPTools)

	// Build generation config.
	genConfig := buildGenerationConfig(body, model)

	// Build tools.
	geminiTools := convertToolsToGemini(tools, hasWebSearch)

	// Build tool config.
	toolConfig := buildToolConfig(body["tool_choice"])

	// Build the Gemini request (safetySettings omitted per upstream protocol).
	geminiReq := GeminiRequest{
		Contents:         contents,
		GenerationConfig: genConfig,
	}
	if sysInstruction != nil {
		geminiReq.SystemInstruction = sysInstruction
	}
	if len(geminiTools) > 0 {
		geminiReq.Tools = geminiTools
		if toolConfig != nil {
			geminiReq.ToolConfig = toolConfig
		}
	}

	// CLIProxyAPIPlus behavior: Claude models on Antigravity MUST have
	// toolConfig.functionCallingConfig.mode = "VALIDATED", even when no
	// tools are declared. This is set in buildRequest() unconditionally
	// for Claude models.
	if strings.Contains(model, "claude") {
		geminiReq.ToolConfig = &GeminiToolConfig{
			FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "VALIDATED"},
		}
	}

	// Build the V1Internal envelope.
	requestType := "agent"
	if hasWebSearch {
		requestType = "web_search"
	}

	if projectID == "" {
		projectID = generateRandomProjectID()
	}

	// Place sessionId inside the request object (not at envelope level).
	sessionID := generateStableSessionID(messages)
	geminiReq.SessionID = sessionID

	// Remove maxOutputTokens for all models — the Antigravity upstream
	// infers appropriate limits. Sending a value that exceeds the upstream
	// limit (e.g. 128000 for Claude) causes INVALID_ARGUMENT.
	if genConfig != nil {
		genConfig.MaxOutputTokens = 0
	}

	return V1InternalRequest{
		Project:     projectID,
		RequestID:   "agent-" + uuid.New().String(),
		Model:       model,
		UserAgent:   "antigravity",
		Request:     geminiReq,
		RequestType: requestType,
	}
}

// ──────────────────────────────────────────────────────────────
// Message extraction and conversion
// ──────────────────────────────────────────────────────────────

// extractMessages extracts messages from the body.
func extractMessages(body map[string]any) []ClaudeMessage {
	rawMsgs, _ := body["messages"].([]any)
	msgs := make([]ClaudeMessage, 0, len(rawMsgs))
	for _, m := range rawMsgs {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := mMap["role"].(string)
		msgs = append(msgs, ClaudeMessage{
			Role:    role,
			Content: mMap["content"],
		})
	}
	return msgs
}

// extractTools extracts tool definitions from the body.
func extractTools(body map[string]any) []ClaudeTool {
	rawTools, _ := body["tools"].([]any)
	if len(rawTools) == 0 {
		return nil
	}
	tools := make([]ClaudeTool, 0, len(rawTools))
	for _, t := range rawTools {
		tMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		tool := ClaudeTool{
			Type:        getStr(tMap, "type"),
			Name:        getStr(tMap, "name"),
			Description: getStr(tMap, "description"),
			InputSchema: tMap["input_schema"],
		}
		// Handle custom/MCP tools with nested spec.
		if tool.Type == "custom" {
			if spec, ok := tMap["custom_tool_spec"].(map[string]any); ok {
				tool.CustomToolSpec = &CustomToolSpec{
					Name:        getStr(spec, "name"),
					Description: getStr(spec, "description"),
					InputSchema: spec["input_schema"],
				}
			}
		}
		tools = append(tools, tool)
	}
	return tools
}

// buildToolIDToNameMap scans assistant messages for tool_use blocks and builds a map
// from tool_use IDs to their names. This is needed to resolve tool_result.name
// from tool_result.tool_use_id.
func buildToolIDToNameMap(messages []ClaudeMessage) map[string]string {
	m := make(map[string]string)
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		blocks := contentToBlocks(msg.Content)
		for _, blk := range blocks {
			if blk.Type == "tool_use" && blk.ID != "" && blk.Name != "" {
				m[blk.ID] = blk.Name
			}
		}
	}
	return m
}

// convertMessagesToContents converts Claude messages to Gemini contents.
func convertMessagesToContents(messages []ClaudeMessage, model string, toolIDToName map[string]string) []GeminiContent {
	isGemini := !strings.Contains(model, "claude")
	var contents []GeminiContent

	for _, msg := range messages {
		geminiRole := "user"
		if msg.Role == "assistant" {
			geminiRole = "model"
		}

		parts := convertContentToParts(msg.Content, msg.Role, isGemini, toolIDToName)
		if len(parts) > 0 {
			contents = append(contents, GeminiContent{
				Role:  geminiRole,
				Parts: parts,
			})
		}
	}
	return contents
}

// convertContentToParts converts message content to Gemini parts.
func convertContentToParts(content any, role string, isGemini bool, toolIDToName map[string]string) []GeminiPart {
	// Simple string content.
	if s, ok := content.(string); ok {
		return []GeminiPart{{Text: s}}
	}

	blocks := contentToBlocks(content)
	var parts []GeminiPart

	for _, blk := range blocks {
		switch blk.Type {
		case "text":
			if blk.Text != "" {
				parts = append(parts, GeminiPart{Text: blk.Text})
			}

		case "thinking":
			if blk.Thinking != "" {
				thought := true
				part := GeminiPart{
					Text:    blk.Thinking,
					Thought: &thought,
				}
				// Signature handling: Gemini models get dummy signatures,
				// Claude models pass through real signatures.
				if blk.Signature != "" {
					if isGemini {
						part.ThoughtSignature = DummyThoughtSignature
					} else {
						part.ThoughtSignature = blk.Signature
					}
				} else if isGemini {
					// Gemini models always need a signature on thinking blocks.
					part.ThoughtSignature = DummyThoughtSignature
				}
				parts = append(parts, part)
			}

		case "tool_use":
			args := blk.Input
			if args == nil {
				args = map[string]any{}
			}
			part := GeminiPart{
				FunctionCall: &GeminiFunctionCall{
					Name: blk.Name,
					Args: args,
				},
			}
			// tool_use blocks also carry thought signatures in sub2api.
			if blk.Signature != "" {
				if isGemini {
					part.ThoughtSignature = DummyThoughtSignature
				} else {
					part.ThoughtSignature = blk.Signature
				}
			}
			parts = append(parts, part)

		case "tool_result":
			resultContent := extractToolResultText(blk.Content, blk.IsError)
			// Resolve the tool name from the ID→name map.
			toolName := blk.Name
			if toolName == "" && blk.ToolUseID != "" {
				if name, ok := toolIDToName[blk.ToolUseID]; ok {
					toolName = name
				} else {
					toolName = blk.ToolUseID // fallback
				}
			}
			parts = append(parts, GeminiPart{
				FunctionResponse: &GeminiFuncResponse{
					Name: toolName,
					Response: map[string]any{
						"output": resultContent,
					},
				},
			})

		case "image":
			if blk.Source != nil {
				parts = append(parts, GeminiPart{
					InlineData: &GeminiInlineData{
						MimeType: blk.Source.MediaType,
						Data:     blk.Source.Data,
					},
				})
			}
		}
	}
	return parts
}

// extractToolResultText extracts text from tool_result content with fallback messages.
func extractToolResultText(content any, isError *bool) string {
	if content == nil {
		if isError != nil && *isError {
			return "Tool execution failed with no output."
		}
		return "Command executed successfully."
	}

	if s, ok := content.(string); ok {
		if s == "" {
			if isError != nil && *isError {
				return "Tool execution failed with no output."
			}
			return "Command executed successfully."
		}
		return s
	}

	if blocks, ok := content.([]any); ok {
		var texts []string
		for _, b := range blocks {
			if m, ok := b.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					texts = append(texts, t)
				}
			}
		}
		result := strings.Join(texts, "\n")
		if result == "" {
			if isError != nil && *isError {
				return "Tool execution failed with no output."
			}
			return "Command executed successfully."
		}
		return result
	}

	b, _ := json.Marshal(content)
	return string(b)
}

// contentToBlocks converts content (string or []any) to []ClaudeContentItem.
func contentToBlocks(content any) []ClaudeContentItem {
	if content == nil {
		return nil
	}
	if s, ok := content.(string); ok {
		return []ClaudeContentItem{{Type: "text", Text: s}}
	}
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}
	items := make([]ClaudeContentItem, 0, len(blocks))
	for _, b := range blocks {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		item := ClaudeContentItem{
			Type:      getStr(m, "type"),
			Text:      getStr(m, "text"),
			Thinking:  getStr(m, "thinking"),
			Signature: getStr(m, "signature"),
			ID:        getStr(m, "id"),
			Name:      getStr(m, "name"),
			ToolUseID: getStr(m, "tool_use_id"),
			Content:   m["content"],
		}
		if isErr, ok := m["is_error"].(bool); ok {
			item.IsError = &isErr
		}
		if inputMap, ok := m["input"].(map[string]any); ok {
			item.Input = inputMap
		}
		if src, ok := m["source"].(map[string]any); ok {
			item.Source = &ImageSource{
				Type:      getStr(src, "type"),
				MediaType: getStr(src, "media_type"),
				Data:      getStr(src, "data"),
			}
		}
		items = append(items, item)
	}
	return items
}

// ──────────────────────────────────────────────────────────────
// System instruction building
// ──────────────────────────────────────────────────────────────

// buildSystemInstruction constructs the system instruction with identity patch
// and optional MCP XML protocol injection.
func buildSystemInstruction(rawSystemText string, hasMCPTools bool) *GeminiContent {
	sysText := filterOpenCodePrompt(rawSystemText)
	// Filter out literal "null" strings from system text.
	if strings.TrimSpace(sysText) == "null" {
		sysText = ""
	}
	sysText = filterSystemPrefixes(sysText)
	sysText = injectIdentityPatch(sysText)
	if hasMCPTools {
		sysText += mcpXMLProtocol
	}
	if sysText == "" {
		return nil
	}
	return &GeminiContent{
		Parts: []GeminiPart{{Text: sysText}},
	}
}

// injectIdentityPatch injects the "You are Antigravity" identity text
// if it's not already present, with silent boundary markers.
func injectIdentityPatch(sysText string) string {
	if strings.Contains(sysText, identityPatchText) {
		return sysText // Already has identity text.
	}
	if strings.Contains(sysText, identityBoundaryStart) {
		return sysText // Already has boundary markers.
	}
	patch := identityBoundaryStart + "\n" + identityPatchText + "\n" + identityBoundaryEnd
	if sysText == "" {
		return patch
	}
	return patch + "\n\n" + sysText
}

// filterOpenCodePrompt strips default OpenCode prompts, keeping only user custom instructions.
func filterOpenCodePrompt(sysText string) string {
	for _, prefix := range openCodeDefaultPromptPrefixes {
		if strings.HasPrefix(sysText, prefix) {
			// Try to find a clear separator (double newline) where user instructions start.
			if idx := strings.Index(sysText, "\n\n---\n\n"); idx != -1 {
				return strings.TrimSpace(sysText[idx+7:])
			}
			// If no separator, look for "Custom instructions:" marker.
			if idx := strings.Index(sysText, "Custom instructions:"); idx != -1 {
				return strings.TrimSpace(sysText[idx:])
			}
			// No user instructions found, remove the entire default prompt.
			return ""
		}
	}
	return sysText
}

// filterSystemPrefixes removes known system block prefixes.
func filterSystemPrefixes(sysText string) string {
	lines := strings.SplitN(sysText, "\n", 2)
	if len(lines) == 0 {
		return sysText
	}
	first := strings.TrimSpace(lines[0])
	// Filter x-anthropic-billing-header and similar prefixes.
	if strings.HasPrefix(first, "x-anthropic-") || strings.HasPrefix(first, "X-Anthropic-") {
		if len(lines) > 1 {
			return strings.TrimSpace(lines[1])
		}
		return ""
	}
	return sysText
}

// ──────────────────────────────────────────────────────────────
// Generation config
// ──────────────────────────────────────────────────────────────

// buildGenerationConfig creates the Gemini generation config with defaults.
func buildGenerationConfig(body map[string]any, model string) *GeminiGenerationConfig {
	gc := &GeminiGenerationConfig{
		MaxOutputTokens: DefaultMaxOutputTokens,
	}

	// Default stop sequences only for non-Claude models.
	// CLIProxyAPIPlus does not inject stopSequences for Claude models;
	// the upstream may reject them as invalid arguments.
	if !strings.Contains(model, "claude") {
		gc.StopSequences = DefaultStopSequences
	}

	// Model-specific max output tokens.
	if strings.Contains(model, "gemini") {
		gc.MaxOutputTokens = GeminiMaxOutputTokens
	}

	// Override with explicit values from request.
	if t, ok := body["temperature"]; ok {
		if f, ok := toFloat64(t); ok {
			gc.Temperature = &f
		}
	}
	if tp, ok := body["top_p"]; ok {
		if f, ok := toFloat64(tp); ok {
			gc.TopP = &f
		}
	}
	if tk, ok := body["top_k"]; ok {
		if i := toIntVal(tk); i > 0 {
			gc.TopK = &i
		}
	}
	if mt, ok := body["max_tokens"]; ok {
		if i := toIntVal(mt); i > 0 {
			gc.MaxOutputTokens = i
		}
	}
	if ss, ok := body["stop_sequences"].([]any); ok && len(ss) > 0 {
		seqs := make([]string, 0, len(ss))
		for _, s := range ss {
			if str, ok := s.(string); ok {
				seqs = append(seqs, str)
			}
		}
		if len(seqs) > 0 {
			// Merge with defaults, dedup.
			merged := mergeStopSequences(DefaultStopSequences, seqs)
			gc.StopSequences = merged
		}
	}

	// Thinking config.
	if thinking, ok := body["thinking"].(map[string]any); ok {
		if thinkType, _ := thinking["type"].(string); thinkType == "enabled" {
			budget := toIntVal(thinking["budget_tokens"])
			if budget > 0 {
				gc.ThinkingConfig = &GeminiThinkingConfig{
					ThinkingBudget:  budget,
					IncludeThoughts: true,
				}
				// Ensure max_tokens > budget.
				gc = ensureMaxTokensGreaterThanBudget(gc, budget)
				// Cap flash model thinking budget.
				if strings.Contains(model, "flash") && budget > GeminiFlashThinkingBudgetCap {
					gc.ThinkingConfig.ThinkingBudget = GeminiFlashThinkingBudgetCap
				}
			}
		}
	}
	// CLIProxyAPIPlus behavior: do NOT force thinkingConfig when the client
	// did not request it. The upstream will reject unexpected thinkingConfig.

	return gc
}

// ensureMaxTokensGreaterThanBudget ensures max_output_tokens > thinking budget.
func ensureMaxTokensGreaterThanBudget(gc *GeminiGenerationConfig, budget int) *GeminiGenerationConfig {
	if gc.MaxOutputTokens <= budget {
		gc.MaxOutputTokens = budget + 1024
	}
	return gc
}

// mergeStopSequences merges two stop sequence lists, deduplicating.
func mergeStopSequences(defaults, custom []string) []string {
	seen := make(map[string]struct{}, len(defaults)+len(custom))
	result := make([]string, 0, len(defaults)+len(custom))
	for _, s := range defaults {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	for _, s := range custom {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// ──────────────────────────────────────────────────────────────
// Tool conversion
// ──────────────────────────────────────────────────────────────

// convertToolsToGemini converts Claude tools to Gemini format with schema cleaning.
func convertToolsToGemini(tools []ClaudeTool, hasWebSearch bool) []GeminiToolDeclaration {
	if len(tools) == 0 && !hasWebSearch {
		return nil
	}

	var result []GeminiToolDeclaration

	// Regular function declarations.
	var declarations []GeminiFunctionDecl
	for _, tool := range tools {
		// Skip web_search/google_search tools — handled separately.
		if tool.Name == "web_search" || tool.Name == "google_search" {
			continue
		}

		name := tool.Name
		desc := tool.Description
		schema := tool.InputSchema

		// Handle MCP custom tools.
		if tool.Type == "custom" && tool.CustomToolSpec != nil {
			name = tool.CustomToolSpec.Name
			desc = tool.CustomToolSpec.Description
			schema = tool.CustomToolSpec.InputSchema
		}

		// Clean the schema for Gemini compatibility.
		cleanedSchema := cleanToolSchema(schema)

		declarations = append(declarations, GeminiFunctionDecl{
			Name:                 name,
			Description:          desc,
			ParametersJSONSchema: cleanedSchema,
		})
	}

	if len(declarations) > 0 {
		result = append(result, GeminiToolDeclaration{
			FunctionDeclarations: declarations,
		})
	}

	// Add Google Search tool if web_search was detected.
	if hasWebSearch {
		result = append(result, GeminiToolDeclaration{
			GoogleSearch: &GeminiGoogleSearch{
				EnhancedContent: &GeminiEnhancedContent{
					ImageSearch: true,
				},
			},
		})
	}

	return result
}

// cleanToolSchema runs the Antigravity schema cleaner on a tool's input schema.
func cleanToolSchema(schema any) any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	cleaned := util.CleanJSONSchemaForAntigravity(string(schemaJSON))

	var result any
	if json.Unmarshal([]byte(cleaned), &result) != nil {
		return schema // fallback to original on parse error
	}
	return result
}

// detectWebSearchTool checks if any tool is a web_search or google_search.
func detectWebSearchTool(tools []ClaudeTool) bool {
	for _, t := range tools {
		if t.Name == "web_search" || t.Name == "google_search" {
			return true
		}
	}
	return false
}

// detectMCPTools checks if any tool has the "mcp__" prefix.
func detectMCPTools(tools []ClaudeTool) bool {
	for _, t := range tools {
		if strings.HasPrefix(t.Name, "mcp__") {
			return true
		}
	}
	return false
}

// buildToolConfig builds the Gemini tool calling config.
// Default mode is VALIDATED to match sub2api behavior.
func buildToolConfig(toolChoice any) *GeminiToolConfig {
	if toolChoice == nil {
		return &GeminiToolConfig{
			FunctionCallingConfig: GeminiFunctionCallingConfig{
				Mode: "VALIDATED",
			},
		}
	}

	// String tool_choice.
	if s, ok := toolChoice.(string); ok {
		switch s {
		case "auto":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "AUTO"},
			}
		case "any":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "ANY"},
			}
		case "none":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "NONE"},
			}
		}
	}

	// Map tool_choice.
	if m, ok := toolChoice.(map[string]any); ok {
		tcType, _ := m["type"].(string)
		switch tcType {
		case "auto":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "AUTO"},
			}
		case "any":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "ANY"},
			}
		case "none":
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "NONE"},
			}
		case "tool":
			name, _ := m["name"].(string)
			return &GeminiToolConfig{
				FunctionCallingConfig: GeminiFunctionCallingConfig{
					Mode:                 "ANY",
					AllowedFunctionNames: []string{name},
				},
			}
		}
	}

	return &GeminiToolConfig{
		FunctionCallingConfig: GeminiFunctionCallingConfig{Mode: "VALIDATED"},
	}
}

// ──────────────────────────────────────────────────────────────
// Session ID and helpers
// ──────────────────────────────────────────────────────────────

// generateStableSessionID generates a deterministic session ID from the first
// user message content using SHA256. This ensures the same conversation
// always maps to the same session ID.
func generateStableSessionID(messages []ClaudeMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			var text string
			if s, ok := msg.Content.(string); ok {
				text = s
			} else {
				blocks := contentToBlocks(msg.Content)
				for _, blk := range blocks {
					if blk.Type == "text" && blk.Text != "" {
						text = blk.Text
						break
					}
				}
			}
			if text != "" {
				hash := sha256.Sum256([]byte(text))
				return fmt.Sprintf("%x", hash[:16]) // 32 hex chars
			}
		}
	}
	// Fallback to UUID if no user message found.
	return uuid.New().String()
}

// generateRandomProjectID generates a random project ID in the format used
// by CLIProxyAPIPlus (e.g. "swift-wave-a3b2c") when no real project ID is available.
func generateRandomProjectID() string {
	adjectives := []string{"useful", "bright", "swift", "calm", "bold", "keen", "warm", "cool", "fair", "deep"}
	nouns := []string{"fuze", "wave", "spark", "flow", "core", "beam", "glow", "node", "link", "edge"}
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	adj := adjectives[int(buf[0])%len(adjectives)]
	noun := nouns[int(buf[1])%len(nouns)]
	suffix := fmt.Sprintf("%x", buf)
	return adj + "-" + noun + "-" + suffix
}

// getStr safely extracts a string from a map.
func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// toFloat64 converts a numeric value to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// extractSystemTextForAntigravity extracts system text from various Anthropic system formats.
// This is the Antigravity-specific version that works with the new type system.
func extractSystemTextForAntigravity(sys any) string {
	return extractSystemText(sys) // reuse existing function
}
