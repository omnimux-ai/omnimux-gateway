package hostedgateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Client is the sole OmniMux outbound to the GeeLark business gateway.
// The API key never leaves this package in responses.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(cfg Config) *Client {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultTimeoutMS * time.Millisecond
	}
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		APIKey:  strings.TrimSpace(cfg.APIKey),
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func NewFromEnv() *Client {
	return NewClient(LoadConfig())
}

func (c *Client) Ready() bool {
	return c != nil && c.BaseURL != "" && c.APIKey != ""
}

func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	if c == nil || c.BaseURL == "" {
		return nil, ErrGatewayUnavailable
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/api/v1/health", nil, false)
	if err != nil {
		return nil, err
	}
	var out HealthStatus
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueueStatus(ctx context.Context) (*QueueStatus, error) {
	raw, err := c.postDomain(ctx, "/api/v1/automation/queue/status", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out QueueStatus
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeviceGuardCheck(ctx context.Context, phoneID string) (*DeviceGuardResult, error) {
	raw, err := c.postDomain(ctx, "/api/v1/deviceguard/check", map[string]any{"phoneId": phoneID})
	if err != nil {
		return nil, err
	}
	var out DeviceGuardResult
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PhoneList(ctx context.Context, page, pageSize int) (*PhoneListPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	env, err := c.postEnvelope(ctx, "/api/v1/phone/list", map[string]any{
		"page":     page,
		"pageSize": pageSize,
	})
	if err != nil {
		return nil, err
	}
	return decodePhoneList(env.Data)
}

func (c *Client) JobsCreate(ctx context.Context, body map[string]any) (*Job, error) {
	raw, err := c.postDomain(ctx, "/api/v1/automation/jobs", body)
	if err != nil {
		return nil, err
	}
	return decodeJob(raw)
}

func (c *Client) JobsGet(ctx context.Context, jobID, idempotencyKey string) (*Job, error) {
	body := map[string]any{}
	if jobID != "" {
		body["jobId"] = jobID
	}
	if idempotencyKey != "" {
		body["idempotencyKey"] = idempotencyKey
	}
	raw, err := c.postDomain(ctx, "/api/v1/automation/jobs/get", body)
	if err != nil {
		return nil, err
	}
	return decodeJob(raw)
}

func (c *Client) JobsCancel(ctx context.Context, jobID string) (*Job, error) {
	raw, err := c.postDomain(ctx, "/api/v1/automation/jobs/cancel", map[string]any{"jobId": jobID})
	if err != nil {
		return nil, err
	}
	return decodeJob(raw)
}

func (c *Client) TaskAdd(ctx context.Context, body map[string]any) (*TaskAddResult, error) {
	env, err := c.postEnvelope(ctx, "/api/v1/automation/task/add", body)
	if err != nil {
		return nil, err
	}
	return decodeTaskAdd(env.Data)
}

func (c *Client) TiktokEdit(ctx context.Context, body map[string]any) (*Envelope, error) {
	return c.postEnvelope(ctx, "/api/v1/rpa/tiktok/edit", body)
}

func (c *Client) AnalyticsAccountsList(ctx context.Context, body map[string]any) (*AnalyticsAccountsPage, error) {
	env, err := c.postEnvelope(ctx, "/api/v1/analytics/accounts/list", body)
	if err != nil {
		return nil, err
	}
	return decodeAnalyticsAccounts(env.Data)
}

func (c *Client) AnalyticsData(ctx context.Context, body map[string]any) (*AnalyticsDataPage, error) {
	env, err := c.postEnvelope(ctx, "/api/v1/analytics/data", body)
	if err != nil {
		return nil, err
	}
	return decodeAnalyticsData(env.Data)
}

func (c *Client) MaterialGetUploadURL(ctx context.Context, body map[string]any) (*MaterialUploadURL, error) {
	env, err := c.postEnvelope(ctx, "/api/v1/material/getUploadUrl", body)
	if err != nil {
		return nil, err
	}
	var out MaterialUploadURL
	if err := remap(env.Data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) MaterialCreate(ctx context.Context, body map[string]any) (*Envelope, error) {
	return c.postEnvelope(ctx, "/api/v1/material/create", body)
}

func (c *Client) postDomain(ctx context.Context, path string, body any) ([]byte, error) {
	raw, _, err := c.do(ctx, http.MethodPost, path, body, true)
	return raw, err
}

func (c *Client) postEnvelope(ctx context.Context, path string, body any) (*Envelope, error) {
	raw, _, err := c.do(ctx, http.MethodPost, path, body, true)
	if err != nil {
		return nil, err
	}
	var env Envelope
	if err := common.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Code != 0 {
		return &env, wrapAPI(200, strconv.Itoa(env.Code), env.Msg, "", string(raw), ErrUpstream)
	}
	return &env, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, requireAuth bool) ([]byte, int, error) {
	if c == nil || c.BaseURL == "" {
		return nil, 0, ErrGatewayUnavailable
	}
	if requireAuth && c.APIKey == "" {
		return nil, 0, ErrGatewayUnavailable
	}
	var rdr io.Reader
	if body != nil {
		b, err := common.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if requireAuth {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, 0, wrapAPI(0, "", err.Error(), "", "", ErrTimeout)
		}
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw, resp.StatusCode, mapHTTPError(resp.StatusCode, raw)
	}
	return raw, resp.StatusCode, nil
}

func mapHTTPError(status int, raw []byte) error {
	text := strings.TrimSpace(string(raw))
	code, msg, phoneID := parseErrorFields(raw)
	if code == "" && looksLikeJSON(raw) {
		// numeric {"code":401,"msg":"..."}
		var n struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := common.Unmarshal(raw, &n); err == nil && n.Code != 0 {
			code = strconv.Itoa(n.Code)
			if msg == "" {
				msg = n.Msg
			}
		}
	}
	if msg == "" {
		msg = text
	}
	sentinel := classify(status, code)
	return wrapAPI(status, code, msg, phoneID, text, sentinel)
}

func parseErrorFields(raw []byte) (code, msg, phoneID string) {
	if !looksLikeJSON(raw) {
		return "", strings.TrimSpace(string(raw)), ""
	}
	var env struct {
		Code    any    `json:"code"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
		PhoneID string `json:"phoneId"`
	}
	if err := common.Unmarshal(raw, &env); err != nil {
		return "", strings.TrimSpace(string(raw)), ""
	}
	switch v := env.Code.(type) {
	case string:
		code = v
	case float64:
		if v != 0 {
			code = strconv.FormatInt(int64(v), 10)
		}
	}
	msg = env.Msg
	if msg == "" {
		msg = env.Message
	}
	return code, msg, env.PhoneID
}

func classify(status int, code string) error {
	switch {
	case status == http.StatusUnauthorized:
		return ErrUnauthorized
	case code == "DEVICE_NOT_MONTHLY":
		return ErrDeviceNotMonthly
	case code == "JOBS_REQUIRE_QUEUE":
		return ErrJobsRequired
	case code == "DEVICE_BUSY":
		return ErrDeviceBusy
	case status == http.StatusBadRequest:
		return ErrInvalidRequest
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return ErrTimeout
	default:
		return ErrUpstream
	}
}

func looksLikeJSON(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
}

func decodeJob(raw []byte) (*Job, error) {
	// jobs success may be {ok, data:{...}} or the job itself
	var wrap struct {
		OK   bool `json:"ok"`
		Data Job  `json:"data"`
		Job
	}
	if err := common.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	out := wrap.Data
	if out.JobID == "" {
		out = wrap.Job
	}
	return &out, nil
}

func decodeTaskAdd(data any) (*TaskAddResult, error) {
	out := &TaskAddResult{Raw: data}
	if data == nil {
		return out, nil
	}
	var box struct {
		TaskIDs []string `json:"taskIds"`
		TaskID  string   `json:"taskId"`
	}
	if err := remap(data, &box); err != nil {
		return out, err
	}
	out.TaskIDs = append([]string{}, box.TaskIDs...)
	if box.TaskID != "" {
		found := false
		for _, id := range out.TaskIDs {
			if id == box.TaskID {
				found = true
				break
			}
		}
		if !found {
			out.TaskIDs = append(out.TaskIDs, box.TaskID)
		}
	}
	return out, nil
}

func decodePhoneList(data any) (*PhoneListPage, error) {
	var page PhoneListPage
	if err := remap(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func decodeAnalyticsAccounts(data any) (*AnalyticsAccountsPage, error) {
	var page AnalyticsAccountsPage
	if err := remap(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func decodeAnalyticsData(data any) (*AnalyticsDataPage, error) {
	var raw struct {
		Items []struct {
			Account       string `json:"account"`
			Channel       int    `json:"channel"`
			PlayCount     int64  `json:"playCount"`
			FollowerCount int64  `json:"followerCount"`
			CommentCount  int64  `json:"commentCount"`
			DiggCount     int64  `json:"diggCount"`
			DataDate      int64  `json:"dataDate"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := remap(data, &raw); err != nil {
		return nil, err
	}
	page := &AnalyticsDataPage{Total: raw.Total, Items: make([]AnalyticsRow, 0, len(raw.Items))}
	for _, it := range raw.Items {
		row := AnalyticsRow{
			Account:       it.Account,
			Channel:       it.Channel,
			PlayCount:     it.PlayCount,
			FollowerCount: it.FollowerCount,
			CommentCount:  it.CommentCount,
			DiggCount:     it.DiggCount,
			DataDate:      it.DataDate,
		}
		if it.PlayCount == -1 {
			row.PlayCountStale = true
		}
		page.Items = append(page.Items, row)
	}
	return page, nil
}

func remap(in any, out any) error {
	if in == nil {
		return fmt.Errorf("hostedgateway: empty data")
	}
	b, err := common.Marshal(in)
	if err != nil {
		return err
	}
	return common.Unmarshal(b, out)
}
