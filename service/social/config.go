package social

import (
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	ProviderZernio       = "zernio"
	ProviderTikTokDirect = "tiktok_direct"
	defaultBaseURL       = "https://zernio.com/api/v1"
	// fork: tiktok-direct — official TikTok Login Kit + Content Posting OAuth.
	// Authorize host: www.tiktok.com; API host: open.tiktokapis.com.
	defaultTikTokBaseURL      = "https://open.tiktokapis.com"
	defaultTikTokAuthorizeURL = "https://www.tiktok.com/v2/auth/authorize/"
	defaultTikTokRedirectURI  = "https://omnimux.ai/api/social/v1/oauth/tiktok/callback"
	defaultTikTokScopes       = "user.info.basic,video.publish,video.upload"
)

// Config is Social Ops runtime configuration (env only; default off).
type Config struct {
	Enabled            bool
	APIKey             string
	BaseURL            string
	Platforms          []string
	WebhookSecret      string
	DefaultRedirectURL string
	// BillingMode from env is legacy ("off"|"fixed"). Seat billing is Option
	// social_ops_setting.billing_mode — env SOCIAL_OPS_BILLING_MODE is not the seat switch.
	// fork: social-seats
	BillingMode string
	// FixedPostQuota raw quota charged when BillingMode == "fixed" (optional, deprecated).
	FixedPostQuota int
	// MaxMediaItems caps media_items on a single post.
	MaxMediaItems int

	// --- TikTok official direct OAuth (fork: tiktok-direct) ---
	// TikTokDirectEnabled switches TikTok Connect/Post/Disconnect from Zernio to the
	// official OmniMux TikTok App (Login Kit + Content Posting API). Default false =
	// current Zernio behavior. Zernio code is kept for rollback.
	TikTokDirectEnabled bool
	// RollbackTikTokToZernio forces TikTok back to Zernio even when direct is enabled.
	RollbackTikTokToZernio bool
	// TikTokClientKey / TikTokClientSecret are the official app credentials (server-only).
	TikTokClientKey    string
	TikTokClientSecret string
	// Sandbox pair takes precedence over the production pair when both are set.
	TikTokSandboxClientKey    string
	TikTokSandboxClientSecret string
	// TikTokRedirectURI must match the TikTok Developer console exactly (protocol/host/path).
	TikTokRedirectURI string
	// TikTokScopes must match the console-granted scopes.
	TikTokScopes string
	// Overridable for tests.
	TikTokBaseURL      string
	TikTokAuthorizeURL string
	// SocialTokenEncKey is a base64 32B AES-GCM key for encrypting direct OAuth tokens
	// at rest. Empty means direct account creation fails closed (no plaintext tokens).
	SocialTokenEncKey string

	// fork: social-media-own-storage — direct-post media tickets come from the
	// gateway's own object storage instead of the Zernio bucket.
	// MediaStorageMaxBytes caps one social media object (SOCIAL_MEDIA_MAX_BYTES).
	MediaStorageMaxBytes int64
	// MediaStoragePresignTTL is the lifetime of one upload/read ticket.
	MediaStoragePresignTTL time.Duration
}

const (
	defaultSocialMediaMaxBytes   = 512 << 20 // 512 MiB
	defaultSocialMediaPresignTTL = 15 * time.Minute
)

