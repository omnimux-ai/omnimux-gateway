package social

// fork: tiktok-direct — official TikTok Login Kit + Content Posting path.
// Calls must explicitly request tiktok_direct; unavailable direct service never falls back.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social/tiktok"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

const (
	// ProviderTikTokDirectStart stores short-lived one-time browser launch tickets
	// in social_oauth_states without requiring a schema migration.
	ProviderTikTokDirectStart = "tiktok_direct_start"
	tikTokOAuthStartPath      = "/api/social/v1/oauth/tiktok/start"
	tikTokOAuthStartTicketTTL = 3 * time.Minute

	tikTokTokenRefreshWindow       = 10 * time.Minute
	tikTokRefreshRedisLockPrefix   = "social:tiktok:refresh_lock:"
	tikTokRefreshRedisLockTTL      = 20 * time.Second
	tikTokRefreshRedisWaitLimit    = 5 * time.Second
	tikTokRefreshRedisPollInterval = 50 * time.Millisecond
)

var (
	ErrTikTokDirectNotConfigured = errors.New("tiktok direct is not configured")
	ErrTikTokOAuthTicketInvalid  = errors.New("tiktok oauth launch ticket is invalid or already used")
	ErrTikTokOAuthTicketExpired  = errors.New("tiktok oauth launch ticket expired")
	ErrTikTokAuthorizeURLInvalid = errors.New("tiktok authorize url is not allowed")
	ErrTikTokTokenExpired        = errors.New("tiktok access token expired; reconnect account")
)

var tikTokRefreshUnlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

type accountLockEntry struct {
	mu   sync.Mutex
	refs int
}

type accountRefreshLocker struct {
	mu    sync.Mutex
	locks map[int64]*accountLockEntry
}

func (l *accountRefreshLocker) lock(accountID int64) func() {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[int64]*accountLockEntry)
	}
	entry := l.locks[accountID]
	if entry == nil {
		entry = &accountLockEntry{}
		l.locks[accountID] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.locks, accountID)
		}
		l.mu.Unlock()
	}
}

func (s *Service) tiktokClient() *tiktok.Client {
	return tiktok.NewClient(s.cfg.TikTokBaseURL)
}

// tiktokDirectUsable validates feature + credentials before any direct call.
func (s *Service) tiktokDirectUsable() error {
	if !s.cfg.Enabled {
		return ErrDisabled
	}
	if !s.cfg.TikTokDirectActive() {
		return ErrTikTokDirectNotConfigured
	}
	if !s.cfg.TikTokDirectConfigured() {
		return ErrTikTokDirectNotConfigured
	}
	return nil
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) effectiveTikTokDefaultRedirect() string {
	defaultRedirect := strings.TrimSpace(s.cfg.DefaultRedirectURL)
	if defaultRedirect == "" {
		defaultRedirect = strings.TrimSpace(s.cfg.TikTokRedirectURI)
	}
	return defaultRedirect
}

func (s *Service) effectiveTikTokAuthorizeBase() string {
	base := strings.TrimSpace(s.cfg.TikTokAuthorizeURL)
	if base == "" {
		return defaultTikTokAuthorizeURL
	}
	return base
}

func validateTikTokAuthorizeURL(rawURL string, requireNoQuery bool) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil || u.User != nil || u.Fragment != "" {
		return ErrTikTokAuthorizeURLInvalid
	}
	if requireNoQuery && u.RawQuery != "" {
		return ErrTikTokAuthorizeURLInvalid
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return ErrTikTokAuthorizeURLInvalid
	}
	if !strings.EqualFold(u.Host, "www.tiktok.com") {
		return ErrTikTokAuthorizeURLInvalid
	}
	if u.Path != "/v2/auth/authorize" && u.Path != "/v2/auth/authorize/" {
		return ErrTikTokAuthorizeURLInvalid
	}
	return nil
}

