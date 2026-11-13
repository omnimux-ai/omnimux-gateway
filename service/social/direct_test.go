package social

// fork: tiktok-direct — routing decision + callback integration tests.

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDirectConfig(t *testing.T, apiURL string) Config {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i * 7)
	}
	return Config{
		Enabled:             true,
		APIKey:              "zernio-key",
		BaseURL:             apiURL, // reuse for zernio stub
		Platforms:           []string{"twitter", "linkedin", "instagram", "tiktok", "youtube", "facebook", "threads", "bluesky"},
		TikTokDirectEnabled: true,
		TikTokClientKey:     "tt-ck",
		TikTokClientSecret:  "tt-cs",
		TikTokRedirectURI:   "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		TikTokScopes:        "user.info.basic,video.upload",
		TikTokBaseURL:       apiURL,
		TikTokAuthorizeURL:  "https://www.tiktok.com/v2/auth/authorize/",
		SocialTokenEncKey:   base64.StdEncoding.EncodeToString(key),
	}
}

func setupDirectDB(t *testing.T) {
	t.Helper()
	clearSocialTables(t)
	require.NoError(t, model.DB.Exec("DELETE FROM social_oauth_states").Error)
}

func TestConnectRoutesToDirect(t *testing.T) {
	setupDirectDB(t)
	cfg := testDirectConfig(t, "http://stub")
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "u42", Quota: 1000000}).Error)

	urlOut, err := s.Connect(context.Background(), 42, ProviderTikTokDirect, "tiktok", "")
	require.NoError(t, err)
	assert.Contains(t, urlOut, "client_key=tt-ck")
	assert.Contains(t, urlOut, "www.tiktok.com/v2/auth/authorize")

	// state persisted
	var st model.SocialOAuthState
	require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 42, ProviderTikTokDirect).First(&st).Error)
	assert.NotEmpty(t, st.State)
}

func TestConnectExplicitZernioIgnoresDirectFlag(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.DB.Create(&model.SocialTenant{UserId: 42, Provider: ProviderZernio, ProviderProfileID: "profile42"}).Error)
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "/connect/tiktok", r.URL.Path)
		_, _ = w.Write([]byte(`{"authUrl":"https://zernio.example/oauth"}`))
	}))
	t.Cleanup(upstream.Close)
	for _, enabled := range []bool{true, false} {
		cfg := testDirectConfig(t, upstream.URL)
		cfg.TikTokDirectEnabled = enabled
		urlOut, err := NewService(cfg).Connect(context.Background(), 42, ProviderZernio, "tiktok", "")
		require.NoError(t, err)
		assert.Equal(t, "https://zernio.example/oauth", urlOut)
	}
	assert.Equal(t, 2, requests)
	var states int64
	require.NoError(t, model.DB.Model(&model.SocialOAuthState{}).Count(&states).Error)
	assert.Zero(t, states, "Zernio entry must not create official OAuth state")
}

func TestConnectDirectRejectsForeignRedirect(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 42, Username: "u42", Quota: 1000000}).Error)
	_, err := NewService(testDirectConfig(t, "http://stub")).Connect(context.Background(), 42, ProviderTikTokDirect, "tiktok", "https://evil.example/phish")
	require.ErrorIs(t, err, ErrRedirectDenied)
	var states int64
	require.NoError(t, model.DB.Model(&model.SocialOAuthState{}).Count(&states).Error)
	assert.Zero(t, states, "a rejected redirect must not create an OAuth state")
}

