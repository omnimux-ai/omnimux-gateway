package social

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social/zernio"
	"gorm.io/gorm"
)

var (
	ErrDisabled       = errors.New("social ops is disabled")
	ErrNotConfigured  = errors.New("social ops upstream is not configured")
	ErrPlatformDenied = errors.New("platform is not allowed")
	ErrAccountDenied  = errors.New("one or more accounts are not owned by this user")
	ErrAccountMissing = errors.New("account not found")
	ErrRedirectDenied = errors.New("redirect_url host is not allowed")
	ErrMediaInvalid   = errors.New("invalid media request")
	// fork: social-media-own-storage — the own bucket cannot issue an upload ticket.
	ErrMediaStorageUnavailable = errors.New("social media storage is not configured")

	// Seat errors are aliases so controllers can errors.Is against this package.
	// fork: social-seats
)

// Service is the Social Ops application service (tenant mapping + Zernio proxy).
type Service struct {
	cfg    Config
	client *zernio.Client
}

func NewService(cfg Config) *Service {
	s := &Service{cfg: cfg}
	if cfg.APIKey != "" {
		s.client = zernio.NewClient(cfg.BaseURL, cfg.APIKey)
	}
	return s
}

func DefaultService() *Service {
	return NewService(LoadConfig())
}

func (s *Service) Config() Config {
	return s.cfg
}

func (s *Service) requireReady() error {
	if !s.cfg.Enabled {
		return ErrDisabled
	}
	if s.client == nil || s.cfg.APIKey == "" {
		return ErrNotConfigured
	}
	return nil
}

// Status is safe for unauthenticated feature probes when enabled checks are done by caller.
func (s *Service) Status() map[string]any {
	out := map[string]any{
		"enabled":            s.cfg.Enabled,
		"upstream_ready":     s.cfg.Ready(),
		"provider":           ProviderZernio,
		"platforms":          s.cfg.Platforms,
		"billing_mode":       s.cfg.BillingMode,
		"webhook_configured": s.cfg.WebhookSecret != "",
	}
	// fork: tiktok-direct — feature probe without any secret material.
	out["tiktok_direct"] = map[string]any{
		"enabled":    s.cfg.TikTokDirectEnabled,
		"active":     s.cfg.TikTokDirectActive(),
		"configured": s.cfg.TikTokDirectConfigured(),
	}
	// fork: social-seats — 未开 Social 不暴露席位价目
	if s.cfg.Enabled {
		for k, v := range publicSeatCatalog() {
			out[k] = v
		}
	}
	return out
}

// EnsureTenant creates or returns the Zernio profile mapping for the user.
func (s *Service) EnsureTenant(ctx context.Context, userId int) (*model.SocialTenant, error) {
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	if userId <= 0 {
		return nil, errors.New("invalid user")
	}
	existing, err := model.GetSocialTenant(userId, ProviderZernio)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	name := fmt.Sprintf("omnimux_u_%d", userId)
	profile, err := s.client.CreateProfile(ctx, name, fmt.Sprintf("OmniMux user %d", userId))
	if err != nil {
		return nil, err
	}
	return model.GetOrCreateSocialTenant(userId, ProviderZernio, profile.ID)
}

// Connect starts OAuth only with the explicitly requested provider.
// fork: social-provider-isolation
func (s *Service) Connect(ctx context.Context, userId int, provider, platform, redirectURL string) (authURL string, err error) {
	if err := validateProvider(provider); err != nil {
		return "", err
	}
	platform = NormalizePlatform(platform)
	if provider == ProviderTikTokDirect {
		if platform != "tiktok" || !s.cfg.PlatformAllowed(platform) {
			return "", ErrPlatformDenied
		}
		if !s.cfg.TikTokDirectConfigured() {
			return "", ErrNotConfigured
		}
		return s.connectTikTokDirect(ctx, userId, redirectURL)
	}
	if err := s.requireReady(); err != nil {
		return "", err
	}
	if !s.cfg.PlatformAllowed(platform) {
		return "", ErrPlatformDenied
	}
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		redirectURL = s.cfg.DefaultRedirectURL
	}
	if redirectURL != "" {
		if err := validateRedirectURL(redirectURL, s.cfg.DefaultRedirectURL); err != nil {
			return "", err
		}
	}
	if err := s.PreflightConnect(userId); err != nil {
		return "", err
	}
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return "", err
	}
	res, err := s.client.GetConnectURL(ctx, platform, tenant.ProviderProfileID, redirectURL)
	if err != nil {
		return "", err
	}
	return res.AuthURL, nil
}

