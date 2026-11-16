package store

// builtinSchema 返回 SQLite / PostgreSQL 的幂等建表语句。
// 与 migrations/0001_init.sql（MySQL 方言）字段对齐；JSON 一律存 TEXT（SQLite）/JSONB（PG）。
func builtinSchema(dialect string) ([]string, error) {
	if dialect == string(DialectPostgres) {
		return []string{`
CREATE TABLE IF NOT EXISTS inspirations (
  id BIGSERIAL PRIMARY KEY,
  type VARCHAR(16) NOT NULL DEFAULT 'link',
  title VARCHAR(512) NOT NULL DEFAULT '',
  content TEXT,
  source_url VARCHAR(1024) NOT NULL DEFAULT '',
  source_url_hash VARCHAR(64) NOT NULL DEFAULT '',
  cover_key VARCHAR(255) NOT NULL DEFAULT '',
  media_keys JSONB,
  hot_score INT NOT NULL DEFAULT 0,
  is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
  analysis JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_source_url_hash ON inspirations(source_url_hash);
CREATE INDEX IF NOT EXISTS idx_type_active ON inspirations(type, deleted_at);
CREATE INDEX IF NOT EXISTS idx_hot ON inspirations(hot_score);
CREATE INDEX IF NOT EXISTS idx_favorite ON inspirations(is_favorite, deleted_at);
`, `
CREATE TABLE IF NOT EXISTS tags (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_name ON tags(name);
`, `
CREATE TABLE IF NOT EXISTS inspiration_tags (
  inspiration_id BIGINT NOT NULL,
  tag_id BIGINT NOT NULL,
  PRIMARY KEY (inspiration_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_tag ON inspiration_tags(tag_id, inspiration_id);
`}, nil
	}
	// SQLite
	return []string{`
CREATE TABLE IF NOT EXISTS inspirations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL DEFAULT 'link',
  title TEXT NOT NULL DEFAULT '',
  content TEXT,
  source_url TEXT NOT NULL DEFAULT '',
  source_url_hash TEXT NOT NULL DEFAULT '',
  cover_key TEXT NOT NULL DEFAULT '',
  media_keys TEXT,
  hot_score INTEGER NOT NULL DEFAULT 0,
  is_favorite INTEGER NOT NULL DEFAULT 0,
  analysis TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_source_url_hash ON inspirations(source_url_hash);
CREATE INDEX IF NOT EXISTS idx_type_active ON inspirations(type, deleted_at);
CREATE INDEX IF NOT EXISTS idx_hot ON inspirations(hot_score);
CREATE INDEX IF NOT EXISTS idx_favorite ON inspirations(is_favorite, deleted_at);
`, `
CREATE TABLE IF NOT EXISTS tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_name ON tags(name);
`, `
CREATE TABLE IF NOT EXISTS inspiration_tags (
  inspiration_id INTEGER NOT NULL,
  tag_id INTEGER NOT NULL,
  PRIMARY KEY (inspiration_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_tag ON inspiration_tags(tag_id, inspiration_id);
`}, nil
}