func tikTokOAuthStartURL(redirectURI, ticket string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || u == nil || u.Host == "" || u.User != nil {
		return "", ErrTikTokDirectNotConfigured
	}
	scheme := strings.ToLower(u.Scheme)
	isLoopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	if scheme != "https" && !(scheme == "http" && isLoopback) {
		return "", ErrTikTokDirectNotConfigured
	}
	q := url.Values{}
	q.Set("ticket", ticket)
	return (&url.URL{
		Scheme:   scheme,
		Host:     u.Host,
		Path:     tikTokOAuthStartPath,
		RawQuery: q.Encode(),
	}).String(), nil
}

// connectTikTokDirect starts official OAuth by issuing a short-lived one-time
// browser launch ticket. The system browser redeems it on /oauth/tiktok/start
// so the CSRF binding cookie is set in the browser that receives the callback.
func (s *Service) connectTikTokDirect(ctx context.Context, userId int, redirectAfter string) (string, error) {
	if err := s.tiktokDirectUsable(); err != nil {
		return "", err
	}
	if err := validateTikTokAuthorizeURL(s.effectiveTikTokAuthorizeBase(), true); err != nil {
		return "", err
	}
	// Seat preflight parity with Zernio path (single billing story).
	if err := s.PreflightConnect(userId); err != nil {
		return "", err
	}
	redirectAfter = strings.TrimSpace(redirectAfter)
	if redirectAfter != "" {
		if err := validateRedirectURL(redirectAfter, s.effectiveTikTokDefaultRedirect()); err != nil {
			return "", err
		}
	}
	ticket, err := randomState()
	if err != nil {
		return "", err
	}
	startURL, err := tikTokOAuthStartURL(s.cfg.TikTokRedirectURI, ticket)
	if err != nil {
		return "", err
	}
	if err := model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:         ticket,
		UserId:        userId,
		Provider:      ProviderTikTokDirectStart,
		RedirectAfter: redirectAfter,
		ExpiresAt:     time.Now().UTC().Add(tikTokOAuthStartTicketTTL),
	}); err != nil {
		return "", err
	}
	return startURL, nil
}

// TikTokStartResult holds the fresh OAuth state and validated TikTok authorize URL.
type TikTokStartResult struct {
	State        string
	AuthorizeURL string
}

// StartTikTokOAuth atomically consumes a one-time launch ticket, persists a fresh
// OAuth state bound to the same user, and returns the strictly validated TikTok
// authorize URL for browser redirection.
func (s *Service) StartTikTokOAuth(ctx context.Context, ticket string) (*TikTokStartResult, error) {
	if err := s.tiktokDirectUsable(); err != nil {
		return nil, err
	}
	if ticket == "" || len(ticket) > 128 || strings.TrimSpace(ticket) != ticket {
		return nil, ErrTikTokOAuthTicketInvalid
	}
	authorizeBase := s.effectiveTikTokAuthorizeBase()
	if err := validateTikTokAuthorizeURL(authorizeBase, true); err != nil {
		return nil, err
	}
	st, err := model.ConsumeSocialOAuthState(ticket, ProviderTikTokDirectStart)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTikTokOAuthTicketInvalid
		}
		if strings.Contains(strings.ToLower(err.Error()), "expired") {
			return nil, ErrTikTokOAuthTicketExpired
		}
		return nil, err
	}
	if st.UserId <= 0 {
		return nil, ErrTikTokOAuthTicketInvalid
	}
	redirectAfter := strings.TrimSpace(st.RedirectAfter)
	if redirectAfter != "" {
		if err := validateRedirectURL(redirectAfter, s.effectiveTikTokDefaultRedirect()); err != nil {
			return nil, err
		}
	}
	state, err := randomState()
	if err != nil {
		return nil, err
	}
	if err := model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:         state,
		UserId:        st.UserId,
		Provider:      ProviderTikTokDirect,
		RedirectAfter: redirectAfter,
	}); err != nil {
		return nil, err
	}
	key, _ := s.cfg.TikTokCredentials()
	authURL := tiktok.AuthorizeURL(authorizeBase, key, s.cfg.TikTokRedirectURI, s.cfg.TikTokScopes, state)
	if err := validateTikTokAuthorizeURL(authURL, false); err != nil {
		return nil, err
	}
	return &TikTokStartResult{State: state, AuthorizeURL: authURL}, nil
}

