// Package model 定义 GORM 模型，与版本化 SQL migrations 对齐。
package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Inspiration struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Type          string         `gorm:"size:16;default:link" json:"type"` // video|image|link
	Title         string         `gorm:"size:512" json:"title"`
	Content       string         `gorm:"type:text" json:"content"`
	SourceURL     string         `gorm:"column:source_url;size:1024" json:"source_url"`
	SourceURLHash string         `gorm:"column:source_url_hash;size:64;uniqueIndex:uk_source_url_hash" json:"-"`
	CoverKey      string         `gorm:"column:cover_key;size:255" json:"cover_key"`
	MediaKeys     datatypes.JSON `gorm:"type:json" json:"media_keys"`
	HotScore      int            `gorm:"default:0" json:"hot_score"`
	CountryCode   string         `gorm:"size:8;default:'';not null;index:idx_country,priority:1" json:"country_code"`
	Category      string         `gorm:"size:64;default:'';not null;index:idx_category,priority:1" json:"category"`
	Duration      int            `gorm:"type:int unsigned;default:0;not null;index:idx_duration" json:"duration"`
	Views         int64          `gorm:"type:bigint unsigned;default:0;not null;index:idx_views" json:"views"`
	TrafficType   string         `gorm:"size:16;default:organic;not null;index:idx_traffic_type,priority:1" json:"traffic_type"`
	PostedAt      *time.Time     `gorm:"type:datetime(3);index:idx_posted_at;serializer:posted_at_utc" json:"posted_at"`
	IsFavorite    bool           `gorm:"default:false" json:"is_favorite"`
	Analysis      datatypes.JSON `gorm:"type:json" json:"analysis"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index;index:idx_country,priority:2;index:idx_category,priority:2;index:idx_traffic_type,priority:2" json:"-"`

	// 非库字段：查询后填充
	Tags []string `gorm:"-" json:"tags,omitempty"`
}

func (Inspiration) TableName() string { return "inspirations" }

type Tag struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:128;uniqueIndex:uk_name" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	// Count 列表 JOIN 聚合填充；创建时不写库（-> 只读）。
	Count int64 `gorm:"column:count;->" json:"count"`
}

func (Tag) TableName() string { return "tags" }

type InspirationTag struct {
	InspirationID uint64 `gorm:"primaryKey"`
	TagID         uint64 `gorm:"primaryKey;index:idx_tag"`
}

func (InspirationTag) TableName() string { return "inspiration_tags" }
