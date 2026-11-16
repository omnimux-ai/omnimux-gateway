package store

import (
	"fmt"

	"gorm.io/gorm"
)

func (d *DB) migrateBuiltin() ([]string, error) {
	initial, err := builtinSchema(string(d.Dialect))
	if err != nil {
		return nil, err
	}
	migrations := []struct {
		version, name string
		statements    []string
	}{
		{"0001", "0001_init (builtin)", initial},
		{"0002", "0002_multidimensional_filters (builtin)", multidimensionalSchema(d.Dialect)},
	}
	var applied []string
	for _, migration := range migrations {
		var count int64
		if err := d.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", migration.version).Scan(&count).Error; err != nil {
			return applied, err
		}
		if count > 0 {
			continue
		}
		if err := d.Transaction(func(tx *gorm.DB) error {
			for _, statement := range migration.statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return tx.Exec("INSERT INTO schema_migrations(version) VALUES (?)", migration.version).Error
		}); err != nil {
			return applied, fmt.Errorf("apply %s: %w", migration.name, err)
		}
		applied = append(applied, migration.name)
	}
	return applied, nil
}

func multidimensionalSchema(dialect Dialect) []string {
	duration := "INTEGER NOT NULL DEFAULT 0"
	views := "INTEGER NOT NULL DEFAULT 0"
	postedAt := "DATETIME NULL"
	if dialect == DialectPostgres {
		// PostgreSQL has no unsigned integer types; BIGINT covers uint32 duration.
		duration = "BIGINT NOT NULL DEFAULT 0"
		views = "BIGINT NOT NULL DEFAULT 0"
		postedAt = "TIMESTAMP(3) WITHOUT TIME ZONE NULL"
	}
	return []string{
		"ALTER TABLE inspirations ADD COLUMN country_code VARCHAR(8) NOT NULL DEFAULT ''",
		"ALTER TABLE inspirations ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT ''",
		"ALTER TABLE inspirations ADD COLUMN duration " + duration,
		"ALTER TABLE inspirations ADD COLUMN views " + views,
		"ALTER TABLE inspirations ADD COLUMN traffic_type VARCHAR(16) NOT NULL DEFAULT 'organic'",
		"ALTER TABLE inspirations ADD COLUMN posted_at " + postedAt,
		"CREATE INDEX idx_country ON inspirations(country_code, deleted_at)",
		"CREATE INDEX idx_category ON inspirations(category, deleted_at)",
		"CREATE INDEX idx_duration ON inspirations(duration)",
		"CREATE INDEX idx_views ON inspirations(views)",
		"CREATE INDEX idx_traffic_type ON inspirations(traffic_type, deleted_at)",
		"CREATE INDEX idx_posted_at ON inspirations(posted_at)",
	}
}