// TikTokCallbackResult is returned to the controller for redirect/render.
type TikTokCallbackResult struct {
	Account       *model.SocialAccount
	RedirectAfter string
}

// HandleTikTokCallback consumes state, exchanges code, and upserts the bound account.
func (s *Service) HandleTikTokCallback(ctx context.Context, code, state string) (*TikTokCallbackResult, error) {
	if err := s.tiktokDirectUsable(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("missing authorization code")
	}
	st, err := model.ConsumeSocialOAuthState(state, ProviderTikTokDirect)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("oauth state not found or already used")
		}
		return nil, err
	}
	key, secret := s.cfg.TikTokCredentials()
	client := s.tiktokClient()
	tok, err := client.ExchangeToken(ctx, key, secret, code, s.cfg.TikTokRedirectURI)
	if err != nil {
		return nil, err
	}
	info, err := client.GetUserInfo(ctx, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	openID := info.OpenID
	if openID == "" {
		openID = tok.OpenID
	}
	if openID == "" {
		return nil, errors.New("tiktok: callback: empty open_id")
	}
	display := info.DisplayName
	username := info.Username
	if username == "" {
		username = display
	}
	if display == "" {
		display = username
	}
	// Encrypt tokens at rest; fail closed when the key is missing (no plaintext rows).
	accessEnc, err := SealToken(tok.AccessToken, s.cfg.SocialTokenEncKey)
	if err != nil {
		return nil, err
	}
	refreshEnc, err := SealToken(tok.RefreshToken, s.cfg.SocialTokenEncKey)
	if err != nil {
		return nil, err
	}
	var expiresAt *time.Time
	if tok.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second)
		expiresAt = &t
	}
	scopes := strings.TrimSpace(tok.Scope)
	meta, _ := common.Marshal(map[string]any{
		"avatar_url": info.AvatarURL,
		"union_id":   info.UnionID,
	})
	acc := &model.SocialAccount{
		UserId:            st.UserId,
		Provider:          ProviderTikTokDirect,
		ProviderAccountID: openID,
		ProviderProfileID: openID,
		Platform:          "tiktok",
		Username:          username,
		DisplayName:       display,
		Status:            model.SocialAccountStatusActive,
	}
	if err := model.UpsertSocialAccount(acc); err != nil {
		return nil, err
	}
	saved, err := model.GetSocialAccountByProviderRef(ProviderTikTokDirect, openID)
	if err != nil {
		return nil, err
	}
	// Persist token vault on the saved row (UpsertSocialAccount intentionally does not touch vault columns).
	updates := map[string]any{
		"access_token_encrypted":  accessEnc,
		"refresh_token_encrypted": refreshEnc,
		"token_expires_at":        expiresAt,
		"scopes_granted":          scopes,
		"metadata":                string(meta),
		"updated_at":              time.Now().UTC(),
	}
	if err := model.DB.Model(&model.SocialAccount{}).Where("id = ? AND user_id = ?", saved.ID, saved.UserId).Updates(updates).Error; err != nil {
		return nil, err
	}
	saved, err = model.GetSocialAccountByIDForUser(saved.ID, saved.UserId)
	if err != nil {
		return nil, err
	}
	redirectAfter := strings.TrimSpace(st.RedirectAfter)
	if redirectAfter != "" {
		if err := validateRedirectURL(redirectAfter, s.effectiveTikTokDefaultRedirect()); err != nil {
			redirectAfter = ""
		}
	}
	return &TikTokCallbackResult{Account: saved, RedirectAfter: redirectAfter}, nil
}

func needsTikTokTokenRefresh(acc *model.SocialAccount, now time.Time) bool {
	return acc != nil && acc.TokenExpiresAt != nil && !now.Add(tikTokTokenRefreshWindow).Before(*acc.TokenExpiresAt)
}

func isTikTokTokenExpired(acc *model.SocialAccount, now time.Time) bool {
	return acc != nil && acc.TokenExpiresAt != nil && !now.Before(*acc.TokenExpiresAt)
}

