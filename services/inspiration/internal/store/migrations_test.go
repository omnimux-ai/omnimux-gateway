package store_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/store"
	"omnimux-inspiration/internal/testutil"
)

const migrationsDir = "../../migrations"

func TestMultidimensionalModelFields(t *testing.T) {
	typ := reflect.TypeOf(model.Inspiration{})
	for _, tt := range []struct {
		name, kind, tag string
	}{
		{"CountryCode", "string", "size:8;default:'';not null;index:idx_country,priority:1"},
		{"Category", "string", "size:64;default:'';not null;index:idx_category,priority:1"},
		{"Duration", "int", "type:int unsigned;default:0;not null;index:idx_duration"},
		{"Views", "int64", "type:bigint unsigned;default:0;not null;index:idx_views"},
		{"TrafficType", "string", "size:16;default:organic;not null;index:idx_traffic_type,priority:1"},
		{"PostedAt", "*time.Time", "type:datetime(3);index:idx_posted_at"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := typ.FieldByName(tt.name)
			if !ok {
				t.Fatalf("missing model field %s", tt.name)
			}
			if field.Type.String() != tt.kind {
				t.Errorf("type = %s, want %s", field.Type, tt.kind)
			}
			for _, tag := range strings.Split(tt.tag, ";") {
				if !strings.Contains(field.Tag.Get("gorm"), tag) {
					t.Errorf("gorm tag %q missing %q", field.Tag.Get("gorm"), tag)
				}
			}
		})
	}
}

func TestMigrateMultidimensionalFresh(t *testing.T) {
	db := testutil.OpenDB(t)
	if err := db.Exec("INSERT INTO inspirations(title, source_url_hash) VALUES (?, ?)", "fresh", "fresh-hash").Error; err != nil {
		t.Fatal(err)
	}
	assertDefaults(t, db, "fresh-hash")
	assertFilterIndexes(t, db)
	assertVersionsAndNoOp(t, db)
}

