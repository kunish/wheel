package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) codexManagementCall(c *gin.Context, method string, path string, query url.Values, reqBody any, out any) error {
	return h.codexManagementCallContext(c.Request.Context(), method, path, query, reqBody, out)
}

func (h *Handler) codexManagementCallContext(ctx context.Context, method string, path string, query url.Values, reqBody any, out any) error {
	base := strings.TrimRight(strings.TrimSpace(h.Config.CodexRuntimeManagementURL), "/")
	fullURL := base + "/v0/management" + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var payload io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, payload)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Management-Key", strings.TrimSpace(h.Config.CodexRuntimeManagementKey))

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request codex management: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read codex management response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("codex management error: %s", msg)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode codex management response: %w", err)
		}
	}

	return nil
}

func (h *Handler) codexManagementUploadFile(ctx context.Context, filename string, content []byte, out any) error {
	base := strings.TrimRight(strings.TrimSpace(h.Config.CodexRuntimeManagementURL), "/")
	fullURL := base + "/v0/management/auth-files"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("create multipart form file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("write multipart file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, &body)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Management-Key", strings.TrimSpace(h.Config.CodexRuntimeManagementKey))

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request codex management upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read codex management upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("codex management error: %s", msg)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode codex management upload response: %w", err)
		}
	}

	return nil
}
