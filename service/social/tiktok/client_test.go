package tiktok

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("https://www.tiktok.com/v2/auth/authorize/", "ck123", "https://omnimux.ai/api/social/v1/oauth/tiktok/callback", "user.info.basic,video.upload", "state-abc")
	assert.Contains(t, u, "client_key=ck123")
	assert.Contains(t, u, "response_type=code")
	assert.Contains(t, u, "scope=user.info.basic%2Cvideo.upload")
	assert.Contains(t, u, "state=state-abc")
	assert.True(t, strings.HasPrefix(u, "https://www.tiktok.com/v2/auth/authorize/?"))
}

func TestExchangeTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/oauth/token/", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "code123", r.Form.Get("code"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"access_token":"at-1","refresh_token":"rt-1","expires_in":86400,"open_id":"openid-1","scope":"user.info.basic,video.upload"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.ExchangeToken(context.Background(), "ck", "cs", "code123", "https://cb")
	require.NoError(t, err)
	assert.Equal(t, "at-1", tk.AccessToken)
	assert.Equal(t, "rt-1", tk.RefreshToken)
	assert.Equal(t, "openid-1", tk.OpenID)
}

func TestExchangeTokenTopLevelShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-top","refresh_token":"rt-top","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid-top","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.ExchangeToken(context.Background(), "ck", "cs", "code", "https://cb")
	require.NoError(t, err)
	assert.Equal(t, "at-top", tk.AccessToken)
	assert.Equal(t, "rt-top", tk.RefreshToken)
	assert.Equal(t, int64(7200), tk.ExpiresIn)
	assert.Equal(t, int64(15552000), tk.RefreshExpiresIn)
	assert.Equal(t, "oid-top", tk.OpenID)
	assert.Equal(t, "user.info.basic,video.upload", tk.Scope)
	assert.Equal(t, "Bearer", tk.TokenType)
}

