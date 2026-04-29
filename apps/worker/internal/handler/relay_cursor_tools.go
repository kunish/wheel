package handler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- Anthropic (tools) → Cursor /api/chat body ---

func cursorSanitizeAnthropicSystem(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var out []string
	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if strings.HasPrefix(strings.ToLower(l), "x-anthropic-billing-header") {
			continue
		}
		if strings.HasPrefix(l, "You are Claude Code") {
			continue
		}
		if strings.HasPrefix(l, "You are Claude,") && strings.Contains(l, "Anthropic") {
			continue
		}
		out = append(out, ln)
	}
	s = strings.Join(out, "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

func cursorAnthropicCombinedSystem(anth map[string]any) string {
	sys := anth["system"]
	if sys == nil {
		return ""
	}
	switch v := sys.(type) {
	case string:
		return cursorSanitizeAnthropicSystem(v)
	case []any:
		var b strings.Builder
		for _, blk := range v {
			m, ok := blk.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if tx, _ := m["text"].(string); tx != "" {
					if b.Len() > 0 {
						b.WriteByte('\n')
					}
					b.WriteString(tx)
				}
			}
		}
		return cursorSanitizeAnthropicSystem(b.String())
	default:
		raw, _ := json.Marshal(sys)
		return cursorSanitizeAnthropicSystem(string(raw))
	}
}

func cursorCompactSchema(schema map[string]any) string {
	if schema == nil {
		return "{}"
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return "{}"
	}
	reqRaw, _ := schema["required"].([]any)
	req := map[string]struct{}{}
	for _, r := range reqRaw {
		if s, ok := r.(string); ok {
			req[s] = struct{}{}
		}
	}
	var parts []string
	for name, propV := range props {
		prop, ok := propV.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := prop["type"].(string)
		if typ == "" {
			typ = "any"
		}
		if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
			var ss []string
			for _, e := range enum {
				if s, ok := e.(string); ok {
					ss = append(ss, s)
				}
			}
			typ = strings.Join(ss, "|")
		}
		if typ == "array" {
			if items, ok := prop["items"].(map[string]any); ok {
				it, _ := items["type"].(string)
				if it == "" {
					it = "any"
				}
				typ = it + "[]"
			}
		}
		if typ == "object" {
			if nested, ok := prop["properties"].(map[string]any); ok && len(nested) > 0 {
				typ = cursorCompactSchema(prop)
			}
		}
		mark := "?"
		if _, ok := req[name]; ok {
			mark = "!"
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s", name, mark, typ))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func cursorToolChoiceConstraint(toolChoice any) string {
	m, ok := toolChoice.(map[string]any)
	if !ok {
		return ""
	}
	typ, _ := m["type"].(string)
	switch typ {
	case "any":
		return "\n\n**MANDATORY**: Your response MUST include at least one ```json action block. Responding with plain text only is NOT acceptable when tool_choice is \"any\"."
	case "tool":
		name, _ := m["name"].(string)
		if name != "" {
			return fmt.Sprintf("\n\n**MANDATORY**: Your response MUST call the %q action using a ```json action block.", name)
		}
	}
	return ""
}

func cursorBuildToolInstructions(tools []map[string]any, toolChoice any) string {
	if len(tools) == 0 {
		return ""
	}
	var lines []string
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name == "" {
			continue
		}
		desc, _ := tool["description"].(string)
		if len(desc) > 50 {
			desc = desc[:50] + "…"
		}
		paramStr := ""
		if sch, ok := tool["input_schema"].(map[string]any); ok {
			cs := cursorCompactSchema(sch)
			if cs != "" && cs != "{}" {
				paramStr = "\n Params: " + cs
			}
		}
		if desc != "" {
			lines = append(lines, fmt.Sprintf("- **%s**: %s%s", name, desc, paramStr))
		} else {
			lines = append(lines, fmt.Sprintf("- **%s**%s", name, paramStr))
		}
	}
	commNames := map[string]struct{}{
		"attempt_completion": {}, "ask_followup_question": {}, "AskFollowupQuestion": {},
	}
	hasComm := false
	for _, t := range tools {
		n, _ := t["name"].(string)
		if _, ok := commNames[n]; ok {
			hasComm = true
			break
		}
	}
	behavior := `Include the structured block when performing actions. For independent actions, include multiple blocks. For dependent actions, wait for each result. Keep explanatory text brief. If you have completed the task or have nothing to execute, respond in plain text without any structured block.`
	if hasComm {
		behavior = `When performing actions, always include the structured block. For independent actions, include multiple blocks. For dependent actions (where one result feeds into the next), wait for each result. When you have nothing to execute or need to ask the user something, use the communication actions (attempt_completion, ask_followup_question). Do not run empty or meaningless commands.`
	}
	return fmt.Sprintf(`You are operating within an IDE environment with access to the following actions. To invoke an action, include it in your response using this structured format:

`+"```"+`json action
{
  "tool": "ACTION_NAME",
  "parameters": {
    "param": "value"
  }
}
`+"```"+`

Available actions:
%s

%s%s`, strings.Join(lines, "\n"), behavior, cursorToolChoiceConstraint(toolChoice))
}

