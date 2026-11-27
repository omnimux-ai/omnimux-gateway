package social

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withSeatBilling(t *testing.T, mode string) {
	t.Helper()
	cfg := operation_setting.GetSocialOpsSetting()
	prevMode := cfg.BillingMode
	prevUSD := cfg.SeatPriceUSD
	prevCap := cfg.SeatsCap
	prevGF := cfg.GrandfatherDays
	prevGrace := cfg.GraceDays
	cfg.BillingMode = mode
	t.Cleanup(func() {
		cfg.BillingMode = prevMode
		cfg.SeatPriceUSD = prevUSD
		cfg.SeatsCap = prevCap
		cfg.GrandfatherDays = prevGF
		cfg.GraceDays = prevGrace
	})
}

func seedSeatUser(t *testing.T, quota int) *model.User {
	t.Helper()
	u := &model.User{
		Username: fmt.Sprintf("seat-%d-%d-%s", quota, common.GetTimestamp(), common.GetRandomString(6)),
		Password: "password1",
		Quota:    quota,
		Status:   1,
		Role:     1,
		AffCode:  common.GetRandomString(8),
	}
	require.NoError(t, model.DB.Create(u).Error)
	return u
}

func TestPreflightConnect_OffModeNil(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "off")
	s := NewService(Config{})
	assert.NoError(t, s.PreflightConnect(1))
}

func TestPreflightConnect_SeatNoVacantInsufficientQuota(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")
	u := seedSeatUser(t, 100)
	s := NewService(Config{})
	err := s.PreflightConnect(u.Id)
	assert.ErrorIs(t, err, ErrSeatQuota)
}

