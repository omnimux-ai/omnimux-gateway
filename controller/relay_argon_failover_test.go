package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const argonNativeTestRequest = `{"contents":[{"role":"user","parts":[{"text":"Replace the object using every reference, keep the original action."},{"inlineData":{"mimeType":"video/mp4","data":"VklERU8="}},{"inlineData":{"mimeType":"image/png","data":"SU1BR0Ux"}},{"inlineData":{"mimeType":"image/png","data":"SU1BR0Uy"}}]}],"systemInstruction":{"parts":[{"text":"Return one object-swap prompt. User intent has priority within object replacement."}]},"generationConfig":{"maxOutputTokens":32,"thinkingConfig":{"thinkingLevel":"low"}}}`
const argonNativeAuthDenial = `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`
const argonNativeTestSuccess = `{"candidates":[{"content":{"role":"model","parts":[{"text":"approved replacement prompt"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5,"thoughtsTokenCount":2,"totalTokenCount":27},"modelVersion":"gemini-4-argon"}`

type argonNativeTestResult struct {
	c                              *gin.Context
	response                       *httptest.ResponseRecorder
	order                          []int
	bodies                         []string
	keys                           []string
	quota, usedQuota, requestCount int
}

// This calls the actual Relay, native Gemini adaptor, native error decoder,
// channel selector and billing path against four controlled HTTP suppliers.
// No real key, remote supplier or production configuration is involved.
func runArgonNativeRelay(t *testing.T, modelName, group string, stream bool, actions [4]string, setup func(*gin.Context)) argonNativeTestResult {
	t.Helper()
	require.NoError(t, i18n.Init())
	var mutex sync.Mutex
	var order []int
	var bodies []string
	var keys []string
	var servers [4]*httptest.Server
	for index := range servers {
		servers[index] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			key := "fixture-key-" + strconv.Itoa(index)
			if actions[index] == "polling-success" {
				assert.Contains(t, []string{key + "-A", key + "-B"}, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
				assert.Equal(t, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), r.Header.Get("x-goog-api-key"))
			} else {
				assert.Equal(t, "Bearer "+key, r.Header.Get("Authorization"))
				assert.Equal(t, key, r.Header.Get("x-goog-api-key"))
			}
			assert.Equal(t, http.MethodPost, r.Method)
			wantPath := "/v1beta/models/" + modelName + ":generateContent"
			if stream {
				wantPath = "/v1beta/models/" + modelName + ":streamGenerateContent"
			}
			assert.Equal(t, wantPath, r.URL.Path)
			mutex.Lock()
			order = append(order, 100-index*10)
			bodies = append(bodies, string(body))
			keys = append(keys, r.Header.Get("x-goog-api-key"))
			mutex.Unlock()
			w.Header().Set("Content-Type", "application/json")
			switch actions[index] {
			case "auth":
				w.WriteHeader(401)
				_, _ = io.WriteString(w, argonNativeAuthDenial)
			case "evoauth":
				w.WriteHeader(401)
				_, _ = io.WriteString(w, `{"error":{"code":"unauthorized","message":"Invalid API key (request id: fixture-123)","param":null,"type":"authentication_error"}}`)
			case "500", "mapped500":
				w.WriteHeader(500)
				_, _ = io.WriteString(w, `{"error":{"code":500,"message":"upstream failed","status":"INTERNAL"}}`)
			case "429":
				w.WriteHeader(429)
				_, _ = io.WriteString(w, `{"error":{"code":429,"message":"limit","status":"RESOURCE_EXHAUSTED"}}`)
			case "usage401":
				w.WriteHeader(401)
				_, _ = io.WriteString(w, `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"},"usageMetadata":{"totalTokenCount":2}}`)
			case "candidates401":
				w.WriteHeader(401)
				_, _ = io.WriteString(w, `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"},"candidates":[{"content":{"parts":[{"text":"already generated"}]}}]}`)
			case "partial401":
				w.Header().Set("Content-Length", strconv.Itoa(len(argonNativeAuthDenial)+15))
				w.WriteHeader(401)
				_, _ = io.WriteString(w, argonNativeAuthDenial)
			case "transport":
				conn, _, err := w.(http.Hijacker).Hijack()
				assert.NoError(t, err)
				if err == nil {
					_ = conn.Close()
				}
			case "timeout":
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
				}
			case "malformed200":
				_, _ = io.WriteString(w, `{"candidates":[`)
			case "partial-stream":
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Content-Length", "10000")
				_, _ = io.WriteString(w, "data: "+argonNativeTestSuccess+"\n\n")
				w.(http.Flusher).Flush()
			default:
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: "+argonNativeTestSuccess+"\n\n")
				} else {
					_, _ = io.WriteString(w, argonNativeTestSuccess)
				}
			}
		}))
		t.Cleanup(servers[index].Close)
	}

	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Log{})
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMemory, oldBatch, oldConsume := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	oldRetry, oldCount, oldErrorLog, oldAutoDisable := common.RetryTimes, constant.CountToken, constant.ErrorLogEnabled, common.AutomaticDisableChannelEnabled
	oldStreamTimeout := constant.StreamingTimeout
	oldRatios := ratio_setting.GroupRatio2JSONString()
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, true, false, false
	common.RetryTimes, constant.CountToken, constant.ErrorLogEnabled, common.AutomaticDisableChannelEnabled = 1, false, false, false
	constant.StreamingTimeout = 30
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(fmt.Sprintf(`{%q:1}`, group)))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldMemory, oldBatch, oldConsume
		common.RetryTimes, constant.CountToken, constant.ErrorLogEnabled, common.AutomaticDisableChannelEnabled = oldRetry, oldCount, oldErrorLog, oldAutoDisable
		constant.StreamingTimeout = oldStreamTimeout
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		if oldMemory && oldDB != nil && oldDB.Migrator().HasTable(&model.Channel{}) && oldDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
	})
	withTieredBillingConfig(t, map[string]string{modelName: "tiered_expr"}, map[string]string{modelName: `p*4+c*20`})
	user := model.User{Username: "argon-native", AffCode: "argon-native", Quota: 1000000, Group: "default", Status: common.UserStatusEnabled, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "argon-native-test", Name: "argon-native", Status: common.TokenStatusEnabled, UnlimitedQuota: true, Group: group}
	require.NoError(t, db.Create(&token).Error)
	var channels [4]model.Channel
	for index := range channels {
		channels[index] = model.Channel{Name: fmt.Sprintf("argon-%d", index), Type: constant.ChannelTypeSub2API, Status: common.ChannelStatusEnabled, Key: fmt.Sprintf("fixture-key-%d", index), Models: modelName, Group: group, BaseURL: common.GetPointer(servers[index].URL), Priority: common.GetPointer(int64(100 - index*10))}
		if actions[index] == "no-key" {
			channels[index].ChannelInfo.IsMultiKey = true
			channels[index].ChannelInfo.MultiKeyStatusList = map[int]int{0: common.ChannelStatusAutoDisabled}
		}
		if actions[index] == "polling-success" {
			channels[index].Key = fmt.Sprintf("fixture-key-%d-A\nfixture-key-%d-B", index, index)
			channels[index].ChannelInfo.IsMultiKey = true
			channels[index].ChannelInfo.MultiKeyMode = constant.MultiKeyModePolling
		}
		if actions[index] == "mapped500" {
			channels[index].StatusCodeMapping = common.GetPointer(`{"500":"401"}`)
		}
		require.NoError(t, db.Create(&channels[index]).Error)
		require.NoError(t, db.Create(&model.Ability{Group: group, Model: modelName, ChannelId: channels[index].Id, Enabled: true, Priority: channels[index].Priority}).Error)
	}
	model.InitChannelCache()
	path := "/v1beta/models/" + modelName + ":generateContent"
	if stream {
		path = "/v1beta/models/" + modelName + ":streamGenerateContent?alt=sse"
	}
	service.InitHttpClient()
	if actions[0] == "timeout" {
		service.GetHttpClient().Timeout = 100 * time.Millisecond
	}
	requestCount := 1
	if actions[0] == "polling-success" {
		requestCount = 2
	}
	var w *httptest.ResponseRecorder
	var c *gin.Context
	for range requestCount {
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(argonNativeTestRequest))
		c.Request.Header.Set("Content-Type", "application/json")
		user.ToBaseUser().WriteContext(c)
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
		common.SetContextKey(c, constant.ContextKeyTokenKey, token.Key)
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
		common.SetContextKey(c, common.RequestIdKey, "argon-native-controlled-test")
		initialChannel, err := model.CacheGetChannel(channels[0].Id)
		require.NoError(t, err)
		initialSetupErr := middleware.SetupContextForSelectedChannel(c, initialChannel, modelName)
		if actions[0] == "no-key" {
			// Distribute currently ignores this local error before it calls Relay.
			// Reproduce that actual boundary rather than treating it as rejection.
			require.NotNil(t, initialSetupErr)
			require.Equal(t, types.ErrorCodeChannelNoAvailableKey, initialSetupErr.GetErrorCode())
		} else {
			require.Nil(t, initialSetupErr)
		}
		if setup != nil {
			setup(c)
		}
		Relay(c, types.RelayFormatGemini)
		if requestCount == 2 {
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Len(t, c.GetStringSlice("use_channel"), 1)
			assert.Equal(t, 1, service.RequestPolicy(c).Attempts)
		}
	}
	assert.Equal(t, 1, common.RetryTimes, "the global retry setting must not change")
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	return argonNativeTestResult{c: c, response: w, order: order, bodies: bodies, keys: keys, quota: after.Quota, usedQuota: after.UsedQuota, requestCount: after.RequestCount}
}

