package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/runtime/sdk/api/handlers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func (h *APIHandler) Completions(c *gin.Context) {
	rawJSON, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: fmt.Sprintf("Invalid request: %v", err),
				Type:    "invalid_request_error",
			},
		})
		return
	}
	if _, ok := requireOpenAIModel(c, rawJSON); !ok {
		return
	}
	if gjson.GetBytes(rawJSON, "stream").Type == gjson.True {
		h.handleCompletionsStreamingResponse(c, rawJSON)
		return
	}

	c.Header("Content-Type", "application/json")
	chatCompletionsJSON := ConvertCompletionsRequestToChatCompletions(rawJSON)
	modelName := gjson.GetBytes(chatCompletionsJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	stopKeepAlive := h.startKeepAlive(c, cliCtx)
	resp, upstreamHeaders, errMsg := h.executeRequest(cliCtx, modelName, chatCompletionsJSON, "")
	stopKeepAlive()
	if errMsg != nil {
		writeRuntimeErrorResponse(c, errMsg)
		cliCancel(errMsg.Error)
		return
	}
	handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
	completionsResp := ConvertChatCompletionsResponseToCompletions(resp)
	_, _ = c.Writer.Write(completionsResp)
	cliCancel(nil)
}

func FormatCompletionsStreamChunk(chunk []byte) []byte {
	converted := ConvertChatCompletionsStreamChunkToCompletions(chunk)
	if converted == nil {
		return nil
	}
	return []byte(formatStreamChunk(converted))
}

func (h *APIHandler) handleCompletionsStreamingResponse(c *gin.Context, rawJSON []byte) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Streaming not supported",
				Type:    "server_error",
			},
		})
		return
	}

	chatCompletionsJSON := ConvertCompletionsRequestToChatCompletions(rawJSON)
	modelName := gjson.GetBytes(chatCompletionsJSON, "model").String()
	cliCtx, cliCancel := h.getContext(c, context.Background())
	dataChan, upstreamHeaders, errChan := h.executeStreamRequest(cliCtx, modelName, chatCompletionsJSON, "")

	setSSEHeaders := func() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
	}

	for {
		select {
		case <-c.Request.Context().Done():
			cliCancel(c.Request.Context().Err())
			return
		case errMsg, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			writeRuntimeErrorResponse(c, errMsg)
			if errMsg != nil {
				cliCancel(errMsg.Error)
			} else {
				cliCancel(nil)
			}
			return
		case chunk, ok := <-dataChan:
			if !ok {
				setSSEHeaders()
				handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
				_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				flusher.Flush()
				cliCancel(nil)
				return
			}

			setSSEHeaders()
			handlers.WriteUpstreamHeaders(c.Writer.Header(), upstreamHeaders)
			if formatted := FormatCompletionsStreamChunk(chunk); formatted != nil {
				_, _ = c.Writer.Write(formatted)
				flusher.Flush()
			}

			keepAliveInterval := time.Duration(0)
			if h != nil && h.inner != nil {
				keepAliveInterval = handlers.StreamingKeepAliveInterval(h.inner.Cfg)
			}
			var keepAlive *time.Ticker
			var keepAliveC <-chan time.Time
			if keepAliveInterval > 0 {
				keepAlive = time.NewTicker(keepAliveInterval)
				defer keepAlive.Stop()
				keepAliveC = keepAlive.C
			}

			var terminalErr any
			for {
				select {
				case <-c.Request.Context().Done():
					cliCancel(c.Request.Context().Err())
					return
				case chunk, ok := <-dataChan:
					if !ok {
						if terminalErr == nil {
							select {
							case errMsg, ok := <-errChan:
								if ok && errMsg != nil {
									terminalErr = errMsg
								}
							default:
							}
						}
						if terminalErr != nil {
							status := http.StatusInternalServerError
							errText := http.StatusText(status)
							switch msg := terminalErr.(type) {
							case interface{ GetStatusCode() int }:
								if code := msg.GetStatusCode(); code > 0 {
									status = code
									errText = http.StatusText(code)
								}
							case interface{ StatusCode() int }:
								if code := msg.StatusCode(); code > 0 {
									status = code
									errText = http.StatusText(code)
								}
							}
							if msg, ok := terminalErr.(interface{ GetError() error }); ok && msg.GetError() != nil && msg.GetError().Error() != "" {
								errText = msg.GetError().Error()
							}
							body := handlers.BuildErrorResponseBody(status, errText)
							_, _ = c.Writer.Write([]byte(formatStreamChunk(body)))
							flusher.Flush()
							if msg, ok := terminalErr.(interface{ GetError() error }); ok {
								cliCancel(msg.GetError())
							} else {
								cliCancel(nil)
							}
							return
						}
						_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
						flusher.Flush()
						cliCancel(nil)
						return
					}
					if formatted := FormatCompletionsStreamChunk(chunk); formatted != nil {
						_, _ = c.Writer.Write(formatted)
						flusher.Flush()
					}
				case errMsg, ok := <-errChan:
					if !ok {
						continue
					}
					if errMsg != nil {
						status := http.StatusInternalServerError
						if errMsg.StatusCode > 0 {
							status = errMsg.StatusCode
						}
						errText := http.StatusText(status)
						if errMsg.Error != nil && errMsg.Error.Error() != "" {
							errText = errMsg.Error.Error()
						}
						body := handlers.BuildErrorResponseBody(status, errText)
						_, _ = c.Writer.Write([]byte(formatStreamChunk(body)))
						flusher.Flush()
						cliCancel(errMsg.Error)
						return
					}
					cliCancel(nil)
					return
				case <-keepAliveC:
					_, _ = c.Writer.Write([]byte(": keep-alive\n\n"))
					flusher.Flush()
				}
			}
		}
	}
}

