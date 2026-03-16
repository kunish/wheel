// Package claude provides response translation functionality for OpenAI to Anthropic API.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/runtime/corelib/util"
)

var (
	dataTag = []byte("data:")
)

// ConvertOpenAIResponseToClaude converts OpenAI streaming response format to Anthropic API format.
func ConvertOpenAIResponseToClaude(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	if *param == nil {
		accum := protocol.NewOpenAIToAnthropicAccum()
		*param = accum
	}

	if !bytes.HasPrefix(rawJSON, dataTag) {
		return []string{}
	}
	rawJSON = bytes.TrimSpace(rawJSON[5:])

	accum := (*param).(*protocol.OpenAIToAnthropicAccum)
	if accum.ToolNameMap == nil {
		accum.ToolNameMap = util.ToolNameMapFromClaudeRequest(originalRequestRawJSON)
	}

	rawStr := strings.TrimSpace(string(rawJSON))
	if rawStr == "[DONE]" {
		return protocol.ConvertOpenAIDoneToAnthropic(accum)
	}

	var chunk protocol.OpenAIChatResponse
	if err := json.Unmarshal(rawJSON, &chunk); err != nil {
		return []string{}
	}

	return protocol.ConvertOpenAIChunkToAnthropic(&chunk, accum, accum.ToolNameMap)
}

// ConvertOpenAIResponseToClaudeNonStream converts a non-streaming OpenAI response to a non-streaming Anthropic response.
func ConvertOpenAIResponseToClaudeNonStream(_ context.Context, _ string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, _ *any) string {
	var resp protocol.OpenAIChatResponse
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		return string(rawJSON)
	}

	toolNameMap := util.ToolNameMapFromClaudeRequest(originalRequestRawJSON)
	anthropicResp := protocol.OpenAIResponseToAnthropic(&resp)

	// Apply tool name mapping
	if toolNameMap != nil {
		for i, block := range anthropicResp.Content {
			if block.Type == "tool_use" {
				anthropicResp.Content[i].Name = util.MapToolName(toolNameMap, block.Name)
			}
		}
	}

	result, err := json.Marshal(anthropicResp)
	if err != nil {
		return string(rawJSON)
	}
	return string(result)
}

func ClaudeTokenCount(ctx context.Context, count int64) string {
	return fmt.Sprintf(`{"input_tokens":%d}`, count)
}
