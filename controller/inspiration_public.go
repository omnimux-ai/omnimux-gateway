package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// fork: inspiration-gateway
// Anonymous, read-only projection of the inspiration sidecar. The public
// inspiration-community pages (marketing surfaces, no account required) read
// this instead of the authenticated /inspiration/v1 relay: every query
// parameter is validated against an allowlist and every response field is
// projected, so the sidecar's internal shape (analysis payloads, media object
// keys, import bookkeeping) never reaches an anonymous caller.

const (
	inspirationPublicMediaPrefix  = "/api/inspiration/v1/public/media/"
	inspirationPublicDefaultSize  = 30
	inspirationPublicMaxSize      = 100
	inspirationPublicCacheControl = "public, max-age=120"
	inspirationPublicDateLayout   = "2006-01-02"
)

var inspirationPublicTypes = map[string]struct{}{
	"video": {},
	"image": {},
	"link":  {},
}

var inspirationPublicTrafficTypes = map[string]struct{}{
	"ad":      {},
	"organic": {},
}

var inspirationPublicSorts = map[string]struct{}{
	"hot":   {},
	"new":   {},
	"views": {},
}

// inspirationPublicSourceHosts are the only hosts a projected item may link
// out to. Rows are imported from several networks; anything else is withheld.
var inspirationPublicSourceHosts = []string{
	"tiktok.com",
	"instagram.com",
	"youtube.com",
	"youtu.be",
}

// InspirationPublicList GET /api/inspiration/v1/public/inspirations
func InspirationPublicList(c *gin.Context) {
	if !inspirationReady(c) {
		return
	}
	values, err := inspirationPublicQuery(c)
	if err != nil {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", err.Error())
		return
	}
	raw, status, err := inspirationUpstream.doJSON(c, http.MethodGet, "/v1/inspirations?"+values.Encode(), nil, true)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "inspiration upstream unavailable")
		return
	}
	if status != http.StatusOK {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "inspiration upstream error")
		return
	}
	page, err := inspirationPublicProject(raw, values)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "inspiration upstream response invalid")
		return
	}
	c.Header("Cache-Control", inspirationPublicCacheControl)
	common.ApiSuccess(c, page)
}

// InspirationPublicMedia GET /api/inspiration/v1/public/media/*key
func InspirationPublicMedia(c *gin.Context) {
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

// inspirationPublicQuery validates the anonymous query string and returns the
// upstream parameters to forward. Unknown parameters are dropped, never passed
// through: the sidecar relay accepts more (and more revealing) filters than a
// public list needs.
func inspirationPublicQuery(c *gin.Context) (url.Values, error) {
	values := url.Values{}

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return nil, errors.New("invalid page")
		}
		page = parsed
	}
	size := inspirationPublicDefaultSize
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > inspirationPublicMaxSize {
			return nil, errors.New("invalid page_size")
		}
		size = parsed
	}
	values.Set("page", strconv.Itoa(page))
	values.Set("page_size", strconv.Itoa(size))

	if raw := strings.ToUpper(strings.TrimSpace(c.Query("country"))); raw != "" {
		if len(raw) != 2 {
			return nil, errors.New("invalid country")
		}
		for _, r := range raw {
			if r < 'A' || r > 'Z' {
				return nil, errors.New("invalid country")
			}
		}
		values.Set("country", raw)
	}
	if raw := strings.TrimSpace(c.Query("type")); raw != "" {
		if _, ok := inspirationPublicTypes[raw]; !ok {
			return nil, errors.New("invalid type")
		}
		values.Set("type", raw)
	}
	if raw := strings.TrimSpace(c.Query("traffic_type")); raw != "" {
		if _, ok := inspirationPublicTrafficTypes[raw]; !ok {
			return nil, errors.New("invalid traffic_type")
		}
		values.Set("traffic_type", raw)
	}
	if raw := strings.TrimSpace(c.Query("sort")); raw != "" {
		if _, ok := inspirationPublicSorts[raw]; !ok {
			return nil, errors.New("invalid sort")
		}
		values.Set("sort", raw)
	}
	for _, key := range []string{"duration_min", "duration_max", "views_min", "views_max"} {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			continue
		}
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			return nil, fmt.Errorf("invalid %s", key)
		}
		values.Set(key, strconv.FormatInt(parsed, 10))
	}
	for _, key := range []string{"posted_after", "posted_before"} {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			continue
		}
		if !validInspirationPublicDate(raw) {
			return nil, fmt.Errorf("invalid %s", key)
		}
		values.Set(key, raw)
	}
	return values, nil
}

