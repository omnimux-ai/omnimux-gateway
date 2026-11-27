package controller

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/social"
	"github.com/QuantumNous/new-api/service/social/tiktok"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const socialPublicMediaTimeout = 60 * time.Second
const maxTikTokWebhookBodyBytes = 64 << 10 // 64 KiB

const socialOAuthBrowserCookiePrefix = "social_tiktok_oauth_"
const socialOAuthConfirmCookiePrefix = "social_tiktok_confirm_"
const socialOAuthBrowserCookieTTL = 10 * time.Minute
const socialOAuthConfirmCookieTTL = 5 * time.Minute

func socialOAuthConfirmCookieName(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	prefix := socialOAuthConfirmCookiePrefix
	if common.SessionCookieSecure {
		prefix = "__Host-" + prefix
	}
	return prefix + hex.EncodeToString(sum[:])
}

func socialOAuthConfirmCookieValue(ticket, nonce string) string {
	return common.GenerateHMACWithKey([]byte("social-oauth-confirm-v1:"+common.SessionSecret), ticket+":"+nonce)
}

func setSocialOAuthConfirmCookie(c *gin.Context, ticket string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(b)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthConfirmCookieName(ticket),
		Value:    socialOAuthConfirmCookieValue(ticket, nonce),
		Path:     "/",
		MaxAge:   int(socialOAuthConfirmCookieTTL / time.Second),
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	return nonce, nil
}

func socialOAuthConfirmCookieValid(c *gin.Context, ticket, nonce string) bool {
	if c == nil || ticket == "" || nonce == "" {
		return false
	}
	value, err := c.Cookie(socialOAuthConfirmCookieName(ticket))
	if err != nil {
		return false
	}
	expected := socialOAuthConfirmCookieValue(ticket, nonce)
	return subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func clearSocialOAuthConfirmCookie(c *gin.Context, ticket string) {
	if c == nil || ticket == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthConfirmCookieName(ticket),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

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
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthBrowserCookieName(state),
		Value:    socialOAuthBrowserCookieValue(state),
		Path:     "/",
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
	if err != nil {
		return false
	}
	expected := socialOAuthBrowserCookieValue(state)
	return subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func clearSocialOAuthBrowserCookie(c *gin.Context, state string) {
	if c == nil || state == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     socialOAuthBrowserCookieName(state),
		Value:    "",
		Path:     "/",
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

var (
	validTikTokUpstreamCodeRe  = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)
	validTikTokUpstreamLogIDRe = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
)

// tikTokBlockedMessageSubstrings prevents secret leakage and avoids legacy
// desktop quota-classifier false positives on TikTok upstream error messages.
var tikTokBlockedMessageSubstrings = []string{
	"bearer",
	"api key",
	"client_secret",
	"quota exceeded",
	"usage limit reached",
	"insufficient quota",
}

type tikTokErrorClassification struct {
	httpStatus int
	code       string
	fallback   string
}

func sanitizeTikTokSocialMessage(raw, fallback string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return fallback
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(msg)), " ")
	for _, blocked := range tikTokBlockedMessageSubstrings {
		if strings.Contains(normalized, blocked) {
			return fallback
		}
	}
	return msg
}

