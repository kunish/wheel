package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

func TestExtractCodexAccountID(t *testing.T) {
	tests := []struct {
		name string
		auth map[string]any
		want string
	}{
		{
			name: "snake case field",
			auth: map[string]any{
				"id_token": map[string]any{"chatgpt_account_id": "acc_snake"},
			},
			want: "acc_snake",
		},
		{
			name: "camel case field",
			auth: map[string]any{
				"id_token": map[string]any{"chatgptAccountId": "acc_camel"},
			},
			want: "acc_camel",
		},
		{
			name: "missing id token",
			auth: map[string]any{"name": "x"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCodexAccountID(tt.auth); got != tt.want {
				t.Fatalf("extractCodexAccountID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseQuotaWindow(t *testing.T) {
	raw := map[string]any{
		"used_percent":         98.5,
		"limit_window_seconds": 604800,
		"reset_after_seconds":  3600,
		"reset_at":             "2026-03-10T10:00:00Z",
	}

	window := parseQuotaWindow(raw)
	if window.UsedPercent != 98.5 {
		t.Fatalf("UsedPercent = %v, want 98.5", window.UsedPercent)
	}
	if window.LimitWindowSeconds != 604800 {
		t.Fatalf("LimitWindowSeconds = %d, want 604800", window.LimitWindowSeconds)
	}
	if window.ResetAfterSeconds != 3600 {
		t.Fatalf("ResetAfterSeconds = %d, want 3600", window.ResetAfterSeconds)
	}
	if window.ResetAt != "2026-03-10T10:00:00Z" {
		t.Fatalf("ResetAt = %q, want 2026-03-10T10:00:00Z", window.ResetAt)
	}
}

func TestFilterAndPaginateAuthFiles(t *testing.T) {
	files := []codexAuthFile{
		{Name: "a.json", Provider: "codex"},
		{Name: "b.json", Provider: "codex"},
		{Name: "c.json", Provider: "gemini"},
	}

	items, total := filterAndPaginateAuthFiles(files, "codex", "", "", 1, 1)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(items) != 1 || items[0].Name != "a.json" {
		t.Fatalf("page result unexpected: %+v", items)
	}
}

func TestFilterAndPaginateAuthFiles_CopilotProviderAlias(t *testing.T) {
	files := []codexAuthFile{
		{Name: "copilot.json", Provider: "github-copilot"},
	}

	items, total := filterAndPaginateAuthFiles(files, "copilot", "", "", 1, 20)
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Name != "copilot.json" {
		t.Fatalf("page result unexpected: %+v", items)
	}
}

func TestSelectCodexAuthFilesForBatch_AllMatchingSearchAndExclusions(t *testing.T) {
	files := []codexAuthFile{
		{Name: "alpha.json", Email: "alpha@example.com", Provider: "codex"},
		{Name: "beta.json", Email: "beta@example.com", Provider: "codex"},
		{Name: "gamma.json", Email: "gamma@example.com", Provider: "codex"},
		{Name: "copilot.json", Email: "copilot@example.com", Provider: "github-copilot"},
	}

	selected, err := selectCodexAuthFilesForBatch(files, codexAuthBatchScope{
		AllMatching:  true,
		Search:       "example",
		Provider:     "codex",
		ExcludeNames: []string{"beta.json"},
	})
	if err != nil {
		t.Fatalf("selectCodexAuthFilesForBatch() error = %v", err)
	}
	if got := []string{selected[0].Name, selected[1].Name}; !slices.Equal(got, []string{"alpha.json", "gamma.json"}) {
		t.Fatalf("selected names = %v, want [alpha.json gamma.json]", got)
	}
}

func TestSelectCodexAuthFilesForBatch_ExplicitNames(t *testing.T) {
	files := []codexAuthFile{
		{Name: "alpha.json", Provider: "codex"},
		{Name: "beta.json", Provider: "codex"},
		{Name: "gamma.json", Provider: "codex"},
	}

	selected, err := selectCodexAuthFilesForBatch(files, codexAuthBatchScope{Names: []string{"gamma.json", "alpha.json"}})
	if err != nil {
		t.Fatalf("selectCodexAuthFilesForBatch() error = %v", err)
	}
	if got := []string{selected[0].Name, selected[1].Name}; !slices.Equal(got, []string{"alpha.json", "gamma.json"}) {
		t.Fatalf("selected names = %v, want [alpha.json gamma.json]", got)
	}
}

func TestParseCodexAuthContent_NormalizesGitHubCopilotProvider(t *testing.T) {
	provider, email, disabled, normalized, raw, err := parseCodexAuthContent([]byte(`{"type":"github-copilot","email":"copilot@example.com"}`))
	if err != nil {
		t.Fatalf("parseCodexAuthContent() error = %v", err)
	}
	if provider != "copilot" {
		t.Fatalf("provider = %q, want copilot", provider)
	}
	if email != "copilot@example.com" {
		t.Fatalf("email = %q, want copilot@example.com", email)
	}
	if disabled {
		t.Fatal("disabled = true, want false")
	}
	if got := stringFromMap(raw, "type"); got != "github-copilot" {
		t.Fatalf("raw type = %q, want github-copilot", got)
	}
	if !strings.Contains(normalized, "github-copilot") {
		t.Fatalf("normalized = %q, want to keep github-copilot type", normalized)
	}
}

func TestCodexManagementUploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v0/management/auth-files" {
			t.Fatalf("path = %s, want /v0/management/auth-files", r.URL.Path)
		}
		if got := r.Header.Get("X-Management-Key"); got != "secret" {
			t.Fatalf("X-Management-Key = %q, want %q", got, "secret")
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content-type = %s, want multipart/form-data", mediaType)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		if got := part.FormName(); got != "file" {
			t.Fatalf("form name = %q, want %q", got, "file")
		}
		if got := part.FileName(); got != "account.json" {
			t.Fatalf("filename = %q, want %q", got, "account.json")
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		if got := string(body); got != `{"provider":"codex"}` {
			t.Fatalf("body = %q, want auth file json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	h := &Handler{Config: &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}}
	var out map[string]any
	err := h.codexManagementUploadFile(t.Context(), "account.json", []byte(`{"provider":"codex"}`), &out)
	if err != nil {
		t.Fatalf("codexManagementUploadFile() error = %v", err)
	}
	if got := out["status"]; got != "ok" {
		t.Fatalf("status = %v, want ok", got)
	}
}

func TestRegisterRoutes_RegistersCodexAuthFileUploadRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{Config: &config.Config{JWTSecret: "test-secret"}}

	h.RegisterRoutes(r)

	for _, route := range r.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/channel/:id/codex/auth-files" {
			return
		}
	}
	t.Fatalf("missing route %s %s", http.MethodPost, "/api/v1/channel/:id/codex/auth-files")
}

func TestCodexManagementUploadFile_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad auth file"))
	}))
	defer server.Close()

	h := &Handler{Config: &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}}
	err := h.codexManagementUploadFile(t.Context(), "account.json", []byte(`{}`), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad auth file") {
		t.Fatalf("error = %q, want to contain bad auth file", err.Error())
	}
}

