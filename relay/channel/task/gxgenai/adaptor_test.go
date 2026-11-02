package gxgenai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSubmitResponse(t *testing.T) {
	body := []byte(`{"taskId":"2084","status":"RUNNING","errorCode":0}`)
	r, err := parseSubmitResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "2084", r.TaskID)
	assert.Equal(t, "RUNNING", r.Status)
}

func TestParseSubmitResponseEmptyErrorCodeString(t *testing.T) {
	body := []byte(`{"taskId":"2084","status":"RUNNING","errorCode":"","errorMessage":""}`)
	r, err := parseSubmitResponse(body)
	require.NoError(t, err)
	assert.Equal(t, "2084", r.TaskID)
	assert.False(t, errorCodeNonZero(r.ErrorCode))
}

func TestParseQuerySuccess(t *testing.T) {
	body := []byte(`{
		"taskId":"2084",
		"status":"SUCCESS",
		"taskCostTime": 45,
		"consumeMoney": 0.02,
		"results":[{"url":"https://cdn.example.com/a.png"}]
	}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
	assert.Equal(t, "https://cdn.example.com/a.png", info.Url)
	assert.Equal(t, 45, info.CompletionTokens)
	// Money must NOT be stashed in TotalTokens (framework token-fallback poison).
	assert.Equal(t, 0, info.TotalTokens)
}

func TestParseQueryRunning(t *testing.T) {
	body := []byte(`{"taskId":"1","status":"RUNNING"}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), info.Status)
}

