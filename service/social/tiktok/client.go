package tiktok

// fork: tiktok-direct — official TikTok Login Kit + Content Posting API client.
// Endpoints are centralized here (single place to patch when TikTok revises paths).
// References: https://developers.tiktok.com/products/login-kit/ and
// https://developers.tiktok.com/products/content-posting-api/ (v2 APIs).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	tokenPath       = "/v2/oauth/token/"
	refreshPath     = "/v2/oauth/token/"
	revokePath      = "/v2/oauth/revoke/"
	userInfoPath    = "/v2/user/info/"
	creatorInfoPath = "/v2/post/publish/creator_info/query/"
	videoUploadPath = "/v2/post/publish/video/init/"
	videoInboxPath  = "/v2/post/publish/inbox/video/upload/"
	photoPostPath   = "/v2/post/publish/content/init/"
	statusPath      = "/v2/post/publish/status/fetch/"
)

// Client is a minimal TikTok open API client. Client Secret never leaves this package.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// APIError is a sanitized upstream error; Body is truncated and never logged by callers.
type APIError struct {
	Status   int
	Code     string
	Message  string
	Body     string
	Endpoint string
}

func (e *APIError) Error() string {
	if e.Code != "" || e.Message != "" {
		return fmt.Sprintf("tiktok: %s HTTP %d %s %s", e.Endpoint, e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("tiktok: %s HTTP %d", e.Endpoint, e.Status)
}

// AuthorizeURL builds the Login Kit authorize URL (redirects the browser, no HTTP call).
func AuthorizeURL(authorizeBase, clientKey, redirectURI, scopes, state string) string {
	q := url.Values{}
	q.Set("client_key", clientKey)
	q.Set("response_type", "code")
	q.Set("scope", scopes)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return strings.TrimRight(authorizeBase, "?") + "?" + q.Encode()
}

// Token is the OAuth token response.
type Token struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	OpenID           string `json:"open_id"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
}

type tokenEnvelope struct {
	Data         *Token        `json:"data"`
	Error        *apiErrorBody `json:"error"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int64         `json:"expires_in"`
	OpenID       string        `json:"open_id"`
}

type apiErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	LogID     string `json:"log_id"`
	ErrorCode string `json:"error_code"`
}

func (c *Client) doForm(ctx context.Context, endpoint, path string, form url.Values, bearer string) ([]byte, int, error) {
	if c == nil || c.BaseURL == "" {
		return nil, 0, errors.New("tiktok: client not configured")
	}
	u := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.doRaw(ctx, endpoint, req)
}

func (c *Client) doJSON(ctx context.Context, endpoint, method, path string, body any, bearer string) ([]byte, int, error) {
	if c == nil || c.BaseURL == "" {
		return nil, 0, errors.New("tiktok: client not configured")
	}
	u := c.BaseURL + path
	var rdr io.Reader
	if body != nil {
		b, err := common.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.doRaw(ctx, endpoint, req)
}

func (c *Client) doRaw(ctx context.Context, endpoint string, req *http.Request) ([]byte, int, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ae := &APIError{Status: resp.StatusCode, Endpoint: endpoint}
		var eb tokenEnvelope
		if e2 := common.Unmarshal(raw, &eb); e2 == nil && eb.Error != nil {
			ae.Code = eb.Error.Code
			if ae.Code == "" {
				ae.Code = eb.Error.ErrorCode
			}
			ae.Message = eb.Error.Message
		}
		if ae.Message == "" {
			msg := string(raw)
			if len(msg) > 200 {
				msg = msg[:200]
			}
			ae.Body = msg
		}
		return raw, resp.StatusCode, ae
	}
	return raw, resp.StatusCode, nil
}

func parseToken(raw []byte) (*Token, error) {
	var env tokenEnvelope
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Data != nil && env.Data.AccessToken != "" {
		return env.Data, nil
	}
	if env.AccessToken != "" {
		return &Token{
			AccessToken:  env.AccessToken,
			RefreshToken: env.RefreshToken,
			ExpiresIn:    env.ExpiresIn,
			OpenID:       env.OpenID,
		}, nil
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "token"}
	}
	return nil, errors.New("tiktok: token: empty access_token")
}

// ExchangeToken exchanges an authorization code for tokens.
func (c *Client) ExchangeToken(ctx context.Context, clientKey, clientSecret, code, redirectURI string) (*Token, error) {
	form := url.Values{}
	form.Set("client_key", clientKey)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	raw, _, err := c.doForm(ctx, "token", tokenPath, form, "")
	if err != nil {
		return nil, err
	}
	return parseToken(raw)
}