func TestResolveCodexLocalAuthDir(t *testing.T) {
	authDir := codexruntime.ManagedAuthDir()
	h := &Handler{Config: &config.Config{}}
	got, err := h.resolveCodexLocalAuthDir()
	if err != nil {
		t.Fatalf("resolveCodexLocalAuthDir() error = %v", err)
	}
	if got != authDir {
		t.Fatalf("auth dir = %q, want %q", got, authDir)
	}
}

func TestListLocalCodexAuthFiles(t *testing.T) {
	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("mkdir auth dir: %v", err)
	}
	content := map[string]any{
		"type":         "codex",
		"email":        "user@example.com",
		"disabled":     true,
		"access_token": "tok",
		"account_id":   "acct-123",
	}
	raw, _ := json.Marshal(content)
	if err := os.WriteFile(filepath.Join(authDir, "user.json"), raw, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	h := &Handler{}
	files, err := h.listLocalAuthFiles(authDir)
	if err != nil {
		t.Fatalf("listLocalAuthFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if files[0].Name != "user.json" || files[0].Provider != "codex" || files[0].Email != "user@example.com" || !files[0].Disabled {
		t.Fatalf("unexpected file entry: %+v", files[0])
	}
	if files[0].AuthIndex == "" {
		t.Fatal("expected auth index to be populated")
	}
}

func TestPatchLocalAuthFileDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("mkdir auth dir: %v", err)
	}
	path := filepath.Join(authDir, "user.json")
	if err := os.WriteFile(path, []byte(`{"type":"codex","disabled":false}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	h := &Handler{}
	if err := h.patchLocalAuthFileDisabled(authDir, "user.json", true); err != nil {
		t.Fatalf("patchLocalAuthFileDisabled() error = %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth file: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal auth file: %v", err)
	}
	if disabled, _ := out["disabled"].(bool); !disabled {
		t.Fatalf("disabled = %v, want true", out["disabled"])
	}
}

func TestParseCodexQuotaBody_NewWhamUsageShape(t *testing.T) {
	body := `{
		"plan_type":"free",
		"rate_limit":{
			"allowed":true,
			"limit_reached":false,
			"primary_window":{
				"used_percent":12,
				"limit_window_seconds":18000,
				"reset_after_seconds":120,
				"reset_at":1735693200
			},
			"secondary_window":{
				"used_percent":34,
				"limit_window_seconds":604800,
				"reset_after_seconds":240,
				"reset_at":1736298000
			}
		},
		"additional_rate_limits":[{
			"limit_name":"codex_other",
			"metered_feature":"codex_other",
			"rate_limit":{
				"allowed":true,
				"limit_reached":false,
				"primary_window":{
					"used_percent":56,
					"limit_window_seconds":604800,
					"reset_after_seconds":360,
					"reset_at":1736299000
				}
			}
		}]
	}`

	weekly, codeReview, planType, err := parseCodexQuotaBody(body)
	if err != nil {
		t.Fatalf("parseCodexQuotaBody() error = %v", err)
	}
	if planType != "free" {
		t.Fatalf("planType = %q, want free", planType)
	}
	if weekly.UsedPercent != 34 {
		t.Fatalf("weekly used = %v, want 34", weekly.UsedPercent)
	}
	if weekly.LimitWindowSeconds != 604800 {
		t.Fatalf("weekly window = %d, want 604800", weekly.LimitWindowSeconds)
	}
	if codeReview.UsedPercent != 56 {
		t.Fatalf("codeReview used = %v, want 56", codeReview.UsedPercent)
	}
}

func TestCollectCodexChannelModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files/models" {
			t.Fatalf("path = %s, want /v0/management/auth-files/models", r.URL.Path)
		}
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		switch name {
		case "channel-7--first.json":
			_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5"},{"id":"gpt-4.1"}]}`))
		case "channel-7--second.json":
			_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5"},{"id":"o3"}]}`))
		default:
			t.Fatalf("unexpected auth file query name: %s", name)
		}
	}))
	defer server.Close()

	h := &Handler{Config: &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}}
	models, err := h.collectCodexChannelModels(t.Context(), 7, types.OutboundCodex, []codexAuthFile{
		{Name: "first.json", Provider: "codex"},
		{Name: "second.json", Provider: "codex"},
	})
	if err != nil {
		t.Fatalf("collectCodexChannelModels() error = %v", err)
	}
	if got, want := strings.Join(models, ","), "gpt-5,gpt-4.1,o3"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}
}