func classifyTikTokSocialError(err error, apiErr *tiktok.APIError) tikTokErrorClassification {
	var normCode string
	var status int
	var endpoint string
	if apiErr != nil {
		normCode = strings.ToLower(strings.TrimSpace(apiErr.Code))
		status = apiErr.Status
		endpoint = strings.ToLower(strings.TrimSpace(apiErr.Endpoint))
	}

	// Transient rate limit takes precedence over refresh/endpoint checks so a
	// transient 429 during token refresh is never misclassified as an invalid token.
	if normCode == "rate_limit_exceeded" || status == http.StatusTooManyRequests {
		return tikTokErrorClassification{
			httpStatus: http.StatusTooManyRequests,
			code:       "SOCIAL_TIKTOK_RATE_LIMITED",
			fallback:   "TikTok rate limit reached; please retry later",
		}
	}
	if normCode == "spam_risk_user_banned_from_posting" {
		return tikTokErrorClassification{
			httpStatus: http.StatusConflict,
			code:       "SOCIAL_TIKTOK_POSTING_BANNED",
			fallback:   "TikTok account is restricted from posting",
		}
	}
	switch normCode {
	case "spam_risk_too_many_posts",
		"reached_active_user_cap",
		"spam_risk_too_many_pending_share",
		"unaudited_client_can_only_post_to_private_accounts",
		"url_ownership_unverified":
		return tikTokErrorClassification{
			httpStatus: http.StatusConflict,
			code:       "SOCIAL_TIKTOK_PUBLISH_RESTRICTED",
			fallback:   "TikTok posting limit reached",
		}
	}
	if normCode == "scope_not_authorized" {
		return tikTokErrorClassification{
			httpStatus: http.StatusConflict,
			code:       "SOCIAL_TIKTOK_SCOPE_MISSING",
			fallback:   "TikTok publishing scope is not authorized; please reconnect account",
		}
	}
	if errors.Is(err, social.ErrTikTokTokenExpired) || endpoint == "refresh" {
		return tikTokErrorClassification{
			httpStatus: http.StatusConflict,
			code:       "SOCIAL_TIKTOK_TOKEN_INVALID",
			fallback:   "TikTok authorization expired or invalid; please reconnect account",
		}
	}
	switch normCode {
	case "access_token_invalid",
		"token_expired",
		"invalid_token",
		"token_revoked",
		"invalid_grant",
		"refresh_token_invalid":
		return tikTokErrorClassification{
			httpStatus: http.StatusConflict,
			code:       "SOCIAL_TIKTOK_TOKEN_INVALID",
			fallback:   "TikTok authorization expired or invalid; please reconnect account",
		}
	}
	return tikTokErrorClassification{
		httpStatus: http.StatusBadGateway,
		code:       "SOCIAL_TIKTOK_UPSTREAM_ERROR",
		fallback:   "TikTok upstream request failed",
	}
}

func writeTikTokSocialError(c *gin.Context, err error, apiErr *tiktok.APIError) {
	cls := classifyTikTokSocialError(err, apiErr)
	rawMsg := ""
	if apiErr != nil {
		rawMsg = apiErr.Message
	} else if err != nil {
		rawMsg = err.Error()
	}
	payload := gin.H{
		"success": false,
		"message": sanitizeTikTokSocialMessage(rawMsg, cls.fallback),
		"code":    cls.code,
	}
	if apiErr != nil {
		if upstreamCode := strings.TrimSpace(apiErr.Code); validTikTokUpstreamCodeRe.MatchString(upstreamCode) {
			payload["upstream_code"] = upstreamCode
		}
		if upstreamLogID := strings.TrimSpace(apiErr.LogID); validTikTokUpstreamLogIDRe.MatchString(upstreamLogID) {
			payload["upstream_log_id"] = upstreamLogID
		}
	}
	c.JSON(cls.httpStatus, payload)
}

