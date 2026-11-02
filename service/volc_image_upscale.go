package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
)

const (
	ModelVolcImageUpscale = "volc-image-upscale"
	VolcReqKey            = "lens_vida_nnsr"
	VolcMaxImageSize      = 5242880 // 5 MB (5 * 1024 * 1024 bytes)
	VolcMinWidth          = 50
	VolcMaxWidth          = 2128
	VolcMinHeight         = 50
	VolcMaxHeight         = 4064
)

type VolcUpscaleImageMeta struct {
	Width  int
	Height int
	Format string
	Size   int64
}

type contextKey string

const bypassSSRFForTestKey contextKey = "bypass_ssrf_for_test"

// WithBypassSSRFForTest returns a context that bypasses SSRF IP checks exclusively for unit tests.
func WithBypassSSRFForTest(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassSSRFForTestKey, true)
}

var reservedCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"255.255.255.255/32",
		"::1/128",
		"::/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			reservedCIDRs = append(reservedCIDRs, block)
		}
	}
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, block := range reservedCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateVolcImageUpscaleURL validates the URL scheme, SSRF protection, size, format, and dimensions.
func ValidateVolcImageUpscaleURL(ctx context.Context, rawURL string, client *http.Client) (*VolcUpscaleImageMeta, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("image URL cannot be empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid image URL: %w", err)
	}

	if strings.ToLower(u.Scheme) != "https" {
		return nil, errors.New("only https is allowed for image URL")
	}

	if u.User != nil {
		return nil, errors.New("userinfo not allowed in image URL")
	}

	hostname := strings.TrimSpace(u.Hostname())
	if hostname == "" {
		return nil, errors.New("empty host in image URL")
	}

	bypassSSRF, _ := ctx.Value(bypassSSRFForTestKey).(bool)

	// Hostname / IP SSRF validation
	if !bypassSSRF {
		if ip := net.ParseIP(hostname); ip != nil {
			if isPrivateOrReservedIP(ip) {
				return nil, fmt.Errorf("private or loopback IP not allowed: %s", ip.String())
			}
		} else {
			lowerHost := strings.ToLower(hostname)
			if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") || strings.HasSuffix(lowerHost, ".local") {
				return nil, fmt.Errorf("private or loopback IP not allowed: %s", hostname)
			}

			// When client is not provided (production / non-mocked test), perform DNS resolution preflight
			if client == nil {
				resolveCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				ips, err := net.DefaultResolver.LookupIP(resolveCtx, "ip", hostname)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve host %s: %w", hostname, err)
				}
				if len(ips) == 0 {
					return nil, fmt.Errorf("host %s resolved to no IP addresses", hostname)
				}
				for _, resolvedIP := range ips {
					if isPrivateOrReservedIP(resolvedIP) {
						return nil, fmt.Errorf("private or loopback IP not allowed: %s resolves to %s", hostname, resolvedIP.String())
					}
				}
			}
		}
	}

	httpClient := client
	if httpClient == nil {
		httpClient = GetSSRFProtectedHTTPClient()
	}

	// Enforce that redirects also strictly remain HTTPS
	clone := *httpClient
	origCheckRedirect := httpClient.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if strings.ToLower(req.URL.Scheme) != "https" {
			return errors.New("redirect to non-https scheme is not allowed")
		}
		if origCheckRedirect != nil {
			return origCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	httpClient = &clone

	reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image returned status %d", resp.StatusCode)
	}

	if resp.ContentLength > VolcMaxImageSize {
		return nil, fmt.Errorf("image size exceeds 5MB limit: %d bytes", resp.ContentLength)
	}

	limitReader := io.LimitReader(resp.Body, VolcMaxImageSize+1)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image body: %w", err)
	}

	if int64(len(bodyBytes)) > VolcMaxImageSize {
		return nil, fmt.Errorf("image size exceeds 5MB limit (%d bytes)", len(bodyBytes))
	}

	if len(bodyBytes) == 0 {
		return nil, errors.New("empty image data")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("unsupported image format or decoding error: %w", err)
	}

	format = strings.ToLower(format)
	if format != "jpeg" && format != "png" && format != "bmp" {
		return nil, fmt.Errorf("unsupported image format %s: only JPG/JPEG/PNG/BMP are supported", format)
	}

	if cfg.Width < VolcMinWidth || cfg.Width > VolcMaxWidth {
		return nil, fmt.Errorf("image width %d out of bounds [%d, %d]", cfg.Width, VolcMinWidth, VolcMaxWidth)
	}

	if cfg.Height < VolcMinHeight || cfg.Height > VolcMaxHeight {
		return nil, fmt.Errorf("image height %d out of bounds [%d, %d]", cfg.Height, VolcMinHeight, VolcMaxHeight)
	}

	return &VolcUpscaleImageMeta{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
		Size:   int64(len(bodyBytes)),
	}, nil
}