func TestParseQueryFailed(t *testing.T) {
	body := []byte(`{"status":"FAILED","failedReason":"oom"}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Equal(t, "oom", info.Reason)
}

func TestParseQueryTransientErrorCodesKeepPolling(t *testing.T) {
	cases := []string{
		`{"errorCode":804,"errorMessage":"still running"}`,
		`{"errorCode":813,"errorMessage":"queued"}`,
		`{"errorCode":1003,"errorMessage":"retry"}`,
		`{"errorCode":1005,"errorMessage":"retry"}`,
		`{"errorCode":"1010","errorMessage":"retry"}`,
		`{"errorCode":1011,"errorMessage":"retry"}`,
		`{"errorCode":1012,"errorMessage":"retry later"}`,
		`{"errorCode":"1012","errorMessage":"retry later"}`,
	}
	for _, raw := range cases {
		info, err := parseQueryToTaskInfo([]byte(raw))
		require.NoError(t, err, raw)
		assert.Equal(t, string(model.TaskStatusInProgress), info.Status, raw)
	}
}

func TestParseQueryPermanentErrorCodeFails(t *testing.T) {
	body := []byte(`{"errorCode":1004,"errorMessage":"task not found"}`)
	info, err := parseQueryToTaskInfo(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
}

func TestBuildRunBodyFromPrompt(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{Prompt: "hello portrait"}
	body, err := buildRunBody("zimage-makeup", req)
	require.NoError(t, err)
	require.Len(t, body.NodeInfoList, 1)
	assert.Equal(t, "59", body.NodeInfoList[0].NodeID)
	assert.Equal(t, "hello portrait", body.NodeInfoList[0].FieldValue)
	assert.False(t, body.UsePersonalQueue)
}

func TestBuildRunBodyIgnoresClientCostControls(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{
		Prompt: "x",
		Metadata: map[string]any{
			"instanceType":      "plus",
			"usePersonalQueue":  true,
			"precharge_seconds": 1,
			"nodeInfoList": []map[string]string{
				{"nodeId": "1", "fieldName": "image", "fieldValue": "a.png"},
			},
		},
	}
	body, err := buildRunBody("ltx2.3-kj", req)
	require.NoError(t, err)
	assert.False(t, body.UsePersonalQueue)
	data, err := common.Marshal(body)
	require.NoError(t, err)
	s := string(data)
	assert.NotContains(t, s, "plus")
	assert.NotContains(t, s, "precharge")
	assert.Contains(t, s, `"usePersonalQueue":false`)
}

func TestBuildRunBodyRequiresNodesForUnknownModel(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{Prompt: "x"}
	_, err := buildRunBody("unknown-app", req)
	require.Error(t, err)
}

func TestBuildRunBodyExplicitNodes(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{
		Metadata: map[string]any{
			"nodeInfoList": []map[string]string{
				{"nodeId": "1", "fieldName": "image", "fieldValue": "a.png"},
			},
		},
	}
	body, err := buildRunBody("ltx2.3-kj", req)
	require.NoError(t, err)
	require.Len(t, body.NodeInfoList, 1)
	assert.Equal(t, "a.png", body.NodeInfoList[0].FieldValue)
}

func TestIsLikelyAppID(t *testing.T) {
	assert.True(t, isLikelyAppID("2003681895185563650"))
	assert.False(t, isLikelyAppID("zimage-makeup"))
	assert.False(t, isLikelyAppID("123"))
}

func TestBuildRequestURL(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://www.runninghub.cn"}

	// Numeric App ID -> AI App path
	u, err := a.BuildRequestURL(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "2003681895185563650"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://www.runninghub.cn/openapi/v2/run/ai-app/2003681895185563650", u)

	// Standard model slug -> Standard model path
	u, err = a.BuildRequestURL(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "alibaba/wan-3.0/reference-to-video"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://www.runninghub.cn/openapi/v2/alibaba/wan-3.0/reference-to-video", u)

	// Slug with leading slash is normalized
	u, err = a.BuildRequestURL(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "/minimax/hailuo-h3/text-to-video"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://www.runninghub.cn/openapi/v2/minimax/hailuo-h3/text-to-video", u)

	// Fallback to OriginModelName if ChannelMeta is nil or UpstreamModelName is empty
	u, err = a.BuildRequestURL(&relaycommon.RelayInfo{
		OriginModelName: "alibaba/wan-3.0/text-to-video",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://www.runninghub.cn/openapi/v2/alibaba/wan-3.0/text-to-video", u)

	// Invalid target without '/' or numeric App ID fails cleanly
	_, err = a.BuildRequestURL(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "plain-name-without-slash"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid gxgenai model mapping")
}

func TestBuildStandardModelBody(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{
		Prompt:   "A cinematic cyber city",
		Duration: 5,
		Size:     "1080P",
		Images:   []string{"https://example.com/ref1.jpg", "https://example.com/ref2.jpg"},
		Metadata: map[string]any{
			"aspectRatio":       "16:9",
			"generateAudio":     true,
			"instanceType":      "pro",    // banned, should be filtered
			"precharge_seconds": 10,       // banned, should be filtered
			"billingContext":    "secret", // internal, should be filtered
		},
	}

	body, err := buildStandardModelBody("wan-3.0", req)
	require.NoError(t, err)
	assert.Equal(t, "A cinematic cyber city", body["prompt"])
	assert.Equal(t, "5", body["duration"])
	assert.Equal(t, "1080P", body["resolution"])
	assert.Equal(t, "16:9", body["aspectRatio"])
	assert.Equal(t, true, body["generateAudio"])
	assert.Equal(t, "https://example.com/ref1.jpg", body["imageUrl"])
	assert.Equal(t, []string{"https://example.com/ref1.jpg", "https://example.com/ref2.jpg"}, body["imageUrls"])
	assert.Nil(t, body["instanceType"])
	assert.Nil(t, body["precharge_seconds"])
	assert.Nil(t, body["billingContext"])
}

func TestEstimateBillingUsesUnitsOne(t *testing.T) {
	a := &TaskAdaptor{}
	ratios := a.EstimateBilling(nil, nil)
	require.Equal(t, 1.0, ratios["units"])
	_, hasSec := ratios["seconds"]
	require.False(t, hasSec, "must not use wall-clock seconds for precharge")
}

func TestAdjustBillingOnCompleteCoinsPath(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 0.02,
				GroupRatio: 1.0,
			},
		},
		Data: []byte(`{"status":"SUCCESS","consumeCoins":17}`),
	}
	q := a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{})
	// 17 coins * usdPerRHCoin() * 2 margin * 500000
	// usdPerRHCoin = (169-17)/72000/USDExchangeRate; default FX 6.8 → ≈0.00031046
	assert.Greater(t, q, 0)
	// 17 * 0.00031046 * 2 * 500000 ≈ 5278
	assert.InDelta(t, 5278, q, 200)
}

func TestAdjustBillingOnCompleteNoUsageKeepsPrecharge(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{ModelRatio: 0.02, GroupRatio: 1.0},
		},
		Data: []byte(`{"status":"SUCCESS"}`),
	}
	assert.Equal(t, 0, a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{CompletionTokens: 100}))
}

func TestAdjustBillingOnCompleteFreeGroupStaysFree(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 1.0,
				GroupRatio: 0,
			},
		},
		Data: []byte(`{"status":"SUCCESS","taskCostTime":100,"consumeMoney":1.5}`),
	}
	info := &relaycommon.TaskInfo{CompletionTokens: 100, TotalTokens: 1_500_000}
	q := a.AdjustBillingOnComplete(task, info)
	assert.Equal(t, 0, q)
}

func TestAdjustBillingOnCompleteMoneyWithMargin(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		// channel 0 → moneyToUSD defaults to USD
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{ModelRatio: 0.02, GroupRatio: 1.0},
		},
		Data: []byte(`{"status":"SUCCESS","consumeMoney":0.01}`),
	}
	q := a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{})
	// 0.01 USD * 2 margin * 500000 = 10000
	assert.Equal(t, 10000, q)
}

func TestSumConsumeMoneyAddsIndependentLines(t *testing.T) {
	assert.Equal(t, 0.03, sumConsumeMoney(0.01, 0.02))
	assert.Equal(t, 0.02, sumConsumeMoney(0, 0.02))
	assert.Equal(t, 0.01, sumConsumeMoney(0.01, 0))
	assert.Equal(t, 0.0, sumConsumeMoney(0, 0))
	// negatives ignored
	assert.Equal(t, 0.05, sumConsumeMoney(0.05, -1))
}

func TestMoneyToUSDCNYVsUSD(t *testing.T) {
	// Default USDExchangeRate 6.8 CNY → 1 USD
	assert.InDelta(t, 1.0, moneyToUSD(6.8, "https://www.runninghub.cn"), 1e-9)
	assert.InDelta(t, 1.0, moneyToUSD(6.8, "https://www.runninghub.cn/openapi"), 1e-9)
	// .ai stays USD
	assert.Equal(t, 0.5, moneyToUSD(0.5, "https://www.runninghub.ai"))
	assert.Equal(t, 0.5, moneyToUSD(0.5, ""))
}

func TestAdjustBillingOnCompleteSumsMoneyLines(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{ModelRatio: 0.02, GroupRatio: 1.0},
		},
		// 0.01 + 0.02 = 0.03 USD * 2 * 500000 = 30000 (not max=0.02 → 20000)
		Data: []byte(`{"status":"SUCCESS","consumeMoney":0.01,"thirdPartyConsumeMoney":0.02}`),
	}
	assert.Equal(t, 30000, a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{}))
}

func TestAdjustBillingIgnoresTotalTokensMoneyHack(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{ModelRatio: 0.02, GroupRatio: 1.0},
		},
		Data: []byte(`{"status":"SUCCESS"}`),
	}
	// Legacy path stuffed micros into TotalTokens; must not bill from that alone.
	assert.Equal(t, 0, a.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{TotalTokens: 1_500_000}))
}

func TestRedactQueryBodyForStorage(t *testing.T) {
	raw := []byte(`{"taskId":"1","status":"SUCCESS","clientId":"secret","consumeMoney":0.1,"results":[{"url":"https://x"}]}`)
	out := redactQueryBodyForStorage(raw)
	s := string(out)
	assert.Contains(t, s, "taskId")
	assert.NotContains(t, s, "clientId")
	assert.NotContains(t, s, "consumeMoney")
}
