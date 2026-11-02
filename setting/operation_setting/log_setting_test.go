package operation_setting

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogRangeSettingCannotOverflowSeconds(t *testing.T) {
	for _, days := range []int{0, -1, 365, 400, math.MaxInt} {
		seconds := (&LogSetting{MaxRangeDays: days}).MaxRangeSeconds()
		require.Positive(t, seconds)
		require.LessOrEqual(t, seconds, int64(math.MaxInt64))
		if days == 365 || days == 400 {
			assert.Equal(t, int64(days)*86400, seconds)
		}
	}
	require.NoError(t, ValidateLogMaxRangeDays("400"), "the default is not a fixed retention ceiling")
	require.NoError(t, ValidateLogMaxRangeDays(strconv.FormatInt(maxLogRangeDays, 10)))
	require.Error(t, ValidateLogMaxRangeDays(strconv.FormatInt(maxLogRangeDays+1, 10)))
	require.Error(t, ValidateLogMaxRangeDays(strconv.FormatInt(math.MaxInt64, 10)))
	require.Error(t, ValidateLogMaxRangeDays("-1"))
}