func TestArgonNativeRelayFourDistinctPrioritiesWithGlobalOne(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, test := range []struct {
			name    string
			actions [4]string
			want    []int
			success bool
		}{
			{"first-success", [4]string{"success", "success", "success"}, []int{100}, true},
			{"second-success", [4]string{"auth", "success", "success"}, []int{100, 90}, true},
			{"third-success", [4]string{"auth", "evoauth", "success"}, []int{100, 90, 80}, true},
			{"fourth-success", [4]string{"auth", "evoauth", "auth", "success"}, []int{100, 90, 80, 70}, true},
			{"all-denied", [4]string{"auth", "auth", "evoauth", "auth"}, []int{100, 90, 80, 70}, false},
		} {
			t.Run(fmt.Sprintf("%s/stream=%t", test.name, stream), func(t *testing.T) {
				result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", stream, test.actions, nil)
				assert.Equal(t, test.want, result.order)
				assert.Equal(t, len(test.want), service.RequestPolicy(result.c).Attempts)
				assert.Len(t, result.c.GetStringSlice("use_channel"), len(test.want))
				for index, body := range result.bodies {
					assert.JSONEq(t, argonNativeTestRequest, body, fmt.Sprintf("attempt %d preserves complete native multimodal input", index+1))
				}
				if test.success {
					assert.Equal(t, 200, result.response.Code, result.response.Body.String())
					assert.Contains(t, result.response.Body.String(), "approved replacement prompt")
					assert.Equal(t, 999890, result.quota)
					assert.Equal(t, 110, result.usedQuota)
					assert.Equal(t, 1, result.requestCount)
				} else {
					assert.Equal(t, 401, result.response.Code)
					assert.Equal(t, 1000000, result.quota)
					assert.Zero(t, result.usedQuota)
					assert.Zero(t, result.requestCount)
				}
			})
		}
	}
}

