package fal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/pkg/errors"
)

// FetchTask executes adaptive two-phase polling for fal.ai queue:
// 1. Calls GET {baseURL}/{endpoint}/requests/{request_id}/status
// 2. If COMPLETED, immediately calls GET {baseURL}/{endpoint}/requests/{request_id} to resolve final video URL
// 3. If 404 on result fetch, treats as IN_PROGRESS to tolerate CDN propagation latency
func (a *TaskAdaptor) FetchTask(baseUrl, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	body := map[string]any{
		"task_id":      task.GetUpstreamTaskID(),
		"action":       task.Action,
		"origin_model": task.Properties.OriginModelName,
	}
	return a.FetchTaskMap(baseUrl, key, body, proxy)
}

func (a *TaskAdaptor) FetchTaskMap(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	endpoint, reqID, err := ParseUpstreamTaskID(taskID)
	if err != nil {
		return nil, err
	}
	if endpoint == "" {
		if originModel, _ := body["origin_model"].(string); originModel != "" {
			endpoint = GetFalEndpoint(originModel)
		}
	}
	if endpoint == "" {
		endpoint = ModelMinimaxH3Max
	}
	endpoint = cleanFalQueueModel(endpoint)

	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		baseUrl = DefaultBaseURL
	}

	authKey := cleanAPIKey(key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}

	// Phase 1: Status check
	statusURL := fmt.Sprintf("%s/%s/requests/%s/status", baseUrl, endpoint, reqID)
	statusReq, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		return nil, err
	}
	statusReq.Header.Set("Accept", "application/json")
	statusReq.Header.Set("Authorization", "Key "+authKey)

	statusResp, err := client.Do(statusReq)
	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}

	if statusResp.StatusCode != http.StatusOK {
		return normalizePollingHTTP(statusResp, false)
	}

	statusBytes, err := io.ReadAll(statusResp.Body)
	_ = statusResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read status body failed: %w", err)
	}

	var statusData FalStatusResponse
	if err := common.Unmarshal(statusBytes, &statusData); err != nil {
		// If unmarshal fails, restore status response and let ParseTaskResult handle it
		statusResp.Body = io.NopCloser(bytes.NewReader(statusBytes))
		return statusResp, nil
	}

	// Phase 2: If COMPLETED, fetch the full result
	if strings.ToUpper(strings.TrimSpace(statusData.Status)) == StatusCompleted {
		resultURL := fmt.Sprintf("%s/%s/requests/%s", baseUrl, endpoint, reqID)
		resultReq, err := http.NewRequest(http.MethodGet, resultURL, nil)
		if err != nil {
			statusResp.Body = io.NopCloser(bytes.NewReader(statusBytes))
			return statusResp, nil
		}
		resultReq.Header.Set("Accept", "application/json")
		resultReq.Header.Set("Authorization", "Key "+authKey)

		resultResp, err := client.Do(resultReq)
		if err != nil {
			// Network error during result fetch; fallback to status response for retry next round
			statusResp.Body = io.NopCloser(bytes.NewReader(statusBytes))
			return statusResp, nil
		}

		if resultResp.StatusCode == http.StatusNotFound {
			// Upstream CDN replication delay, treat as IN_PROGRESS
			_ = resultResp.Body.Close()
			inProgressBody, _ := common.Marshal(map[string]any{
				"status": StatusInProgress,
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(inProgressBody)),
				Header:     statusResp.Header,
			}, nil
		}

		return normalizePollingHTTP(resultResp, true)
	}

	// Non-completed status (IN_QUEUE, IN_PROGRESS, FAILED)
	statusResp.Body = io.NopCloser(bytes.NewReader(statusBytes))
	return statusResp, nil
}

func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
	return a.parseTaskResultBody(respBody)
}

// parseTaskResultBody parses either a FalResultResponse or a FalStatusResponse into relaycommon.TaskInfo.
func (a *TaskAdaptor) parseTaskResultBody(respBody []byte) (*relaycommon.TaskInfo, error) {
	var payload struct {
		Status        string          `json:"status"`
		Video         *FalVideoFile   `json:"video"`
		Error         *FalErrorDetail `json:"error"`
		Detail        any             `json:"detail"`
		QueuePosition int             `json:"queue_position"`
		Logs          []any           `json:"logs"`
	}
	if err := common.Unmarshal(respBody, &payload); err != nil {
		return nil, errors.Wrap(err, "unmarshal fal task result")
	}

	info := &relaycommon.TaskInfo{Code: 0}

	// Explicit failures must not be hidden by a stale video field.
	if payload.Detail != nil && falErrorMessage(payload.Detail) != "" {
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure), Reason: falErrorMessage(payload.Detail)}, nil
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure), Reason: payload.Error.Message}, nil
	}
	status := strings.ToUpper(strings.TrimSpace(payload.Status))
	switch status {
	case StatusFailed, "FAILURE", "ERROR", "CANCELLED", "CANCELED":
		return &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure), Reason: "task generation failed"}, nil
	}
	// Only a usable HTTP(S) artifact establishes successful completion.
	if payload.Video != nil && validHTTPMediaURL(strings.TrimSpace(payload.Video.URL)) {
		videoURL := strings.TrimSpace(payload.Video.URL)
		info.Status = string(model.TaskStatusSuccess)
		info.Url = videoURL
		info.RemoteUrl = videoURL
		info.Progress = taskcommon.ProgressComplete
		return info, nil
	}

	// Map non-failure states without a usable artifact.
	switch status {
	case StatusCompleted:
		// Queue completion is not artifact availability; retry the result request.
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	case StatusInQueue, "QUEUED":
		info.Status = string(model.TaskStatusQueued)
		info.Progress = taskcommon.ProgressQueued
	case StatusInProgress, "RUNNING":
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	default:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	}

	return info, nil
}

func cleanAPIKey(key string) string {
	k := strings.TrimSpace(key)
	if strings.HasPrefix(strings.ToLower(k), "key ") {
		return strings.TrimSpace(k[4:])
	}
	if strings.HasPrefix(strings.ToLower(k), "bearer ") {
		return strings.TrimSpace(k[7:])
	}
	return k
}

func cleanFalQueueModel(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	for _, suffix := range []string{
		"/lip-sync/image-to-video",
		"/lip-sync",
		"/text-to-video",
		"/image-to-video",
		"/reference-to-video",
	} {
		if strings.HasSuffix(endpoint, suffix) {
			return strings.TrimSuffix(endpoint, suffix)
		}
	}
	return endpoint
}
