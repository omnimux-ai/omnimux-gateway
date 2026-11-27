package social

// fork: tiktok-direct — routing decision + callback integration tests.

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
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
		Enabled:   true,
		APIKey:    "zernio-key",
		BaseURL:   apiURL, // reuse for zernio stub
		Platforms: []string{"twitter", "linkedin", "instagram", "tiktok", "youtube", "facebook", "threads", "bluesky"},
		// fork: zernio-retire — the rollback source stays reachable in tests.
		ZernioEnabled:       true,
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
	parsed, err := url.Parse(urlOut)
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "omnimux.ai", parsed.Host)
	assert.Equal(t, tikTokOAuthStartPath, parsed.Path)
	ticket := parsed.Query().Get("ticket")
	require.NotEmpty(t, ticket)

	// One-time launch ticket persisted with short TTL (<= 3m) under start provider.
	var st model.SocialOAuthState
	require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 42, ProviderTikTokDirectStart).First(&st).Error)
	assert.Equal(t, ticket, st.State)
	assert.True(t, st.ExpiresAt.After(time.Now().UTC()))
	assert.True(t, st.ExpiresAt.Before(time.Now().UTC().Add(tikTokOAuthStartTicketTTL+5*time.Second)))

	// Redeeming ticket produces a fresh callback state and strictly validated TikTok URL.
	started, err := s.StartTikTokOAuth(context.Background(), ticket)
	require.NoError(t, err)
	assert.NotEqual(t, ticket, started.State)
	assert.Contains(t, started.AuthorizeURL, "client_key=tt-ck")
	assert.Contains(t, started.AuthorizeURL, "https://www.tiktok.com/v2/auth/authorize/")
	assert.Contains(t, started.AuthorizeURL, "state="+started.State)

	// Ticket cannot be replayed.
	_, err = s.StartTikTokOAuth(context.Background(), ticket)
	require.ErrorIs(t, err, ErrTikTokOAuthTicketInvalid)

	// Fresh callback state persisted under ProviderTikTokDirect.
	var cbState model.SocialOAuthState
	require.NoError(t, model.DB.Where("user_id = ? AND provider = ?", 42, ProviderTikTokDirect).First(&cbState).Error)
	assert.Equal(t, started.State, cbState.State)
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
			_, _ = w.Write([]byte(`{"access_token":"at-refreshed","refresh_token":"rt-new","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-9","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"at-9","refresh_token":"rt-9","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-9","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	})
	mux.HandleFunc("/v2/user/info/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-9","display_name":"Demo Creator","username":"democreator","avatar_url":"https://img"}},"error":{"code":"ok","message":"","log_id":"log-ui"}}`))
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
		ScopesGranted:        "user.info.basic,video.publish,video.upload",
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

