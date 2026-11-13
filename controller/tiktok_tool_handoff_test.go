package controller

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	handoffWorkID  = "7470610564927032609"
	handoffPostURL = "https://www.tiktok.com/@mythicshadowws/video/7470610564927032609"
	handoffCDNURL  = "https://v77-perf.tiktokcdn-eu.com/v.mp4"
)

// handoffPass is the tool pass for the current test; Turnstile tokens are
// single use, so each test verifies once.
var handoffPass []*http.Cookie

type handoffTransfer struct {
	mu   sync.Mutex
	urls []string
	err  error
}

// tiktokHandoffTestEnv adds Redis, a resolved post and stubbed transfer/store
// to the tool fixture.
func tiktokHandoffTestEnv(t *testing.T, durationMs int) (*miniredis.Miniredis, *handoffTransfer) {
	t.Helper()
	tiktokToolTestEnv(t)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldRDB := common.RDB
	common.RedisEnabled, common.RDB = true, client
	tiktokToolCache.entries[handoffWorkID] = tiktokToolCacheEntry{
		body:    tiktokPost{id: handoffWorkID, kind: "video", uniqueID: "mythicshadowws", videoURL: handoffCDNURL, durationMs: durationMs},
		expires: time.Now().Add(tiktokToolCacheTTL),
	}
	handoffPass = tiktokToolPass(t)
	transfer := &handoffTransfer{}
	oldTransfer, oldStore := tiktokHandoffTransfer, tiktokHandoffStore
	tiktokHandoffTransfer = func(_ context.Context, videoURL string) ([]byte, service.InfluencerVideoMetadata, error) {
		transfer.mu.Lock()
		defer transfer.mu.Unlock()
		transfer.urls = append(transfer.urls, videoURL)
		return []byte("mp4"), service.InfluencerVideoMetadata{Duration: 15}, transfer.err
	}
	tiktokHandoffStore = func(_ *gin.Context, userID int, _ []byte, name string, _ *service.InfluencerVideoMetadata) (*dto.FileInfoData, error) {
		return &dto.FileInfoData{FileURL: "https://files.omnimux.ai/influencer/reference/f_" + name}, nil
	}
	t.Cleanup(func() {
		tiktokHandoffBeforeConsume = nil
		tiktokHandoffTransfer, tiktokHandoffStore = oldTransfer, oldStore
		common.RDB = oldRDB
		require.NoError(t, client.Close())
	})
	return server, transfer
}

func issueHandoff(t *testing.T, userID int) string {
	t.Helper()
	recorder := tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"`+handoffWorkID+`"}`, handoffPass, userID)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	token := gjson.GetBytes(recorder.Body.Bytes(), "data.token").String()
	require.NotEmpty(t, token)
	return token
}

func redeemHandoff(t *testing.T, token string, userID int) (int, string, string) {
	t.Helper()
	recorder := tiktokToolRequest(t, RedeemTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff/redeem", `{"token":"`+token+`"}`, nil, userID)
	return recorder.Code, tiktokToolCode(recorder), recorder.Body.String()
}

func TestTikTokHandoffTokenCarriesNoLinkAndRedisKeepsOnlyTheWorkID(t *testing.T) {
	server, _ := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 0)

	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	assert.Len(t, raw, 32, "the token is random bytes only")
	assert.NotContains(t, token, handoffWorkID)
	keys := server.Keys()
	var stored []string
	for _, key := range keys {
		if value, err := server.Get(key); err == nil && len(key) > len(tiktokHandoffKeyPrefix) && key[:len(tiktokHandoffKeyPrefix)] == tiktokHandoffKeyPrefix {
			stored = append(stored, value)
			assert.NotContains(t, key, token, "Redis stores a hash of the token")
			assert.Equal(t, tiktokHandoffTTL, server.TTL(key))
		}
	}
	require.Len(t, stored, 1)
	for _, forbidden := range []string{handoffPostURL, "tiktok.com", "tiktokcdn"} {
		assert.NotContains(t, stored[0], forbidden)
	}
}

func TestTikTokHandoffIssueRequiresThePassAndARecentlyResolvedVideo(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	recorder := tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"`+handoffWorkID+`"}`, nil, 0)
	assert.Equal(t, "verification_required", tiktokToolCode(recorder))

	recorder = tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"7470610564927032000"}`, handoffPass, 0)
	assert.Equal(t, http.StatusGone, recorder.Code)
	assert.Equal(t, "handoff_expired", tiktokToolCode(recorder))

	recorder = tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"https://www.tiktok.com/x"}`, handoffPass, 0)
	assert.Equal(t, "invalid_request", tiktokToolCode(recorder))
}

