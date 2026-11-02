package hostedaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedWarming(t *testing.T, phone string) (*model.User, *model.HostedAccount, *model.HostedWarmupPlan) {
	t.Helper()
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: phone, Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, WarmupStatus: model.HostedWarmupInProgress,
		AssignedUserID: u.Id, BrandConfig: `{"nick_name":"north","keywords":["k1"]}`,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	plan := &model.HostedWarmupPlan{
		AccountID: acc.ID, PlaybookCode: warmupPlaybook, CurrentDay: 1,
		State: "generating", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	return u, acc, plan
}

func analyticsHandler(play int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/data" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		envelopeOK(w, map[string]any{
			"items": []map[string]any{{"playCount": play, "account": "a"}},
			"total": 1,
		})
	})
}

func TestWarmupNegativePlayCountNotReady(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "1")
	_, acc, plan := seedWarming(t, "phone-stale")
	svc := NewService(gwClient(t, analyticsHandler(-1)))
	n, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)
	assert.Greater(t, n, 0)

	var storedAcc model.HostedAccount
	require.NoError(t, model.DB.First(&storedAcc, acc.ID).Error)
	assert.Equal(t, model.HostedAccountWarmingUp, storedAcc.Status)
	assert.NotEqual(t, model.HostedWarmupReady, storedAcc.WarmupStatus)
	assert.NotEqual(t, 100, storedAcc.Readiness)

	var storedPlan model.HostedWarmupPlan
	require.NoError(t, model.DB.First(&storedPlan, plan.ID).Error)
	assert.Equal(t, "verifying", storedPlan.State)
	assert.NotEqual(t, "passed", storedPlan.State)
}

func TestWarmupPlayCount500Ready(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "1")
	_, acc, plan := seedWarming(t, "phone-ready")
	svc := NewService(gwClient(t, analyticsHandler(500)))
	_, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)

	var storedAcc model.HostedAccount
	require.NoError(t, model.DB.First(&storedAcc, acc.ID).Error)
	assert.Equal(t, model.HostedAccountReady, storedAcc.Status)
	assert.Equal(t, model.HostedWarmupReady, storedAcc.WarmupStatus)
	assert.Equal(t, 100, storedAcc.Readiness)

	var storedPlan model.HostedWarmupPlan
	require.NoError(t, model.DB.First(&storedPlan, plan.ID).Error)
	assert.Equal(t, "passed", storedPlan.State)
}

func TestWarmupWithoutFastForwardDoesNotSkipDays(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "0")
	_, acc, plan := seedWarming(t, "phone-slow")
	svc := NewService(nil)
	_, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)

	var storedPlan model.HostedWarmupPlan
	require.NoError(t, model.DB.First(&storedPlan, plan.ID).Error)
	assert.Equal(t, 1, storedPlan.CurrentDay)
	assert.NotEqual(t, "verifying", storedPlan.State)
	assert.NotEqual(t, "passed", storedPlan.State)

	var count int64
	require.NoError(t, model.DB.Model(&model.HostedTask{}).Where("account_id = ?", acc.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestWarmupReservedPhoneSkipped(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "1")
	_, acc, _ := seedWarming(t, ReservedMonthlyPhoneID)
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`nope`))
	})))
	n, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	var count int64
	require.NoError(t, model.DB.Model(&model.HostedTask{}).Where("account_id = ?", acc.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	var storedAcc model.HostedAccount
	require.NoError(t, model.DB.First(&storedAcc, acc.ID).Error)
	assert.Equal(t, model.HostedAccountWarmingUp, storedAcc.Status)
}

func TestWarmupFastForwardCreatesSevenDays(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "1")
	_, acc, _ := seedWarming(t, "phone-ff")
	svc := NewService(gwClient(t, analyticsHandler(-1)))
	_, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)
	var tasks []*model.HostedTask
	require.NoError(t, model.DB.Where("account_id = ?", acc.ID).Find(&tasks).Error)
	assert.GreaterOrEqual(t, len(tasks), 8) // 7 browse + d7 clip
	raw, _ := json.Marshal(tasks)
	assert.NotContains(t, string(raw), "password")
}

func TestWarmupSecondTickAdvancesDayWithoutFastForward(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "0")
	_, acc, plan := seedWarming(t, "phone-day2")
	svc := NewService(nil)
	_, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)
	_, err = svc.RunWarmupTick(context.Background())
	require.NoError(t, err)

	var storedPlan model.HostedWarmupPlan
	require.NoError(t, model.DB.First(&storedPlan, plan.ID).Error)
	assert.Equal(t, 2, storedPlan.CurrentDay, "日更路径第二 tick 必须推进 CurrentDay")
	assert.NotEqual(t, "verifying", storedPlan.State)

	var count int64
	require.NoError(t, model.DB.Model(&model.HostedTask{}).Where("account_id = ?", acc.ID).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestAggregatePlayCountStaleNeverCountsAsReady(t *testing.T) {
	play, stale := aggregatePlayCount(&hostedgateway.AnalyticsDataPage{
		Items: []hostedgateway.AnalyticsRow{{PlayCount: -1, PlayCountStale: true}},
	})
	assert.Equal(t, int64(-1), play)
	assert.True(t, stale)

	play, stale = aggregatePlayCount(&hostedgateway.AnalyticsDataPage{
		Items: []hostedgateway.AnalyticsRow{
			{PlayCount: -1, PlayCountStale: true, Account: "a"},
			{PlayCount: 800, PlayCountStale: false, Account: "a"},
		},
	})
	assert.True(t, stale, "任一 PlayCountStale 行不得被 800 盖掉，evaluateD7 才能拒绝 ready")
	assert.True(t, stale || play < 0)
}

func TestWarmupMixedStaleRowDoesNotReady(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_WARMUP_FAST_FORWARD", "1")
	_, acc, plan := seedWarming(t, "phone-mixed-stale")
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/data" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		envelopeOK(w, map[string]any{
			"items": []map[string]any{
				{"playCount": -1, "account": "a"},
				{"playCount": 800, "account": "a"},
			},
			"total": 2,
		})
	})))
	_, err := svc.RunWarmupTick(context.Background())
	require.NoError(t, err)

	var storedAcc model.HostedAccount
	require.NoError(t, model.DB.First(&storedAcc, acc.ID).Error)
	assert.Equal(t, model.HostedAccountWarmingUp, storedAcc.Status)
	assert.NotEqual(t, model.HostedWarmupReady, storedAcc.WarmupStatus)

	var storedPlan model.HostedWarmupPlan
	require.NoError(t, model.DB.First(&storedPlan, plan.ID).Error)
	assert.Equal(t, "verifying", storedPlan.State)
}
