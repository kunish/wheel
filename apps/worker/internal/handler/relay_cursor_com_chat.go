package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Cursor web chat API (browser-style), used when client requests include tools.
// Reference behaviour: https://github.com/7836246/cursor2api — POST https://cursor.com/api/chat, SSE events type "text-delta".

const cursorComChatURL = "https://cursor.com/api/chat"

func cursorComChatUserAgent() string {
	if s := strings.TrimSpace(os.Getenv("CURSOR_COM_CHAT_USER_AGENT")); s != "" {
		return s
	}
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
}

func cursorComChatIdleTimeout() time.Duration {
	if s := strings.TrimSpace(os.Getenv("CURSOR_COM_CHAT_IDLE_TIMEOUT")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return 120 * time.Second
}

// cursorPreferAgentOverWebChat is deprecated for routing: when an HTTP client exists, Cursor channels always use
// cursor.com/api/chat. This function remains for compatibility; CURSOR_USE_AGENT is ignored in that case.
func cursorPreferAgentOverWebChat() bool {
	s := strings.TrimSpace(os.Getenv("CURSOR_USE_AGENT"))
	return s == "1" || strings.EqualFold(s, "true") || strings.EqualFold(s, "yes")
}

func cursorComChatAPIModel(mappedTarget string) string {
	mappedTarget = strings.TrimSpace(mappedTarget)
	if mappedTarget == "" {
		if s := strings.TrimSpace(os.Getenv("CURSOR_COM_CHAT_MODEL")); s != "" {
			return s
		}
		return "anthropic/claude-sonnet-4.6"
	}
	if strings.Contains(mappedTarget, "/") {
		return mappedTarget
	}
	// Wheel Cursor ids look like "claude-4.5-sonnet"; Cursor web API expects "anthropic/...".
	return "anthropic/" + mappedTarget
}

func cursorComChatChromeHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	h.Set("X-Path", "/api/chat")
	h.Set("Sec-Ch-Ua", `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`)
	h.Set("X-Method", "POST")
	h.Set("Sec-Ch-Ua-Bitness", `"64"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Arch", `"x86"`)
	h.Set("Sec-Ch-Ua-Platform-Version", `"19.0.0"`)
	h.Set("Origin", "https://cursor.com")
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Referer", "https://cursor.com/")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Priority", "u=1, i")
	h.Set("User-Agent", cursorComChatUserAgent())
	h.Set("X-Is-Human", "")
	return h
}

func cursorDeriveConversationID(anth map[string]any) string {
	h := sha256.New()
	if sys := anth["system"]; sys != nil {
		switch v := sys.(type) {
		case string:
			if len(v) > 500 {
				v = v[:500]
			}
			_, _ = h.Write([]byte(v))
		case []any:
			var b strings.Builder
			for _, blk := range v {
				m, ok := blk.(map[string]any)
				if !ok {
					continue
				}
				if t, _ := m["type"].(string); t == "text" {
					if tx, _ := m["text"].(string); tx != "" {
						b.WriteString(tx)
					}
				}
			}
			s := b.String()
			if len(s) > 500 {
				s = s[:500]
			}
			_, _ = h.Write([]byte(s))
		}
	}
	if msgs, ok := anth["messages"].([]any); ok {
		for _, m := range msgs {
			msg, ok := m.(map[string]any)
			if !ok {
				continue
			}
			if r, _ := msg["role"].(string); r == "user" {
				switch c := msg["content"].(type) {
				case string:
					s := c
					if len(s) > 1000 {
						s = s[:1000]
					}
					_, _ = h.Write([]byte(s))
				case []any:
					b, _ := json.Marshal(c)
					if len(b) > 1000 {
						b = b[:1000]
					}
					_, _ = h.Write(b)
				}
				break
			}
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:16]
}

// cursorAccessTokenFromChannelKey returns the Bearer token for Cursor web chat / Agent (same as channel key JSON).
func cursorAccessTokenFromChannelKey(raw string) string {
	cred, err := parseCursorCredentials(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cred.AccessToken)
}

// cursorApplyComChatAuth adds session auth expected by cursor.com/api/chat (Bearer + optional Cookie).
func cursorApplyComChatAuth(hdr http.Header, accessToken string) {
	if hdr == nil {
		return
	}
	if t := strings.TrimSpace(accessToken); t != "" {
		hdr.Set("Authorization", "Bearer "+t)
	}
	if c := strings.TrimSpace(os.Getenv("CURSOR_COM_CHAT_COOKIE")); c != "" {
		hdr.Set("Cookie", c)
	}
}

func cursorShortID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// cursorComChatExtractTextDelta pulls assistant text from SSE JSON events (shape varies by Cursor web deploy).
func cursorComChatExtractTextDelta(ev map[string]any) string {
	if ev == nil {
		return ""
	}
	typ, _ := ev["type"].(string)
	if typ == "thinking-delta" {
		return ""
	}
	switch typ {
	case "text-delta", "content-delta", "assistant-delta", "message-delta":
		if d, _ := ev["delta"].(string); d != "" {
			return d
		}
	case "":
		if d, _ := ev["delta"].(string); d != "" {
			return d
		}
		if t, _ := ev["text"].(string); t != "" {
			return t
		}
	default:
		if strings.HasSuffix(typ, "-delta") {
			if d, _ := ev["delta"].(string); d != "" {
				return d
			}
		}
	}
	if p, ok := ev["part"].(map[string]any); ok {
		if t, _ := p["text"].(string); t != "" {
			return t
		}
	}
	return ""
}

// cursorComChatEventError detects terminal error events in the cursor.com SSE stream.
func cursorComChatEventError(ev map[string]any) (msg string, ok bool) {
	if ev == nil {
		return "", false
	}
	typ, _ := ev["type"].(string)
	if typ == "error" || strings.HasSuffix(typ, "_error") {
		if s, _ := ev["message"].(string); s != "" {
			return s, true
		}
		if errObj, okm := ev["error"].(map[string]any); okm {
			if s, _ := errObj["message"].(string); s != "" {
				return s, true
			}
		}
		if s, _ := ev["error"].(string); s != "" {
			return s, true
		}
	}
	if errObj, okm := ev["error"].(map[string]any); okm {
		if s, _ := errObj["message"].(string); s != "" {
			return s, true
		}
	}
	return "", false
}

// postCursorCom accumulates text-delta events from cursor.com/api/chat (idle-timeout between chunks).
func postCursorComChat(ctx context.Context, client *http.Client, chatBody map[string]any, accessToken string, onDelta func(string)) error {
	payload, err := json.Marshal(chatBody)
	if err != nil {
		return err
	}
	idle := cursorComChatIdleTimeout()
	reqCtx, reqCancel := context.WithCancel(ctx)
	defer reqCancel()

	lastMu := sync.Mutex{}
	lastActivity := time.Now()
	resetIdle := func() {
		lastMu.Lock()
		lastActivity = time.Now()
		lastMu.Unlock()
	}
	resetIdle()

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-reqCtx.Done():
				return
			case <-t.C:
				lastMu.Lock()
				l := lastActivity
				lastMu.Unlock()
				if time.Since(l) > idle {
					reqCancel()
					return
				}
			}
		}
	}()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cursorComChatURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header = cursorComChatChromeHeaders()
	cursorApplyComChatAuth(req.Header, accessToken)

	if client == nil {
		client = http.DefaultClient
	}
	origClient := client
	if client.Timeout > 0 {
		client = &http.Client{
			Transport:     origClient.Transport,
			CheckRedirect: origClient.CheckRedirect,
			Jar:           origClient.Jar,
			Timeout:       0,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			lastMu.Lock()
			stale := time.Since(lastActivity) > idle
			lastMu.Unlock()
			if stale {
				return fmt.Errorf("cursor.com/api/chat: idle timeout after %v", idle)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("cursor.com/api/chat: %s — %s", resp.Status, strings.TrimSpace(string(b)))
	}

	sc := bufio.NewScanner(resp.Body)
	// Large lines for SSE data
	const maxBuf = 1024 * 1024
	sc.Buffer(make([]byte, 64*1024), maxBuf)

	for sc.Scan() {
		resetIdle()
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		if errMsg, isErr := cursorComChatEventError(ev); isErr {
			return fmt.Errorf("cursor.com/api/chat: %s", errMsg)
		}
		if delta := cursorComChatExtractTextDelta(ev); delta != "" && onDelta != nil {
			onDelta(delta)
		}
	}
	err = sc.Err()
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(reqCtx.Err(), context.Canceled) {
		lastMu.Lock()
		stale := time.Since(lastActivity) > idle
		lastMu.Unlock()
		if stale {
			return fmt.Errorf("cursor.com/api/chat: idle timeout after %v", idle)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return err
}

func collectCursorComChatText(ctx context.Context, client *http.Client, chatBody map[string]any, accessToken string) (string, error) {
	var acc strings.Builder
	err := postCursorComChat(ctx, client, chatBody, accessToken, func(s string) {
		acc.WriteString(s)
	})
	return acc.String(), err
}
