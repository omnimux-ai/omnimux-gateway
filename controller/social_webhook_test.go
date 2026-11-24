package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failIfReadBody struct {
	readCalled bool
}

func (b *failIfReadBody) Read(_ []byte) (int, error) {
	b.readCalled = true
	return 0, errors.New("body must not be read when webhook is disabled")
}

func (b *failIfReadBody) Close() error { return nil }

func signControllerTikTokWebhook(secret string, ts int64, rawBody []byte) string {
	tStr := fmt.Sprintf("%d", ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tStr))
	mac.Write([]byte{'.'})
	mac.Write(rawBody)
	return fmt.Sprintf("t=%s,s=%s", tStr, hex.EncodeToString(mac.Sum(nil)))
}

func setupSocialWebhookControllerTest(t *testing.T, cfg social.Config) (*gorm.DB, *gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.SocialAccount{}, &model.SocialPost{}))

	oldDB, oldSvc := model.DB, socialSvc
	oldDBType := common.MainDatabaseType()
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	setSocialService(social.NewService(cfg))

	var logBuf bytes.Buffer
	common.LogWriterMu.Lock()
	oldErrWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuf
	common.LogWriterMu.Unlock()

	t.Cleanup(func() {
		common.SetLogLanguage("en")
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldErrWriter
		common.LogWriterMu.Unlock()
		model.DB = oldDB
		common.SetMainDatabaseType(oldDBType)
		setSocialService(oldSvc)
		_ = sqlDB.Close()
	})

	r := gin.New()
	r.POST("/api/social/v1/webhooks/tiktok", SocialTikTokWebhook)
	return db, r, &logBuf
}

func webhookTestConfig() social.Config {
	return social.Config{
		Enabled:                       true,
		Platforms:                     []string{"tiktok"},
		TikTokDirectEnabled:           true,
		TikTokClientKey:               "ctrl-client-key",
		TikTokClientSecret:            "ctrl-super-secret-do-not-log",
		TikTokRedirectURI:             "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		TikTokWebhookEnabled:          true,
		TikTokWebhookToleranceSeconds: 300,
		SocialTokenEncKey:             base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}
}

func TestSocialTikTokWebhookDisabledFailsClosedBeforeBodyRead(t *testing.T) {
	cfg := webhookTestConfig()
	cfg.TikTokWebhookEnabled = false
	_, r, logBuf := setupSocialWebhookControllerTest(t, cfg)

	body := &failIfReadBody{}
	req := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", io.NopCloser(body))
	req.Header.Set("TikTok-Signature", "t=1700000000,s="+strings.Repeat("a", 64))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, body.readCalled, "handler must reject 403 before reading body")
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, logBuf.String(), "reason=webhook_disabled")
	assert.NotContains(t, logBuf.String(), cfg.TikTokClientSecret)
}

func TestSocialTikTokWebhookPayloadTooLargeReturns413(t *testing.T) {
	cfg := webhookTestConfig()
	_, r, logBuf := setupSocialWebhookControllerTest(t, cfg)

	// 64 KiB + 1 byte MUST return 413 Payload Too Large, never truncate to 401.
	oversized := bytes.Repeat([]byte("a"), maxTikTokWebhookBodyBytes+1)
	sig := signControllerTikTokWebhook(cfg.TikTokClientSecret, time.Now().Unix(), oversized)
	req := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(oversized))
	req.Header.Set("TikTok-Signature", sig)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, logBuf.String(), "reason=payload_too_large")
	assert.NotContains(t, logBuf.String(), sig)
	assert.NotContains(t, logBuf.String(), cfg.TikTokClientSecret)

	// Exact 64 KiB signed JSON payload is accepted (200 OK).
	prefix := fmt.Sprintf(`{"client_key":%q,"event":"test.ping","create_time":%d,"pad":"`, cfg.TikTokClientKey, time.Now().Unix())
	suffix := `"}`
	padLen := maxTikTokWebhookBodyBytes - len(prefix) - len(suffix)
	require.Greater(t, padLen, 0)
	exactBody := []byte(prefix + strings.Repeat("x", padLen) + suffix)
	require.Equal(t, maxTikTokWebhookBodyBytes, len(exactBody))
	exactSig := signControllerTikTokWebhook(cfg.TikTokClientSecret, time.Now().Unix(), exactBody)
	req2 := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(exactBody))
	req2.Header.Set("TikTok-Signature", exactSig)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
}

