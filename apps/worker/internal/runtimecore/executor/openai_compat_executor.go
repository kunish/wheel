package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	translatorbridge "github.com/kunish/wheel/apps/worker/internal/runtimeapi/translatorbridge"
	runtimeconfig "github.com/kunish/wheel/apps/worker/internal/runtimecore/config"
	runtimetranslator "github.com/kunish/wheel/apps/worker/internal/runtimecore/translator"
	cliproxyauth "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy/executor"
)

type OpenAICompatExecutor struct {
	provider string
	cfg      *runtimeconfig.Config
	trans    *translatorbridge.Adapter
}

func NewOpenAICompatExecutor(provider string, cfg *runtimeconfig.Config) *OpenAICompatExecutor {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "openai-compatibility"
	}
	return &OpenAICompatExecutor{provider: provider, cfg: cfg, trans: translatorbridge.Default()}
}

func (e *OpenAICompatExecutor) Identifier() string { return e.provider }

func (e *OpenAICompatExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	if req == nil {
		return nil
	}
	_, apiKey := e.resolveCredentials(auth)
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if auth != nil {
		applyCustomHeadersFromAttrs(req, auth.Attributes)
	}
	return nil
}

func (e *OpenAICompatExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("openai compat executor: request is nil")
	}
	if ctx == nil {
		ctx = req.Context()
	}
	httpReq := req.WithContext(ctx)
	if err := e.PrepareRequest(httpReq, auth); err != nil {
		return nil, err
	}
	return (&http.Client{}).Do(httpReq)
}

func (e *OpenAICompatExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	baseURL, apiKey := e.resolveCredentials(auth)
	if baseURL == "" {
		return cliproxyexecutor.Response{}, statusErr{code: http.StatusUnauthorized, msg: "missing provider baseURL"}
	}

	from := opts.SourceFormat.String()
	to := runtimetranslator.FormatOpenAI
	endpoint := "/chat/completions"
	if opts.Alt == "responses/compact" {
		to = runtimetranslator.FormatOpenAIResponse
		endpoint = "/responses/compact"
	}
	translated := e.trans.TranslateRequest(runtimetranslator.FromString(from), to, req.Model, bytes.Clone(req.Payload), opts.Stream)
	url := strings.TrimSuffix(baseURL, "/") + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(translated))
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "wheel-openai-compat")
	if auth != nil {
		applyCustomHeadersFromAttrs(httpReq, auth.Attributes)
	}

	httpResp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return cliproxyexecutor.Response{}, statusErr{code: httpResp.StatusCode, msg: string(body)}
	}
	translatedResp := e.trans.TranslateNonStream(ctx, to, runtimetranslator.FromString(from), req.Model, opts.OriginalRequest, translated, body, nil)
	return cliproxyexecutor.Response{Payload: []byte(translatedResp), Headers: httpResp.Header.Clone()}, nil
}

func (e *OpenAICompatExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	baseURL, apiKey := e.resolveCredentials(auth)
	if baseURL == "" {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing provider baseURL"}
	}

	from := opts.SourceFormat.String()
	to := runtimetranslator.FormatOpenAI
	translated := e.trans.TranslateRequest(runtimetranslator.FromString(from), to, req.Model, bytes.Clone(req.Payload), true)
	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(translated))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "wheel-openai-compat")
	if auth != nil {
		applyCustomHeadersFromAttrs(httpReq, auth.Attributes)
	}

	httpResp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		defer func() { _ = httpResp.Body.Close() }()
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, statusErr{code: httpResp.StatusCode, msg: string(body)}
	}

	out := make(chan cliproxyexecutor.StreamChunk)
	go func() {
		defer close(out)
		defer func() { _ = httpResp.Body.Close() }()
		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(nil, 1024*1024)
		var param any
		for scanner.Scan() {
			line := bytes.Clone(scanner.Bytes())
			if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			chunks := e.trans.TranslateStream(ctx, to, runtimetranslator.FromString(from), req.Model, opts.OriginalRequest, translated, line, &param)
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

func (e *OpenAICompatExecutor) CountTokens(ctx context.Context, _ *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	from := opts.SourceFormat.String()
	to := runtimetranslator.FormatOpenAI
	translated := e.trans.TranslateRequest(runtimetranslator.FromString(from), to, req.Model, bytes.Clone(req.Payload), false)
	enc, err := tokenizerForModel(req.Model)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	count, err := countOpenAIChatTokens(enc, translated)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	usage := buildOpenAIUsageJSON(count)
	translatedUsage := e.trans.TranslateTokenCount(ctx, to, runtimetranslator.FromString(from), count, usage)
	return cliproxyexecutor.Response{Payload: []byte(translatedUsage)}, nil
}

func (e *OpenAICompatExecutor) Refresh(_ context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	return auth, nil
}

func (e *OpenAICompatExecutor) resolveCredentials(auth *cliproxyauth.Auth) (baseURL, apiKey string) {
	if auth == nil || auth.Attributes == nil {
		return "", ""
	}
	baseURL = strings.TrimSpace(auth.Attributes["base_url"])
	apiKey = strings.TrimSpace(auth.Attributes["api_key"])
	return baseURL, apiKey
}
