// Package media stores inspiration covers/files on local disk and optionally R2.
package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/errs"
)

const (
	defaultMaxUploadBytes = 5 << 20
	fetchTimeout          = 15 * time.Second
	maxRedirects          = 3
	sniffBytes            = 512
)

var (
	ErrUnsupportedMedia = errs.Wrap("UNSUPPORTED_MEDIA", http.StatusUnsupportedMediaType, "unsupported media type", nil)
	ErrFetchFailed      = errs.Wrap("FETCH_FAILED", http.StatusBadGateway, "failed to fetch remote media", nil)
	ErrSSRF             = errs.Wrap("INVALID_PARAM", http.StatusBadRequest, "url remote address is not allowed", nil)
	ErrR2Unavailable    = errs.Wrap("R2_UNAVAILABLE", http.StatusServiceUnavailable, "object storage unavailable", nil)
)

type Stored struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Storage     string `json:"storage"`
}

type Store struct {
	dir        string
	urlPrefix  string
	maxBytes   int64
	r2         *r2Client
	httpClient *http.Client
}

func New(cfg *config.Config) *Store {
	maxBytes := cfg.MaxUploadMB << 20
	if maxBytes <= 0 {
		maxBytes = defaultMaxUploadBytes
	}
	return &Store{
		dir:        cfg.MediaDir,
		urlPrefix:  strings.TrimRight(cfg.MediaURLPrefix, "/"),
		maxBytes:   maxBytes,
		r2:         newR2Client(cfg),
		httpClient: newHTTPClient(nil),
	}
}

func newHTTPClient(resolver *net.Resolver) *http.Client {
	dialer := &net.Dialer{
		Resolver: resolver,
		ControlContext: func(_ context.Context, _, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrSSRF
			}
			ip := net.ParseIP(host)
			if ip == nil || blockedIP(ip) {
				return ErrSSRF
			}
			return nil
		},
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		Timeout: fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return validateRemoteURL(req.URL)
		},
	}
}

func (s *Store) HasR2() bool { return s.r2 != nil && s.r2.configured() }

func (s *Store) SaveBytes(ctx context.Context, kind string, raw []byte) (*Stored, error) {
	if int64(len(raw)) > s.maxBytes {
		return nil, errs.Wrap("PAYLOAD_TOO_LARGE", http.StatusRequestEntityTooLarge, "upload exceeds size limit", nil)
	}
	ctype, ext, err := sniff(kind, raw)
	if err != nil {
		return nil, err
	}
	key, err := newKey(kind, ext)
	if err != nil {
		return nil, err
	}
	if err := s.writeLocal(key, raw); err != nil {
		return nil, errs.Wrap("INTERNAL", 500, "write media", err)
	}
	storage := "local"
	if s.HasR2() {
		if err := s.r2.Put(ctx, key, raw, ctype); err != nil {
			_ = os.Remove(s.mustLocalPath(key))
			return nil, ErrR2Unavailable
		}
		storage = "local+r2"
	}
	return &Stored{
		Key:         key,
		URL:         s.urlPrefix + "/" + key,
		ContentType: ctype,
		Size:        int64(len(raw)),
		Storage:     storage,
	}, nil
}

func (s *Store) SaveURL(ctx context.Context, kind, rawURL string) (*Stored, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, errs.ErrInvalidParam
	}
	if err := validateRemoteURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrFetchFailed
	}
	req.Header.Set("User-Agent", "omnimux-inspiration/media")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, ErrFetchFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrFetchFailed
	}
	limited := io.LimitReader(resp.Body, s.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, ErrFetchFailed
	}
	return s.SaveBytes(ctx, kind, raw)
}

func (s *Store) SafeLocalPath(key string) (string, error) {
	if !validKey(key) {
		return "", errs.ErrInvalidParam
	}
	full := filepath.Join(s.dir, filepath.FromSlash(key))
	root := filepath.Clean(s.dir) + string(os.PathSeparator)
	if !strings.HasPrefix(full, root) && full != filepath.Clean(s.dir) {
		return "", errs.ErrInvalidParam
	}
	return full, nil
}

func (s *Store) mustLocalPath(key string) string {
	p, _ := s.SafeLocalPath(key)
	return p
}

func (s *Store) writeLocal(key string, raw []byte) error {
	full, err := s.SafeLocalPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, raw, 0o644)
}

func validKey(key string) bool {
	if key == "" || strings.Contains(key, "..") || strings.Contains(key, "\\") || strings.HasPrefix(key, "/") {
		return false
	}
	return true
}

func newKey(kind, ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	now := time.Now().UTC()
	prefix := "covers"
	if kind == "media" {
		prefix = "files"
	}
	return fmt.Sprintf("%s/%04d/%02d/%s%s", prefix, now.Year(), now.Month(), id, ext), nil
}

func sniff(kind string, raw []byte) (contentType, ext string, err error) {
	if len(raw) < 12 {
		return "", "", ErrUnsupportedMedia
	}
	head := raw
	if len(head) > sniffBytes {
		head = head[:sniffBytes]
	}
	switch {
	case bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg", ".jpg", nil
	case bytes.HasPrefix(head, []byte{0x89, 0x50, 0x4e, 0x47}):
		return "image/png", ".png", nil
	case bytes.HasPrefix(head, []byte("GIF8")):
		return "image/gif", ".gif", nil
	case bytes.HasPrefix(head, []byte("RIFF")) && bytes.Contains(head[:12], []byte("WEBP")):
		return "image/webp", ".webp", nil
	}
	if kind == "media" && bytes.Contains(head[:min(len(head), 12)], []byte("ftyp")) {
		return "video/mp4", ".mp4", nil
	}
	return "", "", ErrUnsupportedMedia
}

func validateRemoteURL(u *url.URL) error {
	if u == nil || u.Host == "" {
		return errs.ErrInvalidParam
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrSSRF
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" || strings.EqualFold(host, "localhost") {
		return ErrSSRF
	}
	if ip := net.ParseIP(host); ip != nil {
		if blockedIP(ip) {
			return ErrSSRF
		}
	}
	return nil
}

var blockedIPv4Prefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

var (
	publicIPv6Prefix    = netip.MustParsePrefix("2000::/3")
	blockedIPv6Prefixes = []netip.Prefix{
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2002::/16"),
		netip.MustParsePrefix("3fff::/20"),
	}
)

func blockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	if addr.Is4() {
		for _, prefix := range blockedIPv4Prefixes {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	if !publicIPv6Prefix.Contains(addr) {
		return true
	}
	for _, prefix := range blockedIPv6Prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