func TestArgonNativeRelayUnknownAcceptanceNeverReplays(t *testing.T) {
	for _, test := range []struct {
		name    string
		actions [4]string
		setup   func(*gin.Context)
		want    []int
		stream  bool
	}{
		{name: "transport", actions: [4]string{"transport", "success", "success"}, want: []int{100}},
		{name: "timeout", actions: [4]string{"timeout", "success", "success"}, want: []int{100}},
		{name: "server", actions: [4]string{"500", "success", "success"}, want: []int{100}},
		{name: "rate", actions: [4]string{"429", "success", "success"}, want: []int{100}},
		{name: "partial-auth-body", actions: [4]string{"partial401", "success", "success"}, want: []int{100}},
		{name: "usage-in-denial", actions: [4]string{"usage401", "success", "success"}, want: []int{100}},
		{name: "candidate-in-denial", actions: [4]string{"candidates401", "success", "success"}, want: []int{100}},
		{name: "malformed-success", actions: [4]string{"malformed200", "success", "success"}, want: []int{100}},
		{name: "partial-stream", actions: [4]string{"partial-stream", "success", "success"}, want: []int{100}, stream: true},
		{name: "evidence-reset", actions: [4]string{"auth", "500", "success"}, want: []int{100, 90}},
		{name: "status-mapping", actions: [4]string{"mapped500", "success", "success"}, want: []int{100}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", test.stream, test.actions, test.setup)
			assert.Equal(t, test.want, result.order)
			if test.name == "status-mapping" {
				assert.Equal(t, http.StatusUnauthorized, result.response.Code, "the channel mapping must actually apply")
			}
		})
	}
}