func TestConnectDirectNotConfigured(t *testing.T) {
	setupDirectDB(t)
	cfg := testDirectConfig(t, "http://stub")
	cfg.TikTokClientKey = ""
	cfg.TikTokClientSecret = ""
	s := NewService(cfg)
	_, err := s.Connect(context.Background(), 42, ProviderTikTokDirect, "tiktok", "")
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestConnectDirectDisabledDoesNotFallBack(t *testing.T) {
	setupDirectDB(t)
	cfg := testDirectConfig(t, "http://stub")
	cfg.TikTokDirectEnabled = false
	s := NewService(cfg)
	_, err := s.Connect(context.Background(), 42, ProviderTikTokDirect, "tiktok", "")
	require.ErrorIs(t, err, ErrTikTokDirectNotConfigured)
}

func TestConnectRollbackDisablesDirectWithoutFallback(t *testing.T) {
	setupDirectDB(t)
	cfg := testDirectConfig(t, "http://stub")
	cfg.RollbackTikTokToZernio = true
	s := NewService(cfg)
	_, err := s.Connect(context.Background(), 42, ProviderTikTokDirect, "tiktok", "")
	require.ErrorIs(t, err, ErrTikTokDirectNotConfigured)
}

func TestDirectProviderRejectsOtherPlatforms(t *testing.T) {
	cfg := testDirectConfig(t, "http://stub")
	s := NewService(cfg)
	for _, platform := range []string{"twitter", "instagram"} {
		_, err := s.Connect(context.Background(), 42, ProviderTikTokDirect, platform, "")
		require.ErrorIs(t, err, ErrPlatformDenied)
	}
}

func tiktokStubServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "refresh_token" {
			_, _ = w.Write([]byte(`{"data":{"access_token":"at-refreshed","refresh_token":"rt-new","expires_in":3600,"open_id":"oid-9"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"access_token":"at-9","refresh_token":"rt-9","expires_in":3600,"open_id":"oid-9","scope":"user.info.basic,video.upload"}}`))
	})
	mux.HandleFunc("/v2/user/info/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-9","display_name":"Demo Creator","username":"democreator","avatar_url":"https://img"}}}`))
	})
	return httptest.NewServer(mux)
}

func TestHandleTikTokCallbackSuccess(t *testing.T) {
	setupDirectDB(t)
	srv := tiktokStubServer(t)
	defer srv.Close()
	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "u7", Quota: 1000000}).Error)
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "state-1",
		UserId:   7,
		Provider: ProviderTikTokDirect,
	}))

	res, err := s.HandleTikTokCallback(context.Background(), "code-1", "state-1")
	require.NoError(t, err)
	require.NotNil(t, res.Account)
	assert.Equal(t, ProviderTikTokDirect, res.Account.Provider)
	assert.Equal(t, "oid-9", res.Account.ProviderAccountID)
	assert.Equal(t, "Demo Creator", res.Account.DisplayName)
	assert.Equal(t, "democreator", res.Account.Username)
	assert.Equal(t, model.SocialAccountStatusActive, res.Account.Status)

	// Token vault persisted (encrypted, not plaintext)
	require.NotNil(t, res.Account.TokenExpiresAt)
	assert.NotContains(t, res.Account.AccessTokenEncrypted, "at-9")
	assert.NotEmpty(t, res.Account.RefreshTokenEncrypted)
	assert.Equal(t, "user.info.basic,video.upload", res.Account.ScopesGranted)

	// state consumed (replay fails)
	_, err = s.HandleTikTokCallback(context.Background(), "code-1", "state-1")
	require.Error(t, err)
}

func TestHandleTikTokCallbackBadState(t *testing.T) {
	setupDirectDB(t)
	srv := tiktokStubServer(t)
	defer srv.Close()
	s := NewService(testDirectConfig(t, srv.URL))
	_, err := s.HandleTikTokCallback(context.Background(), "code", "no-such-state")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state")
}

func TestHandleTikTokCallbackExpiredState(t *testing.T) {
	setupDirectDB(t)
	srv := tiktokStubServer(t)
	defer srv.Close()
	s := NewService(testDirectConfig(t, srv.URL))
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:     "old",
		UserId:    1,
		Provider:  ProviderTikTokDirect,
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	}))
	_, err := s.HandleTikTokCallback(context.Background(), "code", "old")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestHandleTikTokCallbackMissingEncKey(t *testing.T) {
	setupDirectDB(t)
	srv := tiktokStubServer(t)
	defer srv.Close()
	cfg := testDirectConfig(t, srv.URL)
	cfg.SocialTokenEncKey = ""
	s := NewService(cfg)
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{State: "s2", UserId: 1, Provider: ProviderTikTokDirect}))
	_, err := s.HandleTikTokCallback(context.Background(), "code", "s2")
	require.ErrorIs(t, err, ErrTokenEncKeyMissing)
}

func TestListAccountsDirectOnly(t *testing.T) {
	setupDirectDB(t)
	// No Zernio endpoint is available or needed for the direct account list.
	require.NoError(t, model.DB.Create(&model.User{Id: 5, Username: "u5"}).Error)
	require.NoError(t, model.DB.Create(&model.SocialAccount{
		UserId:            5,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-x",
		ProviderProfileID: "oid-x",
		Platform:          "tiktok",
		Username:          "ttuser",
		DisplayName:       "TT User",
		Status:            model.SocialAccountStatusActive,
	}).Error)
	s := NewService(testDirectConfig(t, "http://stub"))
	rows, err := s.ListAccounts(context.Background(), 5, ProviderTikTokDirect)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, ProviderTikTokDirect, rows[0].Provider)
}

func TestCreatePostDirectNoMediaRejected(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 3, Username: "u3"}).Error)
	require.NoError(t, model.DB.Create(&model.SocialAccount{
		UserId:            3,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: "oid-3",
		Platform:          "tiktok",
		Status:            model.SocialAccountStatusActive,
	}).Error)
	var acc model.SocialAccount
	require.NoError(t, model.DB.Where("user_id = ?", 3).First(&acc).Error)
	s := NewService(testDirectConfig(t, "http://stub"))
	_, err := s.CreatePost(context.Background(), 3, CreatePostInput{
		Provider:   ProviderTikTokDirect,
		Content:    "hello",
		AccountIDs: []int64{acc.ID},
		PublishNow: true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMediaInvalid)
}

func TestCreatePostDirectVideo(t *testing.T) {
	setupDirectDB(t)
	var gotVideoURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/post/publish/creator_info/query/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"creator_username":"u"}}`))
	})
	mux.HandleFunc("/v2/post/publish/video/init/", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SourceInfo struct {
				VideoURL string `json:"video_url"`
			} `json:"source_info"`
		}
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		_ = common.Unmarshal(buf[:n], &body)
		gotVideoURL = body.SourceInfo.VideoURL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-42"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 4, Username: "u4"}).Error)
	enc, err := SealToken("at-4", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.SocialAccount{
		UserId:               4,
		Provider:             ProviderTikTokDirect,
		ProviderAccountID:    "oid-4",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: enc,
	}).Error)
	var acc model.SocialAccount
	require.NoError(t, model.DB.Where("user_id = ?", 4).First(&acc).Error)

	post, err := s.CreatePost(context.Background(), 4, CreatePostInput{
		Provider:   ProviderTikTokDirect,
		Content:    "launch video",
		AccountIDs: []int64{acc.ID},
		PublishNow: true,
		MediaItems: []map[string]any{{"url": "https://cdn.example.com/v.mp4"}},
	})
	require.NoError(t, err)
	assert.Equal(t, ProviderTikTokDirect, post.Provider)
	assert.Equal(t, "pub-42", post.ProviderPostID)
	assert.Equal(t, "submitted", post.Status)
	// Third-party storage URL must be wrapped under verified omnimux.ai origin for TikTok PULL_FROM_URL.
	assert.True(t, strings.HasPrefix(gotVideoURL, "https://omnimux.ai/api/social/v1/public/media/"), "expected wrapped omnimux.ai URL, got %s", gotVideoURL)
	tok := strings.TrimPrefix(gotVideoURL, "https://omnimux.ai/api/social/v1/public/media/")
	unwrapped, err := s.OpenPublicMediaPullToken(tok)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/v.mp4", unwrapped)
}

