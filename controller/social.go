package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/social"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const socialPublicMediaTimeout = 60 * time.Second

const socialOAuthBrowserCookiePrefix = "social_tiktok_oauth_"
const socialOAuthBrowserCookieTTL = 10 * time.Minute

func socialOAuthBrowserCookieName(state string) string {
	sum := sha256.Sum256([]byte(state))
	prefix := socialOAuthBrowserCookiePrefix
	if common.SessionCookieSecure {
		prefix = "__Host-" + prefix
	}
	return prefix + hex.EncodeToString(sum[:])
}

func socialOAuthBrowserCookieValue(state string) string {
	return common.GenerateHMACWithKey([]byte("social-oauth-browser-v1:"+common.SessionSecret), state)
}

func setSocialOAuthBrowserCookie(c *gin.Context, state string) {
	if state == "" {
		return
	}
	path := "/api/social/v1/oauth/tiktok"
	if common.SessionCookieSecure {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthBrowserCookieName(state),
		Value:    socialOAuthBrowserCookieValue(state),
		Path:     path,
		MaxAge:   int(socialOAuthBrowserCookieTTL / time.Second),
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func socialOAuthBrowserCookieValid(c *gin.Context, state string) bool {
	if c == nil || state == "" {
		return false
	}
	value, err := c.Cookie(socialOAuthBrowserCookieName(state))
	return err == nil && value == socialOAuthBrowserCookieValue(state)
}

func clearSocialOAuthBrowserCookie(c *gin.Context, state string) {
	if c == nil || state == "" {
		return
	}
	path := "/api/social/v1/oauth/tiktok"
	if common.SessionCookieSecure {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthBrowserCookieName(state),
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

var socialPublicMediaPassthroughHeaders = []string{
	"Content-Type",
	"Content-Length",
	"Content-Range",
	"Accept-Ranges",
	"ETag",
	"Last-Modified",
	"Cache-Control",
}

// socialSvc is package-level for simple wiring; tests can replace via setSocialService.
var socialSvc = social.DefaultService()

func setSocialService(s *social.Service) {
	socialSvc = s
}

func socialError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, social.ErrProviderInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": "invalid-provider"})
	case errors.Is(err, social.ErrAccountProviderMismatch):
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error(), "code": "account-provider-mismatch"})
	case errors.Is(err, social.ErrPostProviderMismatch):
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error(), "code": "post-provider-mismatch"})
	case errors.Is(err, social.ErrTikTokDirectNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_OPS_NOT_CONFIGURED"})
	case errors.Is(err, social.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_OPS_DISABLED"})
	case errors.Is(err, social.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_OPS_NOT_CONFIGURED"})
	case errors.Is(err, social.ErrPlatformDenied):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_PLATFORM_DENIED"})
	case errors.Is(err, social.ErrAccountDenied):
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_ACCOUNT_DENIED"})
	case errors.Is(err, social.ErrAccountMissing):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_ACCOUNT_NOT_FOUND"})
	case errors.Is(err, social.ErrRedirectDenied):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_REDIRECT_DENIED"})
	case errors.Is(err, social.ErrMediaInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": "SOCIAL_MEDIA_INVALID"})
	case errors.Is(err, social.ErrSeatQuota):
		c.JSON(http.StatusBadRequest, gin.H{
			"success":      false,
			"message":      "社媒绑定席位不足或余额不够",
			"code":         "SOCIAL_SEAT_QUOTA",
			"need_credits": social.NeedCreditsForPrice(),
		})
	case errors.Is(err, social.ErrSeatCap):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "社媒绑定席位已达上限，请先解绑一个账号", "code": "SOCIAL_SEAT_CAP"})
	case errors.Is(err, social.ErrSeatExpired):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "社媒绑定席位已过期，请续费或解绑", "code": "SOCIAL_SEAT_EXPIRED"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found", "code": "SOCIAL_NOT_FOUND"})
	default:
		// Do not leak upstream secrets; client errors already sanitized in service layer.
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "bearer") || strings.Contains(strings.ToLower(msg), "api key") {
			msg = "upstream request failed"
		}
		common.ApiErrorMsg(c, msg)
	}
}

// SocialStatus GET /api/social/v1/status
func SocialStatus(c *gin.Context) {
	common.ApiSuccess(c, socialSvc.Status())
}