func TestArgonNativeRelayLaterUnknownAcceptanceStopsBeforeRemainingRoutes(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, test := range []struct {
			name    string
			actions [4]string
			want    []int
		}{
			{"second-server-error", [4]string{"auth", "500", "success", "success"}, []int{100, 90}},
			{"third-server-error", [4]string{"auth", "evoauth", "500", "success"}, []int{100, 90, 80}},
			{"second-transport-error", [4]string{"auth", "transport", "success", "success"}, []int{100, 90}},
			{"third-transport-error", [4]string{"auth", "evoauth", "transport", "success"}, []int{100, 90, 80}},
		} {
			t.Run(fmt.Sprintf("%s/stream=%t", test.name, stream), func(t *testing.T) {
				result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", stream, test.actions, nil)
				assert.Equal(t, test.want, result.order)
				assert.Equal(t, len(test.want), service.RequestPolicy(result.c).Attempts)
				events := service.RequestPolicy(result.c).Events()
				require.NotEmpty(t, events)
				assert.Equal(t, "acceptance_unknown", events[len(events)-1].Decision.Reason)
				assert.Equal(t, 1000000, result.quota)
				assert.Zero(t, result.usedQuota)
				assert.Zero(t, result.requestCount)
			})
		}
	}
}

func TestArgonNativeRelayPreservesSequentialPollingKeys(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", stream, [4]string{"polling-success", "success", "success"}, nil)
			assert.Equal(t, []string{"fixture-key-0-A", "fixture-key-0-B"}, result.keys)
			assert.Equal(t, []int{100, 100}, result.order, "two independent successful requests use only their primary")
			assert.Equal(t, 999780, result.quota)
			assert.Equal(t, 220, result.usedQuota)
			assert.Equal(t, 2, result.requestCount)
		})
	}
}

func TestArgonNativeRelayPreservesOldBudgetAndPins(t *testing.T) {
	for _, test := range []struct {
		name, model, group string
		setup              func(*gin.Context)
		want               []int
	}{
		{name: "old-model", model: "gemini-3.8-flash", group: "gemini-4-argon-multimodal", want: []int{100, 90}},
		{name: "other-group", model: "gemini-4-argon", group: "default", want: []int{100, 90}},
		{name: "single-pin", model: "gemini-4-argon", group: "gemini-4-argon-multimodal", want: []int{100}, setup: func(c *gin.Context) {
			service.GetChannelConstraints(c).AddPin(appdto.ChannelPin{ChannelId: c.GetInt("channel_id"), Source: appdto.PinSourceToken, RetryMode: appdto.PinRetrySingleAttempt})
		}},
		{name: "same-channel-pin", model: "gemini-4-argon", group: "gemini-4-argon-multimodal", want: []int{100}, setup: func(c *gin.Context) {
			service.GetChannelConstraints(c).AddPin(appdto.ChannelPin{ChannelId: c.GetInt("channel_id"), Source: appdto.PinSourceOriginTask, RetryMode: appdto.PinRetrySameChannel})
		}},
		{name: "strict-session", model: "gemini-4-argon", group: "gemini-4-argon-multimodal", want: []int{100}, setup: func(c *gin.Context) {
			c.Set("channel_affinity_skip_retry_on_failure", true)
			service.RequestPolicy(c).SessionMode = "strict"
		}},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", test.name, stream), func(t *testing.T) {
				result := runArgonNativeRelay(t, test.model, test.group, stream, [4]string{"auth", "auth", "success", "success"}, test.setup)
				assert.Equal(t, test.want, result.order)
				assert.Equal(t, len(test.want), service.RequestPolicy(result.c).Attempts)
			})
		}
	}
}