func isTikTokTokenInvalidError(err error) bool {
	var apiErr *tiktok.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(apiErr.Code)) {
	case "invalid_grant", "invalid_token", "invalid_request", "unauthorized_client",
		"access_denied", "access_token_invalid", "refresh_token_invalid",
		"token_expired", "token_revoked", "scope_not_authorized":
		return true
	default:
		return false
	}
}

func sanitizeTikTokRefreshError(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(msg, secret) {
			return errors.New("tiktok token refresh failed")
		}
	}
	return fmt.Errorf("tiktok token refresh failed: %w", err)
}

func acquireTikTokRefreshRedisLock(ctx context.Context, accountID int64) (func(), error) {
	if !common.RedisEnabled || common.RDB == nil || accountID <= 0 {
		return func() {}, nil
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return func() {}, nil
	}
	key := tikTokRefreshRedisLockPrefix + strconv.FormatInt(accountID, 10)
	holder := hex.EncodeToString(raw)
	deadline := time.Now().Add(tikTokRefreshRedisWaitLimit)
	for {
		held, err := common.RDB.SetNX(ctx, key, holder, tikTokRefreshRedisLockTTL).Result()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// Fall back to DB conditional update when Redis is transiently unavailable.
			return func() {}, nil
		}
		if held {
			return func() {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tikTokRefreshUnlockScript.Run(releaseCtx, common.RDB, []string{key}, holder).Err()
			}, nil
		}
		if time.Now().After(deadline) {
			return func() {}, nil
		}
		timer := time.NewTimer(tikTokRefreshRedisPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// accessTokenForAccount decrypts (and refreshes with per-account serialization when near expiry)
// the account's access token without mutating the caller's struct.
func (s *Service) accessTokenForAccount(ctx context.Context, acc *model.SocialAccount) (string, error) {
	tok, _, err := s.accessTokenAndScopesForAccount(ctx, acc)
	return tok, err
}

func (s *Service) accessTokenAndScopesForAccount(ctx context.Context, acc *model.SocialAccount) (string, string, error) {
	if acc == nil || acc.AccessTokenEncrypted == "" {
		return "", "", ErrTikTokDirectNotConfigured
	}
	now := time.Now().UTC()
	if !needsTikTokTokenRefresh(acc, now) {
		plain, err := OpenToken(acc.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if err != nil {
			return "", "", err
		}
		return plain, acc.ScopesGranted, nil
	}

	unlockLocal := s.refreshLocks.lock(acc.ID)
	defer unlockLocal()

	unlockRedis, err := acquireTikTokRefreshRedisLock(ctx, acc.ID)
	if err != nil {
		return "", "", err
	}
	defer unlockRedis()

	// Double-check latest DB row after acquiring lock(s).
	current := acc
	if acc.ID > 0 && acc.UserId > 0 {
		latest, loadErr := model.GetSocialAccountByIDForUser(acc.ID, acc.UserId)
		if loadErr == nil && latest != nil {
			current = latest
		} else if loadErr != nil && !errors.Is(loadErr, gorm.ErrRecordNotFound) {
			return "", "", loadErr
		}
	}
	if current.AccessTokenEncrypted == "" {
		return "", "", ErrTikTokDirectNotConfigured
	}
	now = time.Now().UTC()
	if !needsTikTokTokenRefresh(current, now) {
		plain, err := OpenToken(current.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if err != nil {
			return "", "", err
		}
		return plain, current.ScopesGranted, nil
	}
	if current.RefreshTokenEncrypted == "" {
		if isTikTokTokenExpired(current, now) {
			return "", "", ErrTikTokTokenExpired
		}
		plain, err := OpenToken(current.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if err != nil {
			return "", "", err
		}
		return plain, current.ScopesGranted, nil
	}
	rt, err := OpenToken(current.RefreshTokenEncrypted, s.cfg.SocialTokenEncKey)
	if err != nil {
		if isTikTokTokenExpired(current, now) {
			return "", "", ErrTikTokTokenExpired
		}
		plain, openErr := OpenToken(current.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if openErr != nil {
			return "", "", openErr
		}
		return plain, current.ScopesGranted, nil
	}

	oldRefreshEnc := current.RefreshTokenEncrypted
	key, secret := s.cfg.TikTokCredentials()
	tok, err := s.tiktokClient().RefreshToken(ctx, key, secret, rt)
	if err != nil {
		// Re-read DB to see if another instance concurrently rotated the refresh token.
		if current.ID > 0 && current.UserId > 0 {
			if latest, rereadErr := model.GetSocialAccountByIDForUser(current.ID, current.UserId); rereadErr == nil &&
				latest != nil &&
				latest.RefreshTokenEncrypted != "" &&
				latest.RefreshTokenEncrypted != oldRefreshEnc &&
				latest.AccessTokenEncrypted != "" &&
				!isTikTokTokenExpired(latest, time.Now().UTC()) {
				plain, openErr := OpenToken(latest.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
				if openErr != nil {
					return "", "", openErr
				}
				return plain, latest.ScopesGranted, nil
			}
		}
		plain, openErr := OpenToken(current.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if isTikTokTokenExpired(current, time.Now().UTC()) || isTikTokTokenInvalidError(err) {
			return "", "", sanitizeTikTokRefreshError(err, rt, plain, key, secret)
		}
		if openErr != nil {
			return "", "", sanitizeTikTokRefreshError(err, rt, key, secret)
		}
		return plain, current.ScopesGranted, nil
	}

	if strings.TrimSpace(tok.AccessToken) == "" {
		return "", "", errors.New("tiktok token refresh returned empty access token")
	}
	accessEnc, err := SealToken(tok.AccessToken, s.cfg.SocialTokenEncKey)
	if err != nil {
		return "", "", err
	}
	refreshEnc, err := SealToken(firstNonEmpty(tok.RefreshToken, rt), s.cfg.SocialTokenEncKey)
	if err != nil {
		return "", "", err
	}
	now = time.Now().UTC()
	updates := map[string]any{
		"access_token_encrypted":  accessEnc,
		"refresh_token_encrypted": refreshEnc,
		"updated_at":              now,
	}
	if tok.ExpiresIn > 0 {
		t := now.Add(time.Duration(tok.ExpiresIn) * time.Second)
		updates["token_expires_at"] = t
	}
	newScopes := strings.TrimSpace(tok.Scope)
	if newScopes != "" {
		updates["scopes_granted"] = newScopes
	}
	res := model.DB.Model(&model.SocialAccount{}).
		Where("id = ? AND user_id = ? AND refresh_token_encrypted = ?", current.ID, current.UserId, oldRefreshEnc).
		Updates(updates)
	if res.Error != nil {
		return "", "", res.Error
	}
	if res.RowsAffected == 0 {
		// Another instance rotated refresh_token_encrypted concurrently; re-read instead of overwriting.
		latest, rereadErr := model.GetSocialAccountByIDForUser(current.ID, current.UserId)
		if rereadErr != nil {
			return "", "", rereadErr
		}
		if latest.AccessTokenEncrypted == "" {
			return "", "", ErrTikTokDirectNotConfigured
		}
		if isTikTokTokenExpired(latest, time.Now().UTC()) {
			return "", "", ErrTikTokTokenExpired
		}
		plain, openErr := OpenToken(latest.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
		if openErr != nil {
			return "", "", openErr
		}
		return plain, latest.ScopesGranted, nil
	}
	return tok.AccessToken, firstNonEmpty(newScopes, current.ScopesGranted), nil
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// GetTikTokCreatorInfo queries live TikTok creator publishing settings for an owned direct account.
func (s *Service) GetTikTokCreatorInfo(ctx context.Context, userId int, provider string, localAccountID int64) (*tiktok.CreatorInfo, error) {
	if err := validateProvider(provider, s.cfg.ZernioActive()); err != nil {
		return nil, err
	}
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	acc, err := model.GetSocialAccountByIDForUser(localAccountID, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountMissing
		}
		return nil, err
	}
	if acc.Provider != provider {
		return nil, ErrAccountProviderMismatch
	}
	if provider != ProviderTikTokDirect {
		return nil, ErrPlatformDenied
	}
	if err := s.tiktokDirectUsable(); err != nil {
		return nil, err
	}
	accessToken, err := s.accessTokenForAccount(ctx, acc)
	if err != nil {
		return nil, err
	}
	return s.tiktokClient().QueryCreatorInfo(ctx, accessToken)
}

func resolveTikTokPostOptions(in *TikTokPostOptions) (*tiktok.PostOptions, error) {
	if in == nil {
		return &tiktok.PostOptions{PrivacyLevel: "SELF_ONLY"}, nil
	}
	if in.MusicUsageConfirmed != nil && !*in.MusicUsageConfirmed {
		return nil, fmt.Errorf("%w: music usage confirmation is required", ErrMediaInvalid)
	}
	privacy := strings.ToUpper(strings.TrimSpace(in.PrivacyLevel))
	if privacy == "" {
		privacy = "SELF_ONLY"
	}
	switch privacy {
	case "PUBLIC_TO_EVERYONE", "MUTUAL_FOLLOW_FRIENDS", "FOLLOWER_OF_CREATOR", "SELF_ONLY":
	default:
		return nil, fmt.Errorf("%w: unsupported tiktok privacy_level %q", ErrMediaInvalid, in.PrivacyLevel)
	}
	commOn := in.CommercialContent != nil && *in.CommercialContent
	yourBrand := in.YourBrand != nil && *in.YourBrand
	branded := in.BrandedContent != nil && *in.BrandedContent
	if commOn && !yourBrand && !branded {
		return nil, fmt.Errorf("%w: commercial content disclosure requires your_brand or branded_content", ErrMediaInvalid)
	}
	if branded && privacy == "SELF_ONLY" {
		return nil, fmt.Errorf("%w: branded content visibility cannot be SELF_ONLY", ErrMediaInvalid)
	}
	out := &tiktok.PostOptions{PrivacyLevel: privacy}
	if in.AllowComment != nil {
		dis := !*in.AllowComment
		out.DisableComment = &dis
	}
	if in.AllowDuet != nil {
		dis := !*in.AllowDuet
		out.DisableDuet = &dis
	}
	if in.AllowStitch != nil {
		dis := !*in.AllowStitch
		out.DisableStitch = &dis
	}
	if commOn || in.YourBrand != nil {
		v := commOn && yourBrand
		out.BrandOrganicToggle = &v
	}
	if commOn || in.BrandedContent != nil {
		v := commOn && branded
		out.BrandContentToggle = &v
	}
	return out, nil
}

var ErrTikTokPublishScopeMissing = errors.New("tiktok account has no publishing scope; reconnect after video.upload or video.publish is granted")

// tiktokPublishCapabilities derives publishing abilities from the granted scopes.
// TikTok grants video.upload (draft to inbox) and video.publish (direct post)
// independently; an approved app may hold only one of them.
func tiktokPublishCapabilities(scopes string) (uploadDraft, directPost bool) {
	for scope := range strings.SplitSeq(scopes, ",") {
		switch strings.TrimSpace(scope) {
		case "video.upload":
			uploadDraft = true
		case "video.publish":
			directPost = true
		}
	}
	return uploadDraft, directPost
}

// tiktokCapabilitiesFor derives publishing abilities strictly from the account's granted scopes (fail-closed).
func (s *Service) tiktokCapabilitiesFor(acc *model.SocialAccount) (uploadDraft, directPost bool) {
	if acc == nil {
		return false, false
	}
	return tiktokPublishCapabilities(acc.ScopesGranted)
}

// TikTokCapabilities reports what the account's granted TikTok scopes allow.
func (s *Service) TikTokCapabilities(acc *model.SocialAccount) (uploadDraft, directPost bool) {
	if acc == nil || acc.Provider != ProviderTikTokDirect {
		return false, false
	}
	return s.tiktokCapabilitiesFor(acc)
}

// uploadTikTokDraft sends one video to the creator's TikTok inbox. The creator
// finishes editing and posting inside TikTok; title and privacy options do not apply.
func (s *Service) uploadTikTokDraft(ctx context.Context, client *tiktok.Client, accessToken string, mediaItems []map[string]any) (*tiktok.PublishInitResponse, error) {
	if len(mediaItems) != 1 {
		return nil, fmt.Errorf("%w: tiktok draft upload accepts exactly one video", ErrMediaInvalid)
	}
	item := mediaItems[0]
	typ := strings.ToLower(fmt.Sprint(item["type"]))
	u := strings.TrimSpace(fmt.Sprint(item["url"]))
	if typ == "image" || isImageURL(u) {
		return nil, fmt.Errorf("%w: tiktok draft upload supports video only; photo posts require video.publish", ErrMediaInvalid)
	}
	wrapped, err := s.WrapTikTokVerifiedMediaURL(u)
	if err != nil {
		return nil, err
	}
	return client.PublishVideoToInboxByURL(ctx, accessToken, wrapped)
}

// createPostTikTokDirect publishes to a single direct account (v1: one account per call path branch).
func (s *Service) createPostTikTokDirect(ctx context.Context, userId int, acc *model.SocialAccount, in CreatePostInput, mediaItems []map[string]any) (*model.SocialPost, error) {
	if err := s.tiktokDirectUsable(); err != nil {
		return nil, err
	}
	// Validate media and TikTok compliance options first so SOCIAL_MEDIA_INVALID surfaces before credential errors.
	if len(mediaItems) == 0 {
		return nil, fmt.Errorf("%w: tiktok direct requires at least one media item (video or photo)", ErrMediaInvalid)
	}
	firstURL := fmt.Sprint(mediaItems[0]["url"])
	if strings.TrimSpace(firstURL) == "" {
		return nil, fmt.Errorf("%w: empty media url", ErrMediaInvalid)
	}
	uploadDraft, directPost := s.tiktokCapabilitiesFor(acc)
	if !uploadDraft && !directPost {
		return nil, ErrTikTokPublishScopeMissing
	}
	accessToken, effectiveScopes, err := s.accessTokenAndScopesForAccount(ctx, acc)
	if err != nil {
		return nil, err
	}
	if effectiveScopes != acc.ScopesGranted {
		uploadDraft, directPost = tiktokPublishCapabilities(effectiveScopes)
		if !uploadDraft && !directPost {
			return nil, ErrTikTokPublishScopeMissing
		}
	}
	client := s.tiktokClient()

	var pub *tiktok.PublishInitResponse
	title := in.Content
	if !directPost {
		pub, err = s.uploadTikTokDraft(ctx, client, accessToken, mediaItems)
	} else {
		postOpts, optsErr := resolveTikTokPostOptions(in.TikTokOptions)
		if optsErr != nil {
			return nil, optsErr
		}
		// Capability check per TikTok guidelines (best-effort; failure does not block v1).
		_, _ = client.QueryCreatorInfo(ctx, accessToken)
		{
			typ := strings.ToLower(fmt.Sprint(mediaItems[0]["type"]))
			u := firstURL
			if typ == "image" || isImageURL(u) {
				urls := make([]string, 0, len(mediaItems))
				for _, m := range mediaItems {
					uu := strings.TrimSpace(fmt.Sprint(m["url"]))
					if uu != "" {
						wrapped, werr := s.WrapTikTokVerifiedMediaURL(uu)
						if werr != nil {
							return nil, werr
						}
						urls = append(urls, wrapped)
					}
				}
				if len(urls) == 0 {
					return nil, fmt.Errorf("%w: empty photo urls", ErrMediaInvalid)
				}
				pub, err = client.PublishPhotoByURLWithOptions(ctx, accessToken, title, urls, postOpts)
			} else {
				if u == "" {
					return nil, fmt.Errorf("%w: empty video url", ErrMediaInvalid)
				}
				wrapped, werr := s.WrapTikTokVerifiedMediaURL(u)
				if werr != nil {
					return nil, werr
				}
				pub, err = client.PublishVideoByURLWithOptions(ctx, accessToken, title, wrapped, postOpts)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	publishMode := "draft"
	if directPost {
		publishMode = "direct"
	}
	reqSnap, _ := common.Marshal(map[string]any{
		"content_preview": truncate(in.Content, 200),
		"account_ids":     in.AccountIDs,
		"publish_now":     in.PublishNow,
		"scheduled_for":   in.ScheduledFor,
		"media_count":     len(mediaItems),
		"provider":        ProviderTikTokDirect,
		"publish_mode":    publishMode,
	})
	ledger := &model.SocialPost{
		UserId:          userId,
		Provider:        ProviderTikTokDirect,
		ProviderPostID:  pub.PublishID,
		IdempotencyKey:  in.IdempotencyKey,
		Status:          "submitted",
		ContentPreview:  truncate(in.Content, 200),
		RequestSnapshot: string(reqSnap),
	}
	if err := model.CreateSocialPost(ledger); err != nil {
		if in.IdempotencyKey != "" {
			if existing, e2 := model.GetSocialPostByIdempotency(userId, in.IdempotencyKey); e2 == nil {
				if err := validateIdempotentPost(existing, in); err != nil {
					return nil, err
				}
				return existing, nil
			}
		}
		return nil, err
	}
	return ledger, nil
}

// refreshTikTokPostStatus polls publish status for a direct post.
func (s *Service) refreshTikTokPostStatus(ctx context.Context, p *model.SocialPost) {
	acc, err := model.GetSocialAccountByProviderRef(ProviderTikTokDirect, providerAccountIDForPost(p))
	if err != nil || acc == nil {
		return
	}
	accessToken, err := s.accessTokenForAccount(ctx, acc)
	if err != nil {
		return
	}
	st, err := s.tiktokClient().GetPublishStatus(ctx, accessToken, p.ProviderPostID)
	if err != nil || st == nil {
		return
	}
	mapped := mapTikTokPublishStatus(st.Status)
	if mapped != "" && mapped != p.Status {
		snap, _ := common.Marshal(map[string]any{"status": st.Status, "fail_reason": st.FailReason, "publish_id": st.PublishID})
		_ = model.UpdateSocialPostResult(p.ID, mapped, p.ProviderPostID, string(snap))
	}
}

// providerAccountIDForPost resolves which direct account a post belongs to (stored in request snapshot).
func providerAccountIDForPost(p *model.SocialPost) string {
	// v1: single-account direct posts carry account open_id implicitly via provider row lookup by user.
	// Stored RequestSnapshot has account_ids (local ids) — resolve first local id → account open_id.
	var snap struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := common.Unmarshal([]byte(p.RequestSnapshot), &snap); err != nil || len(snap.AccountIDs) == 0 {
		return ""
	}
	acc, err := model.GetSocialAccountByIDForUser(snap.AccountIDs[0], p.UserId)
	if err != nil || acc == nil {
		return ""
	}
	return acc.ProviderAccountID
}

func mapTikTokPublishStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PUBLISH_COMPLETE":
		return "published"
	case "FAILED", "FAIL":
		return "failed"
	case "PROCESSING_DOWNLOAD", "PROCESSING_UPLOAD", "SEND_TO_USER_INBOX", "PUBLISH_INITIATED":
		return "processing"
	default:
		return ""
	}
}

func isImageURL(u string) bool {
	u = strings.ToLower(u)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		if strings.Contains(u, ext) {
			return true
		}
	}
	return false
}

// disconnectTikTokDirect revokes upstream token (best effort) and marks the account disconnected.
func (s *Service) disconnectTikTokDirect(ctx context.Context, userId int, acc *model.SocialAccount) error {
	if acc.AccessTokenEncrypted != "" {
		if plain, err := OpenToken(acc.AccessTokenEncrypted, s.cfg.SocialTokenEncKey); err == nil && plain != "" {
			key, secret := s.cfg.TikTokCredentials()
			_ = s.tiktokClient().Revoke(ctx, key, secret, plain) // best effort
		}
	}
	if err := model.MarkSocialAccountDisconnected(acc.ID, userId); err != nil {
		return err
	}
	return s.vacateAfterDisconnect(userId, acc.ID)
}
