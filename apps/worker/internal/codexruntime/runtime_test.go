package codexruntime

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
)

func TestMaterializeChannelAuthFiles_PreservesExistingFilesOnWriteFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := EnsureManagedConfig(""); err != nil {
		t.Fatalf("EnsureManagedConfig() error = %v", err)
	}
	authDir := ManagedAuthDir()
	existingPath := filepath.Join(authDir, ManagedAuthFileName(7, "old.json"))
	if err := os.WriteFile(existingPath, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatalf("seed existing auth file: %v", err)
	}
	blockingPath := filepath.Join(authDir, ManagedAuthFileName(7, "new.json"))
	if err := os.MkdirAll(blockingPath, 0o700); err != nil {
		t.Fatalf("create blocking dir: %v", err)
	}

	db, mock := newRuntimeTestDB(t)
	expectRuntimeCodexAuthFileList(mock, 7, []string{"new.json"}, []string{`{"fresh":true}`})

	err := MaterializeChannelAuthFiles(t.Context(), db, 7)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, statErr := os.Stat(existingPath); statErr != nil {
		t.Fatalf("expected existing auth file to remain, stat error = %v", statErr)
	}
}

func TestEnsureManagedConfig_PreservesExistingManagementKeyWhenEmptyKeyPassed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := EnsureManagedConfig("secret-key"); err != nil {
		t.Fatalf("EnsureManagedConfig(secret-key) error = %v", err)
	}
	if err := EnsureManagedConfig(""); err != nil {
		t.Fatalf("EnsureManagedConfig(empty) error = %v", err)
	}

	content, err := os.ReadFile(ManagedConfigPath())
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	if !strings.Contains(string(content), "secret-key: secret-key") {
		t.Fatalf("expected management key to be preserved, config = %s", string(content))
	}
}

func TestMaterializeAuthFiles_PreservesExistingFilesOnWriteFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := EnsureManagedConfig(""); err != nil {
		t.Fatalf("EnsureManagedConfig() error = %v", err)
	}
	authDir := ManagedAuthDir()
	existingPath := filepath.Join(authDir, ManagedAuthFileName(7, "old.json"))
	if err := os.WriteFile(existingPath, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatalf("seed existing auth file: %v", err)
	}

	db, mock := newRuntimeTestDB(t)
	expectRuntimeChannelList(mock, []int{7}, []string{"codex"})
	expectRuntimeChannelKeyList(mock)
	expectRuntimeCodexAuthFileList(mock, 7, []string{strings.Repeat("a", 300) + ".json"}, []string{`{"fresh":true}`})

	err := MaterializeAuthFiles(t.Context(), db)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, statErr := os.Stat(existingPath); statErr != nil {
		t.Fatalf("expected existing auth file to remain, stat error = %v", statErr)
	}
}

func newRuntimeTestDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqldb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqldb.Close()
	})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("8.0.36"))
	return bun.NewDB(sqldb, mysqldialect.New()), mock
}

func expectRuntimeCodexAuthFileList(mock sqlmock.Sqlmock, channelID int, names []string, contents []string) {
	rows := sqlmock.NewRows([]string{"id", "channel_id", "name", "provider", "email", "disabled", "content", "created_at", "updated_at"})
	for i, name := range names {
		rows.AddRow(i+1, channelID, name, "codex", name+"@example.com", false, contents[i], "2026-03-06 00:00:00", "2026-03-06 00:00:00")
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `codex_auth_file`.`id`, `codex_auth_file`.`channel_id`, `codex_auth_file`.`name`, `codex_auth_file`.`provider`, `codex_auth_file`.`email`, `codex_auth_file`.`disabled`, `codex_auth_file`.`content`, `codex_auth_file`.`created_at`, `codex_auth_file`.`updated_at` FROM `codex_auth_files` AS `codex_auth_file` WHERE (channel_id = ") + "[0-9]+" + regexp.QuoteMeta(") ORDER BY name ASC")).
		WillReturnRows(rows)
}

func expectRuntimeChannelList(mock sqlmock.Sqlmock, ids []int, types []string) {
	rows := sqlmock.NewRows([]string{"id", "name", "type", "enabled", "base_urls", "model", "fetched_model", "custom_model", "proxy", "auto_sync", "auto_group", "custom_header", "param_override", "channel_proxy", "order"})
	for i, id := range ids {
		rows.AddRow(id, "channel", types[i], true, "[]", "[]", "[]", "", false, false, "", "[]", nil, nil, i)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel`.`id`, `channel`.`name`, `channel`.`type`, `channel`.`enabled`, `channel`.`base_urls`, `channel`.`model`, `channel`.`fetched_model`, `channel`.`custom_model`, `channel`.`proxy`, `channel`.`auto_sync`, `channel`.`auto_group`, `channel`.`custom_header`, `channel`.`param_override`, `channel`.`channel_proxy`, `channel`.`order` FROM `channels` AS `channel` ORDER BY `order` ASC, id ASC")).
		WillReturnRows(rows)
}

func expectRuntimeChannelKeyList(mock sqlmock.Sqlmock) {
	rows := sqlmock.NewRows([]string{"id", "channel_id", "enabled", "channel_key", "status_code", "last_use_timestamp", "total_cost", "remark"})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `channel_key`.`id`, `channel_key`.`channel_id`, `channel_key`.`enabled`, `channel_key`.`channel_key`, `channel_key`.`status_code`, `channel_key`.`last_use_timestamp`, `channel_key`.`total_cost`, `channel_key`.`remark` FROM `channel_keys` AS `channel_key`")).
		WillReturnRows(rows)
}