// RefreshToken refreshes an access token.
func (c *Client) RefreshToken(ctx context.Context, clientKey, clientSecret, refreshToken string) (*Token, error) {
	form := url.Values{}
	form.Set("client_key", clientKey)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	raw, _, err := c.doForm(ctx, "refresh", refreshPath, form, "")
	if err != nil {
		return nil, err
	}
	return parseToken(raw)
}

// Revoke revokes an access token (best effort).
func (c *Client) Revoke(ctx context.Context, clientKey, clientSecret, accessToken string) error {
	form := url.Values{}
	form.Set("client_key", clientKey)
	form.Set("client_secret", clientSecret)
	form.Set("token", accessToken)
	_, _, err := c.doForm(ctx, "revoke", revokePath, form, "")
	return err
}

// UserInfo is the authorized user's profile.
type UserInfo struct {
	OpenID      string `json:"open_id"`
	UnionID     string `json:"union_id"`
	AvatarURL   string `json:"avatar_url"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

type userInfoEnvelope struct {
	Data struct {
		User UserInfo `json:"user"`
	} `json:"data"`
	Error *apiErrorBody `json:"error"`
}

// GetUserInfo fetches profile for the authorized account.
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	q := url.Values{}
	// fork: user.info.basic does not authorize username (user.info.profile).
	q.Set("fields", "open_id,union_id,avatar_url,display_name")
	u := c.BaseURL + userInfoPath + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	raw, _, err := c.doRaw(ctx, "user_info", req)
	if err != nil {
		return nil, err
	}
	var env userInfoEnvelope
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "user_info"}
	}
	if env.Data.User.OpenID == "" {
		return nil, errors.New("tiktok: user_info: empty open_id")
	}
	return &env.Data.User, nil
}

// CreatorInfo mirrors the publishing capability of the authorized account.
type CreatorInfo struct {
	CreatorNickname         string   `json:"creator_nickname"`
	CreatorUsername         string   `json:"creator_username"`
	CreatorAvatarURL        string   `json:"creator_avatar_url"`
	PrivacyLevelOptions     []string `json:"privacy_level_options"`
	CommentDisabled         bool     `json:"comment_disabled"`
	DuetDisabled            bool     `json:"duet_disabled"`
	StitchDisabled          bool     `json:"stitch_disabled"`
	MaxVideoPostDurationSec int      `json:"max_video_post_duration_sec"`
}

// PostOptions carries optional TikTok Content Sharing Guidelines fields for a post.
type PostOptions struct {
	PrivacyLevel       string
	DisableComment     *bool
	DisableDuet        *bool
	DisableStitch      *bool
	BrandContentToggle *bool
	BrandOrganicToggle *bool
}

func buildPostInfo(title, defaultPrivacy string, opts *PostOptions) map[string]any {
	privacyLevel := defaultPrivacy
	if opts != nil && strings.TrimSpace(opts.PrivacyLevel) != "" {
		privacyLevel = strings.TrimSpace(opts.PrivacyLevel)
	}
	if privacyLevel == "" {
		privacyLevel = "SELF_ONLY"
	}
	postInfo := map[string]any{
		"title":         title,
		"privacy_level": privacyLevel,
	}
	if opts != nil {
		if opts.DisableComment != nil {
			postInfo["disable_comment"] = *opts.DisableComment
		}
		if opts.DisableDuet != nil {
			postInfo["disable_duet"] = *opts.DisableDuet
		}
		if opts.DisableStitch != nil {
			postInfo["disable_stitch"] = *opts.DisableStitch
		}
		if opts.BrandContentToggle != nil {
			postInfo["brand_content_toggle"] = *opts.BrandContentToggle
		}
		if opts.BrandOrganicToggle != nil {
			postInfo["brand_organic_toggle"] = *opts.BrandOrganicToggle
		}
	}
	return postInfo
}

// QueryCreatorInfo is required before publishing per TikTok UX guidelines.
func (c *Client) QueryCreatorInfo(ctx context.Context, accessToken string) (*CreatorInfo, error) {
	raw, _, err := c.doJSON(ctx, "creator_info", http.MethodPost, creatorInfoPath, map[string]any{}, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  CreatorInfo   `json:"data"`
		Error *apiErrorBody `json:"error"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "creator_info"}
	}
	return &env.Data, nil
}

// PublishInitResponse is the accepted publish task.
type PublishInitResponse struct {
	PublishID string `json:"publish_id"`
}

// PublishVideoByURL initializes a direct video post pulled from a public URL.
func (c *Client) PublishVideoByURL(ctx context.Context, accessToken, title, videoURL, privacyLevel string) (*PublishInitResponse, error) {
	return c.PublishVideoByURLWithOptions(ctx, accessToken, title, videoURL, &PostOptions{PrivacyLevel: privacyLevel})
}

