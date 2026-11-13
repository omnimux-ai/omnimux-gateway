package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemOneRouterEndToEnd(t *testing.T) {
	setupRelayRouterTestDB(t)
	ratio_setting.InitRatioSettings()

	// 1. 模拟 TypeSafe AI 上游 HTTP 接收端
	var receivedUpstreamPath string
	var receivedAuthHeader string
	var receivedBody map[string]any

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUpstreamPath = r.URL.Path
		receivedAuthHeader = r.Header.Get("Authorization")
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "jev-1.13.0",
			"judgments": [
				{
					"id": "q1",
					"primitive": "noul",
					"probability": 0.92,
					"answer": true
				}
			],
			"usage": {
				"input_tokens": 100,
				"output_tokens": 20,
				"total_tokens": 120
			}
		}`))
	}))
	defer mockUpstream.Close()

	// 2. 数据库迁移并配置 User, Token 与 Channel (TypeSafe channel 74)
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.Log{},
		&model.UserSubscription{},
	))

	initialQuota := 10000000
	user := model.User{
		Username: "jev-tester",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    initialQuota,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	testKey := "jevtestkey12345"
	token := model.Token{
		UserId:         user.Id,
		Key:            testKey,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		RemainQuota:    initialQuota,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	baseURL := mockUpstream.URL
	mapping := `{"jev":"jev-latest"}`
	channelKey := "sk-typesafe-upstream-key"
	priority := int64(100)
	weight := uint(10)
	ch := model.Channel{
		Id:           74,
		Name:         "[官方直连-TypeSafe] TypeSafe AI",
		Type:         constant.ChannelTypeTypeSafe,
		Status:       common.ChannelStatusEnabled,
		BaseURL:      &baseURL,
		Key:          channelKey,
		Group:        "default",
		Models:       "jev",
		ModelMapping: &mapping,
		Priority:     &priority,
		Weight:       &weight,
	}
	require.NoError(t, model.DB.Create(&ch).Error)
	require.NoError(t, ch.UpdateAbilities(nil))
	model.InitChannelCache()

	// 3. 构建路由
	engine := gin.New()
	SetRelayRouter(engine)

	// 4. 发起 POST /v1/systemone 请求
	reqPayload := map[string]any{
		"model": "jev",
		"input": map[string]any{"text": "check if user is approved"},
		"state": map[string]any{
			"code": "if user.is_active { approve() }",
		},
		"questions": []any{
			map[string]any{
				"id":           "q1",
				"instructions": "check if user is approved",
			},
		},
	}
	reqBytes, _ := json.Marshal(reqPayload)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/systemone", bytes.NewReader(reqBytes))
	request.Header.Set("Authorization", "Bearer "+testKey)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	// 5. 校验结果
	require.Equal(t, http.StatusOK, recorder.Code, "response: "+recorder.Body.String())
	assert.Equal(t, "/v1/systemone", receivedUpstreamPath)
	assert.Equal(t, "Bearer "+channelKey, receivedAuthHeader)
	// 上游接收到的 model 应该被映射为 jev-latest
	assert.Equal(t, "jev-latest", receivedBody["model"])

	var clientResp map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &clientResp))
	assert.Equal(t, "jev-1.13.0", clientResp["model"])

	// 6. 校验 Quota 扣除
	var updatedUser model.User
	require.NoError(t, model.DB.First(&updatedUser, user.Id).Error)
	// New API 计费规则：ModelRatio 1.0 对应 1 token = 1 quota ($0.002/1k tokens)
	// 100 input tokens * ModelRatio(0.021) * GroupRatio(1.0) = 2.1 quota -> 取整 2 quota
	// 输出 20 tokens 免费 (CompletionRatio = 0)
	expectedConsumedQuota := 2
	actualConsumed := initialQuota - updatedUser.Quota
	assert.Equal(t, expectedConsumedQuota, actualConsumed, fmt.Sprintf("expected %d consumed, got %d", expectedConsumedQuota, actualConsumed))
	time.Sleep(50 * time.Millisecond)
}

func TestSystemOneRouterUnauthorized(t *testing.T) {
	setupRelayRouterTestDB(t)

	engine := gin.New()
	SetRelayRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev"}`))
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestSystemOneRouterDefaultModelOmitted(t *testing.T) {
	setupRelayRouterTestDB(t)
	ratio_setting.InitRatioSettings()

	var receivedBody map[string]any
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","usage":{"input_tokens":50,"output_tokens":0}}`))
	}))
	defer mockUpstream.Close()

	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.Log{},
		&model.UserSubscription{},
	))

	user := model.User{
		Username: "jev-default-tester",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000000,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	testKey := "jevdefaultkey999"
	token := model.Token{
		UserId:         user.Id,
		Key:            testKey,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	baseURL := mockUpstream.URL
	mapping := `{"jev":"jev-latest"}`
	priority := int64(100)
	weight := uint(10)
	ch := model.Channel{
		Id:           74,
		Name:         "[官方直连-TypeSafe] TypeSafe AI",
		Type:         constant.ChannelTypeTypeSafe,
		Status:       common.ChannelStatusEnabled,
		BaseURL:      &baseURL,
		Key:          "mock-key",
		Group:        "default",
		Models:       "jev",
		ModelMapping: &mapping,
		Priority:     &priority,
		Weight:       &weight,
	}
	require.NoError(t, model.DB.Create(&ch).Error)
	require.NoError(t, ch.UpdateAbilities(nil))
	model.InitChannelCache()

	engine := gin.New()
	SetRelayRouter(engine)

	// 请求体中故意不传 model 字段
	reqPayload := map[string]any{
		"input":     map[string]any{"fact": "omnimux"},
		"state":     map[string]any{"fact": "omnimux"},
		"questions": []any{map[string]any{"id": "q0"}},
	}
	reqBytes, _ := json.Marshal(reqPayload)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/systemone", bytes.NewReader(reqBytes))
	request.Header.Set("Authorization", "Bearer "+testKey)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	// 验证上游收到的请求体中补齐了映射后的模型名称
	assert.Equal(t, "jev-latest", receivedBody["model"])
	time.Sleep(50 * time.Millisecond)
}

func TestSystemOneRouterNullBodyDoesNotPanic(t *testing.T) {
	setupRelayRouterTestDB(t)
	ratio_setting.InitRatioSettings()

	var receivedUpstreamBody string
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedUpstreamBody = string(bodyBytes)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","usage":{"input_tokens":10,"output_tokens":0}}`))
	}))
	defer mockUpstream.Close()

	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.Log{},
		&model.UserSubscription{},
	))

	user := model.User{
		Username: "jev-null-tester",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    10000000,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	testKey := "jevnullkey000"
	token := model.Token{
		UserId:         user.Id,
		Key:            testKey,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	baseURL := mockUpstream.URL
	priority := int64(100)
	weight := uint(10)
	ch := model.Channel{
		Id:       74,
		Name:     "[官方直连-TypeSafe] TypeSafe AI",
		Type:     constant.ChannelTypeTypeSafe,
		Status:   common.ChannelStatusEnabled,
		BaseURL:  &baseURL,
		Key:      "mock-key",
		Group:    "default",
		Models:   "jev",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, model.DB.Create(&ch).Error)
	require.NoError(t, ch.UpdateAbilities(nil))
	model.InitChannelCache()

	engine := gin.New()
	SetRelayRouter(engine)

	// 请求体为 JSON null
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader("null"))
	request.Header.Set("Authorization", "Bearer "+testKey)
	request.Header.Set("Content-Type", "application/json")

	// 验证不会发生 panic 崩溃
	assert.NotPanics(t, func() {
		engine.ServeHTTP(recorder, request)
	})
	assert.Equal(t, http.StatusOK, recorder.Code)
	var forwarded map[string]any
	require.NoError(t, json.Unmarshal([]byte(receivedUpstreamBody), &forwarded))
	assert.Equal(t, "jev", forwarded["model"], "null JSON must be normalized to the validated public model")
	time.Sleep(50 * time.Millisecond)
}

