package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

type startCodexOAuthTestResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL                    string `json:"url"`
		State                  string `json:"state"`
		FlowType               string `json:"flowType"`
		UserCode               string `json:"user_code,omitempty"`
		VerificationURI        string `json:"verification_uri,omitempty"`
		SupportsManualCallback bool   `json:"supportsManualCallbackImport"`
		ExpiresAt              string `json:"expiresAt"`
	} `json:"data"`
}

type codexOAuthTransportResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Status                       string `json:"status"`
		Phase                        string `json:"phase"`
		Code                         string `json:"code,omitempty"`
		Error                        string `json:"error,omitempty"`
		ExpiresAt                    string `json:"expiresAt,omitempty"`
		CanRetry                     bool   `json:"canRetry"`
		SupportsManualCallbackImport bool   `json:"supportsManualCallbackImport"`
	} `json:"data"`
}

type codexOAuthCallbackResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Status                string `json:"status"`
		Phase                 string `json:"phase"`
		Code                  string `json:"code,omitempty"`
		Error                 string `json:"error,omitempty"`
		ShouldContinuePolling bool   `json:"shouldContinuePolling"`
	} `json:"data"`
}

func TestStartCodexOAuth_ReturnsExistingSessionWhenForceRestartIsFalse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 101, types.OutboundCodex)

	expiresAt := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	storeOAuthSession("existing-state", codexOAuthSession{
		ChannelID:      101,
		Provider:       "codex",
		FlowType:       "redirect",
		URL:            "https://auth.openai.com/authorize?state=existing-state",
		SupportsManual: true,
		State:          "existing-state",
		ExpiresAt:      expiresAt,
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/101/codex/oauth/start", bytes.NewReader([]byte(`{"force_restart":false}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "101"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decodeStartCodexOAuthResponse(t, rec)
	if resp.Data.State != "existing-state" {
		t.Fatalf("state = %q, want existing-state", resp.Data.State)
	}
	if resp.Data.URL != "https://auth.openai.com/authorize?state=existing-state" {
		t.Fatalf("url = %q", resp.Data.URL)
	}
	if resp.Data.FlowType != "redirect" {
		t.Fatalf("flowType = %q, want redirect", resp.Data.FlowType)
	}
	if !resp.Data.SupportsManualCallback {
		t.Fatal("supportsManualCallbackImport = false, want true")
	}
	if resp.Data.ExpiresAt != expiresAt.Format(time.RFC3339) {
		t.Fatalf("expiresAt = %q, want %q", resp.Data.ExpiresAt, expiresAt.Format(time.RFC3339))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_ForceRestartSupersedesSameChannelSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	managementCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		managementCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://auth.openai.com/authorize?state=new-state","state":"new-state"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 102, types.OutboundCodex)

	storeOAuthSession("old-state", codexOAuthSession{
		ChannelID:      102,
		Provider:       "codex",
		FlowType:       "redirect",
		URL:            "https://auth.openai.com/authorize?state=old-state",
		SupportsManual: true,
		State:          "old-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/102/codex/oauth/start", bytes.NewReader([]byte(`{"force_restart":true}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "102"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if managementCalls != 1 {
		t.Fatalf("managementCalls = %d, want 1", managementCalls)
	}
	resp := decodeStartCodexOAuthResponse(t, rec)
	if resp.Data.State != "new-state" {
		t.Fatalf("state = %q, want new-state", resp.Data.State)
	}
	if existing, ok := loadOAuthSession("old-state"); !ok {
		t.Fatal("old session missing, want retained superseded record")
	} else {
		if existing.LastPhase != "expired" {
			t.Fatalf("old session phase = %q, want expired", existing.LastPhase)
		}
		if existing.LastCode != "session_superseded" {
			t.Fatalf("old session code = %q, want session_superseded", existing.LastCode)
		}
	}
	if next, ok := loadOAuthSession("new-state"); !ok {
		t.Fatal("new session missing, want stored active record")
	} else if next.LastPhase != "awaiting_callback" {
		t.Fatalf("new session phase = %q, want awaiting_callback", next.LastPhase)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_RejectsForceRestartAcrossChannels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	managementCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		managementCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://auth.openai.com/authorize?state=new-state","state":"new-state"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 312, types.OutboundCodex)

	storeOAuthSession("channel-311-state", codexOAuthSession{
		ChannelID:      311,
		Provider:       "codex",
		FlowType:       "redirect",
		URL:            "https://auth.openai.com/authorize?state=channel-311-state",
		SupportsManual: true,
		State:          "channel-311-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/312/codex/oauth/start", bytes.NewReader([]byte(`{"force_restart":true}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "312"}}

	h.StartCodexOAuth(c)

	decodeCodexOAuthError(t, rec, http.StatusConflict, "another runtime OAuth session is already active on this worker; wait for it to finish or expire before starting a new one")
	if managementCalls != 0 {
		t.Fatalf("managementCalls = %d, want 0", managementCalls)
	}
	if _, ok := loadOAuthSession("channel-311-state"); !ok {
		t.Fatal("existing session missing, want retained active record")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_RejectsConcurrentSessionAcrossChannels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	managementCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		managementCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://auth.openai.com/authorize?state=new-state","state":"new-state"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 302, types.OutboundCodex)

	storeOAuthSession("channel-301-state", codexOAuthSession{
		ChannelID:      301,
		Provider:       "codex",
		FlowType:       "redirect",
		URL:            "https://auth.openai.com/authorize?state=channel-301-state",
		SupportsManual: true,
		State:          "channel-301-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/302/codex/oauth/start", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "302"}}

	h.StartCodexOAuth(c)

	decodeCodexOAuthError(t, rec, http.StatusConflict, "another runtime OAuth session is already active on this worker; wait for it to finish or expire before starting a new one")
	if managementCalls != 0 {
		t.Fatalf("managementCalls = %d, want 0", managementCalls)
	}
	if _, ok := loadOAuthSession("channel-301-state"); !ok {
		t.Fatal("existing session missing, want retained active record")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_AllowsConcurrentSessionAcrossDifferentImportScopes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	managementCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		managementCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://github.com/login/device","state":"copilot-state","user_code":"ABCD-EFGH","verification_uri":"https://github.com/login/device"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 304, types.OutboundCopilot)

	storeOAuthSession("channel-303-state", codexOAuthSession{
		ChannelID:      303,
		Provider:       "codex",
		FlowType:       "redirect",
		URL:            "https://auth.openai.com/authorize?state=channel-303-state",
		SupportsManual: true,
		State:          "channel-303-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/304/copilot/oauth/start", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "304"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if managementCalls != 1 {
		t.Fatalf("managementCalls = %d, want 1", managementCalls)
	}
	resp := decodeStartCodexOAuthResponse(t, rec)
	if resp.Data.State != "copilot-state" {
		t.Fatalf("state = %q, want copilot-state", resp.Data.State)
	}
	if resp.Data.FlowType != "device_code" {
		t.Fatalf("flowType = %q, want device_code", resp.Data.FlowType)
	}
	if _, ok := loadOAuthSession("channel-303-state"); !ok {
		t.Fatal("existing codex session missing, want retained active record")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_ReturnsRedirectMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://auth.openai.com/authorize?state=redirect-state","state":"redirect-state"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 103, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/103/codex/oauth/start", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "103"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decodeStartCodexOAuthResponse(t, rec)
	if resp.Data.FlowType != "redirect" {
		t.Fatalf("flowType = %q, want redirect", resp.Data.FlowType)
	}
	if !resp.Data.SupportsManualCallback {
		t.Fatal("supportsManualCallbackImport = false, want true")
	}
	if resp.Data.UserCode != "" {
		t.Fatalf("user_code = %q, want empty", resp.Data.UserCode)
	}
	if resp.Data.VerificationURI != "" {
		t.Fatalf("verification_uri = %q, want empty", resp.Data.VerificationURI)
	}
	if resp.Data.ExpiresAt == "" {
		t.Fatal("expiresAt = empty, want RFC3339 timestamp")
	}
	if _, err := time.Parse(time.RFC3339, resp.Data.ExpiresAt); err != nil {
		t.Fatalf("expiresAt parse error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_ReturnsDeviceCodeMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://github.com/login/device","state":"device-state","user_code":"ABCD-EFGH","verification_uri":"https://github.com/login/device"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 104, types.OutboundCopilot)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/104/copilot/oauth/start", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "104"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decodeStartCodexOAuthResponse(t, rec)
	if resp.Data.FlowType != "device_code" {
		t.Fatalf("flowType = %q, want device_code", resp.Data.FlowType)
	}
	if resp.Data.UserCode != "ABCD-EFGH" {
		t.Fatalf("user_code = %q, want ABCD-EFGH", resp.Data.UserCode)
	}
	if resp.Data.VerificationURI != "https://github.com/login/device" {
		t.Fatalf("verification_uri = %q", resp.Data.VerificationURI)
	}
	if resp.Data.SupportsManualCallback {
		t.Fatal("supportsManualCallbackImport = true, want false")
	}
	if resp.Data.ExpiresAt == "" {
		t.Fatal("expiresAt = empty, want RFC3339 timestamp")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_ReturnsExpiredForMissingSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 201, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/201/codex/oauth/status?state=missing-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "201"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "expired" {
		t.Fatalf("status = %q, want expired", resp.Data.Status)
	}
	if resp.Data.Phase != "expired" {
		t.Fatalf("phase = %q, want expired", resp.Data.Phase)
	}
	if resp.Data.Code != "session_missing" {
		t.Fatalf("code = %q, want session_missing", resp.Data.Code)
	}
	if !resp.Data.CanRetry {
		t.Fatal("canRetry = false, want true")
	}
	if !resp.Data.SupportsManualCallbackImport {
		t.Fatal("supportsManualCallbackImport = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_ReturnsExpiredForSupersededSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 202, types.OutboundCodex)

	storeOAuthSession("superseded-state", codexOAuthSession{
		ChannelID:      202,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "superseded-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})
	supersedeOAuthSessions(202, "codex", "next-state")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/202/codex/oauth/status?state=superseded-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "202"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "expired" {
		t.Fatalf("status = %q, want expired", resp.Data.Status)
	}
	if resp.Data.Code != "session_superseded" {
		t.Fatalf("code = %q, want session_superseded", resp.Data.Code)
	}
	if resp.Data.Phase != "expired" {
		t.Fatalf("phase = %q, want expired", resp.Data.Phase)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_MapsRuntimeWaitToAwaitingCallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"waiting"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 203, types.OutboundCodex)
	storeOAuthSession("waiting-state", codexOAuthSession{
		ChannelID:      203,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "waiting-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/203/codex/oauth/status?state=waiting-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "203"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "waiting" {
		t.Fatalf("status = %q, want waiting", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if resp.Data.Code != "" {
		t.Fatalf("code = %q, want empty", resp.Data.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_MapsDeviceCodeWaitToAwaitingBrowser(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"waiting"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 211, types.OutboundCopilot)
	storeOAuthSession("device-wait-state", codexOAuthSession{
		ChannelID:      211,
		Provider:       "github",
		FlowType:       "device_code",
		SupportsManual: false,
		State:          "device-wait-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_browser",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/211/copilot/oauth/status?state=device-wait-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "211"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "waiting" {
		t.Fatalf("status = %q, want waiting", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_browser" {
		t.Fatalf("phase = %q, want awaiting_browser", resp.Data.Phase)
	}
	if resp.Data.SupportsManualCallbackImport {
		t.Fatal("supportsManualCallbackImport = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_RetainsCompletedSessionAfterSuccessfulImport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 212, types.OutboundCodex)
	expectCodexChannelTypeLookup(mock, 212, types.OutboundCodex)
	expectCodexAuthFileListForSync(mock, 212, nil)
	expectCodexChannelTypeLookup(mock, 212, types.OutboundCodex)
	expectCodexAuthFileListForSync(mock, 212, nil)
	expectCodexChannelTypeLookup(mock, 212, types.OutboundCodex)
	storeOAuthSession("complete-state", codexOAuthSession{
		ChannelID:      212,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "complete-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "callback_received",
		Existing:       map[string]struct{}{},
	})

	first := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(first)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/212/codex/oauth/status?state=complete-state", nil)
	c1.Params = gin.Params{{Key: "id", Value: "212"}}
	h.GetCodexOAuthStatus(c1)

	firstResp := decodeCodexOAuthTransportResponse(t, first, http.StatusOK)
	if firstResp.Data.Status != "ok" {
		t.Fatalf("first status = %q, want ok", firstResp.Data.Status)
	}
	if firstResp.Data.Phase != "completed" {
		t.Fatalf("first phase = %q, want completed", firstResp.Data.Phase)
	}

	second := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(second)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/212/codex/oauth/status?state=complete-state", nil)
	c2.Params = gin.Params{{Key: "id", Value: "212"}}
	h.GetCodexOAuthStatus(c2)

	secondResp := decodeCodexOAuthTransportResponse(t, second, http.StatusOK)
	if secondResp.Data.Status != "ok" {
		t.Fatalf("second status = %q, want ok", secondResp.Data.Status)
	}
	if secondResp.Data.Phase != "completed" {
		t.Fatalf("second phase = %q, want completed", secondResp.Data.Phase)
	}
	if statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want 1", statusCalls)
	}
	if stored, ok := loadOAuthSession("complete-state"); !ok {
		t.Fatal("completed session missing, want retained terminal record")
	} else if stored.LastPhase != "completed" {
		t.Fatalf("stored phase = %q, want completed", stored.LastPhase)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_DegradesUnexpectedOKWithoutCallbackProgression(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 213, types.OutboundCodex)
	storeOAuthSession("lost-state", codexOAuthSession{
		ChannelID:      213,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "lost-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
		Existing:       map[string]struct{}{},
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/213/codex/oauth/status?state=lost-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "213"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "expired" {
		t.Fatalf("status = %q, want expired", resp.Data.Status)
	}
	if resp.Data.Phase != "expired" {
		t.Fatalf("phase = %q, want expired", resp.Data.Phase)
	}
	if resp.Data.Code != "session_missing" {
		t.Fatalf("code = %q, want session_missing", resp.Data.Code)
	}
	if statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want 1", statusCalls)
	}
	if stored, ok := loadOAuthSession("lost-state"); !ok {
		t.Fatal("session missing, want retained expired record")
	} else if stored.LastCode != "session_missing" || stored.LastPhase != "expired" {
		t.Fatalf("stored session = %+v, want expired session_missing", stored)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_AcceptsDuplicateReplay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	callbackHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"waiting"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 204, types.OutboundCodex)
	expectCodexChannelTypeLookup(mock, 204, types.OutboundCodex)
	storeOAuthSession("callback-state", codexOAuthSession{
		ChannelID:      204,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "callback-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	callbackURL := `http://localhost:1455/callback?code=abc123&state=callback-state`
	first := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(first)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/204/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"`+callbackURL+`"}`)))
	c1.Request.Header.Set("Content-Type", "application/json")
	c1.Params = gin.Params{{Key: "id", Value: "204"}}

	h.SubmitCodexOAuthCallback(c1)

	firstResp := decodeCodexOAuthCallbackResponse(t, first, http.StatusOK)
	if firstResp.Data.Status != "accepted" {
		t.Fatalf("first status = %q, want accepted", firstResp.Data.Status)
	}
	if firstResp.Data.Phase != "callback_received" {
		t.Fatalf("first phase = %q, want callback_received", firstResp.Data.Phase)
	}
	if !firstResp.Data.ShouldContinuePolling {
		t.Fatal("first shouldContinuePolling = false, want true")
	}

	second := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(second)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/204/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"`+callbackURL+`"}`)))
	c2.Request.Header.Set("Content-Type", "application/json")
	c2.Params = gin.Params{{Key: "id", Value: "204"}}

	h.SubmitCodexOAuthCallback(c2)

	secondResp := decodeCodexOAuthCallbackResponse(t, second, http.StatusOK)
	if secondResp.Data.Status != "duplicate" {
		t.Fatalf("second status = %q, want duplicate", secondResp.Data.Status)
	}
	if secondResp.Data.Phase != "callback_received" {
		t.Fatalf("second phase = %q, want callback_received", secondResp.Data.Phase)
	}
	if secondResp.Data.Code != "duplicate_callback" {
		t.Fatalf("second code = %q, want duplicate_callback", secondResp.Data.Code)
	}
	if !secondResp.Data.ShouldContinuePolling {
		t.Fatal("second shouldContinuePolling = false, want true")
	}
	if callbackHits != 1 {
		t.Fatalf("callbackHits = %d, want 1", callbackHits)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_RejectsStateMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 205, types.OutboundCodex)
	storeOAuthSession("expected-state", codexOAuthSession{
		ChannelID:      205,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "expected-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/205/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"http://localhost:1455/callback?code=abc123&state=other-state"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "205"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if resp.Data.Code != "state_mismatch" {
		t.Fatalf("code = %q, want state_mismatch", resp.Data.Code)
	}
	if resp.Data.ShouldContinuePolling {
		t.Fatal("shouldContinuePolling = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_ReturnsStructuredInvalidCallbackURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 206, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/206/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"://bad"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "206"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if resp.Data.Code != "invalid_callback_url" {
		t.Fatalf("code = %q, want invalid_callback_url", resp.Data.Code)
	}
	if resp.Data.Error != "The pasted callback URL is invalid." {
		t.Fatalf("error = %q, want human readable callback validation message", resp.Data.Error)
	}
	if resp.Data.ShouldContinuePolling {
		t.Fatal("shouldContinuePolling = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_ReturnsStructuredMissingState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 214, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/214/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"http://localhost:1455/callback?code=abc123"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "214"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Code != "missing_state" {
		t.Fatalf("code = %q, want missing_state", resp.Data.Code)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_ReturnsExpiredForMissingSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 207, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/207/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"http://localhost:1455/callback?code=abc123&state=missing-state"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "207"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Code != "session_missing" {
		t.Fatalf("code = %q, want session_missing", resp.Data.Code)
	}
	if resp.Data.Phase != "expired" {
		t.Fatalf("phase = %q, want expired", resp.Data.Phase)
	}
	if resp.Data.ShouldContinuePolling {
		t.Fatal("shouldContinuePolling = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_ReturnsFailedWhenAuthImportFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 208, types.OutboundCodex)
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		t.Fatalf("resolveCodexLocalAuthDir() error = %v", err)
	}
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", authDir, err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "broken.json"), []byte(`{"type":"codex","email":"broken@example.com"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") AND (name = 'broken.json')")).
		WillReturnError(errors.New("sql: no rows in result set"))
	expectCodexAuthFileInsert(mock, errors.New("insert failed"))
	storeOAuthSession("import-state", codexOAuthSession{
		ChannelID:      208,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "import-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "callback_received",
		Existing:       map[string]struct{}{},
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/208/codex/oauth/status?state=import-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "208"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Phase != "failed" {
		t.Fatalf("phase = %q, want failed", resp.Data.Phase)
	}
	if resp.Data.Code != "auth_import_failed" {
		t.Fatalf("code = %q, want auth_import_failed", resp.Data.Code)
	}
	if resp.Data.Error == "" {
		t.Fatal("error = empty, want human-readable message")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_ImportsOnlyMatchingProviderScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 216, types.OutboundCodex)
	authDir, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		t.Fatalf("resolveCodexLocalAuthDir() error = %v", err)
	}
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", authDir, err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "codex.json"), []byte(`{"type":"codex","email":"codex@example.com","access_token":"codex-token"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(codex.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "copilot.json"), []byte(`{"type":"github-copilot","email":"copilot@example.com","access_token":"copilot-token"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(copilot.json) error = %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") AND (name = 'codex.json')")).
		WillReturnError(errors.New("sql: no rows in result set"))
	expectCodexAuthFileInsert(mock, nil)
	storeOAuthSession("provider-scope-state", codexOAuthSession{
		ChannelID:      216,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "provider-scope-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "callback_received",
		Existing:       map[string]struct{}{},
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/216/codex/oauth/status?state=provider-scope-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "216"}}

	h.GetCodexOAuthStatus(c)

	resp := decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "ok" {
		t.Fatalf("status = %q, want ok", resp.Data.Status)
	}
	if resp.Data.Phase != "completed" {
		t.Fatalf("phase = %q, want completed", resp.Data.Phase)
	}
	if _, err := os.Stat(filepath.Join(authDir, "codex.json")); !os.IsNotExist(err) {
		t.Fatalf("codex.json still exists after import, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(authDir, "copilot.json")); err != nil {
		t.Fatalf("copilot.json missing after scoped import, err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetCodexOAuthStatus_LogsPhaseTransitionsWithoutSensitiveCallbackData(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"waiting"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 209, types.OutboundCodex)
	storeOAuthSession("logged-state", codexOAuthSession{
		ChannelID:      209,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "logged-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "initializing",
	})

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/209/codex/oauth/status?state=logged-state", nil)
	c.Params = gin.Params{{Key: "id", Value: "209"}}

	h.GetCodexOAuthStatus(c)

	decodeCodexOAuthTransportResponse(t, rec, http.StatusOK)
	logOutput := logs.String()
	if !strings.Contains(logOutput, "oauth phase transition") {
		t.Fatalf("logs = %q, want oauth phase transition entry", logOutput)
	}
	if strings.Contains(logOutput, "logged-state") {
		t.Fatalf("logs leaked oauth state: %q", logOutput)
	}
	if strings.Contains(logOutput, "code=abc123") || strings.Contains(logOutput, "access_token") || strings.Contains(logOutput, "callback?") {
		t.Fatalf("logs leaked sensitive callback data: %q", logOutput)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_RejectsProviderMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: "http://127.0.0.1:1",
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 210, types.OutboundCopilot)
	storeOAuthSession("provider-state", codexOAuthSession{
		ChannelID:      210,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "provider-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/210/copilot/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"http://localhost:1455/callback?code=abc123&state=provider-state"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "210"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if resp.Data.Code != "provider_mismatch" {
		t.Fatalf("code = %q, want provider_mismatch", resp.Data.Code)
	}
	if resp.Data.ShouldContinuePolling {
		t.Fatal("shouldContinuePolling = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSubmitCodexOAuthCallback_MapsRuntimeBusinessErrorIntoCallbackContract(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)
	resetCodexOAuthSessionsForTest()

	callbackHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","error":"oauth flow is not pending"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 215, types.OutboundCodex)
	storeOAuthSession("runtime-error-state", codexOAuthSession{
		ChannelID:      215,
		Provider:       "codex",
		FlowType:       "redirect",
		SupportsManual: true,
		State:          "runtime-error-state",
		ExpiresAt:      time.Now().Add(10 * time.Minute),
		LastPhase:      "awaiting_callback",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/215/codex/oauth/callback", bytes.NewReader([]byte(`{"callback_url":"http://localhost:1455/callback?code=abc123&state=runtime-error-state"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "215"}}

	h.SubmitCodexOAuthCallback(c)

	resp := decodeCodexOAuthCallbackResponse(t, rec, http.StatusOK)
	if resp.Data.Status != "error" {
		t.Fatalf("status = %q, want error", resp.Data.Status)
	}
	if resp.Data.Phase != "awaiting_callback" {
		t.Fatalf("phase = %q, want awaiting_callback", resp.Data.Phase)
	}
	if resp.Data.Code != "runtime_callback_rejected" {
		t.Fatalf("code = %q, want runtime_callback_rejected", resp.Data.Code)
	}
	if resp.Data.Error != "oauth flow is not pending" {
		t.Fatalf("error = %q, want oauth flow is not pending", resp.Data.Error)
	}
	if resp.Data.ShouldContinuePolling {
		t.Fatal("shouldContinuePolling = true, want false")
	}
	if callbackHits != 1 {
		t.Fatalf("callbackHits = %d, want 1", callbackHits)
	}
	if stored, ok := loadOAuthSession("runtime-error-state"); !ok {
		t.Fatal("session missing, want pending session retained")
	} else if stored.LastPhase != "awaiting_callback" {
		t.Fatalf("stored phase = %q, want awaiting_callback", stored.LastPhase)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func decodeStartCodexOAuthResponse(t *testing.T, rec *httptest.ResponseRecorder) startCodexOAuthTestResponse {
	t.Helper()
	var resp startCodexOAuthTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	return resp
}

func decodeCodexOAuthTransportResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) codexOAuthTransportResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp codexOAuthTransportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	return resp
}

func decodeCodexOAuthCallbackResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) codexOAuthCallbackResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp codexOAuthCallbackResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	return resp
}

func decodeCodexOAuthError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantError string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if resp.Success {
		t.Fatalf("success = true, body = %s", rec.Body.String())
	}
	if resp.Error != wantError {
		t.Fatalf("error = %q, want %q", resp.Error, wantError)
	}
}

func resetCodexOAuthSessionsForTest() {
	codexOAuthSessions.Range(func(key, _ any) bool {
		codexOAuthSessions.Delete(key)
		return true
	})
}
