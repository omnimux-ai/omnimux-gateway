package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSocialUSDToQuota(t *testing.T) {
	q2, err := SocialUSDToQuota(2)
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, q2)
	q8, err := SocialUSDToQuota(8)
	require.NoError(t, err)
	assert.Equal(t, 4_000_000, q8)
	_, err = SocialUSDToQuota(-1)
	assert.Error(t, err)
}

func TestChargeSocialSeatTx(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 5_000_000)
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ChargeSocialSeatTx(tx, u.Id, 4_000_000)
	})
	require.NoError(t, err)
	var got User
	require.NoError(t, DB.First(&got, u.Id).Error)
	assert.Equal(t, 1_000_000, got.Quota)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ChargeSocialSeatTx(tx, u.Id, 4_000_000)
	})
	assert.ErrorIs(t, err, ErrSocialSeatQuota)
}

func TestOccupyVacantForAccountSwapRules(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 8_000_000)
	now := time.Now().UTC()
	later := &SocialSeat{
		UserId:    u.Id,
		Status:    SocialSeatStatusVacant,
		ExpiresAt: now.Add(48 * time.Hour),
	}
	sooner := &SocialSeat{
		UserId:    u.Id,
		Status:    SocialSeatStatusVacant,
		ExpiresAt: now.Add(24 * time.Hour),
	}
	require.NoError(t, DB.Create(later).Error)
	require.NoError(t, DB.Create(sooner).Error)

	// Legacy unbound vacant seat picks soonest-expiring and sets last_account_id without incrementing swap_count.
	var taken *SocialSeat
	err := DB.Transaction(func(tx *gorm.DB) error {
		s, e := OccupyVacantForAccountTx(tx, u.Id, 99, SocialSeatMaxSwapsPerWindow, now)
		taken = s
		return e
	})
	require.NoError(t, err)
	require.NotNil(t, taken)
	assert.Equal(t, sooner.ID, taken.ID)
	assert.Equal(t, SocialSeatStatusOccupied, taken.Status)
	require.NotNil(t, taken.OccupiedAccountID)
	assert.Equal(t, int64(99), *taken.OccupiedAccountID)
	require.NotNil(t, taken.LastAccountID)
	assert.Equal(t, int64(99), *taken.LastAccountID)
	assert.Equal(t, 0, taken.SwapCount)

	// Remove the second seat so swap tests exercise a single pit cleanly.
	require.NoError(t, DB.Delete(later).Error)

	// Same-account rebind 3 times: 0 swap_count growth.
	for range 3 {
		vac, verr := VacateOccupiedSeatForAccount(u.Id, 99)
		require.NoError(t, verr)
		require.NotNil(t, vac)
		assert.Equal(t, SocialSeatStatusVacant, vac.Status)
		require.NotNil(t, vac.LastAccountID)
		assert.Equal(t, int64(99), *vac.LastAccountID)

		var rebound *SocialSeat
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			s, e := OccupyVacantForAccountTx(tx, u.Id, 99, SocialSeatMaxSwapsPerWindow, now)
			rebound = s
			return e
		}))
		assert.Equal(t, sooner.ID, rebound.ID)
		assert.Equal(t, 0, rebound.SwapCount)
	}

	// Cross-account swap #1 (99 -> 100) and #2 (100 -> 101).
	_, err = VacateOccupiedSeatForAccount(u.Id, 99)
	require.NoError(t, err)
	var swap1 *SocialSeat
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		s, e := OccupyVacantForAccountTx(tx, u.Id, 100, SocialSeatMaxSwapsPerWindow, now)
		swap1 = s
		return e
	}))
	assert.Equal(t, 1, swap1.SwapCount)
	require.NotNil(t, swap1.LastAccountID)
	assert.Equal(t, int64(100), *swap1.LastAccountID)

	_, err = VacateOccupiedSeatForAccount(u.Id, 100)
	require.NoError(t, err)
	var swap2 *SocialSeat
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		s, e := OccupyVacantForAccountTx(tx, u.Id, 101, SocialSeatMaxSwapsPerWindow, now)
		swap2 = s
		return e
	}))
	assert.Equal(t, 2, swap2.SwapCount)
	require.NotNil(t, swap2.LastAccountID)
	assert.Equal(t, int64(101), *swap2.LastAccountID)

	// After 2 cross-account swaps, unbind 101:
	// - different account 102 cannot take the pit (ErrRecordNotFound)
	// - same account 101 can still rebind without limit.
	_, err = VacateOccupiedSeatForAccount(u.Id, 101)
	require.NoError(t, err)
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, e := OccupyVacantForAccountTx(tx, u.Id, 102, SocialSeatMaxSwapsPerWindow, now)
		return e
	})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var sameAfterCap *SocialSeat
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		s, e := OccupyVacantForAccountTx(tx, u.Id, 101, SocialSeatMaxSwapsPerWindow, now)
		sameAfterCap = s
		return e
	}))
	assert.Equal(t, sooner.ID, sameAfterCap.ID)
	assert.Equal(t, 2, sameAfterCap.SwapCount)
}