func TestArgonNativeRelayAutoGroupFromDistributorStaysBounded(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, final := range []string{"success", "auth"} {
			t.Run(fmt.Sprintf("stream=%t/final=%s", stream, final), func(t *testing.T) {
				result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", stream, [4]string{"auth", "auth", "evoauth", final}, func(c *gin.Context) {
					oldUsable, oldRatios := setting.UserUsableGroups2JSONString(), ratio_setting.GroupRatio2JSONString()
					oldLimit := setting.GetMaxTokenAutoGroups()
					t.Cleanup(func() {
						require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
						require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
						require.NoError(t, setting.UpdateMaxTokenAutoGroups(strconv.Itoa(oldLimit)))
					})
					require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gemini-4-argon-multimodal":"Argon","other-group":"Other"}`))
					require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gemini-4-argon-multimodal":1,"other-group":1}`))
					require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))
					primary, err := model.CacheGetChannel(c.GetInt("channel_id"))
					require.NoError(t, err)
					outside := model.Channel{Name: "outside-group", Type: primary.Type, Status: common.ChannelStatusEnabled, Key: primary.Key, Models: primary.Models, Group: "other-group", BaseURL: primary.BaseURL, Priority: common.GetPointer(int64(200))}
					require.NoError(t, model.DB.Create(&outside).Error)
					require.NoError(t, model.DB.Create(&model.Ability{Group: outside.Group, Model: outside.Models, ChannelId: outside.Id, Enabled: true, Priority: outside.Priority}).Error)
					model.InitChannelCache()
					common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")
					common.SetContextKey(c, constant.ContextKeyTokenGroup, "auto")
					common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, []string{"gemini-4-argon-multimodal", "other-group"})
					common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, true)
					middleware.Distribute()(c)
					require.False(t, c.IsAborted(), c.Writer.Status())
					assert.Equal(t, "auto", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
					assert.Equal(t, "gemini-4-argon-multimodal", common.GetContextKeyString(c, constant.ContextKeyAutoGroup))
					assert.Equal(t, "auto", relaycommon.GenRelayInfoGemini(c, nil).UsingGroup, "the real distributor leaves normalization to request billing")
				})
				assert.Equal(t, []int{100, 90, 80, 70}, result.order)
				assert.True(t, service.ArgonNativeRetryConfigured(result.c))
				assert.Equal(t, 4, service.RequestPolicy(result.c).Attempts)
				for _, event := range service.RequestPolicy(result.c).Events() {
					assert.Equal(t, "gemini-4-argon-multimodal", event.Group)
				}
				if final == "success" {
					assert.Equal(t, http.StatusOK, result.response.Code, result.response.Body.String())
					assert.Equal(t, 110, result.usedQuota)
					assert.Equal(t, 1, result.requestCount)
				} else {
					assert.Equal(t, http.StatusUnauthorized, result.response.Code, result.response.Body.String())
					assert.Zero(t, result.usedQuota)
					assert.Zero(t, result.requestCount)
				}
			})
		}
	}
}

func TestArgonNativeRelayCountsLocalUnsentCandidate(t *testing.T) {
	local := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", false, [4]string{"auth", "no-key", "success"}, nil)
	assert.Equal(t, []int{100, 80}, local.order, "the unsent candidate occupies the middle slot")
	assert.Len(t, local.c.GetStringSlice("use_channel"), 3)
	assert.Equal(t, 3, service.RequestPolicy(local.c).Attempts)
	assert.Equal(t, 999890, local.quota)
	assert.Equal(t, 110, local.usedQuota)
	assert.Equal(t, 1, local.requestCount)
}