// SocialSeats GET /api/social/v1/seats
// fork: social-seats
func SocialSeats(c *gin.Context) {
	view, err := socialSvc.SeatsView(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// SocialListAccounts GET /api/social/v1/accounts
func SocialListAccounts(c *gin.Context) {
	userId := c.GetInt("id")
	rows, err := socialSvc.ListAccounts(c.Request.Context(), userId, c.Query("provider"))
	if err != nil {
		socialError(c, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, a := range rows {
		out = append(out, gin.H{
			"id":           a.ID,
			"platform":     a.Platform,
			"username":     a.Username,
			"display_name": a.DisplayName,
			"status":       a.Status,
			// fork: social-provider-isolation — public source identity.
			"provider": a.Provider,
		})
	}
	common.ApiSuccess(c, gin.H{"accounts": out})
}

type socialConnectRequest struct {
	Provider    string `json:"provider"`
	Platform    string `json:"platform"`
	RedirectURL string `json:"redirect_url"`
}

// SocialConnect POST /api/social/v1/connect
func SocialConnect(c *gin.Context) {
	var req socialConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	authURL, err := socialSvc.Connect(c.Request.Context(), c.GetInt("id"), req.Provider, req.Platform, req.RedirectURL)
	if err != nil {
		socialError(c, err)
		return
	}
	if strings.EqualFold(req.Provider, social.ProviderTikTokDirect) {
		if parsed, parseErr := url.Parse(authURL); parseErr == nil {
			setSocialOAuthBrowserCookie(c, parsed.Query().Get("state"))
		}
	}
	common.ApiSuccess(c, gin.H{"auth_url": authURL, "platform": social.NormalizePlatform(req.Platform), "provider": req.Provider})
}

// SocialTikTokCallback handles the official TikTok OAuth redirect (anonymous, state-bound).
// fork: tiktok-direct
// GET (and POST for defensive parity) /api/social/v1/oauth/tiktok/callback?code=...&state=...
func SocialTikTokCallback(c *gin.Context) {
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	c.Header("X-Content-Type-Options", "nosniff")
	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		code = c.PostForm("code")
	}
	if state == "" {
		state = c.PostForm("state")
	}
	if code == "" || state == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage("TikTok 授权失败", "missing code or state", "")))
		return
	}
	if !socialOAuthBrowserCookieValid(c, state) {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage("TikTok 授权失败", "authorization flow does not belong to this browser", "")))
		return
	}
	defer clearSocialOAuthBrowserCookie(c, state)
	res, err := socialSvc.HandleTikTokCallback(c.Request.Context(), code, state)
	if err != nil {
		sanitized := err.Error()
		if strings.Contains(strings.ToLower(sanitized), "bearer") || strings.Contains(strings.ToLower(sanitized), "client_secret") {
			sanitized = "upstream request failed"
		}
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage("TikTok 授权失败", sanitized, "")))
		return
	}
	if res.RedirectAfter != "" {
		c.Redirect(http.StatusFound, res.RedirectAfter)
		return
	}
	handle := res.Account.DisplayName
	if handle == "" {
		handle = res.Account.Username
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage("TikTok 已连接", "账号 @"+handle+" 已绑定到 OmniMux，可以返回客户端了。", handle)))
}

// HandleTikTokCallback is exported on the service for controller wiring.
// (Wrapper keeps service method testable without gin.)

func renderTikTokCallbackPage(title, msg, handle string) string {
	deepLink := ""
	if handle != "" {
		deepLink = `<p><a href="omnimux://social/connected" style="display:inline-block;margin-top:12px;padding:10px 18px;background:#000;color:#fff;border-radius:999px;text-decoration:none">返回 OmniMux</a></p>`
	}
	safeTitle := html.EscapeString(title)
	safeMsg := html.EscapeString(msg)
	return `<!doctype html><html lang="zh"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + safeTitle + `</title>` +
		`<style>body{font-family:-apple-system,system-ui,sans-serif;background:#0A0A0B;color:#EDEDED;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}main{text-align:center;max-width:420px;padding:32px}h1{font-size:22px;margin:0 0 8px}p{color:#A19F9D;font-size:14px;line-height:1.6}</style></head><body><main>` +
		`<img src="https://omnimux.ai/logo.png" alt="" width="56" height="56" style="border-radius:14px">` +
		`<h1>` + safeTitle + `</h1><p>` + safeMsg + `</p>` + deepLink +
		`<p style="margin-top:24px;font-size:12px;color:#5b5b5b">omnimux.ai · Twinspace Labs LLC</p></main></body></html>`
}

// SocialDisconnectAccount DELETE /api/social/v1/accounts/:id
func SocialDisconnectAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid account id")
		return
	}
	if err := socialSvc.DisconnectAccount(c.Request.Context(), c.GetInt("id"), c.Query("provider"), id); err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"disconnected": true, "id": id})
}

