package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// fork: influencer-preset-publish
//
// An administrator publishes one finished influencer character into the
// inspiration library. The row carries both halves of the character: the sheet
// parameters that rebuild it (tier/selection/brief/seed) and the durable asset
// references, so the library never depends on the upstream render still being
// reachable. Assets live in the shared storage driver under keys this table
// owns; nothing here expires.
type InfluencerPreset struct {
	Id   int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name string `json:"name" gorm:"type:varchar(120);not null"`

	// Character parameters, copied from the source task's persisted sheet meta.
	// Selection is the marshalled JSON object of category -> option ids; text
	// keeps the column portable across SQLite, MySQL and PostgreSQL without a
	// driver.Valuer.
	Tier      string `json:"tier" gorm:"type:varchar(64);not null"`
	Selection string `json:"selection" gorm:"type:text;not null"`
	Brief     string `json:"brief" gorm:"type:text"`
	Seed      int64  `json:"seed"`

	// Card artwork: the character render. The mime type is stored because the
	// asset route streams these bytes back with the type they were written as.
	PreviewKey    string `json:"-" gorm:"type:varchar(255);not null"`
	PreviewMime   string `json:"-" gorm:"type:varchar(64);not null"`
	PreviewWidth  int    `json:"preview_width"`
	PreviewHeight int    `json:"preview_height"`

	// fork: image-thumbnails; optional derived card artwork, never a source image.
	ThumbnailKey         string `json:"-" gorm:"type:varchar(255)"`
	ThumbnailMime        string `json:"-" gorm:"type:varchar(64)"`
	ThumbnailWidth       int    `json:"-"`
	ThumbnailHeight      int    `json:"-"`
	ThumbnailAttempts    int    `json:"-"`
	ThumbnailAttemptedAt int64  `json:"-"`

	// Full character sheet: the turnaround board when one was published,
	// otherwise the same render as the card.
	SheetKey    string `json:"-" gorm:"type:varchar(255);not null"`
	SheetMime   string `json:"-" gorm:"type:varchar(64);not null"`
	SheetWidth  int    `json:"sheet_width"`
	SheetHeight int    `json:"sheet_height"`

	// The history record this came from, so the same render cannot be
	// published twice by accident.
	SourceTaskID string `json:"source_task_id" gorm:"type:varchar(191);index"`
	CreatedBy    int    `json:"created_by" gorm:"index"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
}

func (InfluencerPreset) TableName() string {
	return "influencer_presets"
}

// SelectionMap decodes the stored parameter object.
func (p *InfluencerPreset) SelectionMap() (map[string][]string, error) {
	if p == nil || strings.TrimSpace(p.Selection) == "" {
		return map[string][]string{}, nil
	}
	var selection map[string][]string
	if err := common.Unmarshal([]byte(p.Selection), &selection); err != nil {
		return nil, err
	}
	if selection == nil {
		selection = map[string][]string{}
	}
	return selection, nil
}

// SetSelectionMap stores the parameter object as JSON.
func (p *InfluencerPreset) SetSelectionMap(selection map[string][]string) error {
	if selection == nil {
		selection = map[string][]string{}
	}
	encoded, err := common.Marshal(selection)
	if err != nil {
		return err
	}
	p.Selection = string(encoded)
	return nil
}

// CreateInfluencerPreset inserts one published character.
func CreateInfluencerPreset(preset *InfluencerPreset) error {
	if preset == nil {
		return errors.New("preset cannot be nil")
	}
	if strings.TrimSpace(preset.Name) == "" {
		return errors.New("preset name is required")
	}
	if strings.TrimSpace(preset.Tier) == "" {
		return errors.New("preset tier is required")
	}
	if strings.TrimSpace(preset.PreviewKey) == "" || strings.TrimSpace(preset.SheetKey) == "" {
		return errors.New("preset assets are required")
	}
	if preset.CreatedAt == 0 {
		preset.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(preset).Error
}

// ListInfluencerPresets returns every published character, newest first.
func ListInfluencerPresets() ([]InfluencerPreset, error) {
	var presets []InfluencerPreset
	err := DB.Order("created_at DESC, id DESC").Find(&presets).Error
	return presets, err
}

// GetInfluencerPreset reads one row by primary key.
func GetInfluencerPreset(id int64) (*InfluencerPreset, error) {
	if id <= 0 {
		return nil, errors.New("invalid preset id")
	}
	var preset InfluencerPreset
	if err := DB.Where("id = ?", id).First(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

// GetInfluencerPresetBySourceTask finds the publication of one history record.
// A nil result with a nil error means the record is not published yet.
func GetInfluencerPresetBySourceTask(sourceTaskID string) (*InfluencerPreset, error) {
	taskID := strings.TrimSpace(sourceTaskID)
	if taskID == "" {
		return nil, nil
	}
	var preset InfluencerPreset
	err := DB.Where("source_task_id = ?", taskID).First(&preset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &preset, nil
}

// DeleteInfluencerPreset removes one published character.
func DeleteInfluencerPreset(id int64) error {
	if id <= 0 {
		return errors.New("invalid preset id")
	}
	return DB.Where("id = ?", id).Delete(&InfluencerPreset{}).Error
}
