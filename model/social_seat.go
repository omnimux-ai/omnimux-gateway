package model

import (
	"errors"
	"fmt"
	"math"
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

	SocialSeatWindowDays        = 30
	SocialSeatMaxValidityDays   = 90
	SocialSeatMaxSwapsPerWindow = 2

	SocialSeatActionOpen     = "social_seat_open"
	SocialSeatActionOccupy   = "social_seat_occupy"
	SocialSeatActionVacate   = "social_seat_vacate"
	SocialSeatActionRenew    = "social_seat_renew"
	SocialSeatActionActivate = "social_seat_activate"
	SocialSeatActionExpire   = "social_seat_expire"
)

var (
	ErrSocialSeatQuota      = errors.New("social seat quota insufficient")
	ErrSocialSeatCap        = errors.New("social seat cap reached")
	ErrSocialSeatExpired    = errors.New("social seat expired")
	ErrSocialSeatRenewLimit = errors.New("social seat max validity limit reached")
	ErrSocialSeatSwapLimit  = errors.New("social seat cross-account swap limit reached")
)

// SocialSeat is a 30-day binding pit. Vacant pits keep expires_at after unbind.
// Truth of expiry / vacancy lives here, not on social_accounts.
type SocialSeat struct {
	ID                int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId            int        `json:"user_id" gorm:"not null;index:idx_social_seats_user_status_exp,priority:1"`
	Status            string     `json:"status" gorm:"type:varchar(32);not null;index:idx_social_seats_user_status_exp,priority:2"`
	ExpiresAt         time.Time  `json:"expires_at" gorm:"index;index:idx_social_seats_user_status_exp,priority:3"`
	OccupiedAccountID *int64     `json:"occupied_account_id"`
	LastAccountID     *int64     `json:"last_account_id" gorm:"index"`
	SwapCount         int        `json:"swap_count" gorm:"not null;default:0"`
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
	if math.IsNaN(amountUSD) || math.IsInf(amountUSD, 0) || amountUSD < 0 {
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

// SyncSocialSeatDebitCache decrements the Redis user quota cache after a committed
// seat charge transaction so cached balances stay aligned with DB writes.
func SyncSocialSeatDebitCache(userId int, quota int, operation string) {
	if userId <= 0 || quota <= 0 {
		return
	}
	if err := cacheDecrUserQuota(userId, int64(quota)); err != nil {
		common.SysLog(fmt.Sprintf("failed to decrease user quota cache after %s: %s", operation, err.Error()))
	}
}

// LockSocialSeatUserTx locks the user row inside a seat mutation transaction to
// serialize concurrent cap checks and quota debits for the same user.
func LockSocialSeatUserTx(tx *gorm.DB, userId int) error {
	if tx == nil {
		return errors.New("nil tx")
	}
	if userId <= 0 {
		return errors.New("invalid user")
	}
	var user User
	return lockForUpdate(tx).Where("id = ?", userId).First(&user).Error
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
	err := lockForUpdate(tx).Where("user_id = ? AND status = ? AND occupied_account_id = ?",
		userId, SocialSeatStatusOccupied, accountID).First(&seat).Error
	if err != nil {
		return nil, err
	}
	return &seat, nil
}

// OccupyVacantForAccountTx assigns an unexpired vacant pit to accountID.
// Same-account rebinds match last_account_id first without incrementing swap_count.
// Cross-account swaps pick the soonest-expiring vacant pit with swap_count < maxSwaps
// (or an unbound legacy pit with last_account_id IS NULL) and increment swap_count.
func OccupyVacantForAccountTx(tx *gorm.DB, userId int, accountID int64, maxSwaps int, now time.Time) (*SocialSeat, error) {
	var seat SocialSeat
	err := lockForUpdate(tx).
		Where("user_id = ? AND status = ? AND expires_at > ? AND last_account_id = ?",
			userId, SocialSeatStatusVacant, now, accountID).
		Order("expires_at ASC, id ASC").
		First(&seat).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = lockForUpdate(tx).
			Where("user_id = ? AND status = ? AND expires_at > ? AND (last_account_id IS NULL OR swap_count < ?)",
				userId, SocialSeatStatusVacant, now, maxSwaps).
			Order("expires_at ASC, id ASC").
			First(&seat).Error
		if err != nil {
			return nil, err
		}
	}
	newSwapCount := seat.SwapCount
	if seat.LastAccountID != nil && *seat.LastAccountID != accountID {
		newSwapCount++
	}
	accID := accountID
	lastID := accountID
	res := tx.Model(&seat).Updates(map[string]any{
		"status":              SocialSeatStatusOccupied,
		"occupied_account_id": accID,
		"last_account_id":     lastID,
		"swap_count":          newSwapCount,
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
	seat.LastAccountID = &lastID
	seat.SwapCount = newSwapCount
	seat.UpdatedAt = now
	return &seat, nil
}

func OpenOccupiedSeatTx(tx *gorm.DB, userId int, accountID int64, quota int, now time.Time, grandfatherUntil *time.Time) (*SocialSeat, error) {
	accID := accountID
	lastID := accountID
	expires := now.Add(SocialSeatWindow())
	if grandfatherUntil != nil && !grandfatherUntil.IsZero() {
		expires = *grandfatherUntil
	}
	seat := &SocialSeat{
		UserId:            userId,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         expires,
		OccupiedAccountID: &accID,
		LastAccountID:     &lastID,
		SwapCount:         0,
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

// OpenFrozenUnactivatedSeatTx materializes a zero-quota frozen seat for a legacy
// active account when seat billing turns on with grandfather_days=0.
// expires_at=now begins the exact grace countdown without counting toward seats_cap.
func OpenFrozenUnactivatedSeatTx(tx *gorm.DB, userId int, accountID int64, now time.Time) (*SocialSeat, error) {
	accID := accountID
	lastID := accountID
	seat := &SocialSeat{
		UserId:            userId,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         now,
		OccupiedAccountID: &accID,
		LastAccountID:     &lastID,
		SwapCount:         0,
		OpenedQuota:       0,
		GrandfatherUntil:  nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := tx.Create(seat).Error; err != nil {
		return nil, err
	}
	return seat, nil
}

// ExpireSocialSeatTx marks a seat expired inside an active transaction without
// emitting a consume log (caller logs after commit when needed).
func ExpireSocialSeatTx(tx *gorm.DB, seatID int64, userId int, now time.Time) error {
	return tx.Model(&SocialSeat{}).
		Where("id = ? AND user_id = ?", seatID, userId).
		Updates(map[string]any{
			"status":              SocialSeatStatusExpired,
			"occupied_account_id": nil,
			"updated_at":          now,
		}).Error
}

func VacateOccupiedSeatForAccount(userId int, accountID int64) (*SocialSeat, error) {
	now := time.Now().UTC()
	var updated *SocialSeat
	var action string
	var extra string
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := LockSocialSeatUserTx(tx, userId); err != nil {
			return err
		}
		var seat SocialSeat
		err := lockForUpdate(tx).Where("user_id = ? AND status = ? AND occupied_account_id = ?",
			userId, SocialSeatStatusOccupied, accountID).First(&seat).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		unexpired := seat.ExpiresAt.After(now) || (seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now))
		if !unexpired {
			if err := ExpireSocialSeatTx(tx, seat.ID, userId, now); err != nil {
				return err
			}
			if err := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
				Updates(map[string]any{"seat_id": nil, "updated_at": now}).Error; err != nil {
				return err
			}
			seat.Status = SocialSeatStatusExpired
			seat.OccupiedAccountID = nil
			seat.UpdatedAt = now
			updated = &seat
			action = SocialSeatActionExpire
			extra = fmt.Sprintf("unbind_expired account_id=%d", accountID)
			return nil
		}
		lastID := accountID
		if seat.LastAccountID != nil {
			lastID = *seat.LastAccountID
		}
		res := tx.Model(&seat).Updates(map[string]any{
			"status":              SocialSeatStatusVacant,
			"occupied_account_id": nil,
			"last_account_id":     lastID,
			"updated_at":          now,
		})
		if res.Error != nil {
			return res.Error
		}
		if err := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
			Updates(map[string]any{"seat_id": nil, "updated_at": now}).Error; err != nil {
			return err
		}
		seat.Status = SocialSeatStatusVacant
		seat.OccupiedAccountID = nil
		seat.LastAccountID = &lastID
		seat.UpdatedAt = now
		updated = &seat
		action = SocialSeatActionVacate
		extra = fmt.Sprintf("account_id=%d", accountID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if updated != nil {
		RecordSocialSeatLog(userId, action, updated.ID, 0, extra)
	}
	return updated, nil
}

// ExpireVacantSeatIfDue atomically transitions a past-due vacant seat to expired.
// Returns true only for the caller that won the transition.
func ExpireVacantSeatIfDue(seatID int64, userId int, now time.Time) (bool, error) {
	res := DB.Model(&SocialSeat{}).
		Where("id = ? AND user_id = ? AND status = ? AND expires_at <= ?",
			seatID, userId, SocialSeatStatusVacant, now).
		Updates(map[string]any{
			"status":              SocialSeatStatusExpired,
			"occupied_account_id": nil,
			"updated_at":          now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 1 {
		RecordSocialSeatLog(userId, SocialSeatActionExpire, seatID, 0, "vacant_expired")
		return true, nil
	}
	return false, nil
}

// ExpireGraceElapsedSeatTx atomically locks user+seat, re-verifies that the seat is
// still occupied and past graceDays, disconnects the account, and expires the seat.
// Only the winning caller receives expired=true and should perform post-commit upstream revoke + log.
func ExpireGraceElapsedSeatTx(tx *gorm.DB, userId int, seatID int64, graceDays int, now time.Time) (bool, *SocialAccount, error) {
	if err := LockSocialSeatUserTx(tx, userId); err != nil {
		return false, nil, err
	}
	var seat SocialSeat
	err := lockForUpdate(tx).Where("id = ? AND user_id = ?", seatID, userId).First(&seat).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if seat.Status != SocialSeatStatusOccupied {
		return false, nil, nil
	}
	if seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now) {
		return false, nil, nil
	}
	if seat.ExpiresAt.After(now) {
		return false, nil, nil
	}
	graceEnd := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
	if !now.After(graceEnd) {
		return false, nil, nil
	}
	var disconnectedAcc *SocialAccount
	if seat.OccupiedAccountID != nil {
		var acc SocialAccount
		if aerr := lockForUpdate(tx).Where("id = ? AND user_id = ?", *seat.OccupiedAccountID, userId).First(&acc).Error; aerr == nil {
			accCopy := acc
			disconnectedAcc = &accCopy
			if uerr := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", acc.ID, userId).
				Updates(map[string]any{
					"status":     SocialAccountStatusDisconnected,
					"seat_id":    nil,
					"updated_at": now,
				}).Error; uerr != nil {
				return false, nil, uerr
			}
		} else if !errors.Is(aerr, gorm.ErrRecordNotFound) {
			return false, nil, aerr
		}
	}
	if err := ExpireSocialSeatTx(tx, seat.ID, userId, now); err != nil {
		return false, nil, err
	}
	return true, disconnectedAcc, nil
}

// RenewSocialSeatResult reports the committed seat, deducted raw quota, and log action.
type RenewSocialSeatResult struct {
	Seat         *SocialSeat
	ChargedQuota int
	Action       string
}

// RenewOrActivateSeatTx manually renews an occupied seat or activates an unactivated account.
// Unexpired renewals extend expires_at by 30d up to now+maxValidityDays; grace renewals
// restart from now+30d. Paid renewals reset swap_count to 0.
func RenewOrActivateSeatTx(
	tx *gorm.DB,
	userId int,
	accountID int64,
	priceQuota int,
	graceDays int,
	maxValidityDays int,
	maxSwaps int,
	seatsCap int,
	now time.Time,
) (*RenewSocialSeatResult, error) {
	if tx == nil {
		return nil, errors.New("nil tx")
	}
	if err := LockSocialSeatUserTx(tx, userId); err != nil {
		return nil, err
	}

	var liveAcc SocialAccount
	if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", accountID, userId).First(&liveAcc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSocialSeatExpired
		}
		return nil, err
	}
	if liveAcc.Status != SocialAccountStatusActive {
		return nil, ErrSocialSeatExpired
	}

	var seat SocialSeat
	err := lockForUpdate(tx).
		Where("user_id = ? AND status = ? AND occupied_account_id = ?",
			userId, SocialSeatStatusOccupied, accountID).
		First(&seat).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hasSeat := err == nil
	isUnactivatedRow := hasSeat && seat.OpenedQuota == 0 && seat.GrandfatherUntil == nil && !seat.ExpiresAt.After(now)
	if hasSeat {
		graceEnd := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
		if now.After(graceEnd) {
			return nil, ErrSocialSeatExpired
		}
	}

	// Path B: legacy unactivated account (no seat row or zero-quota frozen placeholder).
	if !hasSeat || isUnactivatedRow {
		vacantSeat, occErr := OccupyVacantForAccountTx(tx, userId, accountID, maxSwaps, now)
		if occErr == nil {
			if isUnactivatedRow {
				if e := ExpireSocialSeatTx(tx, seat.ID, userId, now); e != nil {
					return nil, e
				}
			}
			sid := vacantSeat.ID
			if e := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
				Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; e != nil {
				return nil, e
			}
			return &RenewSocialSeatResult{
				Seat:         vacantSeat,
				ChargedQuota: 0,
				Action:       SocialSeatActionOccupy,
			}, nil
		}
		if !errors.Is(occErr, gorm.ErrRecordNotFound) {
			return nil, occErr
		}

		purchased, cerr := CountPurchasedSeatsTx(tx, userId, now)
		if cerr != nil {
			return nil, cerr
		}
		if purchased >= seatsCap {
			vacantCount, verr := CountVacantUnexpiredTx(tx, userId, now)
			if verr == nil && vacantCount > 0 {
				return nil, ErrSocialSeatSwapLimit
			}
			return nil, ErrSocialSeatCap
		}
		if cerr := ChargeSocialSeatTx(tx, userId, priceQuota); cerr != nil {
			return nil, cerr
		}

		next := now.Add(SocialSeatWindow())
		if isUnactivatedRow {
			accID := accountID
			if uerr := tx.Model(&seat).Updates(map[string]any{
				"expires_at":        next,
				"swap_count":        0,
				"opened_quota":      priceQuota,
				"last_account_id":   accID,
				"grandfather_until": nil,
				"updated_at":        now,
			}).Error; uerr != nil {
				return nil, uerr
			}
			seat.ExpiresAt = next
			seat.SwapCount = 0
			seat.OpenedQuota = priceQuota
			seat.LastAccountID = &accID
			seat.GrandfatherUntil = nil
			seat.UpdatedAt = now
			sid := seat.ID
			if uerr := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
				Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; uerr != nil {
				return nil, uerr
			}
			return &RenewSocialSeatResult{
				Seat:         &seat,
				ChargedQuota: priceQuota,
				Action:       SocialSeatActionActivate,
			}, nil
		}

		created, oerr := OpenOccupiedSeatTx(tx, userId, accountID, priceQuota, now, nil)
		if oerr != nil {
			return nil, oerr
		}
		sid := created.ID
		if uerr := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
			Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; uerr != nil {
			return nil, uerr
		}
		return &RenewSocialSeatResult{
			Seat:         created,
			ChargedQuota: priceQuota,
			Action:       SocialSeatActionActivate,
		}, nil
	}

	// Path A: renew existing active or frozen-in-grace occupied seat.
	if !seat.ExpiresAt.After(now) {
		purchased, cerr := CountPurchasedSeatsTx(tx, userId, now)
		if cerr != nil {
			return nil, cerr
		}
		if purchased >= seatsCap {
			return nil, ErrSocialSeatCap
		}
	}
	var next time.Time
	if seat.ExpiresAt.After(now) {
		next = seat.ExpiresAt.Add(SocialSeatWindow())
	} else {
		next = now.Add(SocialSeatWindow())
	}
	if next.After(now.Add(time.Duration(maxValidityDays) * 24 * time.Hour)) {
		return nil, ErrSocialSeatRenewLimit
	}
	if cerr := ChargeSocialSeatTx(tx, userId, priceQuota); cerr != nil {
		return nil, cerr
	}
	accID := accountID
	if uerr := tx.Model(&seat).Updates(map[string]any{
		"expires_at":        next,
		"swap_count":        0,
		"opened_quota":      priceQuota,
		"grandfather_until": nil,
		"last_account_id":   accID,
		"updated_at":        now,
	}).Error; uerr != nil {
		return nil, uerr
	}
	seat.ExpiresAt = next
	seat.SwapCount = 0
	seat.OpenedQuota = priceQuota
	seat.GrandfatherUntil = nil
	seat.LastAccountID = &accID
	seat.UpdatedAt = now
	sid := seat.ID
	if uerr := tx.Model(&SocialAccount{}).Where("id = ? AND user_id = ?", accountID, userId).
		Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; uerr != nil {
		return nil, uerr
	}
	return &RenewSocialSeatResult{
		Seat:         &seat,
		ChargedQuota: priceQuota,
		Action:       SocialSeatActionRenew,
	}, nil
}

func ListVacantUnexpiredSeats(userId int, now time.Time) ([]*SocialSeat, error) {
	var rows []*SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND expires_at > ?", userId, SocialSeatStatusVacant, now).
		Order("expires_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

func ListOccupiedSeatsByAccountIDs(userId int, accountIDs []int64) (map[int64]*SocialSeat, error) {
	out := make(map[int64]*SocialSeat, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	var rows []*SocialSeat
	err := DB.Where("user_id = ? AND status = ? AND occupied_account_id IN ?",
		userId, SocialSeatStatusOccupied, accountIDs).
		Order("id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.OccupiedAccountID == nil {
			continue
		}
		if _, exists := out[*r.OccupiedAccountID]; !exists {
			out[*r.OccupiedAccountID] = r
		}
	}
	return out, nil
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
// Valid only when occupied and unexpired (or inside an active grandfather window);
// expired seats (including those in the manual-renewal grace window) are frozen and return ErrSocialSeatExpired.
func AccountSeatUsable(userId int, accountID int64, now time.Time, graceDays int) error {
	_ = graceDays
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