func TestCollectCodexChannelModels_CopilotProviderAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files/models" {
			t.Fatalf("path = %s, want /v0/management/auth-files/models", r.URL.Path)
		}
		if name := r.URL.Query().Get("name"); name != "channel-7--copilot.json" {
			t.Fatalf("unexpected auth file query name: %s", name)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5.2"}]}`))
	}))
	defer server.Close()

	h := &Handler{Config: &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}}
	models, err := h.collectCodexChannelModels(t.Context(), 7, types.OutboundCopilot, []codexAuthFile{{Name: "copilot.json", Provider: "github-copilot"}})
	if err != nil {
		t.Fatalf("collectCodexChannelModels() error = %v", err)
	}
	if got, want := strings.Join(models, ","), "gpt-5.2"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}
}

func TestCollectCodexChannelModels_PreservesNewlyExposedRuntimeModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files/models" {
			t.Fatalf("path = %s, want /v0/management/auth-files/models", r.URL.Path)
		}
		if name := r.URL.Query().Get("name"); name != "channel-9--latest.json" {
			t.Fatalf("unexpected auth file query name: %s", name)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5.4"},{"id":"gpt-5.3-codex"}]}`))
	}))
	defer server.Close()

	h := &Handler{Config: &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}}
	models, err := h.collectCodexChannelModels(t.Context(), 9, types.OutboundCodex, []codexAuthFile{{Name: "latest.json", Provider: "codex"}})
	if err != nil {
		t.Fatalf("collectCodexChannelModels() error = %v", err)
	}
	if got, want := strings.Join(models, ","), "gpt-5.4,gpt-5.3-codex"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}
}

func TestSyncCodexChannelModels_PersistsGPT54IntoModelAndFetchedModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	h, mock := newCodexUploadTestHandler(t)
	h.Cache = cache.New()
	t.Cleanup(h.Cache.Close)
	expectCodexChannelTypeLookup(mock, 55, types.OutboundCodex)
	expectCodexAuthFileListForSync(mock, 55, []string{"latest.json"})
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `channels` SET ") + ".*" + regexp.QuoteMeta("WHERE (id = 55)")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files/models" {
			t.Fatalf("path = %s, want /v0/management/auth-files/models", r.URL.Path)
		}
		if name := r.URL.Query().Get("name"); name != "channel-55--latest.json" {
			t.Fatalf("unexpected auth file query name: %s", name)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5.4"},{"id":"gpt-5.3-codex"}]}`))
	}))
	defer server.Close()
	h.Config = &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}

	err := h.syncCodexChannelModels(t.Context(), 55)
	if err != nil {
		t.Fatalf("syncCodexChannelModels() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCollectCodexChannelModels_RetriesEmptyModelsUntilRuntimeCatchesUp(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/auth-files/models" {
			t.Fatalf("path = %s, want /v0/management/auth-files/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")

		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		if currentCall == 1 {
			_, _ = w.Write([]byte(`{"models":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"id":"gpt-5"},{"id":"o3"}]}`))
	}))
	defer server.Close()

	h, _ := newCodexUploadTestHandler(t)
	h.Config = &config.Config{CodexRuntimeManagementURL: server.URL, CodexRuntimeManagementKey: "secret"}
	models, err := h.collectCodexChannelModels(t.Context(), 7, types.OutboundCodex, []codexAuthFile{{Name: "first.json", Provider: "codex"}})
	if err != nil {
		t.Fatalf("collectCodexChannelModels() error = %v", err)
	}
	if got, want := strings.Join(models, ","), "gpt-5,o3"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}
	mu.Lock()
	defer mu.Unlock()
	if callCount < 2 {
		t.Fatalf("callCount = %d, want at least 2", callCount)
	}
}