// ListAccounts never syncs or returns accounts from another provider.
// fork: social-provider-isolation
func (s *Service) ListAccounts(ctx context.Context, userId int, provider string) ([]*model.SocialAccount, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	if provider == ProviderTikTokDirect {
		if err := s.tiktokDirectUsable(); err != nil {
			return nil, err
		}
		return model.ListSocialAccountsByUser(userId, ProviderTikTokDirect)
	}
	return s.listAccountsZernio(ctx, userId)
}

// listAccountsZernio synchronizes only the user's Zernio accounts.
func (s *Service) listAccountsZernio(ctx context.Context, userId int) ([]*model.SocialAccount, error) {
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	upstream, err := s.client.ListAccounts(ctx, tenant.ProviderProfileID)
	if err != nil {
		// Still return local cache if upstream fails after prior sync
		local, lerr := model.ListSocialAccountsByUser(userId, ProviderZernio)
		if lerr == nil && len(local) > 0 {
			return local, nil
		}
		return nil, err
	}
	if seatBillingOn() {
		_ = s.lazyExpireAndRenew(ctx, userId)
		_ = s.grandfatherActiveAccounts(userId)
	}
	var firstErr error
	for _, a := range upstream {
		if a.ID == "" {
			continue
		}
		if _, ierr := s.ingestUpstreamAccount(ctx, userId, tenant, a); ierr != nil {
			if firstErr == nil && (errors.Is(ierr, ErrSeatQuota) || errors.Is(ierr, ErrSeatCap) || errors.Is(ierr, ErrSeatExpired)) {
				firstErr = ierr
			}
		}
	}
	rows, lerr := model.ListSocialAccountsByUser(userId, ProviderZernio)
	if lerr != nil {
		return nil, lerr
	}
	if firstErr != nil && len(rows) == 0 {
		return nil, firstErr
	}
	return rows, nil
}

// DisconnectAccount disconnects upstream and marks local row disconnected.
// fork: tiktok-direct — direct accounts revoke official token best-effort.
func (s *Service) DisconnectAccount(ctx context.Context, userId int, provider string, localAccountID int64) error {
	if err := validateProvider(provider); err != nil {
		return err
	}
	acc, err := model.GetSocialAccountByIDForUser(localAccountID, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAccountMissing
		}
		return err
	}
	if err := validateAccountProvider(provider, []*model.SocialAccount{acc}); err != nil {
		return err
	}
	if provider == ProviderTikTokDirect {
		if !s.cfg.Enabled {
			return ErrDisabled
		}
		return s.disconnectTikTokDirect(ctx, userId, acc)
	}
	if err := s.requireReady(); err != nil {
		return err
	}
	if err := s.client.DisconnectAccount(ctx, acc.ProviderAccountID); err != nil {
		// If already gone upstream, still mark local disconnected.
		var apiErr *zernio.APIError
		if !(errors.As(err, &apiErr) && (apiErr.Status == 404 || apiErr.Status == 410)) {
			return err
		}
	}
	if err := model.MarkSocialAccountDisconnected(localAccountID, userId); err != nil {
		return err
	}
	return s.vacateAfterDisconnect(userId, localAccountID)
}

