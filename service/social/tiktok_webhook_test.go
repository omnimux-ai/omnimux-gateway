package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWebhookConfig() Config {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return Config{
		Enabled:                       true,
		Platforms:                     []string{"tiktok"},
		TikTokDirectEnabled:           true,
		TikTokClientKey:               "tt-client-key",
		TikTokClientSecret:            "tt-client-secret-top-secret",
		TikTokRedirectURI:             defaultTikTokRedirectURI,
		TikTokScopes:                  defaultTikTokScopes,
		TikTokWebhookEnabled:          true,
		TikTokWebhookToleranceSeconds: 300,
		SocialTokenEncKey:             base64.StdEncoding.EncodeToString(key),
	}
}

func signTikTokWebhook(secret string, ts int64, rawBody []byte) string {
	tStr := fmt.Sprintf("%d", ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tStr))
	mac.Write([]byte{'.'})
	mac.Write(rawBody)
	return fmt.Sprintf("t=%s,s=%s", tStr, hex.EncodeToString(mac.Sum(nil)))
}

func makeTikTokWebhookPayload(t *testing.T, clientKey, event, openID string, createTime int64, contentJSON string) []byte {
	t.Helper()
	b, err := common.Marshal(map[string]any{
		"client_key":  clientKey,
		"event":       event,
		"create_time": createTime,
		"user_openid": openID,
		"content":     contentJSON,
	})
	require.NoError(t, err)
	return b
}

func TestTikTokWebhookFailClosedGates(t *testing.T) {
	now := time.Unix(1700000500, 0)
	base := testWebhookConfig()
	body := makeTikTokWebhookPayload(t, base.TikTokClientKey, TikTokWebhookEventComplete, "oid-1", now.Unix(), `{"publish_id":"pub-1"}`)
	sig := signTikTokWebhook(base.TikTokClientSecret, now.Unix(), body)

	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"social ops disabled", func(c *Config) { c.Enabled = false }},
		{"tiktok direct disabled", func(c *Config) { c.TikTokDirectEnabled = false }},
		{"rollback to zernio active", func(c *Config) { c.RollbackTikTokToZernio = true }},
		{"tiktok webhook disabled", func(c *Config) { c.TikTokWebhookEnabled = false }},
		{"missing client_key", func(c *Config) { c.TikTokClientKey = "" }},
		{"missing client_secret", func(c *Config) { c.TikTokClientSecret = "" }},
		{"sandbox enabled without sandbox secret", func(c *Config) {
			c.TikTokSandboxEnabled = true
			c.TikTokSandboxClientKey = "sbx-k"
			c.TikTokSandboxClientSecret = ""
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			svc := NewService(cfg)
			assert.False(t, svc.TikTokWebhookReady())
			_, err := svc.handleTikTokWebhookAt(context.Background(), []string{sig}, body, now)
			assert.ErrorIs(t, err, ErrTikTokWebhookDisabled)
		})
	}
}

