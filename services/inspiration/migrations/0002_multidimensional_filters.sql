-- 0002_multidimensional_filters.sql — up-only MySQL migration.
ALTER TABLE inspirations
  ADD COLUMN country_code VARCHAR(8) NOT NULL DEFAULT '',
  ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN duration INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN views BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN traffic_type VARCHAR(16) NOT NULL DEFAULT 'organic',
  ADD COLUMN posted_at DATETIME(3) NULL,
  ADD INDEX idx_country (country_code, deleted_at),
  ADD INDEX idx_category (category, deleted_at),
  ADD INDEX idx_duration (duration),
  ADD INDEX idx_views (views),
  ADD INDEX idx_traffic_type (traffic_type, deleted_at),
  ADD INDEX idx_posted_at (posted_at);