func validInspirationPublicDate(raw string) bool {
	_, err := time.Parse(inspirationPublicDateLayout, raw)
	return err == nil
}

// inspirationPublicPage is the projected, cacheable list payload.
type inspirationPublicPage struct {
	Total     int64                   `json:"total"`
	Page      int                     `json:"page"`
	PageSize  int                     `json:"page_size"`
	PageCount int                     `json:"page_count"`
	Items     []inspirationPublicItem `json:"items"`
}

type inspirationPublicCreator struct {
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

type inspirationPublicItem struct {
	ID          uint64                    `json:"id"`
	Type        string                    `json:"type"`
	Title       string                    `json:"title"`
	Caption     string                    `json:"caption"`
	Country     string                    `json:"country"`
	Category    string                    `json:"category"`
	Duration    int                       `json:"duration"`
	Views       int64                     `json:"views"`
	TrafficType string                    `json:"trafficType"`
	PostedAt    *string                   `json:"postedAt"`
	CoverURL    *string                   `json:"coverUrl"`
	MediaURLs   []string                  `json:"mediaUrls"`
	SourceURL   *string                   `json:"sourceUrl"`
	EmbedURL    *string                   `json:"embedUrl"`
	Creator     *inspirationPublicCreator `json:"creator"`
}

type inspirationUpstreamItem struct {
	ID          uint64         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	SourceURL   string         `json:"source_url"`
	CoverKey    string         `json:"cover_key"`
	MediaKeys   []string       `json:"media_keys"`
	CountryCode string         `json:"country_code"`
	Category    string         `json:"category"`
	Duration    int            `json:"duration"`
	Views       int64          `json:"views"`
	TrafficType string         `json:"traffic_type"`
	PostedAt    *string        `json:"posted_at"`
	Analysis    map[string]any `json:"analysis"`
}

type inspirationUpstreamPage struct {
	Total int64                     `json:"total"`
	Page  int                       `json:"page"`
	Size  int                       `json:"size"`
	Items []inspirationUpstreamItem `json:"items"`
}

type inspirationUpstreamEnvelope struct {
	Success bool                    `json:"success"`
	Data    inspirationUpstreamPage `json:"data"`
}

func inspirationPublicProject(raw []byte, values url.Values) (inspirationPublicPage, error) {
	var envelope inspirationUpstreamEnvelope
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return inspirationPublicPage{}, err
	}
	if !envelope.Success {
		return inspirationPublicPage{}, errors.New("upstream reported failure")
	}
	page := inspirationPublicPage{
		Total:    envelope.Data.Total,
		Page:     inspirationPublicInt(values.Get("page"), 1),
		PageSize: inspirationPublicInt(values.Get("page_size"), inspirationPublicDefaultSize),
		Items:    make([]inspirationPublicItem, 0, len(envelope.Data.Items)),
	}
	if page.PageSize > 0 {
		page.PageCount = int((page.Total + int64(page.PageSize) - 1) / int64(page.PageSize))
	}
	for _, item := range envelope.Data.Items {
		page.Items = append(page.Items, inspirationPublicItemFrom(item))
	}
	return page, nil
}

