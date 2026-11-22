// Package analytics sends product events through a bounded background queue.
package analytics

import (
	"bytes"
	"context"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	EventSignUpSuccess     = "sign_up_success"
	EventFirstVideoStart   = "first_video_start"
	EventFirstVideoSuccess = "first_video_success"
	EventTopUpPaid         = "topup_paid"

	queueSize        = 256
	workerCount      = 2
	jobTimeout       = 3 * time.Second
	requestTimeout   = 3 * time.Second
	maxResponseBytes = 8192
)

// Event carries internal identifiers and server-derived properties. The
// collector applies its own whitelist and hashes identifiers before sending.
type Event struct {
	Name       string
	UserID     int
	OrderRef   string
	OccurredAt int64 // Unix seconds at the committed business event.
	Data       map[string]any
}

type config struct {
	endpoint      string
	siteID        string
	siteURL       string
	userKeySecret string
}

type client struct {
	config
	hostname   string
	httpClient *http.Client
	queue      chan func(context.Context) (Event, bool)
	workers    sync.WaitGroup
	warnings   sync.Map
}

func newClient(cfg config) *client {
	cfg.endpoint = strings.TrimSpace(cfg.endpoint)
	cfg.siteID = strings.TrimSpace(cfg.siteID)
	cfg.siteURL = strings.TrimSpace(cfg.siteURL)
	cfg.userKeySecret = strings.TrimSpace(cfg.userKeySecret)
	c := &client{config: cfg}

	endpoint, err := url.Parse(cfg.endpoint)
	siteID, idErr := uuid.Parse(cfg.siteID)
	if err != nil || !validEndpoint(endpoint) || len(cfg.siteID) != 36 ||
		idErr != nil || siteID == uuid.Nil || cfg.userKeySecret == "" {
		return c
	}
	if cfg.siteURL != "" {
		site, err := url.Parse(cfg.siteURL)
		if err != nil || (site.Scheme != "https" && site.Scheme != "http") || site.User != nil {
			return c
		}
		c.hostname = canonicalHostname(site.Hostname())
		if c.hostname == "" {
			return c
		}
	}

	c.siteID = siteID.String()
	c.httpClient = &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c.queue = make(chan func(context.Context) (Event, bool), queueSize)
	for range workerCount {
		c.workers.Go(func() {
			for job := range c.queue {
				c.runJob(job)
			}
		})
	}
	return c
}

func validEndpoint(endpoint *url.URL) bool {
	if endpoint == nil || endpoint.Hostname() == "" || endpoint.User != nil ||
		endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return false
	}
	switch endpoint.Scheme {
	case "https":
		return true
	case "http":
		host := endpoint.Hostname()
		return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
	default:
		return false
	}
}

var defaultClient = sync.OnceValue(func() *client {
	return newClient(config{
		endpoint:      os.Getenv("ANALYTICS_ENDPOINT"),
		siteID:        os.Getenv("UMAMI_WEBSITE_ID"),
		siteURL:       os.Getenv("ANALYTICS_SITE_URL"),
		userKeySecret: os.Getenv("ANALYTICS_USER_KEY_SECRET"),
	})
})

// SiteID returns the configured Umami website ID for namespacing video claims.
func SiteID() string {
	return defaultClient().siteID
}

// Track schedules a single event off the request path.
func Track(event Event) {
	if event.OccurredAt == 0 {
		event.OccurredAt = time.Now().Unix()
	}
	c := defaultClient()
	event.Data = maps.Clone(event.Data)
	c.enqueue(func(context.Context) (Event, bool) { return event, true })
}

// Enqueue schedules work, such as a video claim, that must finish before an
// event can be emitted. The job receives a deadline and may decline to send.
func Enqueue(job func(context.Context) (Event, bool)) {
	defaultClient().enqueue(job)
}

func (c *client) enqueue(job func(context.Context) (Event, bool)) {
	if c.queue == nil {
		c.warnOnce("disabled because collector configuration is missing or invalid")
		return
	}
	select {
	case c.queue <- job:
	default:
		c.warnOnce("queue full")
	}
}

func (c *client) runJob(job func(context.Context) (Event, bool)) {
	defer func() {
		if recover() != nil {
			c.warnOnce("background event failed")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()
	event, ok := job(ctx)
	if !ok {
		return
	}
	if reason := c.send(event); reason != "" {
		c.warnOnce(reason)
	}
}

// WarnOnce logs a fixed application reason once per process. Callers must pass
// constant categories so user data and transport errors stay out of logs.
func WarnOnce(reason string) {
	defaultClient().warnOnce(reason)
}

func (c *client) warnOnce(reason string) {
	if _, logged := c.warnings.LoadOrStore(reason, struct{}{}); !logged {
		go common.SysError(common.LogText("analytics: %s", reason))
	}
}

type umamiEnvelope struct {
	Type    string       `json:"type"`
	Payload umamiPayload `json:"payload"`
}

type umamiPayload struct {
	Website   string         `json:"website"`
	Hostname  string         `json:"hostname,omitempty"`
	URL       string         `json:"url"`
	Name      string         `json:"name"`
	ID        string         `json:"id"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Data      map[string]any `json:"data"`
}

// writeAudit records the same sanitized JSON sent to the collector.
func writeAudit(body []byte) error {
	common.LogWriterMu.RLock()
	defer common.LogWriterMu.RUnlock()
	_, err := gin.DefaultWriter.Write(append(body, '\n'))
	return err
}

// send returns only fixed diagnostic categories. The collector may acknowledge
// a rejected event with a 2xx response, so require its documented cache token.
func (c *client) send(event Event) string {
	payload, ok := c.payload(event)
	if !ok {
		return "invalid event"
	}
	body, err := common.Marshal(umamiEnvelope{Type: "event", Payload: payload})
	if err != nil {
		return "invalid event"
	}
	if err := writeAudit(body); err != nil {
		c.warnOnce("audit log unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "invalid collector configuration"
	}
	request.Header.Set("Content-Type", "application/json")
	// Keep the analytics User-Agent stable across gateway releases. Umami
	// includes it in the session identity even when a distinct ID is present.
	request.Header.Set("User-Agent", "OmniMux-Gateway/1.0 (OmniMux Product Analytics)")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "collector unavailable"
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "collector rejected event"
	}
	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "collector unavailable"
	}
	var acknowledgment struct {
		Cache    string `json:"cache"`
		Disabled bool   `json:"disabled"`
	}
	if len(body) > maxResponseBytes || common.Unmarshal(body, &acknowledgment) != nil ||
		strings.TrimSpace(acknowledgment.Cache) == "" || acknowledgment.Disabled {
		return "collector rejected event"
	}
	return ""
}