func TestSocialTikTokWebhookAuthFailuresUniform401AndZeroLeakLogs(t *testing.T) {
	cfg := webhookTestConfig()
	_, r, logBuf := setupSocialWebhookControllerTest(t, cfg)

	now := time.Now().Unix()
	bodySentinel := "RAW_BODY_SENTINEL_987654321"
	validPayload := []byte(fmt.Sprintf(`{"client_key":%q,"event":"test.ping","create_time":%d,"note":%q}`, cfg.TikTokClientKey, now, bodySentinel))
	wrongKeyPayload := []byte(fmt.Sprintf(`{"client_key":"wrong-key","event":"test.ping","create_time":%d,"note":%q}`, now, bodySentinel))

	cases := []struct {
		name           string
		headers        []string
		body           []byte
		wantLogSnippet string
	}{
		{
			name:           "missing_header",
			headers:        nil,
			body:           validPayload,
			wantLogSnippet: "reason=missing_header",
		},
		{
			name:           "malformed_signature",
			headers:        []string{"t=bad,s=nothex"},
			body:           validPayload,
			wantLogSnippet: "reason=malformed_signature",
		},
		{
			name:           "bad_hmac",
			headers:        []string{signControllerTikTokWebhook("wrong-secret", now, validPayload)},
			body:           validPayload,
			wantLogSnippet: "reason=bad_hmac",
		},
		{
			name:           "timestamp_out_of_window",
			headers:        []string{signControllerTikTokWebhook(cfg.TikTokClientSecret, now-600, validPayload)},
			body:           validPayload,
			wantLogSnippet: "reason=timestamp_out_of_window sig_age_seconds=",
		},
		{
			name:           "client_key_mismatch",
			headers:        []string{signControllerTikTokWebhook(cfg.TikTokClientSecret, now, wrongKeyPayload)},
			body:           wrongKeyPayload,
			wantLogSnippet: "reason=client_key_mismatch",
		},
	}

	var firstBody string
	for _, lang := range []string{"en", "zh-CN"} {
		common.SetLogLanguage(lang)
		for _, tc := range cases {
			t.Run(lang+"/"+tc.name, func(t *testing.T) {
				logBuf.Reset()
				req := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok?leak=query_secret", bytes.NewReader(tc.body))
				for _, h := range tc.headers {
					req.Header.Add("TikTok-Signature", h)
				}
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
				assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
				if firstBody == "" {
					firstBody = rec.Body.String()
				} else {
					assert.Equal(t, firstBody, rec.Body.String(), "all auth rejections must return identical 401 body")
				}

				logged := logBuf.String()
				assert.Contains(t, logged, tc.wantLogSnippet)
				if tc.name == "timestamp_out_of_window" {
					assert.Contains(t, logged, "tolerance_seconds=300")
				}
				if lang == "zh-CN" {
					assert.Contains(t, logged, "TikTok Webhook 鉴权失败")
				}
				assert.NotContains(t, logged, cfg.TikTokClientSecret)
				assert.NotContains(t, logged, bodySentinel)
				assert.NotContains(t, logged, "query_secret")
				for _, h := range tc.headers {
					assert.NotContains(t, logged, h)
				}
			})
		}
	}
}

func TestSocialTikTokWebhookEndToEndAndErrorMapping(t *testing.T) {
	cfg := webhookTestConfig()
	db, r, _ := setupSocialWebhookControllerTest(t, cfg)

	acc := &model.SocialAccount{
		UserId:            77,
		Provider:          social.ProviderTikTokDirect,
		ProviderAccountID: "oid-ctrl-77",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, db.Create(acc).Error)
	post := &model.SocialPost{
		UserId:          77,
		Provider:        social.ProviderTikTokDirect,
		ProviderPostID:  "pub-ctrl-1",
		Status:          model.SocialPostStatusSubmitted,
		RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
	}
	require.NoError(t, db.Create(post).Error)

	// 1) Valid signature + malformed JSON -> 400 Bad Request
	badJSON := []byte("{bad-json")
	reqBad := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(badJSON))
	reqBad.Header.Set("TikTok-Signature", signControllerTikTokWebhook(cfg.TikTokClientSecret, time.Now().Unix(), badJSON))
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, recBad.Code)
	assert.Equal(t, "no-store", recBad.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recBad.Header().Get("X-Content-Type-Options"))

	// 2) Valid signed publicly_available -> 200 OK and DB updated
	okBody, err := common.Marshal(map[string]any{
		"client_key":  cfg.TikTokClientKey,
		"event":       social.TikTokWebhookEventPubliclyAvailable,
		"create_time": time.Now().Unix(),
		"user_openid": "oid-ctrl-77",
		"content":     `{"publish_id":"pub-ctrl-1","post_id":"7210402925882379563","publish_type":"DIRECT_POST"}`,
	})
	require.NoError(t, err)
	reqOK := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(okBody))
	reqOK.Header.Set("TikTok-Signature", signControllerTikTokWebhook(cfg.TikTokClientSecret, time.Now().Unix(), okBody))
	recOK := httptest.NewRecorder()
	r.ServeHTTP(recOK, reqOK)
	assert.Equal(t, http.StatusOK, recOK.Code, recOK.Body.String())
	assert.Equal(t, "no-store", recOK.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recOK.Header().Get("X-Content-Type-Options"))

	var saved model.SocialPost
	require.NoError(t, db.First(&saved, post.ID).Error)
	assert.Equal(t, model.SocialPostStatusPublished, saved.Status)
	assert.Contains(t, saved.ResultSnapshot, `"7210402925882379563"`)

	// 3) Real DB error -> 500 Internal Server Error without leaking SQL details.
	require.NoError(t, db.Migrator().DropTable(&model.SocialPost{}))
	reqErr := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(okBody))
	reqErr.Header.Set("TikTok-Signature", signControllerTikTokWebhook(cfg.TikTokClientSecret, time.Now().Unix(), okBody))
	recErr := httptest.NewRecorder()
	r.ServeHTTP(recErr, reqErr)
	assert.Equal(t, http.StatusInternalServerError, recErr.Code)
	assert.NotContains(t, recErr.Body.String(), "social_posts")
	assert.Equal(t, "no-store", recErr.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recErr.Header().Get("X-Content-Type-Options"))
}