func TestExchangeTokenFlatError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "HTTP 400 flat string error",
			status: http.StatusBadRequest,
			body:   `{"error":"invalid_grant","error_description":"The authorization code is invalid or expired.","log_id":"log-400"}`,
		},
		{
			name:   "HTTP 200 flat string error",
			status: http.StatusOK,
			body:   `{"error":"invalid_grant","error_description":"The refresh token is invalid.","log_id":"log-200"}`,
		},
		{
			name:   "HTTP 401 nested object error",
			status: http.StatusUnauthorized,
			body:   `{"error":{"code":"access_token_invalid","message":"The access token is invalid.","log_id":"log-401"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := NewClient(srv.URL).ExchangeToken(context.Background(), "ck", "cs", "code", "https://cb")
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tc.status, apiErr.Status)
			assert.Equal(t, "token", apiErr.Endpoint)
			assert.NotContains(t, apiErr.Error(), "HTTP 0")
			assert.NotEmpty(t, apiErr.Code)
			assert.NotEmpty(t, apiErr.Message)
			assert.NotEmpty(t, apiErr.LogID)
		})
	}
}

func TestExchangeTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_grant","message":"bad code"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.ExchangeToken(context.Background(), "ck", "cs", "code", "https://cb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRefreshToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "rt-old", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","expires_in":7200,"refresh_expires_in":15552000,"open_id":"oid","scope":"user.info.basic,video.upload","token_type":"Bearer"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	tk, err := c.RefreshToken(context.Background(), "ck", "cs", "rt-old")
	require.NoError(t, err)
	assert.Equal(t, "at-new", tk.AccessToken)
	assert.Equal(t, "rt-new", tk.RefreshToken)
	assert.Equal(t, int64(15552000), tk.RefreshExpiresIn)
	assert.Equal(t, "user.info.basic,video.upload", tk.Scope)
	assert.Equal(t, "Bearer", tk.TokenType)
}

func TestGetUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		// Model a token granted only user.info.basic: extended fields fail OAuth binding.
		for _, field := range strings.Split(r.URL.Query().Get("fields"), ",") {
			switch field {
			case "open_id", "union_id", "avatar_url", "display_name":
			default:
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"code":"scope_not_authorized","message":"requires extended profile scope"}}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"data":{"user":{"open_id":"oid-1","display_name":"Test User"}}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	u, err := c.GetUserInfo(context.Background(), "at-1")
	require.NoError(t, err)
	assert.Equal(t, "oid-1", u.OpenID)
	assert.Equal(t, "Test User", u.DisplayName)
}

func TestGetUserInfoHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"access_token_invalid","message":"expired"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.GetUserInfo(context.Background(), "bad")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.Status)
	assert.Equal(t, "access_token_invalid", apiErr.Code)
}

func TestRevoke(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotForm = r.Form
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	require.NoError(t, c.Revoke(context.Background(), "ck", "cs", "at-1"))
	assert.Equal(t, "at-1", gotForm.Get("token"))
}

func TestPublishVideoByURL(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/post/publish/video/init/", r.URL.Path)
		assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-1"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	res, err := c.PublishVideoByURL(context.Background(), "at-1", "hello", "https://cdn.example.com/v.mp4", "")
	require.NoError(t, err)
	assert.Equal(t, "pub-1", res.PublishID)
	assert.Contains(t, gotBody, `"privacy_level":"SELF_ONLY"`)
	assert.Contains(t, gotBody, `"video_url":"https://cdn.example.com/v.mp4"`)
}

func TestPublishVideoToInboxByURL(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/post/publish/inbox/video/init/", r.URL.Path)
		assert.Equal(t, "Bearer at-1", r.Header.Get("Authorization"))
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-draft-1"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	res, err := c.PublishVideoToInboxByURL(context.Background(), "at-1", "https://cdn.example.com/v.mp4")
	require.NoError(t, err)
	assert.Equal(t, "pub-draft-1", res.PublishID)
	assert.Contains(t, gotBody, `"source":"PULL_FROM_URL"`)
	assert.Contains(t, gotBody, `"video_url":"https://cdn.example.com/v.mp4"`)
	assert.NotContains(t, gotBody, "post_info")
}

func TestPublishPhotoToInboxByURL(t *testing.T) {
	t.Run("multi-photo carousel with explicit zero cover index and false is_aigc", func(t *testing.T) {
		var gotPath, gotAuth, gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			buf := make([]byte, 8192)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-photo-draft-1"}}`))
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		zero := 0
		isAIGC := false
		res, err := c.PublishPhotoToInboxByURL(
			context.Background(),
			"at-photo-1",
			[]string{
				"https://omnimux.ai/api/social/v1/public/media/tok-1",
				"https://omnimux.ai/api/social/v1/public/media/tok-2",
			},
			&PhotoDraftOptions{
				Title:           "Spring Lookbook",
				Description:     "Two-slide carousel #ootd",
				PhotoCoverIndex: &zero,
				IsAIGC:          &isAIGC,
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "pub-photo-draft-1", res.PublishID)
		assert.Equal(t, "/v2/post/publish/content/init/", gotPath)
		assert.Equal(t, "Bearer at-photo-1", gotAuth)
		assert.Contains(t, gotBody, `"post_mode":"MEDIA_UPLOAD"`)
		assert.Contains(t, gotBody, `"media_type":"PHOTO"`)
		assert.Contains(t, gotBody, `"source":"PULL_FROM_URL"`)
		assert.Contains(t, gotBody, `"photo_images":["https://omnimux.ai/api/social/v1/public/media/tok-1","https://omnimux.ai/api/social/v1/public/media/tok-2"]`)
		assert.Contains(t, gotBody, `"photo_cover_index":0`)
		assert.Contains(t, gotBody, `"title":"Spring Lookbook"`)
		assert.Contains(t, gotBody, `"description":"Two-slide carousel #ootd"`)
		assert.Contains(t, gotBody, `"is_aigc":false`)
		assert.NotContains(t, gotBody, "privacy_level")
		assert.NotContains(t, gotBody, "disable_comment")
		assert.NotContains(t, gotBody, "disable_duet")
		assert.NotContains(t, gotBody, "disable_stitch")
		assert.NotContains(t, gotBody, "brand_content_toggle")
		assert.NotContains(t, gotBody, "brand_organic_toggle")
	})

	t.Run("nil options omits post_info and photo_cover_index", func(t *testing.T) {
		var gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-photo-draft-min"}}`))
		}))
		defer srv.Close()

		c := NewClient(srv.URL)
		res, err := c.PublishPhotoToInboxByURL(context.Background(), "at-min", []string{"https://omnimux.ai/api/social/v1/public/media/tok-only"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "pub-photo-draft-min", res.PublishID)
		assert.Contains(t, gotBody, `"post_mode":"MEDIA_UPLOAD"`)
		assert.Contains(t, gotBody, `"media_type":"PHOTO"`)
		assert.NotContains(t, gotBody, "post_info")
		assert.NotContains(t, gotBody, "photo_cover_index")
	})

	t.Run("empty photo_images fails locally", func(t *testing.T) {
		c := NewClient("http://127.0.0.1:1")
		_, err := c.PublishPhotoToInboxByURL(context.Background(), "at-1", nil, nil)
		require.Error(t, err)
	})
}

