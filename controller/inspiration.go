package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// fork: inspiration-gateway
// Reverse proxy onto the inspiration sidecar (omnimux-inspiration).
// Lives in controller because service/ was not named for this change.

const (
	inspirationDefaultTimeoutSec      = 5
	inspirationDefaultMediaTimeoutSec = 30
	inspirationMaxJSONBytes           = 8 << 20
	inspirationGatewayMediaPrefix     = "/api/inspiration/v1/media/"
	inspirationUpstreamMediaPrefix    = "/media/"
)

var inspirationHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Authorization":       {},
}

type inspirationConfig struct {
	Enabled      bool
	BaseURL      string
	Token        string
	Timeout      time.Duration
	MediaTimeout time.Duration
}

func (cfg inspirationConfig) configured() bool {
	return cfg.BaseURL != "" && cfg.Token != ""
}

type inspirationClient struct {
	cfg       inspirationConfig
	http      *http.Client
	mediaHTTP *http.Client
}

func loadInspirationConfig() inspirationConfig {
	timeoutSec := common.GetEnvOrDefault("INSPIRATION_TIMEOUT", inspirationDefaultTimeoutSec)
	if timeoutSec <= 0 {
		timeoutSec = inspirationDefaultTimeoutSec
	}
	mediaTimeoutSec := common.GetEnvOrDefault("INSPIRATION_MEDIA_TIMEOUT", inspirationDefaultMediaTimeoutSec)
	if mediaTimeoutSec <= 0 {
		mediaTimeoutSec = inspirationDefaultMediaTimeoutSec
	}
	return inspirationConfig{
		Enabled:      common.GetEnvOrDefaultBool("INSPIRATION_ENABLED", false),
		BaseURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("INSPIRATION_URL")), "/"),
		Token:        strings.TrimSpace(os.Getenv("INSPIRATION_TOKEN")),
		Timeout:      time.Duration(timeoutSec) * time.Second,
		MediaTimeout: time.Duration(mediaTimeoutSec) * time.Second,
	}
}

func newInspirationClient(cfg inspirationConfig) *inspirationClient {
	return &inspirationClient{
		cfg:       cfg,
		http:      &http.Client{Timeout: cfg.Timeout},
		mediaHTTP: &http.Client{Timeout: cfg.MediaTimeout},
	}
}

var inspirationUpstream = newInspirationClient(loadInspirationConfig())

func setInspirationClientForTest(c *inspirationClient) *inspirationClient {
	prev := inspirationUpstream
	if c == nil {
		inspirationUpstream = newInspirationClient(loadInspirationConfig())
	} else {
		inspirationUpstream = c
	}
	return prev
}

func inspirationJSONError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

func inspirationReady(c *gin.Context) bool {
	cfg := inspirationUpstream.cfg
	if !cfg.Enabled {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_DISABLED", "inspiration is disabled")
		return false
	}
	if !cfg.configured() {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_NOT_CONFIGURED", "inspiration upstream is not configured")
		return false
	}
	return true
}

// InspirationStatus GET /api/inspiration/v1/status
func InspirationStatus(c *gin.Context) {
	cfg := inspirationUpstream.cfg
	common.ApiSuccess(c, gin.H{
		"enabled":       cfg.Enabled,
		"configured":    cfg.configured(),
		"gateway_ready": cfg.Enabled && cfg.configured(),
	})
}

// InspirationQuery POST /api/inspiration/v1/query → GET /v1/inspirations
func InspirationQuery(c *gin.Context) {
	if !inspirationReady(c) {
		return
	}
	values, err := inspirationQueryFromBody(c)
	if err != nil {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "invalid query body")
		return
	}
	path := "/v1/inspirations"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	inspirationUpstream.proxyJSON(c, http.MethodGet, path, nil)
}

// InspirationList GET /api/inspiration/v1/inspirations
func InspirationList(c *gin.Context) {
	if !inspirationReady(c) {
		return
	}
	path := "/v1/inspirations"
	if raw := c.Request.URL.RawQuery; raw != "" {
		path += "?" + raw
	}
	inspirationUpstream.proxyJSON(c, http.MethodGet, path, nil)
}