// SocialAccountCreatorInfo GET /api/social/v1/accounts/:id/creator-info?provider=tiktok_direct
// fork: tiktok-direct — queries live TikTok creator publishing capability and UX constraints.
func SocialAccountCreatorInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid account id")
		return
	}
	info, err := socialSvc.GetTikTokCreatorInfo(c.Request.Context(), c.GetInt("id"), c.Query("provider"), id)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"creator_nickname":            info.CreatorNickname,
		"creator_username":            info.CreatorUsername,
		"creator_avatar_url":          info.CreatorAvatarURL,
		"privacy_level_options":       info.PrivacyLevelOptions,
		"comment_disabled":            info.CommentDisabled,
		"duet_disabled":               info.DuetDisabled,
		"stitch_disabled":             info.StitchDisabled,
		"max_video_post_duration_sec": info.MaxVideoPostDurationSec,
	})
}

// SocialPublicMediaProxy GET /api/social/v1/public/media/:token
// fork: tiktok-direct — serves short-lived signed media under verified omnimux.ai for TikTok PULL_FROM_URL.
func SocialPublicMediaProxy(c *gin.Context) {
	upstreamURL, err := socialSvc.OpenPublicMediaPullToken(c.Param("token"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid or expired media token", "code": "SOCIAL_MEDIA_INVALID"})
		return
	}
	if err := service.ValidateSSRFProtectedFetchURL(upstreamURL); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid or expired media token", "code": "SOCIAL_MEDIA_INVALID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), socialPublicMediaTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid media request", "code": "SOCIAL_MEDIA_INVALID"})
		return
	}
	if rng := c.GetHeader("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}
	if ir := c.GetHeader("If-Range"); ir != "" {
		req.Header.Set("If-Range", ir)
	}
	resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "message": "media upstream unreachable", "code": "SOCIAL_MEDIA_UPSTREAM_ERROR"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "message": "media upstream rejected request", "code": "SOCIAL_MEDIA_UPSTREAM_ERROR"})
		return
	}
	for _, h := range socialPublicMediaPassthroughHeaders {
		if v := resp.Header.Get(h); v != "" {
			c.Writer.Header().Set(h, v)
		}
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") && !strings.HasPrefix(contentType, "audio/") {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="media"`)
	}
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

type socialCreatePostRequest struct {
	Provider      string                    `json:"provider"`
	Content       string                    `json:"content"`
	AccountIDs    []int64                   `json:"account_ids"`
	PublishNow    *bool                     `json:"publish_now"`
	ScheduledFor  string                    `json:"scheduled_for"`
	Timezone      string                    `json:"timezone"`
	MediaItems    []map[string]any          `json:"media_items"`
	Queued        bool                      `json:"queued"`
	TikTokOptions *social.TikTokPostOptions `json:"tiktok_options"`
}

// SocialCreatePost POST /api/social/v1/posts
func SocialCreatePost(c *gin.Context) {
	var req socialCreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	idem := c.GetHeader("Idempotency-Key")
	if idem == "" {
		idem = c.GetHeader("X-Idempotency-Key")
	}
	publishNow := true
	if req.PublishNow != nil {
		publishNow = *req.PublishNow
	}
	if req.ScheduledFor != "" {
		publishNow = false
	}
	post, err := socialSvc.CreatePost(c.Request.Context(), c.GetInt("id"), social.CreatePostInput{
		Provider:       req.Provider,
		Content:        req.Content,
		AccountIDs:     req.AccountIDs,
		PublishNow:     publishNow,
		ScheduledFor:   req.ScheduledFor,
		Timezone:       req.Timezone,
		MediaItems:     req.MediaItems,
		IdempotencyKey: idem,
		Queued:         req.Queued,
		TikTokOptions:  req.TikTokOptions,
	})
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":               post.ID,
		"provider":         post.Provider,
		"status":           post.Status,
		"provider_post_id": post.ProviderPostID,
		"content_preview":  post.ContentPreview,
		"idempotency_key":  post.IdempotencyKey,
		"created_at":       post.CreatedAt,
	})
}