func cursorFormatToolCallJSON(name string, input map[string]any) string {
	b, _ := json.MarshalIndent(map[string]any{"tool": name, "parameters": input}, "", "  ")
	return "```json action\n" + string(b) + "\n```"
}

func cursorExtractToolResultText(block map[string]any) string {
	c := block["content"]
	if c == nil {
		return ""
	}
	if s, ok := c.(string); ok {
		return s
	}
	if arr, ok := c.([]any); ok {
		var parts []string
		for _, x := range arr {
			m, ok := x.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if tx, _ := m["text"].(string); tx != "" {
					parts = append(parts, tx)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	raw, _ := json.Marshal(c)
	return string(raw)
}

func cursorHasToolResultBlock(msg map[string]any) bool {
	c := msg["content"]
	arr, ok := c.([]any)
	if !ok {
		return false
	}
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := m["type"].(string); typ == "tool_result" {
			return true
		}
	}
	return false
}

func cursorExtractToolResultNatural(msg map[string]any) string {
	c := msg["content"]
	arr, ok := c.([]any)
	if !ok {
		if s, ok := c.(string); ok {
			return s
		}
		return ""
	}
	var parts []string
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		switch typ, _ := m["type"].(string); typ {
		case "tool_result":
			txt := cursorExtractToolResultText(m)
			isErr, _ := m["is_error"].(bool)
			if isErr {
				parts = append(parts, "The action encountered an error:\n"+txt)
			} else {
				parts = append(parts, "Action output:\n"+txt)
			}
		case "text":
			if tx, _ := m["text"].(string); tx != "" {
				parts = append(parts, tx)
			}
		}
	}
	return strings.Join(parts, "\n\n") + "\n\nContinue with the next action."
}

func cursorExtractAnthropicMessageText(msg map[string]any) string {
	c := msg["content"]
	if s, ok := c.(string); ok {
		return s
	}
	arr, ok := c.([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		switch typ, _ := m["type"].(string); typ {
		case "text":
			if tx, _ := m["text"].(string); tx != "" {
				parts = append(parts, tx)
			}
		case "image":
			parts = append(parts, "[Image: not processed in Cursor web chat path]")
		case "tool_use":
			nm, _ := m["name"].(string)
			var input map[string]any
			if in, ok := m["input"].(map[string]any); ok {
				input = in
			}
			parts = append(parts, cursorFormatToolCallJSON(nm, input))
		case "tool_result":
			txt := cursorExtractToolResultText(m)
			pref := "Output"
			if b, _ := m["is_error"].(bool); b {
				pref = "Error"
			}
			parts = append(parts, pref+":\n"+txt)
		}
	}
	return strings.Join(parts, "\n\n")
}

// cursorOpenAIToolCallsToActionText turns OpenAI-style assistant tool_calls into ```json action``` text.
func cursorOpenAIToolCallsToActionText(msg map[string]any) string {
	tcs, ok := msg["tool_calls"].([]any)
	if !ok || len(tcs) == 0 {
		return ""
	}
	var parts []string
	for _, tc := range tcs {
		m, ok := tc.(map[string]any)
		if !ok {
			continue
		}
		fn, _ := m["function"].(map[string]any)
		if fn == nil {
			continue
		}
		name, _ := fn["name"].(string)
		argStr, _ := fn["arguments"].(string)
		var args map[string]any
		if argStr != "" {
			_ = json.Unmarshal([]byte(argStr), &args)
		}
		if args == nil {
			args = map[string]any{}
		}
		parts = append(parts, cursorFormatToolCallJSON(name, args))
	}
	return strings.Join(parts, "\n\n")
}

func cursorCursorMessage(role string, text string) map[string]any {
	return map[string]any{
		"id":   cursorShortID(),
		"role": role,
		"parts": []any{map[string]any{
			"type": "text",
			"text": text,
		}},
	}
}

// anthropicBodyToCursorComChat converts Anthropic Messages JSON (in map form) to Cursor /api/chat body.
func anthropicBodyToCursorComChat(anth map[string]any, cursorModel string) (map[string]any, error) {
	if anth == nil {
		return nil, fmt.Errorf("empty anthropic body")
	}
	var tools []map[string]any
	for _, t := range namedJSONArray(anth, "tools") {
		m, ok := t.(map[string]any)
		if ok {
			tools = append(tools, m)
		}
	}
	combined := cursorAnthropicCombinedSystem(anth)
	tc := anth["tool_choice"]

	var messages []map[string]any

	if len(tools) > 0 {
		ti := cursorBuildToolInstructions(tools, tc)
		full := ti
		if combined != "" {
			full = combined + "\n\n---\n\n" + ti
		}
		messages = append(messages, cursorCursorMessage("user", full))

		// Few-shot: pick read-like and write-like examples
		readName := ""
		writeName := ""
		for _, t := range tools {
			n, _ := t["name"].(string)
			ln := strings.ToLower(n)
			if readName == "" && strings.Contains(ln, "read") {
				readName = n
			}
			if writeName == "" && (strings.Contains(ln, "write") || strings.Contains(ln, "bash") || strings.Contains(ln, "exec")) {
				writeName = n
			}
		}
		if readName == "" && len(tools) > 0 {
			readName, _ = tools[0]["name"].(string)
		}
		ex1 := cursorFormatToolCallJSON(readName, map[string]any{"file_path": "src/index.ts"})
		var few strings.Builder
		few.WriteString("Understood. I'll use the structured format. First steps:\n\n")
		few.WriteString(ex1)
		if writeName != "" && writeName != readName {
			few.WriteString("\n\n")
			few.WriteString(cursorFormatToolCallJSON(writeName, map[string]any{"command": "ls -la"}))
		}
		messages = append(messages, cursorCursorMessage("assistant", few.String()))
	}

	msgs, _ := anth["messages"].([]any)
	hasTools := len(tools) > 0
	for i, item := range msgs {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		switch role {
		case "assistant":
			txt := cursorExtractAnthropicMessageText(msg)
			if txt == "" {
				txt = cursorOpenAIToolCallsToActionText(msg)
			}
			if txt == "" {
				continue
			}
			messages = append(messages, cursorCursorMessage("assistant", txt))
		case "user":
			if cursorHasToolResultBlock(msg) {
				messages = append(messages, cursorCursorMessage("user", cursorExtractToolResultNatural(msg)))
				continue
			}
			txt := cursorExtractAnthropicMessageText(msg)
			if txt == "" {
				continue
			}
			isLast := true
			for j := i + 1; j < len(msgs); j++ {
				nm, ok := msgs[j].(map[string]any)
				if !ok {
					continue
				}
				if rr, _ := nm["role"].(string); rr == "user" {
					isLast = false
					break
				}
			}
			suffix := "\n\nRespond with the appropriate action using the structured format."
			if hasTools && isLast {
				suffix = "\n\nFirst think step by step if needed, then respond with the appropriate action using the structured format."
			}
			messages = append(messages, cursorCursorMessage("user", txt+suffix))
		default:
			txt := cursorExtractAnthropicMessageText(msg)
			if txt != "" {
				messages = append(messages, cursorCursorMessage(role, txt))
			}
		}
	}

	out := map[string]any{
		"model":    cursorModel,
		"id":       cursorDeriveConversationID(anth),
		"messages": messages,
		"trigger":  "submit-message",
	}
	return out, nil
}

// openAIChatBodyToAnthropicShape builds a minimal Anthropic-shaped map from OpenAI chat body (for /v1/chat/completions + tools).
func openAIChatBodyToAnthropicShape(body map[string]any) map[string]any {
	out := map[string]any{}
	if m, ok := body["model"].(string); ok {
		out["model"] = m
	}
	// messages
	if msgs, ok := body["messages"].([]any); ok {
		out["messages"] = msgs
	}
	// tools: OpenAI tools + legacy "functions" → Anthropic tools
	var anthTools []any
	for _, t := range namedJSONArray(body, "tools") {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := m["type"].(string); typ != "function" {
			continue
		}
		fn, ok := m["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params, _ := fn["parameters"].(map[string]any)
		anthTools = append(anthTools, map[string]any{
			"name":         name,
			"description":  desc,
			"input_schema": params,
		})
	}
	if len(anthTools) == 0 {
		for _, t := range namedJSONArray(body, "functions") {
			m, ok := t.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			desc, _ := m["description"].(string)
			params, _ := m["parameters"].(map[string]any)
			anthTools = append(anthTools, map[string]any{
				"name":         name,
				"description":  desc,
				"input_schema": params,
			})
		}
	}
	if len(anthTools) > 0 {
		out["tools"] = anthTools
	}
	if tc := body["tool_choice"]; tc != nil {
		// OpenAI tool_choice can be string "auto" / object — pass through when map
		if m, ok := tc.(map[string]any); ok {
			out["tool_choice"] = m
		} else if s, ok := tc.(string); ok {
			switch s {
			case "required", "any":
				out["tool_choice"] = map[string]any{"type": "any"}
			case "auto", "none":
				// omit
			default:
				out["tool_choice"] = map[string]any{"type": s}
			}
		}
	}
	return out
}

func cursorAnthropicSourceForTools(p *relayAttemptParams) (map[string]any, error) {
	if p == nil {
		return nil, fmt.Errorf("nil params")
	}
	if p.BridgeOriginalBody != nil {
		return p.BridgeOriginalBody, nil
	}
	// req.Body may have been mutated after parse; prefer original wire JSON when it implies tools.
	if p.InboundSnapshot != nil {
		if cursorBodyImpliesClientTooling(p.InboundSnapshot) || anthropicBodyImpliesTooling(p.InboundSnapshot) {
			if p.IsAnthropicInbound {
				return p.InboundSnapshot, nil
			}
			return openAIChatBodyToAnthropicShape(p.InboundSnapshot), nil
		}
	}
	if len(p.InboundRawJSON) > 0 && relayHeuristicToolsInJSON(p.InboundRawJSON) {
		var m map[string]any
		if err := json.Unmarshal(p.InboundRawJSON, &m); err == nil && len(m) > 0 {
			if p.IsAnthropicInbound {
				return m, nil
			}
			return openAIChatBodyToAnthropicShape(m), nil
		}
	}
	if p.IsAnthropicInbound {
		// Should not happen normally (BridgeOriginalBody set in retry)
		return openAIChatBodyToAnthropicShape(p.Body), nil
	}
	return openAIChatBodyToAnthropicShape(p.Body), nil
}
