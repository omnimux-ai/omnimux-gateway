package operation_setting

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// SocialOpsSetting is Option-backed Social Ops seat billing. Runtime reads
// this pointer so admin changes take effect without restart.
//
// fork: social-seats
type SocialOpsSetting struct {
	BillingMode     string  `json:"billing_mode"`      // off | seat （fixed 视为 off）
	SeatPriceUSD    float64 `json:"seat_price_usd"`    // 默认 $8 / 30 天
	SeatsCap        int     `json:"seats_cap"`         // 每用户未过期已购硬顶（含空闲）
	GrandfatherDays int     `json:"grandfather_days"`  // 存量无席账号宽限天数
	GraceDays       int     `json:"grace_days"`        // 占用席欠费冻帖天数
}

var socialOpsSetting = SocialOpsSetting{
	BillingMode:     "off",
	SeatPriceUSD:    8,
	SeatsCap:        20,
	GrandfatherDays: 7,
	GraceDays:       3,
}

func init() {
	config.GlobalConfig.Register("social_ops_setting", &socialOpsSetting)
}

func GetSocialOpsSetting() *SocialOpsSetting {
	return &socialOpsSetting
}

// EffectiveSocialSeatBillingMode returns "seat" only when Option is seat.
// "fixed" and anything else collapse to "off".
func EffectiveSocialSeatBillingMode() string {
	mode := strings.ToLower(strings.TrimSpace(socialOpsSetting.BillingMode))
	if mode == "seat" {
		return "seat"
	}
	return "off"
}

func SocialSeatBillingEnabled() bool {
	return EffectiveSocialSeatBillingMode() == "seat"
}

func SocialSeatPriceUSD() float64 {
	usd := socialOpsSetting.SeatPriceUSD
	if usd <= 0 {
		return 8
	}
	return usd
}

// SocialSeatPriceCredits is the display unit: 1 USD = 10 积分.
func SocialSeatPriceCredits() int {
	return int(math.Round(SocialSeatPriceUSD() * 10))
}

func SocialSeatsCap() int {
	cap := socialOpsSetting.SeatsCap
	if cap <= 0 {
		return 20
	}
	return cap
}

func SocialSeatGrandfatherDays() int {
	d := socialOpsSetting.GrandfatherDays
	if d < 0 {
		return 0
	}
	if d == 0 {
		return 7
	}
	return d
}

func SocialSeatGraceDays() int {
	d := socialOpsSetting.GraceDays
	if d < 0 {
		return 0
	}
	if d == 0 {
		return 3
	}
	return d
}