func TestPurchasedCountIncludesVacantAndCap(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 10)
	now := time.Now().UTC()
	require.NoError(t, DB.Create(&SocialSeat{
		UserId: u.Id, Status: SocialSeatStatusOccupied, ExpiresAt: now.Add(time.Hour),
	}).Error)
	require.NoError(t, DB.Create(&SocialSeat{
		UserId: u.Id, Status: SocialSeatStatusVacant, ExpiresAt: now.Add(2 * time.Hour),
	}).Error)
	require.NoError(t, DB.Create(&SocialSeat{
		UserId: u.Id, Status: SocialSeatStatusExpired, ExpiresAt: now.Add(-time.Hour),
	}).Error)
	n, err := CountPurchasedSeats(u.Id, now)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestVacateDoesNotRefund(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 4_000_000)
	now := time.Now().UTC()
	acc := &SocialAccount{
		UserId:            u.Id,
		Provider:          "zernio",
		ProviderAccountID: "acc_vacate",
		Platform:          "twitter",
		Status:            SocialAccountStatusActive,
	}
	require.NoError(t, DB.Create(acc).Error)
	seat := &SocialSeat{
		UserId:            u.Id,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         now.Add(10 * 24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		OpenedQuota:       4_000_000,
	}
	require.NoError(t, DB.Create(seat).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ChargeSocialSeatTx(tx, u.Id, 4_000_000)
	}))

	got, err := VacateOccupiedSeatForAccount(u.Id, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, SocialSeatStatusVacant, got.Status)
	assert.Nil(t, got.OccupiedAccountID)
	assert.Equal(t, seat.ExpiresAt.Unix(), got.ExpiresAt.Unix())

	var user User
	require.NoError(t, DB.First(&user, u.Id).Error)
	assert.Equal(t, 0, user.Quota)
}

func TestSocialUSDToQuotaUsesQuotaPerUnit(t *testing.T) {
	assert.Equal(t, 500000.0, common.QuotaPerUnit)
}

func TestVacateExpiredSeatMarksExpiredDirectly(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 1_000_000)
	now := time.Now().UTC()
	acc := &SocialAccount{
		UserId:            u.Id,
		Provider:          "tiktok_direct",
		ProviderAccountID: "oid_vacate_exp",
		Platform:          "tiktok",
		Status:            SocialAccountStatusActive,
	}
	require.NoError(t, DB.Create(acc).Error)
	seat := &SocialSeat{
		UserId:            u.Id,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         now.Add(-24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		LastAccountID:     &acc.ID,
		OpenedQuota:       1_000_000,
	}
	require.NoError(t, DB.Create(seat).Error)

	got, err := VacateOccupiedSeatForAccount(u.Id, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, SocialSeatStatusExpired, got.Status)
	assert.Nil(t, got.OccupiedAccountID)
}

