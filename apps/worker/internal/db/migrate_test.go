package db

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBackfillNullTextColumnInBatches(t *testing.T) {
	sqldb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer func() {
		_ = sqldb.Close()
	}()

	selectSQL := regexp.QuoteMeta("SELECT id FROM relay_logs WHERE request_headers IS NULL ORDER BY id LIMIT 2")
	updateSQL := regexp.QuoteMeta("UPDATE relay_logs SET request_headers = ? WHERE id IN (?,?)")

	mock.ExpectQuery(selectSQL).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	mock.ExpectExec(updateSQL).
		WithArgs("", 1, 2).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(selectSQL).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := backfillNullTextColumnInBatches(sqldb, "relay_logs", "request_headers", "", 2); err != nil {
		t.Fatalf("backfillNullTextColumnInBatches() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