func TestTikTokMediaTokenAndComplianceValidation(t *testing.T) {
	setupDirectDB(t)
	s := NewService(testDirectConfig(t, "https://api.example.com"))

	// Reject private/non-https URLs when sealing media token
	_, err := s.SealPublicMediaPullToken("http://cdn.example.com/v.mp4", time.Minute)
	require.ErrorIs(t, err, ErrMediaInvalid)
	_, err = s.SealPublicMediaPullToken("https://127.0.0.1/v.mp4", time.Minute)
	require.ErrorIs(t, err, ErrMediaInvalid)
	_, err = s.SealPublicMediaPullToken("https://10.0.0.1/v.mp4", time.Minute)
	require.ErrorIs(t, err, ErrMediaInvalid)

	// Expired token is rejected
	expiredTok, err := s.SealPublicMediaPullToken("https://cdn.example.com/v.mp4", -time.Minute)
	require.NoError(t, err)
	_, err = s.OpenPublicMediaPullToken(expiredTok)
	require.ErrorIs(t, err, ErrMediaInvalid)

	// Already-verified omnimux.ai URL is kept as-is
	sameOrigin, err := s.WrapTikTokVerifiedMediaURL("https://omnimux.ai/media/clip.mp4")
	require.NoError(t, err)
	assert.Equal(t, "https://omnimux.ai/media/clip.mp4", sameOrigin)

	// Compliance option validation: commercial_content on without subtype fails
	tr := true
	f := false
	_, err = resolveTikTokPostOptions(&TikTokPostOptions{
		PrivacyLevel:      "PUBLIC_TO_EVERYONE",
		CommercialContent: &tr,
	})
	require.ErrorIs(t, err, ErrMediaInvalid)

	// Branded content cannot be SELF_ONLY
	_, err = resolveTikTokPostOptions(&TikTokPostOptions{
		PrivacyLevel:      "SELF_ONLY",
		CommercialContent: &tr,
		BrandedContent:    &tr,
	})
	require.ErrorIs(t, err, ErrMediaInvalid)

	// Music usage explicitly false fails
	_, err = resolveTikTokPostOptions(&TikTokPostOptions{
		PrivacyLevel:        "SELF_ONLY",
		MusicUsageConfirmed: &f,
	})
	require.ErrorIs(t, err, ErrMediaInvalid)
}