func TestTikTokHandoffIssueRejectsVideosOutsideTheRecreateWindow(t *testing.T) {
	for _, durationMs := range []int{3000, 31000} {
		tiktokHandoffTestEnv(t, durationMs)
		recorder := tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"`+handoffWorkID+`"}`, handoffPass, 0)
		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code, "%dms", durationMs)
		assert.Equal(t, "handoff_duration", tiktokToolCode(recorder))
	}
}

func TestTikTokHandoffRedeemRefetchesByWorkIDAndReturnsTheOwnedUpload(t *testing.T) {
	_, transfer := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 0)

	status, _, body := redeemHandoff(t, token, 42)
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, []string{handoffCDNURL}, transfer.urls, "the transfer reads the post's media, not the page's signed path")
	assert.Equal(t, "https://files.omnimux.ai/influencer/reference/f_tiktok-"+handoffWorkID+".mp4", gjson.Get(body, "data.url").String())
	assert.NotContains(t, body, "tiktokcdn")
	assert.NotContains(t, body, handoffPostURL)
}

func TestTikTokHandoffRedeemNeedsASessionAndKeepsTheToken(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 0)

	status, code, _ := redeemHandoff(t, token, 0)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "login_required", code)
	status, _, body := redeemHandoff(t, token, 42)
	assert.Equal(t, http.StatusOK, status, body)
}

func TestTikTokHandoffRedeemRejectsAnotherOwner(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	userToken := issueHandoff(t, 42)
	status, code, _ := redeemHandoff(t, userToken, 43)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "handoff_forbidden", code)

	guestToken := issueHandoff(t, 0)
	recorder := tiktokToolRequest(t, func(c *gin.Context) {
		c.Request.Header.Set("User-Agent", "another-browser")
		RedeemTikTokToolHandoff(c)
	}, http.MethodPost, "/api/tools/tiktok/handoff/redeem", `{"token":"`+guestToken+`"}`, nil, 42)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Equal(t, "handoff_forbidden", tiktokToolCode(recorder))

	status, _, body := redeemHandoff(t, userToken, 42)
	assert.Equal(t, http.StatusOK, status, "a refused owner check does not burn the token: %s", body)
}

func TestTikTokHandoffRedeemIsSingleUse(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 42)
	status, _, body := redeemHandoff(t, token, 42)
	require.Equal(t, http.StatusOK, status, body)

	status, code, _ := redeemHandoff(t, token, 42)
	assert.Equal(t, http.StatusGone, status)
	assert.Equal(t, "handoff_expired", code)
}

func TestTikTokHandoffRedeemFailsAfterTheTokenExpires(t *testing.T) {
	server, transfer := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 42)
	server.FastForward(tiktokHandoffTTL + time.Second)

	status, code, _ := redeemHandoff(t, token, 42)
	assert.Equal(t, http.StatusGone, status)
	assert.Equal(t, "handoff_expired", code)
	assert.Empty(t, transfer.urls)
}

func TestTikTokHandoffRedeemMapsTransferRejections(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{service.ErrTikTokHandoffDuration, http.StatusUnprocessableEntity, "handoff_duration"},
		{service.ErrTikTokHandoffTooLarge, http.StatusRequestEntityTooLarge, "handoff_too_large"},
		{service.ErrInfluencerVideoUnavailable, http.StatusBadGateway, "handoff_unavailable"},
	} {
		_, transfer := tiktokHandoffTestEnv(t, 15000)
		transfer.err = tc.err
		token := issueHandoff(t, 42)
		status, code, _ := redeemHandoff(t, token, 42)
		assert.Equal(t, tc.status, status, tc.code)
		assert.Equal(t, tc.code, code)
	}
}

func TestTikTokHandoffIsClosedWithoutRedis(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	common.RedisEnabled = false
	recorder := tiktokToolRequest(t, IssueTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff", `{"id":"`+handoffWorkID+`"}`, nil, 0)
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "handoff_unavailable", tiktokToolCode(recorder))
}

// Several signed-in accounts in the guest's browser race for one guest token:
// every one passes the owner check and holds its own user lock before any
// consumes, so only the atomic consume keeps the token single use.
func TestTikTokHandoffConcurrentRedeemsTransferOnce(t *testing.T) {
	_, transfer := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 0)
	const redeemers = 4
	var arrived sync.WaitGroup
	arrived.Add(redeemers)
	allArrived := make(chan struct{})
	go func() { arrived.Wait(); close(allArrived) }()
	tiktokHandoffBeforeConsume = func() {
		arrived.Done()
		select {
		case <-allArrived:
		case <-time.After(5 * time.Second):
		}
	}

	statuses := make([]int, redeemers)
	var done sync.WaitGroup
	for i := 0; i < redeemers; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			recorder := tiktokToolRequest(t, RedeemTikTokToolHandoff, http.MethodPost, "/api/tools/tiktok/handoff/redeem", `{"token":"`+token+`"}`, nil, 100+i)
			statuses[i] = recorder.Code
		}(i)
	}
	done.Wait()

	succeeded := 0
	for _, status := range statuses {
		if status == http.StatusOK {
			succeeded++
		} else {
			assert.Equal(t, http.StatusGone, status)
		}
	}
	assert.Equal(t, 1, succeeded, "statuses %v", statuses)
	assert.Len(t, transfer.urls, 1, "one token copies one video")
}

func TestTikTokHandoffRedeemIsRateLimitedPerUserWithoutTouchingTheToken(t *testing.T) {
	_, transfer := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 42)
	missing := strings.Repeat("A", 43)
	for i := 0; i < tiktokHandoffRedeemUserLimit; i++ {
		status, code, _ := redeemHandoff(t, missing, 42)
		require.Equal(t, http.StatusGone, status, code)
	}
	status, code, _ := redeemHandoff(t, token, 42)
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "rate_limited", code)
	assert.Empty(t, transfer.urls)

	status, code, _ = redeemHandoff(t, missing, 43)
	assert.Equal(t, http.StatusGone, status, "another account on the IP keeps its own allowance: %s", code)
}

func TestTikTokHandoffRedeemIsRateLimitedPerIP(t *testing.T) {
	tiktokHandoffTestEnv(t, 15000)
	missing := strings.Repeat("A", 43)
	for i := 0; i < tiktokHandoffRedeemIPLimit; i++ {
		status, code, _ := redeemHandoff(t, missing, 1000+i)
		require.Equal(t, http.StatusGone, status, code)
	}
	status, code, _ := redeemHandoff(t, missing, 5000)
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "rate_limited", code)
}

func TestTikTokHandoffRedeemHoldsACrossInstanceLockPerUser(t *testing.T) {
	server, transfer := tiktokHandoffTestEnv(t, 15000)
	token := issueHandoff(t, 42)
	lockKey := tiktokHandoffLockPrefix + strconv.Itoa(42)
	// Another instance is transferring for this user.
	require.NoError(t, server.Set(lockKey, "other-instance"))
	server.SetTTL(lockKey, tiktokHandoffLockTTL)

	status, code, _ := redeemHandoff(t, token, 42)
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "busy", code)
	assert.Empty(t, transfer.urls)
	held, err := server.Get(lockKey)
	require.NoError(t, err)
	assert.Equal(t, "other-instance", held, "a refused request does not release another holder's lock")

	// The other instance died: its lock expires on its own.
	server.FastForward(tiktokHandoffLockTTL + time.Second)
	var lockTTL time.Duration
	tiktokHandoffBeforeConsume = func() { lockTTL = server.TTL(lockKey) }
	status, _, body := redeemHandoff(t, token, 42)
	require.Equal(t, http.StatusOK, status, "the token survived the busy refusal: %s", body)
	assert.Equal(t, tiktokHandoffLockTTL, lockTTL, "the transfer lock carries a TTL")
	assert.False(t, server.Exists(lockKey), "the holder releases its lock")
}
