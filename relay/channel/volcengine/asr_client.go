package volcengine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
)

// VolcengineASRClient 火山引擎 ASR 大模型 HTTP 交互客户端
type VolcengineASRClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewVolcengineASRClient 实例化客户端
func NewVolcengineASRClient(baseURL string, client *http.Client) *VolcengineASRClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://openspeech.bytedance.com"
	}
	return &VolcengineASRClient{
		httpClient: client,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// SubmitTask 提交 ASR 大模型异步识别任务
func (c *VolcengineASRClient) SubmitTask(ctx context.Context, apiKey string, resourceId string, req *VolcengineASRSubmitRequest) (*VolcengineASRSubmitResponse, error) {
	url := c.baseURL + PathAUCBigModelSubmit
	jsonData, err := common.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal submit request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create submit request failed: %w", err)
	}

	cleanKey := extractVolcengineApiKey(apiKey)
	reqId := uuid.New().String()

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", cleanKey)
	httpReq.Header.Set("X-Api-Resource-Id", resourceId)
	httpReq.Header.Set("X-Api-Request-Id", reqId)
	httpReq.Header.Set("X-Api-Sequence", "-1")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("submit request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read submit response failed: %w", err)
	}

	var submitResp VolcengineASRSubmitResponse
	if len(bodyBytes) > 0 {
		_ = common.Unmarshal(bodyBytes, &submitResp)
	}

	headerStatusCode := resp.Header.Get("X-Api-Status-Code")
	headerMessage := resp.Header.Get("X-Api-Message")
	if submitResp.StatusCode == "" {
		submitResp.StatusCode = headerStatusCode
	}
	if submitResp.Message == "" {
		submitResp.Message = headerMessage
	}

	if submitResp.StatusCode != "" && submitResp.StatusCode != StatusSuccess {
		return &submitResp, fmt.Errorf("upstream submit error [%s]: %s", submitResp.StatusCode, submitResp.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return &submitResp, fmt.Errorf("upstream submit returned http %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if submitResp.TaskId == "" {
		headerReqId := resp.Header.Get("X-Api-Request-Id")
		if headerReqId != "" {
			submitResp.TaskId = headerReqId
		} else {
			submitResp.TaskId = reqId
		}
	}

	return &submitResp, nil
}

// QueryTask 查询 ASR 任务转写进度与结果
func (c *VolcengineASRClient) QueryTask(ctx context.Context, apiKey string, resourceId string, taskId string) (*VolcengineASRQueryResponse, error) {
	url := c.baseURL + PathAUCBigModelQuery

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("create query request failed: %w", err)
	}

	cleanKey := extractVolcengineApiKey(apiKey)

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", cleanKey)
	httpReq.Header.Set("X-Api-Resource-Id", resourceId)
	httpReq.Header.Set("X-Api-Request-Id", taskId)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read query response failed: %w", err)
	}

	var queryResp VolcengineASRQueryResponse
	if len(bodyBytes) > 0 {
		_ = common.Unmarshal(bodyBytes, &queryResp)
	}

	headerStatusCode := resp.Header.Get("X-Api-Status-Code")
	headerMessage := resp.Header.Get("X-Api-Message")
	if queryResp.StatusCode == "" {
		queryResp.StatusCode = headerStatusCode
	}
	if queryResp.Message == "" {
		queryResp.Message = headerMessage
	}

	if resp.StatusCode != http.StatusOK && queryResp.StatusCode == "" {
		return nil, fmt.Errorf("upstream query returned http %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return &queryResp, nil
}