func TestCreatePostMixedProvidersRejected(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 6, Username: "u6"}).Error)
	require.NoError(t, model.DB.Create(&model.SocialAccount{UserId: 6, Provider: ProviderTikTokDirect, ProviderAccountID: "oid-a", Platform: "tiktok", Status: model.SocialAccountStatusActive}).Error)
	require.NoError(t, model.DB.Create(&model.SocialAccount{UserId: 6, Provider: ProviderZernio, ProviderAccountID: "z-a", Platform: "twitter", Status: model.SocialAccountStatusActive}).Error)
	var rows []model.SocialAccount
	require.NoError(t, model.DB.Where("user_id = ?", 6).Find(&rows).Error)
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	s := NewService(testDirectConfig(t, "http://stub"))
	_, err := s.CreatePost(context.Background(), 6, CreatePostInput{
		Provider:   ProviderTikTokDirect,
		Content:    "x",
		AccountIDs: ids,
		MediaItems: []map[string]any{{"url": "https://cdn.example.com/v.mp4"}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAccountProviderMismatch)
}

func TestDisconnectDirect(t *testing.T) {
	setupDirectDB(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/revoke/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 8, Username: "u8"}).Error)
	enc, err := SealToken("at-8", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.SocialAccount{
		UserId:               8,
		Provider:             ProviderTikTokDirect,
		ProviderAccountID:    "oid-8",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: enc,
	}).Error)
	var acc model.SocialAccount
	require.NoError(t, model.DB.Where("user_id = ?", 8).First(&acc).Error)
	require.NoError(t, s.DisconnectAccount(context.Background(), 8, ProviderTikTokDirect, acc.ID))
	updated, err := model.GetSocialAccountByIDForUser(acc.ID, 8)
	require.NoError(t, err)
	assert.Equal(t, model.SocialAccountStatusDisconnected, updated.Status)
}

func TestMapTikTokPublishStatus(t *testing.T) {
	assert.Equal(t, "published", mapTikTokPublishStatus("PUBLISH_COMPLETE"))
	assert.Equal(t, "failed", mapTikTokPublishStatus("FAILED"))
	assert.Equal(t, "processing", mapTikTokPublishStatus("PROCESSING_UPLOAD"))
	assert.Equal(t, "", mapTikTokPublishStatus("SOMETHING_ELSE"))
}

func TestConsumeStateSingleUse(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{State: "once", UserId: 1, Provider: ProviderTikTokDirect}))
	st, err := model.ConsumeSocialOAuthState("once", ProviderTikTokDirect)
	require.NoError(t, err)
	assert.Equal(t, "once", st.State)
	_, err = model.ConsumeSocialOAuthState("once", ProviderTikTokDirect)
	require.Error(t, err)
}

var _ = url.Values{} // keep url import if needed later
