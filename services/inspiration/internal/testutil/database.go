// Package testutil provides isolated databases for integration tests.
package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mysql "github.com/go-sql-driver/mysql"

	"omnimux-inspiration/internal/store"
)

// OpenDB returns a migrated, per-test database. SQLite is the default; setting
// INSPIRATION_TEST_MYSQL_DSN selects MySQL and requires CREATE/DROP DATABASE rights.
// The supplied database is never used: only a newly created database is dropped.
func OpenDB(t *testing.T) *store.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	if raw := os.Getenv("INSPIRATION_TEST_MYSQL_DSN"); raw != "" {
		cfg, err := mysql.ParseDSN(strings.TrimPrefix(raw, "mysql://"))
		if err != nil {
			t.Fatal("parse test MySQL DSN:", err)
		}
		cfg.DBName = ""
		cfg.ParseTime = true
		admin, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal("open test MySQL admin:", err)
		}
		t.Cleanup(func() { _ = admin.Close() })
		var suffix [12]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			t.Fatal("generate test database name:", err)
		}
		name := "inspiration_test_" + hex.EncodeToString(suffix[:])
		if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
			t.Fatal("create test MySQL database:", err)
		}
		t.Cleanup(func() {
			if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
				t.Error("drop owned test MySQL database:", err)
			}
		})
		cfg.DBName = name
		dsn = "mysql://" + cfg.FormatDSN()
	}
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatal("open test database:", err)
	}
	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatal("get test connection:", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migrations")
	}
	if _, err := db.Migrate(filepath.Join(filepath.Dir(source), "..", "..", "migrations")); err != nil {
		t.Fatal("migrate test database:", err)
	}
	return db
}
