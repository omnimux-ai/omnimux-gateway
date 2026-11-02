package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSocialUSDToQuota(t *testing.T) {
	q, err := SocialUSDToQuota(8)
	require.NoError(t, err)
	assert.Equal(t, 4_000_000, q)
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

func TestOccupyEarliestVacant(t *testing.T) {
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

	var taken *SocialSeat
	err := DB.Transaction(func(tx *gorm.DB) error {
		s, e := OccupyEarliestVacantTx(tx, u.Id, 99, now)
		taken = s
		return e
	})
	require.NoError(t, err)
	require.NotNil(t, taken)
	assert.Equal(t, sooner.ID, taken.ID)
	assert.Equal(t, SocialSeatStatusOccupied, taken.Status)
	require.NotNil(t, taken.OccupiedAccountID)
	assert.Equal(t, int64(99), *taken.OccupiedAccountID)
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
