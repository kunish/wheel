package db

import (
	"regexp"
	"testing"
)

func TestMigrationDDLs_NoTextLikeDefaults(t *testing.T) {
	all := append([]string{}, initSchema...)
	all = append(all, initAlters...)

	textDefault := regexp.MustCompile(`(?i)\b(?:tinytext|text|mediumtext|longtext|json|tinyblob|blob|mediumblob|longblob)\b[^,;)]*\bdefault\b`)

	for _, ddl := range all {
		if match := textDefault.FindString(ddl); match != "" {
			t.Fatalf("ddl contains unsupported default on text-like column: %q\nDDL: %s", match, ddl)
		}
	}
}