// SocialGetPost GET /api/social/v1/posts/:id
func SocialGetPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid post id")
		return
	}
	refresh := c.DefaultQuery("refresh", "1") != "0"
	post, err := socialSvc.GetPost(c.Request.Context(), c.GetInt("id"), c.Query("provider"), id, refresh)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":               post.ID,
		"provider":         post.Provider,
		"status":           post.Status,
		"provider_post_id": post.ProviderPostID,
		"content_preview":  post.ContentPreview,
		"result":           jsonRawOrNil(post.ResultSnapshot),
		"created_at":       post.CreatedAt,
		"updated_at":       post.UpdatedAt,
	})
}

// SocialPresignMedia POST /api/social/v1/media/presign
func SocialPresignMedia(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		body = map[string]any{}
	}
	res, err := socialSvc.PresignMedia(c.Request.Context(), c.GetInt("id"), body)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, res)
}

func parseLocalAccountIDs(raw string) []int64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// SocialDailyMetrics GET /api/social/v1/analytics/daily-metrics
func SocialDailyMetrics(c *gin.Context) {
	userId := c.GetInt("id")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")
	platform := c.Query("platform")
	timezone := c.Query("timezone")
	accountIDs := parseLocalAccountIDs(c.Query("account_ids"))

	data, err := socialSvc.GetDailyMetrics(c.Request.Context(), userId, fromDate, toDate, platform, timezone, accountIDs)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialBestTimeToPost GET /api/social/v1/analytics/best-time-to-post
func SocialBestTimeToPost(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")
	timezone := c.Query("timezone")
	var accountID int64
	if s := c.Query("account_id"); s != "" {
		accountID, _ = strconv.ParseInt(s, 10, 64)
	}

	data, err := socialSvc.GetBestTimeToPost(c.Request.Context(), userId, platform, accountID, timezone)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialPostingFrequency GET /api/social/v1/analytics/posting-frequency
func SocialPostingFrequency(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")

	data, err := socialSvc.GetPostingFrequency(c.Request.Context(), userId, platform)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialContentDecay GET /api/social/v1/analytics/content-decay
func SocialContentDecay(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")

	data, err := socialSvc.GetContentDecay(c.Request.Context(), userId, platform)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialFollowerStats GET /api/social/v1/accounts/follower-stats
func SocialFollowerStats(c *gin.Context) {
	userId := c.GetInt("id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	accountIDs := parseLocalAccountIDs(c.Query("account_ids"))

	data, err := socialSvc.GetFollowerStats(c.Request.Context(), userId, days, accountIDs)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialPostsAnalytics GET /api/social/v1/analytics/posts
func SocialPostsAnalytics(c *gin.Context) {
	userId := c.GetInt("id")
	postID := c.Query("post_id")
	platform := c.Query("platform")
	sortBy := c.Query("sortBy")
	sortOrder := c.Query("sortOrder")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	data, err := socialSvc.GetPostsAnalytics(c.Request.Context(), userId, postID, platform, sortBy, sortOrder, page, limit)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

type syncExternalPostsBody struct {
	AccountID int64  `json:"account_id"`
	URL       string `json:"url"`
}

// SocialSyncExternalPosts POST /api/social/v1/analytics/sync-external-posts
func SocialSyncExternalPosts(c *gin.Context) {
	var body syncExternalPostsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if body.AccountID <= 0 {
		common.ApiErrorMsg(c, "account_id is required")
		return
	}
	data, err := socialSvc.SyncExternalPosts(c.Request.Context(), c.GetInt("id"), body.AccountID, body.URL)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialInboxVolume GET /api/social/v1/inbox-analytics/volume
func SocialInboxVolume(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")

	data, err := socialSvc.GetInboxVolume(c.Request.Context(), userId, platform, fromDate, toDate)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialInboxResponseTime GET /api/social/v1/inbox-analytics/response-time
func SocialInboxResponseTime(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")

	data, err := socialSvc.GetInboxResponseTime(c.Request.Context(), userId, platform, fromDate, toDate)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialInboxHeatmap GET /api/social/v1/inbox-analytics/heatmap
func SocialInboxHeatmap(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")
	timezone := c.Query("timezone")

	data, err := socialSvc.GetInboxHeatmap(c.Request.Context(), userId, platform, timezone)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

// SocialInboxSourceBreakdown GET /api/social/v1/inbox-analytics/source-breakdown
func SocialInboxSourceBreakdown(c *gin.Context) {
	userId := c.GetInt("id")
	platform := c.Query("platform")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")

	data, err := socialSvc.GetInboxSourceBreakdown(c.Request.Context(), userId, platform, fromDate, toDate)
	if err != nil {
		socialError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func jsonRawOrNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var v any
	if err := common.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	return v
}
