package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hash/crc32"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type referenceFetchTransport func(*http.Request) (*http.Response, error)

func (tr referenceFetchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return tr(r) }

func TestInfluencerReferenceValidatesContentAndFetchBoundary(t *testing.T) {
	var pngData, jpegData, gifData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	require.NoError(t, png.Encode(&pngData, img))
	require.NoError(t, jpeg.Encode(&jpegData, img, nil))
	require.NoError(t, gif.Encode(&gifData, img, nil))
	oversizedPNG := append([]byte{}, pngData.Bytes()...)
	binary.BigEndian.PutUint32(oversizedPNG[16:20], 10000)
	binary.BigEndian.PutUint32(oversizedPNG[20:24], 5000)
	binary.BigEndian.PutUint32(oversizedPNG[29:33], crc32.ChecksumIEEE(oversizedPNG[12:29]))
	webpData, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		data   []byte
		length int64
		status int
		want   string
	}{
		{name: "PNG", data: pngData.Bytes(), status: 200},
		{name: "JPEG", data: jpegData.Bytes(), status: 200},
		{name: "WebP", data: webpData, status: 200},
		{name: "exact 5 MiB", data: append(append([]byte{}, pngData.Bytes()...), make([]byte, 5*1024*1024-len(pngData.Bytes()))...), status: 200},
		{name: "GIF", data: gifData.Bytes(), status: 200, want: "JPEG, PNG or WebP"},
		{name: "invalid bytes", data: []byte("not an image"), status: 200, want: "decoded"},
		{name: "truncated PNG", data: pngData.Bytes()[:len(pngData.Bytes())-20], status: 200, want: "decoded"},
		{name: "content length", data: pngData.Bytes(), length: 5*1024*1024 + 1, status: 200, want: "5 MiB"},
		{name: "chunked over limit", data: bytes.Repeat([]byte("x"), 5*1024*1024+1), length: -1, status: 200, want: "5 MiB"},
		{name: "missing", status: 404, want: "HTTP 404"},
		{name: "decompression bomb", data: oversizedPNG, status: 200, want: "decoded size is too large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) {
				assert.Empty(t, r.Header.Get("Authorization"))
				assert.Empty(t, r.Header.Get("Cookie"))
				deadline, ok := r.Context().Deadline()
				require.True(t, ok)
				assert.LessOrEqual(t, time.Until(deadline), 15*time.Second)
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, ContentLength: tc.length, Body: io.NopCloser(bytes.NewReader(tc.data)), Request: r}, nil
			})}
			err := ValidateInfluencerReference(context.Background(), "https://8.8.8.8/image", client)
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestInfluencerReferenceRejectsUnsafeRedirects(t *testing.T) {
	for _, target := range []string{"http://8.8.8.8/a", "https://127.0.0.1/a", "https://u:p@8.8.8.8/a"} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})}
			require.Error(t, ValidateInfluencerReference(context.Background(), "https://8.8.8.8/start", client))
			assert.Equal(t, 1, calls)
		})
	}
}

func TestInfluencerReferenceErrorsDoNotExposeCapabilities(t *testing.T) {
	const secret = "test-private-reference-capability"
	for _, redirect := range []bool{false, true} {
		client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) {
			if redirect && r.URL.Path == "/start" {
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://8.8.4.4/private?signature=" + secret}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			}
			return nil, fmt.Errorf("transport failed at %s", r.URL.String())
		})}
		raw := "https://8.8.8.8/private?signature=" + secret
		if redirect {
			raw = "https://8.8.8.8/start"
		}
		err := ValidateInfluencerReference(context.Background(), raw, client)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), secret)
		assert.NotContains(t, err.Error(), "https://")
	}
}

func TestInfluencerReferencePropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("fetch: %w", r.Context().Err()) })}
	require.ErrorContains(t, ValidateInfluencerReference(ctx, "https://8.8.8.8/image", client), "fetch was canceled")
}

func TestInfluencerReferencePublicOnlyTransportRejectsPrivateDNS(t *testing.T) {
	client := NewInfluencerReferenceHTTPClient()
	defer client.CloseIdleConnections()
	require.ErrorContains(t, ValidateInfluencerReference(context.Background(), "https://localhost/reference.png", client), "could not be fetched")
}

