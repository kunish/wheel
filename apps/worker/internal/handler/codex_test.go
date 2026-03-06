package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
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

	items, total := filterAndPaginateAuthFiles(files, "codex", "", 1, 1)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(items) != 1 || items[0].Name != "a.json" {
		t.Fatalf("page result unexpected: %+v", items)
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
	models, err := h.collectCodexChannelModels(t.Context(), 7, []codexAuthFile{
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

func TestUploadCodexAuthFileBatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	h, mock := newCodexUploadTestHandler(t)
	expectCodexChannelLookups(mock, 7)
	expectCodexAuthFileInsert(mock, nil)
	expectCodexAuthFileInsert(mock, errors.New("duplicate auth file"))
	expectCodexAuthFileListForSync(mock, 7, []string{"valid.json"})

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
	if resp.Data.Total != 3 || resp.Data.SuccessCount != 1 || resp.Data.FailedCount != 2 {
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
	if resp.Data.Results[2].Name != "valid.json" || resp.Data.Results[2].Status != "error" || resp.Data.Results[2].Error == "" {
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
	expectCodexAuthFileInsert(mock, nil)
	expectCodexAuthFileListForSync(mock, 9, []string{"single.json"})

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

func expectCodexAuthFileInsert(mock sqlmock.Sqlmock, err error) {
	exec := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `codex_auth_files`"))
	if err != nil {
		exec.WillReturnError(err)
		return
	}
	exec.WillReturnResult(sqlmock.NewResult(1, 1))
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