func socialError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var tikTokErr *tiktok.APIError
	switch {
	case errors.Is(err, social.ErrProviderInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error(), "code": "invalid-provider"})
	// fork: zernio-retire — the removed source is gone, not merely absent.
	case errors.Is(err, social.ErrProviderRetired):
		c.JSON(http.StatusGone, gin.H{"success": false, "message": err.Error(), "code": "provider-retired"})
	// fork: tiktok-draft-upload — the account token does not grant a publish scope.
	case errors.Is(err, social.ErrTikTokPublishScopeMissing):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": sanitizeTikTokSocialMessage(err.Error(), "TikTok publishing scope is not authorized; please reconnect account"),
			"code":    "SOCIAL_TIKTOK_SCOPE_MISSING",
		})
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
	// fork: tiktok-direct — structured TikTok upstream error mapping (never 401/403).
	case errors.As(err, &tikTokErr) || errors.Is(err, social.ErrTikTokTokenExpired):
		writeTikTokSocialError(c, err, tikTokErr)
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
		uploadDraft, publishDirect := socialSvc.TikTokCapabilities(a)
		out = append(out, gin.H{
			"id":           a.ID,
			"platform":     a.Platform,
			"username":     a.Username,
			"display_name": a.DisplayName,
			"status":       a.Status,
			// fork: social-provider-isolation — public source identity.
			"provider": a.Provider,
			// fork: tiktok-draft-upload — granted-scope capabilities.
			"capabilities": gin.H{
				"upload_draft":   uploadDraft,
				"publish_direct": publishDirect,
			},
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
	c.Header("Cache-Control", "no-store")
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
	common.ApiSuccess(c, gin.H{"auth_url": authURL, "platform": social.NormalizePlatform(req.Platform), "provider": req.Provider})
}

// SocialTikTokOAuthStart redeems a one-time launch ticket in the user's browser,
// binds the fresh OAuth state to that browser via an HttpOnly cookie, and redirects
// to the server-validated TikTok authorize URL.
// fork: tiktok-direct
// GET / POST /api/social/v1/oauth/tiktok/start
func SocialTikTokOAuthStart(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-Content-Type-Options", "nosniff")
	lang := i18n.StatedLang(c)
	isZh := strings.HasPrefix(lang, "zh")
	renderStartError := func(status int, zhMsg, enMsg string) {
		title, msg := "TikTok Authorization Failed", enMsg
		if isZh {
			title, msg = "TikTok 授权失败", zhMsg
		}
		c.Header("Vary", "Accept-Language")
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; sandbox")
		c.Data(status, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage(lang, title, msg, "")))
	}
	renderTicketErr := func(err error) {
		switch {
		case errors.Is(err, social.ErrTikTokOAuthTicketExpired):
			renderStartError(
				http.StatusBadRequest,
				"授权链接已过期，请返回客户端重新发起连接。",
				"Authorization link has expired. Please return to OmniMux and try again.",
			)
		case errors.Is(err, social.ErrTikTokOAuthTicketInvalid):
			renderStartError(
				http.StatusBadRequest,
				"授权链接无效或已使用，请返回客户端重新发起连接。",
				"Authorization link is invalid or has already been used. Please return to OmniMux and try again.",
			)
		case errors.Is(err, social.ErrDisabled), errors.Is(err, social.ErrTikTokDirectNotConfigured):
			renderStartError(
				http.StatusServiceUnavailable,
				"TikTok 授权服务暂不可用，请稍后重试。",
				"TikTok authorization service is temporarily unavailable. Please try again later.",
			)
		default:
			renderStartError(
				http.StatusBadRequest,
				"无法启动 TikTok 授权，请返回客户端重新发起连接。",
				"Unable to start TikTok authorization. Please return to OmniMux and try again.",
			)
		}
	}

	sessionUserID := c.GetInt("id")
	var ticket string
	if c.Request.Method == http.MethodPost {
		ticket = strings.TrimSpace(c.PostForm("ticket"))
		csrf := strings.TrimSpace(c.PostForm("csrf"))
		if ticket == "" {
			renderStartError(
				http.StatusBadRequest,
				"授权链接无效或已过期，请返回客户端重新发起连接。",
				"Authorization link is invalid or expired. Please return to OmniMux and try again.",
			)
			return
		}
		if !socialOAuthConfirmCookieValid(c, ticket, csrf) {
			renderStartError(
				http.StatusForbidden,
				"授权确认已失效，请重新打开授权链接并确认。",
				"Authorization confirmation has expired. Please reopen the link and confirm again.",
			)
			return
		}
		clearSocialOAuthConfirmCookie(c, ticket)
		info, err := socialSvc.InspectTikTokOAuthTicket(c.Request.Context(), ticket)
		if err != nil {
			renderTicketErr(err)
			return
		}
		if sessionUserID > 0 && sessionUserID != info.UserId {
			renderStartError(
				http.StatusForbidden,
				"此授权链接属于其他 OmniMux 账号，已拒绝绑定。",
				"This authorization link belongs to another OmniMux account and has been rejected.",
			)
			return
		}
	} else {
		ticket = strings.TrimSpace(c.Query("ticket"))
		if ticket == "" {
			renderStartError(
				http.StatusBadRequest,
				"授权链接无效或已过期，请返回客户端重新发起连接。",
				"Authorization link is invalid or expired. Please return to OmniMux and try again.",
			)
			return
		}
		info, err := socialSvc.InspectTikTokOAuthTicket(c.Request.Context(), ticket)
		if err != nil {
			renderTicketErr(err)
			return
		}
		if sessionUserID > 0 && sessionUserID != info.UserId {
			renderStartError(
				http.StatusForbidden,
				"此授权链接属于其他 OmniMux 账号，已拒绝绑定。",
				"This authorization link belongs to another OmniMux account and has been rejected.",
			)
			return
		}
		// Unauthenticated browsers (for example CLI browser handoff) must explicitly
		// confirm via a CSRF-bound POST before consuming the launch ticket and
		// binding the OAuth state cookie, blocking cross-site login-CSRF attacks.
		if sessionUserID == 0 {
			nonce, err := setSocialOAuthConfirmCookie(c, ticket)
			if err != nil {
				renderStartError(
					http.StatusInternalServerError,
					"无法启动 TikTok 授权，请稍后重试。",
					"Unable to start TikTok authorization. Please try again later.",
				)
				return
			}
			c.Header("Vary", "Accept-Language")
			c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; form-action 'self'")
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderTikTokStartConfirmPage(lang, info.AccountLabel, ticket, nonce)))
			return
		}
	}

	res, err := socialSvc.StartTikTokOAuth(c.Request.Context(), ticket)
	if err != nil {
		renderTicketErr(err)
		return
	}
	c.Header("Content-Security-Policy", "frame-ancestors 'none'")
	setSocialOAuthBrowserCookie(c, res.State)
	c.Redirect(http.StatusFound, res.AuthorizeURL)
}

