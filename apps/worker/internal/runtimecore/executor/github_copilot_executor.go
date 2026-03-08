package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	translatorbridge "github.com/kunish/wheel/apps/worker/internal/runtimeapi/translatorbridge"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	runtimetranslator "github.com/kunish/wheel/apps/worker/internal/runtimecore/translator"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	githubCopilotChatPath      = "/chat/completions"
	githubCopilotResponsesPath = "/responses"
	githubCopilotBaseURL       = "https://api.githubcopilot.com"
	githubCopilotTokenCacheTTL = 25 * time.Minute
	tokenExpiryBuffer          = 5 * time.Minute
	copilotUserAgent           = "GitHubCopilotChat/0.35.0"
	copilotEditorVersion       = "vscode/1.107.0"
	copilotPluginVersion       = "copilot-chat/0.35.0"
	copilotIntegrationID       = "vscode-chat"
	copilotOpenAIIntent        = "conversation-panel"
	copilotGitHubAPIVer        = "2025-04-01"
)

type cachedAPIToken struct {
	token       string
	apiEndpoint string
	expiresAt   time.Time
}

// GitHubCopilotExecutor handles the owned GitHub Copilot execution path used by Wheel runtime tests.
type GitHubCopilotExecutor struct {
	cfg   *runtimeconfig.Config
	mu    sync.RWMutex
	cache map[string]*cachedAPIToken
	trans *translatorbridge.Adapter
}

func NewGitHubCopilotExecutor(cfg *runtimeconfig.Config) *GitHubCopilotExecutor {
	return &GitHubCopilotExecutor{cfg: cfg, cache: make(map[string]*cachedAPIToken), trans: translatorbridge.Default()}
}

func (e *GitHubCopilotExecutor) Identifier() string { return "github-copilot" }

func (e *GitHubCopilotExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	apiToken, baseURL, err := e.ensureAPIToken(auth)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}

	from := opts.SourceFormat
	useResponses := useGitHubCopilotResponsesEndpoint(from.String(), req.Model)
	to := "openai"
	if useResponses {
		to = "openai-response"
	}
	body := e.trans.TranslateRequest(translateFormat(from.String()), translateFormat(to), req.Model, bytes.Clone(req.Payload), false)
	body = e.normalizeModel(req.Model, body)
	body = flattenAssistantContent(body)
	body = normalizeGitHubCopilotChatTools(body)
	body, _ = sjson.SetBytes(body, "stream", false)

	path := githubCopilotChatPath
	if useResponses {
		path = githubCopilotResponsesPath
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	e.applyHeaders(httpReq, apiToken, body)

	httpResp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return cliproxyexecutor.Response{}, statusErr{code: httpResp.StatusCode, msg: string(data)}
	}
	return cliproxyexecutor.Response{Payload: data}, nil
}

func (e *GitHubCopilotExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	apiToken, baseURL, err := e.ensureAPIToken(auth)
	if err != nil {
		return nil, err
	}

	from := opts.SourceFormat
	useResponses := useGitHubCopilotResponsesEndpoint(from.String(), req.Model)
	to := "openai"
	if useResponses {
		to = "openai-response"
	}
	body := e.trans.TranslateRequest(translateFormat(from.String()), translateFormat(to), req.Model, bytes.Clone(req.Payload), true)
	body = e.normalizeModel(req.Model, body)
	body = flattenAssistantContent(body)
	body = normalizeGitHubCopilotChatTools(body)
	body, _ = sjson.SetBytes(body, "stream", true)

	path := githubCopilotChatPath
	if useResponses {
		path = githubCopilotResponsesPath
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	e.applyHeaders(httpReq, apiToken, body)

	httpResp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		defer func() { _ = httpResp.Body.Close() }()
		data, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, statusErr{code: httpResp.StatusCode, msg: string(data)}
	}

	out := make(chan cliproxyexecutor.StreamChunk)
	go func() {
		defer close(out)
		defer func() { _ = httpResp.Body.Close() }()

		scanner := bufio.NewScanner(httpResp.Body)
		var param any
		for scanner.Scan() {
			line := bytes.Clone(scanner.Bytes())
			chunks := e.trans.TranslateStream(ctx, translateFormat(to), translateFormat(from.String()), req.Model, bytes.Clone(opts.OriginalRequest), body, line, &param)
			for i := range chunks {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunks[i])}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- cliproxyexecutor.StreamChunk{Err: err}
		}
	}()

	return &cliproxyexecutor.StreamResult{Headers: httpResp.Header.Clone(), Chunks: out}, nil
}

func (e *GitHubCopilotExecutor) CountTokens(context.Context, *cliproxyauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "count tokens not supported for github-copilot"}
}

func (e *GitHubCopilotExecutor) Refresh(_ context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	if metaStringValue(auth.Metadata, "access_token") == "" {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing github access token"}
	}
	return auth, nil
}

