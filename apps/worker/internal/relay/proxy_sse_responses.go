package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// responsesWriterWrapper wraps an http.ResponseWriter and intercepts SSE writes,
// converting Chat Completions SSE events to Responses API SSE events on the fly.
// This sits on top of ALL upstream format converters (Anthropic→OpenAI, Gemini→OpenAI, etc.)
// so the conversion chain is: Provider SSE → Chat Completions SSE → Responses API SSE.
type responsesWriterWrapper struct {
	underlying http.ResponseWriter
	flusher    http.Flusher
	convert    func(string) []string
	buf        bytes.Buffer
}

func (w *responsesWriterWrapper) Header() http.Header {
	return w.underlying.Header()
}

func (w *responsesWriterWrapper) WriteHeader(statusCode int) {
	w.underlying.WriteHeader(statusCode)
}

func (w *responsesWriterWrapper) Write(p []byte) (int, error) {
	n := len(p)
	w.buf.Write(p)

	// Process complete lines from buffer
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Incomplete line — put it back
			w.buf.WriteString(line)
			break
		}
		line = strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			converted := w.convert(data)
			if converted != nil {
				for _, l := range converted {
					fmt.Fprintf(w.underlying, "%s\n", l)
				}
			}
		}
		// Skip non-data lines (empty lines, event: lines from passthrough)
	}

	return n, nil
}

func (w *responsesWriterWrapper) Flush() {
	w.flusher.Flush()
}