func TestCreatePostTikTokDraftUpload(t *testing.T) {
	setupDirectDB(t)
	var gotPath, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/post/publish/inbox/video/init/", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"draft-42"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 12, Username: "u12"}).Error)
	enc, err := SealToken("at-12", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.SocialAccount{
		UserId:               12,
		Provider:             ProviderTikTokDirect,
		ProviderAccountID:    "oid-12",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: enc,
		ScopesGranted:        "user.info.basic,video.upload",
	}).Error)
	var acc model.SocialAccount
	require.NoError(t, model.DB.Where("user_id = ?", 12).First(&acc).Error)

	post, err := s.CreatePost(context.Background(), 12, CreatePostInput{
		Provider:   ProviderTikTokDirect,
		Content:    "draft me",
		AccountIDs: []int64{acc.ID},
		PublishNow: true,
		MediaItems: []map[string]any{{"url": "https://cdn.example.com/v.mp4"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "draft-42", post.ProviderPostID)
	assert.Equal(t, "submitted", post.Status)
	assert.Equal(t, "/v2/post/publish/inbox/video/init/", gotPath)
	assert.Contains(t, gotBody, `"source":"PULL_FROM_URL"`)
	assert.Contains(t, gotBody, "https://omnimux.ai/api/social/v1/public/media/")
	assert.NotContains(t, gotBody, "post_info")
}

func TestCreatePostTikTokDraftPhotoCarousel(t *testing.T) {
	t.Run("single JPEG and multi-photo JPEG/WebP carousel upload via MEDIA_UPLOAD with wrapped URLs", func(t *testing.T) {
		setupDirectDB(t)
		var calls int
		var gotPath, gotBody string
		mux := http.NewServeMux()
		mux.HandleFunc("/v2/post/publish/content/init/", func(w http.ResponseWriter, r *http.Request) {
			calls++
			gotPath = r.URL.Path
			buf := make([]byte, 16384)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"publish_id":"photo-draft-1"}}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := testDirectConfig(t, srv.URL)
		s := NewService(cfg)
		require.NoError(t, model.DB.Create(&model.User{Id: 13, Username: "u13"}).Error)
		enc, err := SealToken("at-13", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		require.NoError(t, model.DB.Create(&model.SocialAccount{
			UserId:               13,
			Provider:             ProviderTikTokDirect,
			ProviderAccountID:    "oid-13",
			Platform:             "tiktok",
			Status:               model.SocialAccountStatusActive,
			AccessTokenEncrypted: enc,
			ScopesGranted:        "user.info.basic,video.upload",
		}).Error)
		var acc model.SocialAccount
		require.NoError(t, model.DB.Where("user_id = ?", 13).First(&acc).Error)

		// 1) Single JPEG photo without explicit options
		post1, err := s.CreatePost(context.Background(), 13, CreatePostInput{
			Provider:   ProviderTikTokDirect,
			Content:    "single photo caption",
			AccountIDs: []int64{acc.ID},
			PublishNow: true,
			MediaItems: []map[string]any{
				{"url": "https://cdn.example.com/look1.jpg?sig=abc"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "photo-draft-1", post1.ProviderPostID)
		assert.Equal(t, "submitted", post1.Status)
		assert.Equal(t, "/v2/post/publish/content/init/", gotPath)
		assert.Contains(t, gotBody, `"post_mode":"MEDIA_UPLOAD"`)
		assert.Contains(t, gotBody, `"media_type":"PHOTO"`)
		assert.Contains(t, gotBody, `"description":"single photo caption"`)

		// 2) Multi-photo JPEG + WebP + extensionless content_type with explicit photo_cover_index=0 and is_aigc=false.
		// Direct-post-only privacy/interaction fields must be omitted in MEDIA_UPLOAD.
		zero := 0
		isAIGC := false
		tr := true
		post2, err := s.CreatePost(context.Background(), 13, CreatePostInput{
			Provider:   ProviderTikTokDirect,
			AccountIDs: []int64{acc.ID},
			PublishNow: true,
			MediaItems: []map[string]any{
				{"url": "https://cdn.example.com/slide1.jpeg?X-Amz-Signature=1"},
				{"url": "https://cdn.example.com/slide2.webp"},
				{"url": "https://cdn.example.com/asset/slide3", "type": "image", "content_type": "image/webp"},
			},
			TikTokOptions: &TikTokPostOptions{
				Title:             "Lookbook Title",
				Description:       "Lookbook Description",
				PhotoCoverIndex:   &zero,
				IsAIGC:            &isAIGC,
				PrivacyLevel:      "PUBLIC_TO_EVERYONE",
				AllowComment:      &tr,
				AllowDuet:         &tr,
				AllowStitch:       &tr,
				CommercialContent: &tr,
				YourBrand:         &tr,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "photo-draft-1", post2.ProviderPostID)
		assert.Equal(t, 2, calls)

		var payload struct {
			PostMode   string `json:"post_mode"`
			MediaType  string `json:"media_type"`
			SourceInfo struct {
				Source          string   `json:"source"`
				PhotoImages     []string `json:"photo_images"`
				PhotoCoverIndex *int     `json:"photo_cover_index"`
			} `json:"source_info"`
			PostInfo map[string]any `json:"post_info"`
		}
		require.NoError(t, common.Unmarshal([]byte(gotBody), &payload))
		assert.Equal(t, "MEDIA_UPLOAD", payload.PostMode)
		assert.Equal(t, "PHOTO", payload.MediaType)
		assert.Equal(t, "PULL_FROM_URL", payload.SourceInfo.Source)
		require.NotNil(t, payload.SourceInfo.PhotoCoverIndex)
		assert.Equal(t, 0, *payload.SourceInfo.PhotoCoverIndex)
		require.Len(t, payload.SourceInfo.PhotoImages, 3)

		wantOriginals := []string{
			"https://cdn.example.com/slide1.jpeg?X-Amz-Signature=1",
			"https://cdn.example.com/slide2.webp",
			"https://cdn.example.com/asset/slide3",
		}
		for i, wrapped := range payload.SourceInfo.PhotoImages {
			require.True(t, strings.HasPrefix(wrapped, "https://omnimux.ai/api/social/v1/public/media/"), "image %d must be wrapped: %s", i, wrapped)
			tok := strings.TrimPrefix(wrapped, "https://omnimux.ai/api/social/v1/public/media/")
			unwrapped, uerr := s.OpenPublicMediaPullToken(tok)
			require.NoError(t, uerr)
			assert.Equal(t, wantOriginals[i], unwrapped)
		}
		assert.Equal(t, "Lookbook Title", payload.PostInfo["title"])
		assert.Equal(t, "Lookbook Description", payload.PostInfo["description"])
		assert.Equal(t, false, payload.PostInfo["is_aigc"])
		assert.Contains(t, gotBody, `"photo_cover_index":0`)
		assert.Contains(t, gotBody, `"is_aigc":false`)
		assert.NotContains(t, gotBody, "privacy_level")
		assert.NotContains(t, gotBody, "disable_comment")
		assert.NotContains(t, gotBody, "disable_duet")
		assert.NotContains(t, gotBody, "disable_stitch")
		assert.NotContains(t, gotBody, "brand_content_toggle")
		assert.NotContains(t, gotBody, "brand_organic_toggle")
	})

	t.Run("rejects unsupported image format, mixed media, over-limit counts, bad cover index, and SSRF URLs", func(t *testing.T) {
		setupDirectDB(t)
		cfg := testDirectConfig(t, "https://api.example.com")
		cfg.MaxMediaItems = 40
		s := NewService(cfg)
		require.NoError(t, model.DB.Create(&model.User{Id: 14, Username: "u14"}).Error)
		enc, err := SealToken("at-14", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		require.NoError(t, model.DB.Create(&model.SocialAccount{
			UserId:               14,
			Provider:             ProviderTikTokDirect,
			ProviderAccountID:    "oid-14",
			Platform:             "tiktok",
			Status:               model.SocialAccountStatusActive,
			AccessTokenEncrypted: enc,
			ScopesGranted:        "user.info.basic,video.upload",
		}).Error)
		var acc model.SocialAccount
		require.NoError(t, model.DB.Where("user_id = ?", 14).First(&acc).Error)

		thirtySixPhotos := make([]map[string]any, 36)
		for i := range thirtySixPhotos {
			thirtySixPhotos[i] = map[string]any{"url": fmt.Sprintf("https://cdn.example.com/p%d.jpg", i)}
		}
		negIdx := -1
		oobIdx := 2
		cases := []struct {
			name string
			in   CreatePostInput
		}{
			{
				name: "PNG unsupported",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://cdn.example.com/a.png", "type": "image"}},
				},
			},
			{
				name: "GIF unsupported",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://cdn.example.com/a.gif"}},
				},
			},
			{
				name: "extensionless image without content_type rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://cdn.example.com/asset/1", "type": "image"}},
				},
			},
			{
				name: "unsupported image content_type rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://cdn.example.com/asset/1", "type": "image", "content_type": "image/png"}},
				},
			},
			{
				name: "mixed photo and video rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{
						{"url": "https://cdn.example.com/a.jpg", "type": "image"},
						{"url": "https://cdn.example.com/v.mp4", "type": "video"},
					},
				},
			},
			{
				name: "exceeds TikTok 35-photo limit",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: thirtySixPhotos,
				},
			},
			{
				name: "negative photo_cover_index rejected",
				in: CreatePostInput{
					Provider:        ProviderTikTokDirect,
					AccountIDs:      []int64{acc.ID},
					MediaItems:      []map[string]any{{"url": "https://cdn.example.com/a.jpg"}},
					PhotoCoverIndex: &negIdx,
				},
			},
			{
				name: "out-of-range photo_cover_index rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://cdn.example.com/a.jpg"}, {"url": "https://cdn.example.com/b.webp"}},
					TikTokOptions: &TikTokPostOptions{
						PhotoCoverIndex: &oobIdx,
					},
				},
			},
			{
				name: "private IP SSRF photo URL rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "https://10.0.0.1/secret.jpg"}},
				},
			},
			{
				name: "non-HTTPS photo URL rejected",
				in: CreatePostInput{
					Provider:   ProviderTikTokDirect,
					AccountIDs: []int64{acc.ID},
					MediaItems: []map[string]any{{"url": "http://cdn.example.com/a.jpg"}},
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := s.CreatePost(context.Background(), 14, tc.in)
				require.ErrorIs(t, err, ErrMediaInvalid)
			})
		}

		// Also verify configured MaxMediaItems cap is respected when lower than 35.
		cfgCap := testDirectConfig(t, "https://api.example.com")
		cfgCap.MaxMediaItems = 2
		sCap := NewService(cfgCap)
		_, err = sCap.CreatePost(context.Background(), 14, CreatePostInput{
			Provider:   ProviderTikTokDirect,
			AccountIDs: []int64{acc.ID},
			MediaItems: []map[string]any{
				{"url": "https://cdn.example.com/1.jpg"},
				{"url": "https://cdn.example.com/2.jpg"},
				{"url": "https://cdn.example.com/3.jpg"},
			},
		})
		require.ErrorIs(t, err, ErrMediaInvalid)
	})
}