func TestPublishWithOptionsMapsComplianceFields(t *testing.T) {
	var gotVideoBody, gotPhotoBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		if r.URL.Path == "/v2/post/publish/video/init/" {
			gotVideoBody = string(buf[:n])
		} else {
			gotPhotoBody = string(buf[:n])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-opt"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	f := false
	tr := true
	opts := &PostOptions{
		PrivacyLevel:       "PUBLIC_TO_EVERYONE",
		DisableComment:     &f,
		DisableDuet:        &tr,
		DisableStitch:      &tr,
		BrandContentToggle: &tr,
		BrandOrganicToggle: &f,
	}
	_, err := c.PublishVideoByURLWithOptions(context.Background(), "at-1", "v title", "https://omnimux.ai/v.mp4", opts)
	require.NoError(t, err)
	assert.Contains(t, gotVideoBody, `"privacy_level":"PUBLIC_TO_EVERYONE"`)
	assert.Contains(t, gotVideoBody, `"disable_comment":false`)
	assert.Contains(t, gotVideoBody, `"disable_duet":true`)
	assert.Contains(t, gotVideoBody, `"disable_stitch":true`)
	assert.Contains(t, gotVideoBody, `"brand_content_toggle":true`)
	assert.Contains(t, gotVideoBody, `"brand_organic_toggle":false`)

	_, err = c.PublishPhotoByURLWithOptions(context.Background(), "at-1", "p title", []string{"https://omnimux.ai/p.jpg"}, opts)
	require.NoError(t, err)
	assert.Contains(t, gotPhotoBody, `"disable_comment":false`)
	assert.NotContains(t, gotPhotoBody, `"disable_duet"`, "photo posts must not send duet/stitch flags")
	assert.NotContains(t, gotPhotoBody, `"disable_stitch"`)
}

func TestGetPublishStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":"PUBLISH_COMPLETE","publish_id":"pub-1"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	st, err := c.GetPublishStatus(context.Background(), "at-1", "pub-1")
	require.NoError(t, err)
	assert.Equal(t, "PUBLISH_COMPLETE", st.Status)
}

func TestQueryCreatorInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"creator_nickname":"n","creator_username":"u","privacy_level_options":["SELF_ONLY","FOLLOWER_OF_CREATOR"],"comment_disabled":false,"duet_disabled":true,"stitch_disabled":true,"max_video_post_duration_sec":600}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	info, err := c.QueryCreatorInfo(context.Background(), "at-1")
	require.NoError(t, err)
	assert.Equal(t, "u", info.CreatorUsername)
	assert.False(t, info.CommentDisabled)
	assert.True(t, info.DuetDisabled)
	assert.True(t, info.StitchDisabled)
}