func inspirationPublicItemFrom(item inspirationUpstreamItem) inspirationPublicItem {
	return inspirationPublicItem{
		ID:          item.ID,
		Type:        item.Type,
		Title:       item.Title,
		Caption:     item.Content,
		Country:     strings.ToUpper(strings.TrimSpace(item.CountryCode)),
		Category:    item.Category,
		Duration:    item.Duration,
		Views:       item.Views,
		TrafficType: item.TrafficType,
		PostedAt:    inspirationPublicInstant(item.PostedAt),
		CoverURL:    inspirationPublicMediaURL(item.CoverKey),
		MediaURLs:   inspirationPublicMediaURLs(item.MediaKeys),
		SourceURL:   inspirationPublicSourceURL(item.SourceURL),
		EmbedURL:    inspirationPublicEmbedURL(item.Analysis),
		Creator:     inspirationPublicCreatorFrom(item.Analysis),
	}
}

func inspirationPublicMediaURLs(keys []string) []string {
	urls := make([]string, 0, len(keys))
	for _, key := range keys {
		if url := inspirationPublicMediaURL(key); url != nil {
			urls = append(urls, *url)
		}
	}
	return urls
}

// inspirationPublicMediaURL maps an upstream "/media/<key>" path onto the
// anonymous gateway path. Absolute URLs (which the sidecar may hand back for
// externally hosted media) and malformed keys are withheld rather than
// published.
func inspirationPublicMediaURL(raw string) *string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, inspirationUpstreamMediaPrefix) {
		return nil
	}
	key := strings.TrimPrefix(raw, inspirationUpstreamMediaPrefix)
	if !validInspirationMediaKey(key) {
		return nil
	}
	out := inspirationPublicMediaPrefix + key
	return &out
}

// inspirationPublicSourceURL publishes an outbound link only when it points at
// a real post on a known network. Imported rows can carry internal asset ids in
// the path (for example "@creator/video/gxgen-<uuid>"); those URLs resolve
// nowhere, so they are hidden instead of surfaced as broken links.
func inspirationPublicSourceURL(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := false
	for _, suffix := range inspirationPublicSourceHosts {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil
	}
	if strings.Contains(strings.ToLower(parsed.Path), "gxgen") {
		return nil
	}
	out := parsed.String()
	return &out
}

// inspirationPublicEmbedURL publishes the network's own player address when the
// row carries one. A quarter of the imported rows have no reachable media
// object at all, so this is the only playback surface the page can offer; the
// value is accepted only when it is a TikTok player URL for a numeric video id.
func inspirationPublicEmbedURL(analysis map[string]any) *string {
	if len(analysis) == 0 {
		return nil
	}
	raw := strings.TrimSpace(inspirationPublicString(analysis["embed_player_url"]))
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "www.tiktok.com" && host != "tiktok.com" {
		return nil
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 3 || segments[0] != "player" {
		return nil
	}
	version := segments[1]
	if len(version) < 2 || version[0] != 'v' || !inspirationPublicDigits(version[1:]) {
		return nil
	}
	if !inspirationPublicDigits(segments[2]) {
		return nil
	}
	out := parsed.String()
	return &out
}

func inspirationPublicDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func inspirationPublicCreatorFrom(analysis map[string]any) *inspirationPublicCreator {
	if len(analysis) == 0 {
		return nil
	}
	raw, ok := analysis["creator"]
	if !ok {
		return nil
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	name := strings.TrimSpace(inspirationPublicString(fields["name"]))
	handle := strings.TrimSpace(inspirationPublicString(fields["handle"]))
	if name == "" && handle == "" {
		return nil
	}
	if name == "" {
		name = handle
	}
	if handle == "" {
		handle = name
	}
	return &inspirationPublicCreator{Name: name, Handle: handle}
}

func inspirationPublicString(value any) string {
	text, _ := value.(string)
	return text
}

func inspirationPublicInstant(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func inspirationPublicInt(raw string, fallback int) int {
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