func LoadConfig() Config {
	cfg := Config{
		Enabled:            common.GetEnvOrDefaultBool("SOCIAL_OPS_ENABLED", false),
		APIKey:             strings.TrimSpace(os.Getenv("SOCIAL_ZERNIO_API_KEY")),
		BaseURL:            strings.TrimRight(common.GetEnvOrDefaultString("SOCIAL_ZERNIO_BASE_URL", defaultBaseURL), "/"),
		WebhookSecret:      strings.TrimSpace(os.Getenv("SOCIAL_OPS_WEBHOOK_SECRET")),
		DefaultRedirectURL: strings.TrimSpace(os.Getenv("SOCIAL_OPS_DEFAULT_REDIRECT_URL")),
		BillingMode:        strings.ToLower(strings.TrimSpace(common.GetEnvOrDefaultString("SOCIAL_OPS_BILLING_MODE", "off"))),
		FixedPostQuota:     common.GetEnvOrDefault("SOCIAL_OPS_POST_QUOTA", 0),
		MaxMediaItems:      common.GetEnvOrDefault("SOCIAL_OPS_MAX_MEDIA_ITEMS", 10),
	}
	raw := strings.TrimSpace(os.Getenv("SOCIAL_OPS_PLATFORMS"))
	if raw == "" {
		// Conservative P0 default; expand after OAuth smoke per platform.
		cfg.Platforms = []string{"twitter", "linkedin", "instagram", "tiktok", "youtube", "facebook", "threads", "bluesky"}
	} else {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(strings.ToLower(p))
			if p != "" {
				cfg.Platforms = append(cfg.Platforms, p)
			}
		}
	}
	if cfg.BillingMode != "fixed" {
		cfg.BillingMode = "off"
	}
	if cfg.MaxMediaItems <= 0 {
		cfg.MaxMediaItems = 10
	}
	// fork: tiktok-direct
	cfg.TikTokDirectEnabled = common.GetEnvOrDefaultBool("TIKTOK_DIRECT_ENABLED", false)
	cfg.RollbackTikTokToZernio = common.GetEnvOrDefaultBool("ROLLBACK_TIKTOK_TO_ZERNIO", false)
	cfg.TikTokClientKey = strings.TrimSpace(os.Getenv("TIKTOK_CLIENT_KEY"))
	cfg.TikTokClientSecret = strings.TrimSpace(os.Getenv("TIKTOK_CLIENT_SECRET"))
	cfg.TikTokSandboxClientKey = strings.TrimSpace(os.Getenv("TIKTOK_SANDBOX_CLIENT_KEY"))
	cfg.TikTokSandboxClientSecret = strings.TrimSpace(os.Getenv("TIKTOK_SANDBOX_CLIENT_SECRET"))
	cfg.TikTokRedirectURI = strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_REDIRECT_URI", defaultTikTokRedirectURI))
	cfg.TikTokScopes = strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_SCOPES", defaultTikTokScopes))
	cfg.TikTokBaseURL = strings.TrimRight(common.GetEnvOrDefaultString("TIKTOK_BASE_URL", defaultTikTokBaseURL), "/")
	cfg.TikTokAuthorizeURL = strings.TrimSpace(common.GetEnvOrDefaultString("TIKTOK_AUTHORIZE_URL", defaultTikTokAuthorizeURL))
	cfg.SocialTokenEncKey = strings.TrimSpace(os.Getenv("SOCIAL_TOKEN_ENC_KEY"))
	// fork: social-media-own-storage
	cfg.MediaStorageMaxBytes = int64(common.GetEnvOrDefault("SOCIAL_MEDIA_MAX_BYTES", defaultSocialMediaMaxBytes))
	if cfg.MediaStorageMaxBytes <= 0 {
		cfg.MediaStorageMaxBytes = defaultSocialMediaMaxBytes
	}
	cfg.MediaStoragePresignTTL = time.Duration(common.GetEnvOrDefault("SOCIAL_MEDIA_PRESIGN_TTL_SECONDS", int(defaultSocialMediaPresignTTL/time.Second))) * time.Second
	if cfg.MediaStoragePresignTTL <= 0 {
		cfg.MediaStoragePresignTTL = defaultSocialMediaPresignTTL
	}
	return cfg
}

func (c Config) Ready() bool {
	return c.Enabled && c.APIKey != "" && c.BaseURL != ""
}

func (c Config) PlatformAllowed(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return false
	}
	// Alias: X → twitter (Zernio uses twitter)
	if platform == "x" {
		platform = "twitter"
	}
	for _, p := range c.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

func NormalizePlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "x" {
		return "twitter"
	}
	return platform
}

// TikTokDirectActive reports whether TikTok traffic should use the official direct path.
func (c Config) TikTokDirectActive() bool {
	return c.TikTokDirectEnabled && !c.RollbackTikTokToZernio
}

// TikTokCredentials returns the effective client key/secret (sandbox pair wins when set).
func (c Config) TikTokCredentials() (key, secret string) {
	key, secret = c.TikTokClientKey, c.TikTokClientSecret
	if c.TikTokSandboxClientKey != "" && c.TikTokSandboxClientSecret != "" {
		return c.TikTokSandboxClientKey, c.TikTokSandboxClientSecret
	}
	return key, secret
}

// TikTokDirectConfigured reports whether direct path has usable credentials + redirect.
func (c Config) TikTokDirectConfigured() bool {
	key, secret := c.TikTokCredentials()
	return key != "" && secret != "" && c.TikTokRedirectURI != ""
}