func TestTikTokPublishCapabilities(t *testing.T) {
	cases := []struct {
		scopes          string
		upload, publish bool
	}{
		{"user.info.basic,video.upload", true, false},
		{"user.info.basic,video.publish,video.upload", true, true},
		{"user.info.basic", false, false},
		{"", false, false},
		{" video.upload , video.publish ", true, true},
	}
	for _, tc := range cases {
		upload, publish := tiktokPublishCapabilities(tc.scopes)
		assert.Equal(t, tc.upload, upload, tc.scopes)
		assert.Equal(t, tc.publish, publish, tc.scopes)
	}
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
	assert.Equal(t, "failed", mapTikTokPublishStatus("FAIL"))
	assert.Equal(t, "sent_to_inbox", mapTikTokPublishStatus("SEND_TO_USER_INBOX"))
	assert.Equal(t, "processing", mapTikTokPublishStatus("PROCESSING_UPLOAD"))
	assert.Equal(t, "processing", mapTikTokPublishStatus("PROCESSING_DOWNLOAD"))
	assert.Equal(t, "processing", mapTikTokPublishStatus("PUBLISH_INITIATED"))
	assert.Equal(t, "", mapTikTokPublishStatus("SOMETHING_ELSE"))

	assert.False(t, isTerminalStatus("sent_to_inbox"))
	assert.False(t, isTerminalStatus("processing"))
	assert.False(t, isTerminalStatus("submitted"))
	assert.True(t, isTerminalStatus("published"))
	assert.True(t, isTerminalStatus("failed"))
}

