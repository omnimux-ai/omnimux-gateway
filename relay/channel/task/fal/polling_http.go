package fal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// normalizePollingHTTP preserves retryable HTTP failures before body-only parsing.
func normalizePollingHTTP(resp *http.Response, result bool) (*http.Response, error) {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	status := StatusFailed
	reason := ""
	if resp.StatusCode >= 500 || resp.StatusCode == 429 || resp.StatusCode == 408 || (result && resp.StatusCode == 404) {
		status = StatusInProgress
	} else {
		var payload map[string]any
		if common.Unmarshal(body, &payload) == nil {
			reason = falErrorMessage(payload["detail"])
			if reason == "" {
				reason = falErrorMessage(payload["error"])
			}
		}
		if reason == "" {
			reason = fmt.Sprintf("fal polling HTTP %d", resp.StatusCode)
		}
	}
	encoded, err := common.Marshal(map[string]any{"status": status, "detail": reason})
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Header: resp.Header, Body: io.NopCloser(bytes.NewReader(encoded))}, nil
}

func falErrorMessage(detail any) string {
	switch value := detail.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := falErrorMessage(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		message, _ := value["msg"].(string)
		if message == "" {
			message, _ = value["message"].(string)
		}
		if location, ok := value["loc"].([]any); ok {
			parts := make([]string, 0, len(location))
			for _, part := range location {
				parts = append(parts, fmt.Sprint(part))
			}
			message = strings.Join(parts, ".") + ": " + message
		}
		if kind, ok := value["type"].(string); ok && kind != "" {
			message += " (" + kind + ")"
		}
		if strings.TrimSpace(message) != "" {
			return strings.TrimSpace(message)
		}
	}
	return "fal returned an unrecognized error detail"
}