func TestTikTokWebhookAuthVerificationAndOrder(t *testing.T) {
	clearSocialTables(t)
	now := time.Unix(1700001000, 0)
	cfg := testWebhookConfig()
	svc := NewService(cfg)

	validBody := makeTikTokWebhookPayload(t, cfg.TikTokClientKey, "test.ping", "oid-1", now.Unix(), `{}`)
	validSig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), validBody)

	t.Run("missing or empty header", func(t *testing.T) {
		for _, hdrs := range [][]string{nil, {}, {""}, {"   "}} {
			_, err := svc.handleTikTokWebhookAt(context.Background(), hdrs, validBody, now)
			var authErr *TikTokWebhookAuthError
			require.ErrorAs(t, err, &authErr)
			assert.Equal(t, TikTokWebhookRejectMissingHeader, authErr.Reason)
		}
	})

	t.Run("malformed or duplicate signature header", func(t *testing.T) {
		badHeaders := [][]string{
			{validSig, validSig},
			{"t=1700001000"},
			{"s=" + hex.EncodeToString(make([]byte, 32))},
			{"t=1700001000,t=1700001000,s=" + hex.EncodeToString(make([]byte, 32))},
			{validSig + ",s=" + hex.EncodeToString(make([]byte, 32))},
			{validSig + ",v=1"},
			{"t=+1700001000,s=" + hex.EncodeToString(make([]byte, 32))},
			{"t=-1700001000,s=" + hex.EncodeToString(make([]byte, 32))},
			{"t=0,s=" + hex.EncodeToString(make([]byte, 32))},
			{"t=notnum,s=" + hex.EncodeToString(make([]byte, 32))},
			{"t=1700001000,s=deadbeef"},
			{"t=1700001000,s=" + "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
			{"garbage"},
		}
		for _, hdrs := range badHeaders {
			_, err := svc.handleTikTokWebhookAt(context.Background(), hdrs, validBody, now)
			var authErr *TikTokWebhookAuthError
			require.ErrorAs(t, err, &authErr, "headers=%v", hdrs)
			assert.Equal(t, TikTokWebhookRejectMalformedSignature, authErr.Reason, "headers=%v", hdrs)
		}
	})

	t.Run("bad HMAC checked before timestamp and JSON parsing", func(t *testing.T) {
		wrongSig := signTikTokWebhook("wrong-secret", now.Unix()-9999, []byte("{not-json"))
		_, err := svc.handleTikTokWebhookAt(context.Background(), []string{wrongSig}, []byte("{not-json"), now)
		var authErr *TikTokWebhookAuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectBadHMAC, authErr.Reason)
	})

	t.Run("HMAC verifies exact raw bytes without JSON re-serialization", func(t *testing.T) {
		// Unformatted JSON with spaces and non-sorted keys.
		raw := []byte(`{  "event" : "test.ping" , "client_key" : "tt-client-key" , "create_time" : 1700001000 }`)
		sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), raw)
		res, err := svc.handleTikTokWebhookAt(context.Background(), []string{sig}, raw, now)
		require.NoError(t, err)
		assert.Equal(t, "ignored_event", res.Action)

		// Single space mutated breaks HMAC.
		tampered := []byte(`{ "event" : "test.ping" , "client_key" : "tt-client-key" , "create_time" : 1700001000 }`)
		_, err = svc.handleTikTokWebhookAt(context.Background(), []string{sig}, tampered, now)
		var authErr *TikTokWebhookAuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectBadHMAC, authErr.Reason)
	})

	t.Run("timestamp window boundary checks past<=tolerance and future<=60s", func(t *testing.T) {
		// Exact past tolerance (300s) allowed.
		sigAtPastBound := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix()-300, validBody)
		res, err := svc.handleTikTokWebhookAt(context.Background(), []string{sigAtPastBound}, validBody, now)
		require.NoError(t, err)
		assert.Equal(t, "ignored_event", res.Action)

		// Past tolerance + 1s (301s) rejected.
		sigTooOld := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix()-301, validBody)
		_, err = svc.handleTikTokWebhookAt(context.Background(), []string{sigTooOld}, validBody, now)
		var authErr *TikTokWebhookAuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectTimestampOutOfWindow, authErr.Reason)
		assert.Equal(t, int64(301), authErr.SigAgeSeconds)
		assert.Equal(t, int64(300), authErr.ToleranceSeconds)

		// Exact future skew (60s) allowed.
		sigAtFutureBound := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix()+60, validBody)
		res, err = svc.handleTikTokWebhookAt(context.Background(), []string{sigAtFutureBound}, validBody, now)
		require.NoError(t, err)
		assert.Equal(t, "ignored_event", res.Action)

		// Future skew + 1s (61s) rejected.
		sigTooFuture := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix()+61, validBody)
		_, err = svc.handleTikTokWebhookAt(context.Background(), []string{sigTooFuture}, validBody, now)
		authErr = nil
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectTimestampOutOfWindow, authErr.Reason)
		assert.Equal(t, int64(-61), authErr.SigAgeSeconds)
	})

	t.Run("valid signature with invalid JSON returns ErrTikTokWebhookBadPayload", func(t *testing.T) {
		for _, bad := range [][]byte{[]byte(""), []byte("not-json"), []byte("[]"), []byte("null"), []byte(`{"client_key":`)} {
			sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), bad)
			_, err := svc.handleTikTokWebhookAt(context.Background(), []string{sig}, bad, now)
			assert.ErrorIs(t, err, ErrTikTokWebhookBadPayload, "body=%q", string(bad))
		}
	})

	t.Run("client_key mismatch rejected as client_key_mismatch", func(t *testing.T) {
		wrongKeyBody := makeTikTokWebhookPayload(t, "other-client-key", TikTokWebhookEventComplete, "oid-1", now.Unix(), `{"publish_id":"pub-1"}`)
		sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), wrongKeyBody)
		_, err := svc.handleTikTokWebhookAt(context.Background(), []string{sig}, wrongKeyBody, now)
		var authErr *TikTokWebhookAuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectClientKeyMismatch, authErr.Reason)
	})

	t.Run("sandbox mode uses sandbox credentials only", func(t *testing.T) {
		sbxCfg := testWebhookConfig()
		sbxCfg.TikTokSandboxEnabled = true
		sbxCfg.TikTokSandboxClientKey = "sbx-key"
		sbxCfg.TikTokSandboxClientSecret = "sbx-secret"
		sbxSvc := NewService(sbxCfg)

		// Prod signature rejected in sandbox mode.
		prodBody := makeTikTokWebhookPayload(t, sbxCfg.TikTokClientKey, "test.ping", "oid-1", now.Unix(), `{}`)
		prodSig := signTikTokWebhook(sbxCfg.TikTokClientSecret, now.Unix(), prodBody)
		_, err := sbxSvc.handleTikTokWebhookAt(context.Background(), []string{prodSig}, prodBody, now)
		var authErr *TikTokWebhookAuthError
		require.ErrorAs(t, err, &authErr)
		assert.Equal(t, TikTokWebhookRejectBadHMAC, authErr.Reason)

		// Sandbox signature succeeds.
		sbxBody := makeTikTokWebhookPayload(t, sbxCfg.TikTokSandboxClientKey, "test.ping", "oid-1", now.Unix(), `{}`)
		sbxSig := signTikTokWebhook(sbxCfg.TikTokSandboxClientSecret, now.Unix(), sbxBody)
		res, err := sbxSvc.handleTikTokWebhookAt(context.Background(), []string{sbxSig}, sbxBody, now)
		require.NoError(t, err)
		assert.Equal(t, "ignored_event", res.Action)
	})
}