// InspirationGet GET /api/inspiration/v1/inspirations/:id
func InspirationGet(c *gin.Context) {
	inspirationProxyJSON(c, http.MethodGet, "/v1/inspirations/"+url.PathEscape(c.Param("id")), nil)
}

// InspirationCreate POST /api/inspiration/v1/inspirations
func InspirationCreate(c *gin.Context) {
	inspirationProxyBody(c, http.MethodPost, "/v1/inspirations")
}

// InspirationBulk POST /api/inspiration/v1/inspirations/bulk → POST /v1/inspirations:bulk
func InspirationBulk(c *gin.Context) {
	inspirationProxyBody(c, http.MethodPost, "/v1/inspirations:bulk")
}

// InspirationUpdate PATCH /api/inspiration/v1/inspirations/:id
func InspirationUpdate(c *gin.Context) {
	inspirationProxyBody(c, http.MethodPatch, "/v1/inspirations/"+url.PathEscape(c.Param("id")))
}

// InspirationDelete DELETE /api/inspiration/v1/inspirations/:id
func InspirationDelete(c *gin.Context) {
	inspirationProxyJSON(c, http.MethodDelete, "/v1/inspirations/"+url.PathEscape(c.Param("id")), nil)
}

// InspirationListTags GET /api/inspiration/v1/tags
func InspirationListTags(c *gin.Context) {
	inspirationProxyJSON(c, http.MethodGet, "/v1/tags", nil)
}

// InspirationCreateTag POST /api/inspiration/v1/tags
func InspirationCreateTag(c *gin.Context) {
	inspirationProxyBody(c, http.MethodPost, "/v1/tags")
}

// InspirationDeleteTag DELETE /api/inspiration/v1/tags/:id
func InspirationDeleteTag(c *gin.Context) {
	inspirationProxyJSON(c, http.MethodDelete, "/v1/tags/"+url.PathEscape(c.Param("id")), nil)
}

// InspirationUploadMedia POST /api/inspiration/v1/media
func InspirationUploadMedia(c *gin.Context) {
	if !inspirationReady(c) {
		return
	}
	inspirationUpstream.proxyUpload(c)
}

// InspirationMedia GET /api/inspiration/v1/media/*key → GET /media/:key
func InspirationMedia(c *gin.Context) {
	if !inspirationReady(c) {
		return
	}
	key := strings.TrimPrefix(c.Param("key"), "/")
	if !validInspirationMediaKey(key) {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "bad media key")
		return
	}
	inspirationUpstream.proxyMedia(c, key)
}

func inspirationProxyBody(c *gin.Context, method, path string) {
	if !inspirationReady(c) {
		return
	}
	body, err := inspirationReadJSONBody(c)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			inspirationJSONError(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large")
			return
		}
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "invalid request body")
		return
	}
	if raw := c.Request.URL.RawQuery; raw != "" {
		path += "?" + raw
	}
	inspirationUpstream.proxyJSON(c, method, path, body)
}

func inspirationProxyJSON(c *gin.Context, method, path string, body []byte) {
	if !inspirationReady(c) {
		return
	}
	inspirationUpstream.proxyJSON(c, method, path, body)
}

func validInspirationMediaKey(key string) bool {
	if key == "" || strings.Contains(key, "..") {
		return false
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return false
	}
	return true
}

func inspirationReadJSONBody(c *gin.Context) ([]byte, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	raw, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > inspirationMaxJSONBytes {
		return nil, common.ErrRequestBodyTooLarge
	}
	return raw, nil
}

