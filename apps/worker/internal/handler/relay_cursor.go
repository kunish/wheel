package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kunish/wheel/apps/worker/internal/protocol"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"golang.org/x/net/http2"
)

// Cursor API — OpenAI-compatible bridge to Cursor IDE AgentService (ConnectRPC over HTTP/2).
// Protocol reference: https://github.com/kunish/cursoride2api

const (
	cursorDefaultBaseURL      = "https://api2.cursor.sh"
	cursorDefaultClientVer    = "2.6.20"
	cursorDefaultDisplayModel = "claude-4.5-sonnet"
	cursorAgentRunPath        = "/agent.v1.AgentService/Run"
	cursorAgentModelsPath     = "/agent.v1.AgentService/GetUsableModels"
	cursorHeartbeatInterval   = 5 * time.Second
)

var (
	cursorHTTP2Once   sync.Once
	cursorHTTP2Client *http.Client
)

func cursorSharedH2Client() *http.Client {
	cursorHTTP2Once.Do(func() {
		t := &http.Transport{}
		_ = http2.ConfigureTransport(t)
		cursorHTTP2Client = &http.Client{Transport: t, Timeout: 0}
	})
	return cursorHTTP2Client
}

// cursorAgentRunTimeout bounds how long a single AgentService.Run round-trip may block on reading
// the streaming body (matches cursoride2api REQUEST_TIMEOUT default ~120s).
func cursorAgentRunTimeout() time.Duration {
	s := strings.TrimSpace(os.Getenv("CURSOR_AGENT_RUN_TIMEOUT"))
	if s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return 120 * time.Second
}

// cursorAgentRunHTTPClient is an HTTP/2 client with a finite timeout for Agent Run only
// (shared transport, separate from the unbounded client used elsewhere).
func cursorAgentRunHTTPClient() *http.Client {
	return &http.Client{
		Transport: cursorSharedH2Client().Transport,
		Timeout:   cursorAgentRunTimeout(),
	}
}

// CursorRelay implements Cursor Agent API requests (non-stream and SSE).
type CursorRelay struct{}

// NewCursorRelay returns a stateless Cursor relay.
func NewCursorRelay() *CursorRelay { return &CursorRelay{} }

type cursorCredentials struct {
	AccessToken   string `json:"accessToken"`
	MachineID     string `json:"machineId"`
	MacMachineID  string `json:"macMachineId"`
	ClientVersion string `json:"clientVersion"`
}

func parseCursorCredentials(raw string) (cursorCredentials, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cursorCredentials{}, fmt.Errorf("empty channel key")
	}
	if raw[0] == '{' {
		var c cursorCredentials
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return cursorCredentials{}, fmt.Errorf("cursor key JSON: %w", err)
		}
		c.AccessToken = strings.TrimSpace(c.AccessToken)
		// cursoride2api token.json: { "tokens": [ { "accessToken", ... }, ... ] }
		if c.AccessToken == "" {
			var wrap struct {
				Tokens []cursorCredentials `json:"tokens"`
			}
			if err := json.Unmarshal([]byte(raw), &wrap); err == nil && len(wrap.Tokens) > 0 {
				c = wrap.Tokens[0]
				c.AccessToken = strings.TrimSpace(c.AccessToken)
				c.MachineID = strings.TrimSpace(c.MachineID)
				c.MacMachineID = strings.TrimSpace(c.MacMachineID)
				c.ClientVersion = strings.TrimSpace(c.ClientVersion)
			}
		}
		if c.AccessToken == "" {
			return cursorCredentials{}, fmt.Errorf("cursor key: accessToken is required")
		}
		if strings.TrimSpace(c.ClientVersion) == "" {
			c.ClientVersion = cursorDefaultClientVer
		}
		return c, nil
	}
	return cursorCredentials{
		AccessToken:   raw,
		ClientVersion: cursorDefaultClientVer,
	}, nil
}