func TestHTTP200BusinessErrorsPopulateStatusEndpointAndLogID(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name         string
		body         string
		wantEndpoint string
		wantCode     string
		wantMessage  string
		wantLogID    string
		call         func(*Client) error
	}{
		{
			name:         "ExchangeToken nested error",
			body:         `{"error":{"code":"invalid_grant","message":"code expired","log_id":"log-exchange-200"}}`,
			wantEndpoint: "token",
			wantCode:     "invalid_grant",
			wantMessage:  "code expired",
			wantLogID:    "log-exchange-200",
			call: func(c *Client) error {
				_, err := c.ExchangeToken(ctx, "ck", "cs", "code", "https://cb")
				return err
			},
		},
		{
			name:         "RefreshToken flat string error keeps refresh endpoint",
			body:         `{"error":"invalid_grant","error_description":"refresh token revoked","log_id":"log-refresh-flat-200"}`,
			wantEndpoint: "refresh",
			wantCode:     "invalid_grant",
			wantMessage:  "refresh token revoked",
			wantLogID:    "log-refresh-flat-200",
			call: func(c *Client) error {
				_, err := c.RefreshToken(ctx, "ck", "cs", "rt-1")
				return err
			},
		},
		{
			name:         "RefreshToken nested error_code fallback keeps refresh endpoint",
			body:         `{"error":{"error_code":"refresh_token_invalid","message":"bad refresh","log_id":"log-refresh-nested-200"}}`,
			wantEndpoint: "refresh",
			wantCode:     "refresh_token_invalid",
			wantMessage:  "bad refresh",
			wantLogID:    "log-refresh-nested-200",
			call: func(c *Client) error {
				_, err := c.RefreshToken(ctx, "ck", "cs", "rt-1")
				return err
			},
		},
		{
			name:         "GetUserInfo nested error",
			body:         `{"data":{"user":{}},"error":{"code":"access_token_invalid","message":"token invalid","log_id":"log-userinfo-200"}}`,
			wantEndpoint: "user_info",
			wantCode:     "access_token_invalid",
			wantMessage:  "token invalid",
			wantLogID:    "log-userinfo-200",
			call: func(c *Client) error {
				_, err := c.GetUserInfo(ctx, "at-1")
				return err
			},
		},
		{
			name:         "GetUserInfo error_code and top-level log_id fallback",
			body:         `{"data":{"user":{}},"error":{"error_code":"scope_not_authorized","message":"missing scope"},"log_id":"log-userinfo-top-200"}`,
			wantEndpoint: "user_info",
			wantCode:     "scope_not_authorized",
			wantMessage:  "missing scope",
			wantLogID:    "log-userinfo-top-200",
			call: func(c *Client) error {
				_, err := c.GetUserInfo(ctx, "at-1")
				return err
			},
		},
		{
			name:         "QueryCreatorInfo nested error",
			body:         `{"data":{},"error":{"code":"spam_risk_too_many_posts","message":"too many posts today","log_id":"log-creator-200"}}`,
			wantEndpoint: "creator_info",
			wantCode:     "spam_risk_too_many_posts",
			wantMessage:  "too many posts today",
			wantLogID:    "log-creator-200",
			call: func(c *Client) error {
				_, err := c.QueryCreatorInfo(ctx, "at-1")
				return err
			},
		},
		{
			name:         "PublishVideoByURLWithOptions nested error",
			body:         `{"data":{},"error":{"code":"unaudited_client_can_only_post_to_private_accounts","message":"private accounts only","log_id":"log-pub-video-200"}}`,
			wantEndpoint: "publish",
			wantCode:     "unaudited_client_can_only_post_to_private_accounts",
			wantMessage:  "private accounts only",
			wantLogID:    "log-pub-video-200",
			call: func(c *Client) error {
				_, err := c.PublishVideoByURLWithOptions(ctx, "at-1", "title", "https://example.com/v.mp4", &PostOptions{PrivacyLevel: "SELF_ONLY"})
				return err
			},
		},
		{
			name:         "PublishPhotoByURLWithOptions error_code fallback",
			body:         `{"data":{},"error":{"error_code":"url_ownership_unverified","message":"verify domain","log_id":"log-pub-photo-200"}}`,
			wantEndpoint: "publish",
			wantCode:     "url_ownership_unverified",
			wantMessage:  "verify domain",
			wantLogID:    "log-pub-photo-200",
			call: func(c *Client) error {
				_, err := c.PublishPhotoByURLWithOptions(ctx, "at-1", "title", []string{"https://example.com/p.jpg"}, &PostOptions{PrivacyLevel: "SELF_ONLY"})
				return err
			},
		},
		{
			name:         "GetPublishStatus nested error",
			body:         `{"data":{},"error":{"code":"rate_limit_exceeded","message":"slow down","log_id":"log-status-200"}}`,
			wantEndpoint: "publish_status",
			wantCode:     "rate_limit_exceeded",
			wantMessage:  "slow down",
			wantLogID:    "log-status-200",
			call: func(c *Client) error {
				_, err := c.GetPublishStatus(ctx, "at-1", "pub-1")
				return err
			},
		},
		{
			name:         "PublishVideoToInboxByURL nested error",
			body:         `{"data":{},"error":{"code":"spam_risk_too_many_pending_share","message":"too many drafts","log_id":"log-inbox-200"}}`,
			wantEndpoint: "video_inbox_init",
			wantCode:     "spam_risk_too_many_pending_share",
			wantMessage:  "too many drafts",
			wantLogID:    "log-inbox-200",
			call: func(c *Client) error {
				_, err := c.PublishVideoToInboxByURL(ctx, "at-1", "https://example.com/v.mp4")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			err := tc.call(NewClient(srv.URL))
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusOK, apiErr.Status)
			assert.Equal(t, tc.wantEndpoint, apiErr.Endpoint)
			assert.Equal(t, tc.wantCode, apiErr.Code)
			assert.Equal(t, tc.wantMessage, apiErr.Message)
			assert.Equal(t, tc.wantLogID, apiErr.LogID)
			assert.NotContains(t, apiErr.Error(), "HTTP 0")
			assert.Contains(t, apiErr.Error(), "HTTP 200")
		})
	}

	t.Run("parseToken defaults zero status to HTTP 200 and preserves endpoint", func(t *testing.T) {
		_, err := parseToken([]byte(`{"error":{"code":"invalid_grant","message":"bad","log_id":"log-zero-status"}}`), "refresh", 0)
		var apiErr *APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusOK, apiErr.Status)
		assert.Equal(t, "refresh", apiErr.Endpoint)
		assert.Equal(t, "invalid_grant", apiErr.Code)
		assert.Equal(t, "log-zero-status", apiErr.LogID)
		assert.NotContains(t, apiErr.Error(), "HTTP 0")
	})
}