func ConvertCompletionsRequestToChatCompletions(rawJSON []byte) []byte {
	root := gjson.ParseBytes(rawJSON)
	prompt := root.Get("prompt").String()
	if prompt == "" {
		prompt = "Complete this:"
	}
	out := `{"model":"","messages":[{"role":"user","content":""}]}`
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}
	out, _ = sjson.Set(out, "messages.0.content", prompt)
	if maxTokens := root.Get("max_tokens"); maxTokens.Exists() {
		out, _ = sjson.Set(out, "max_tokens", maxTokens.Int())
	}
	if temperature := root.Get("temperature"); temperature.Exists() {
		out, _ = sjson.Set(out, "temperature", temperature.Float())
	}
	if topP := root.Get("top_p"); topP.Exists() {
		out, _ = sjson.Set(out, "top_p", topP.Float())
	}
	if frequencyPenalty := root.Get("frequency_penalty"); frequencyPenalty.Exists() {
		out, _ = sjson.Set(out, "frequency_penalty", frequencyPenalty.Float())
	}
	if presencePenalty := root.Get("presence_penalty"); presencePenalty.Exists() {
		out, _ = sjson.Set(out, "presence_penalty", presencePenalty.Float())
	}
	if stop := root.Get("stop"); stop.Exists() {
		out, _ = sjson.SetRaw(out, "stop", stop.Raw)
	}
	if stream := root.Get("stream"); stream.Exists() {
		out, _ = sjson.Set(out, "stream", stream.Bool())
	}
	if logprobs := root.Get("logprobs"); logprobs.Exists() {
		out, _ = sjson.Set(out, "logprobs", logprobs.Bool())
	}
	if topLogprobs := root.Get("top_logprobs"); topLogprobs.Exists() {
		out, _ = sjson.Set(out, "top_logprobs", topLogprobs.Int())
	}
	if echo := root.Get("echo"); echo.Exists() {
		out, _ = sjson.Set(out, "echo", echo.Bool())
	}
	return []byte(out)
}