// createOpenAIToResponsesSSEConverter creates a stateful converter from
// OpenAI Chat Completions SSE chunks to OpenAI Responses API SSE events.
func createOpenAIToResponsesSSEConverter() func(string) []string {
	started := false
	respID := "resp_" + uuid.New().String()[:24]
	itemID := "msg_" + uuid.New().String()[:24]
	msgModel := ""
	var fullText strings.Builder

	// Tool call state
	type toolCallState struct {
		itemID    string
		name      string
		args      strings.Builder
		itemIndex int
	}
	nextOutputIndex := 1 // 0 is reserved for the text message item
	toolCalls := map[int]*toolCallState{}

	return func(jsonStr string) []string {
		if jsonStr == "[DONE]" {
			// Emit content_part.done, output_item.done, response.completed
			var lines []string

			// Close text content part
			lines = append(lines, responsesEvent("response.content_part.done", map[string]any{
				"type":          "response.content_part.done",
				"item_id":       itemID,
				"output_index":  0,
				"content_index": 0,
				"part": map[string]any{
					"type":        "output_text",
					"text":        fullText.String(),
					"annotations": []any{},
				},
			})...)

			// Close text message item
			lines = append(lines, responsesEvent("response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": 0,
				"item": map[string]any{
					"id":     itemID,
					"type":   "message",
					"role":   "assistant",
					"status": "completed",
					"content": []any{
						map[string]any{
							"type":        "output_text",
							"text":        fullText.String(),
							"annotations": []any{},
						},
					},
				},
			})...)

			// Close tool call items
			for _, tc := range toolCalls {
				lines = append(lines, responsesEvent("response.output_item.done", map[string]any{
					"type":         "response.output_item.done",
					"output_index": tc.itemIndex,
					"item": map[string]any{
						"id":        tc.itemID,
						"type":      "function_call",
						"status":    "completed",
						"name":      tc.name,
						"call_id":   tc.itemID,
						"arguments": tc.args.String(),
					},
				})...)
			}

			// response.completed
			output := []any{
				map[string]any{
					"id":     itemID,
					"type":   "message",
					"role":   "assistant",
					"status": "completed",
					"content": []any{
						map[string]any{
							"type":        "output_text",
							"text":        fullText.String(),
							"annotations": []any{},
						},
					},
				},
			}
			for _, tc := range toolCalls {
				output = append(output, map[string]any{
					"id":        tc.itemID,
					"type":      "function_call",
					"status":    "completed",
					"name":      tc.name,
					"call_id":   tc.itemID,
					"arguments": tc.args.String(),
				})
			}

			lines = append(lines, responsesEvent("response.completed", map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":     respID,
					"object": "response",
					"status": "completed",
					"model":  msgModel,
					"output": output,
				},
			})...)

			return lines
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
			return nil
		}

		if model, ok := obj["model"].(string); ok {
			msgModel = model
		}
		if id, ok := obj["id"].(string); ok && strings.HasPrefix(id, "msg_") {
			itemID = id
		}

		var lines []string

		if !started {
			started = true
			// response.created
			lines = append(lines, responsesEvent("response.created", map[string]any{
				"type": "response.created",
				"response": map[string]any{
					"id":     respID,
					"object": "response",
					"status": "in_progress",
					"model":  msgModel,
					"output": []any{},
				},
			})...)

			// response.in_progress
			lines = append(lines, responsesEvent("response.in_progress", map[string]any{
				"type": "response.in_progress",
				"response": map[string]any{
					"id":     respID,
					"object": "response",
					"status": "in_progress",
					"model":  msgModel,
					"output": []any{},
				},
			})...)

			// response.output_item.added (message item)
			lines = append(lines, responsesEvent("response.output_item.added", map[string]any{
				"type":         "response.output_item.added",
				"output_index": 0,
				"item": map[string]any{
					"id":      itemID,
					"type":    "message",
					"role":    "assistant",
					"status":  "in_progress",
					"content": []any{},
				},
			})...)

			// response.content_part.added
			lines = append(lines, responsesEvent("response.content_part.added", map[string]any{
				"type":          "response.content_part.added",
				"item_id":       itemID,
				"output_index":  0,
				"content_index": 0,
				"part": map[string]any{
					"type":        "output_text",
					"text":        "",
					"annotations": []any{},
				},
			})...)
		}

		choices, _ := obj["choices"].([]any)
		if len(choices) == 0 {
			if len(lines) > 0 {
				return lines
			}
			return nil
		}

		choice, _ := choices[0].(map[string]any)
		if choice == nil {
			return lines
		}

		delta, _ := choice["delta"].(map[string]any)

		if delta != nil {
			// Text content delta
			if content, ok := delta["content"].(string); ok && content != "" {
				fullText.WriteString(content)
				lines = append(lines, responsesEvent("response.output_text.delta", map[string]any{
					"type":          "response.output_text.delta",
					"item_id":       itemID,
					"output_index":  0,
					"content_index": 0,
					"delta":         content,
				})...)
			}

			// Tool calls
			if tcs, ok := delta["tool_calls"].([]any); ok {
				for _, tc := range tcs {
					tcMap, ok := tc.(map[string]any)
					if !ok {
						continue
					}
					tcIdx := toInt(tcMap["index"])

					if _, seen := toolCalls[tcIdx]; !seen {
						tcID, _ := tcMap["id"].(string)
						if tcID == "" {
							tcID = "call_" + uuid.New().String()[:24]
						}
						fn, _ := tcMap["function"].(map[string]any)
						fnName, _ := fn["name"].(string)

						outputIdx := nextOutputIndex
						nextOutputIndex++

						toolCalls[tcIdx] = &toolCallState{
							itemID:    tcID,
							name:      fnName,
							itemIndex: outputIdx,
						}

						// Emit output_item.added for function_call
						lines = append(lines, responsesEvent("response.output_item.added", map[string]any{
							"type":         "response.output_item.added",
							"output_index": outputIdx,
							"item": map[string]any{
								"id":        tcID,
								"type":      "function_call",
								"status":    "in_progress",
								"name":      fnName,
								"call_id":   tcID,
								"arguments": "",
							},
						})...)
					}

					if fn, ok := tcMap["function"].(map[string]any); ok {
						if args, ok := fn["arguments"].(string); ok && args != "" {
							tc := toolCalls[tcIdx]
							tc.args.WriteString(args)
							lines = append(lines, responsesEvent("response.function_call_arguments.delta", map[string]any{
								"type":         "response.function_call_arguments.delta",
								"item_id":      tc.itemID,
								"output_index": tc.itemIndex,
								"delta":        args,
							})...)
						}
					}
				}
			}
		}

		if len(lines) > 0 {
			return lines
		}
		return nil
	}
}

// responsesEvent builds an SSE event with the given event name and data.
func responsesEvent(eventName string, data map[string]any) []string {
	b, _ := json.Marshal(data)
	return []string{
		"event: " + eventName,
		"data: " + string(b),
		"",
	}
}
