package social

import (
	"context"
	"fmt"
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
		Username: fmt.Sprintf("seat-%d-%d", quota, common.GetTimestamp()),
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