func TestSystemOneRouterRejectsInvalidUsageWithoutCharging(t *testing.T) {
	testCases := []struct {
		name         string
		upstreamBody string
	}{
		{
			name:         "malformed JSON",
			upstreamBody: `{"model": invalid json`,
		},
		{
			name:         "usage missing",
			upstreamBody: `{"model":"jev-1.13.0","judgments":[]}`,
		},
		{
			name:         "zero usage",
			upstreamBody: `{"model":"jev-1.13.0","usage":{"input_tokens":0,"output_tokens":0}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setupRelayRouterTestDB(t)
			ratio_setting.InitRatioSettings()

			mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(testCase.upstreamBody))
			}))
			defer mockUpstream.Close()

			require.NoError(t, model.DB.AutoMigrate(
				&model.User{},
				&model.Token{},
				&model.Channel{},
				&model.Ability{},
				&model.Log{},
				&model.UserSubscription{},
			))

			initialQuota := 10000000
			user := model.User{
				Username: "jev-invalid-usage-tester",
				Status:   common.UserStatusEnabled,
				Group:    "default",
				Quota:    initialQuota,
			}
			require.NoError(t, model.DB.Create(&user).Error)

			testKey := "jevinvalidusagekey"
			token := model.Token{
				UserId:         user.Id,
				Key:            testKey,
				Status:         common.TokenStatusEnabled,
				ExpiredTime:    -1,
				UnlimitedQuota: true,
				RemainQuota:    initialQuota,
			}
			require.NoError(t, model.DB.Create(&token).Error)

			baseURL := mockUpstream.URL
			priority := int64(100)
			weight := uint(10)
			ch := model.Channel{
				Id:       74,
				Name:     "[官方直连-TypeSafe] TypeSafe AI",
				Type:     constant.ChannelTypeTypeSafe,
				Status:   common.ChannelStatusEnabled,
				BaseURL:  &baseURL,
				Key:      "mock-key",
				Group:    "default",
				Models:   "jev",
				Priority: &priority,
				Weight:   &weight,
			}
			require.NoError(t, model.DB.Create(&ch).Error)
			require.NoError(t, ch.UpdateAbilities(nil))
			model.InitChannelCache()

			engine := gin.New()
			SetRelayRouter(engine)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev","state":{},"questions":[{"id":"q1"}]}`))
			request.Header.Set("Authorization", "Bearer "+testKey)
			request.Header.Set("Content-Type", "application/json")

			engine.ServeHTTP(recorder, request)

			assert.NotEqual(t, http.StatusOK, recorder.Code, "invalid upstream usage must not be returned as success: %s", recorder.Body.String())
			var updatedUser model.User
			require.NoError(t, model.DB.First(&updatedUser, user.Id).Error)
			assert.Equal(t, initialQuota, updatedUser.Quota, "failed upstream response must not charge user quota")
		})
	}
}
