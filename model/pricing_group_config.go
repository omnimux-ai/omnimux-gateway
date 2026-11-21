package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const PricingGroupConfigKey = "ModelPricingGroups"

var (
	ErrPricingGroupConfigConflict = errors.New("model pricing groups changed, reload before saving")
	errPricingGroupConfigKey      = errors.New("ModelPricingGroups requires a versioned single-option update with the exact key")
)

const emptyPricingGroupConfig = `{"version":0,"models":{}}`

// PricingGroupConfig is the versioned display metadata for exact catalog model IDs.
// Version zero represents an option that has not been saved yet.
type PricingGroupConfig struct {
	Version uint64                       `json:"version"`
	Models  map[string]PricingGroupEntry `json:"models"`
}

type PricingGroupEntry struct {
	DefaultGroup string   `json:"default_group"`
	GroupOrder   []string `json:"group_order"`
}

type PricingGroupConfigIssue struct {
	ModelName    string `json:"model_name"`
	DefaultGroup string `json:"default_group,omitempty"`
	GroupOrder   string `json:"group_order,omitempty"`
}

func (entry PricingGroupEntry) HasValidOrder(enabled []string) bool {
	if len(entry.GroupOrder) == 0 || len(entry.GroupOrder) != len(enabled) {
		return false
	}
	remaining := make(map[string]bool, len(enabled))
	for _, group := range enabled {
		remaining[group] = true
	}
	for _, group := range entry.GroupOrder {
		if !remaining[group] {
			return false
		}
		delete(remaining, group)
	}
	return len(remaining) == 0
}

func (entry PricingGroupEntry) catalogIssue(row Pricing) PricingGroupConfigIssue {
	issue := PricingGroupConfigIssue{ModelName: row.ModelName}
	switch {
	case entry.DefaultGroup == "":
		issue.DefaultGroup = "missing"
	case !slices.Contains(row.EnableGroup, entry.DefaultGroup):
		issue.DefaultGroup = "not_enabled"
	}
	switch {
	case len(entry.GroupOrder) == 0:
		issue.GroupOrder = "missing"
	case !entry.HasValidOrder(row.EnableGroup):
		issue.GroupOrder = "group_set_mismatch"
	}
	return issue
}

// CheckPricingGroupConfig checks persisted settings against freshly read catalog
// inputs, before caller permissions or defensive fallbacks.
func CheckPricingGroupConfig(ctx context.Context) (PricingGroupConfig, []PricingGroupConfigIssue, error) {
	config, err := LoadPricingGroupConfig(ctx)
	if err != nil {
		return PricingGroupConfig{}, nil, err
	}
	rows, err := readCurrentCatalogPricing(ctx)
	if err != nil {
		return PricingGroupConfig{}, nil, err
	}
	issues := make([]PricingGroupConfigIssue, 0)
	for _, row := range rows {
		issue := config.Models[row.ModelName].catalogIssue(row)
		if issue.DefaultGroup != "" || issue.GroupOrder != "" {
			issues = append(issues, issue)
		}
	}
	slices.SortFunc(issues, func(a, b PricingGroupConfigIssue) int {
		return strings.Compare(a.ModelName, b.ModelName)
	})
	return config, issues, nil
}

// LoadPricingGroupConfig reads the primary database on each request so a
// committed edit is visible even on instances whose option sync has yet to run.
func LoadPricingGroupConfig(ctx context.Context) (PricingGroupConfig, error) {
	return readPricingGroupConfig(DB.WithContext(ctx))
}

func readPricingGroupConfig(db *gorm.DB) (PricingGroupConfig, error) {
	var option Option
	result := db.Where(&Option{Key: PricingGroupConfigKey}).Find(&option)
	if result.Error != nil {
		return PricingGroupConfig{}, result.Error
	}
	if result.RowsAffected == 0 {
		return PricingGroupConfig{Models: map[string]PricingGroupEntry{}}, nil
	}
	if option.Key != PricingGroupConfigKey {
		return PricingGroupConfig{}, errPricingGroupConfigKey
	}
	return decodePricingGroupConfig(option.Value)
}

// rejectGenericPricingGroupConfigWrites guards both existing and first writes.
// The MySQL expression inherits the actual key column's collation even when
// the subquery has no rows. It therefore detects aliases before any insert.
func rejectGenericPricingGroupConfigWrites(keys []string) error {
	if slices.Contains(keys, PricingGroupConfigKey) {
		return errPricingGroupConfigKey
	}
	if len(keys) == 0 || !common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return nil
	}
	var matches bool
	query := "SELECT COALESCE((SELECT " + commonKeyCol + " FROM options WHERE 1 = 0), ?) IN ?"
	if err := DB.Raw(query, PricingGroupConfigKey, keys).Scan(&matches).Error; err != nil {
		return err
	}
	if matches {
		return errPricingGroupConfigKey
	}
	return nil
}

func decodePricingGroupConfig(value string) (PricingGroupConfig, error) {
	var document struct {
		Version *uint64                      `json:"version"`
		Models  map[string]PricingGroupEntry `json:"models"`
	}
	if err := common.UnmarshalJsonStr(value, &document); err != nil || document.Version == nil || document.Models == nil {
		return PricingGroupConfig{}, errors.New("ModelPricingGroups must be a JSON object with a version and a models map")
	}
	return PricingGroupConfig{Version: *document.Version, Models: document.Models}, nil
}

// savePricingGroupConfig replaces the document only at the expected version.
// Insert the initial row in the same transaction to also serialize first saves.
func savePricingGroupConfig(value string) error {
	config, err := decodePricingGroupConfig(value)
	if err != nil {
		return err
	}
	rows, err := readCurrentCatalogPricing(context.Background())
	if err != nil {
		return err
	}
	catalog := make(map[string]Pricing, len(rows))
	for _, row := range rows {
		catalog[row.ModelName] = row
	}
	for name, entry := range config.Models {
		row, exists := catalog[name]
		if !exists {
			return fmt.Errorf("model %q is not in the pricing catalog", name)
		}
		issue := entry.catalogIssue(row)
		if issue.DefaultGroup != "" {
			return fmt.Errorf("default_group for %q must be one of its enable_groups", name)
		}
		if issue.GroupOrder != "" {
			return fmt.Errorf("group_order for %q must list each enable_groups entry once", name)
		}
	}
	if config.Version == math.MaxUint64 {
		return errors.New("ModelPricingGroups version is out of range")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		initial := Option{Key: PricingGroupConfigKey, Value: emptyPricingGroupConfig}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
			return err
		}
		current, err := readPricingGroupConfig(lockForUpdate(tx))
		if err != nil {
			return err
		}
		if current.Version != config.Version {
			return ErrPricingGroupConfigConflict
		}
		config.Version++
		encoded, err := common.Marshal(config)
		if err != nil {
			return err
		}
		return tx.Model(&Option{}).Where(&Option{Key: PricingGroupConfigKey}).Update("value", string(encoded)).Error
	})
}