func renderTikTokStartConfirmPage(lang, accountLabel, ticket, csrf string) string {
	htmlLang := "en"
	title := "Confirm TikTok Account Connection"
	bodyPrefix := "You are about to connect your TikTok account to OmniMux account "
	bodySuffix := ". If you did not initiate this connection, close this page."
	buttonLabel := "Continue to TikTok Authorization"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		htmlLang = "zh"
		title = "确认连接 TikTok 账号"
		bodyPrefix = "您即将把 TikTok 账号绑定到 OmniMux 账号 "
		bodySuffix = "。如果不是您本人发起的连接，请直接关闭此页面。"
		buttonLabel = "继续前往 TikTok 授权"
	}
	safeAccount := html.EscapeString(accountLabel)
	safeTicket := html.EscapeString(ticket)
	safeCSRF := html.EscapeString(csrf)
	return `<!doctype html><html lang="` + htmlLang + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + title + `</title>` +
		`<style>body{font-family:-apple-system,system-ui,sans-serif;background:#0A0A0B;color:#EDEDED;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}main{text-align:center;max-width:420px;padding:32px}h1{font-size:22px;margin:0 0 12px}p{color:#A19F9D;font-size:14px;line-height:1.6}strong{color:#EDEDED}button{margin-top:20px;padding:10px 22px;background:#EDEDED;color:#0A0A0B;border:0;border-radius:999px;font-size:14px;font-weight:600;cursor:pointer}</style></head><body><main>` +
		tikTokCallbackLogoSVG +
		`<h1>` + title + `</h1>` +
		`<p>` + bodyPrefix + `<strong>` + safeAccount + `</strong>` + bodySuffix + `</p>` +
		`<form method="post" action="/api/social/v1/oauth/tiktok/start">` +
		`<input type="hidden" name="ticket" value="` + safeTicket + `">` +
		`<input type="hidden" name="csrf" value="` + safeCSRF + `">` +
		`<button type="submit">` + buttonLabel + `</button></form>` +
		`</main></body></html>`
}