func ConvertChatCompletionsResponseToCompletions(rawJSON []byte) []byte {
	root := gjson.ParseBytes(rawJSON)
	out := `{"id":"","object":"text_completion","created":0,"model":"","choices":[]}`
	if id := root.Get("id"); id.Exists() {
		out, _ = sjson.Set(out, "id", id.String())
	}
	if created := root.Get("created"); created.Exists() {
		out, _ = sjson.Set(out, "created", created.Int())
	}
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}
	if usage := root.Get("usage"); usage.Exists() {
		out, _ = sjson.SetRaw(out, "usage", usage.Raw)
	}
	var choices []interface{}
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			completionsChoice := map[string]interface{}{
				"index": choice.Get("index").Int(),
			}
			if message := choice.Get("message"); message.Exists() {
				if content := message.Get("content"); content.Exists() {
					completionsChoice["text"] = content.String()
				}
			} else if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() {
					completionsChoice["text"] = content.String()
				}
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() {
				completionsChoice["finish_reason"] = finishReason.String()
			}
			if logprobs := choice.Get("logprobs"); logprobs.Exists() {
				completionsChoice["logprobs"] = logprobs.Value()
			}
			choices = append(choices, completionsChoice)
			return true
		})
	}
	if len(choices) > 0 {
		choicesJSON, _ := json.Marshal(choices)
		out, _ = sjson.SetRaw(out, "choices", string(choicesJSON))
	}
	return []byte(out)
}

func ConvertChatCompletionsStreamChunkToCompletions(chunkData []byte) []byte {
	if bytes.HasPrefix(chunkData, []byte("data:")) {
		chunkData = bytes.TrimSpace(chunkData[5:])
	}
	if bytes.Equal(chunkData, []byte("[DONE]")) {
		return nil
	}

	root := gjson.ParseBytes(chunkData)
	hasContent := false
	hasUsage := root.Get("usage").Exists()
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() && content.String() != "" {
					hasContent = true
					return false
				}
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() && finishReason.String() != "" && finishReason.String() != "null" {
				hasContent = true
				return false
			}
			return true
		})
	}
	if !hasContent && !hasUsage {
		return nil
	}

	out := `{"id":"","object":"text_completion","created":0,"model":"","choices":[]}`
	if id := root.Get("id"); id.Exists() {
		out, _ = sjson.Set(out, "id", id.String())
	}
	if created := root.Get("created"); created.Exists() {
		out, _ = sjson.Set(out, "created", created.Int())
	}
	if model := root.Get("model"); model.Exists() {
		out, _ = sjson.Set(out, "model", model.String())
	}

	var choices []interface{}
	if chatChoices := root.Get("choices"); chatChoices.Exists() && chatChoices.IsArray() {
		chatChoices.ForEach(func(_, choice gjson.Result) bool {
			completionsChoice := map[string]interface{}{
				"index": choice.Get("index").Int(),
			}
			if delta := choice.Get("delta"); delta.Exists() {
				if content := delta.Get("content"); content.Exists() && content.String() != "" {
					completionsChoice["text"] = content.String()
				} else {
					completionsChoice["text"] = ""
				}
			} else {
				completionsChoice["text"] = ""
			}
			if finishReason := choice.Get("finish_reason"); finishReason.Exists() && finishReason.String() != "null" {
				completionsChoice["finish_reason"] = finishReason.String()
			}
			if logprobs := choice.Get("logprobs"); logprobs.Exists() {
				completionsChoice["logprobs"] = logprobs.Value()
			}
			choices = append(choices, completionsChoice)
			return true
		})
	}
	if len(choices) > 0 {
		choicesJSON, _ := json.Marshal(choices)
		out, _ = sjson.SetRaw(out, "choices", string(choicesJSON))
	}
	if usage := root.Get("usage"); usage.Exists() {
		out, _ = sjson.SetRaw(out, "usage", usage.Raw)
	}
	return []byte(out)
}
