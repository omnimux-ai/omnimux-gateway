package social

// fork: tiktok-direct — official TikTok Login Kit + Content Posting path.
// Calls must explicitly request tiktok_direct; unavailable direct service never falls back.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social/tiktok"
	"gorm.io/gorm"
)

var ErrTikTokDirectNotConfigured = errors.New("tiktok direct is not configured")

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

// connectTikTokDirect starts official OAuth: persists one-time state and returns authorize URL.
func (s *Service) connectTikTokDirect(ctx context.Context, userId int, redirectAfter string) (string, error) {
	if err := s.tiktokDirectUsable(); err != nil {
		return "", err
	}
	// Seat preflight parity with Zernio path (single billing story).
	if err := s.PreflightConnect(userId); err != nil {
		return "", err
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	redirectAfter = strings.TrimSpace(redirectAfter)
	if redirectAfter != "" {
		defaultRedirect := strings.TrimSpace(s.cfg.DefaultRedirectURL)
		if defaultRedirect == "" {
			defaultRedirect = s.cfg.TikTokRedirectURI
		}
		if err := validateRedirectURL(redirectAfter, defaultRedirect); err != nil {
			return "", err
		}
	}
	if err := model.CreateSocialOAuthState(&model.SocialOAuthState{
		State:         state,
		UserId:        userId,
		Provider:      ProviderTikTokDirect,
		RedirectAfter: redirectAfter,
	}); err != nil {
		return "", err
	}
	key, _ := s.cfg.TikTokCredentials()
	return tiktok.AuthorizeURL(s.cfg.TikTokAuthorizeURL, key, s.cfg.TikTokRedirectURI, s.cfg.TikTokScopes, state), nil
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
	scopes := tok.Scope
	if scopes == "" {
		scopes = s.cfg.TikTokScopes
	}
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
	return &TikTokCallbackResult{Account: saved, RedirectAfter: st.RedirectAfter}, nil
}

// accessTokenForAccount decrypts (and refreshes if expired) the account's access token.
func (s *Service) accessTokenForAccount(ctx context.Context, acc *model.SocialAccount) (string, error) {
	if acc == nil || acc.AccessTokenEncrypted == "" {
		return "", ErrTikTokDirectNotConfigured
	}
	plain, err := OpenToken(acc.AccessTokenEncrypted, s.cfg.SocialTokenEncKey)
	if err != nil {
		return "", err
	}
	// Refresh if within 10 minutes of expiry.
	if acc.TokenExpiresAt != nil && time.Now().UTC().Add(10*time.Minute).After(*acc.TokenExpiresAt) && acc.RefreshTokenEncrypted != "" {
		rt, err := OpenToken(acc.RefreshTokenEncrypted, s.cfg.SocialTokenEncKey)
		if err != nil {
			return plain, nil // fall back to stale access token; upstream will 401 and caller handles
		}
		key, secret := s.cfg.TikTokCredentials()
		tok, err := s.tiktokClient().RefreshToken(ctx, key, secret, rt)
		if err != nil {
			return plain, nil
		}
		accessEnc, e1 := SealToken(tok.AccessToken, s.cfg.SocialTokenEncKey)
		refreshEnc, e2 := SealToken(firstNonEmpty(tok.RefreshToken, rt), s.cfg.SocialTokenEncKey)
		if e1 == nil && e2 == nil {
			updates := map[string]any{
				"access_token_encrypted":  accessEnc,
				"refresh_token_encrypted": refreshEnc,
				"updated_at":              time.Now().UTC(),
			}
			if tok.ExpiresIn > 0 {
				t := time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second)
				updates["token_expires_at"] = t
			}
			_ = model.DB.Model(&model.SocialAccount{}).Where("id = ?", acc.ID).Updates(updates).Error
			return tok.AccessToken, nil
		}
	}
	return plain, nil
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// GetTikTokCreatorInfo queries live TikTok creator publishing settings for an owned direct account.
func (s *Service) GetTikTokCreatorInfo(ctx context.Context, userId int, provider string, localAccountID int64) (*tiktok.CreatorInfo, error) {
	if err := validateProvider(provider); err != nil {
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
	postOpts, err := resolveTikTokPostOptions(in.TikTokOptions)
	if err != nil {
		return nil, err
	}
	accessToken, err := s.accessTokenForAccount(ctx, acc)
	if err != nil {
		return nil, err
	}
	client := s.tiktokClient()
	// Capability check per TikTok guidelines (best-effort; failure does not block v1).
	_, _ = client.QueryCreatorInfo(ctx, accessToken)

	var pub *tiktok.PublishInitResponse
	title := in.Content
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
	if err != nil {
		return nil, err
	}

	reqSnap, _ := common.Marshal(map[string]any{
		"content_preview": truncate(in.Content, 200),
		"account_ids":     in.AccountIDs,
		"publish_now":     in.PublishNow,
		"scheduled_for":   in.ScheduledFor,
		"media_count":     len(mediaItems),
		"provider":        ProviderTikTokDirect,
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