// SocialTikTokCallback handles the official TikTok OAuth redirect (anonymous, state-bound).
// fork: tiktok-direct
// GET (and POST for defensive parity) /oauth/tiktok/callback and /api/social/v1/oauth/tiktok/callback?code=...&state=...
func SocialTikTokCallback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; sandbox")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Vary", "Accept-Language")
	lang := i18n.StatedLang(c)
	isZh := strings.HasPrefix(lang, "zh")
	errTitle := "TikTok Authorization Failed"
	if isZh {
		errTitle = "TikTok 授权失败"
	}
	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		code = c.PostForm("code")
	}
	if state == "" {
		state = c.PostForm("state")
	}
	if code == "" || state == "" {
		msg := "missing code or state"
		if isZh {
			msg = "缺少授权参数（code 或 state），请返回客户端重新发起连接。"
		}
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage(lang, errTitle, msg, "")))
		return
	}
	if !socialOAuthBrowserCookieValid(c, state) {
		msg := "authorization flow does not belong to this browser"
		if isZh {
			msg = "当前浏览器与发起授权的会话不一致，请在同一浏览器中重试。"
		}
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage(lang, errTitle, msg, "")))
		return
	}
	// Clear the one-time browser binding cookie before writing the response so Set-Cookie is sent.
	clearSocialOAuthBrowserCookie(c, state)
	res, err := socialSvc.HandleTikTokCallback(c.Request.Context(), code, state)
	if err != nil {
		sanitized := err.Error()
		lower := strings.ToLower(sanitized)
		if strings.Contains(lower, "bearer") || strings.Contains(lower, "client_secret") || strings.Contains(sanitized, state) || strings.Contains(sanitized, code) {
			sanitized = "upstream request failed"
		}
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage(lang, errTitle, sanitized, "")))
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
	title := "TikTok Connected"
	msg := "Account @" + handle + " is connected to OmniMux. You can return to the app now."
	if isZh {
		title = "TikTok 已连接"
		msg = "账号 @" + handle + " 已绑定到 OmniMux，可以返回客户端了。"
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderTikTokCallbackPage(lang, title, msg, handle)))
}

// HandleTikTokCallback is exported on the service for controller wiring.
// (Wrapper keeps service method testable without gin.)

