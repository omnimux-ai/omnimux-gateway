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
	"path"
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

	tikTokMaxPhotoItems = 35
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

func isKnownTikTokImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".webp", ".png", ".gif", ".bmp", ".svg", ".heic", ".heif", ".avif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func isSupportedTikTokPhotoExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func isSupportedTikTokPhotoMIME(ct string) bool {
	switch ct {
	case "image/jpeg", "image/jpg", "image/webp":
		return true
	default:
		return false
	}
}

func isKnownTikTokVideoExt(ext string) bool {
	switch ext {
	case ".mp4", ".mov", ".webm", ".m4v", ".avi", ".mpeg", ".mpg":
		return true
	default:
		return false
	}
}

func itemStringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func itemContentType(rawItems []map[string]any, idx int) string {
	if idx < 0 || idx >= len(rawItems) || rawItems[idx] == nil {
		return ""
	}
	ct := firstString(rawItems[idx], "content_type", "contentType", "mime_type", "mimeType")
	if semi := strings.IndexByte(ct, ';'); semi >= 0 {
		ct = ct[:semi]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// classifyTikTokMediaItems validates media URLs, rejects mixed photo+video payloads,
// enforces TikTok's 1..35 JPEG/WebP photo rules (bounded by maxConfigured when set),
// and enforces single-video payloads.
func classifyTikTokMediaItems(rawItems, normalizedItems []map[string]any, maxConfigured int) (isPhoto bool, urls []string, err error) {
	if len(normalizedItems) == 0 {
		return false, nil, fmt.Errorf("%w: tiktok direct requires at least one media item (video or photo)", ErrMediaInvalid)
	}
	type classifiedItem struct {
		url string
		ext string
		ct  string
	}
	var hasImage, hasVideo bool
	items := make([]classifiedItem, 0, len(normalizedItems))
	for i, m := range normalizedItems {
		rawURL := itemStringValue(m, "url")
		if rawURL == "" {
			return false, nil, fmt.Errorf("%w: empty media url", ErrMediaInvalid)
		}
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil || parsed == nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return false, nil, fmt.Errorf("%w: invalid media url", ErrMediaInvalid)
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "https" && scheme != "http" {
			return false, nil, fmt.Errorf("%w: media url must use https", ErrMediaInvalid)
		}
		ext := strings.ToLower(path.Ext(parsed.Path))
		typ := strings.ToLower(itemStringValue(m, "type"))
		if typ == "photo" {
			typ = "image"
		}
		ct := itemContentType(rawItems, i)
		if typ != "" && typ != "image" && typ != "video" {
			return false, nil, fmt.Errorf("%w: unsupported tiktok media type %q", ErrMediaInvalid, typ)
		}
		imgSignal := typ == "image" || strings.HasPrefix(ct, "image/") || isKnownTikTokImageExt(ext)
		vidSignal := typ == "video" || strings.HasPrefix(ct, "video/") || isKnownTikTokVideoExt(ext)
		if imgSignal && vidSignal {
			return false, nil, fmt.Errorf("%w: conflicting media type and format in media_items[%d]", ErrMediaInvalid, i)
		}
		if ct != "" && !strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "video/") {
			return false, nil, fmt.Errorf("%w: unsupported tiktok content_type %q", ErrMediaInvalid, ct)
		}
		if imgSignal {
			hasImage = true
		} else if vidSignal || (ext == "" && typ == "" && ct == "") {
			hasVideo = true
		} else {
			return false, nil, fmt.Errorf("%w: unsupported tiktok media format %q", ErrMediaInvalid, ext)
		}
		items = append(items, classifiedItem{url: rawURL, ext: ext, ct: ct})
	}
	if hasImage && hasVideo {
		return false, nil, fmt.Errorf("%w: tiktok does not allow mixing photos and videos in one post", ErrMediaInvalid)
	}
	if hasImage {
		limit := tikTokMaxPhotoItems
		if maxConfigured > 0 && maxConfigured < limit {
			limit = maxConfigured
		}
		if len(items) < 1 || len(items) > limit {
			return false, nil, fmt.Errorf("%w: tiktok photo post accepts 1 to %d images", ErrMediaInvalid, limit)
		}
		urls = make([]string, 0, len(items))
		for _, it := range items {
			if it.ct != "" && !isSupportedTikTokPhotoMIME(it.ct) {
				return false, nil, fmt.Errorf("%w: tiktok photo posts support JPEG and WebP only (got %s)", ErrMediaInvalid, it.ct)
			}
			if it.ext != "" && !isSupportedTikTokPhotoExt(it.ext) {
				return false, nil, fmt.Errorf("%w: tiktok photo posts support JPEG and WebP only (got %s)", ErrMediaInvalid, it.ext)
			}
			if it.ct == "" && it.ext == "" {
				return false, nil, fmt.Errorf("%w: tiktok photo posts require a JPEG or WebP image url or content_type", ErrMediaInvalid)
			}
			urls = append(urls, it.url)
		}
		return true, urls, nil
	}
	if len(items) != 1 {
		return false, nil, fmt.Errorf("%w: tiktok video post accepts exactly one video", ErrMediaInvalid)
	}
	return false, []string{items[0].url}, nil
}

func resolveTikTokPhotoDraftOptions(in CreatePostInput, photoCount int) (*tiktok.PhotoDraftOptions, error) {
	var title, desc string
	var coverIdx *int
	var isAIGC *bool
	if in.TikTokOptions != nil {
		title = strings.TrimSpace(in.TikTokOptions.Title)
		desc = strings.TrimSpace(in.TikTokOptions.Description)
		coverIdx = in.TikTokOptions.PhotoCoverIndex
		isAIGC = in.TikTokOptions.IsAIGC
	}
	if title == "" {
		title = strings.TrimSpace(in.Title)
	}
	if desc == "" {
		desc = strings.TrimSpace(in.Description)
	}
	if desc == "" {
		desc = strings.TrimSpace(in.Content)
	}
	if coverIdx == nil {
		coverIdx = in.PhotoCoverIndex
	}
	if isAIGC == nil {
		isAIGC = in.IsAIGC
	}
	if coverIdx != nil && (*coverIdx < 0 || *coverIdx >= photoCount) {
		return nil, fmt.Errorf("%w: photo_cover_index %d out of range [0,%d)", ErrMediaInvalid, *coverIdx, photoCount)
	}
	if title == "" && desc == "" && coverIdx == nil && isAIGC == nil {
		return nil, nil
	}
	return &tiktok.PhotoDraftOptions{
		Title:           title,
		Description:     desc,
		PhotoCoverIndex: coverIdx,
		IsAIGC:          isAIGC,
	}, nil
}

func applyTikTokDirectExtraOptions(opts *tiktok.PostOptions, in CreatePostInput, isPhoto bool, photoCount int) (string, error) {
	title := strings.TrimSpace(in.Content)
	var optTitle, optDesc string
	var coverIdx *int
	var isAIGC *bool
	if in.TikTokOptions != nil {
		optTitle = strings.TrimSpace(in.TikTokOptions.Title)
		optDesc = strings.TrimSpace(in.TikTokOptions.Description)
		coverIdx = in.TikTokOptions.PhotoCoverIndex
		isAIGC = in.TikTokOptions.IsAIGC
	}
	if optTitle == "" {
		optTitle = strings.TrimSpace(in.Title)
	}
	if optTitle != "" {
		title = optTitle
	}
	if optDesc == "" {
		optDesc = strings.TrimSpace(in.Description)
	}
	if coverIdx == nil {
		coverIdx = in.PhotoCoverIndex
	}
	if isAIGC == nil {
		isAIGC = in.IsAIGC
	}
	if isPhoto {
		if coverIdx != nil && (*coverIdx < 0 || *coverIdx >= photoCount) {
			return "", fmt.Errorf("%w: photo_cover_index %d out of range [0,%d)", ErrMediaInvalid, *coverIdx, photoCount)
		}
		opts.Description = optDesc
		opts.PhotoCoverIndex = coverIdx
	}
	opts.IsAIGC = isAIGC
	return title, nil
}

// uploadTikTokDraft sends either a single video (/v2/post/publish/inbox/video/init/)
// or a photo carousel (/v2/post/publish/content/init/ with post_mode="MEDIA_UPLOAD")
// to the creator's TikTok inbox. Scope: video.upload.
func (s *Service) uploadTikTokDraft(ctx context.Context, client *tiktok.Client, accessToken string, in CreatePostInput, mediaItems []map[string]any) (*tiktok.PublishInitResponse, error) {
	isPhoto, rawURLs, err := classifyTikTokMediaItems(in.MediaItems, mediaItems, s.cfg.MaxMediaItems)
	if err != nil {
		return nil, err
	}
	if isPhoto {
		draftOpts, err := resolveTikTokPhotoDraftOptions(in, len(rawURLs))
		if err != nil {
			return nil, err
		}
		wrappedURLs := make([]string, 0, len(rawURLs))
		for _, u := range rawURLs {
			wrapped, werr := s.WrapTikTokVerifiedMediaURL(u)
			if werr != nil {
				return nil, werr
			}
			wrappedURLs = append(wrappedURLs, wrapped)
		}
		return client.PublishPhotoToInboxByURL(ctx, accessToken, wrappedURLs, draftOpts)
	}
	wrapped, err := s.WrapTikTokVerifiedMediaURL(rawURLs[0])
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
	if itemStringValue(mediaItems[0], "url") == "" {
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
	if !directPost {
		pub, err = s.uploadTikTokDraft(ctx, client, accessToken, in, mediaItems)
	} else {
		postOpts, optsErr := resolveTikTokPostOptions(in.TikTokOptions)
		if optsErr != nil {
			return nil, optsErr
		}
		isPhoto, rawURLs, mediaErr := classifyTikTokMediaItems(in.MediaItems, mediaItems, s.cfg.MaxMediaItems)
		if mediaErr != nil {
			return nil, mediaErr
		}
		title, extraErr := applyTikTokDirectExtraOptions(postOpts, in, isPhoto, len(rawURLs))
		if extraErr != nil {
			return nil, extraErr
		}
		// Capability check per TikTok guidelines (best-effort; failure does not block v1).
		_, _ = client.QueryCreatorInfo(ctx, accessToken)
		if isPhoto {
			urls := make([]string, 0, len(rawURLs))
			for _, uu := range rawURLs {
				wrapped, werr := s.WrapTikTokVerifiedMediaURL(uu)
				if werr != nil {
					return nil, werr
				}
				urls = append(urls, wrapped)
			}
			pub, err = client.PublishPhotoByURLWithOptions(ctx, accessToken, title, urls, postOpts)
		} else {
			wrapped, werr := s.WrapTikTokVerifiedMediaURL(rawURLs[0])
			if werr != nil {
				return nil, werr
			}
			pub, err = client.PublishVideoByURLWithOptions(ctx, accessToken, title, wrapped, postOpts)
		}
	}
	if err != nil {
		return nil, err
	}

	publishMode := "draft"
	if directPost {
		publishMode = "direct"
	}
	previewText := in.Content
	if previewText == "" && in.TikTokOptions != nil {
		previewText = firstNonEmpty(in.TikTokOptions.Title, in.TikTokOptions.Description)
	}
	if previewText == "" {
		previewText = firstNonEmpty(in.Title, in.Description)
	}
	reqSnap, _ := common.Marshal(map[string]any{
		"content_preview": truncate(previewText, 200),
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
		ContentPreview:  truncate(previewText, 200),
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
	if mapped != "" {
		publishID := strings.TrimSpace(st.PublishID)
		if publishID == "" {
			publishID = p.ProviderPostID
		}
		// fork: tiktok-webhooks — share the locked monotonic transition guard with webhooks
		// so a slow poll can never regress terminal status or clobber webhook post_ids.
		_, _, _ = model.ApplySocialPostStateUpdateByID(p.ID, model.SocialPostStateUpdate{
			TargetStatus: mapped,
			SnapshotFields: map[string]any{
				"status":      st.Status,
				"fail_reason": st.FailReason,
				"publish_id":  publishID,
			},
		})
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
	case "SEND_TO_USER_INBOX":
		return "sent_to_inbox"
	case "PROCESSING_DOWNLOAD", "PROCESSING_UPLOAD", "PUBLISH_INITIATED":
		return "processing"
	default:
		return ""
	}
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