func TestApplySeatGate_OccupyVacantDoesNotCharge(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")
	u := seedSeatUser(t, 8_000_000)
	now := time.Now().UTC()
	vacant := &model.SocialSeat{
		UserId:    u.Id,
		Status:    model.SocialSeatStatusVacant,
		ExpiresAt: now.Add(48 * time.Hour),
	}
	require.NoError(t, model.DB.Create(vacant).Error)

	s := NewService(Config{})
	acc, err := s.applySeatGate(context.Background(), u.Id, ingestDraft{
		UserId:            u.Id,
		Provider:          ProviderZernio,
		ProviderAccountID: "acc_vacant_reuse",
		Platform:          "twitter",
		Username:          "alice",
		DisplayName:       "Alice",
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.NotNil(t, acc.SeatID)
	assert.Equal(t, vacant.ID, *acc.SeatID)

	var user model.User
	require.NoError(t, model.DB.First(&user, u.Id).Error)
	assert.Equal(t, 8_000_000, user.Quota)

	var seat model.SocialSeat
	require.NoError(t, model.DB.First(&seat, vacant.ID).Error)
	assert.Equal(t, model.SocialSeatStatusOccupied, seat.Status)
	require.NotNil(t, seat.OccupiedAccountID)
	assert.Equal(t, acc.ID, *seat.OccupiedAccountID)
}

func TestPurgeUserSeats_MarksOccupiedExpired(t *testing.T) {
	clearSocialTables(t)
	u := seedSeatUser(t, 4_000_000)
	acc := &model.SocialAccount{
		UserId:            u.Id,
		Provider:          ProviderZernio,
		ProviderAccountID: "acc_purge",
		Platform:          "twitter",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := time.Now().UTC()
	seat := &model.SocialSeat{
		UserId:            u.Id,
		Status:            model.SocialSeatStatusOccupied,
		ExpiresAt:         now.Add(10 * 24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		OpenedQuota:       4_000_000,
	}
	require.NoError(t, model.DB.Create(seat).Error)

	s := &Service{cfg: Config{}}
	require.Nil(t, s.client)
	require.NoError(t, s.PurgeUserSeats(context.Background(), u.Id))

	var got model.SocialSeat
	require.NoError(t, model.DB.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusExpired, got.Status)
	assert.Nil(t, got.OccupiedAccountID)
}

func TestSocialOpsSettingDefaultsAndZeroDays(t *testing.T) {
	cfg := operation_setting.GetSocialOpsSetting()
	prev := *cfg
	t.Cleanup(func() { *cfg = prev })

	assert.Equal(t, "off", operation_setting.EffectiveSocialSeatBillingMode())
	assert.Equal(t, 2.0, operation_setting.SocialSeatPriceUSD())
	assert.Equal(t, 20, operation_setting.SocialSeatPriceCredits())
	assert.Equal(t, 20, operation_setting.SocialSeatsCap())
	assert.Equal(t, 0, operation_setting.SocialSeatGrandfatherDays())
	assert.Equal(t, 7, operation_setting.SocialSeatGraceDays())
	assert.Equal(t, 90, operation_setting.SocialSeatMaxValidityDays())
	assert.Equal(t, 2, operation_setting.SocialSeatMaxSwapsPerWindow())

	// Zero grandfather_days must not silently fall back to 7.
	cfg.GrandfatherDays = 0
	assert.Equal(t, 0, operation_setting.SocialSeatGrandfatherDays())
	cfg.GrandfatherDays = -3
	assert.Equal(t, 0, operation_setting.SocialSeatGrandfatherDays())
	cfg.GrandfatherDays = 5
	assert.Equal(t, 5, operation_setting.SocialSeatGrandfatherDays())

	// Zero grace_days stays 0; negative normalizes to 0.
	cfg.GraceDays = 0
	assert.Equal(t, 0, operation_setting.SocialSeatGraceDays())
	cfg.GraceDays = -2
	assert.Equal(t, 0, operation_setting.SocialSeatGraceDays())
	cfg.GraceDays = 7
	assert.Equal(t, 7, operation_setting.SocialSeatGraceDays())
}

func TestReconcileExpiredSeats_NoAutoChargeAndGraceAutoDisconnect(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")

	var revokeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/oauth/revoke/" {
			revokeCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)

	u := seedSeatUser(t, 10_000_000) // plenty of quota; must never be auto-charged
	enc, err := SealToken("at-grace", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	acc := &model.SocialAccount{
		UserId:               u.Id,
		Provider:             ProviderTikTokDirect,
		ProviderAccountID:    "oid-grace-1",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: enc,
	}
	require.NoError(t, model.DB.Create(acc).Error)

	// 3 days expired (within 7d grace): zero auto-charge, blocked from posting.
	now := time.Now().UTC()
	seat := &model.SocialSeat{
		UserId:            u.Id,
		Status:            model.SocialSeatStatusOccupied,
		ExpiresAt:         now.Add(-3 * 24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		LastAccountID:     &acc.ID,
		OpenedQuota:       1_000_000,
	}
	require.NoError(t, model.DB.Create(seat).Error)

	view, err := s.SeatsView(context.Background(), u.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, view.SeatsOccupied)
	assert.Equal(t, 7, view.GraceDays)
	assert.Equal(t, 90, view.MaxValidityDays)
	assert.Equal(t, 2, view.MaxSwapsPerWindow)

	rows, err := s.ListAccounts(context.Background(), u.Id, ProviderTikTokDirect)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	err = s.guardSeatsForAccounts(u.Id, rows)
	assert.ErrorIs(t, err, ErrSeatExpired)

	var afterGraceCheck model.User
	require.NoError(t, model.DB.First(&afterGraceCheck, u.Id).Error)
	assert.Equal(t, 10_000_000, afterGraceCheck.Quota, "expired seat in grace must never auto-charge")
	assert.Zero(t, revokeCalls.Load())

	// Push past 7-day grace -> auto-disconnect + TikTok Revoke + seat expired.
	require.NoError(t, model.DB.Model(&model.SocialSeat{}).Where("id = ?", seat.ID).
		Update("expires_at", now.Add(-8*24*time.Hour)).Error)

	rows, err = s.ListAccounts(context.Background(), u.Id, ProviderTikTokDirect)
	require.NoError(t, err)
	assert.Empty(t, rows)
	assert.EqualValues(t, 1, revokeCalls.Load())

	var refreshedAcc model.SocialAccount
	require.NoError(t, model.DB.First(&refreshedAcc, acc.ID).Error)
	assert.Equal(t, model.SocialAccountStatusDisconnected, refreshedAcc.Status)

	var refreshedSeat model.SocialSeat
	require.NoError(t, model.DB.First(&refreshedSeat, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusExpired, refreshedSeat.Status)
}

func TestLegacyAccountZeroGrandfatherMaterializesUnactivatedAndActivates(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")
	operation_setting.GetSocialOpsSetting().GrandfatherDays = 0

	cfg := testDirectConfig(t, "http://stub")
	s := NewService(cfg)
	u := seedSeatUser(t, 2_000_000)

	legacy := &model.SocialAccount{
		UserId:            u.Id,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-legacy-0gf",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(legacy).Error)

	rows, err := s.ListAccounts(context.Background(), u.Id, ProviderTikTokDirect)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	// Immediately frozen (unactivated), not counted in seats_purchased, blocks posting.
	views, err := s.BuildAccountSeatViews(u.Id, rows)
	require.NoError(t, err)
	require.NotNil(t, views[legacy.ID])
	assert.Equal(t, "unactivated", views[legacy.ID].Status)
	assert.True(t, views[legacy.ID].CanRenew)
	assert.Equal(t, 20, views[legacy.ID].PriceCredits)

	seatSummary, err := s.SeatsView(context.Background(), u.Id)
	require.NoError(t, err)
	assert.Equal(t, 0, seatSummary.SeatsPurchased)

	assert.ErrorIs(t, s.guardSeatsForAccounts(u.Id, rows), ErrSeatExpired)

	// Manual activation charges 20 credits (1_000_000 raw quota) and unlocks posting.
	activated, err := s.RenewAccountSeat(context.Background(), u.Id, ProviderTikTokDirect, legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", activated.Status)
	assert.Equal(t, 0, activated.SwapCount)
	assert.Equal(t, 2, activated.SwapsRemaining)

	var afterUser model.User
	require.NoError(t, model.DB.First(&afterUser, u.Id).Error)
	assert.Equal(t, 1_000_000, afterUser.Quota)
	assert.NoError(t, s.guardSeatsForAccounts(u.Id, rows))
}

func TestSeatLifecycle_SameAccountRebindCrossSwapAndManualRenew(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")

	cfg := testDirectConfig(t, "http://stub")
	s := NewService(cfg)
	// Balance for 2 seats total (40 credits = 2_000_000 raw quota) + 2 renewals (40 credits).
	u := seedSeatUser(t, 4_000_000)

	bind := func(openID string) (*model.SocialAccount, error) {
		existing, _ := model.GetSocialAccountByProviderRef(ProviderTikTokDirect, openID)
		return s.applySeatGate(context.Background(), u.Id, ingestDraft{
			UserId:            u.Id,
			Provider:          ProviderTikTokDirect,
			ProviderAccountID: openID,
			ProviderProfileID: openID,
			Platform:          "tiktok",
			Username:          openID,
			DisplayName:       openID,
		}, existing)
	}

	// 1) First bind A: charges 20 credits (1_000_000).
	accA, err := bind("oid-a")
	require.NoError(t, err)
	require.NotNil(t, accA.SeatID)
	seat1ID := *accA.SeatID

	// 2) Unbind A -> rebind A 3 times: 0 credits, swap_count stays 0.
	for range 3 {
		require.NoError(t, s.vacateAfterDisconnect(u.Id, accA.ID))
		rebound, rerr := bind("oid-a")
		require.NoError(t, rerr)
		assert.Equal(t, seat1ID, *rebound.SeatID)
	}
	var seat1 model.SocialSeat
	require.NoError(t, model.DB.First(&seat1, seat1ID).Error)
	assert.Equal(t, 0, seat1.SwapCount)

	// 3) Cross-account swaps: A -> B (swap=1), B -> C (swap=2), both 0 charge.
	require.NoError(t, s.vacateAfterDisconnect(u.Id, accA.ID))
	accB, err := bind("oid-b")
	require.NoError(t, err)
	assert.Equal(t, seat1ID, *accB.SeatID)

	require.NoError(t, s.vacateAfterDisconnect(u.Id, accB.ID))
	accC, err := bind("oid-c")
	require.NoError(t, err)
	assert.Equal(t, seat1ID, *accC.SeatID)
	require.NoError(t, model.DB.First(&seat1, seat1ID).Error)
	assert.Equal(t, 2, seat1.SwapCount)

	// 4) Unbind C -> same account C still rebinds for free; then unbind C -> seat1 shows same_account_only=true.
	require.NoError(t, s.vacateAfterDisconnect(u.Id, accC.ID))
	seatView, err := s.SeatsView(context.Background(), u.Id)
	require.NoError(t, err)
	require.Len(t, seatView.VacantSeats, 1)
	assert.Equal(t, 0, seatView.VacantSeats[0].SwapsRemaining)
	assert.True(t, seatView.VacantSeats[0].SameAccountOnly)

	// 5) 3rd different account D cannot reuse swap-exhausted seat1 -> opens new seat2 for 20 credits.
	accD, err := bind("oid-d")
	require.NoError(t, err)
	require.NotNil(t, accD.SeatID)
	assert.NotEqual(t, seat1ID, *accD.SeatID)

	var afterD model.User
	require.NoError(t, model.DB.First(&afterD, u.Id).Error)
	assert.Equal(t, 2_000_000, afterD.Quota)

	// 6) Renew D twice to hit 90d cap; 3rd renew returns ErrSeatRenewLimit.
	r1, err := s.RenewAccountSeat(context.Background(), u.Id, ProviderTikTokDirect, accD.ID)
	require.NoError(t, err)
	assert.True(t, r1.CanRenew)
	r2, err := s.RenewAccountSeat(context.Background(), u.Id, ProviderTikTokDirect, accD.ID)
	require.NoError(t, err)
	assert.False(t, r2.CanRenew)
	assert.Equal(t, "SOCIAL_SEAT_RENEW_LIMIT", r2.RenewBlockedReason)

	_, err = s.RenewAccountSeat(context.Background(), u.Id, ProviderTikTokDirect, accD.ID)
	assert.ErrorIs(t, err, ErrSeatRenewLimit)
}

func TestApplySeatGate_FrozenAccountReOAuthCannotBypassFreeze(t *testing.T) {
	clearSocialTables(t)
	withSeatBilling(t, "seat")

	s := NewService(testDirectConfig(t, "http://stub"))
	u := seedSeatUser(t, 1_000_000)
	acc := &model.SocialAccount{
		UserId:            u.Id,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-frozen-reauth",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	frozen := &model.SocialSeat{
		UserId:            u.Id,
		Status:            model.SocialSeatStatusOccupied,
		ExpiresAt:         time.Now().UTC().Add(-24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		LastAccountID:     &acc.ID,
		OpenedQuota:       1_000_000,
	}
	require.NoError(t, model.DB.Create(frozen).Error)

	// Re-OAuth charges 20 credits for a fresh seat and expires the stale frozen seat row.
	got, err := s.applySeatGate(context.Background(), u.Id, ingestDraft{
		UserId:            u.Id,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-frozen-reauth",
		ProviderProfileID: "oid-frozen-reauth",
		Platform:          "tiktok",
		Username:          "frozenuser",
		DisplayName:       "Frozen User",
	}, acc)
	require.NoError(t, err)
	require.NotNil(t, got.SeatID)
	assert.NotEqual(t, frozen.ID, *got.SeatID)

	var oldSeat model.SocialSeat
	require.NoError(t, model.DB.First(&oldSeat, frozen.ID).Error)
	assert.Equal(t, model.SocialSeatStatusExpired, oldSeat.Status)

	var afterUser model.User
	require.NoError(t, model.DB.First(&afterUser, u.Id).Error)
	assert.Equal(t, 0, afterUser.Quota)
}
