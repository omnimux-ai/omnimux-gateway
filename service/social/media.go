package social

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/storage"
)

const (
	tiktokPublicMediaPathPrefix = "/api/social/v1/public/media/"
	tiktokPublicMediaTokenTTL   = 2 * time.Hour
)

type tiktokMediaPullTokenPayload struct {
	URL       string `json:"u"`
	ExpiresAt int64  `json:"e"`
}

func isLoopbackOrPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func isLoopbackOrPrivateLiteralIP(host string) bool {
	return isLoopbackOrPrivateIP(net.ParseIP(strings.TrimSpace(host)))
}

// legacyIPv4Host handles non-canonical inet_aton forms (127.1, 2130706433)
// without depending on the host resolver; Go's LookupIP behavior for these
// aliases varies by platform.
func legacyIPv4Host(host string) net.IP {
	if host == "" {
		return nil
	}
	if strings.Trim(host, "0123456789") == "" {
		value, err := strconv.ParseUint(host, 10, 32)
		if err != nil {
			return nil
		}
		return net.IPv4(byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
	}
	parts := strings.Split(host, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil
	}
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return nil
		}
		value, err := strconv.ParseInt(part, 10, 32)
		if err != nil || value < 0 || value > 255 {
			return nil
		}
		values = append(values, value)
	}
	switch len(values) {
	case 2:
		return net.IPv4(byte(values[0]), 0, 0, byte(values[1]))
	case 3:
		return net.IPv4(byte(values[0]), byte(values[1]), 0, byte(values[2]))
	case 4:
		return net.IPv4(byte(values[0]), byte(values[1]), byte(values[2]), byte(values[3]))
	default:
		return nil
	}
}

func validatePublicMediaSourceURL(rawURL string, allowLoopbackForTest bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("%w: invalid media url", ErrMediaInvalid)
	}
	scheme := strings.ToLower(u.Scheme)
	hostOnly := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if hostOnly == "" {
		return nil, fmt.Errorf("%w: invalid media host", ErrMediaInvalid)
	}
	if scheme == "http" {
		if !allowLoopbackForTest || (hostOnly != "127.0.0.1" && hostOnly != "localhost") {
			return nil, fmt.Errorf("%w: media url must use https", ErrMediaInvalid)
		}
		return u, nil
	}
	if scheme != "https" {
		return nil, fmt.Errorf("%w: media url must use https", ErrMediaInvalid)
	}
	if hostOnly == "localhost" || strings.HasSuffix(hostOnly, ".local") || strings.HasSuffix(hostOnly, ".internal") || isLoopbackOrPrivateLiteralIP(hostOnly) || isLoopbackOrPrivateIP(legacyIPv4Host(hostOnly)) {
		if !allowLoopbackForTest {
			return nil, fmt.Errorf("%w: media host is not allowed", ErrMediaInvalid)
		}
	}
	if !allowLoopbackForTest {
		if ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), hostOnly); err == nil {
			for _, address := range ips {
				if isLoopbackOrPrivateIP(address.IP) {
					return nil, fmt.Errorf("%w: media host is not allowed", ErrMediaInvalid)
				}
			}
		}
	}
	return u, nil
}