func (e *GitHubCopilotExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("github-copilot executor: request is nil")
	}
	apiToken, _, err := e.ensureAPIToken(auth)
	if err != nil {
		return nil, err
	}
	httpReq := req.Clone(ctx)
	e.applyHeaders(httpReq, apiToken, nil)
	return (&http.Client{}).Do(httpReq)
}

func (e *GitHubCopilotExecutor) ensureAPIToken(auth *cliproxyauth.Auth) (string, string, error) {
	if auth == nil {
		return "", "", statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	accessToken := metaStringValue(auth.Metadata, "access_token")
	if accessToken == "" {
		return "", "", statusErr{code: http.StatusUnauthorized, msg: "missing github access token"}
	}
	e.mu.RLock()
	if cached, ok := e.cache[accessToken]; ok && cached.expiresAt.After(time.Now().Add(tokenExpiryBuffer)) {
		e.mu.RUnlock()
		return cached.token, cached.apiEndpoint, nil
	}
	e.mu.RUnlock()

	return "", "", statusErr{code: http.StatusUnauthorized, msg: "missing cached copilot api token"}
}

func (e *GitHubCopilotExecutor) applyHeaders(r *http.Request, apiToken string, body []byte) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+apiToken)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("User-Agent", copilotUserAgent)
	r.Header.Set("Editor-Version", copilotEditorVersion)
	r.Header.Set("Editor-Plugin-Version", copilotPluginVersion)
	r.Header.Set("Openai-Intent", copilotOpenAIIntent)
	r.Header.Set("Copilot-Integration-Id", copilotIntegrationID)
	r.Header.Set("X-Github-Api-Version", copilotGitHubAPIVer)
	r.Header.Set("X-Request-Id", uuid.NewString())
	initiator := "user"
	if role := detectLastConversationRole(body); role == "assistant" || role == "tool" {
		initiator = "agent"
	}
	r.Header.Set("X-Initiator", initiator)
}

func (e *GitHubCopilotExecutor) normalizeModel(model string, body []byte) []byte {
	baseModel := parseModelSuffix(model)
	if strings.HasPrefix(baseModel, "claude-") {
		baseModel = strings.Replace(baseModel, "-4-1", "-4.1", 1)
		baseModel = strings.Replace(baseModel, "-4-5", "-4.5", 1)
		baseModel = strings.Replace(baseModel, "-4-6", "-4.6", 1)
	}
	if baseModel != model {
		body, _ = sjson.SetBytes(body, "model", baseModel)
	}
	return body
}

func useGitHubCopilotResponsesEndpoint(sourceFormat string, model string) bool {
	if sourceFormat == "openai-response" {
		return true
	}
	return strings.Contains(strings.ToLower(parseModelSuffix(model)), "codex")
}

func parseModelSuffix(model string) string {
	if idx := strings.IndexByte(model, '('); idx >= 0 {
		return strings.TrimSpace(model[:idx])
	}
	return strings.TrimSpace(model)
}

func metaStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	v, _ := metadata[key].(string)
	return strings.TrimSpace(v)
}

func translateFormat(v string) runtimetranslator.Format {
	return runtimetranslator.FromString(v)
}

type statusErr struct {
	code int
	msg  string
}

func (e statusErr) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}

func (e statusErr) StatusCode() int { return e.code }

func detectLastConversationRole(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		arr := messages.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			if role := arr[i].Get("role").String(); role != "" {
				return role
			}
		}
	}
	if inputs := gjson.GetBytes(body, "input"); inputs.Exists() && inputs.IsArray() {
		arr := inputs.Array()
		for i := len(arr) - 1; i >= 0; i-- {
			item := arr[i]
			if role := item.Get("role").String(); role != "" {
				return role
			}
			switch item.Get("type").String() {
			case "function_call", "function_call_arguments":
				return "assistant"
			case "function_call_output", "function_call_response", "tool_result":
				return "tool"
			}
		}
	}
	return ""
}

func flattenAssistantContent(body []byte) []byte {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}
	result := body
	for i, msg := range messages.Array() {
		if msg.Get("role").String() != "assistant" {
			continue
		}
		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		hasNonText := false
		var textParts []string
		for _, part := range content.Array() {
			if t := part.Get("type").String(); t != "" && t != "text" {
				hasNonText = true
				break
			}
			if part.Get("type").String() == "text" {
				if txt := part.Get("text").String(); txt != "" {
					textParts = append(textParts, txt)
				}
			}
		}
		if hasNonText {
			continue
		}
		result, _ = sjson.SetBytes(result, fmt.Sprintf("messages.%d.content", i), strings.Join(textParts, ""))
	}
	return result
}

func normalizeGitHubCopilotChatTools(body []byte) []byte {
	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return body
	}
	if toolChoice.Type == gjson.String {
		switch toolChoice.String() {
		case "auto", "none", "required":
			return body
		}
	}
	body, _ = sjson.SetBytes(body, "tool_choice", "auto")
	return body
}

func decodeJSON(raw []byte) map[string]any {
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