// tikTokCallbackLogoSVG is the inline brand mark derived from web/public/logo.svg
// (without the geometrically redundant clipPath) so TikTok OAuth callback pages
// render cleanly under Content-Security-Policy: default-src 'none'; ...; sandbox.
// fork: tiktok-direct
const tikTokCallbackLogoSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1254 1254" width="56" height="56" fill="none" aria-hidden="true" style="display:block;margin:0 auto 16px">` +
	`<rect width="1254" height="1254" rx="280" fill="#b8b7ff"/>` +
	`<g transform="translate(627 627) scale(0.9) translate(-627 -627)">` +
	`<g transform="translate(0 1254) scale(0.1 -0.1)" fill="#121213" stroke="none">` +
	`<path d="M6140 11075 c-139 -45 -194 -180 -155 -380 64 -321 -32 -625 -266 -851 -124 -119 -249 -191 -570 -330 -668 -288 -1134 -629 -1523 -1116 -386 -481 -639 -1054 -746 -1689 -42 -253 -54 -430 -47 -728 7 -336 18 -419 137 -1061 102 -550 86 -978 -48 -1263 -62 -133 -119 -212 -257 -357 -127 -134 -182 -205 -225 -290 -113 -221 -83 -455 80 -633 77 -84 163 -140 265 -174 69 -23 97 -27 205 -27 186 -1 248 20 506 170 152 88 208 107 314 108 75 1 91 -3 140 -28 80 -42 165 -135 239 -262 183 -314 288 -417 499 -492 69 -24 91 -27 212 -27 146 1 196 11 318 66 111 50 203 120 368 280 173 167 248 227 362 283 189 95 420 102 619 19 110 -47 208 -121 388 -293 180 -172 287 -251 404 -300 152 -62 333 -78 471 -39 217 60 345 181 541 513 78 133 156 216 235 253 50 23 72 27 144 27 102 -1 153 -18 290 -100 290 -173 465 -215 679 -164 115 28 193 71 276 155 143 143 202 333 161 523 -31 147 -88 237 -274 434 -66 70 -141 156 -166 192 -67 93 -133 240 -169 374 -31 117 -31 118 -31 382 -1 294 9 398 70 715 37 193 84 492 106 670 8 71 13 228 13 445 -1 356 -6 421 -61 720 -194 1062 -829 1958 -1753 2476 -230 129 -268 182 -312 441 -29 171 -60 278 -119 401 -199 417 -609 800 -1005 937 -98 34 -244 43 -315 20z"/>` +
	`</g>` +
	`<rect x="447" y="562" width="75" height="172" rx="37.5" ry="37.5" fill="#ffffff"/>` +
	`<rect x="731" y="562" width="75" height="172" rx="37.5" ry="37.5" fill="#ffffff"/>` +
	`</g></svg>`

func renderTikTokCallbackPage(lang, title, msg, handle string) string {
	htmlLang, ctaLabel := "en", "Return to OmniMux"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		htmlLang, ctaLabel = "zh", "返回 OmniMux"
	}
	deepLink := ""
	if handle != "" {
		deepLink = `<p><a href="omnimux://social/connected" style="display:inline-block;margin-top:12px;padding:10px 18px;background:#000;color:#fff;border-radius:999px;text-decoration:none">` + ctaLabel + `</a></p>`
	}
	safeTitle := html.EscapeString(title)
	safeMsg := html.EscapeString(msg)
	return `<!doctype html><html lang="` + htmlLang + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + safeTitle + `</title>` +
		`<style>body{font-family:-apple-system,system-ui,sans-serif;background:#0A0A0B;color:#EDEDED;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}main{text-align:center;max-width:420px;padding:32px}h1{font-size:22px;margin:0 0 8px}p{color:#A19F9D;font-size:14px;line-height:1.6}</style></head><body><main>` +
		tikTokCallbackLogoSVG +
		`<h1>` + safeTitle + `</h1><p>` + safeMsg + `</p>` + deepLink +
		`<p style="margin-top:24px;font-size:12px;color:#5b5b5b">omnimux.ai — Twinspace Labs LLC</p></main></body></html>`
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
	Provider        string                    `json:"provider"`
	Content         string                    `json:"content"`
	AccountIDs      []int64                   `json:"account_ids"`
	PublishNow      *bool                     `json:"publish_now"`
	ScheduledFor    string                    `json:"scheduled_for"`
	Timezone        string                    `json:"timezone"`
	MediaItems      []map[string]any          `json:"media_items"`
	Queued          bool                      `json:"queued"`
	TikTokOptions   *social.TikTokPostOptions `json:"tiktok_options"`
	Title           string                    `json:"title,omitempty"`
	Description     string                    `json:"description,omitempty"`
	PhotoCoverIndex *int                      `json:"photo_cover_index,omitempty"`
	IsAIGC          *bool                     `json:"is_aigc,omitempty"`
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
		Provider:        req.Provider,
		Content:         req.Content,
		AccountIDs:      req.AccountIDs,
		PublishNow:      publishNow,
		ScheduledFor:    req.ScheduledFor,
		Timezone:        req.Timezone,
		MediaItems:      req.MediaItems,
		IdempotencyKey:  idem,
		Queued:          req.Queued,
		TikTokOptions:   req.TikTokOptions,
		Title:           req.Title,
		Description:     req.Description,
		PhotoCoverIndex: req.PhotoCoverIndex,
		IsAIGC:          req.IsAIGC,
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

