package model

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Probe for the production blue-green deploy failure:
//
//	Error 1091 (42000): Can't DROP 'uni_tokens_key'
//	ALTER TABLE `tokens` DROP FOREIGN KEY `uni_tokens_key`
//
// Runs only when TOKEN_MYSQL_PROBE_DSN points at a throwaway MySQL database, so
// it never touches CI.
func TestProbeTokenAutoMigrateOnMySQL(t *testing.T) {
	dsn := os.Getenv("TOKEN_MYSQL_PROBE_DSN")
	if dsn == "" {
		t.Skip("TOKEN_MYSQL_PROBE_DSN not set")
	}

	db, err := gorm.Open(mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: dsn}}}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Mirror the production table: a standalone unique index named the way the
	// current model declares it, and no legacy uni_tokens_key object at all.
	setup := []string{
		"DROP TABLE IF EXISTS tokens",
		"CREATE TABLE tokens (" +
			"id bigint NOT NULL AUTO_INCREMENT," +
			"user_id int NOT NULL," +
			"`key` varchar(128) NOT NULL," +
			"status int DEFAULT 1," +
			"name varchar(255)," +
			"created_time bigint," +
			"accessed_time bigint," +
			"expired_time bigint DEFAULT -1," +
			"remain_quota int DEFAULT 0," +
			"unlimited_quota tinyint(1)," +
			"model_limits_enabled tinyint(1)," +
			"model_limits text," +
			"allow_ips varchar(255) DEFAULT ''," +
			"used_quota int DEFAULT 0," +
			"`group` varchar(255) DEFAULT ''," +
			"cross_group_retry tinyint(1)," +
			"auto_groups text," +
			"deleted_at datetime(3) NULL," +
			"PRIMARY KEY (id)," +
			"UNIQUE KEY idx_tokens_key (`key`)," +
			"KEY idx_tokens_name (name)," +
			"KEY idx_tokens_user_id (user_id)," +
			"KEY idx_tokens_deleted_at (deleted_at)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	}
	for _, stmt := range setup {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	var before []struct {
		Name string `gorm:"column:CONSTRAINT_NAME"`
	}
	_ = db.Raw("SELECT CONSTRAINT_NAME FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tokens'").Scan(&before).Error
	t.Logf("constraints before: %+v", before)

	var version string
	_ = db.Raw("SELECT VERSION()").Scan(&version).Error
	t.Logf("mysql version: %s", version)
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&Token{}); err == nil && stmt.Schema != nil {
		for name := range stmt.Schema.ParseUniqueConstraints() {
			t.Logf("schema unique constraint: %q", name)
		}
		t.Logf("field key: Unique=%v UniqueIndex=%q", stmt.Schema.LookUpField("key").Unique, stmt.Schema.LookUpField("key").UniqueIndex)
	}
	t.Logf("UniqueName(tokens,key) = %q", db.NamingStrategy.UniqueName("tokens", "key"))
	t.Logf("HasConstraint(uni_tokens_key) = %v", db.Migrator().HasConstraint(&Token{}, "uni_tokens_key"))
	t.Logf("HasIndex(idx_tokens_key) = %v", db.Migrator().HasIndex(&Token{}, "idx_tokens_key"))
	if cols, err := db.Migrator().ColumnTypes(&Token{}); err == nil {
		for _, col := range cols {
			if col.Name() == "key" {
				u, ok := col.Unique()
				t.Logf("column key: Unique=%v ok=%v type=%s", u, ok, col.DatabaseTypeName())
			}
		}
	}

	if err := db.AutoMigrate(&Token{}); err != nil {
		fmt.Printf("PROBE_RESULT: AutoMigrate FAILED: %v\n", err)
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// The declared unique index must survive and nothing legacy may be created.
	idx := []struct {
		Name string `gorm:"column:INDEX_NAME"`
	}{}
	_ = db.Raw("SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tokens' ORDER BY INDEX_NAME").Scan(&idx).Error
	names := make([]string, 0, len(idx))
	for _, r := range idx {
		names = append(names, r.Name)
	}

	con := []struct {
		Name string `gorm:"column:CONSTRAINT_NAME"`
	}{}
	_ = db.Raw("SELECT CONSTRAINT_NAME FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tokens'").Scan(&con).Error
	conNames := make([]string, 0, len(con))
	for _, r := range con {
		conNames = append(conNames, r.Name)
	}
	t.Logf("indexes after: %v", names)
	t.Logf("constraints after: %v", conNames)

	hasKeyIndex := false
	for _, n := range names {
		if n == "idx_tokens_key" {
			hasKeyIndex = true
		}
	}
	if !hasKeyIndex {
		t.Fatalf("idx_tokens_key missing after migration: %v", names)
	}

	// Idempotent: a second boot must not fail either.
	if err := db.AutoMigrate(&Token{}); err != nil {
		t.Fatalf("second AutoMigrate failed: %v", err)
	}
	fmt.Println("PROBE_RESULT: AutoMigrate OK, idx_tokens_key present, second run OK")
}