func inspirationQueryFromBody(c *gin.Context) (url.Values, error) {
	raw, err := inspirationReadJSONBody(c)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return url.Values{}, nil
	}
	var body map[string]any
	if err := common.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	values := url.Values{}
	for key, value := range body {
		if value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if v != "" {
				values.Set(key, v)
			}
		case bool:
			values.Set(key, strconv.FormatBool(v))
		case float64:
			values.Set(key, strconv.FormatInt(int64(v), 10))
		case []any:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				s := strings.TrimSpace(fmt.Sprint(item))
				if s != "" && s != "<nil>" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				values.Set(key, strings.Join(parts, ","))
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				values.Set(key, s)
			}
		}
	}
	return values, nil
}

func (cl *inspirationClient) proxyJSON(c *gin.Context, method, path string, body []byte) {
	raw, status, err := cl.doJSON(c, method, path, body, method == http.MethodGet)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	if status >= 500 || status == http.StatusUnauthorized {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "inspiration upstream error")
		return
	}
	if looksLikeJSON(raw) {
		raw = rewriteInspirationMediaURLs(raw)
	}
	contentType := "application/json; charset=utf-8"
	c.Data(status, contentType, raw)
}

func (cl *inspirationClient) doJSON(c *gin.Context, method, path string, body []byte, retry bool) ([]byte, int, error) {
	raw, status, err := cl.roundTripJSON(c, method, path, body)
	if !retry {
		return raw, status, err
	}
	if err != nil || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout {
		return cl.roundTripJSON(c, method, path, body)
	}
	return raw, status, err
}

func (cl *inspirationClient) roundTripJSON(c *gin.Context, method, path string, body []byte) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), method, cl.cfg.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cl.cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if rid := c.GetString(common.RequestIdKey); rid != "" {
		req.Header.Set("X-Request-Id", rid)
	}
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, inspirationMaxJSONBytes))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (cl *inspirationClient) proxyUpload(c *gin.Context) {
	path := "/v1/media"
	if raw := c.Request.URL.RawQuery; raw != "" {
		path += "?" + raw
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, cl.cfg.BaseURL+path, c.Request.Body)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	req.Header.Set("Authorization", "Bearer "+cl.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if ctype := c.GetHeader("Content-Type"); ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if rid := c.GetString(common.RequestIdKey); rid != "" {
		req.Header.Set("X-Request-Id", rid)
	}
	if c.Request.ContentLength > 0 {
		req.ContentLength = c.Request.ContentLength
	}
	resp, err := cl.mediaHTTP.Do(req)
	if err != nil {
		status := http.StatusServiceUnavailable
		if isTimeoutErr(err) {
			status = http.StatusGatewayTimeout
		}
		inspirationJSONError(c, status, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, inspirationMaxJSONBytes))
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "read upstream response failed")
		return
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusUnauthorized {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "inspiration upstream error")
		return
	}
	if looksLikeJSON(raw) {
		raw = rewriteInspirationMediaURLs(raw)
	}
	contentType := "application/json; charset=utf-8"
	c.Data(resp.StatusCode, contentType, raw)
}

func (cl *inspirationClient) proxyMedia(c *gin.Context, key string) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, cl.cfg.BaseURL+inspirationUpstreamMediaPrefix+key, nil)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	req.Header.Set("Authorization", "Bearer "+cl.cfg.Token)
	if rng := c.GetHeader("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}
	if ir := c.GetHeader("If-Range"); ir != "" {
		req.Header.Set("If-Range", ir)
	}
	if rid := c.GetString(common.RequestIdKey); rid != "" {
		req.Header.Set("X-Request-Id", rid)
	}
	resp, err := cl.mediaHTTP.Do(req)
	if err != nil {
		status := http.StatusServiceUnavailable
		if isTimeoutErr(err) {
			status = http.StatusGatewayTimeout
		}
		inspirationJSONError(c, status, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	defer resp.Body.Close()
	for name, values := range resp.Header {
		if _, hop := inspirationHopHeaders[http.CanonicalHeaderKey(name)]; hop {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(name, value)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func rewriteInspirationMediaURLs(raw []byte) []byte {
	return bytes.ReplaceAll(raw, []byte(`"`+inspirationUpstreamMediaPrefix), []byte(`"`+inspirationGatewayMediaPrefix))
}

func looksLikeJSON(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}
