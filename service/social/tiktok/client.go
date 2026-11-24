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
	videoInboxPath  = "/v2/post/publish/inbox/video/init/"
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
	LogID    string
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
	Data             *Token        `json:"data"`
	Error            *apiErrorBody `json:"error"`
	ErrorDescription string        `json:"error_description"`
	LogID            string        `json:"log_id"`
	AccessToken      string        `json:"access_token"`
	RefreshToken     string        `json:"refresh_token"`
	ExpiresIn        int64         `json:"expires_in"`
	RefreshExpiresIn int64         `json:"refresh_expires_in"`
	OpenID           string        `json:"open_id"`
	Scope            string        `json:"scope"`
	TokenType        string        `json:"token_type"`
}

type apiErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	LogID     string `json:"log_id"`
	ErrorCode string `json:"error_code"`
}

// UnmarshalJSON accepts both TikTok v2 nested object errors
// ({"code":"...","message":"...","log_id":"..."}) and OAuth v2 flat string
// errors ("invalid_grant").
func (e *apiErrorBody) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var code string
		if err := common.Unmarshal(trimmed, &code); err != nil {
			return err
		}
		e.Code = code
		return nil
	}
	type rawAPIErrorBody apiErrorBody
	var obj rawAPIErrorBody
	if err := common.Unmarshal(trimmed, &obj); err != nil {
		return err
	}
	*e = apiErrorBody(obj)
	return nil
}

func (e *apiErrorBody) apiError(endpoint string, status int, fallbackLogID string) *APIError {
	if e == nil {
		return nil
	}
	if status == 0 {
		status = http.StatusOK
	}
	code := e.Code
	if code == "" {
		code = e.ErrorCode
	}
	logID := e.LogID
	if logID == "" {
		logID = fallbackLogID
	}
	return &APIError{
		Status:   status,
		Code:     code,
		Message:  e.Message,
		LogID:    logID,
		Endpoint: endpoint,
	}
}

