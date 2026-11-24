package controller

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social"
	"github.com/QuantumNous/new-api/service/social/tiktok"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func assertNoLegacyQuotaClassifierPhrases(t *testing.T, msg string) {
	t.Helper()
	lower := strings.ToLower(msg)
	assert.NotContains(t, lower, "quota exceeded")
	assert.NotContains(t, lower, "usage limit reached")
	assert.NotContains(t, lower, "insufficient quota")
}

func invokeSocialError(t *testing.T, err error) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/social/v1/posts", nil)
	socialError(ctx, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body), "response body must be valid JSON: %s", rec.Body.String())
	return rec, body
}

func TestSocialErrorTikTokMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name             string
		err              error
		wantStatus       int
		wantCode         string
		wantUpstreamCode string
		wantLogID        string
	}{
		{
			name: "rate_limit_exceeded on HTTP 200 returns 429",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "rate_limit_exceeded",
				Message:  "Too many requests; slow down",
				LogID:    "20261124-rl-1",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusTooManyRequests,
			wantCode:         "SOCIAL_TIKTOK_RATE_LIMITED",
			wantUpstreamCode: "rate_limit_exceeded",
			wantLogID:        "20261124-rl-1",
		},
		{
			name: "upstream HTTP 429 without known code still returns 429",
			err: &tiktok.APIError{
				Status:   http.StatusTooManyRequests,
				Code:     "too_many_requests",
				Message:  "Slow down",
				LogID:    "20261124-rl-2",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusTooManyRequests,
			wantCode:         "SOCIAL_TIKTOK_RATE_LIMITED",
			wantUpstreamCode: "too_many_requests",
			wantLogID:        "20261124-rl-2",
		},
		{
			name: "transient 429 during refresh takes precedence over refresh endpoint",
			err: fmt.Errorf("tiktok token refresh failed: %w", &tiktok.APIError{
				Status:   http.StatusTooManyRequests,
				Code:     "rate_limit_exceeded",
				Message:  "Too many refresh requests",
				LogID:    "20261124-rl-refresh-429",
				Endpoint: "refresh",
			}),
			wantStatus:       http.StatusTooManyRequests,
			wantCode:         "SOCIAL_TIKTOK_RATE_LIMITED",
			wantUpstreamCode: "rate_limit_exceeded",
			wantLogID:        "20261124-rl-refresh-429",
		},
		{
			name: "HTTP 200 rate_limit_exceeded during refresh takes precedence over refresh endpoint",
			err: fmt.Errorf("tiktok token refresh failed: %w", &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "rate_limit_exceeded",
				Message:  "Rate limited",
				LogID:    "20261124-rl-refresh-200",
				Endpoint: "refresh",
			}),
			wantStatus:       http.StatusTooManyRequests,
			wantCode:         "SOCIAL_TIKTOK_RATE_LIMITED",
			wantUpstreamCode: "rate_limit_exceeded",
			wantLogID:        "20261124-rl-refresh-200",
		},
		{
			name: "spam_risk_user_banned_from_posting returns 409 POSTING_BANNED",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "spam_risk_user_banned_from_posting",
				Message:  "User is banned from posting",
				LogID:    "20261124-ban-1",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_POSTING_BANNED",
			wantUpstreamCode: "spam_risk_user_banned_from_posting",
			wantLogID:        "20261124-ban-1",
		},
		{
			name: "spam_risk_too_many_posts returns 409 PUBLISH_RESTRICTED",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "spam_risk_too_many_posts",
				Message:  "User has reached daily posting limit",
				LogID:    "20261124-pub-1",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "spam_risk_too_many_posts",
			wantLogID:        "20261124-pub-1",
		},
		{
			name: "reached_active_user_cap returns 409 PUBLISH_RESTRICTED",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "reached_active_user_cap",
				Message:  "Daily cap reached",
				LogID:    "20261124-pub-2",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "reached_active_user_cap",
			wantLogID:        "20261124-pub-2",
		},
		{
			name: "spam_risk_too_many_pending_share returns 409 PUBLISH_RESTRICTED",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "spam_risk_too_many_pending_share",
				Message:  "Too many pending shares",
				LogID:    "20261124-pub-3",
				Endpoint: "video_inbox_init",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "spam_risk_too_many_pending_share",
			wantLogID:        "20261124-pub-3",
		},
		{
			name: "unaudited_client_can_only_post_to_private_accounts returns 409 PUBLISH_RESTRICTED",
			err: &tiktok.APIError{
				Status:   http.StatusForbidden,
				Code:     "unaudited_client_can_only_post_to_private_accounts",
				Message:  "Client must post to private accounts until audited",
				LogID:    "20261124-pub-4",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "unaudited_client_can_only_post_to_private_accounts",
			wantLogID:        "20261124-pub-4",
		},
		{
			name: "url_ownership_unverified returns 409 PUBLISH_RESTRICTED",
			err: &tiktok.APIError{
				Status:   http.StatusBadRequest,
				Code:     "url_ownership_unverified",
				Message:  "URL domain is not verified",
				LogID:    "20261124-pub-5",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "url_ownership_unverified",
			wantLogID:        "20261124-pub-5",
		},
		{
			name: "scope_not_authorized on upstream 403 returns 409 SCOPE_MISSING (never 403)",
			err: &tiktok.APIError{
				Status:   http.StatusForbidden,
				Code:     "scope_not_authorized",
				Message:  "Scope video.publish is not authorized",
				LogID:    "20261124-scope-1",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_SCOPE_MISSING",
			wantUpstreamCode: "scope_not_authorized",
			wantLogID:        "20261124-scope-1",
		},
		{
			name:       "ErrTikTokTokenExpired returns 409 TOKEN_INVALID",
			err:        social.ErrTikTokTokenExpired,
			wantStatus: http.StatusConflict,
			wantCode:   "SOCIAL_TIKTOK_TOKEN_INVALID",
		},
		{
			name:       "wrapped ErrTikTokTokenExpired returns 409 TOKEN_INVALID",
			err:        fmt.Errorf("account load: %w", social.ErrTikTokTokenExpired),
			wantStatus: http.StatusConflict,
			wantCode:   "SOCIAL_TIKTOK_TOKEN_INVALID",
		},
		{
			name: "access_token_invalid on upstream 401 returns 409 TOKEN_INVALID (never 401)",
			err: &tiktok.APIError{
				Status:   http.StatusUnauthorized,
				Code:     "access_token_invalid",
				Message:  "The access token is invalid",
				LogID:    "20261124-tok-1",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "access_token_invalid",
			wantLogID:        "20261124-tok-1",
		},
		{
			name: "token_expired returns 409 TOKEN_INVALID",
			err: &tiktok.APIError{
				Status:   http.StatusUnauthorized,
				Code:     "token_expired",
				Message:  "Access token expired",
				LogID:    "20261124-tok-2",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "token_expired",
			wantLogID:        "20261124-tok-2",
		},
		{
			name: "invalid_token returns 409 TOKEN_INVALID",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "invalid_token",
				Message:  "Invalid token",
				LogID:    "20261124-tok-3",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "invalid_token",
			wantLogID:        "20261124-tok-3",
		},
		{
			name: "token_revoked returns 409 TOKEN_INVALID",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "token_revoked",
				Message:  "Token revoked by user",
				LogID:    "20261124-tok-4",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "token_revoked",
			wantLogID:        "20261124-tok-4",
		},
		{
			name: "invalid_grant returns 409 TOKEN_INVALID",
			err: &tiktok.APIError{
				Status:   http.StatusBadRequest,
				Code:     "invalid_grant",
				Message:  "Grant is invalid",
				LogID:    "20261124-tok-5",
				Endpoint: "token",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "invalid_grant",
			wantLogID:        "20261124-tok-5",
		},
		{
			name: "refresh_token_invalid returns 409 TOKEN_INVALID",
			err: &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "refresh_token_invalid",
				Message:  "Refresh token invalid",
				LogID:    "20261124-tok-6",
				Endpoint: "refresh",
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "refresh_token_invalid",
			wantLogID:        "20261124-tok-6",
		},
		{
			name: "wrapped refresh endpoint error with generic code returns 409 TOKEN_INVALID",
			err: fmt.Errorf("tiktok token refresh failed: %w", &tiktok.APIError{
				Status:   http.StatusBadRequest,
				Code:     "invalid_request",
				Message:  "Refresh rejected",
				LogID:    "20261124-tok-7",
				Endpoint: "refresh",
			}),
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "invalid_request",
			wantLogID:        "20261124-tok-7",
		},
		{
			name: "unclassified upstream 401 returns 502 UPSTREAM_ERROR (never 401)",
			err: &tiktok.APIError{
				Status:   http.StatusUnauthorized,
				Code:     "unknown_auth_failure",
				Message:  "Upstream rejected request",
				LogID:    "20261124-up-401",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusBadGateway,
			wantCode:         "SOCIAL_TIKTOK_UPSTREAM_ERROR",
			wantUpstreamCode: "unknown_auth_failure",
			wantLogID:        "20261124-up-401",
		},
		{
			name: "unclassified upstream 403 returns 502 UPSTREAM_ERROR (never 403)",
			err: &tiktok.APIError{
				Status:   http.StatusForbidden,
				Code:     "unknown_forbidden_failure",
				Message:  "Upstream denied request",
				LogID:    "20261124-up-403",
				Endpoint: "creator_info",
			},
			wantStatus:       http.StatusBadGateway,
			wantCode:         "SOCIAL_TIKTOK_UPSTREAM_ERROR",
			wantUpstreamCode: "unknown_forbidden_failure",
			wantLogID:        "20261124-up-403",
		},
		{
			name: "unclassified upstream 500 with empty message returns 502 UPSTREAM_ERROR",
			err: &tiktok.APIError{
				Status:   http.StatusInternalServerError,
				Code:     "internal_error",
				Message:  "",
				LogID:    "20261124-up-500",
				Endpoint: "publish",
			},
			wantStatus:       http.StatusBadGateway,
			wantCode:         "SOCIAL_TIKTOK_UPSTREAM_ERROR",
			wantUpstreamCode: "internal_error",
			wantLogID:        "20261124-up-500",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := invokeSocialError(t, tc.err)
			assert.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
			assert.NotEqual(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, false, body["success"])
			assert.Equal(t, tc.wantCode, body["code"])
			msg, ok := body["message"].(string)
			require.True(t, ok)
			assert.NotEmpty(t, msg)
			assertNoLegacyQuotaClassifierPhrases(t, msg)

			if tc.wantUpstreamCode != "" {
				assert.Equal(t, tc.wantUpstreamCode, body["upstream_code"])
			} else {
				_, exists := body["upstream_code"]
				assert.False(t, exists, "upstream_code should be omitted")
			}
			if tc.wantLogID != "" {
				assert.Equal(t, tc.wantLogID, body["upstream_log_id"])
			} else {
				_, exists := body["upstream_log_id"]
				assert.False(t, exists, "upstream_log_id should be omitted")
			}
		})
	}
}

func TestSocialErrorTikTokMessageSanitizationPreservesStructuredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	triggers := []struct {
		label      string
		rawMessage string
		forbidden  string
	}{
		{"quota exceeded mixed case", "Daily Quota Exceeded for this creator", "quota exceeded"},
		{"usage limit reached", "Creator Usage Limit Reached today", "usage limit reached"},
		{"insufficient quota", "INSUFFICIENT QUOTA to post right now", "insufficient quota"},
		{"bearer secret", "Authorization Bearer tok-secret-value rejected", "bearer"},
		{"api key secret", "Upstream rejected API Key sk-secret-value", "api key"},
	}

	categories := []struct {
		name             string
		makeErr          func(msg string) error
		wantStatus       int
		wantCode         string
		wantUpstreamCode string
		wantLogID        string
	}{
		{
			name: "SOCIAL_TIKTOK_RATE_LIMITED",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusTooManyRequests, Code: "rate_limit_exceeded", Message: msg, LogID: "log-rl-1", Endpoint: "creator_info"}
			},
			wantStatus:       http.StatusTooManyRequests,
			wantCode:         "SOCIAL_TIKTOK_RATE_LIMITED",
			wantUpstreamCode: "rate_limit_exceeded",
			wantLogID:        "log-rl-1",
		},
		{
			name: "SOCIAL_TIKTOK_POSTING_BANNED",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusOK, Code: "spam_risk_user_banned_from_posting", Message: msg, LogID: "log-ban-1", Endpoint: "creator_info"}
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_POSTING_BANNED",
			wantUpstreamCode: "spam_risk_user_banned_from_posting",
			wantLogID:        "log-ban-1",
		},
		{
			name: "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusOK, Code: "spam_risk_too_many_posts", Message: msg, LogID: "log-pub-1", Endpoint: "publish"}
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			wantUpstreamCode: "spam_risk_too_many_posts",
			wantLogID:        "log-pub-1",
		},
		{
			name: "SOCIAL_TIKTOK_SCOPE_MISSING_api",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusForbidden, Code: "scope_not_authorized", Message: msg, LogID: "log-scope-1", Endpoint: "publish"}
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_SCOPE_MISSING",
			wantUpstreamCode: "scope_not_authorized",
			wantLogID:        "log-scope-1",
		},
		{
			name: "SOCIAL_TIKTOK_SCOPE_MISSING_sentinel",
			makeErr: func(msg string) error {
				return fmt.Errorf("%s: %w", msg, social.ErrTikTokPublishScopeMissing)
			},
			wantStatus: http.StatusConflict,
			wantCode:   "SOCIAL_TIKTOK_SCOPE_MISSING",
		},
		{
			name: "SOCIAL_TIKTOK_TOKEN_INVALID_api",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusUnauthorized, Code: "access_token_invalid", Message: msg, LogID: "log-tok-1", Endpoint: "creator_info"}
			},
			wantStatus:       http.StatusConflict,
			wantCode:         "SOCIAL_TIKTOK_TOKEN_INVALID",
			wantUpstreamCode: "access_token_invalid",
			wantLogID:        "log-tok-1",
		},
		{
			name: "SOCIAL_TIKTOK_TOKEN_INVALID_sentinel",
			makeErr: func(msg string) error {
				return fmt.Errorf("%s: %w", msg, social.ErrTikTokTokenExpired)
			},
			wantStatus: http.StatusConflict,
			wantCode:   "SOCIAL_TIKTOK_TOKEN_INVALID",
		},
		{
			name: "SOCIAL_TIKTOK_UPSTREAM_ERROR",
			makeErr: func(msg string) error {
				return &tiktok.APIError{Status: http.StatusBadGateway, Code: "internal_failure", Message: msg, LogID: "log-up-1", Endpoint: "publish"}
			},
			wantStatus:       http.StatusBadGateway,
			wantCode:         "SOCIAL_TIKTOK_UPSTREAM_ERROR",
			wantUpstreamCode: "internal_failure",
			wantLogID:        "log-up-1",
		},
	}

	for _, cat := range categories {
		for _, tr := range triggers {
			t.Run(cat.name+"/"+tr.label, func(t *testing.T) {
				rec, body := invokeSocialError(t, cat.makeErr(tr.rawMessage))
				assert.Equal(t, cat.wantStatus, rec.Code, rec.Body.String())
				assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
				assert.NotEqual(t, http.StatusForbidden, rec.Code)
				assert.Equal(t, false, body["success"])
				assert.Equal(t, cat.wantCode, body["code"])
				if cat.wantUpstreamCode != "" {
					assert.Equal(t, cat.wantUpstreamCode, body["upstream_code"])
				}
				if cat.wantLogID != "" {
					assert.Equal(t, cat.wantLogID, body["upstream_log_id"])
				}

				msg, ok := body["message"].(string)
				require.True(t, ok)
				assert.NotEmpty(t, msg)
				assert.NotContains(t, strings.ToLower(msg), tr.forbidden)
				assert.NotContains(t, msg, "tok-secret-value")
				assert.NotContains(t, msg, "sk-secret-value")
				assertNoLegacyQuotaClassifierPhrases(t, msg)
			})
		}
	}
}

func TestSocialErrorTikTokFieldValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid upstream_code values are omitted", func(t *testing.T) {
		badCodes := []struct {
			name string
			code string
		}{
			{"empty", ""},
			{"whitespace only", "   "},
			{"contains space", "spam_risk too_many"},
			{"uppercase", "SPAM_RISK_TOO_MANY_POSTS"},
			{"special dollar", "bad$code"},
			{"special slash", "bad/code"},
			{"newline injection", "bad\ncode"},
			{"over 64 bytes", strings.Repeat("a", 65)},
		}
		for _, tc := range badCodes {
			t.Run(tc.name, func(t *testing.T) {
				rec, body := invokeSocialError(t, &tiktok.APIError{
					Status:   http.StatusOK,
					Code:     tc.code,
					Message:  "upstream error",
					LogID:    "valid-log-id-1",
					Endpoint: "publish",
				})
				assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
				assert.NotEqual(t, http.StatusForbidden, rec.Code)
				_, hasCode := body["upstream_code"]
				assert.False(t, hasCode, "invalid upstream_code %q must be omitted", tc.code)
				assert.Equal(t, "valid-log-id-1", body["upstream_log_id"])
			})
		}
	})

	t.Run("valid boundary upstream_code values are preserved", func(t *testing.T) {
		exact64 := strings.Repeat("a", 64)
		for _, validCode := range []string{"v2.error_code-1", exact64} {
			_, body := invokeSocialError(t, &tiktok.APIError{
				Status:   http.StatusBadGateway,
				Code:     validCode,
				Message:  "upstream error",
				LogID:    "valid-log-id-2",
				Endpoint: "publish",
			})
			assert.Equal(t, validCode, body["upstream_code"])
		}
	})

	t.Run("invalid upstream_log_id values are omitted", func(t *testing.T) {
		badLogIDs := []struct {
			name  string
			logID string
		}{
			{"empty", ""},
			{"whitespace only", "   "},
			{"contains space", "log id 123"},
			{"contains newline", "log\nid"},
			{"html injection", "<script>alert(1)</script>"},
			{"contains slash", "log/id"},
			{"over 128 bytes", strings.Repeat("L", 129)},
		}
		for _, tc := range badLogIDs {
			t.Run(tc.name, func(t *testing.T) {
				_, body := invokeSocialError(t, &tiktok.APIError{
					Status:   http.StatusOK,
					Code:     "spam_risk_too_many_posts",
					Message:  "too many posts",
					LogID:    tc.logID,
					Endpoint: "publish",
				})
				assert.Equal(t, "spam_risk_too_many_posts", body["upstream_code"])
				_, hasLogID := body["upstream_log_id"]
				assert.False(t, hasLogID, "invalid upstream_log_id %q must be omitted", tc.logID)
			})
		}
	})

	t.Run("valid boundary upstream_log_id values are preserved", func(t *testing.T) {
		exact128 := strings.Repeat("A", 128)
		for _, validLogID := range []string{"2026.11.24:ABC_def-01", exact128} {
			_, body := invokeSocialError(t, &tiktok.APIError{
				Status:   http.StatusOK,
				Code:     "spam_risk_too_many_posts",
				Message:  "too many posts",
				LogID:    validLogID,
				Endpoint: "publish",
			})
			assert.Equal(t, validLogID, body["upstream_log_id"])
		}
	})
}