func TestGetPostTikTokSentToInboxContinuesPolling(t *testing.T) {
	cases := []struct {
		name           string
		finalUpstream  string
		finalFail      string
		wantFinalState string
	}{
		{
			name:           "sent_to_inbox transitions to published on later refresh",
			finalUpstream:  "PUBLISH_COMPLETE",
			wantFinalState: "published",
		},
		{
			name:           "sent_to_inbox transitions to failed on later refresh",
			finalUpstream:  "FAILED",
			finalFail:      "user_cancelled",
			wantFinalState: "failed",
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupDirectDB(t)
			var statusCalls atomic.Int32
			statuses := []struct {
				status     string
				failReason string
			}{
				{status: "PROCESSING_UPLOAD"},
				{status: "SEND_TO_USER_INBOX"},
				{status: tc.finalUpstream, failReason: tc.finalFail},
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/post/publish/status/fetch/", func(w http.ResponseWriter, r *http.Request) {
				callIdx := int(statusCalls.Add(1)) - 1
				if callIdx >= len(statuses) {
					callIdx = len(statuses) - 1
				}
				cur := statuses[callIdx]
				w.Header().Set("Content-Type", "application/json")
				resp, _ := common.Marshal(map[string]any{
					"data": map[string]any{
						"status":      cur.status,
						"fail_reason": cur.failReason,
						"publish_id":  "pub-inbox-poll",
					},
					"error": map[string]any{"code": "ok", "message": ""},
				})
				_, _ = w.Write(resp)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			cfg := testDirectConfig(t, srv.URL)
			s := NewService(cfg)
			userID := 110 + idx
			require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: fmt.Sprintf("u%d", userID)}).Error)
			enc, err := SealToken("at-poll", cfg.SocialTokenEncKey)
			require.NoError(t, err)
			acc := &model.SocialAccount{
				UserId:               userID,
				Provider:             ProviderTikTokDirect,
				ProviderAccountID:    fmt.Sprintf("oid-poll-%d", idx),
				Platform:             "tiktok",
				Status:               model.SocialAccountStatusActive,
				AccessTokenEncrypted: enc,
				ScopesGranted:        "user.info.basic,video.upload",
			}
			require.NoError(t, model.DB.Create(acc).Error)
			post := &model.SocialPost{
				UserId:          userID,
				Provider:        ProviderTikTokDirect,
				ProviderPostID:  "pub-inbox-poll",
				Status:          "submitted",
				RequestSnapshot: fmt.Sprintf(`{"account_ids":[%d]}`, acc.ID),
			}
			require.NoError(t, model.CreateSocialPost(post))

			// Poll 1: PROCESSING_UPLOAD -> processing
			p1, err := s.GetPost(context.Background(), userID, ProviderTikTokDirect, post.ID, true)
			require.NoError(t, err)
			assert.Equal(t, "processing", p1.Status)
			assert.Contains(t, p1.ResultSnapshot, `"status":"PROCESSING_UPLOAD"`)

			// Poll 2: SEND_TO_USER_INBOX -> sent_to_inbox (non-terminal)
			p2, err := s.GetPost(context.Background(), userID, ProviderTikTokDirect, post.ID, true)
			require.NoError(t, err)
			assert.Equal(t, "sent_to_inbox", p2.Status)
			assert.Contains(t, p2.ResultSnapshot, `"status":"SEND_TO_USER_INBOX"`)

			// Poll 3: refresh=1 from sent_to_inbox MUST continue polling upstream
			p3, err := s.GetPost(context.Background(), userID, ProviderTikTokDirect, post.ID, true)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFinalState, p3.Status)
			assert.Contains(t, p3.ResultSnapshot, `"status":"`+tc.finalUpstream+`"`)
			assert.Equal(t, int32(3), statusCalls.Load())

			// Poll 4: once terminal (published/failed), refresh=1 does NOT call upstream again
			p4, err := s.GetPost(context.Background(), userID, ProviderTikTokDirect, post.ID, true)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFinalState, p4.Status)
			assert.Equal(t, int32(3), statusCalls.Load())
		})
	}
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

