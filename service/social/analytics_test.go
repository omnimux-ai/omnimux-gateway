package social

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalytics_TenantIsolation_And_AccountGuard(t *testing.T) {
	clearSocialTables(t)

	// Create users & accounts
	user1ID := 10
	user2ID := 20

	require.NoError(t, model.DB.Create(&model.SocialTenant{
		UserId:            user1ID,
		Provider:          ProviderZernio,
		ProviderProfileID: "prof_user_10",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SocialTenant{
		UserId:            user2ID,
		Provider:          ProviderZernio,
		ProviderProfileID: "prof_user_20",
	}).Error)

	acc1 := &model.SocialAccount{
		UserId:            user1ID,
		Provider:          ProviderZernio,
		ProviderAccountID: "zernio_acc_10_tiktok",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	acc2 := &model.SocialAccount{
		UserId:            user2ID,
		Provider:          ProviderZernio,
		ProviderAccountID: "zernio_acc_20_twitter",
		Platform:          "twitter",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(acc1).Error)
	require.NoError(t, model.DB.Create(acc2).Error)

	// Mock Zernio Server
	var capturedProfileID string
	var capturedAccountIDs string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedProfileID = r.URL.Query().Get("profileId")
		capturedAccountIDs = r.URL.Query().Get("accountIds")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"dailyData": []map[string]any{
				{"date": "2026-08-01", "postCount": 5},
			},
		})
	}))
	defer srv.Close()

	svc := NewService(Config{
		Enabled: true,
		BaseURL: srv.URL,
		APIKey:  "test-secret-key",
	})

	ctx := context.Background()

	// 1. Happy path: User 1 queries their own account
	res, err := svc.GetDailyMetrics(ctx, user1ID, "2026-08-01", "2026-08-05", "tiktok", "UTC", []int64{acc1.ID})
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "/analytics/daily-metrics", capturedPath)
	assert.Equal(t, "prof_user_10", capturedProfileID)
	assert.Equal(t, "zernio_acc_10_tiktok", capturedAccountIDs)

	// 2. IDOR protection: User 1 attempts to query User 2's account
	_, err = svc.GetDailyMetrics(ctx, user1ID, "2026-08-01", "2026-08-05", "twitter", "UTC", []int64{acc2.ID})
	assert.ErrorIs(t, err, ErrAccountDenied)

	// 3. Mixed accounts: User 1 passes [acc1.ID, acc2.ID] -> rejected
	_, err = svc.GetDailyMetrics(ctx, user1ID, "2026-08-01", "2026-08-05", "tiktok", "UTC", []int64{acc1.ID, acc2.ID})
	assert.ErrorIs(t, err, ErrAccountDenied)
}

func TestAnalytics_AllEndpoints_HappyPath(t *testing.T) {
	clearSocialTables(t)

	userID := 100
	require.NoError(t, model.DB.Create(&model.SocialTenant{
		UserId:            userID,
		Provider:          ProviderZernio,
		ProviderProfileID: "prof_user_100",
	}).Error)

	acc := &model.SocialAccount{
		UserId:            userID,
		Provider:          ProviderZernio,
		ProviderAccountID: "zernio_acc_100",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(acc).Error)

	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"path":    r.URL.Path,
		})
	}))
	defer srv.Close()

	svc := NewService(Config{
		Enabled: true,
		BaseURL: srv.URL,
		APIKey:  "test-secret-key",
	})
	ctx := context.Background()

	// 1. BestTimeToPost
	res, err := svc.GetBestTimeToPost(ctx, userID, "tiktok", acc.ID, "UTC")
	require.NoError(t, err)
	assert.Equal(t, "/analytics/best-time-to-post", lastPath)
	assert.NotNil(t, res)

	// 2. PostingFrequency
	res, err = svc.GetPostingFrequency(ctx, userID, "tiktok")
	require.NoError(t, err)
	assert.Equal(t, "/analytics/posting-frequency", lastPath)
	assert.NotNil(t, res)

	// 3. ContentDecay
	res, err = svc.GetContentDecay(ctx, userID, "tiktok")
	require.NoError(t, err)
	assert.Equal(t, "/analytics/content-decay", lastPath)
	assert.NotNil(t, res)

	// 4. FollowerStats
	res, err = svc.GetFollowerStats(ctx, userID, 30, []int64{acc.ID})
	require.NoError(t, err)
	assert.Equal(t, "/accounts/follower-stats", lastPath)
	assert.NotNil(t, res)

	// 5. PostsAnalytics
	res, err = svc.GetPostsAnalytics(ctx, userID, "post_1", "tiktok", "date", "desc", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, "/analytics", lastPath)
	assert.NotNil(t, res)

	// 6. SyncExternalPosts
	res, err = svc.SyncExternalPosts(ctx, userID, acc.ID, "https://tiktok.com/@test/123")
	require.NoError(t, err)
	assert.Equal(t, "/analytics/sync-external-posts", lastPath)
	assert.NotNil(t, res)

	// 7. InboxVolume
	res, err = svc.GetInboxVolume(ctx, userID, "tiktok", "2026-08-01", "2026-08-10")
	require.NoError(t, err)
	assert.Equal(t, "/inbox-analytics/volume", lastPath)
	assert.NotNil(t, res)

	// 8. InboxResponseTime
	res, err = svc.GetInboxResponseTime(ctx, userID, "tiktok", "2026-08-01", "2026-08-10")
	require.NoError(t, err)
	assert.Equal(t, "/inbox-analytics/response-time", lastPath)
	assert.NotNil(t, res)

	// 9. InboxHeatmap
	res, err = svc.GetInboxHeatmap(ctx, userID, "tiktok", "UTC")
	require.NoError(t, err)
	assert.Equal(t, "/inbox-analytics/heatmap", lastPath)
	assert.NotNil(t, res)

	// 10. InboxSourceBreakdown
	res, err = svc.GetInboxSourceBreakdown(ctx, userID, "tiktok", "2026-08-01", "2026-08-10")
	require.NoError(t, err)
	assert.Equal(t, "/inbox-analytics/source-breakdown", lastPath)
	assert.NotNil(t, res)
}