// SocialTikTokWebhook POST /api/social/v1/webhooks/tiktok
// fork: tiktok-webhooks — fail-closed, HMAC-SHA256 verified Content Posting webhook endpoint.
func SocialTikTokWebhook(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")

	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}

	if !socialSvc.TikTokWebhookReady() {
		logger.LogWarn(c.Request.Context(), common.LogText("TikTok webhook rejected reason=webhook_disabled path=%q client_ip=%s", path, c.ClientIP()))
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "tiktok webhook is disabled",
			"code":    "SOCIAL_WEBHOOK_DISABLED",
		})
		return
	}

	var rawBody []byte
	if c.Request.Body != nil {
		var err error
		rawBody, err = io.ReadAll(io.LimitReader(c.Request.Body, maxTikTokWebhookBodyBytes+1))
		if err != nil {
			logger.LogWarn(c.Request.Context(), common.LogText("TikTok webhook failed to read body path=%q client_ip=%s", path, c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "failed to read request body",
				"code":    "SOCIAL_WEBHOOK_INVALID",
			})
			return
		}
		if len(rawBody) > maxTikTokWebhookBodyBytes {
			logger.LogWarn(c.Request.Context(), common.LogText("TikTok webhook rejected reason=payload_too_large path=%q client_ip=%s max_bytes=%d", path, c.ClientIP(), maxTikTokWebhookBodyBytes))
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"message": "request body too large",
				"code":    "SOCIAL_WEBHOOK_PAYLOAD_TOO_LARGE",
			})
			return
		}
	}

	_, err := socialSvc.HandleTikTokWebhook(c.Request.Context(), c.Request.Header.Values("TikTok-Signature"), rawBody)
	if err != nil {
		if errors.Is(err, social.ErrTikTokWebhookDisabled) {
			logger.LogWarn(c.Request.Context(), common.LogText("TikTok webhook rejected reason=webhook_disabled path=%q client_ip=%s", path, c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "tiktok webhook is disabled",
				"code":    "SOCIAL_WEBHOOK_DISABLED",
			})
			return
		}
		var authErr *social.TikTokWebhookAuthError
		if errors.As(err, &authErr) {
			if authErr.Reason == social.TikTokWebhookRejectTimestampOutOfWindow {
				logger.LogWarn(c.Request.Context(), common.LogText(
					"TikTok webhook auth failed reason=%s sig_age_seconds=%d tolerance_seconds=%d path=%q client_ip=%s",
					authErr.Reason, authErr.SigAgeSeconds, authErr.ToleranceSeconds, path, c.ClientIP(),
				))
			} else {
				logger.LogWarn(c.Request.Context(), common.LogText(
					"TikTok webhook auth failed reason=%s path=%q client_ip=%s",
					authErr.Reason, path, c.ClientIP(),
				))
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "unauthorized",
				"code":    "SOCIAL_WEBHOOK_UNAUTHORIZED",
			})
			return
		}
		if errors.Is(err, social.ErrTikTokWebhookBadPayload) {
			logger.LogWarn(c.Request.Context(), common.LogText("TikTok webhook rejected reason=invalid_payload path=%q client_ip=%s", path, c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "invalid webhook payload",
				"code":    "SOCIAL_WEBHOOK_INVALID",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "internal error",
			"code":    "SOCIAL_WEBHOOK_INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
