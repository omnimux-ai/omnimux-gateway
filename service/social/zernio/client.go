package zernio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// FlexibleID accepts either a bare string id or an object with "_id" (Zernio accounts.profileId).
type FlexibleID string

func (f *FlexibleID) UnmarshalJSON(data []byte) error {
	if f == nil {
		return fmt.Errorf("FlexibleID: nil receiver")
	}
	if len(data) == 0 || string(data) == "null" {
		*f = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexibleID(s)
		return nil
	}
	var obj struct {
		ID string `json:"_id"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*f = FlexibleID(obj.ID)
	return nil
}

func (f FlexibleID) String() string { return string(f) }

// Client is a minimal Zernio REST client. The API key never leaves this package
// in responses; callers must not log Request objects that embed Authorization.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

type APIError struct {
	Status  int
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("zernio: HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("zernio: HTTP %d", e.Status)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, headers map[string]string) ([]byte, int, error) {
	if c == nil || c.APIKey == "" {
		return nil, 0, fmt.Errorf("zernio: client not configured")
	}
	u := c.BaseURL + path
	if len(query) > 0 {
		u = u + "?" + query.Encode()
	}
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
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
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
		msg := extractErrorMessage(raw)
		return raw, resp.StatusCode, &APIError{Status: resp.StatusCode, Message: msg, Body: string(raw)}
	}
	return raw, resp.StatusCode, nil
}

func extractErrorMessage(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := common.Unmarshal(raw, &envelope); err == nil {
		if envelope.Error != "" {
			return envelope.Error
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}
	s := string(raw)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// --- Profiles ---

type Profile struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type createProfileResponse struct {
	Profile Profile `json:"profile"`
}

func (c *Client) CreateProfile(ctx context.Context, name, description string) (*Profile, error) {
	raw, _, err := c.do(ctx, http.MethodPost, "/profiles", nil, map[string]string{
		"name":        name,
		"description": description,
	}, nil)
	if err != nil {
		return nil, err
	}
	var out createProfileResponse
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Profile.ID == "" {
		// Some responses may return the profile at top level
		var p Profile
		if err := common.Unmarshal(raw, &p); err == nil && p.ID != "" {
			return &p, nil
		}
		return nil, fmt.Errorf("zernio: create profile: empty id")
	}
	return &out.Profile, nil
}

// --- Connect ---

type ConnectURLResult struct {
	AuthURL string `json:"authUrl"`
	URL     string `json:"url"`
	State   string `json:"state"`
}

func (c *Client) GetConnectURL(ctx context.Context, platform, profileID, redirectURL string) (*ConnectURLResult, error) {
	q := url.Values{}
	q.Set("profileId", profileID)
	if redirectURL != "" {
		q.Set("redirect_url", redirectURL)
	}
	path := "/connect/" + url.PathEscape(platform)
	raw, _, err := c.do(ctx, http.MethodGet, path, q, nil, nil)
	if err != nil {
		return nil, err
	}
	var out ConnectURLResult
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.AuthURL == "" {
		out.AuthURL = out.URL
	}
	if out.AuthURL == "" {
		return nil, fmt.Errorf("zernio: connect: empty authUrl")
	}
	return &out, nil
}

// --- Accounts ---

type Account struct {
	ID        string     `json:"_id"`
	ProfileID FlexibleID `json:"profileId"`
	Platform  string     `json:"platform"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	// DisplayName is preferred when present (Zernio accounts).
	DisplayName string `json:"displayName"`
}

type listAccountsResponse struct {
	Accounts []Account `json:"accounts"`
}

func (c *Client) ListAccounts(ctx context.Context, profileID string) ([]Account, error) {
	q := url.Values{}
	if profileID != "" {
		q.Set("profileId", profileID)
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/accounts", q, nil, nil)
	if err != nil {
		return nil, err
	}
	var out listAccountsResponse
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Accounts, nil
}

func (c *Client) DisconnectAccount(ctx context.Context, accountID string) error {
	_, _, err := c.do(ctx, http.MethodDelete, "/accounts/"+url.PathEscape(accountID), nil, nil, nil)
	return err
}

// --- Media ---

type PresignResult struct {
	UploadURL string `json:"uploadUrl"`
	PublicURL string `json:"publicUrl"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expiresIn"`
	// Alternate shapes
	URL string `json:"url"`
}

func (c *Client) PresignMedia(ctx context.Context, body map[string]any) (*PresignResult, error) {
	raw, _, err := c.do(ctx, http.MethodPost, "/media/presign", nil, body, nil)
	if err != nil {
		return nil, err
	}
	var out PresignResult
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.UploadURL == "" && out.URL != "" {
		out.UploadURL = out.URL
	}
	return &out, nil
}

// --- Posts ---

type CreatePostRequest struct {
	Content          string              `json:"content"`
	Platforms        []PostPlatformTarget `json:"platforms"`
	PublishNow       bool                `json:"publishNow,omitempty"`
	ScheduledFor     string              `json:"scheduledFor,omitempty"`
	Timezone         string              `json:"timezone,omitempty"`
	MediaItems       []map[string]any    `json:"mediaItems,omitempty"`
	QueuedFromProfile string             `json:"queuedFromProfile,omitempty"`
}

type PostPlatformTarget struct {
	Platform  string `json:"platform"`
	AccountID string `json:"accountId"`
}

type Post struct {
	ID           string `json:"_id"`
	Status       string `json:"status"`
	ScheduledFor string `json:"scheduledFor"`
	// Keep remainder as raw for snapshot
}

type createPostResponse struct {
	Post         Post            `json:"post"`
	ExistingPost *Post           `json:"existingPost"`
}

func (c *Client) CreatePost(ctx context.Context, req CreatePostRequest, idempotencyKey string) (*Post, []byte, error) {
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["x-request-id"] = idempotencyKey
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/posts", nil, req, headers)
	if err != nil {
		// 409 content-hash may still include existingPostId — surface as error for now
		return nil, raw, err
	}
	var out createPostResponse
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, raw, err
	}
	if out.ExistingPost != nil && out.ExistingPost.ID != "" {
		return out.ExistingPost, raw, nil
	}
	if out.Post.ID == "" {
		var p Post
		if err := common.Unmarshal(raw, &p); err == nil && p.ID != "" {
			return &p, raw, nil
		}
		return nil, raw, fmt.Errorf("zernio: create post: empty id")
	}
	return &out.Post, raw, nil
}

func (c *Client) GetPost(ctx context.Context, postID string) (*Post, []byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/posts/"+url.PathEscape(postID), nil, nil, nil)
	if err != nil {
		return nil, raw, err
	}
	var out createPostResponse
	if err := common.Unmarshal(raw, &out); err == nil && out.Post.ID != "" {
		return &out.Post, raw, nil
	}
	var p Post
	if err := common.Unmarshal(raw, &p); err != nil {
		return nil, raw, err
	}
	return &p, raw, nil
}

// --- Analytics & Inbox ---

// AnalyticsQuery holds parameters for social analytics endpoints.
type AnalyticsQuery struct {
	ProfileID  string
	AccountIDs []string
	AccountID  string
	Platform   string
	FromDate   string
	ToDate     string
	Timezone   string
	Days       int
	SortBy     string
	SortOrder  string
	Page       int
	Limit      int
	PostID     string
}

func (q AnalyticsQuery) toURLValues() url.Values {
	v := url.Values{}
	if q.ProfileID != "" {
		v.Set("profileId", q.ProfileID)
	}
	if len(q.AccountIDs) > 0 {
		v.Set("accountIds", strings.Join(q.AccountIDs, ","))
	}
	if q.AccountID != "" {
		v.Set("accountId", q.AccountID)
	}
	if q.Platform != "" {
		v.Set("platform", q.Platform)
	}
	if q.FromDate != "" {
		v.Set("fromDate", q.FromDate)
	}
	if q.ToDate != "" {
		v.Set("toDate", q.ToDate)
	}
	if q.Timezone != "" {
		v.Set("timezone", q.Timezone)
	}
	if q.Days > 0 {
		v.Set("days", strconv.Itoa(q.Days))
	}
	if q.SortBy != "" {
		v.Set("sortBy", q.SortBy)
	}
	if q.SortOrder != "" {
		v.Set("sortOrder", q.SortOrder)
	}
	if q.Page > 0 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.PostID != "" {
		v.Set("postId", q.PostID)
	}
	return v
}

func (c *Client) GetDailyMetrics(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/analytics/daily-metrics", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetBestTimeToPost(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/analytics/best-time-to-post", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetPostingFrequency(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/analytics/posting-frequency", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetContentDecay(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/analytics/content-decay", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetFollowerStats(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/accounts/follower-stats", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetPostsAnalytics(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/analytics", q.toURLValues(), nil, nil)
	return raw, err
}

type SyncExternalPostsRequest struct {
	AccountID string `json:"accountId"`
	URL       string `json:"url,omitempty"`
}

func (c *Client) SyncExternalPosts(ctx context.Context, accountID, postURL string) ([]byte, error) {
	req := SyncExternalPostsRequest{
		AccountID: accountID,
		URL:       postURL,
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/analytics/sync-external-posts", nil, req, nil)
	return raw, err
}

func (c *Client) GetInboxVolume(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/inbox-analytics/volume", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetInboxResponseTime(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/inbox-analytics/response-time", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetInboxHeatmap(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/inbox-analytics/heatmap", q.toURLValues(), nil, nil)
	return raw, err
}

func (c *Client) GetInboxSourceBreakdown(ctx context.Context, q AnalyticsQuery) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/inbox-analytics/source-breakdown", q.toURLValues(), nil, nil)
	return raw, err
}