// SealPublicMediaPullToken seals an upstream media URL into a short-lived token for TikTok PULL_FROM_URL.
func (s *Service) SealPublicMediaPullToken(rawURL string, ttl time.Duration) (string, error) {
	allowLoopback := strings.HasPrefix(strings.ToLower(s.cfg.TikTokBaseURL), "http://127.0.0.1") || strings.HasPrefix(strings.ToLower(s.cfg.TikTokBaseURL), "http://localhost")
	u, err := validatePublicMediaSourceURL(rawURL, allowLoopback)
	if err != nil {
		return "", err
	}
	if ttl == 0 {
		ttl = tiktokPublicMediaTokenTTL
	}
	payload, err := common.Marshal(tiktokMediaPullTokenPayload{
		URL:       u.String(),
		ExpiresAt: time.Now().UTC().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	return SealToken(string(payload), s.cfg.SocialTokenEncKey)
}

// OpenPublicMediaPullToken verifies and decrypts a TikTok media pull token, returning the validated upstream URL.
func (s *Service) OpenPublicMediaPullToken(token string) (string, error) {
	plain, err := OpenToken(strings.TrimSpace(token), s.cfg.SocialTokenEncKey)
	if err != nil {
		return "", fmt.Errorf("%w: invalid media token", ErrMediaInvalid)
	}
	var payload tiktokMediaPullTokenPayload
	if err := common.Unmarshal([]byte(plain), &payload); err != nil {
		return "", fmt.Errorf("%w: malformed media token", ErrMediaInvalid)
	}
	if payload.ExpiresAt <= time.Now().UTC().Unix() {
		return "", fmt.Errorf("%w: media token expired", ErrMediaInvalid)
	}
	allowLoopback := strings.HasPrefix(strings.ToLower(s.cfg.TikTokBaseURL), "http://127.0.0.1") || strings.HasPrefix(strings.ToLower(s.cfg.TikTokBaseURL), "http://localhost")
	u, err := validatePublicMediaSourceURL(payload.URL, allowLoopback)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// WrapTikTokVerifiedMediaURL ensures a media URL sits under the verified redirect origin (e.g. https://omnimux.ai).
func (s *Service) WrapTikTokVerifiedMediaURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("%w: empty media url", ErrMediaInvalid)
	}
	redirectParsed, err := url.Parse(strings.TrimSpace(s.cfg.TikTokRedirectURI))
	if err != nil || redirectParsed == nil || redirectParsed.Host == "" || redirectParsed.Scheme == "" {
		return rawURL, nil
	}
	mediaParsed, err := url.Parse(rawURL)
	if err != nil || mediaParsed == nil || mediaParsed.Host == "" {
		return "", fmt.Errorf("%w: invalid media url", ErrMediaInvalid)
	}
	if strings.EqualFold(mediaParsed.Host, redirectParsed.Host) && strings.EqualFold(mediaParsed.Scheme, redirectParsed.Scheme) {
		return rawURL, nil
	}
	tok, err := s.SealPublicMediaPullToken(rawURL, tiktokPublicMediaTokenTTL)
	if err != nil {
		return "", err
	}
	origin := redirectParsed.Scheme + "://" + redirectParsed.Host
	return origin + tiktokPublicMediaPathPrefix + tok, nil
}

// Allowed Zernio contentType values (from upstream validation error).
var allowedContentTypes = map[string]string{
	"image/jpeg":      "image",
	"image/jpg":       "image",
	"image/png":       "image",
	"image/webp":      "image",
	"image/gif":       "image",
	"video/mp4":       "video",
	"video/mpeg":      "video",
	"video/quicktime": "video",
	"video/avi":       "video",
	"video/x-msvideo": "video",
	"video/webm":      "video",
	"video/x-m4v":     "video",
	"application/pdf": "document",
	"audio/mpeg":      "audio",
	"audio/mp4":       "audio",
	"audio/aac":       "audio",
	"audio/ogg":       "audio",
	"audio/wav":       "audio",
	"audio/x-wav":     "audio",
	"audio/webm":      "audio",
}

// PresignInput is the OmniMux-normalized media presign request.
type PresignInput struct {
	Filename    string
	ContentType string
	SizeBytes   int64
}

// NormalizePresignInput accepts OmniMux snake_case and legacy camelCase maps.
func NormalizePresignInput(body map[string]any) (PresignInput, error) {
	var in PresignInput
	if body == nil {
		return in, errors.New("filename and content_type are required")
	}
	in.Filename = firstString(body, "filename", "fileName", "file_name", "name")
	in.ContentType = firstString(body, "content_type", "contentType", "mimeType", "mime_type")
	in.SizeBytes = firstInt64(body, "size_bytes", "size", "fileSize", "file_size")

	in.Filename = path.Base(strings.TrimSpace(in.Filename))
	in.ContentType = strings.ToLower(strings.TrimSpace(in.ContentType))
	if in.Filename == "" || in.Filename == "." || in.Filename == "/" {
		return in, errors.New("filename is required")
	}
	if in.ContentType == "" {
		return in, errors.New("content_type is required")
	}
	if _, ok := allowedContentTypes[in.ContentType]; !ok {
		return in, fmt.Errorf("unsupported content_type: %s", in.ContentType)
	}
	return in, nil
}

// ZernioPresignBody maps OmniMux input to upstream POST /media/presign body.
func ZernioPresignBody(in PresignInput) map[string]any {
	return map[string]any{
		"filename":    in.Filename,
		"contentType": in.ContentType,
	}
}

// NormalizeMediaItems converts client media_items into Zernio mediaItems [{url,type?}].
func NormalizeMediaItems(raw []map[string]any, maxItems int) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if maxItems > 0 && len(raw) > maxItems {
		return nil, fmt.Errorf("too many media_items: max %d", maxItems)
	}
	out := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		if item == nil {
			return nil, fmt.Errorf("media_items[%d] is empty", i)
		}
		u := firstString(item, "url", "public_url", "publicUrl", "mediaUrl", "media_url")
		u = strings.TrimSpace(u)
		if u == "" {
			return nil, fmt.Errorf("media_items[%d].url is required", i)
		}
		if _, err := url.ParseRequestURI(u); err != nil {
			return nil, fmt.Errorf("media_items[%d].url is invalid", i)
		}
		entry := map[string]any{"url": u}
		typ := firstString(item, "type", "media_type", "mediaType")
		typ = strings.ToLower(strings.TrimSpace(typ))
		if typ == "" {
			// Infer from URL extension when possible
			ext := strings.ToLower(path.Ext(u))
			switch ext {
			case ".mp4", ".mov", ".webm", ".m4v", ".avi", ".mpeg", ".mpg":
				typ = "video"
			case ".png", ".jpg", ".jpeg", ".gif", ".webp":
				typ = "image"
			case ".mp3", ".aac", ".wav", ".ogg", ".m4a":
				typ = "audio"
			case ".pdf":
				typ = "document"
			}
		}
		if typ != "" {
			entry["type"] = typ
		}
		out = append(out, entry)
	}
	return out, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return int64(t)
			case int64:
				return t
			case int:
				return int64(t)
			case int32:
				return int64(t)
			}
		}
	}
	return 0
}

