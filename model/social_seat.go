package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// fork: social-seats
const (
	SocialSeatStatusOccupied = "occupied"
	SocialSeatStatusVacant   = "vacant"
	SocialSeatStatusExpired  = "expired"

	SocialSeatWindowDays = 30

	SocialSeatActionOpen   = "social_seat_open"
	SocialSeatActionOccupy = "social_seat_occupy"
	SocialSeatActionVacate = "social_seat_vacate"
	SocialSeatActionRenew  = "social_seat_renew"
	SocialSeatActionExpire = "social_seat_expire"
)

var (
	ErrSocialSeatQuota   = errors.New("social seat quota insufficient")
	ErrSocialSeatCap     = errors.New("social seat cap reached")
	ErrSocialSeatExpired = errors.New("social seat expired")
)

// SocialSeat is a 30-day binding pit. Vacant pits keep expires_at after unbind.
// Truth of expiry / vacancy lives here, not on social_accounts.
type SocialSeat struct {
	ID                int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId            int        `json:"user_id" gorm:"not null;index:idx_social_seats_user_status_exp,priority:1"`
	Status            string     `json:"status" gorm:"type:varchar(32);not null;index:idx_social_seats_user_status_exp,priority:2"`
	ExpiresAt         time.Time  `json:"expires_at" gorm:"index;index:idx_social_seats_user_status_exp,priority:3"`
	OccupiedAccountID *int64     `json:"occupied_account_id"`
	OpenedQuota       int        `json:"opened_quota" gorm:"default:0"`
	GrandfatherUntil  *time.Time `json:"grandfather_until"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (SocialSeat) TableName() string {
	return "social_seats"
}

func SocialSeatWindow() time.Duration {
	return time.Duration(SocialSeatWindowDays) * 24 * time.Hour
}

// SocialUSDToQuota converts a USD seat price to raw quota, rounding up.
// Independent of hosted SKU conversion on purpose.
func SocialUSDToQuota(amountUSD float64) (int, error) {
	if amountUSD < 0 {
		return 0, errors.New("invalid social seat amount")
	}
	if amountUSD == 0 {
		return 0, nil
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("额度单位配置错误")
	}
	d := decimal.NewFromFloat(amountUSD).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Ceil()
	quota, clamp := common.QuotaFromDecimalChecked(d)
	if clamp != nil {
		return 0, clamp
	}
	if quota < 0 {
		return 0, errors.New("invalid social seat amount")
	}
	return quota, nil
}

// ChargeSocialSeatTx locks the user row and subtracts raw quota.
// Mirrors chargeHostedBalanceTx; does not touch hosted_orders.
func ChargeSocialSeatTx(tx *gorm.DB, userId int, quota int) error {
	if tx == nil {
		return errors.New("nil tx")
	}
	if userId <= 0 {
		return errors.New("invalid user")
	}
	if quota < 0 {
		return errors.New("invalid social seat quota")
	}
	var user User
	if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	if quota > 0 && user.Quota < quota {
		return ErrSocialSeatQuota
	}
	if quota > 0 {
		res := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, quota).
			Update("quota", gorm.Expr("quota - ?", quota))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrSocialSeatQuota
		}
	}
	return nil
}

func RecordSocialSeatLog(userId int, action string, seatID int64, quota int, extra string) {
	suffix := ""
	if extra != "" {
		suffix = " " + extra
	}
	RecordLog(userId, LogTypeConsume, common.NewMessage("{{action}} seat_id={{seat_id}} quota={{quota}}{{extra}}", map[string]any{"action": action, "seat_id": seatID, "quota": quota, "extra": suffix}))
}

func CountPurchasedSeatsTx(tx *gorm.DB, userId int, now time.Time) (int, error) {
	var n int64
	err := tx.Model(&SocialSeat{}).
		Where("user_id = ? AND status IN ? AND expires_at > ?", userId,
			[]string{SocialSeatStatusOccupied, SocialSeatStatusVacant}, now).
		Count(&n).Error
	return int(n), err
}

func CountVacantUnexpiredTx(tx *gorm.DB, userId int, now time.Time) (int, error) {
	var n int64
	err := tx.Model(&SocialSeat{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusVacant, now).
		Count(&n).Error
	return int(n), err
}

func CountOccupiedUnexpiredTx(tx *gorm.DB, userId int, now time.Time) (int, error) {
	var n int64
	err := tx.Model(&SocialSeat{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusOccupied, now).
		Count(&n).Error
	return int(n), err
}

func CountPurchasedSeats(userId int, now time.Time) (int, error) {
	return CountPurchasedSeatsTx(DB, userId, now)
}

func CountVacantUnexpired(userId int, now time.Time) (int, error) {
	return CountVacantUnexpiredTx(DB, userId, now)
}

func CountOccupiedUnexpired(userId int, now time.Time) (int, error) {
	return CountOccupiedUnexpiredTx(DB, userId, now)
}

func ListUserSeats(userId int) ([]*SocialSeat, error) {
	var rows []*SocialSeat
	err := DB.Where("user_id = ?", userId).Order("expires_at asc, id asc").Find(&rows).Error
	return rows, err
}

func ListOccupiedExpiredSeats(userId int, now time.Time) ([]*SocialSeat, error) {
	var rows []*SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND expires_at <= ?", userId, SocialSeatStatusOccupied, now).
		Find(&rows).Error
	return rows, err
}

func ListVacantExpiredSeats(userId int, now time.Time) ([]*SocialSeat, error) {
	var rows []*SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND expires_at <= ?", userId, SocialSeatStatusVacant, now).
		Find(&rows).Error
	return rows, err
}

func GetOccupiedSeatForAccount(userId int, accountID int64) (*SocialSeat, error) {
	return GetOccupiedSeatForAccountTx(DB, userId, accountID)
}

func GetOccupiedSeatForAccountTx(tx *gorm.DB, userId int, accountID int64) (*SocialSeat, error) {
	var seat SocialSeat
	err := tx.Where("user_id = ? AND status = ? AND occupied_account_id = ?",
		userId, SocialSeatStatusOccupied, accountID).First(&seat).Error
	if err != nil {
		return nil, err
	}
	return &seat, nil
}

func GetValidOccupiedSeatForAccount(userId int, accountID int64, now time.Time) (*SocialSeat, error) {
	var seat SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND occupied_account_id = ? AND expires_at > ?",
		userId, SocialSeatStatusOccupied, accountID, now).First(&seat).Error
	if err != nil {
		return nil, err
	}
	return &seat, nil
}

// OccupyEarliestVacantTx takes the soonest-expiring unexpired vacant pit.
func OccupyEarliestVacantTx(tx *gorm.DB, userId int, accountID int64, now time.Time) (*SocialSeat, error) {
	var seat SocialSeat
	err := lockForUpdate(tx).
		Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusVacant, now).
		Order("expires_at ASC, id ASC").
		First(&seat).Error
	if err != nil {
		return nil, err
	}
	accID := accountID
	res := tx.Model(&seat).Updates(map[string]any{
		"status":              SocialSeatStatusOccupied,
		"occupied_account_id": accID,
		"updated_at":          now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, errors.New("failed to occupy social seat")
	}
	seat.Status = SocialSeatStatusOccupied
	seat.OccupiedAccountID = &accID
	seat.UpdatedAt = now
	return &seat, nil
}

func OpenOccupiedSeatTx(tx *gorm.DB, userId int, accountID int64, quota int, now time.Time, grandfatherUntil *time.Time) (*SocialSeat, error) {
	accID := accountID
	expires := now.Add(SocialSeatWindow())
	if grandfatherUntil != nil && !grandfatherUntil.IsZero() {
		expires = *grandfatherUntil
	}
	seat := &SocialSeat{
		UserId:            userId,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         expires,
		OccupiedAccountID: &accID,
		OpenedQuota:       quota,
		GrandfatherUntil:  grandfatherUntil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := tx.Create(seat).Error; err != nil {
		return nil, err
	}
	return seat, nil
}

func VacateOccupiedSeatForAccount(userId int, accountID int64) (*SocialSeat, error) {
	now := time.Now().UTC()
	var seat SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND occupied_account_id = ?",
		userId, SocialSeatStatusOccupied, accountID).First(&seat).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	res := DB.Model(&seat).Updates(map[string]any{
		"status":              SocialSeatStatusVacant,
		"occupied_account_id": nil,
		"updated_at":          now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	_ = ClearSocialAccountSeatID(accountID, userId)
	seat.Status = SocialSeatStatusVacant
	seat.OccupiedAccountID = nil
	seat.UpdatedAt = now
	RecordSocialSeatLog(userId, SocialSeatActionVacate, seat.ID, 0, fmt.Sprintf("account_id=%d", accountID))
	return &seat, nil
}

func MarkSocialSeatExpired(seat *SocialSeat, extra string) error {
	if seat == nil {
		return errors.New("nil seat")
	}
	now := time.Now().UTC()
	res := DB.Model(seat).Updates(map[string]any{
		"status":              SocialSeatStatusExpired,
		"occupied_account_id": nil,
		"updated_at":          now,
	})
	if res.Error != nil {
		return res.Error
	}
	if seat.OccupiedAccountID != nil {
		_ = ClearSocialAccountSeatID(*seat.OccupiedAccountID, seat.UserId)
	}
	seat.Status = SocialSeatStatusExpired
	seat.OccupiedAccountID = nil
	RecordSocialSeatLog(seat.UserId, SocialSeatActionExpire, seat.ID, 0, extra)
	return nil
}

func RenewSocialSeat(seat *SocialSeat, quota int, now time.Time) error {
	if seat == nil {
		return errors.New("nil seat")
	}
	next := seat.ExpiresAt.Add(SocialSeatWindow())
	if next.Before(now) {
		next = now.Add(SocialSeatWindow())
	}
	res := DB.Model(seat).Updates(map[string]any{
		"expires_at":   next,
		"opened_quota": quota,
		"updated_at":   now,
	})
	if res.Error != nil {
		return res.Error
	}
	seat.ExpiresAt = next
	seat.OpenedQuota = quota
	RecordSocialSeatLog(seat.UserId, SocialSeatActionRenew, seat.ID, quota, "")
	return nil
}

func BindAccountSeat(accountID int64, userId int, seatID int64) error {
	res := DB.Model(&SocialAccount{}).
		Where("id = ? AND user_id = ?", accountID, userId).
		Updates(map[string]any{
			"seat_id":    seatID,
			"updated_at": time.Now().UTC(),
		})
	return res.Error
}

func ClearSocialAccountSeatID(accountID int64, userId int) error {
	res := DB.Model(&SocialAccount{}).
		Where("id = ? AND user_id = ?", accountID, userId).
		Updates(map[string]any{
			"seat_id":    nil,
			"updated_at": time.Now().UTC(),
		})
	return res.Error
}

func ExpireAllUserSeats(userId int) error {
	now := time.Now().UTC()
	res := DB.Model(&SocialSeat{}).Where("user_id = ? AND status <> ?", userId, SocialSeatStatusExpired).
		Updates(map[string]any{
			"status":              SocialSeatStatusExpired,
			"occupied_account_id": nil,
			"updated_at":          now,
		})
	if res.Error != nil {
		return res.Error
	}
	_ = DB.Model(&SocialAccount{}).Where("user_id = ?", userId).
		Updates(map[string]any{
			"status":     SocialAccountStatusDisconnected,
			"seat_id":    nil,
			"updated_at": now,
		})
	RecordSocialSeatLog(userId, SocialSeatActionExpire, 0, 0, "purge_all")
	return nil
}

// SeatCounts is the wallet / GET /seats snapshot.
type SeatCounts struct {
	Purchased        int
	Occupied         int
	Vacant           int
	NextExpiry       *time.Time
	GrandfatherUntil *time.Time
}

func CountUserSeatState(userId int, now time.Time) (SeatCounts, error) {
	var out SeatCounts
	var err error
	out.Purchased, err = CountPurchasedSeats(userId, now)
	if err != nil {
		return out, err
	}
	out.Occupied, err = CountOccupiedUnexpired(userId, now)
	if err != nil {
		return out, err
	}
	out.Vacant, err = CountVacantUnexpired(userId, now)
	if err != nil {
		return out, err
	}

	var occupied SocialSeat
	err = DB.Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusOccupied, now).
		Order("expires_at ASC").First(&occupied).Error
	if err == nil {
		t := occupied.ExpiresAt
		out.NextExpiry = &t
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	} else {
		var vacant SocialSeat
		err = DB.Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusVacant, now).
			Order("expires_at ASC").First(&vacant).Error
		if err == nil {
			t := vacant.ExpiresAt
			out.NextExpiry = &t
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return out, err
		}
	}

	var gf SocialSeat
	err = DB.Where("user_id = ? AND grandfather_until IS NOT NULL AND grandfather_until > ? AND status IN ?",
		userId, now, []string{SocialSeatStatusOccupied, SocialSeatStatusVacant}).
		Order("grandfather_until DESC").First(&gf).Error
	if err == nil && gf.GrandfatherUntil != nil {
		out.GrandfatherUntil = gf.GrandfatherUntil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	return out, nil
}

// AccountSeatUsable reports whether an active account may publish.
// Valid: occupied unexpired, or occupied expired but still inside grace, or grandfather_until > now.
func AccountSeatUsable(userId int, accountID int64, now time.Time, graceDays int) error {
	seat, err := GetOccupiedSeatForAccount(userId, accountID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSocialSeatExpired
		}
		return err
	}
	if seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now) {
		return nil
	}
	if seat.ExpiresAt.After(now) {
		return nil
	}
	graceEnd := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
	if now.Before(graceEnd) || now.Equal(graceEnd) {
		return ErrSocialSeatExpired
	}
	return ErrSocialSeatExpired
}

func ListActiveAccountsWithoutSeat(userId int) ([]*SocialAccount, error) {
	var rows []*SocialAccount
	err := DB.Where("user_id = ? AND status = ?", userId, SocialAccountStatusActive).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*SocialAccount, 0)
	for _, a := range rows {
		_, err := GetOccupiedSeatForAccount(userId, a.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out = append(out, a)
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func UpsertSocialAccountTx(tx *gorm.DB, a *SocialAccount) error {
	if a == nil || a.UserId <= 0 || a.Provider == "" || a.ProviderAccountID == "" || a.Platform == "" {
		return errors.New("invalid social account")
	}
	if a.Status == "" {
		a.Status = SocialAccountStatusActive
	}
	var existing SocialAccount
	err := tx.Where("provider = ? AND provider_account_id = ?", a.Provider, a.ProviderAccountID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(a).Error; err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if existing.UserId != a.UserId {
		return errors.New("social account already bound to another user")
	}
	updates := map[string]any{
		"provider_profile_id": a.ProviderProfileID,
		"platform":            a.Platform,
		"username":            a.Username,
		"display_name":        a.DisplayName,
		"status":              a.Status,
		"updated_at":          time.Now().UTC(),
	}
	if a.SeatID != nil {
		updates["seat_id"] = a.SeatID
	}
	if err := tx.Model(&existing).Updates(updates).Error; err != nil {
		return err
	}
	a.ID = existing.ID
	return nil
}

func ListOccupiedSeatsByUser(userId int) ([]*SocialSeat, error) {
	var rows []*SocialSeat
	err := DB.Where("user_id = ? AND status = ?", userId, SocialSeatStatusOccupied).Find(&rows).Error
	return rows, err
}
