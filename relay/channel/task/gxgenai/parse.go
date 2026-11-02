package gxgenai

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// 币多多 package (ops evidence): ¥169 → 72000 RH coins + ¥17 wallet.
// usdPerRHCoin = (169-17)/72000/FX — used only for settle when consumeCoins present.
// FX MUST follow operation_setting.USDExchangeRate (ops canonical: scripts/ops/fx.json).
// Precharge is NOT hardcoded per app: ops set ModelRatio from est coins (see skill).
const (
	rhCoinPackageCNY    = 169.0
	rhCoinPackageWallet = 17.0
	rhCoinPackageCoins  = 72000.0
)

// rhCNYPerUSD is 1 USD = X CNY. Sole live source: option USDExchangeRate.
func rhCNYPerUSD() float64 {
	rate := operation_setting.USDExchangeRate
	if rate <= 0 {
		return 6.8
	}
	return rate
}

// usdPerRHCoin converts one RH consumer coin to USD via 币多多 package + FX.
func usdPerRHCoin() float64 {
	return (rhCoinPackageCNY - rhCoinPackageWallet) / rhCoinPackageCoins / rhCNYPerUSD()
}

// Known default prompt→node mappings for catalog apps (override via metadata.nodeInfoList).
var defaultPromptNodes = map[string][]nodeInfo{
	"zimage-makeup": {
		{NodeID: "59", FieldName: "value"},
	},
	"zimage-makeup-async": {
		{NodeID: "59", FieldName: "value"},
	},
}

// RH query error codes that mean "not terminal — keep polling" (official docs).
// 804 still running, 813 queued; 1003/1005/1010/1011/1012 retry later.
// 1012 must not permanent-fail+refund while upstream still expects retry.
var transientQueryErrorCodes = map[int]struct{}{
	804:  {},
	813:  {},
	1003: {},
	1005: {},
	1010: {},
	1011: {},
	1012: {},
}

type nodeInfo struct {
	NodeID     string `json:"nodeId"`
	FieldName  string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

type runBody struct {
	NodeInfoList     []nodeInfo `json:"nodeInfoList"`
	UsePersonalQueue bool       `json:"usePersonalQueue"`
	// instanceType intentionally omitted from client path — ops-only cost surface.
}

type submitResponse struct {
	TaskID       string `json:"taskId"`
	Status       string `json:"status"`
	ErrorCode    any    `json:"errorCode"` // upstream may send "" or number
	ErrorMessage string `json:"errorMessage"`
	Data         *struct {
		TaskID string `json:"taskId"`
		Status string `json:"status"`
	} `json:"data"`
}

type queryResponse struct {
	TaskID                 string         `json:"taskId"`
	Status                 string         `json:"status"`
	ErrorCode              any            `json:"errorCode"`
	ErrorMessage           string         `json:"errorMessage"`
	FailedReason           any            `json:"failedReason"`
	TaskCostTime           any            `json:"taskCostTime"`
	Usage                  any            `json:"usage"`
	Results                []any          `json:"results"`
	PromptTips             any            `json:"promptTips"`
	NetWssUrl              string         `json:"netWssUrl"`
	ClientID               string         `json:"clientId"`
	ConsumeCoins           any            `json:"consumeCoins"`
	ConsumeMoney           any            `json:"consumeMoney"`
	ThirdPartyConsumeMoney any            `json:"thirdPartyConsumeMoney"`
	Data                   *queryResponse `json:"data"`
}

// usageSnapshot holds metered fields extracted for settlement (not all exposed to clients).
type usageSnapshot struct {
	Seconds                int
	ConsumeMoney           float64 // prefer as account currency major units
	ThirdPartyConsumeMoney float64
	ConsumeCoins           float64
	HasMoney               bool
	HasCoins               bool
}

func errorCodeNonZero(v any) bool {
	code, ok := parseErrorCode(v)
	return ok && code != 0
}

func parseErrorCode(v any) (int, bool) {
	switch t := v.(type) {
	case nil:
		return 0, true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, true
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return n, true
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" || s == "<nil>" {
			return 0, true
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}

func isTransientQueryError(v any) bool {
	code, ok := parseErrorCode(v)
	if !ok || code == 0 {
		return false
	}
	_, yes := transientQueryErrorCodes[code]
	return yes
}

func failedReasonString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, err := common.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		s := strings.TrimSpace(string(b))
		if s == "{}" || s == "null" {
			return ""
		}
		return s
	}
}

