package sora

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// grok2api / xAI Imagine Video uses POST /v1/videos/generations → {request_id}
// and GET /v1/videos/{request_id} with status pending|done|failed|expired.
// Type-1 OpenAI channels otherwise speak Sora POST /v1/videos.

type grokSubmitResponse struct {
	RequestID string `json:"request_id"`
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Error     *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type grokPollVideo struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
}

type grokPollResponse struct {
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	Model     string         `json:"model"`
	Video     *grokPollVideo `json:"video"`
	Error     *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func grokImagineName(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	for _, name := range []string{info.OriginModelName, info.GetUpstreamModelName()} {
		if isGrokImagineVideoName(name) {
			return name
		}
	}
	return ""
}

func isGrokImagineVideo(info *relaycommon.RelayInfo) bool {
	return grokImagineName(info) != ""
}

func isGrokImagineVideoName(name string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(name)), "grok-imagine-video")
}

func grokImagineUpstreamModel(name string) string {
	n := strings.TrimSpace(name)
	switch strings.ToLower(n) {
	case "grok-imagine-video-1-5":
		return "grok-imagine-video-1.5"
	default:
		return n
	}
}

func grokSubmitTaskID(resp grokSubmitResponse) string {
	for _, id := range []string{resp.RequestID, resp.ID, resp.TaskID} {
		if strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

func looksLikeGrokPoll(resp grokPollResponse) bool {
	if resp.Video != nil && strings.TrimSpace(resp.Video.URL) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(resp.Status)) {
	case "pending", "done", "failed", "expired":
		return true
	default:
		return false
	}
}

func parseGrokPoll(resp grokPollResponse) *relaycommon.TaskInfo {
	out := &relaycommon.TaskInfo{Code: 0}
	switch strings.ToLower(strings.TrimSpace(resp.Status)) {
	case "pending":
		out.Status = model.TaskStatusQueued
	case "done", "completed", "success":
		out.Status = model.TaskStatusSuccess
		// Url intentionally left empty so task_polling.go routes through taskcommon.BuildProxyURL(task.TaskID) -> controller.VideoProxy;
		// upstream returns container-internal URLs which must never reach clients.
		out.Url = ""
		out.Progress = taskcommon.ProgressComplete
	case "failed", "expired", "cancelled":
		out.Status = model.TaskStatusFailure
		if resp.Error != nil && strings.TrimSpace(resp.Error.Message) != "" {
			out.Reason = resp.Error.Message
		} else if strings.EqualFold(resp.Status, "expired") {
			out.Reason = "request expired"
		} else {
			out.Reason = "task failed"
		}
	default:
		out.Status = model.TaskStatusQueued
	}
	if resp.Progress > 0 && resp.Progress < 100 {
		out.Progress = fmt.Sprintf("%d%%", resp.Progress)
	} else if out.Status == model.TaskStatusSuccess {
		out.Progress = "100%"
	}
	return out
}
