package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/image/webp"
)

const influencerReferenceMaxBytes int64 = 5 * 1024 * 1024

// The reference policy retains enabled administrator restrictions and always requires public IPs.
func influencerReferenceProtection() (*common.SSRFProtection, bool, error) {
	protection, enabled, err := currentFetchProtection()
	if err != nil {
		return nil, true, err
	}
	if !enabled {
		protection = &common.SSRFProtection{}
	} else {
		copy := *protection
		protection = &copy
	}
	protection.AllowPrivateIp = false
	protection.ApplyIPFilterForDomain = true
	return protection, true, nil
}

func influencerReferenceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("reference image must be a public HTTPS URL without userinfo")
	}
	port := 443
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return fmt.Errorf("invalid reference port")
		}
	}
	protection, _, err := influencerReferenceProtection()
	if err != nil {
		return err
	}
	return protection.ValidateNetworkTarget(u.Hostname(), port)
}

// NewInfluencerReferenceHTTPClient reuses protected fetch with a feature-local public-only policy.
func NewInfluencerReferenceHTTPClient() *http.Client {
	client := newProtectedFetchHTTPClientWithProxy(nil, nil, influencerReferenceProtection, func(*http.Request) (*url.URL, error) { return nil, nil })
	// Use its DNS-pinned transport directly: the generic URL precheck resolves DNS
	// outside the request context and follows deployment-wide private-IP settings.
	transport := client.Transport.(*ssrfProtectedRoundTripper).transportFor(nil).(*http.Transport)
	if transport.TLSClientConfig != nil {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		transport.TLSClientConfig.InsecureSkipVerify = false
	}
	client.Transport = transport
	return client
}

// ValidateInfluencerReference checks a bounded, decodable image before native precharge.
func ValidateInfluencerReference(ctx context.Context, rawURL string, base *http.Client) error {
	_, _, _, err := readInfluencerReference(ctx, rawURL, base, nil)
	return err
}

// readInfluencerReference retains the validated original bytes rather than fetching
// a mutable reference again after validation.
func readInfluencerReference(ctx context.Context, rawURL string, base *http.Client, authorizeURL func(context.Context, string) error) ([]byte, image.Config, string, error) {
	var empty image.Config
	rawURL = strings.TrimSpace(rawURL)
	if err := influencerReferenceURL(rawURL); err != nil {
		return nil, empty, "", err
	}
	if base == nil {
		base = NewInfluencerReferenceHTTPClient()
		defer base.CloseIdleConnections()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if authorizeURL != nil {
		if err := authorizeURL(ctx, rawURL); err != nil {
			return nil, empty, "", err
		}
	}
	client := *base
	client.Timeout = 15 * time.Second
	client.Jar = nil
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many reference redirects")
		}
		if err := influencerReferenceURL(req.URL.String()); err != nil {
			return err
		}
		if authorizeURL != nil {
			return authorizeURL(req.Context(), req.URL.String())
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, empty, "", fmt.Errorf("reference image request is invalid")
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, empty, "", fmt.Errorf("reference image fetch was canceled")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, empty, "", fmt.Errorf("reference image fetch timed out")
		}
		return nil, empty, "", fmt.Errorf("reference image could not be fetched; check its availability and HTTPS certificate")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, empty, "", fmt.Errorf("reference image returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > influencerReferenceMaxBytes {
		return nil, empty, "", fmt.Errorf("reference image exceeds 5 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, influencerReferenceMaxBytes+1))
	if err != nil {
		return nil, empty, "", fmt.Errorf("reference image could not be read")
	}
	config, format, err := validateInfluencerImageBytes(data)
	return data, config, format, err
}

func validateInfluencerImageBytes(data []byte) (image.Config, string, error) {
	var empty image.Config
	if int64(len(data)) > influencerReferenceMaxBytes {
		return empty, "", fmt.Errorf("reference image exceeds 5 MiB")
	}
	config, format, err := getImageConfig(bytes.NewReader(data))
	if err != nil {
		return empty, "", fmt.Errorf("reference image could not be decoded")
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return empty, "", fmt.Errorf("reference image must be JPEG, PNG or WebP")
	}
	// Bound each reference edge and decoded area before allocating image memory.
	if config.Width > 23170 || config.Height > 23170 {
		return empty, "", fmt.Errorf("reference image edge exceeds 23170 pixels")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > 40_000_000/int64(config.Height) {
		return empty, "", fmt.Errorf("reference image decoded size is too large")
	}
	if format == "webp" {
		_, err = webp.Decode(bytes.NewReader(data))
	} else {
		_, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return empty, "", fmt.Errorf("reference image could not be decoded")
	}
	return config, format, nil
}