func isLikelyAppID(s string) bool {
	if len(s) < 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func extractNodeInfoList(metadata map[string]any) ([]nodeInfo, error) {
	if metadata == nil {
		return nil, nil
	}
	raw, ok := metadata["nodeInfoList"]
	if !ok || raw == nil {
		return nil, nil
	}
	b, err := common.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var nodes []nodeInfo
	if err := common.Unmarshal(b, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func buildRunBody(modelName string, req *relaycommon.TaskSubmitReq) (*runBody, error) {
	nodes, err := extractNodeInfoList(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("invalid metadata.nodeInfoList: %w", err)
	}
	if len(nodes) == 0 {
		defs := defaultPromptNodes[modelName]
		if len(defs) == 0 {
			base := strings.TrimSuffix(modelName, "-async")
			defs = defaultPromptNodes[base]
		}
		if len(defs) == 0 {
			return nil, fmt.Errorf("metadata.nodeInfoList is required for model %s", modelName)
		}
		prompt := strings.TrimSpace(req.Prompt)
		if prompt == "" || prompt == "gxgenai-task" {
			return nil, fmt.Errorf("prompt is required when nodeInfoList is omitted")
		}
		nodes = make([]nodeInfo, len(defs))
		for i, d := range defs {
			nodes[i] = nodeInfo{
				NodeID:     d.NodeID,
				FieldName:  d.FieldName,
				FieldValue: prompt,
			}
		}
	}

	// Security: never forward client instanceType / precharge / free-form queue flags
	// that change upstream cost without matching settle fields.
	return &runBody{
		NodeInfoList:     nodes,
		UsePersonalQueue: false,
	}, nil
}

func buildStandardModelBody(modelName string, req *relaycommon.TaskSubmitReq) (map[string]any, error) {
	out := make(map[string]any)

	// Forward user-supplied metadata fields (like aspectRatio, generateAudio, seed, etc.)
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			if k == "nodeInfoList" || k == "billingContext" || k == "instanceType" || k == "precharge_seconds" {
				continue
			}
			out[k] = v
		}
	}

	// Prompt
	prompt := strings.TrimSpace(req.Prompt)
	if prompt != "" && prompt != "gxgenai-task" {
		if _, exists := out["prompt"]; !exists {
			out["prompt"] = prompt
		}
	}

	// Images
	if len(req.Images) > 0 {
		if _, exists := out["imageUrls"]; !exists {
			out["imageUrls"] = req.Images
		}
		if _, exists := out["imageUrl"]; !exists {
			out["imageUrl"] = req.Images[0]
		}
	} else if req.Image != "" {
		if _, exists := out["imageUrl"]; !exists {
			out["imageUrl"] = req.Image
		}
		if _, exists := out["imageUrls"]; !exists {
			out["imageUrls"] = []string{req.Image}
		}
	}

	// Duration
	if req.Duration > 0 {
		if _, exists := out["duration"]; !exists {
			out["duration"] = fmt.Sprintf("%d", req.Duration)
		}
	}

	// Resolution / Size
	if req.Size != "" {
		if _, exists := out["resolution"]; !exists {
			out["resolution"] = req.Size
		}
	}

	return out, nil
}

func parseSubmitResponse(body []byte) (*submitResponse, error) {
	var resp submitResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.TaskID == "" && resp.Data != nil {
		resp.TaskID = resp.Data.TaskID
		if resp.Status == "" {
			resp.Status = resp.Data.Status
		}
	}
	if resp.TaskID == "" {
		var alt map[string]any
		if err := common.Unmarshal(body, &alt); err == nil {
			if v, ok := alt["task_id"].(string); ok {
				resp.TaskID = v
			}
		}
	}
	return &resp, nil
}

func parseQueryToTaskInfo(respBody []byte) (*relaycommon.TaskInfo, error) {
	var q queryResponse
	if err := common.Unmarshal(respBody, &q); err != nil {
		return nil, err
	}
	if q.Data != nil && q.Status == "" {
		// Nested envelope: merge usage-ish fields from outer if needed
		inner := *q.Data
		if inner.TaskCostTime == nil {
			inner.TaskCostTime = q.TaskCostTime
		}
		if inner.Usage == nil {
			inner.Usage = q.Usage
		}
		if inner.ConsumeMoney == nil {
			inner.ConsumeMoney = q.ConsumeMoney
		}
		if inner.ConsumeCoins == nil {
			inner.ConsumeCoins = q.ConsumeCoins
		}
		if inner.ThirdPartyConsumeMoney == nil {
			inner.ThirdPartyConsumeMoney = q.ThirdPartyConsumeMoney
		}
		q = inner
	}

	info := &relaycommon.TaskInfo{}

	// Transient upstream codes without status → keep polling (do NOT fail+refund).
	if errorCodeNonZero(q.ErrorCode) && strings.TrimSpace(q.Status) == "" {
		if isTransientQueryError(q.ErrorCode) {
			info.Status = string(model.TaskStatusInProgress)
			info.Progress = taskcommon.ProgressInProgress
			return info, nil
		}
		info.Status = string(model.TaskStatusFailure)
		info.Reason = q.ErrorMessage
		if info.Reason == "" {
			info.Reason = fmt.Sprintf("errorCode=%v", q.ErrorCode)
		}
		info.Progress = taskcommon.ProgressComplete
		return info, nil
	}

	status := strings.ToUpper(strings.TrimSpace(q.Status))
	switch status {
	case "SUCCESS", "SUCCEED", "SUCCEEDED", "COMPLETED", "DONE":
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		if url := firstResultURL(q.Results); url != "" {
			info.Url = url
			info.RemoteUrl = url
		}
	case "FAILED", "FAILURE", "ERROR", "CANCELLED", "CANCELED":
		info.Status = string(model.TaskStatusFailure)
		info.Progress = taskcommon.ProgressComplete
		info.Reason = failedReasonString(q.FailedReason)
		if info.Reason == "" {
			info.Reason = q.ErrorMessage
		}
	case "RUNNING", "PROCESSING", "IN_PROGRESS":
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	case "QUEUED", "CREATE", "CREATED", "PENDING", "WAITING", "":
		info.Status = string(model.TaskStatusSubmitted)
		info.Progress = taskcommon.ProgressSubmitted
	default:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	}

	snap := extractUsageSnapshot(&q)
	if snap.Seconds > 0 {
		info.CompletionTokens = snap.Seconds
	}
	// Do NOT encode money into TotalTokens. Framework settle falls back to
	// RecalculateTaskQuotaByTokens when AdjustBillingOnComplete returns 0; stuffing
	// currency micros there can charge free groups and mis-bill ModelRatio models.
	// Settlement reads consumeMoney/Coins from task.Data (poll stores raw query body).

	return info, nil
}

// sumConsumeMoney sums independent RH cost fields (runtime + third-party API).
// Official fields are separate line items; must not take max of the two.
func sumConsumeMoney(consumeMoney, thirdParty float64) float64 {
	total := 0.0
	if consumeMoney > 0 {
		total += consumeMoney
	}
	if thirdParty > 0 {
		total += thirdParty
	}
	return total
}

// moneyToUSD converts account major-unit money to USD for OmniMux quota math.
// runninghub.cn consumer wallets report CNY; runninghub.ai report USD.
// Unknown bases default to USD (conservative: do not over-convert).
func moneyToUSD(amount float64, channelBaseURL string) float64 {
	if amount <= 0 {
		return 0
	}
	base := strings.ToLower(strings.TrimSpace(channelBaseURL))
	if strings.Contains(base, "runninghub.cn") {
		return amount / rhCNYPerUSD()
	}
	return amount
}

// channelBaseURLForTask resolves channel base URL for currency inference at settle.
func channelBaseURLForTask(channelID int) string {
	if channelID <= 0 {
		return ""
	}
	ch, err := model.GetChannelById(channelID, true)
	if err != nil || ch == nil {
		return ""
	}
	return ch.GetBaseURL()
}

func extractUsageSnapshot(q *queryResponse) usageSnapshot {
	var snap usageSnapshot
	if sec, ok := anyToSeconds(q.TaskCostTime); ok {
		snap.Seconds = sec
	} else if sec, ok := extractSecondsFromUsage(q.Usage); ok {
		snap.Seconds = sec
	}
	if m, ok := anyToFloat(q.ConsumeMoney); ok && m > 0 {
		snap.ConsumeMoney = m
		snap.HasMoney = true
	}
	if m, ok := anyToFloat(q.ThirdPartyConsumeMoney); ok && m > 0 {
		snap.ThirdPartyConsumeMoney = m
		snap.HasMoney = true
	}
	if c, ok := anyToFloat(q.ConsumeCoins); ok && c > 0 {
		snap.ConsumeCoins = c
		snap.HasCoins = true
	}
	// usage object may nest the same fields
	if q.Usage != nil {
		b, err := common.Marshal(q.Usage)
		if err == nil {
			var m map[string]any
			if common.Unmarshal(b, &m) == nil {
				if !snap.HasMoney {
					if v, ok := anyToFloat(m["consumeMoney"]); ok && v > 0 {
						snap.ConsumeMoney = v
						snap.HasMoney = true
					}
					if v, ok := anyToFloat(m["thirdPartyConsumeMoney"]); ok && v > 0 {
						snap.ThirdPartyConsumeMoney = v
						snap.HasMoney = true
					}
				}
				if !snap.HasCoins {
					if v, ok := anyToFloat(m["consumeCoins"]); ok && v > 0 {
						snap.ConsumeCoins = v
						snap.HasCoins = true
					}
				}
				if snap.Seconds <= 0 {
					if sec, ok := anyToSeconds(m["taskCostTime"]); ok {
						snap.Seconds = sec
					}
				}
			}
		}
	}
	return snap
}

func firstResultURL(results []any) string {
	for _, r := range results {
		switch v := r.(type) {
		case string:
			if strings.HasPrefix(v, "http") {
				return v
			}
		case map[string]any:
			for _, key := range []string{"url", "fileUrl", "file_url", "output"} {
				if s, ok := v[key].(string); ok && strings.HasPrefix(s, "http") {
					return s
				}
			}
		}
	}
	return ""
}

func anyToSeconds(v any) (int, bool) {
	f, ok := anyToFloat(v)
	if !ok || f <= 0 {
		return 0, false
	}
	return int(f), true
}

func anyToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		s = strings.TrimSuffix(s, "s")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func extractSecondsFromUsage(usage any) (int, bool) {
	if usage == nil {
		return 0, false
	}
	b, err := common.Marshal(usage)
	if err != nil {
		return 0, false
	}
	var m map[string]any
	if err := common.Unmarshal(b, &m); err != nil {
		return 0, false
	}
	for _, key := range []string{"taskCostTime", "task_cost_time", "costTime", "duration", "seconds"} {
		if sec, ok := anyToSeconds(m[key]); ok {
			return sec, true
		}
	}
	return 0, false
}

func extractUsageFromTaskData(data []byte) usageSnapshot {
	if len(data) == 0 {
		return usageSnapshot{}
	}
	var q queryResponse
	if err := common.Unmarshal(data, &q); err != nil {
		return usageSnapshot{}
	}
	if q.Data != nil && q.Status == "" {
		inner := *q.Data
		if inner.TaskCostTime == nil {
			inner.TaskCostTime = q.TaskCostTime
		}
		if inner.Usage == nil {
			inner.Usage = q.Usage
		}
		if inner.ConsumeMoney == nil {
			inner.ConsumeMoney = q.ConsumeMoney
		}
		if inner.ConsumeCoins == nil {
			inner.ConsumeCoins = q.ConsumeCoins
		}
		if inner.ThirdPartyConsumeMoney == nil {
			inner.ThirdPartyConsumeMoney = q.ThirdPartyConsumeMoney
		}
		q = inner
	}
	return extractUsageSnapshot(&q)
}

func mapRHStatusToVideoStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS", "SUCCEED", "SUCCEEDED", "COMPLETED", "DONE":
		return "completed"
	case "FAILED", "FAILURE", "ERROR":
		return "failed"
	case "RUNNING", "PROCESSING":
		return "in_progress"
	default:
		return "queued"
	}
}

// redactQueryBodyForStorage strips fields that should not be shown to end users
// when task.Data is later returned via task fetch APIs.
func redactQueryBodyForStorage(body []byte) []byte {
	var m map[string]any
	if err := common.Unmarshal(body, &m); err != nil {
		return body
	}
	for _, key := range []string{
		"clientId", "client_id", "netWssUrl", "promptTips",
		"consumeCoins", "consumeMoney", "thirdPartyConsumeMoney",
		"usage", "taskUsageList",
	} {
		delete(m, key)
	}
	// Keep taskId/status/results/url for polling UX; strip nested data noise if present
	if data, ok := m["data"].(map[string]any); ok {
		for _, key := range []string{
			"clientId", "client_id", "netWssUrl", "promptTips",
			"consumeCoins", "consumeMoney", "thirdPartyConsumeMoney",
			"usage", "taskUsageList",
		} {
			delete(data, key)
		}
	}
	out, err := common.Marshal(m)
	if err != nil {
		return body
	}
	return out
}
