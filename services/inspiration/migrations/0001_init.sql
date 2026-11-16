-- 0001_init.sql — 初始 schema
-- MySQL 方言。SQLite/PostgreSQL 环境由代码内建的方言模板创建等价结构。

CREATE TABLE IF NOT EXISTS inspirations (
  id            BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  type          VARCHAR(16)  NOT NULL DEFAULT 'link',
  title         VARCHAR(512) NOT NULL DEFAULT '',
  content       TEXT,
  source_url    VARCHAR(1024) NOT NULL DEFAULT '',
  source_url_hash CHAR(64)   NOT NULL DEFAULT '',
  cover_key     VARCHAR(255) NOT NULL DEFAULT '',
  media_keys    JSON,
  hot_score     INT          NOT NULL DEFAULT 0,
  is_favorite   TINYINT(1)   NOT NULL DEFAULT 0,
  analysis      JSON,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at    DATETIME(3)  NULL,
  UNIQUE KEY uk_source_url_hash (source_url_hash),
  KEY idx_type_active (type, deleted_at),
  KEY idx_hot (hot_score),
  KEY idx_favorite (is_favorite, deleted_at),
  FULLTEXT KEY ft_title_content (title, content) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tags (
  id   BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS inspiration_tags (
  inspiration_id BIGINT UNSIGNED NOT NULL,
  tag_id         BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (inspiration_id, tag_id),
  KEY idx_tag (tag_id, inspiration_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