func TestTikTokWebhookLifecycleEventsAndRedTeamScenarios(t *testing.T) {
	clearSocialTables(t)
	now := time.Unix(1700002000, 0)
	cfg := testWebhookConfig()
	svc := NewService(cfg)

	acc := &model.SocialAccount{
		UserId:            201,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-owner-1",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(acc).Error)

	deliver := func(t *testing.T, event, openID string, createTime int64, contentJSON string) (*TikTokWebhookResult, error) {
		t.Helper()
		body := makeTikTokWebhookPayload(t, cfg.TikTokClientKey, event, openID, createTime, contentJSON)
		sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), body)
		return svc.handleTikTokWebhookAt(context.Background(), []string{sig}, body, now)
	}

	t.Run("inbox_delivered -> publicly_available (number+string post_id) -> delayed complete -> multi-post takedown -> deleted absorbing", func(t *testing.T) {
		post := &model.SocialPost{
			UserId:          201,
			Provider:        ProviderTikTokDirect,
			ProviderPostID:  "pub-life-1",
			Status:          model.SocialPostStatusSubmitted,
			RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
		}
		require.NoError(t, model.CreateSocialPost(post))

		// 1) inbox_delivered: submitted -> sent_to_inbox
		res, err := deliver(t, TikTokWebhookEventInboxDelivered, "oid-owner-1", now.Unix()-50, `{"publish_id":"pub-life-1","publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusSentToInbox, res.Status)

		// 2) publicly_available BEFORE complete with 64-bit numeric post_id (precision preserved!)
		res, err = deliver(t, TikTokWebhookEventPubliclyAvailable, "oid-owner-1", now.Unix()-40, `{"publish_id":"pub-life-1","post_id":7210402925882379563,"publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)

		// 3) Second post_id as string in multi-post upload
		res, err = deliver(t, TikTokWebhookEventPubliclyAvailable, "oid-owner-1", now.Unix()-35, `{"publish_id":"pub-life-1","post_id":"7210402925882379564","publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)

		// 4) Delayed post.publish.complete MUST NOT wipe post_ids
		res, err = deliver(t, TikTokWebhookEventComplete, "oid-owner-1", now.Unix()-45, `{"publish_id":"pub-life-1","publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)
		got, err := model.GetSocialPostByIDForUser(post.ID, 201)
		require.NoError(t, err)
		var snap map[string]any
		require.NoError(t, common.Unmarshal([]byte(got.ResultSnapshot), &snap))
		assert.Equal(t, []any{"7210402925882379563", "7210402925882379564"}, snap["post_ids"])
		assert.Equal(t, true, snap["publish_complete"])

		// 5) Late inbox_delivered and failed MUST NOT overwrite published
		res, err = deliver(t, TikTokWebhookEventInboxDelivered, "oid-owner-1", now.Unix()-30, `{"publish_id":"pub-life-1","publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "noop", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)

		res, err = deliver(t, TikTokWebhookEventFailed, "oid-owner-1", now.Unix()-25, `{"publish_id":"pub-life-1","reason":"late_fail"}`)
		require.NoError(t, err)
		assert.Equal(t, "noop", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)

		// 6) Official single-"l" no_longer_publicaly_available for post 1: post 2 survives -> stays published
		res, err = deliver(t, TikTokWebhookEventNoLongerPublicaly, "oid-owner-1", now.Unix()-20, `{"publish_id":"pub-life-1","post_id":7210402925882379563,"publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)

		// 7) Defensive double-"l" no_longer_publicly_available for post 2: no surviving posts -> deleted
		res, err = deliver(t, TikTokWebhookEventNoLongerPublicly, "oid-owner-1", now.Unix()-10, `{"publish_id":"pub-life-1","post_id":"7210402925882379564","publish_type":"INBOX_UPLOAD"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusDeleted, res.Status)

		// 8) deleted is strictly absorbing
		res, err = deliver(t, TikTokWebhookEventPubliclyAvailable, "oid-owner-1", now.Unix()-5, `{"publish_id":"pub-life-1","post_id":"7210402925882379565"}`)
		require.NoError(t, err)
		assert.Equal(t, "noop", res.Action)
		assert.Equal(t, model.SocialPostStatusDeleted, res.Status)
	})

	t.Run("failed transition stores reason and is irreversible", func(t *testing.T) {
		post := &model.SocialPost{
			UserId:          201,
			Provider:        ProviderTikTokDirect,
			ProviderPostID:  "pub-fail-1",
			Status:          model.SocialPostStatusProcessing,
			RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
		}
		require.NoError(t, model.CreateSocialPost(post))

		res, err := deliver(t, TikTokWebhookEventFailed, "oid-owner-1", now.Unix(), `{"publish_id":"pub-fail-1","reason":"file_format_check_failed","publish_type":"DIRECT_POST"}`)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusFailed, res.Status)

		got, err := model.GetSocialPostByIDForUser(post.ID, 201)
		require.NoError(t, err)
		assert.Contains(t, got.ResultSnapshot, `"reason":"file_format_check_failed"`)
		assert.Contains(t, got.ResultSnapshot, `"fail_reason":"file_format_check_failed"`)

		// Cannot transition out of failed.
		res, err = deliver(t, TikTokWebhookEventComplete, "oid-owner-1", now.Unix()+1, `{"publish_id":"pub-fail-1"}`)
		require.NoError(t, err)
		assert.Equal(t, "noop", res.Action)
		assert.Equal(t, model.SocialPostStatusFailed, res.Status)
	})

	t.Run("user_openid mismatch, unknown publish_id, and object content handling", func(t *testing.T) {
		post := &model.SocialPost{
			UserId:          201,
			Provider:        ProviderTikTokDirect,
			ProviderPostID:  "pub-guard-1",
			Status:          model.SocialPostStatusSubmitted,
			RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
		}
		require.NoError(t, model.CreateSocialPost(post))

		// Mismatched user_openid ignored without mutating DB.
		res, err := deliver(t, TikTokWebhookEventComplete, "oid-different-user", now.Unix(), `{"publish_id":"pub-guard-1"}`)
		require.NoError(t, err)
		assert.Equal(t, "ignored_openid_mismatch", res.Action)
		unchanged, err := model.GetSocialPostByIDForUser(post.ID, 201)
		require.NoError(t, err)
		assert.Equal(t, model.SocialPostStatusSubmitted, unchanged.Status)

		// Unknown publish_id ignored cleanly.
		res, err = deliver(t, TikTokWebhookEventComplete, "oid-owner-1", now.Unix(), `{"publish_id":"pub-nonexistent"}`)
		require.NoError(t, err)
		assert.Equal(t, "ignored_unknown_post", res.Action)

		// Missing publish_id ignored cleanly.
		res, err = deliver(t, TikTokWebhookEventComplete, "oid-owner-1", now.Unix(), `{}`)
		require.NoError(t, err)
		assert.Equal(t, "ignored_missing_publish_id", res.Action)

		// Malformed inner content JSON returns ErrTikTokWebhookBadPayload (400).
		_, err = deliver(t, TikTokWebhookEventComplete, "oid-owner-1", now.Unix(), `{not-json`)
		assert.ErrorIs(t, err, ErrTikTokWebhookBadPayload)

		// Defensive object-shaped content also accepted.
		objBody, err := common.Marshal(map[string]any{
			"client_key":  cfg.TikTokClientKey,
			"event":       TikTokWebhookEventComplete,
			"create_time": now.Unix(),
			"user_openid": "oid-owner-1",
			"content":     map[string]any{"publish_id": "pub-guard-1", "publish_type": "DIRECT_POST"},
		})
		require.NoError(t, err)
		sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), objBody)
		res, err = svc.handleTikTokWebhookAt(context.Background(), []string{sig}, objBody, now)
		require.NoError(t, err)
		assert.Equal(t, "updated", res.Action)
		assert.Equal(t, model.SocialPostStatusPublished, res.Status)
	})
}

func TestRefreshTikTokPostStatusCoGovernanceWithWebhook(t *testing.T) {
	clearSocialTables(t)
	now := time.Unix(1700003000, 0)

	// Stub TikTok status/fetch returning a stale PROCESSING_UPLOAD or PUBLISH_COMPLETE response.
	upstreamStatus := "PROCESSING_UPLOAD"
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/post/publish/status/fetch/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, _ := common.Marshal(map[string]any{
			"data": map[string]any{
				"status":      upstreamStatus,
				"fail_reason": "",
				"publish_id":  "pub-race-1",
			},
			"error": map[string]any{"code": "ok", "message": ""},
		})
		_, _ = w.Write(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testWebhookConfig()
	cfg.TikTokBaseURL = srv.URL
	svc := NewService(cfg)

	enc, err := SealToken("at-race", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	acc := &model.SocialAccount{
		UserId:               202,
		Provider:             ProviderTikTokDirect,
		ProviderAccountID:    "oid-race-1",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: enc,
		ScopesGranted:        "user.info.basic,video.upload",
	}
	require.NoError(t, model.DB.Create(acc).Error)

	post := &model.SocialPost{
		UserId:          202,
		Provider:        ProviderTikTokDirect,
		ProviderPostID:  "pub-race-1",
		Status:          model.SocialPostStatusSubmitted,
		RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
	}
	require.NoError(t, model.CreateSocialPost(post))

	// Fast webhook advances row to published with post_ids.
	body := makeTikTokWebhookPayload(t, cfg.TikTokClientKey, TikTokWebhookEventPubliclyAvailable, "oid-race-1", now.Unix(), `{"publish_id":"pub-race-1","post_id":"7210402925882379777"}`)
	sig := signTikTokWebhook(cfg.TikTokClientSecret, now.Unix(), body)
	_, err = svc.handleTikTokWebhookAt(context.Background(), []string{sig}, body, now)
	require.NoError(t, err)

	// Slow poll that captured an older in-memory copy of `post` (still status=submitted) runs refreshTikTokPostStatus.
	for _, stale := range []string{"PROCESSING_UPLOAD", "SEND_TO_USER_INBOX", "PUBLISH_COMPLETE", "FAILED"} {
		upstreamStatus = stale
		svc.refreshTikTokPostStatus(context.Background(), post)

		got, err := model.GetSocialPostByIDForUser(post.ID, 202)
		require.NoError(t, err)
		assert.Equal(t, model.SocialPostStatusPublished, got.Status, "poll %s must not overwrite published", stale)
		assert.Contains(t, got.ResultSnapshot, `"7210402925882379777"`, "poll %s must preserve webhook post_ids", stale)
	}
}
