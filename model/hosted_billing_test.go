package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHostedDB(t *testing.T) {
	t.Helper()
	truncateTables(t)
}

func seedHostedUser(t *testing.T, quota int) *User {
	t.Helper()
	u := &User{
		Username: fmt.Sprintf("hosted-%d-%d", quota, common.GetTimestamp()),
		Password: "password1",
		Quota:    quota,
		Status:   1,
		Role:     1,
		AffCode:  common.GetRandomString(8),
	}
	require.NoError(t, DB.Create(u).Error)
	return u
}

func seedHostedSKU(t *testing.T) *HostedSKU {
	t.Helper()
	sku := &HostedSKU{Platform: HostedPlatformTikTok, Region: HostedRegionUS, PriceUSD: 99, Enabled: true, SortOrder: 1}
	require.NoError(t, DB.Create(sku).Error)
	return sku
}

func TestHostedUSDToQuota(t *testing.T) {
	setupHostedDB(t)
	q, clamp, err := HostedUSDToQuota(99)
	require.NoError(t, err)
	assert.Nil(t, clamp)
	assert.Equal(t, 49500000, q)
	_, _, err = HostedUSDToQuota(-1)
	assert.ErrorIs(t, err, ErrHostedInvalidAmount)
}

func TestHostedAddonAmountCeil(t *testing.T) {
	amount, err := HostedAddonAmountUSD(99, 1, 15)
	require.NoError(t, err)
	assert.Equal(t, 50.0, amount)
	amount, err = HostedAddonAmountUSD(99, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 4.0, amount)
}

func TestPurchaseHostedChannelsWithBalance(t *testing.T) {
	setupHostedDB(t)
	u := seedHostedUser(t, 100000000)
	sku := seedHostedSKU(t)

	res, err := PurchaseHostedChannelsWithBalance(u.Id, sku.ID, 1, 0, HostedOrderKindInitial, "k1", "")
	require.NoError(t, err)
	require.NotNil(t, res.Order)
	assert.Equal(t, HostedOrderPaid, res.Order.Status)
	assert.Equal(t, 1, res.Entitlement.Quantity)
	assert.Equal(t, 99.0, res.AmountUSD)
	assert.Equal(t, int64(0), res.Entitlement.ExpireAt)

	var user User
	require.NoError(t, DB.First(&user, u.Id).Error)
	assert.Equal(t, 100000000-49500000, user.Quota)

	again, err := PurchaseHostedChannelsWithBalance(u.Id, sku.ID, 1, 0, HostedOrderKindInitial, "k1", "")
	require.NoError(t, err)
	assert.True(t, again.Idempotent)
	assert.Equal(t, res.Order.ID, again.Order.ID)

	require.NoError(t, DB.First(&user, u.Id).Error)
	assert.Equal(t, 100000000-49500000, user.Quota)
}

func TestPurchaseHostedRejectsOverLimitAndInsufficient(t *testing.T) {
	setupHostedDB(t)
	poor := seedHostedUser(t, 1)
	sku := seedHostedSKU(t)
	_, err := PurchaseHostedChannelsWithBalance(poor.Id, sku.ID, 1, 0, HostedOrderKindInitial, "", "")
	assert.ErrorIs(t, err, ErrHostedBalanceInsufficient)

	rich := seedHostedUser(t, 500000000)
	_, err = PurchaseHostedChannelsWithBalance(rich.Id, sku.ID, 6, 0, HostedOrderKindInitial, "", "")
	assert.ErrorIs(t, err, ErrHostedInvalidQuantity)

	_, err = PurchaseHostedChannelsWithBalance(rich.Id, sku.ID, 5, 0, HostedOrderKindInitial, "a", "")
	require.NoError(t, err)
	_, err = PurchaseHostedChannelsWithBalance(rich.Id, sku.ID, 1, 0, HostedOrderKindInitial, "b", "")
	assert.ErrorIs(t, err, ErrHostedOrderLimit)
}

func TestRenewHostedEntitlementExtendsExpire(t *testing.T) {
	setupHostedDB(t)
	u := seedHostedUser(t, 200000000)
	sku := seedHostedSKU(t)
	res, err := PurchaseHostedChannelsWithBalance(u.Id, sku.ID, 1, 0, HostedOrderKindInitial, "buy", "")
	require.NoError(t, err)
	now := hostedNow()
	require.NoError(t, MarkHostedExpireStarted(res.Entitlement, now))

	renewed, err := RenewHostedEntitlement(u.Id, sku.ID, "renew-1")
	require.NoError(t, err)
	assert.Equal(t, HostedOrderKindRenew, renewed.Order.Kind)
	assert.Greater(t, renewed.Entitlement.ExpireAt, now+HostedPeriodSeconds()-2)
	assert.Equal(t, 1, renewed.Entitlement.Quantity)
}

func TestAddonKeepsExpireAddsQuantity(t *testing.T) {
	setupHostedDB(t)
	u := seedHostedUser(t, 200000000)
	sku := seedHostedSKU(t)
	res, err := PurchaseHostedChannelsWithBalance(u.Id, sku.ID, 1, 0, HostedOrderKindInitial, "buy", "")
	require.NoError(t, err)
	now := hostedNow()
	require.NoError(t, MarkHostedExpireStarted(res.Entitlement, now))
	expire := res.Entitlement.ExpireAt

	addon, err := PurchaseHostedChannelsWithBalance(u.Id, sku.ID, 1, 15, HostedOrderKindAddon, "add", "")
	require.NoError(t, err)
	assert.Equal(t, 2, addon.Entitlement.Quantity)
	assert.Equal(t, expire, addon.Entitlement.ExpireAt)
	assert.Equal(t, 50.0, addon.AmountUSD)
}
