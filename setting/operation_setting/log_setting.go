package operation_setting

import (
	"math"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const LogMaxRangeDaysOptionKey = "log_setting.max_range_days"
const maxLogRangeDays = math.MaxInt64 / (24 * 60 * 60)

// LogSetting limits how far regular users can reach into usage logs.
type LogSetting struct {
	// UserExportEnabled lets regular users export their own usage logs.
	// Administrators can always export.
	UserExportEnabled bool `json:"user_export_enabled"`
	// MaxRangeDays is the longest time range a user can search in their logs
	// and dashboard, and the longest range any export can cover.
	MaxRangeDays int `json:"max_range_days"`
}

var logSetting = LogSetting{
	UserExportEnabled: true,
	MaxRangeDays:      365,
}

func init() {
	config.GlobalConfig.Register("log_setting", &logSetting)
}

func GetLogSetting() *LogSetting {
	return &logSetting
}

func (s *LogSetting) MaxRangeSeconds() int64 {
	return min(max(int64(s.MaxRangeDays), 1), int64(maxLogRangeDays)) * 24 * 60 * 60
}

func ValidateLogMaxRangeDays(value string) error {
	days, err := strconv.Atoi(value)
	if err != nil || days < 1 {
		return common.NewMessage("The longest log time range must be at least 1 day")
	}
	if int64(days) > maxLogRangeDays {
		return common.NewMessage("The time range cannot exceed {{days}} days", map[string]any{"days": maxLogRangeDays})
	}
	return nil
}