func TestMultidimensionalModelRoundTrip(t *testing.T) {
	db := testutil.OpenDB(t)
	posted := time.Date(2026, 9, 10, 4, 0, 1, 123000000, time.UTC)
	item := model.Inspiration{
		Title: "roundtrip", SourceURLHash: "roundtrip-hash", CountryCode: "US",
		Category: "beauty", Duration: math.MaxUint32, Views: math.MaxInt64,
		TrafficType: "ad", PostedAt: &posted,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	assertVersionsAndNoOp(t, db)
	var got model.Inspiration
	if err := db.First(&got, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.CountryCode != item.CountryCode || got.Category != item.Category || got.Duration != item.Duration || got.Views != item.Views || got.TrafficType != item.TrafficType || got.PostedAt == nil || !got.PostedAt.Equal(posted) {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if got.PostedAt.Location() != time.UTC {
		t.Errorf("posted_at location = %s, want UTC", got.PostedAt.Location())
	}
	if db.Dialect == store.DialectMySQL {
		for column, want := range map[string]string{
			"country_code": "varchar(8)", "category": "varchar(64)",
			"duration": "int unsigned", "views": "bigint unsigned",
			"traffic_type": "varchar(16)", "posted_at": "datetime(3)",
		} {
			var actual string
			if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'inspirations' AND COLUMN_NAME = ?`, column).Scan(&actual).Error; err != nil {
				t.Fatal(err)
			}
			if actual != want {
				t.Errorf("%s type=%s, want %s", column, actual, want)
			}
		}
	}
}

func TestMigrateMultidimensionalLegacy(t *testing.T) {
	db := testutil.OpenDB(t)
	// Reconstruct version 0001 inside this test's disposable database.
	for _, table := range []string{"inspiration_tags", "tags", "inspirations", "schema_migrations"} {
		if err := db.Exec("DROP TABLE " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
	if db.Dialect == store.DialectMySQL {
		dir := t.TempDir()
		init, err := os.ReadFile(filepath.Join(migrationsDir, "0001_init.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "0001_init.sql"), init, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Migrate(dir); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := db.Exec(legacySQLiteSchema).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO inspirations(id, title, content, source_url_hash, hot_score, is_favorite, deleted_at)
		VALUES (42, 'legacy', 'keep this content', 'legacy-hash', 17, 1, '2025-01-02 03:04:05')`).Error; err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{"INSERT INTO tags(id, name) VALUES (7, 'existing')", "INSERT INTO inspiration_tags(inspiration_id, tag_id) VALUES (42, 7)"} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	applied, err := db.Migrate(migrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || !strings.HasPrefix(applied[0], "0002") {
		t.Fatalf("upgrade applied = %v, want only 0002", applied)
	}
	assertDefaults(t, db, "legacy-hash")
	var legacy model.Inspiration
	if err := db.Unscoped().First(&legacy, 42).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.Title != "legacy" || legacy.Content != "keep this content" || legacy.HotScore != 17 || !legacy.IsFavorite || !legacy.DeletedAt.Valid {
		t.Fatalf("legacy data changed: %+v", legacy)
	}
	var count int64
	if err := db.Table("inspiration_tags").Where("inspiration_id = 42 AND tag_id = 7").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("legacy tag association: count=%d err=%v", count, err)
	}
	assertFilterIndexes(t, db)
	assertVersionsAndNoOp(t, db)
}

func TestOpenDBIsolationAndCleanup(t *testing.T) {
	observer := testutil.OpenDB(t)
	var owned *store.DB
	var name string
	t.Run("disposable", func(t *testing.T) {
		owned = testutil.OpenDB(t)
		if err := owned.Exec("INSERT INTO inspirations(source_url_hash) VALUES ('isolated')").Error; err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := observer.Table("inspirations").Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("test databases share data: count=%d err=%v", count, err)
		}
		if owned.Dialect == store.DialectMySQL {
			if err := owned.Raw("SELECT DATABASE()").Scan(&name).Error; err != nil {
				t.Fatal(err)
			}
		}
	})
	if owned == nil {
		t.Fatal("disposable database did not open")
	}
	conn, err := owned.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Ping(); err == nil {
		t.Error("cleanup left test connection open")
	}
	if name != "" {
		var count int64
		if err := observer.Raw("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?", name).Scan(&count).Error; err != nil || count != 0 {
			t.Fatalf("cleanup left owned MySQL database: count=%d err=%v", count, err)
		}
	}
}

func assertDefaults(t *testing.T, db *store.DB, hash string) {
	t.Helper()
	var row struct {
		CountryCode string
		Category    string
		Duration    int
		Views       int64
		TrafficType string
		PostedAt    *time.Time
	}
	if err := db.Raw("SELECT country_code, category, duration, views, traffic_type, posted_at FROM inspirations WHERE source_url_hash = ?", hash).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CountryCode != "" || row.Category != "" || row.Duration != 0 || row.Views != 0 || row.TrafficType != "organic" || row.PostedAt != nil {
		t.Fatalf("unexpected defaults: %+v", row)
	}
	for _, col := range []string{"country_code", "category", "duration", "views", "traffic_type"} {
		if err := db.Exec("UPDATE inspirations SET "+col+" = NULL WHERE source_url_hash = ?", hash).Error; err == nil {
			t.Errorf("%s accepts NULL", col)
		}
	}
}

func assertFilterIndexes(t *testing.T, db *store.DB) {
	t.Helper()
	for index, columns := range map[string][]string{
		"idx_country":      {"country_code", "deleted_at"},
		"idx_category":     {"category", "deleted_at"},
		"idx_duration":     {"duration"},
		"idx_views":        {"views"},
		"idx_traffic_type": {"traffic_type", "deleted_at"},
		"idx_posted_at":    {"posted_at"},
	} {
		var actual []string
		if db.Dialect == store.DialectMySQL {
			if err := db.Raw(`SELECT COLUMN_NAME FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'inspirations' AND INDEX_NAME = ? ORDER BY SEQ_IN_INDEX`, index).Scan(&actual).Error; err != nil {
				t.Fatal(err)
			}
		} else {
			var rows []struct{ Name string }
			if err := db.Raw("PRAGMA index_info(" + index + ")").Scan(&rows).Error; err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				actual = append(actual, row.Name)
			}
		}
		if !reflect.DeepEqual(actual, columns) {
			t.Errorf("%s columns=%v, want %v", index, actual, columns)
		}
	}
}

func assertVersionsAndNoOp(t *testing.T, db *store.DB) {
	t.Helper()
	var before []struct{ Version, AppliedAt string }
	if err := db.Raw("SELECT version, applied_at FROM schema_migrations ORDER BY version").Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || before[0].Version != "0001" || before[1].Version != "0002" {
		t.Fatalf("migration versions = %+v", before)
	}
	for i := 0; i < 2; i++ {
		applied, err := db.Migrate(migrationsDir)
		if err != nil || len(applied) != 0 {
			t.Fatalf("rerun applied=%v err=%v, want no-op", applied, err)
		}
	}
	var after []struct{ Version, AppliedAt string }
	if err := db.Raw("SELECT version, applied_at FROM schema_migrations ORDER BY version").Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("rerun changed migration records: before=%+v after=%+v", before, after)
	}
}

const legacySQLiteSchema = `
CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP);
INSERT INTO schema_migrations(version) VALUES ('0001');
CREATE TABLE inspirations (
 id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL DEFAULT 'link',
 title TEXT NOT NULL DEFAULT '', content TEXT, source_url TEXT NOT NULL DEFAULT '',
 source_url_hash TEXT NOT NULL DEFAULT '', cover_key TEXT NOT NULL DEFAULT '',
 media_keys TEXT, hot_score INTEGER NOT NULL DEFAULT 0, is_favorite INTEGER NOT NULL DEFAULT 0,
 analysis TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME NULL
);
CREATE UNIQUE INDEX uk_source_url_hash ON inspirations(source_url_hash);
CREATE INDEX idx_type_active ON inspirations(type, deleted_at);
CREATE INDEX idx_hot ON inspirations(hot_score);
CREATE INDEX idx_favorite ON inspirations(is_favorite, deleted_at);
CREATE TABLE tags (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE UNIQUE INDEX uk_name ON tags(name);
CREATE TABLE inspiration_tags (inspiration_id INTEGER NOT NULL, tag_id INTEGER NOT NULL, PRIMARY KEY(inspiration_id, tag_id));
CREATE INDEX idx_tag ON inspiration_tags(tag_id, inspiration_id);
`
