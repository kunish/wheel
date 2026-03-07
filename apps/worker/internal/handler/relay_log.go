package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ── Async Logging ───────────────────────────────────────────────

func (h *RelayHandler) asyncRecordLog(
	model, targetModel string,
	channel *types.Channel,
	key *types.ChannelKey,
	apiKeyId int,
	body map[string]any,
	requestHeaders map[string]string,
	responseHeaders http.Header,
	upstreamBodyForLog *string,
	result *relayResult,
	attempts []attemptRecord,
	startTime time.Time,
) {
	if h.LogWriter == nil {
		return
	}

	cost := relay.CalculateCost(targetModel, result.InputTokens, result.OutputTokens, context.Background(), h.DB,
		&relay.CacheTokens{
			CacheReadTokens:     result.CacheReadTokens,
			CacheCreationTokens: result.CacheCreationTokens,
		})

	logBody := serializeForLog(body)
	requestHeadersJSON := serializeRequestHeadersForLog(requestHeaders)
	responseHeadersJSON := serializeResponseHeadersForLog(responseHeaders)

	respContent := result.ResponseContent
	if result.ThinkingContent != "" {
		respContent = "<|thinking|>" + result.ThinkingContent + "<|/thinking|>" + respContent
	}
	if respContent == "" && result.StreamID != "" {
		respContent = "[streaming]"
	}

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	_ = json.Unmarshal(attemptsJSON, &attemptsVal)

	var costInfo *db.CostInfo
	if cost > 0 {
		costInfo = &db.CostInfo{
			ApiKeyID:     apiKeyId,
			ChannelKeyID: key.ID,
			Cost:         cost,
		}
	}

	h.LogWriter.Submit(types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channel.ID,
		ChannelName:      channel.Name,
		ActualModelName:  targetModel,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		FTUT:             result.FirstTokenTime,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		Cost:             cost,
		RequestContent:   logBody,
		RequestHeaders:   requestHeadersJSON,
		UpstreamContent:  upstreamBodyForLog,
		ResponseContent:  respContent,
		ResponseHeaders:  responseHeadersJSON,
		Error:            "",
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	}, costInfo, result.StreamID)

	// Record observability metrics
	ctx := context.Background()
	h.Observer.RecordRequest(ctx, channel.Name, targetModel, "", 200)
	h.Observer.RecordDuration(ctx, channel.Name, targetModel, time.Since(startTime))
	h.Observer.RecordTokens(ctx, channel.Name, targetModel, "input", result.InputTokens)
	h.Observer.RecordTokens(ctx, channel.Name, targetModel, "output", result.OutputTokens)
	if cost > 0 {
		h.Observer.RecordCost(ctx, channel.Name, targetModel, cost)
	}
	if result.FirstTokenTime > 0 {
		h.Observer.RecordTTFB(ctx, channel.Name, targetModel, result.FirstTokenTime)
	}
}

func (h *RelayHandler) asyncErrorLog(
	model string,
	channelID int,
	channelName string,
	body map[string]any,
	lastError string,
	attempts []attemptRecord,
	startTime time.Time,
	streamID string,
) {
	if h.LogWriter == nil {
		return
	}

	logBody := serializeForLog(body)

	attemptsJSON, _ := json.Marshal(attempts)
	var attemptsVal types.AttemptList
	_ = json.Unmarshal(attemptsJSON, &attemptsVal)

	h.LogWriter.Submit(types.RelayLog{
		Time:             time.Now().Unix(),
		RequestModelName: model,
		ChannelID:        channelID,
		ChannelName:      channelName,
		ActualModelName:  model,
		UseTime:          int(time.Since(startTime).Milliseconds()),
		RequestContent:   logBody,
		RequestHeaders:   "",
		ResponseContent:  lastError,
		ResponseHeaders:  "",
		Error:            lastError,
		Attempts:         attemptsVal,
		TotalAttempts:    len(attempts),
	}, nil, streamID)
}

// ── Log Serialization ──────────────────────────────────────────────

func serializeForLog(body map[string]any) string {
	result, _ := json.Marshal(body)
	return string(result)
}

func serializeRequestHeadersForLog(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	cloned := make(map[string]string, len(headers))
	for k, v := range headers {
		if shouldRedactHeader(k) {
			cloned[k] = "[REDACTED]"
			continue
		}
		cloned[k] = v
	}
	b, _ := json.Marshal(cloned)
	return string(b)
}

func serializeResponseHeadersForLog(headers http.Header) string {
	if len(headers) == 0 {
		return ""
	}
	cloned := make(map[string][]string, len(headers))
	for k, values := range headers {
		if shouldRedactHeader(k) {
			cloned[k] = []string{"[REDACTED]"}
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		cloned[k] = copied
	}
	b, _ := json.Marshal(cloned)
	return string(b)
}

func shouldRedactHeader(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "authorization", "api-key", "x-api-key", "cookie", "set-cookie", "proxy-authorization":
		return true
	default:
		return false
	}
}

// apiError returns an error response in the correct format based on the request type.
// Anthropic format: {"type":"error","error":{"type":"...","message":"..."}}
// OpenAI format:    {"error":{"message":"...","type":"...","param":null,"code":null}}
func apiError(c *gin.Context, status int, errType, message string, isAnthropicInbound bool) {
	if isAnthropicInbound {
		c.JSON(status, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": message,
			},
		})
	} else {
		c.JSON(status, relay.OpenAIErrorBody(errType, message))
	}
}
