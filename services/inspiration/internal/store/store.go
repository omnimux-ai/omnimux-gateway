// Package store 数据库连接、方言探测与 SQL 迁移执行器。
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"omnimux-inspiration/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Dialect 探测的数据库方言。
type Dialect string

const (
	DialectMySQL    Dialect = "mysql"
	DialectPostgres Dialect = "postgres"
	DialectSQLite   Dialect = "sqlite"
)

// Probe 解析 DSN 得到方言与归一化连接串。
func Probe(rawDSN string) (Dialect, string, error) {
	dsn := strings.TrimSpace(rawDSN)
	switch {
	case dsn == "":
		return "", "", fmt.Errorf("empty DSN")
	case strings.HasPrefix(dsn, "mysql://"):
		return DialectMySQL, strings.TrimPrefix(dsn, "mysql://"), nil
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return DialectPostgres, dsn, nil
	case strings.Contains(dsn, "tcp("): // 网关同款裸 DSN user:pass@tcp(host:3306)/db
		return DialectMySQL, dsn, nil
	case strings.HasPrefix(dsn, "file:"), strings.HasSuffix(dsn, ".db"), strings.HasSuffix(dsn, ".sqlite"), strings.HasSuffix(dsn, ".sqlite3"):
		return DialectSQLite, dsn, nil
	default:
		// 兜底：当作 sqlite 文件路径（本地零依赖开发）
		return DialectSQLite, dsn, nil
	}
}

// Open 建立连接。dialect 为空时自动探测。
func Open(rawDSN string) (*DB, error) {
	dialect, dsn, err := Probe(rawDSN)
	if err != nil {
		return nil, err
	}
	var g *gorm.DB
	switch dialect {
	case DialectMySQL:
		g, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case DialectPostgres:
		g, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		g, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dialect, err)
	}
	if dialect == DialectMySQL || dialect == DialectPostgres {
		if err := g.Callback().Query().Before("gorm:query").Register("inspiration:posted_at_utc", model.ReadPostedAtUTC); err != nil {
			return nil, err
		}
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	if dialect == DialectSQLite {
		sqlDB.SetMaxOpenConns(1) // SQLite 写串行，避免 database is locked
	} else {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(10)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	return &DB{DB: g, Dialect: dialect}, nil
}

// DB 包装 gorm.DB 与方言。
type DB struct {
	*gorm.DB
	Dialect Dialect
}

// Ping 探活。
func (d *DB) Ping(ctx context.Context) error {
	d2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return d.DB.WithContext(d2).Exec("SELECT 1").Error
}

var migFileRe = regexp.MustCompile(`^\d{4}.*\.sql$`)

// Migrate 按文件名序执行 migrations 目录下未应用的迁移。
// MySQL 方言执行 SQL 文件；SQLite/PG 执行内建的版本化迁移。
func (d *DB) Migrate(dir string) (applied []string, err error) {
	if err := d.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	if d.Dialect == DialectMySQL {
		return d.migrateMySQL(dir)
	}
	return d.migrateBuiltin()
}

func (d *DB) ensureMigrationsTable() error {
	switch d.Dialect {
	case DialectPostgres:
		return d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ DEFAULT now())`).Error
	case DialectSQLite:
		return d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`).Error
	default:
		return d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(32) PRIMARY KEY, applied_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3))`).Error
	}
}

func (d *DB) migrateMySQL(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, f := range files {
		base := filepath.Base(f)
		if base == "seed.sql" {
			continue
		}
		if migFileRe.MatchString(base) {
			names = append(names, f)
		}
	}
	sort.Strings(names)
	applied := []string{}
	for _, f := range names {
		version := strings.SplitN(filepath.Base(f), "_", 2)[0]
		var n int64
		if err := d.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&n).Error; err != nil {
			return applied, err
		}
		if n > 0 {
			continue
		}
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return applied, err
		}
		if err := d.execSQLScript(string(sqlBytes)); err != nil {
			return applied, fmt.Errorf("apply %s: %w", filepath.Base(f), err)
		}
		if err := d.Exec("INSERT INTO schema_migrations(version) VALUES (?)", version).Error; err != nil {
			return applied, err
		}
		applied = append(applied, filepath.Base(f))
	}
	return applied, nil
}

// execSQLScript 按语句拆分执行（Go MySQL driver 默认不允许 multiStatements）。
func (d *DB) execSQLScript(script string) error {
	for _, stmt := range splitSQL(script) {
		if err := d.Exec(stmt).Error; err != nil {
			return fmt.Errorf("%w\nstmt: %s", err, truncate(stmt, 120))
		}
	}
	return nil
}

func splitSQL(script string) []string {
	lines := strings.Split(script, "\n")
	var cleaned []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		cleaned = append(cleaned, ln)
	}
	joined := strings.Join(cleaned, "\n")
	parts := strings.Split(joined, ";")
	out := []string{}
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SeedMySQL 执行 seed.sql（仅 MySQL 本地/CI）。
func (d *DB) SeedMySQL(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return d.execSQLScript(string(b))
}
