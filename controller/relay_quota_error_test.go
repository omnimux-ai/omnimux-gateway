package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Exercises the real Relay -> billing rejection -> HTTP writer chain before any
// upstream/channel attempt. The optional artifact feeds the external SDK probe.
func TestRelayAccountQuotaResponse(t *testing.T) {
	oldRatios := ratio_setting.ModelRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatios)) })
	oldDB, oldRedis, oldCount := model.DB, common.RedisEnabled, constant.CountToken
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		constant.CountToken = oldCount
		_ = sqlDB.Close()
	})
	model.DB, common.RedisEnabled, constant.CountToken = db, false, true
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := model.User{Id: 668, Username: "quota-compat", Quota: 0}
	require.NoError(t, db.Create(&user).Error)
	var fixtures []map[string]any
	for _, quota := range []int{0, 1} {
		for _, stream := range []bool{false, true} {
			for _, endpoint := range []struct {
				path   string
				format types.RelayFormat
				input  string
			}{
				{"/v1/chat/completions", types.RelayFormatOpenAI, `"messages":[{"role":"user","content":"hello"}],"max_tokens":100`},
				{"/v1/responses", types.RelayFormatOpenAIResponses, `"input":"hello world this input costs more than one quota unit","max_output_tokens":100`},
			} {
				t.Run(fmt.Sprintf("%s/quota=%d/stream=%t", endpoint.path, quota, stream), func(t *testing.T) {
					require.NoError(t, db.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", quota).Error)
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					c.Request = httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(fmt.Sprintf(`{"model":"gpt-4o","stream":%t,%s}`, stream, endpoint.input)))
					c.Request.Header.Set("Content-Type", "application/json")
					c.Set(string(constant.ContextKeyUserId), user.Id)
					c.Set(string(constant.ContextKeyOriginalModel), "gpt-4o")
					c.Set(string(constant.ContextKeyUsingGroup), "default")
					c.Set(string(constant.ContextKeyUserGroup), "default")
					c.Set(string(constant.ContextKeyUserSetting), dto.UserSetting{BillingPreference: "wallet_only"})
					c.Set(common.RequestIdKey, "quota-compat-test")
					Relay(c, endpoint.format)
					require.Equal(t, http.StatusPaymentRequired, w.Code, w.Body.String())
					var body struct {
						Error types.OpenAIError `json:"error"`
					}
					require.NoError(t, common.Unmarshal(w.Body.Bytes(), &body))
					assert.Equal(t, "insufficient_quota", body.Error.Code)
					assert.Equal(t, "insufficient_quota", body.Error.Type)
					assert.Contains(t, body.Error.Message, "insufficient_quota:")
					assert.Contains(t, body.Error.Message, "quota-compat-test")
					assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
					assert.Empty(t, c.GetStringSlice("use_channel"))
					current, err := model.GetUserQuota(user.Id, true)
					require.NoError(t, err)
					assert.Equal(t, quota, current)
					fixtures = append(fixtures, map[string]any{"name": t.Name(), "status": w.Code, "body": body, "stream": stream, "path": endpoint.path})
				})
			}
		}
	}
	if path := os.Getenv("QUOTA_COMPAT_FIXTURES"); path != "" {
		data, err := common.Marshal(fixtures)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0600))
	}
}

func TestOpenAIQuotaErrorBoundary(t *testing.T) {
	local := func(code types.ErrorCode, status int) *types.NewAPIError {
		return types.NewErrorWithStatusCode(errors.New("用户剩余额度: 403.000000, 需要预扣费额度: 500.000000"), code, status, types.ErrOptionWithSkipRetry())
	}
	for _, tc := range []struct {
		name   string
		err    *types.NewAPIError
		format types.RelayFormat
		status int
	}{
		{"local", local(types.ErrorCodeInsufficientUserQuota, 403), types.RelayFormatOpenAI, 402},
		{"compact", local(types.ErrorCodeInsufficientUserQuota, 403), types.RelayFormatOpenAIResponsesCompaction, 402},
		{"upstream-same-code", types.WithOpenAIError(types.OpenAIError{Code: "insufficient_user_quota", Message: "quota"}, 403), types.RelayFormatOpenAI, 403},
		{"auth", local(types.ErrorCodeAccessDenied, 401), types.RelayFormatOpenAI, 401},
		{"permission", local(types.ErrorCodeAccessDenied, 403), types.RelayFormatOpenAI, 403},
		{"token", local(types.ErrorCodePreConsumeTokenQuotaFailed, 403), types.RelayFormatOpenAI, 403},
		{"rate-limit", local(types.ErrorCodeBadResponseStatusCode, 429), types.RelayFormatOpenAI, 429},
		{"server", local(types.ErrorCodeBadResponseStatusCode, 503), types.RelayFormatOpenAI, 503},
		{"gemini", local(types.ErrorCodeInsufficientUserQuota, 403), types.RelayFormatGemini, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			before := tc.err.ToOpenAIError()
			retry := service.DecideRelayRetry(c, tc.err, 2).Action == "retry"
			writeOpenAIRelayError(c, tc.format, tc.err)
			assert.Equal(t, tc.status, w.Code)
			assert.Equal(t, before, tc.err.ToOpenAIError())
			assert.Equal(t, retry, service.DecideRelayRetry(c, tc.err, 2).Action == "retry")
			if tc.status == 402 {
				assert.False(t, retry)
				assert.NotContains(t, w.Body.String(), "403.000000")
				assert.Contains(t, w.Body.String(), "用户余额或订阅额度不足")
			} else {
				var actual struct {
					Error types.OpenAIError `json:"error"`
				}
				require.NoError(t, common.Unmarshal(w.Body.Bytes(), &actual))
				expectedJSON, err := common.Marshal(before)
				require.NoError(t, err)
				actualJSON, err := common.Marshal(actual.Error)
				require.NoError(t, err)
				assert.JSONEq(t, string(expectedJSON), string(actualJSON))
			}
		})
	}
}