func TestSocialTikTokUpstreamErrorsEndToEndRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldService := model.DB, socialSvc
	model.DB = db
	t.Cleanup(func() {
		model.DB, socialSvc = oldDB, oldService
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.SocialAccount{}, &model.SocialPost{}, &model.SocialTenant{}, &model.SocialOAuthState{}))

	encKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	sealedAccess, err := social.SealToken("at-live-1", encKey)
	require.NoError(t, err)
	sealedRefresh, err := social.SealToken("rt-live-1", encKey)
	require.NoError(t, err)

	future := time.Now().UTC().Add(2 * time.Hour)
	expired := time.Now().UTC().Add(-2 * time.Hour)

	accounts := []model.SocialAccount{
		{
			ID:                    10,
			UserId:                7,
			Provider:              social.ProviderTikTokDirect,
			ProviderAccountID:     "oid-direct",
			Platform:              "tiktok",
			Status:                "active",
			ScopesGranted:         "user.info.basic,video.publish",
			AccessTokenEncrypted:  sealedAccess,
			RefreshTokenEncrypted: sealedRefresh,
			TokenExpiresAt:        &future,
		},
		{
			ID:                    11,
			UserId:                7,
			Provider:              social.ProviderTikTokDirect,
			ProviderAccountID:     "oid-draft",
			Platform:              "tiktok",
			Status:                "active",
			ScopesGranted:         "user.info.basic,video.upload",
			AccessTokenEncrypted:  sealedAccess,
			RefreshTokenEncrypted: sealedRefresh,
			TokenExpiresAt:        &future,
		},
		{
			ID:                   12,
			UserId:               7,
			Provider:             social.ProviderTikTokDirect,
			ProviderAccountID:    "oid-expired-no-refresh",
			Platform:             "tiktok",
			Status:               "active",
			ScopesGranted:        "user.info.basic,video.publish",
			AccessTokenEncrypted: sealedAccess,
			TokenExpiresAt:       &expired,
		},
		{
			ID:                    13,
			UserId:                7,
			Provider:              social.ProviderTikTokDirect,
			ProviderAccountID:     "oid-expired-with-refresh",
			Platform:              "tiktok",
			Status:                "active",
			ScopesGranted:         "user.info.basic,video.publish",
			AccessTokenEncrypted:  sealedAccess,
			RefreshTokenEncrypted: sealedRefresh,
			TokenExpiresAt:        &expired,
		},
	}
	require.NoError(t, db.Create(&accounts).Error)

	type mockResp struct {
		status int
		body   string
	}
	var nextCreatorInfo atomic.Value
	var nextVideoInit atomic.Value
	var nextInboxInit atomic.Value
	var nextRefresh atomic.Value

	nextCreatorInfo.Store(mockResp{status: http.StatusOK, body: `{"data":{"creator_username":"ok","privacy_level_options":["SELF_ONLY"]},"error":{"code":"ok","message":"","log_id":"log-ok"}}`})
	nextVideoInit.Store(mockResp{status: http.StatusOK, body: `{"data":{"publish_id":"pub-ok"},"error":{"code":"ok","message":"","log_id":"log-ok"}}`})
	nextInboxInit.Store(mockResp{status: http.StatusOK, body: `{"data":{"publish_id":"inbox-ok"},"error":{"code":"ok","message":"","log_id":"log-ok"}}`})
	nextRefresh.Store(mockResp{status: http.StatusOK, body: `{"error":"invalid_grant","error_description":"refresh failed","log_id":"log-ref-default"}`})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var resp mockResp
		switch r.URL.Path {
		case "/v2/post/publish/creator_info/query/":
			resp = nextCreatorInfo.Load().(mockResp)
		case "/v2/post/publish/video/init/":
			resp = nextVideoInit.Load().(mockResp)
		case "/v2/post/publish/inbox/video/init/":
			resp = nextInboxInit.Load().(mockResp)
		case "/v2/oauth/token/":
			resp = nextRefresh.Load().(mockResp)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	t.Cleanup(upstream.Close)

	setSocialService(social.NewService(social.Config{
		Enabled:             true,
		Platforms:           []string{"tiktok"},
		TikTokDirectEnabled: true,
		TikTokClientKey:     "test-client-key",
		TikTokClientSecret:  "test-client-secret",
		TikTokRedirectURI:   "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		TikTokBaseURL:       upstream.URL,
		SocialTokenEncKey:   encKey,
	}))

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7); c.Next() })
	router.GET("/api/social/v1/accounts/:id/creator-info", SocialAccountCreatorInfo)
	router.POST("/api/social/v1/posts", SocialCreatePost)

	doReq := func(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var out map[string]any
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
		return rec, out
	}

	t.Run("GET creator-info HTTP 200 spam_risk_too_many_posts with quota phrase", func(t *testing.T) {
		nextCreatorInfo.Store(mockResp{
			status: http.StatusOK,
			body:   `{"data":{},"error":{"code":"spam_risk_too_many_posts","message":"Daily quota exceeded","log_id":"log-ci-posts-1"}}`,
		})
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/10/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, false, body["success"])
		assert.Equal(t, "SOCIAL_TIKTOK_PUBLISH_RESTRICTED", body["code"])
		assert.Equal(t, "spam_risk_too_many_posts", body["upstream_code"])
		assert.Equal(t, "log-ci-posts-1", body["upstream_log_id"])
		msg, _ := body["message"].(string)
		assert.Equal(t, "TikTok posting limit reached", msg)
		assertNoLegacyQuotaClassifierPhrases(t, msg)
	})

	t.Run("GET creator-info HTTP 200 spam_risk_user_banned_from_posting", func(t *testing.T) {
		nextCreatorInfo.Store(mockResp{
			status: http.StatusOK,
			body:   `{"data":{},"error":{"code":"spam_risk_user_banned_from_posting","message":"User banned","log_id":"log-ci-ban-1"}}`,
		})
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/10/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_POSTING_BANNED", body["code"])
		assert.Equal(t, "spam_risk_user_banned_from_posting", body["upstream_code"])
		assert.Equal(t, "log-ci-ban-1", body["upstream_log_id"])
	})

	t.Run("GET creator-info HTTP 401 access_token_invalid never returns 401", func(t *testing.T) {
		nextCreatorInfo.Store(mockResp{
			status: http.StatusUnauthorized,
			body:   `{"error":{"code":"access_token_invalid","message":"Bearer at-live-1 expired","log_id":"log-ci-401"}}`,
		})
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/10/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_TOKEN_INVALID", body["code"])
		assert.Equal(t, "access_token_invalid", body["upstream_code"])
		assert.Equal(t, "log-ci-401", body["upstream_log_id"])
		msg, _ := body["message"].(string)
		assert.NotContains(t, strings.ToLower(msg), "bearer")
	})

	t.Run("GET creator-info expired account without refresh returns 409 TOKEN_INVALID", func(t *testing.T) {
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/12/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_TOKEN_INVALID", body["code"])
		_, hasCode := body["upstream_code"]
		assert.False(t, hasCode)
	})

	t.Run("GET creator-info expired account refresh HTTP 200 invalid_grant returns 409 TOKEN_INVALID", func(t *testing.T) {
		nextRefresh.Store(mockResp{
			status: http.StatusOK,
			body:   `{"error":"invalid_grant","error_description":"The refresh token is invalid.","log_id":"log-ref-200"}`,
		})
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/13/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_TOKEN_INVALID", body["code"])
		assert.Equal(t, "invalid_grant", body["upstream_code"])
		assert.Equal(t, "log-ref-200", body["upstream_log_id"])
	})

	t.Run("GET creator-info expired account refresh HTTP 429 returns 429 RATE_LIMITED", func(t *testing.T) {
		nextRefresh.Store(mockResp{
			status: http.StatusTooManyRequests,
			body:   `{"error":{"code":"rate_limit_exceeded","message":"Usage limit reached","log_id":"log-ref-429"}}`,
		})
		rec, body := doReq(http.MethodGet, "/api/social/v1/accounts/13/creator-info?provider=tiktok_direct", "")
		assert.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_RATE_LIMITED", body["code"])
		assert.Equal(t, "rate_limit_exceeded", body["upstream_code"])
		assert.Equal(t, "log-ref-429", body["upstream_log_id"])
		msg, _ := body["message"].(string)
		assertNoLegacyQuotaClassifierPhrases(t, msg)
	})

	// Reset creator-info to succeed for POST /posts direct-post tests.
	nextCreatorInfo.Store(mockResp{
		status: http.StatusOK,
		body:   `{"data":{"creator_username":"ok","privacy_level_options":["SELF_ONLY"]},"error":{"code":"ok","message":"","log_id":"log-ok"}}`,
	})

	t.Run("POST posts direct publish HTTP 200 unaudited_client returns 409 PUBLISH_RESTRICTED", func(t *testing.T) {
		nextVideoInit.Store(mockResp{
			status: http.StatusOK,
			body:   `{"data":{},"error":{"code":"unaudited_client_can_only_post_to_private_accounts","message":"Insufficient quota for public posting","log_id":"log-post-unaudited"}}`,
		})
		rec, body := doReq(http.MethodPost, "/api/social/v1/posts", `{
			"provider": "tiktok_direct",
			"content": "hello",
			"account_ids": [10],
			"media_items": [{"url": "https://omnimux.ai/video.mp4"}],
			"tiktok_options": {"privacy_level": "SELF_ONLY"}
		}`)
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_PUBLISH_RESTRICTED", body["code"])
		assert.Equal(t, "unaudited_client_can_only_post_to_private_accounts", body["upstream_code"])
		assert.Equal(t, "log-post-unaudited", body["upstream_log_id"])
		msg, _ := body["message"].(string)
		assertNoLegacyQuotaClassifierPhrases(t, msg)
	})

	t.Run("POST posts direct publish HTTP 403 scope_not_authorized never returns 403", func(t *testing.T) {
		nextVideoInit.Store(mockResp{
			status: http.StatusForbidden,
			body:   `{"error":{"code":"scope_not_authorized","message":"Missing video.publish","log_id":"log-post-scope"}}`,
		})
		rec, body := doReq(http.MethodPost, "/api/social/v1/posts", `{
			"provider": "tiktok_direct",
			"content": "hello",
			"account_ids": [10],
			"media_items": [{"url": "https://omnimux.ai/video.mp4"}],
			"tiktok_options": {"privacy_level": "SELF_ONLY"}
		}`)
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_SCOPE_MISSING", body["code"])
		assert.Equal(t, "scope_not_authorized", body["upstream_code"])
		assert.Equal(t, "log-post-scope", body["upstream_log_id"])
	})

	t.Run("POST posts draft upload HTTP 200 spam_risk_too_many_pending_share returns 409 PUBLISH_RESTRICTED", func(t *testing.T) {
		nextInboxInit.Store(mockResp{
			status: http.StatusOK,
			body:   `{"data":{},"error":{"code":"spam_risk_too_many_pending_share","message":"Too many pending drafts","log_id":"log-inbox-pending"}}`,
		})
		rec, body := doReq(http.MethodPost, "/api/social/v1/posts", `{
			"provider": "tiktok_direct",
			"content": "draft clip",
			"account_ids": [11],
			"media_items": [{"url": "https://omnimux.ai/video.mp4"}]
		}`)
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_PUBLISH_RESTRICTED", body["code"])
		assert.Equal(t, "spam_risk_too_many_pending_share", body["upstream_code"])
		assert.Equal(t, "log-inbox-pending", body["upstream_log_id"])
	})

	t.Run("POST posts HTTP 500 unclassified error returns 502 UPSTREAM_ERROR", func(t *testing.T) {
		nextVideoInit.Store(mockResp{
			status: http.StatusInternalServerError,
			body:   `{"error":{"code":"internal_error","message":"api key internal failure","log_id":"log-post-500"}}`,
		})
		rec, body := doReq(http.MethodPost, "/api/social/v1/posts", `{
			"provider": "tiktok_direct",
			"content": "hello",
			"account_ids": [10],
			"media_items": [{"url": "https://omnimux.ai/video.mp4"}],
			"tiktok_options": {"privacy_level": "SELF_ONLY"}
		}`)
		assert.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
		assert.Equal(t, "SOCIAL_TIKTOK_UPSTREAM_ERROR", body["code"])
		assert.Equal(t, "internal_error", body["upstream_code"])
		assert.Equal(t, "log-post-500", body["upstream_log_id"])
		msg, _ := body["message"].(string)
		assert.Equal(t, "TikTok upstream request failed", msg)
	})
}