// presignUsesOwnStorage routes a media ticket by the publishing path the request is
// for. An explicit provider wins; otherwise the active TikTok path decides, because
// the shipped desktop build sends no provider on this route.
// fork: social-media-own-storage
func (s *Service) presignUsesOwnStorage(body map[string]any) bool {
	switch strings.ToLower(firstString(body, "provider", "source")) {
	case ProviderTikTokDirect:
		return true
	case ProviderZernio:
		return false
	}
	return s.cfg.TikTokDirectActive()
}

// presignOwnMediaStorage signs one PUT ticket on the gateway's own bucket and returns
// a URL the upstream platform can read. It never falls back to Zernio: an unavailable
// bucket must fail loudly instead of writing the file to a third party.
// fork: social-media-own-storage
func (s *Service) presignOwnMediaStorage(ctx context.Context, userId int, in PresignInput) (map[string]any, error) {
	if in.SizeBytes > s.cfg.MediaStorageMaxBytes {
		return nil, fmt.Errorf("%w: media exceeds %d bytes", ErrMediaInvalid, s.cfg.MediaStorageMaxBytes)
	}
	driver := storage.GetStorageDriver()
	if driver == nil {
		return nil, ErrMediaStorageUnavailable
	}
	key := socialMediaObjectKey(userId, in.Filename, time.Now().UTC())
	ticket, err := driver.PresignPut(ctx, key, in.ContentType, s.cfg.MediaStoragePresignTTL)
	if errors.Is(err, storage.ErrPresignUnsupported) {
		return nil, ErrMediaStorageUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMediaInvalid, err)
	}
	if ticket == nil || strings.TrimSpace(ticket.URL) == "" {
		return nil, ErrMediaStorageUnavailable
	}
	publicURL, err := socialReadableMediaURL(ctx, driver, key, in.ContentType, s.cfg.MediaStoragePresignTTL)
	if err != nil {
		return nil, err
	}
	headers := ticket.Headers
	if headers == nil {
		headers = map[string]string{"Content-Type": in.ContentType}
	}
	out := map[string]any{
		"upload_url":    ticket.URL,
		"upload_method": ticket.Method,
		"public_url":    publicURL,
		"headers":       headers,
		"filename":      in.Filename,
		"content_type":  in.ContentType,
		"key":           key,
	}
	if ttl := time.Until(ticket.ExpiresAt); ttl > 0 {
		out["expires_in"] = int(ttl.Seconds())
	}
	return out, nil
}

// socialMediaObjectKey keeps every social upload in its own namespace and never lets
// the client choose the key.
// fork: social-media-own-storage
func socialMediaObjectKey(userId int, filename string, now time.Time) string {
	ext := strings.ToLower(path.Ext(path.Base(strings.TrimSpace(filename))))
	if len(ext) > 8 || !isPlainExtension(ext) {
		ext = ""
	}
	return fmt.Sprintf("social/%d/%s/%s%s", userId, now.Format("20060102"), uuid.NewString(), ext)
}

func isPlainExtension(ext string) bool {
	if len(ext) < 2 || ext[0] != '.' {
		return false
	}
	for _, r := range ext[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// socialReadableMediaURL prefers the configured public base and falls back to a
// short-lived presigned GET, so direct posting works without a public bucket origin.
// fork: social-media-own-storage
func socialReadableMediaURL(ctx context.Context, driver storage.StorageDriver, key, contentType string, ttl time.Duration) (string, error) {
	if publicURL := strings.TrimSpace(driver.GetPublicURL(key)); publicURL != "" {
		return publicURL, nil
	}
	if presigner, ok := driver.(storage.DownloadPresigner); ok {
		ticket, err := presigner.PresignGet(ctx, key, contentType, "inline", ttl)
		if err == nil && ticket != nil && strings.TrimSpace(ticket.URL) != "" {
			return ticket.URL, nil
		}
	}
	return "", ErrMediaStorageUnavailable
}