// AccountGuard ensures every local account id is active and owned by userId.
// Returns the resolved accounts in the same order as ids when possible.
func AccountGuard(userId int, localIDs []int64) ([]*model.SocialAccount, error) {
	if len(localIDs) == 0 {
		return nil, ErrAccountDenied
	}
	// Dedup while preserving request set
	seen := make(map[int64]struct{}, len(localIDs))
	uniq := make([]int64, 0, len(localIDs))
	for _, id := range localIDs {
		if id <= 0 {
			return nil, ErrAccountDenied
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	rows, err := model.GetOwnedSocialAccountsByIDs(userId, uniq)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(uniq) {
		return nil, ErrAccountDenied
	}
	return rows, nil
}

// TikTokPostOptions carries optional Content Sharing Guidelines metadata for a TikTok Direct post.
type TikTokPostOptions struct {
	PrivacyLevel        string `json:"privacy_level,omitempty"`
	AllowComment        *bool  `json:"allow_comment,omitempty"`
	AllowDuet           *bool  `json:"allow_duet,omitempty"`
	AllowStitch         *bool  `json:"allow_stitch,omitempty"`
	CommercialContent   *bool  `json:"commercial_content,omitempty"`
	YourBrand           *bool  `json:"your_brand,omitempty"`
	BrandedContent      *bool  `json:"branded_content,omitempty"`
	MusicUsageConfirmed *bool  `json:"music_usage_confirmed,omitempty"`
}

// CreatePostInput is the user-facing create-post body.
type CreatePostInput struct {
	Provider       string
	Content        string
	AccountIDs     []int64
	PublishNow     bool
	ScheduledFor   string
	Timezone       string
	MediaItems     []map[string]any
	IdempotencyKey string
	Queued         bool
	TikTokOptions  *TikTokPostOptions
}

// CreatePost publishes or schedules a post for the user's owned accounts only.
// fork: social-provider-isolation — provider and owned account set are checked before replay or I/O.
func (s *Service) CreatePost(ctx context.Context, userId int, in CreatePostInput) (*model.SocialPost, error) {
	if err := validateProvider(in.Provider); err != nil {
		return nil, err
	}
	in.Content = strings.TrimSpace(in.Content)
	mediaItems, err := NormalizeMediaItems(in.MediaItems, s.cfg.MaxMediaItems)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMediaInvalid, err)
	}
	if in.Content == "" && len(mediaItems) == 0 {
		return nil, errors.New("content or media is required")
	}
	accounts, err := AccountGuard(userId, in.AccountIDs)
	if err != nil {
		return nil, err
	}
	if err := validateAccountProvider(in.Provider, accounts); err != nil {
		return nil, err
	}
	if in.IdempotencyKey != "" {
		if existing, err := model.GetSocialPostByIdempotency(userId, in.IdempotencyKey); err == nil {
			if err := validateIdempotentPost(existing, in); err != nil {
				return nil, err
			}
			return existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if err := s.guardSeatsForAccounts(userId, accounts); err != nil {
		return nil, err
	}
	if in.Provider == ProviderTikTokDirect {
		if !s.cfg.TikTokDirectActive() {
			return nil, ErrTikTokDirectNotConfigured
		}
		if len(accounts) > 1 {
			return nil, fmt.Errorf("%w: tiktok direct v1 supports one account per post", ErrMediaInvalid)
		}
		if err := s.requireEnabledOnly(); err != nil {
			return nil, err
		}
		return s.createPostTikTokDirect(ctx, userId, accounts[0], in, mediaItems)
	}
	return s.createPostZernio(ctx, userId, in, mediaItems, accounts)
}

// requireEnabledOnly gates non-zernio paths on the feature flag only (no zernio key needed).
func (s *Service) requireEnabledOnly() error {
	if !s.cfg.Enabled {
		return ErrDisabled
	}
	return nil
}

// createPostZernio submits the validated Zernio account set.
func (s *Service) createPostZernio(ctx context.Context, userId int, in CreatePostInput, mediaItems []map[string]any, accounts []*model.SocialAccount) (*model.SocialPost, error) {
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}

	targets := make([]zernio.PostPlatformTarget, 0, len(accounts))
	for _, a := range accounts {
		targets = append(targets, zernio.PostPlatformTarget{
			Platform:  a.Platform,
			AccountID: a.ProviderAccountID,
		})
	}

	req := zernio.CreatePostRequest{
		Content:   in.Content,
		Platforms: targets,
	}
	if in.Queued {
		req.QueuedFromProfile = tenant.ProviderProfileID
	} else if in.ScheduledFor != "" {
		req.ScheduledFor = in.ScheduledFor
		req.Timezone = in.Timezone
	} else {
		req.PublishNow = true
		if in.PublishNow {
			req.PublishNow = true
		}
	}
	if len(mediaItems) > 0 {
		req.MediaItems = mediaItems
	}

	reqSnap, _ := common.Marshal(map[string]any{
		"content_preview": truncate(in.Content, 200),
		"account_ids":     in.AccountIDs,
		"publish_now":     req.PublishNow,
		"scheduled_for":   req.ScheduledFor,
		"queued":          in.Queued,
		"media_count":     len(mediaItems),
	})

	post, raw, err := s.client.CreatePost(ctx, req, in.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	status := post.Status
	if status == "" {
		status = "submitted"
	}
	ledger := &model.SocialPost{
		UserId:          userId,
		Provider:        ProviderZernio,
		ProviderPostID:  post.ID,
		IdempotencyKey:  in.IdempotencyKey,
		Status:          status,
		ContentPreview:  truncate(in.Content, 200),
		RequestSnapshot: string(reqSnap),
		ResultSnapshot:  string(raw),
	}
	if err := model.CreateSocialPost(ledger); err != nil {
		// Upstream already accepted; return a synthetic view if local insert races on idempotency
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

// GetPost returns a post owned by the user; optionally refreshes from upstream.
// fork: tiktok-direct — direct posts refresh via official publish status API.
func (s *Service) GetPost(ctx context.Context, userId int, provider string, postID int64, refresh bool) (*model.SocialPost, error) {
	if err := validateProvider(provider); err != nil {
		return nil, err
	}
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	p, err := model.GetSocialPostByIDForUser(postID, userId)
	if err != nil {
		return nil, err
	}
	if p.Provider != provider {
		return nil, ErrPostProviderMismatch
	}
	if provider == ProviderTikTokDirect {
		if refresh && !isTerminalStatus(p.Status) && p.ProviderPostID != "" {
			s.refreshTikTokPostStatus(ctx, p)
			return model.GetSocialPostByIDForUser(postID, userId)
		}
		return p, nil
	}
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	if !refresh || p.ProviderPostID == "" || s.client == nil {
		return p, nil
	}
	if isTerminalStatus(p.Status) {
		return p, nil
	}
	up, raw, err := s.client.GetPost(ctx, p.ProviderPostID)
	if err != nil {
		return p, nil // return cached
	}
	status := up.Status
	if status == "" {
		status = p.Status
	}
	_ = model.UpdateSocialPostResult(p.ID, status, up.ID, string(raw))
	return model.GetSocialPostByIDForUser(postID, userId)
}

// PresignMedia issues one upload ticket for a social post's media.
// Client must PUT the file to upload_url, then pass public_url in posts.media_items.
// fork: social-media-own-storage — the official TikTok direct path signs against the
// gateway's own bucket so a Zernio outage cannot block direct publishing.
func (s *Service) PresignMedia(ctx context.Context, userId int, body map[string]any) (map[string]any, error) {
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	if userId <= 0 {
		return nil, errors.New("invalid user")
	}
	in, err := NormalizePresignInput(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMediaInvalid, err)
	}
	if s.presignUsesOwnStorage(body) {
		return s.presignOwnMediaStorage(ctx, userId, in)
	}
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	if _, err := s.EnsureTenant(ctx, userId); err != nil {
		return nil, err
	}
	res, err := s.client.PresignMedia(ctx, ZernioPresignBody(in))
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"upload_url": res.UploadURL,
		"public_url": res.PublicURL,
		"headers": map[string]string{
			"Content-Type": in.ContentType,
		},
		"filename":     in.Filename,
		"content_type": in.ContentType,
	}
	if res.ExpiresIn > 0 {
		out["expires_in"] = res.ExpiresIn
	}
	if res.Key != "" {
		out["key"] = res.Key
	}
	return out, nil
}

func isTerminalStatus(status string) bool {
	switch strings.ToLower(status) {
	case "published", "failed", "partial", "deleted":
		return true
	default:
		return false
	}
}

func truncate(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// validateRedirectURL allows empty, exact default, or same host as default.
func validateRedirectURL(redirectURL, defaultURL string) error {
	u, err := url.Parse(redirectURL)
	if err != nil || u.Host == "" {
		return ErrRedirectDenied
	}
	if defaultURL == "" {
		if u.Scheme == "https" {
			return nil
		}
		if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
			return nil
		}
		return ErrRedirectDenied
	}
	d, err := url.Parse(defaultURL)
	if err != nil || d.Host == "" {
		return ErrRedirectDenied
	}
	if !strings.EqualFold(u.Hostname(), d.Hostname()) {
		return ErrRedirectDenied
	}
	return nil
}

// --- Analytics & Inbox Service Handlers ---

func (s *Service) validateAndMapAccountIDs(userId int, localIDs []int64) ([]string, error) {
	if len(localIDs) == 0 {
		return nil, nil
	}
	accounts, err := model.GetOwnedSocialAccountsByIDs(userId, localIDs)
	if err != nil || len(accounts) != len(localIDs) {
		return nil, ErrAccountDenied
	}
	out := make([]string, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, a.ProviderAccountID)
	}
	return out, nil
}

func (s *Service) validateAndMapSingleAccountID(userId int, localID int64) (string, error) {
	if localID <= 0 {
		return "", nil
	}
	a, err := model.GetSocialAccountByIDForUser(localID, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrAccountDenied
		}
		return "", err
	}
	return a.ProviderAccountID, nil
}