func TestArgonNativeRelayInitialUnsentKeyFailureUsesRemainingCandidates(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, test := range []struct {
			name      string
			actions   [4]string
			wantOrder []int
			attempts  int
			success   bool
			setup     func(*gin.Context)
		}{
			{name: "primary-no-key", actions: [4]string{"no-key", "success", "success"}, wantOrder: []int{90}, attempts: 2, success: true},
			{name: "primary-no-key-with-stale-context", actions: [4]string{"no-key", "success", "success"}, wantOrder: []int{90}, attempts: 2, success: true, setup: func(c *gin.Context) {
				channel, err := model.CacheGetChannel(c.GetInt("channel_id"))
				require.NoError(t, err)
				common.SetContextKey(c, constant.ContextKeyChannelKey, "fixture-key-0")
				common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
				common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
				common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
			}},
			{name: "first-two-no-key", actions: [4]string{"no-key", "no-key", "success"}, wantOrder: []int{80}, attempts: 3, success: true},
			{name: "first-three-no-key", actions: [4]string{"no-key", "no-key", "no-key", "success"}, wantOrder: []int{70}, attempts: 4, success: true},
			{name: "all-no-key", actions: [4]string{"no-key", "no-key", "no-key", "no-key"}, attempts: 4},
			{name: "single-pin", actions: [4]string{"no-key", "success", "success"}, attempts: 1, setup: func(c *gin.Context) {
				service.GetChannelConstraints(c).AddPin(appdto.ChannelPin{ChannelId: c.GetInt("channel_id"), Source: appdto.PinSourceToken, RetryMode: appdto.PinRetrySingleAttempt})
			}},
			{name: "same-channel-pin", actions: [4]string{"no-key", "success", "success"}, attempts: 1, setup: func(c *gin.Context) {
				service.GetChannelConstraints(c).AddPin(appdto.ChannelPin{ChannelId: c.GetInt("channel_id"), Source: appdto.PinSourceOriginTask, RetryMode: appdto.PinRetrySameChannel})
			}},
			{name: "strict-session", actions: [4]string{"no-key", "success", "success"}, attempts: 1, setup: func(c *gin.Context) {
				c.Set("channel_affinity_skip_retry_on_failure", true)
				service.RequestPolicy(c).SessionMode = "strict"
			}},
			{name: "invalid-user-request", actions: [4]string{"no-key", "success", "success"}, setup: func(c *gin.Context) {
				c.Request.Body = io.NopCloser(strings.NewReader(`{"contents":[]}`))
			}},
		} {
			t.Run(fmt.Sprintf("%s/stream=%t", test.name, stream), func(t *testing.T) {
				result := runArgonNativeRelay(t, "gemini-4-argon", "gemini-4-argon-multimodal", stream, test.actions, test.setup)
				assert.Equal(t, test.wantOrder, result.order)
				for _, body := range result.bodies {
					assert.JSONEq(t, argonNativeTestRequest, body, "safe local recovery preserves all native inputs")
				}
				used := result.c.GetStringSlice("use_channel")
				assert.Len(t, used, test.attempts)
				assert.Equal(t, test.attempts, service.RequestPolicy(result.c).Attempts)
				seen := make(map[string]bool)
				for _, channelID := range used {
					assert.False(t, seen[channelID], "candidate must not repeat")
					seen[channelID] = true
				}
				if test.success {
					assert.Equal(t, 200, result.response.Code, result.response.Body.String())
					assert.Equal(t, 999890, result.quota)
					assert.Equal(t, 110, result.usedQuota)
					assert.Equal(t, 1, result.requestCount)
				} else {
					assert.GreaterOrEqual(t, result.response.Code, 400, result.response.Body.String())
					assert.Equal(t, 1000000, result.quota)
					assert.Zero(t, result.usedQuota)
					assert.Zero(t, result.requestCount)
				}
			})
		}
	}
}