func TestCollectCodexQuotaItems_PreservesInputOrderWhenFetchCompletesOutOfOrder(t *testing.T) {
	files := []codexAuthFile{
		{
			Name:      "first.json",
			Provider:  "codex",
			Email:     "first@example.com",
			AuthIndex: "auth-first",
			Raw: map[string]any{
				"account_id": "acct-first",
			},
		},
		{
			Name:      "second.json",
			Provider:  "codex",
			Email:     "second@example.com",
			AuthIndex: "auth-second",
			Raw: map[string]any{
				"account_id": "acct-second",
			},
		},
	}

	started := make(chan string, len(files))
	releaseFirst := make(chan struct{})
	secondFinished := make(chan struct{})
	var mu sync.Mutex
	completionOrder := make([]string, 0, len(files))
	itemsCh := make(chan []codexQuotaItem, 1)

	h := &Handler{}
	go func() {
		itemsCh <- h.collectCodexQuotaItems(context.Background(), files, 2, func(_ context.Context, file codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error) {
			started <- file.Name
			if file.Name == "first.json" {
				<-releaseFirst
			}

			mu.Lock()
			completionOrder = append(completionOrder, file.Name)
			mu.Unlock()
			if file.Name == "second.json" {
				close(secondFinished)
			}

			return codexQuotaWindow{UsedPercent: float64(len(file.Name))}, codexQuotaWindow{}, file.Name + "-plan", nil
		})
	}()

	firstStarted := <-started
	secondStarted := <-started
	startedNames := []string{firstStarted, secondStarted}
	slices.Sort(startedNames)
	if !slices.Equal(startedNames, []string{"first.json", "second.json"}) {
		close(releaseFirst)
		t.Fatalf("unexpected fetch start set: %q, %q", firstStarted, secondStarted)
	}
	<-secondFinished
	close(releaseFirst)

	items := <-itemsCh
	if got, want := completionOrder, []string{"second.json", "first.json"}; !slices.Equal(got, want) {
		t.Fatalf("completion order = %v, want %v", got, want)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if got := []string{items[0].Name, items[1].Name}; !slices.Equal(got, []string{"first.json", "second.json"}) {
		t.Fatalf("item order = %v, want [first.json second.json]", got)
	}
	if got := []string{items[0].PlanType, items[1].PlanType}; !slices.Equal(got, []string{"first.json-plan", "second.json-plan"}) {
		t.Fatalf("plan types = %v, want stable mapping", got)
	}
}

func TestListCodexQuota_UsesBoundedConcurrencyAndPreservesOrder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 21)
	expectCodexAuthFileListForQuota(mock, 21, []types.CodexAuthFile{
		{ChannelID: 21, Name: "alpha.json", Provider: "codex", Email: "alpha@example.com", Content: `{"type":"codex","access_token":"token-alpha","account_id":"acct-alpha"}`},
		{ChannelID: 21, Name: "beta.json", Provider: "codex", Email: "beta@example.com", Content: `{"type":"codex","access_token":"token-beta","account_id":"acct-beta"}`},
		{ChannelID: 21, Name: "charlie.json", Provider: "codex", Email: "charlie@example.com", Content: `{"type":"codex","access_token":"token-charlie","account_id":"acct-charlie"}`},
		{ChannelID: 21, Name: "delta.json", Provider: "codex", Email: "delta@example.com", Content: `{"type":"codex","access_token":"token-delta","account_id":"acct-delta"}`},
		{ChannelID: 21, Name: "echo.json", Provider: "codex", Email: "echo@example.com", Content: `{"type":"codex","access_token":"token-echo","account_id":"acct-echo"}`},
		{ChannelID: 21, Name: "gamma.json", Provider: "codex", Email: "gamma@example.com", Content: `{"type":"codex","access_token":"token-gamma","account_id":"acct-gamma"}`},
	})

	var mu sync.Mutex
	inFlight := 0
	maxConcurrent := 0
	entered := make(chan string, 6)
	release := make(chan struct{})
	h.codexQuotaDo = func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.String() != codexQuotaEndpoint {
			t.Fatalf("url = %s, want %s", r.URL.String(), codexQuotaEndpoint)
		}

		mu.Lock()
		inFlight++
		if inFlight > maxConcurrent {
			maxConcurrent = inFlight
		}
		mu.Unlock()

		accountID := r.Header.Get("Chatgpt-Account-Id")
		entered <- accountID
		<-release

		mu.Lock()
		inFlight--
		mu.Unlock()

		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("missing Authorization header")
		}

		body := `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"secondary_window":{"used_percent":25,"limit_window_seconds":604800}},"additional_rate_limits":[{"metered_feature":"review","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":5,"limit_window_seconds":604800}}}]}`
		statusCode := http.StatusOK
		if accountID == "acct-beta" {
			statusCode = http.StatusInternalServerError
			body = `{}`
		}

		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/21/codex/quota?pageSize=6", nil)
	c.Params = gin.Params{{Key: "id", Value: "21"}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ListCodexQuota(c)
	}()

	for range codexQuotaFetchConcurrency {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for initial quota requests")
		}
	}
	select {
	case accountID := <-entered:
		close(release)
		t.Fatalf("started request beyond concurrency limit before release: %s", accountID)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ListCodexQuota to finish")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if maxConcurrent != codexQuotaFetchConcurrency {
		t.Fatalf("max concurrent fetches = %d, want %d", maxConcurrent, codexQuotaFetchConcurrency)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []codexQuotaItem `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	if resp.Data.Total != 6 {
		t.Fatalf("total = %d, want 6", resp.Data.Total)
	}
	if len(resp.Data.Items) != 6 {
		t.Fatalf("len(items) = %d, want 6", len(resp.Data.Items))
	}
	if got := []string{resp.Data.Items[0].Name, resp.Data.Items[1].Name, resp.Data.Items[2].Name, resp.Data.Items[3].Name, resp.Data.Items[4].Name, resp.Data.Items[5].Name}; !slices.Equal(got, []string{"alpha.json", "beta.json", "charlie.json", "delta.json", "echo.json", "gamma.json"}) {
		t.Fatalf("item order = %v, want stable input order", got)
	}
	if resp.Data.Items[0].PlanType != "pro" || resp.Data.Items[0].Error != "" {
		t.Fatalf("unexpected first item: %+v", resp.Data.Items[0])
	}
	if resp.Data.Items[1].Error != "quota request returned status 500" {
		t.Fatalf("unexpected second item error: %+v", resp.Data.Items[1])
	}
	for i := 2; i < len(resp.Data.Items); i++ {
		if resp.Data.Items[i].PlanType != "pro" || resp.Data.Items[i].Error != "" {
			t.Fatalf("unexpected item %d: %+v", i, resp.Data.Items[i])
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListCodexQuota_CopilotReturnsSnapshotQuota(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelTypeLookup(mock, 34, types.OutboundCopilot)
	expectCodexAuthFileListForQuota(mock, 34, []types.CodexAuthFile{
		{ChannelID: 34, Name: "copilot.json", Provider: "copilot", Email: "copilot@example.com", Content: `{"type":"github-copilot","access_token":"copilot-token"}`},
	})

	h.codexQuotaDo = func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.github.com/copilot_internal/user" {
			t.Fatalf("url = %s, want copilot quota endpoint", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer copilot-token" {
			t.Fatalf("authorization = %q", got)
		}
		body := `{
			"copilot_plan":"business",
			"quota_reset_date":"2026-03-31T00:00:00Z",
			"quota_snapshots":{
				"chat":{"quota_id":"chat","percent_remaining":75,"remaining":750,"entitlement":1000},
				"completions":{"quota_id":"completions","percent_remaining":40,"remaining":40,"entitlement":100},
				"premium_interactions":{"quota_id":"premium_interactions","percent_remaining":10,"remaining":1,"entitlement":10}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channel/34/copilot/quota", nil)
	c.Params = gin.Params{{Key: "id", Value: "34"}}

	h.ListCodexQuota(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Items []codexQuotaItem `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected items payload: %+v", resp.Data)
	}
	item := resp.Data.Items[0]
	if item.Error != "" {
		t.Fatalf("unexpected item error: %+v", item)
	}
	if item.PlanType != "business" {
		t.Fatalf("plan type = %q, want business", item.PlanType)
	}
	if item.ResetAt != "2026-03-31T00:00:00Z" {
		t.Fatalf("resetAt = %q", item.ResetAt)
	}
	if len(item.Snapshots) != 3 {
		t.Fatalf("snapshot len = %d, want 3", len(item.Snapshots))
	}
	if item.Snapshots[0].ID != "chat" || item.Snapshots[0].PercentRemaining != 75 {
		t.Fatalf("unexpected first snapshot: %+v", item.Snapshots[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPatchCodexAuthFileStatusBatch_LocalByNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	h, mock := newCodexUploadTestHandler(t)
	h.Cache = cache.New()
	t.Cleanup(h.Cache.Close)
	expectCodexChannelTypeLookup(mock, 41, types.OutboundCodex)
	expectCodexAuthFileListForQuota(mock, 41, []types.CodexAuthFile{
		{ChannelID: 41, Name: "alpha.json", Provider: "codex", Email: "alpha@example.com", Content: `{"type":"codex","email":"alpha@example.com","access_token":"token-alpha"}`},
		{ChannelID: 41, Name: "beta.json", Provider: "codex", Email: "beta@example.com", Content: `{"type":"codex","email":"beta@example.com","access_token":"token-beta"}`},
	})
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `codex_auth_files` SET ")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `codex_auth_files` SET ")).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/channel/41/codex/auth-files/status/batch", strings.NewReader(`{"names":["alpha.json","beta.json"],"disabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "41"}}

	h.PatchCodexAuthFileStatusBatch(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool                    `json:"success"`
		Data    codexAuthUploadResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || resp.Data.SuccessCount != 2 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteCodexAuthFileBatch_LocalAllMatching(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	h, mock := newCodexUploadTestHandler(t)
	h.Cache = cache.New()
	t.Cleanup(h.Cache.Close)
	expectCodexChannelTypeLookup(mock, 42, types.OutboundCodex)
	expectCodexAuthFileListForQuota(mock, 42, []types.CodexAuthFile{
		{ChannelID: 42, Name: "alpha.json", Provider: "codex", Email: "alpha@example.com", Content: `{"type":"codex","email":"alpha@example.com","access_token":"token-alpha"}`},
		{ChannelID: 42, Name: "beta.json", Provider: "codex", Email: "beta@example.com", Content: `{"type":"codex","email":"beta@example.com","access_token":"token-beta"}`},
		{ChannelID: 42, Name: "other.json", Provider: "codex", Email: "other@another.dev", Content: `{"type":"codex","email":"other@another.dev","access_token":"token-other"}`},
	})
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `codex_auth_files` AS `codex_auth_file` WHERE (id = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `codex_auth_files` AS `codex_auth_file` WHERE (id = 2)")).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/42/codex/auth-files/delete/batch", strings.NewReader(`{"allMatching":true,"search":"example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	h.DeleteCodexAuthFileBatch(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool                    `json:"success"`
		Data    codexAuthUploadResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || resp.Data.SuccessCount != 2 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUploadCodexAuthFileBatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 7)
	expectCodexAuthFileInsert(mock, nil) // single batch upsert for both valid files
	expectCodexAuthFileListForSync(mock, 7, []string{"valid.json"})
	// model sync runs asynchronously in a goroutine — no SQL expectations

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteMultipartFile(t, writer, "files", "valid.json", `{"type":"codex","email":"valid@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "broken.json", `{"type":`)
	mustWriteMultipartFile(t, writer, "files", "valid.json", `{"type":"codex","email":"duplicate@example.com","access_token":"token"}`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/7/codex/auth-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "7"}}

	h.UploadCodexAuthFile(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total        int `json:"total"`
			SuccessCount int `json:"successCount"`
			FailedCount  int `json:"failedCount"`
			Results      []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	// With batch upsert, both valid.json files succeed (ON DUPLICATE KEY UPDATE handles duplicates).
	// Only broken.json fails (parse error). Total=3, Success=2, Failed=1.
	if resp.Data.Total != 3 || resp.Data.SuccessCount != 2 || resp.Data.FailedCount != 1 {
		t.Fatalf("unexpected counts: %+v", resp.Data)
	}
	if len(resp.Data.Results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(resp.Data.Results))
	}
	if resp.Data.Results[0].Name != "valid.json" || resp.Data.Results[0].Status != "ok" || resp.Data.Results[0].Error != "" {
		t.Fatalf("unexpected first result: %+v", resp.Data.Results[0])
	}
	if resp.Data.Results[1].Name != "broken.json" || resp.Data.Results[1].Status != "error" || !strings.Contains(resp.Data.Results[1].Error, "invalid auth file json") {
		t.Fatalf("unexpected second result: %+v", resp.Data.Results[1])
	}
	if resp.Data.Results[2].Name != "valid.json" || resp.Data.Results[2].Status != "ok" {
		t.Fatalf("unexpected third result: %+v", resp.Data.Results[2])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUploadCodexAuthFileSingleFileCompatibility(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 9)
	expectCodexAuthFileInsert(mock, nil) // single batch upsert
	expectCodexAuthFileListForSync(mock, 9, []string{"single.json"})
	// model sync runs asynchronously — no SQL expectations

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteMultipartFile(t, writer, "file", "single.json", `{"type":"codex","email":"single@example.com","access_token":"token"}`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/9/codex/auth-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "9"}}

	h.UploadCodexAuthFile(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total        int `json:"total"`
			SuccessCount int `json:"successCount"`
			FailedCount  int `json:"failedCount"`
			Results      []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || resp.Data.Total != 1 || resp.Data.SuccessCount != 1 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if len(resp.Data.Results) != 1 || resp.Data.Results[0].Name != "single.json" || resp.Data.Results[0].Status != "ok" {
		t.Fatalf("unexpected results: %+v", resp.Data.Results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUploadCodexAuthFileBatch_LocalMaterializesAfterLoop(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 11)
	expectCodexAuthFileInsert(mock, nil) // single batch upsert for all 5 files
	expectCodexAuthFileListForSync(mock, 11, []string{"first.json", "second.json", "third.json", "fourth.json", "fifth.json"})
	// model sync runs asynchronously — no SQL expectations

	authDir := codexruntime.ManagedAuthDir()
	configPath := codexruntime.ManagedConfigPath()
	configDir := filepath.Dir(configPath)
	firstMaterializedPath := filepath.Join(authDir, codexruntime.ManagedAuthFileName(11, "first.json"))

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	locked := make(chan struct{}, 1)
	stopWatcher := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		for {
			select {
			case <-stopWatcher:
				return
			default:
			}
			if _, err := os.Stat(firstMaterializedPath); err == nil {
				_ = os.Chmod(configPath, 0o400)
				locked <- struct{}{}
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(stopWatcher)
		<-watcherDone
		_ = os.Chmod(configPath, 0o600)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteMultipartFile(t, writer, "files", "first.json", `{"type":"codex","email":"first@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "second.json", `{"type":"codex","email":"second@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "third.json", `{"type":"codex","email":"third@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "fourth.json", `{"type":"codex","email":"fourth@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "fifth.json", `{"type":"codex","email":"fifth@example.com","access_token":"token"}`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/11/codex/auth-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "11"}}

	h.UploadCodexAuthFile(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total        int `json:"total"`
			SuccessCount int `json:"successCount"`
			FailedCount  int `json:"failedCount"`
			Results      []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	if resp.Data.Total != 5 || resp.Data.SuccessCount != 5 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected counts: %+v", resp.Data)
	}
	for i, result := range resp.Data.Results {
		if result.Status != "ok" || result.Error != "" {
			t.Fatalf("result %d = %+v, want ok", i, result)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUploadCodexAuthFileBatch_DoesNotDependOnTempMultipartFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMPDIR", "/path/that/does/not/exist")

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 11)
	expectCodexAuthFileInsert(mock, nil) // single batch upsert
	expectCodexAuthFileListForSync(mock, 11, []string{"one.json", "two.json"})
	// model sync runs asynchronously — no SQL expectations

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteMultipartFile(t, writer, "files", "one.json", `{"type":"codex","email":"one@example.com","access_token":"token-1"}`)
	mustWriteMultipartFile(t, writer, "files", "two.json", `{"type":"codex","email":"two@example.com","access_token":"token-2"}`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	c, router := gin.CreateTestContext(rec)
	router.MaxMultipartMemory = 1
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/11/codex/auth-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "11"}}

	h.UploadCodexAuthFile(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total        int `json:"total"`
			SuccessCount int `json:"successCount"`
			FailedCount  int `json:"failedCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || resp.Data.Total != 2 || resp.Data.SuccessCount != 2 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUploadCodexAuthFileBatch_MaterializeFailurePreservesPerFileResults(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 12)
	expectCodexAuthFileInsert(mock, nil) // single batch upsert

	configPath := codexruntime.ManagedConfigPath()
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := os.Chmod(configPath, 0o400); err != nil {
		t.Fatalf("chmod config: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(configPath, 0o600)
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteMultipartFile(t, writer, "files", "first.json", `{"type":"codex","email":"first@example.com","access_token":"token"}`)
	mustWriteMultipartFile(t, writer, "files", "second.json", `{"type":"codex","email":"second@example.com","access_token":"token"}`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channel/12/codex/auth-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	h.UploadCodexAuthFile(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total        int `json:"total"`
			SuccessCount int `json:"successCount"`
			FailedCount  int `json:"failedCount"`
			Results      []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Error  string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", rec.Body.String())
	}
	if resp.Data.Total != 2 || resp.Data.SuccessCount != 2 || resp.Data.FailedCount != 0 {
		t.Fatalf("unexpected counts: %+v", resp.Data)
	}
	for i, result := range resp.Data.Results {
		if result.Status != "ok" || result.Error != "" {
			t.Fatalf("result %d = %+v, want ok without materialize error leakage", i, result)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func newCodexUploadTestHandler(t *testing.T) (*Handler, sqlmock.Sqlmock) {
	t.Helper()

	sqldb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqldb.Close()
	})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("8.0.36"))

	return &Handler{
		DB:     bun.NewDB(sqldb, mysqldialect.New()),
		Config: &config.Config{},
	}, mock
}

func expectCodexChannelLookups(mock sqlmock.Sqlmock, channelID int) {
	channelRows := sqlmock.NewRows([]string{"id", "name", "type", "enabled", "base_urls", "model", "fetched_model", "custom_model", "proxy", "auto_sync", "auto_group", "custom_header", "param_override", "channel_proxy", "order"}).
		AddRow(channelID, "Codex", types.OutboundCodex, true, "[]", "[]", "[]", "", false, false, 0, "[]", nil, nil, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel`.`id`, `channel`.`name`, `channel`.`type`, `channel`.`enabled`, `channel`.`base_urls`, `channel`.`model`, `channel`.`fetched_model`, `channel`.`custom_model`, `channel`.`proxy`, `channel`.`auto_sync`, `channel`.`auto_group`, `channel`.`custom_header`, `channel`.`param_override`, `channel`.`channel_proxy`, `channel`.`order` FROM `channels` AS `channel` WHERE (id = ") + "[0-9]+" + regexp.QuoteMeta(")")).
		WillReturnRows(channelRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel_key`.`id`, `channel_key`.`channel_id`, `channel_key`.`enabled`, `channel_key`.`channel_key`, `channel_key`.`status_code`, `channel_key`.`last_use_timestamp`, `channel_key`.`total_cost`, `channel_key`.`remark` FROM `channel_keys` AS `channel_key` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(")")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "enabled", "channel_key", "status_code", "last_use_timestamp", "total_cost", "remark"}))
}

func expectCodexChannelTypeLookup(mock sqlmock.Sqlmock, channelID int, channelType types.OutboundType) {
	channelRows := sqlmock.NewRows([]string{"id", "name", "type", "enabled", "base_urls", "model", "fetched_model", "custom_model", "proxy", "auto_sync", "auto_group", "custom_header", "param_override", "channel_proxy", "order"}).
		AddRow(channelID, "Codex", channelType, true, "[]", "[]", "[]", "", false, false, 0, "[]", nil, nil, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel`.`id`, `channel`.`name`, `channel`.`type`, `channel`.`enabled`, `channel`.`base_urls`, `channel`.`model`, `channel`.`fetched_model`, `channel`.`custom_model`, `channel`.`proxy`, `channel`.`auto_sync`, `channel`.`auto_group`, `channel`.`custom_header`, `channel`.`param_override`, `channel`.`channel_proxy`, `channel`.`order` FROM `channels` AS `channel` WHERE (id = ") + "[0-9]+" + regexp.QuoteMeta(")")).
		WillReturnRows(channelRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel_key`.`id`, `channel_key`.`channel_id`, `channel_key`.`enabled`, `channel_key`.`channel_key`, `channel_key`.`status_code`, `channel_key`.`last_use_timestamp`, `channel_key`.`total_cost`, `channel_key`.`remark` FROM `channel_keys` AS `channel_key` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(")")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel_id", "enabled", "channel_key", "status_code", "last_use_timestamp", "total_cost", "remark"}))
}

func expectCodexAuthFileInsert(mock sqlmock.Sqlmock, err error) {
	exec := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `codex_auth_files`"))
	if err != nil {
		exec.WillReturnError(err)
		return
	}
	exec.WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectCodexAuthFileListForQuota(mock sqlmock.Sqlmock, channelID int, files []types.CodexAuthFile) {
	rows := sqlmock.NewRows([]string{"id", "channel_id", "name", "provider", "email", "disabled", "content", "created_at", "updated_at"})
	for i, file := range files {
		rows.AddRow(i+1, channelID, file.Name, file.Provider, file.Email, file.Disabled, file.Content, "2026-03-06 00:00:00", "2026-03-06 00:00:00")
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") ORDER BY name ASC")).
		WillReturnRows(rows)
}

func expectCodexAuthFileListForSync(mock sqlmock.Sqlmock, channelID int, names []string) {
	rows := sqlmock.NewRows([]string{"id", "channel_id", "name", "provider", "email", "disabled", "content", "created_at", "updated_at"})
	for i, name := range names {
		rows.AddRow(i+1, channelID, name, "codex", name+"@example.com", false, `{"type":"codex","access_token":"token"}`, "2026-03-06 00:00:00", "2026-03-06 00:00:00")
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") ORDER BY name ASC")).
		WillReturnRows(rows)
}

func mustWriteMultipartFile(t *testing.T, writer *multipart.Writer, field string, name string, content string) {
	t.Helper()

	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		t.Fatalf("CreateFormFile(%q, %q) error = %v", field, name, err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("write multipart %q error = %v", name, err)
	}
}

func TestManagementAuthEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		channelType types.OutboundType
		want        string
	}{
		{"codex", types.OutboundCodex, "/codex-auth-url"},
		{"copilot", types.OutboundCopilot, "/github-auth-url"},
		{"codex-cli", types.OutboundCodexCLI, "/codex-auth-url"},
		{"antigravity", types.OutboundAntigravity, "/antigravity-auth-url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := managementAuthEndpoint(tt.channelType); got != tt.want {
				t.Fatalf("managementAuthEndpoint(%d) = %q, want %q", tt.channelType, got, tt.want)
			}
		})
	}
}

func TestStartCodexOAuth_AntigravityRoutesToAntigravityAuthURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","url":"https://accounts.google.com/o/oauth2/v2/auth?state=test123","state":"test123"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 36, types.OutboundAntigravity)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/36/codex/oauth/start", nil)
	c.Params = gin.Params{{Key: "id", Value: "36"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/v0/management/antigravity-auth-url" {
		t.Fatalf("management path = %q, want /v0/management/antigravity-auth-url", requestedPath)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_CopilotRoutesToGitHubAuthURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","url":"https://github.com/login/device","state":"test789","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 34, types.OutboundCopilot)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/34/copilot/oauth/start", nil)
	c.Params = gin.Params{{Key: "id", Value: "34"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/v0/management/github-auth-url" {
		t.Fatalf("management path = %q, want /v0/management/github-auth-url", requestedPath)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			URL      string `json:"url"`
			State    string `json:"state"`
			UserCode string `json:"user_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.UserCode != "ABCD-1234" {
		t.Fatalf("user_code = %q, want ABCD-1234", resp.Data.UserCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestStartCodexOAuth_CodexRoutesToCodexAuthURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","url":"https://auth.openai.com/authorize?state=test456","state":"test456"}`))
	}))
	defer server.Close()

	h, mock := newCodexUploadTestHandler(t)
	h.Config = &config.Config{
		CodexRuntimeManagementURL: server.URL,
		CodexRuntimeManagementKey: "secret",
	}
	expectCodexChannelTypeLookup(mock, 33, types.OutboundCodex)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/channel/33/codex/oauth/start", nil)
	c.Params = gin.Params{{Key: "id", Value: "33"}}

	h.StartCodexOAuth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if requestedPath != "/v0/management/codex-auth-url" {
		t.Fatalf("management path = %q, want /v0/management/codex-auth-url", requestedPath)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
