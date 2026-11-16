package model

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostedAtReadProjection(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		dialect          gorm.Dialector
	}{
		{"mysql", "DATE_FORMAT(", mysql.New(mysql.Config{DSN: "user:pass@tcp(127.0.0.1:3306)/unused", SkipInitializeWithVersion: true})},
		{"postgres", "TO_CHAR(", postgres.Open("postgres://user:pass@127.0.0.1/unused?TimeZone=America%2FNew_York")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(tc.dialect, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Callback().Query().Before("gorm:query").Register("test:posted_at_utc", ReadPostedAtUTC); err != nil {
				t.Fatal(err)
			}
			stmt := db.Find(&[]Inspiration{}).Statement
			if !strings.Contains(stmt.SQL.String(), tc.expression) {
				t.Fatalf("missing raw UTC projection: %s", stmt.SQL.String())
			}
			var count int64
			sql := db.Model(&Inspiration{}).Count(&count).Statement.SQL.String()
			if strings.Contains(sql, tc.expression) || !strings.Contains(sql, "count(*)") {
				t.Fatalf("count changed: %s", sql)
			}
			sql = db.Find(&[]Tag{}).Statement.SQL.String()
			if strings.Contains(sql, tc.expression) {
				t.Fatalf("unrelated model changed: %s", sql)
			}
		})
	}
}