func (env *tokenEnvelope) apiError(endpoint string, status int) *APIError {
	if env == nil || env.Error == nil {
		return nil
	}
	ae := env.Error.apiError(endpoint, status, env.LogID)
	if ae.Message == "" {
		ae.Message = env.ErrorDescription
	}
	return ae
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
		if e2 := common.Unmarshal(raw, &eb); e2 == nil {
			if parsed := eb.apiError(endpoint, resp.StatusCode); parsed != nil {
				ae = parsed
			} else {
				if eb.ErrorDescription != "" {
					ae.Message = eb.ErrorDescription
				}
				if eb.LogID != "" {
					ae.LogID = eb.LogID
				}
			}
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

func parseToken(raw []byte, endpoint string, status int) (*Token, error) {
	var env tokenEnvelope
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Data != nil && env.Data.AccessToken != "" {
		return env.Data, nil
	}
	if env.AccessToken != "" {
		return &Token{
			AccessToken:      env.AccessToken,
			RefreshToken:     env.RefreshToken,
			ExpiresIn:        env.ExpiresIn,
			RefreshExpiresIn: env.RefreshExpiresIn,
			OpenID:           env.OpenID,
			Scope:            env.Scope,
			TokenType:        env.TokenType,
		}, nil
	}
	if env.Error != nil && env.Error.Code != "ok" {
		if endpoint == "" {
			endpoint = "token"
		}
		if status == 0 {
			status = http.StatusOK
		}
		return nil, env.apiError(endpoint, status)
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
	raw, status, err := c.doForm(ctx, "token", tokenPath, form, "")
	if err != nil {
		return nil, err
	}
	return parseToken(raw, "token", status)
}

// RefreshToken refreshes an access token.
func (c *Client) RefreshToken(ctx context.Context, clientKey, clientSecret, refreshToken string) (*Token, error) {
	form := url.Values{}
	form.Set("client_key", clientKey)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	raw, status, err := c.doForm(ctx, "refresh", refreshPath, form, "")
	if err != nil {
		return nil, err
	}
	return parseToken(raw, "refresh", status)
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
	LogID string        `json:"log_id"`
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
	raw, status, err := c.doRaw(ctx, "user_info", req)
	if err != nil {
		return nil, err
	}
	var env userInfoEnvelope
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, env.Error.apiError("user_info", status, env.LogID)
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
	Description        string
	PhotoCoverIndex    *int
	IsAIGC             *bool
}

// PhotoDraftOptions carries optional metadata allowed by TikTok's
// /v2/post/publish/content/init/ endpoint in post_mode="MEDIA_UPLOAD".
// Direct-post-only fields (privacy_level, disable_comment, disable_duet,
// disable_stitch, brand_*_toggle) are intentionally excluded.
type PhotoDraftOptions struct {
	Title           string
	Description     string
	PhotoCoverIndex *int
	IsAIGC          *bool
}

type photoInboxPostInfo struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	IsAIGC      *bool  `json:"is_aigc,omitempty"`
}

type photoInboxSourceInfo struct {
	Source          string   `json:"source"`
	PhotoImages     []string `json:"photo_images"`
	PhotoCoverIndex *int     `json:"photo_cover_index,omitempty"`
}

type photoInboxInitRequest struct {
	PostInfo   *photoInboxPostInfo  `json:"post_info,omitempty"`
	SourceInfo photoInboxSourceInfo `json:"source_info"`
	PostMode   string               `json:"post_mode"`
	MediaType  string               `json:"media_type"`
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
		if opts.IsAIGC != nil {
			postInfo["is_aigc"] = *opts.IsAIGC
		}
	}
	return postInfo
}

// QueryCreatorInfo is required before publishing per TikTok UX guidelines.
func (c *Client) QueryCreatorInfo(ctx context.Context, accessToken string) (*CreatorInfo, error) {
	raw, status, err := c.doJSON(ctx, "creator_info", http.MethodPost, creatorInfoPath, map[string]any{}, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  CreatorInfo   `json:"data"`
		Error *apiErrorBody `json:"error"`
		LogID string        `json:"log_id"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, env.Error.apiError("creator_info", status, env.LogID)
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
	if opts != nil {
		if desc := strings.TrimSpace(opts.Description); desc != "" {
			postInfo["description"] = desc
		}
	}
	sourceInfo := map[string]any{
		"source":       "PULL_FROM_URL",
		"photo_images": photoURLs,
	}
	if opts != nil && opts.PhotoCoverIndex != nil {
		sourceInfo["photo_cover_index"] = *opts.PhotoCoverIndex
	}
	body := map[string]any{
		"post_info":   postInfo,
		"source_info": sourceInfo,
		"post_mode":   "DIRECT_POST",
		"media_type":  "PHOTO",
	}
	return c.publishInit(ctx, accessToken, photoPostPath, body)
}

// PublishPhotoToInboxByURL sends one or more photos to the creator's TikTok inbox as a draft.
// Scope: video.upload. Uses /v2/post/publish/content/init/ with post_mode="MEDIA_UPLOAD".
// https://developers.tiktok.com/docs/en/content-posting-api-reference-photo-post
func (c *Client) PublishPhotoToInboxByURL(ctx context.Context, accessToken string, photoURLs []string, opts *PhotoDraftOptions) (*PublishInitResponse, error) {
	if len(photoURLs) == 0 {
		return nil, errors.New("tiktok: photo inbox init: empty photo_images")
	}
	req := photoInboxInitRequest{
		SourceInfo: photoInboxSourceInfo{
			Source:      "PULL_FROM_URL",
			PhotoImages: photoURLs,
		},
		PostMode:  "MEDIA_UPLOAD",
		MediaType: "PHOTO",
	}
	if opts != nil {
		title := strings.TrimSpace(opts.Title)
		desc := strings.TrimSpace(opts.Description)
		if title != "" || desc != "" || opts.IsAIGC != nil {
			req.PostInfo = &photoInboxPostInfo{
				Title:       title,
				Description: desc,
				IsAIGC:      opts.IsAIGC,
			}
		}
		if opts.PhotoCoverIndex != nil {
			req.SourceInfo.PhotoCoverIndex = opts.PhotoCoverIndex
		}
	}
	return c.publishInit(ctx, accessToken, photoPostPath, req)
}

func (c *Client) publishInit(ctx context.Context, accessToken, path string, body any) (*PublishInitResponse, error) {
	raw, status, err := c.doJSON(ctx, "publish", http.MethodPost, path, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  PublishInitResponse `json:"data"`
		Error *apiErrorBody       `json:"error"`
		LogID string              `json:"log_id"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, env.Error.apiError("publish", status, env.LogID)
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
	raw, status, err := c.doJSON(ctx, "publish_status", http.MethodPost, statusPath, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data  PublishStatus `json:"data"`
		Error *apiErrorBody `json:"error"`
		LogID string        `json:"log_id"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, env.Error.apiError("publish_status", status, env.LogID)
	}
	return &env.Data, nil
}

// PublishVideoToInboxByURL sends a video to the creator's TikTok inbox as a draft.
// Scope: video.upload. The creator finishes editing and posting inside TikTok.
// https://developers.tiktok.com/docs/en/content-posting-api-reference-upload-video
func (c *Client) PublishVideoToInboxByURL(ctx context.Context, accessToken, videoURL string) (*PublishInitResponse, error) {
	body := map[string]any{
		"source_info": map[string]any{
			"source":    "PULL_FROM_URL",
			"video_url": videoURL,
		},
	}
	raw, status, err := c.doJSON(ctx, "video_inbox_init", http.MethodPost, videoInboxPath, body, accessToken)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			PublishID string `json:"publish_id"`
		} `json:"data"`
		Error *apiErrorBody `json:"error"`
		LogID string        `json:"log_id"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil && env.Error.Code != "ok" {
		return nil, env.Error.apiError("video_inbox_init", status, env.LogID)
	}
	if env.Data.PublishID == "" {
		return nil, errors.New("tiktok: video inbox init: empty publish_id")
	}
	return &PublishInitResponse{PublishID: env.Data.PublishID}, nil
}