func TestHandleTikTokCallbackPartialAndEmptyScopesFailClosed(t *testing.T) {
	cases := []struct {
		name      string
		tokenBody string
		wantScope string
	}{
		{
			name:      "user denies video.upload",
			tokenBody: `{"access_token":"at-basic","refresh_token":"rt-basic","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-basic","scope":"user.info.basic","token_type":"Bearer"}`,
			wantScope: "user.info.basic",
		},
		{
			name:      "upstream omits scope",
			tokenBody: `{"access_token":"at-empty","refresh_token":"rt-empty","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-empty","token_type":"Bearer"}`,
			wantScope: "",
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupDirectDB(t)
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.tokenBody))
			})
			mux.HandleFunc("/v2/user/info/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-partial","display_name":"Basic User"}},"error":{"code":"ok","message":""}}`))
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			cfg := testDirectConfig(t, srv.URL)
			cfg.TikTokScopes = "user.info.basic,video.upload"
			s := NewService(cfg)
			userID := 70 + i
			require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "u_partial", Quota: 1000000}).Error)
			require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
				State:    "state-partial",
				UserId:   userID,
				Provider: ProviderTikTokDirect,
			}))

			res, err := s.HandleTikTokCallback(context.Background(), "code-partial", "state-partial")
			require.NoError(t, err)
			assert.Equal(t, tc.wantScope, res.Account.ScopesGranted)

			uploadDraft, publishDirect := s.TikTokCapabilities(res.Account)
			assert.False(t, uploadDraft)
			assert.False(t, publishDirect)

			_, err = s.CreatePost(context.Background(), userID, CreatePostInput{
				Provider:   ProviderTikTokDirect,
				Content:    "should be blocked",
				AccountIDs: []int64{res.Account.ID},
				PublishNow: true,
				MediaItems: []map[string]any{{"url": "https://cdn.example.com/v.mp4"}},
			})
			require.ErrorIs(t, err, ErrTikTokPublishScopeMissing)
		})
	}
}

func TestAccessTokenConcurrentRefreshSingleUpstreamCall(t *testing.T) {
	setupDirectDB(t)
	var refreshCalls atomic.Int32
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var startOnce sync.Once

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "rt-initial", r.Form.Get("refresh_token"))
		refreshCalls.Add(1)
		startOnce.Do(func() { close(refreshStarted) })
		<-releaseRefresh
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-rotated","refresh_token":"rt-rotated","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-race","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)
	require.NoError(t, model.DB.Create(&model.User{Id: 90, Username: "u90"}).Error)
	oldAccessEnc, err := SealToken("at-expiring", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	oldRefreshEnc, err := SealToken("rt-initial", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	expiringSoon := time.Now().UTC().Add(2 * time.Minute)
	acc := &model.SocialAccount{
		UserId:                90,
		Provider:              ProviderTikTokDirect,
		ProviderAccountID:     "oid-race",
		Platform:              "tiktok",
		Status:                model.SocialAccountStatusActive,
		AccessTokenEncrypted:  oldAccessEnc,
		RefreshTokenEncrypted: oldRefreshEnc,
		TokenExpiresAt:        &expiringSoon,
		ScopesGranted:         "user.info.basic",
	}
	require.NoError(t, model.DB.Create(acc).Error)

	const workers = 12
	tokens := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		accCopy := *acc
		wg.Go(func() {
			tokens[i], errs[i] = s.accessTokenForAccount(context.Background(), &accCopy)
		})
	}

	<-refreshStarted
	close(releaseRefresh)
	wg.Wait()

	assert.Equal(t, int32(1), refreshCalls.Load(), "concurrent callers in expiry window must trigger exactly one RefreshToken")
	for i := range workers {
		require.NoError(t, errs[i])
		assert.Equal(t, "at-rotated", tokens[i])
	}

	saved, err := model.GetSocialAccountByIDForUser(acc.ID, 90)
	require.NoError(t, err)
	gotRefresh, err := OpenToken(saved.RefreshTokenEncrypted, cfg.SocialTokenEncKey)
	require.NoError(t, err)
	assert.Equal(t, "rt-rotated", gotRefresh)
	assert.Equal(t, "user.info.basic,video.upload", saved.ScopesGranted)
}

func TestAccessTokenConcurrentRefreshMultiInstanceRedisLock(t *testing.T) {
	setupDirectDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	oldEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, rdb
	t.Cleanup(func() {
		common.RedisEnabled, common.RDB = oldEnabled, oldRDB
		_ = rdb.Close()
	})

	var refreshCalls atomic.Int32
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var startOnce sync.Once

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
		refreshCalls.Add(1)
		startOnce.Do(func() { close(refreshStarted) })
		<-releaseRefresh
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-redis-new","refresh_token":"rt-redis-new","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-redis","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	// Two separate Service instances simulate two gateway processes sharing DB + Redis.
	s1 := NewService(cfg)
	s2 := NewService(cfg)

	require.NoError(t, model.DB.Create(&model.User{Id: 96, Username: "u96"}).Error)
	oldAccessEnc, err := SealToken("at-redis-old", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	oldRefreshEnc, err := SealToken("rt-redis-old", cfg.SocialTokenEncKey)
	require.NoError(t, err)
	expiringSoon := time.Now().UTC().Add(2 * time.Minute)
	acc := &model.SocialAccount{
		UserId:                96,
		Provider:              ProviderTikTokDirect,
		ProviderAccountID:     "oid-redis",
		Platform:              "tiktok",
		Status:                model.SocialAccountStatusActive,
		AccessTokenEncrypted:  oldAccessEnc,
		RefreshTokenEncrypted: oldRefreshEnc,
		TokenExpiresAt:        &expiringSoon,
		ScopesGranted:         "user.info.basic,video.upload",
	}
	require.NoError(t, model.DB.Create(acc).Error)

	var tok1, tok2 string
	var err1, err2 error
	var wg sync.WaitGroup
	acc1, acc2 := *acc, *acc
	wg.Go(func() { tok1, err1 = s1.accessTokenForAccount(context.Background(), &acc1) })
	<-refreshStarted
	wg.Go(func() { tok2, err2 = s2.accessTokenForAccount(context.Background(), &acc2) })
	// Give s2 a moment to contend on the Redis lock before releasing s1.
	time.Sleep(80 * time.Millisecond)
	close(releaseRefresh)
	wg.Wait()

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "at-redis-new", tok1)
	assert.Equal(t, "at-redis-new", tok2)
	assert.Equal(t, int32(1), refreshCalls.Load(), "Redis distributed lock must serialize cross-instance refresh")
	assert.Empty(t, mr.Keys(), "Redis lock key must be released after refresh")

	// Token-verified unlock must not delete a lock acquired by another holder.
	release, err := acquireTikTokRefreshRedisLock(context.Background(), acc.ID)
	require.NoError(t, err)
	keys := mr.Keys()
	require.Len(t, keys, 1)
	require.NoError(t, mr.Set(keys[0], "other-holder"))
	release()
	val, err := mr.Get(keys[0])
	require.NoError(t, err)
	assert.Equal(t, "other-holder", val)
}

func TestAccessTokenRefreshOptimisticUpdateAndFailureSafety(t *testing.T) {
	t.Run("optimistic update yields to concurrent DB rotation", func(t *testing.T) {
		setupDirectDB(t)
		cfg := testDirectConfig(t, "http://placeholder")
		require.NoError(t, model.DB.Create(&model.User{Id: 91, Username: "u91"}).Error)
		oldAccessEnc, err := SealToken("at-old-91", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		oldRefreshEnc, err := SealToken("rt-old-91", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		expiringSoon := time.Now().UTC().Add(2 * time.Minute)
		acc := &model.SocialAccount{
			UserId:                91,
			Provider:              ProviderTikTokDirect,
			ProviderAccountID:     "oid-91",
			Platform:              "tiktok",
			Status:                model.SocialAccountStatusActive,
			AccessTokenEncrypted:  oldAccessEnc,
			RefreshTokenEncrypted: oldRefreshEnc,
			TokenExpiresAt:        &expiringSoon,
			ScopesGranted:         "user.info.basic,video.upload",
		}
		require.NoError(t, model.DB.Create(acc).Error)

		mux := http.NewServeMux()
		mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
			// Simulate another instance rotating the DB row while this refresh is in flight.
			otherAccessEnc, e1 := SealToken("at-from-peer", cfg.SocialTokenEncKey)
			require.NoError(t, e1)
			otherRefreshEnc, e2 := SealToken("rt-from-peer", cfg.SocialTokenEncKey)
			require.NoError(t, e2)
			future := time.Now().UTC().Add(2 * time.Hour)
			require.NoError(t, model.DB.Model(&model.SocialAccount{}).Where("id = ?", acc.ID).Updates(map[string]any{
				"access_token_encrypted":  otherAccessEnc,
				"refresh_token_encrypted": otherRefreshEnc,
				"token_expires_at":        future,
			}).Error)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"at-stale-loser","refresh_token":"rt-stale-loser","expires_in":7200,"open_id":"oid-91","token_type":"Bearer"}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()
		cfg.TikTokBaseURL = srv.URL
		s := NewService(cfg)

		gotToken, err := s.accessTokenForAccount(context.Background(), acc)
		require.NoError(t, err)
		assert.Equal(t, "at-from-peer", gotToken)

		saved, err := model.GetSocialAccountByIDForUser(acc.ID, 91)
		require.NoError(t, err)
		gotRefresh, err := OpenToken(saved.RefreshTokenEncrypted, cfg.SocialTokenEncKey)
		require.NoError(t, err)
		assert.Equal(t, "rt-from-peer", gotRefresh, "conditional update must not overwrite newer rotation")
	})

	t.Run("expired token never falls back on refresh failure and never leaks secrets", func(t *testing.T) {
		setupDirectDB(t)
		const secretAccess = "act.super-secret-expired-token"
		const secretRefresh = "rft.super-secret-refresh-token"
		mux := http.NewServeMux()
		mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Refresh token is expired or invalid.","log_id":"log-fail"}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := testDirectConfig(t, srv.URL)
		s := NewService(cfg)
		require.NoError(t, model.DB.Create(&model.User{Id: 92, Username: "u92"}).Error)
		accessEnc, err := SealToken(secretAccess, cfg.SocialTokenEncKey)
		require.NoError(t, err)
		refreshEnc, err := SealToken(secretRefresh, cfg.SocialTokenEncKey)
		require.NoError(t, err)
		expired := time.Now().UTC().Add(-time.Minute)
		acc := &model.SocialAccount{
			UserId:                92,
			Provider:              ProviderTikTokDirect,
			ProviderAccountID:     "oid-92",
			Platform:              "tiktok",
			Status:                model.SocialAccountStatusActive,
			AccessTokenEncrypted:  accessEnc,
			RefreshTokenEncrypted: refreshEnc,
			TokenExpiresAt:        &expired,
			ScopesGranted:         "user.info.basic,video.upload",
		}
		require.NoError(t, model.DB.Create(acc).Error)

		_, err = s.accessTokenForAccount(context.Background(), acc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_grant")
		assert.NotContains(t, err.Error(), secretAccess)
		assert.NotContains(t, err.Error(), secretRefresh)
	})

	t.Run("invalid_grant inside pre-expiry window fails closed when not refreshed by peer", func(t *testing.T) {
		setupDirectDB(t)
		mux := http.NewServeMux()
		mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Refresh token revoked","log_id":"log-revoked"}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := testDirectConfig(t, srv.URL)
		s := NewService(cfg)
		require.NoError(t, model.DB.Create(&model.User{Id: 93, Username: "u93"}).Error)
		accessEnc, err := SealToken("at-93", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		refreshEnc, err := SealToken("rt-93", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		expiringSoon := time.Now().UTC().Add(3 * time.Minute)
		acc := &model.SocialAccount{
			UserId:                93,
			Provider:              ProviderTikTokDirect,
			ProviderAccountID:     "oid-93",
			Platform:              "tiktok",
			Status:                model.SocialAccountStatusActive,
			AccessTokenEncrypted:  accessEnc,
			RefreshTokenEncrypted: refreshEnc,
			TokenExpiresAt:        &expiringSoon,
		}
		require.NoError(t, model.DB.Create(acc).Error)

		_, err = s.accessTokenForAccount(context.Background(), acc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_grant")
	})

	t.Run("refresh failure recovers when peer already rotated DB row", func(t *testing.T) {
		setupDirectDB(t)
		cfg := testDirectConfig(t, "http://placeholder")
		require.NoError(t, model.DB.Create(&model.User{Id: 94, Username: "u94"}).Error)
		oldAccessEnc, err := SealToken("at-old-94", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		oldRefreshEnc, err := SealToken("rt-old-94", cfg.SocialTokenEncKey)
		require.NoError(t, err)
		expired := time.Now().UTC().Add(-time.Minute)
		acc := &model.SocialAccount{
			UserId:                94,
			Provider:              ProviderTikTokDirect,
			ProviderAccountID:     "oid-94",
			Platform:              "tiktok",
			Status:                model.SocialAccountStatusActive,
			AccessTokenEncrypted:  oldAccessEnc,
			RefreshTokenEncrypted: oldRefreshEnc,
			TokenExpiresAt:        &expired,
		}
		require.NoError(t, model.DB.Create(acc).Error)

		mux := http.NewServeMux()
		mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
			// Another instance rotated before TikTok rejected our stale refresh token.
			peerAccessEnc, e1 := SealToken("at-peer-94", cfg.SocialTokenEncKey)
			require.NoError(t, e1)
			peerRefreshEnc, e2 := SealToken("rt-peer-94", cfg.SocialTokenEncKey)
			require.NoError(t, e2)
			future := time.Now().UTC().Add(2 * time.Hour)
			require.NoError(t, model.DB.Model(&model.SocialAccount{}).Where("id = ?", acc.ID).Updates(map[string]any{
				"access_token_encrypted":  peerAccessEnc,
				"refresh_token_encrypted": peerRefreshEnc,
				"token_expires_at":        future,
			}).Error)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Refresh token already used","log_id":"log-peer"}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()
		cfg.TikTokBaseURL = srv.URL
		s := NewService(cfg)

		gotToken, err := s.accessTokenForAccount(context.Background(), acc)
		require.NoError(t, err)
		assert.Equal(t, "at-peer-94", gotToken)
	})
}

func TestTikTokOAuthStartRejectsUnapprovedAuthorizeURLAndDomainMismatch(t *testing.T) {
	setupDirectDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 95, Username: "u95", Quota: 1000000}).Error)

	for _, badBase := range []string{
		"https://evil.example/v2/auth/authorize/",
		"http://www.tiktok.com/v2/auth/authorize/",
		"https://www.tiktok.com.evil.example/v2/auth/authorize/",
		"https://www.tiktok.com/v2/auth/authorize/?evil=1",
		"https://www.tiktok.com/redirect",
	} {
		cfg := testDirectConfig(t, "http://stub")
		cfg.TikTokAuthorizeURL = badBase
		s := NewService(cfg)
		_, err := s.Connect(context.Background(), 95, ProviderTikTokDirect, "tiktok", "")
		require.ErrorIs(t, err, ErrTikTokAuthorizeURLInvalid, badBase)

		require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
			State:     "t-bad-base",
			UserId:    95,
			Provider:  ProviderTikTokDirectStart,
			ExpiresAt: time.Now().UTC().Add(time.Minute),
		}))
		_, err = s.StartTikTokOAuth(context.Background(), "t-bad-base")
		require.ErrorIs(t, err, ErrTikTokAuthorizeURLInvalid, badBase)
		require.NoError(t, model.DB.Exec("DELETE FROM social_oauth_states").Error)
	}

	// Launch ticket cannot be consumed as callback state (domain separation).
	cfg := testDirectConfig(t, "http://stub")
	s := NewService(cfg)
	startURL, err := s.Connect(context.Background(), 95, ProviderTikTokDirect, "tiktok", "")
	require.NoError(t, err)
	u, err := url.Parse(startURL)
	require.NoError(t, err)
	ticket := u.Query().Get("ticket")
	_, err = s.HandleTikTokCallback(context.Background(), "code-1", ticket)
	require.Error(t, err)
}

func TestHandleTikTokCallbackSeatBilling(t *testing.T) {
	setupDirectDB(t)
	withSeatBilling(t, "seat")

	var revokeCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		code := r.Form.Get("code")
		oid := "oid-" + code
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"at-%s","refresh_token":"rt-%s","expires_in":7200,"refresh_expires_in":15552000,"open_id":"%s","scope":"user.info.basic,video.upload","token_type":"Bearer"}`, code, code, oid)
	})
	mux.HandleFunc("/v2/user/info/", func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer at-")
		oid := "oid-" + auth
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":{"user":{"open_id":"%s","display_name":"Creator %s","username":"user_%s"}},"error":{"code":"ok","message":""}}`, oid, auth, auth)
	})
	mux.HandleFunc("/v2/oauth/revoke/", func(w http.ResponseWriter, r *http.Request) {
		revokeCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := testDirectConfig(t, srv.URL)
	s := NewService(cfg)

	// 1) First callback charges 20 credits (1_000_000) and links seat_id, last_account_id, swap_count=0.
	u1 := seedSeatUser(t, 1_000_000)
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "seat-st-1",
		UserId:   u1.Id,
		Provider: ProviderTikTokDirect,
	}))
	res, err := s.HandleTikTokCallback(context.Background(), "first", "seat-st-1")
	require.NoError(t, err)
	require.NotNil(t, res.Account)
	require.NotNil(t, res.Account.SeatID)
	assert.NotEmpty(t, res.Account.AccessTokenEncrypted)

	var afterU1 model.User
	require.NoError(t, model.DB.First(&afterU1, u1.Id).Error)
	assert.Equal(t, 0, afterU1.Quota)

	var seat model.SocialSeat
	require.NoError(t, model.DB.First(&seat, *res.Account.SeatID).Error)
	assert.Equal(t, model.SocialSeatStatusOccupied, seat.Status)
	require.NotNil(t, seat.LastAccountID)
	assert.Equal(t, res.Account.ID, *seat.LastAccountID)
	assert.Equal(t, 0, seat.SwapCount)
	assert.Equal(t, 1_000_000, seat.OpenedQuota)

	// 2) Concurrent 2x callbacks with balance for only 1 seat:
	// exactly 1 succeeds, 1 fails with ErrSeatQuota + triggers TikTok Revoke + leaves no orphan account.
	u2 := seedSeatUser(t, 1_000_000)
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "seat-conc-1",
		UserId:   u2.Id,
		Provider: ProviderTikTokDirect,
	}))
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "seat-conc-2",
		UserId:   u2.Id,
		Provider: ProviderTikTokDirect,
	}))

	type cbOutcome struct {
		res *TikTokCallbackResult
		err error
	}
	outcomes := make([]cbOutcome, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, tc := range []struct{ code, state string }{
		{"conc1", "seat-conc-1"},
		{"conc2", "seat-conc-2"},
	} {
		wg.Go(func() {
			<-start
			r, e := s.HandleTikTokCallback(context.Background(), tc.code, tc.state)
			outcomes[i] = cbOutcome{res: r, err: e}
		})
	}
	close(start)
	wg.Wait()

	var successes, quotaFails int
	for _, o := range outcomes {
		if o.err == nil {
			successes++
			require.NotNil(t, o.res)
			require.NotNil(t, o.res.Account)
			require.NotNil(t, o.res.Account.SeatID)
		} else {
			assert.ErrorIs(t, o.err, ErrSeatQuota)
			quotaFails++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, quotaFails)
	assert.EqualValues(t, 1, revokeCalls.Load(), "failed concurrent callback must revoke upstream token")

	var u2Accounts int64
	require.NoError(t, model.DB.Model(&model.SocialAccount{}).Where("user_id = ?", u2.Id).Count(&u2Accounts).Error)
	assert.EqualValues(t, 1, u2Accounts, "failed concurrent callback must not leave orphan account")

	var afterU2 model.User
	require.NoError(t, model.DB.First(&afterU2, u2.Id).Error)
	assert.Equal(t, 0, afterU2.Quota)

	// 3) Concurrent 2x callbacks for the SAME open_id with balance for 2 seats:
	// must charge only once, create only 1 occupied seat, and never double-bill.
	u3 := seedSeatUser(t, 2_000_000)
	sameRevokeBefore := revokeCalls.Load()
	sameMux := http.NewServeMux()
	sameMux.HandleFunc("/v2/oauth/token/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-same-oid","refresh_token":"rt-same-oid","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-same-concurrent","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	})
	sameMux.HandleFunc("/v2/user/info/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-same-concurrent","display_name":"Same Creator","username":"same_creator"}},"error":{"code":"ok","message":""}}`))
	})
	sameSrv := httptest.NewServer(sameMux)
	defer sameSrv.Close()
	sameSvc := NewService(testDirectConfig(t, sameSrv.URL))

	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "same-conc-1",
		UserId:   u3.Id,
		Provider: ProviderTikTokDirect,
	}))
	require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:    "same-conc-2",
		UserId:   u3.Id,
		Provider: ProviderTikTokDirect,
	}))

	sameOutcomes := make([]cbOutcome, 2)
	sameStart := make(chan struct{})
	var sameWG sync.WaitGroup
	for i, stName := range []string{"same-conc-1", "same-conc-2"} {
		sameWG.Go(func() {
			<-sameStart
			r, e := sameSvc.HandleTikTokCallback(context.Background(), "same-code", stName)
			sameOutcomes[i] = cbOutcome{res: r, err: e}
		})
	}
	close(sameStart)
	sameWG.Wait()

	for _, o := range sameOutcomes {
		require.NoError(t, o.err)
		require.NotNil(t, o.res)
		require.NotNil(t, o.res.Account)
		require.NotNil(t, o.res.Account.SeatID)
	}
	assert.Equal(t, sameOutcomes[0].res.Account.ID, sameOutcomes[1].res.Account.ID)
	assert.Equal(t, *sameOutcomes[0].res.Account.SeatID, *sameOutcomes[1].res.Account.SeatID)
	assert.Equal(t, sameRevokeBefore, revokeCalls.Load())

	var u3Seats int64
	require.NoError(t, model.DB.Model(&model.SocialSeat{}).Where("user_id = ? AND status = ?", u3.Id, model.SocialSeatStatusOccupied).Count(&u3Seats).Error)
	assert.EqualValues(t, 1, u3Seats, "concurrent same-account callbacks must open at most one occupied seat")

	var afterU3 model.User
	require.NoError(t, model.DB.First(&afterU3, u3.Id).Error)
	assert.Equal(t, 1_000_000, afterU3.Quota, "concurrent same-account callbacks must charge only once")
}