// PublishVideoByURLWithOptions initializes a direct video post with Content Sharing Guidelines options.
func (c *Client) PublishVideoByURLWithOptions(ctx context.Context, accessToken, title, videoURL string, opts *PostOptions) (*PublishInitResponse, error) {
	body := map[string]any{
		"post_info": buildPostInfo(title, "SELF_ONLY", opts),
		"source_info": map[string]any{
			"source":    "PULL_FROM_URL",
			"video_url": videoURL,
		},
	}
	return c.publishInit(ctx, accessToken, videoUploadPath, body)
}

// PublishPhotoByURL initializes a photo post pulled from public URLs.
func (c *Client) PublishPhotoByURL(ctx context.Context, accessToken, title string, photoURLs []string, privacyLevel string) (*PublishInitResponse, error) {
	return c.PublishPhotoByURLWithOptions(ctx, accessToken, title, photoURLs, &PostOptions{PrivacyLevel: privacyLevel})
}

// PublishPhotoByURLWithOptions initializes a photo post with Content Sharing Guidelines options.
func (c *Client) PublishPhotoByURLWithOptions(ctx context.Context, accessToken, title string, photoURLs []string, opts *PostOptions) (*PublishInitResponse, error) {
	postInfo := buildPostInfo(title, "SELF_ONLY", opts)
	delete(postInfo, "disable_duet")
	delete(postInfo, "disable_stitch")
	body := map[string]any{
		"post_info": postInfo,
		"source_info": map[string]any{
			"source":       "PULL_FROM_URL",
			"photo_images": photoURLs,
		},
		"post_mode":  "DIRECT_POST",
		"media_type": "PHOTO",
	}
	return c.publishInit(ctx, accessToken, photoPostPath, body)
}

func (c *Client) publishInit(ctx context.Context, accessToken, path string, body any) (*PublishInitResponse, error) {
	raw, _, err := c.doJSON(ctx, "publish", http.MethodPost, path, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  PublishInitResponse `json:"data"`
		Error *apiErrorBody       `json:"error"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "publish"}
	}
	if env.Data.PublishID == "" {
		return nil, errors.New("tiktok: publish: empty publish_id")
	}
	return &env.Data, nil
}

// PublishStatus is the publish task status.
type PublishStatus struct {
	Status     string `json:"status"`
	FailReason string `json:"fail_reason"`
	PublishID  string `json:"publish_id"`
}

// GetPublishStatus fetches publish task status.
func (c *Client) GetPublishStatus(ctx context.Context, accessToken, publishID string) (*PublishStatus, error) {
	body := map[string]any{"publish_id": publishID}
	raw, _, err := c.doJSON(ctx, "publish_status", http.MethodPost, statusPath, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  PublishStatus `json:"data"`
		Error *apiErrorBody `json:"error"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "publish_status"}
	}
	return &env.Data, nil
}

// UploadVideoFile uploads raw video bytes via FILE_UPLOAD (used when pull_by_url unavailable).
func (c *Client) UploadVideoFile(ctx context.Context, accessToken, title string, video []byte, filename string, privacyLevel string) (*PublishInitResponse, error) {
	if privacyLevel == "" {
		privacyLevel = "SELF_ONLY"
	}
	body := map[string]any{
		"post_info": map[string]any{
			"title":         title,
			"privacy_level": privacyLevel,
		},
		"source_info": map[string]any{
			"source":            "FILE_UPLOAD",
			"video_size":        len(video),
			"chunk_size":        len(video),
			"total_chunk_count": 1,
		},
	}
	// TikTok FILE_UPLOAD flow first initializes then uploads to a signed URL.
	// v1: rely on PULL_FROM_URL path primarily; this helper exists for completeness.
	raw, _, err := c.doJSON(ctx, "video_upload_init", http.MethodPost, videoInboxPath, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		Error *apiErrorBody `json:"error"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, &APIError{Code: env.Error.Code, Message: env.Error.Message, Endpoint: "video_upload_init"}
	}
	if env.Data.PublishID == "" {
		return nil, errors.New("tiktok: video upload: empty publish_id")
	}
	// Upload bytes to signed URL (multipart form per spec; single chunk v1)
	if env.Data.UploadURL != "" {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, err := w.CreateFormFile("video", filename)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(video); err != nil {
			return nil, err
		}
		_ = w.Close()
		upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, env.Data.UploadURL, &buf)
		if err != nil {
			return nil, err
		}
		upReq.Header.Set("Content-Type", w.FormDataContentType())
		upReq.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := c.HTTPClient.Do(upReq)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &APIError{Status: resp.StatusCode, Endpoint: "video_upload"}
		}
	}
	return &PublishInitResponse{PublishID: env.Data.PublishID}, nil
}