func unmarshalJSONResponse(raw []byte) (any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) GetDailyMetrics(ctx context.Context, userId int, fromDate, toDate, platform, timezone string, localAccountIDs []int64) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	providerAccountIDs, err := s.validateAndMapAccountIDs(userId, localAccountIDs)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetDailyMetrics(ctx, zernio.AnalyticsQuery{
		ProfileID:  tenant.ProviderProfileID,
		AccountIDs: providerAccountIDs,
		Platform:   platform,
		FromDate:   fromDate,
		ToDate:     toDate,
		Timezone:   timezone,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetBestTimeToPost(ctx context.Context, userId int, platform string, localAccountID int64, timezone string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	providerAccountID, err := s.validateAndMapSingleAccountID(userId, localAccountID)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetBestTimeToPost(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		AccountID: providerAccountID,
		Platform:  platform,
		Timezone:  timezone,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetPostingFrequency(ctx context.Context, userId int, platform string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetPostingFrequency(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetContentDecay(ctx context.Context, userId int, platform string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetContentDecay(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetFollowerStats(ctx context.Context, userId int, days int, localAccountIDs []int64) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	providerAccountIDs, err := s.validateAndMapAccountIDs(userId, localAccountIDs)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetFollowerStats(ctx, zernio.AnalyticsQuery{
		ProfileID:  tenant.ProviderProfileID,
		AccountIDs: providerAccountIDs,
		Days:       days,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetPostsAnalytics(ctx context.Context, userId int, postID string, platform, sortBy, sortOrder string, page, limit int) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetPostsAnalytics(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		PostID:    postID,
		Platform:  platform,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) SyncExternalPosts(ctx context.Context, userId int, localAccountID int64, postURL string) (any, error) {
	if localAccountID <= 0 {
		return nil, errors.New("account_id is required")
	}
	if _, err := s.EnsureTenant(ctx, userId); err != nil {
		return nil, err
	}
	providerAccountID, err := s.validateAndMapSingleAccountID(userId, localAccountID)
	if err != nil {
		return nil, err
	}
	if providerAccountID == "" {
		return nil, ErrAccountDenied
	}
	raw, err := s.client.SyncExternalPosts(ctx, providerAccountID, postURL)
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetInboxVolume(ctx context.Context, userId int, platform, fromDate, toDate string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetInboxVolume(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
		FromDate:  fromDate,
		ToDate:    toDate,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetInboxResponseTime(ctx context.Context, userId int, platform, fromDate, toDate string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetInboxResponseTime(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
		FromDate:  fromDate,
		ToDate:    toDate,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetInboxHeatmap(ctx context.Context, userId int, platform, timezone string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetInboxHeatmap(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
		Timezone:  timezone,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}

func (s *Service) GetInboxSourceBreakdown(ctx context.Context, userId int, platform, fromDate, toDate string) (any, error) {
	tenant, err := s.EnsureTenant(ctx, userId)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.GetInboxSourceBreakdown(ctx, zernio.AnalyticsQuery{
		ProfileID: tenant.ProviderProfileID,
		Platform:  platform,
		FromDate:  fromDate,
		ToDate:    toDate,
	})
	if err != nil {
		return nil, err
	}
	return unmarshalJSONResponse(raw)
}