func TestInfluencerReferenceHonorsStricterGlobalPolicy(t *testing.T) {
	fetch := system_setting.GetFetchSetting()
	old := *fetch
	t.Cleanup(func() { *fetch = old })
	for _, tc := range []struct {
		name, raw, want string
		setting         system_setting.FetchSetting
	}{
		{"disabled still public", "https://127.0.0.1/a", "private IP", system_setting.FetchSetting{}},
		{"private permission still public", "https://127.0.0.1/a", "private IP", system_setting.FetchSetting{EnableSSRFProtection: true, AllowPrivateIp: true}},
		{"domain blacklist", "https://blocked.example/a", "domain in blacklist", system_setting.FetchSetting{EnableSSRFProtection: true, DomainList: []string{"blocked.example"}}},
		{"domain whitelist", "https://other.example/a", "domain not in whitelist", system_setting.FetchSetting{EnableSSRFProtection: true, DomainFilterMode: true, DomainList: []string{"allowed.example"}}},
		{"IP blacklist", "https://8.8.8.8/a", "ip in blacklist", system_setting.FetchSetting{EnableSSRFProtection: true, IpList: []string{"8.8.8.8"}}},
		{"IP whitelist", "https://8.8.8.8/a", "ip not in whitelist", system_setting.FetchSetting{EnableSSRFProtection: true, IpFilterMode: true, IpList: []string{"1.1.1.1"}}},
		{"port restriction", "https://8.8.8.8:8443/a", "port 8443 is not allowed", system_setting.FetchSetting{EnableSSRFProtection: true, AllowedPorts: []string{"443"}}},
		{"invalid config fail closed", "https://8.8.8.8/a", "invalid port configuration", system_setting.FetchSetting{EnableSSRFProtection: true, AllowedPorts: []string{"bad"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*fetch = tc.setting
			calls := 0
			client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected fetch") })}
			require.ErrorContains(t, ValidateInfluencerReference(context.Background(), tc.raw, client), tc.want)
			assert.Zero(t, calls)
		})
	}
}

func TestInfluencerReferenceTransportEnforcesTLS(t *testing.T) {
	old := common.TLSInsecureSkipVerify
	common.TLSInsecureSkipVerify = true
	t.Cleanup(func() { common.TLSInsecureSkipVerify = old })
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data.Bytes()) }))
	defer server.Close()
	client := NewInfluencerReferenceHTTPClient()
	defer client.CloseIdleConnections()
	transport := client.Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	require.ErrorContains(t, ValidateInfluencerReference(context.Background(), "https://8.8.8.8/reference.png", client), "certificate")
	assert.True(t, transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify)
}

func TestInfluencerReferenceDNSPolicyPinsValidatedPublicIP(t *testing.T) {
	fetch := system_setting.GetFetchSetting()
	old := *fetch
	t.Cleanup(func() { *fetch = old })
	*fetch = system_setting.FetchSetting{EnableSSRFProtection: true, AllowPrivateIp: true, ApplyIPFilterForDomain: false, IpList: []string{"8.8.8.8"}}
	for _, ip := range []string{"127.0.0.1", "8.8.8.8", "1.1.1.1"} {
		t.Run(ip, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			dialed := ""
			dialer := &protectedFetchDialer{resolver: referenceContextResolver(func(lookupCtx context.Context, host string) ([]net.IPAddr, error) {
				_, ok := lookupCtx.Deadline()
				assert.True(t, ok)
				assert.Equal(t, "reference.example", host)
				return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
			}), getProtection: influencerReferenceProtection, dialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialed = address
				return nil, errors.New("test dial boundary")
			}}
			_, err := dialer.DialContext(ctx, "tcp", "reference.example:443")
			if ip == "1.1.1.1" {
				require.ErrorContains(t, err, "test dial boundary")
				assert.Equal(t, "1.1.1.1:443", dialed)
			} else {
				require.Error(t, err)
				assert.Empty(t, dialed)
			}
		})
	}
}

type referenceContextResolver func(context.Context, string) ([]net.IPAddr, error)

func (r referenceContextResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return r(ctx, host)
}

func TestInfluencerReferenceRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{"http://8.8.8.8/a.png", "https://user:password@8.8.8.8/a.png", "https://127.0.0.1/a.png", "https://10.0.0.1/a.png", "https://[::1]/a.png", "data:image/png;base64,AA", "/relative.png"} {
		t.Run(raw, func(t *testing.T) {
			require.Error(t, ValidateInfluencerReference(context.Background(), raw, NewInfluencerReferenceHTTPClient()))
		})
	}
}

func TestInfluencerReferenceBoundsBothEdgesWithoutReplacingAreaProtection(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		accepted      bool
	}{
		{"exact width", 23170, 1, true}, {"exact height", 1, 23170, true},
		{"width too large", 23171, 1, false}, {"height too large", 1, 23171, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, tc.width, tc.height))))
			client := &http.Client{Transport: referenceFetchTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data.Bytes())), Request: r}, nil
			})}
			err := ValidateInfluencerReference(context.Background(), "https://8.8.8.8/reference.png", client)
			if tc.accepted {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "23170")
			}
		})
	}
}