// cursorModelIDsFromUsableModelsJSON extracts model ids from GetUsableModels JSON.
// The API usually returns { "models": [ { "modelId": "..." } ] } but some responses
// use string ids, snake_case fields, or a Connect "result" wrapper.
func cursorModelIDsFromUsableModelsJSON(body []byte) ([]string, error) {
	var top map[string]any
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, err
	}
	if inner, ok := top["result"].(map[string]any); ok {
		top = inner
	}
	rawModels, _ := top["models"].([]any)
	if rawModels == nil {
		rawModels, _ = top["usableModels"].([]any)
	}
	out := make([]string, 0, len(rawModels))
	seen := map[string]struct{}{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, rm := range rawModels {
		switch v := rm.(type) {
		case string:
			add(v)
		case map[string]any:
			var id string
			for _, key := range []string{"modelId", "model_id", "id", "name", "modelName", "model"} {
				if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
					id = strings.TrimSpace(s)
					break
				}
				if n, ok := v[key].(float64); ok {
					id = fmt.Sprintf("%.0f", n)
					break
				}
			}
			add(id)
		}
	}
	return out, nil
}

// cursorFallbackUsableModelIDs returns known Cursor upstream model ids from the local
// mapping table when GetUsableModels returns an empty list (still HTTP 200).
func cursorFallbackUsableModelIDs() []string {
	m := cursorModelMapping()
	seen := make(map[string]struct{}, len(m))
	out := make([]string, 0, len(m))
	for _, v := range m {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func cursorModelMapping() map[string]string {
	return map[string]string{
		"gpt-4":             "composer-2",
		"gpt-4o":            "composer-2",
		"gpt-4o-mini":       "composer-2-fast",
		"gpt-4-turbo":       "composer-2",
		"gpt-3.5-turbo":     "composer-1.5",
		"claude-3-opus":     "claude-4.6-opus-high",
		"claude-3-sonnet":   "claude-4.6-sonnet-medium",
		"claude-3.5-sonnet": "claude-4.5-sonnet",
		// Group-style aliases (hyphen minor / name-major ordering).
		"claude-opus-4-6":          "claude-4.6-opus-high",
		"claude-opus-4.6":          "claude-4.6-opus-high",
		"claude-sonnet-4-6":        "claude-4.6-sonnet-medium",
		"claude-sonnet-4.6":        "claude-4.6-sonnet-medium",
		"claude-4-6-opus-high":     "claude-4.6-opus-high",
		"claude-4.6-opus-high":     "claude-4.6-opus-high",
		"claude-4-6-sonnet-medium": "claude-4.6-sonnet-medium",
		"claude-4.6-sonnet-medium": "claude-4.6-sonnet-medium",
		"claude-sonnet-4-5":        "claude-4.5-sonnet",
		"claude-sonnet-4.5":        "claude-4.5-sonnet",
		"gemini-pro":               "gemini-3.1-pro",
		"composer-2":               "composer-2",
		"composer-2-fast":          "composer-2-fast",
		"composer-1.5":             "composer-1.5",
		"default":                  "default",
	}
}

func mapCursorUpstreamModel(requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return cursorDefaultDisplayModel
	}
	mapping := cursorModelMapping()
	if m, ok := mapping[requested]; ok {
		return m
	}
	norm := normalizeClaudeHyphenVersions(requested)
	if norm != requested {
		if m, ok := mapping[norm]; ok {
			return m
		}
	}
	return requested
}

// cursorChecksum matches cursoride2api generateChecksum (Date.now()/1e6 based).
func cursorChecksum(machineID, macMachineID string) string {
	k := 165
	t := int(time.Now().UnixMilli() / 1_000_000)
	b := []byte{
		byte(t >> 40), byte(t >> 32), byte(t >> 24),
		byte(t >> 16), byte(t >> 8), byte(t),
	}
	for i := 0; i < len(b); i++ {
		b[i] = byte((int(b[i]^byte(k)) + (i % 256)) & 0xff)
		k = int(b[i])
	}
	prefix := base64.StdEncoding.EncodeToString(b)
	if strings.TrimSpace(macMachineID) != "" {
		return prefix + machineID + "/" + macMachineID
	}
	return prefix + machineID
}

func encodeCursorFrame(obj map[string]any) ([]byte, error) {
	jsonBuf, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	frame := make([]byte, 5+len(jsonBuf))
	frame[0] = 0
	frame[1] = byte(len(jsonBuf) >> 24)
	frame[2] = byte(len(jsonBuf) >> 16)
	frame[3] = byte(len(jsonBuf) >> 8)
	frame[4] = byte(len(jsonBuf))
	copy(frame[5:], jsonBuf)
	return frame, nil
}

func cursorShellAndOS() (osName, shell string) {
	switch runtime.GOOS {
	case "windows":
		return "windows", "powershell"
	default:
		return runtime.GOOS, "bash"
	}
}

func cursorExecIDs(exec map[string]any) (idInt int, execID string) {
	execID, _ = exec["execId"].(string)
	switch v := exec["id"].(type) {
	case float64:
		idInt = int(v)
	case int:
		idInt = v
	case int64:
		idInt = int(v)
	}
	return idInt, execID
}

func cursorExecReply(exec map[string]any, write func(map[string]any)) {
	idInt, execID := cursorExecIDs(exec)

	if _, ok := exec["requestContextArgs"]; ok {
		o, sh := cursorShellAndOS()
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"requestContextResult": map[string]any{
					"success": map[string]any{
						"requestContext": map[string]any{
							"env": map[string]any{
								"operatingSystem": o,
								"defaultShell":    sh,
							},
						},
					},
				},
			},
		})
		return
	}
	if _, ok := exec["readArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"readResult": map[string]any{"fileNotFound": map[string]any{}},
			},
		})
		return
	}
	if _, ok := exec["lsArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"lsResult": map[string]any{"error": map[string]any{"path": "", "error": "Headless mode"}},
			},
		})
		return
	}
	if _, ok := exec["shellArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"shellResult": map[string]any{"rejected": map[string]any{"reason": "Headless mode"}},
			},
		})
		return
	}
	if _, ok := exec["grepArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"grepResult": map[string]any{"error": map[string]any{"error": "Headless mode"}},
			},
		})
		return
	}
	if _, ok := exec["writeArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID, "writeResult": map[string]any{},
			},
		})
		return
	}
	if _, ok := exec["deleteArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"deleteResult": map[string]any{"error": map[string]any{"path": "", "error": "Headless mode"}},
			},
		})
		return
	}
	if _, ok := exec["diagnosticsArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id": idInt, "execId": execID,
				"diagnosticsResult": map[string]any{"diagnostics": []any{}},
			},
		})
		return
	}
	// Newer Cursor builds may request screen capture during agent runs.
	// Use an empty result; a wrongly-shaped "rejected" branch has been observed to stall the agent.
	if _, ok := exec["recordScreenArgs"]; ok {
		write(map[string]any{
			"execClientMessage": map[string]any{
				"id":                 idInt,
				"execId":             execID,
				"recordScreenResult": map[string]any{},
			},
		})
		return
	}
	write(map[string]any{
		"execClientMessage": map[string]any{
			"id": idInt, "execId": execID,
			"requestContextResult": map[string]any{
				"error": map[string]any{"error": "Unknown exec type"},
			},
		},
	})
}