func TestRenewOrActivateSeatTx(t *testing.T) {
	truncateTables(t)
	u := seedHostedUser(t, 4_000_000)
	now := time.Now().UTC().Truncate(time.Second)
	acc := &SocialAccount{
		UserId:            u.Id,
		Provider:          "tiktok_direct",
		ProviderAccountID: "oid_renew_tx",
		Platform:          "tiktok",
		Status:            SocialAccountStatusActive,
	}
	require.NoError(t, DB.Create(acc).Error)

	// Path B: activate materialized unactivated frozen row (opened_quota=0, expires_at=now).
	placeholder := &SocialSeat{
		UserId:            u.Id,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         now,
		OccupiedAccountID: &acc.ID,
		LastAccountID:     &acc.ID,
		SwapCount:         0,
		OpenedQuota:       0,
	}
	require.NoError(t, DB.Create(placeholder).Error)

	var res *RenewSocialSeatResult
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var e error
		res, e = RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	}))
	assert.Equal(t, SocialSeatActionActivate, res.Action)
	assert.Equal(t, 1_000_000, res.ChargedQuota)
	assert.Equal(t, placeholder.ID, res.Seat.ID)
	assert.Equal(t, now.Add(30*24*time.Hour).Unix(), res.Seat.ExpiresAt.Unix())

	// Simulate swap_count=2 then renew twice to reach 90d cap; 3rd renew hits ErrSocialSeatRenewLimit.
	require.NoError(t, DB.Model(&SocialSeat{}).Where("id = ?", placeholder.ID).Update("swap_count", 2).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var e error
		res, e = RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	}))
	assert.Equal(t, SocialSeatActionRenew, res.Action)
	assert.Equal(t, 0, res.Seat.SwapCount)
	assert.Equal(t, now.Add(60*24*time.Hour).Unix(), res.Seat.ExpiresAt.Unix())

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var e error
		res, e = RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	}))
	assert.Equal(t, now.Add(90*24*time.Hour).Unix(), res.Seat.ExpiresAt.Unix())

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, e := RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	})
	assert.ErrorIs(t, err, ErrSocialSeatRenewLimit)

	// Path A grace renewal (day 3 past expiry): resets to now+30d and swap_count=0.
	require.NoError(t, DB.Model(&SocialSeat{}).Where("id = ?", placeholder.ID).Updates(map[string]any{
		"expires_at": now.Add(-3 * 24 * time.Hour),
		"swap_count": 2,
	}).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var e error
		res, e = RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	}))
	assert.Equal(t, SocialSeatActionRenew, res.Action)
	assert.Equal(t, 0, res.Seat.SwapCount)
	assert.Equal(t, now.Add(30*24*time.Hour).Unix(), res.Seat.ExpiresAt.Unix())

	// Past grace (>7d) returns ErrSocialSeatExpired.
	require.NoError(t, DB.Model(&SocialSeat{}).Where("id = ?", placeholder.ID).Update("expires_at", now.Add(-8*24*time.Hour)).Error)
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, e := RenewOrActivateSeatTx(tx, u.Id, acc.ID, 1_000_000, 7, SocialSeatMaxValidityDays, SocialSeatMaxSwapsPerWindow, 20, now)
		return e
	})
	assert.ErrorIs(t, err, ErrSocialSeatExpired)
}

func TestSyncSocialSeatDebitCacheUpdatesRedis(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	prevRDB := common.RDB
	prevRedis := common.RedisEnabled
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = client.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevRedis
	})

	const userId = 909
	require.NoError(t, populateUserCache(User{
		Id:          userId,
		Username:    "u909",
		Quota:       2_000_000,
		Status:      1,
		Role:        1,
		AuthVersion: 1,
	}))

	SyncSocialSeatDebitCache(userId, 1_000_000, SocialSeatActionOpen)

	got, err := getUserQuotaCache(userId)
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, got)
}