func cursorMessagesToPrompt(body map[string]any) (string, error) {
	raw, ok := body["messages"]
	if !ok {
		return "", fmt.Errorf("messages required")
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return "", fmt.Errorf("messages required")
	}
	var b strings.Builder
	for _, item := range arr {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "" {
			role = "user"
		}
		var content string
		switch c := msg["content"].(type) {
		case string:
			content = c
		case []any:
			for _, p := range c {
				part, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if typ, _ := part["type"].(string); typ == "text" {
					if t, _ := part["text"].(string); t != "" {
						content += t + "\n"
					}
				}
			}
			content = strings.TrimSpace(content)
		default:
		}
		if content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		switch role {
		case "system":
			fmt.Fprintf(&b, "<system>\n%s\n</system>", content)
		case "assistant":
			fmt.Fprintf(&b, "<assistant>\n%s\n</assistant>", content)
		default:
			b.WriteString(content)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("empty prompt from messages")
	}
	return out, nil
}

func cursorExtractTextDelta(iu map[string]any) string {
	if iu == nil {
		return ""
	}
	for _, key := range []string{"textDelta", "text_delta"} {
		td, ok := iu[key]
		if !ok {
			continue
		}
		switch v := td.(type) {
		case string:
			return v
		case map[string]any:
			if s, _ := v["text"].(string); s != "" {
				return s
			}
			if s, _ := v["delta"].(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func cursorParseTokenCount(v any) int {
	switch t := v.(type) {
	case string:
		var n int
		_, _ = fmt.Sscanf(t, "%d", &n)
		return n
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	default:
		return 0
	}
}

func cursorParseTurnEndedMap(te map[string]any) (inT, outT int) {
	if te == nil {
		return 0, 0
	}
	inT = cursorParseTokenCount(te["inputTokens"])
	if inT == 0 {
		inT = cursorParseTokenCount(te["input_tokens"])
	}
	outT = cursorParseTokenCount(te["outputTokens"])
	if outT == 0 {
		outT = cursorParseTokenCount(te["output_tokens"])
	}
	return inT, outT
}

func cursorExtractTurnEnded(iu map[string]any) (ended bool, inputTok, outputTok int) {
	if iu == nil {
		return false, 0, 0
	}
	for _, key := range []string{"turnEnded", "turn_ended"} {
		te, ok := iu[key].(map[string]any)
		if !ok {
			continue
		}
		inT, outT := cursorParseTurnEndedMap(te)
		return true, inT, outT
	}
	return false, 0, 0
}

func cursorProcessInteractionUpdate(
	iu map[string]any,
	onDelta func(string),
) (ended bool, inputTok, outputTok int) {
	if iu == nil {
		return false, 0, 0
	}
	if _, ok := iu["heartbeat"]; ok {
		return false, 0, 0
	}
	if ok, inT, outT := cursorExtractTurnEnded(iu); ok {
		return true, inT, outT
	}
	if d := cursorExtractTextDelta(iu); d != "" && onDelta != nil {
		onDelta(d)
	}
	// Matches cursoride2api: these frames are expected until turnEnded arrives.
	if _, ok := iu["thinkingDelta"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["thinking_delta"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["thinkingCompleted"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["thinking_completed"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["tokenDelta"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["token_delta"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["stepCompleted"]; ok {
		return false, 0, 0
	}
	if _, ok := iu["step_completed"]; ok {
		return false, 0, 0
	}

	if msg, ok := iu["message"].(map[string]any); ok {
		if ok, inT, outT := cursorExtractTurnEnded(msg); ok {
			return true, inT, outT
		}
		if d := cursorExtractTextDelta(msg); d != "" && onDelta != nil {
			onDelta(d)
		}
	}
	return false, 0, 0
}

func cursorParseErrorPayload(msg map[string]any) string {
	errObj, ok := msg["error"].(map[string]any)
	if !ok {
		return ""
	}
	if details, ok := errObj["details"].([]any); ok && len(details) > 0 {
		if d0, ok := details[0].(map[string]any); ok {
			if v, _ := d0["value"].(string); v != "" {
				if dec, err := base64.StdEncoding.DecodeString(v); err == nil {
					return string(dec)
				}
			}
		}
	}
	if m, _ := errObj["message"].(string); m != "" {
		return m
	}
	if c, _ := errObj["code"].(string); c != "" {
		return c
	}
	return "Unknown error"
}

type cursorAgentOutcome struct {
	fullText    string
	inputTok    int
	outputTok   int
	errText     string
	earlyFinish bool
}

func cursorApplyCustomHeaders(req *http.Request, ch *types.Channel) {
	if req == nil || ch == nil {
		return
	}
	for _, h := range ch.CustomHeader {
		k := strings.TrimSpace(h.Key)
		if k == "" {
			continue
		}
		req.Header.Set(k, h.Value)
	}
}

func (r *CursorRelay) runCursorAgent(
	ctx context.Context,
	ch *types.Channel,
	baseURL string,
	cred cursorCredentials,
	prompt string,
	cursorModel string,
	onDelta func(string),
) (*cursorAgentOutcome, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = cursorDefaultBaseURL
	}
	runURL := baseURL + cursorAgentRunPath

	pr, pw := io.Pipe()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, runURL, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("connect-protocol-version", "1")
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("x-cursor-checksum", cursorChecksum(cred.MachineID, cred.MacMachineID))
	req.Header.Set("x-cursor-client-version", cred.ClientVersion)
	tz := time.Now().Location().String()
	if tz == "" || tz == "Local" {
		tz = "UTC"
	}
	req.Header.Set("x-cursor-timezone", tz)
	req.Header.Set("x-request-id", uuid.NewString())
	cursorApplyCustomHeaders(req, ch)

	convID := uuid.NewString()
	runReq := map[string]any{
		"runRequest": map[string]any{
			"conversationState": map[string]any{},
			"action": map[string]any{
				"userMessageAction": map[string]any{
					"userMessage": map[string]any{"text": prompt},
				},
			},
			"modelDetails": map[string]any{
				"modelId":          cursorModel,
				"displayName":      cursorModel,
				"displayNameShort": cursorModel,
			},
			"requestedModel": map[string]any{"modelId": cursorModel},
			"conversationId": convID,
		},
	}

	frameMu := &sync.Mutex{}
	pumpCtx, pumpCancel := context.WithCancel(ctx)
	defer pumpCancel()

	outcome := &cursorAgentOutcome{}

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- cursorWritePump(pumpCtx, pw, frameMu, runReq)
	}()

	client := cursorAgentRunHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		pumpCancel()
		_ = pr.CloseWithError(err)
		<-writerDone
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		pumpCancel()
		_ = pr.Close()
		<-writerDone
		return nil, fmt.Errorf("cursor upstream %d: %s", resp.StatusCode, string(b))
	}

	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			pumpCancel()
			_ = pr.Close()
			<-writerDone
			return outcome, ctx.Err()
		default:
		}
		n, rerr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		offset := 0
		for offset+5 <= len(buf) {
			length := int(buf[offset+1])<<24 | int(buf[offset+2])<<16 | int(buf[offset+3])<<8 | int(buf[offset+4])
			if length < 0 || length > 50*1024*1024 || offset+5+length > len(buf) {
				break
			}
			payload := buf[offset+5 : offset+5+length]
			offset += 5 + length
			var msg map[string]any
			if json.Unmarshal(payload, &msg) != nil {
				continue
			}
			if errText := cursorParseErrorPayload(msg); errText != "" {
				outcome.errText = errText
				outcome.earlyFinish = true
				break
			}
			if msg["kvServerMessage"] != nil {
				continue
			}
			if msg["conversationCheckpointUpdate"] != nil {
				continue
			}
			if msg["interactionQuery"] != nil {
				continue
			}
			if ok, inT, outT := cursorExtractTurnEnded(msg); ok {
				outcome.inputTok = inT
				outcome.outputTok = outT
				outcome.earlyFinish = true
				break
			}
			if es, ok := msg["execServerMessage"].(map[string]any); ok {
				cursorExecReply(es, func(m map[string]any) {
					if err := cursorWriteFrameSync(frameMu, pw, m); err != nil {
						outcome.errText = fmt.Sprintf("cursor exec reply write: %v", err)
						outcome.earlyFinish = true
					}
				})
				if outcome.earlyFinish {
					break
				}
				continue
			}
			if iu, ok := msg["interactionUpdate"].(map[string]any); ok {
				end, inT, outT := cursorProcessInteractionUpdate(iu, onDelta)
				if end {
					outcome.inputTok = inT
					outcome.outputTok = outT
					outcome.earlyFinish = true
					break
				}
			}
		}
		buf = buf[offset:]
		if outcome.earlyFinish {
			break
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			pumpCancel()
			_ = pr.Close()
			<-writerDone
			return outcome, rerr
		}
	}

	pumpCancel()
	_ = pr.Close()
	<-writerDone
	return outcome, nil
}

// cursorWriteFrameSync writes one Connect-style frame to the request body pipe.
// Caller must use the same mutex as cursorWritePump so exec replies interleave safely with heartbeats.
func cursorWriteFrameSync(mu *sync.Mutex, pw *io.PipeWriter, obj map[string]any) error {
	mu.Lock()
	defer mu.Unlock()
	frame, err := encodeCursorFrame(obj)
	if err != nil {
		return err
	}
	_, err = pw.Write(frame)
	return err
}

func cursorWritePump(ctx context.Context, pw *io.PipeWriter, frameMu *sync.Mutex, initial map[string]any) error {
	if err := cursorWriteFrameSync(frameMu, pw, initial); err != nil {
		_ = pw.CloseWithError(err)
		return err
	}
	tick := time.NewTicker(cursorHeartbeatInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = pw.Close()
			return ctx.Err()
		case <-tick.C:
			if err := cursorWriteFrameSync(frameMu, pw, map[string]any{"clientHeartbeat": map[string]any{}}); err != nil {
				_ = pw.CloseWithError(err)
				return err
			}
		}
	}
}

func (r *CursorRelay) cursorChannelBaseURL(ch *types.Channel) string {
	if ch == nil || len(ch.BaseUrls) == 0 {
		return cursorDefaultBaseURL
	}
	u := strings.TrimSpace(ch.BaseUrls[0].URL)
	if u == "" {
		return cursorDefaultBaseURL
	}
	return strings.TrimRight(u, "/")
}

// ProxyNonStreaming runs Cursor Agent and returns an OpenAI chat.completion JSON object.
func (r *CursorRelay) ProxyNonStreaming(
	ctx context.Context,
	ch *types.Channel,
	rawKey string,
	requestModel string,
	cursorModel string,
	body map[string]any,
) (*relay.ProxyResult, error) {
	if r == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}
	prompt, err := cursorMessagesToPrompt(body)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}

	var acc strings.Builder
	outcome, err := r.runCursorAgent(ctx, ch, r.cursorChannelBaseURL(ch), cred, prompt, cursorModel, func(s string) {
		acc.WriteString(s)
	})
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("cursor agent: %v", err), StatusCode: http.StatusBadGateway}
	}
	if outcome.errText != "" {
		return nil, &relay.ProxyError{Message: outcome.errText, StatusCode: http.StatusBadGateway}
	}
	text := acc.String()
	resp := cursorOpenAIChatCompletionResponse(requestModel, text, outcome.inputTok, outcome.outputTok)
	return &relay.ProxyResult{
		Response:        resp,
		InputTokens:     outcome.inputTok,
		OutputTokens:    outcome.outputTok,
		StatusCode:      http.StatusOK,
		UpstreamHeaders: http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func cursorChatCompletionID() string {
	s := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(s) > 24 {
		s = s[:24]
	}
	return "chatcmpl-" + s
}

// cursorStreamOpenAIMeta holds a stable id/created timestamp for one streamed completion.
type cursorStreamOpenAIMeta struct {
	CompletionID string
	Created      int64
	Model        string
}

func newCursorStreamOpenAIMeta(requestedModel string) cursorStreamOpenAIMeta {
	return cursorStreamOpenAIMeta{
		CompletionID: cursorChatCompletionID(),
		Created:      time.Now().Unix(),
		Model:        requestedModel,
	}
}

func (m cursorStreamOpenAIMeta) openAIChunkMap(delta string, finishReason *string) map[string]any {
	deltaMap := map[string]any{}
	if delta != "" {
		deltaMap["content"] = delta
	}
	return map[string]any{
		"id":      m.CompletionID,
		"object":  "chat.completion.chunk",
		"created": m.Created,
		"model":   m.Model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         deltaMap,
			"finish_reason": finishReason,
		}},
	}
}

func (m cursorStreamOpenAIMeta) openAIChunkJSON(delta string, finishReason *string) []byte {
	b, _ := json.Marshal(m.openAIChunkMap(delta, finishReason))
	return b
}

func (m cursorStreamOpenAIMeta) sseRole() []byte {
	ch := map[string]any{
		"id":      m.CompletionID,
		"object":  "chat.completion.chunk",
		"created": m.Created,
		"model":   m.Model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"role": "assistant", "content": ""},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(ch)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data: %s\n\n", b)
	return buf.Bytes()
}

func (m cursorStreamOpenAIMeta) sseData(delta string, finishReason *string) []byte {
	b, _ := json.Marshal(m.openAIChunkMap(delta, finishReason))
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data: %s\n\n", b)
	return buf.Bytes()
}

func cursorOpenAIChatCompletionResponse(requestedModel, text string, inTok, outTok int) map[string]any {
	id := cursorChatCompletionID()
	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   requestedModel,
		"choices": []any{map[string]any{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": text,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     inTok,
			"completion_tokens": outTok,
			"total_tokens":      inTok + outTok,
		},
	}
}

// ProxyStreaming streams completion chunks to w.
// Default: OpenAI chat.completion.chunk SSE. With anthropicInbound or geminiNative,
// applies the same conversions as the generic relay (OpenAI SSE → Anthropic / Gemini stream JSON).
func (r *CursorRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	ch *types.Channel,
	rawKey string,
	requestModel string,
	cursorModel string,
	body map[string]any,
	anthropicInbound bool,
	geminiNative bool,
) (*relay.StreamCompleteInfo, error) {
	if r == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}
	prompt, err := cursorMessagesToPrompt(body)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusBadRequest}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	meta := newCursorStreamOpenAIMeta(requestModel)
	var convertAnthropic func(string) []string
	if anthropicInbound {
		convertAnthropic = relay.CreateOpenAIToAnthropicSSEConverter()
	}
	var gemAccum *protocol.OpenAIToGeminiAccum
	if geminiNative {
		gemAccum = protocol.NewOpenAIToGeminiAccum()
	}

	openAIPassthrough := convertAnthropic == nil && gemAccum == nil

	if openAIPassthrough {
		_, _ = w.Write(meta.sseRole())
		if flusher != nil {
			flusher.Flush()
		}
	}

	emitFromOpenAIJSON := func(chunkJSON []byte) {
		if len(chunkJSON) == 0 {
			return
		}
		if convertAnthropic != nil {
			for _, line := range convertAnthropic(string(chunkJSON)) {
				fmt.Fprintf(w, "%s\n", line)
			}
			return
		}
		if gemAccum != nil {
			for _, gl := range protocol.ConvertOpenAIChunkToGemini(chunkJSON, gemAccum) {
				fmt.Fprintf(w, "data: %s\n\n", gl)
			}
		}
	}

	// After 200 + SSE headers, clients (e.g. Anthropic /v1/messages) must always see a terminal
	// chunk; otherwise UIs spin forever if the agent stream stalls or errors mid-flight.
	sseTerminalSent := false
	emitStreamTerminal := func() {
		if sseTerminalSent {
			return
		}
		sseTerminalSent = true
		stop := "stop"
		if openAIPassthrough {
			_, _ = w.Write(meta.sseData("", &stop))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		} else {
			emitFromOpenAIJSON(meta.openAIChunkJSON("", &stop))
			if convertAnthropic != nil {
				for _, line := range convertAnthropic("[DONE]") {
					fmt.Fprintf(w, "%s\n", line)
				}
			}
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	defer emitStreamTerminal()

	var acc strings.Builder
	started := time.Now()
	firstMs := 0
	var sentFirst bool

	outcome, agentErr := r.runCursorAgent(ctx, ch, r.cursorChannelBaseURL(ch), cred, prompt, cursorModel, func(s string) {
		if s == "" {
			return
		}
		if !sentFirst {
			firstMs = int(time.Since(started).Milliseconds())
			sentFirst = true
		}
		acc.WriteString(s)
		if openAIPassthrough {
			_, _ = w.Write(meta.sseData(s, nil))
		} else {
			emitFromOpenAIJSON(meta.openAIChunkJSON(s, nil))
		}
		if flusher != nil {
			flusher.Flush()
		}
	})
	if agentErr != nil {
		msg := fmt.Sprintf("\n\n[Error: cursor agent: %v]", agentErr)
		if openAIPassthrough {
			_, _ = w.Write(meta.sseData(msg, nil))
		} else {
			emitFromOpenAIJSON(meta.openAIChunkJSON(msg, nil))
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	if outcome != nil && outcome.errText != "" {
		errText := "\n\n[Error: " + outcome.errText + "]"
		if openAIPassthrough {
			_, _ = w.Write(meta.sseData(errText, nil))
		} else {
			emitFromOpenAIJSON(meta.openAIChunkJSON(errText, nil))
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	inT, outT := 0, 0
	if outcome != nil {
		inT, outT = outcome.inputTok, outcome.outputTok
	}
	return &relay.StreamCompleteInfo{
		InputTokens:     inT,
		OutputTokens:    outT,
		FirstTokenTime:  firstMs,
		ResponseContent: acc.String(),
		UpstreamHeaders: w.Header().Clone(),
	}, nil
}

// FetchUsableModels returns model IDs from Cursor GetUsableModels (optional; for “fetch models” UI).
func (r *CursorRelay) FetchUsableModels(ctx context.Context, ch *types.Channel, rawKey string) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("cursor relay not configured")
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, err
	}
	base := r.cursorChannelBaseURL(ch)
	url := base + cursorAgentModelsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("connect-protocol-version", "1")
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("x-cursor-checksum", cursorChecksum(cred.MachineID, cred.MacMachineID))
	req.Header.Set("x-cursor-client-version", cred.ClientVersion)
	req.Header.Set("x-request-id", uuid.NewString())
	cursorApplyCustomHeaders(req, ch)

	resp, err := cursorSharedH2Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cursor models %d: %s", resp.StatusCode, string(b))
	}
	out, err := cursorModelIDsFromUsableModelsJSON(b)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		out = cursorFallbackUsableModelIDs()
	}
	return out, nil
}
