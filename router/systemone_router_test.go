package router

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const systemOneTestStartQuota = 100000

func setupTypeSafeRouterTest(t *testing.T, startQuota int, upstreamHandler http.HandlerFunc) (*gin.Engine, *httptest.Server) {
	t.Helper()
	setupRelayRouterTestDB(t)
	require.NoError(t, i18n.Init())
	service.InitHttpClient()
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Task{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.Option{}))

	billingSettings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	savedBilling := *billingSettings
	savedRatios := ratio_setting.ModelRatio2JSONString()
	savedPrices := ratio_setting.ModelPrice2JSONString()
	savedGroups := ratio_setting.GroupRatio2JSONString()
	savedUsableGroups := setting.UserUsableGroups2JSONString()
	savedMemory := common.MemoryCacheEnabled
	savedBatch := common.BatchUpdateEnabled
	savedQuotaPerUnit := common.QuotaPerUnit
	savedRetryTimes := common.RetryTimes
	savedRegistry := jsplugin.DefaultRegistry

	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.QuotaPerUnit = 500000
	common.RetryTimes = 0

	*billingSettings = billing_setting.BillingSetting{
		BillingMode:       map[string]string{},
		BillingExpr:       map[string]string{},
		PluginBillingExpr: map[string]string{},
	}
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))

	source, err := builtinplugins.Source("typesafe")
	require.NoError(t, err)
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	_, err = jsplugin.DefaultRegistry.RegisterFactory(source, jsplugin.Options{Key: "typesafe"})
	require.NoError(t, err)

	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(func() {
		upstream.Close()
		jsplugin.DefaultRegistry = savedRegistry
		*billingSettings = savedBilling
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(savedUsableGroups))
		common.MemoryCacheEnabled = savedMemory
		common.BatchUpdateEnabled = savedBatch
		common.QuotaPerUnit = savedQuotaPerUnit
		common.RetryTimes = savedRetryTimes
		model.InitChannelCache()
	})

	require.NoError(t, model.DB.Create(&model.User{
		Id:       90,
		Username: "typesafe-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    startQuota,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:         90,
		Key:            "typesafetoken",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
	}).Error)

	channel := model.Channel{
		Id:           65,
		Type:         constant.ChannelTypeTaskPlugin,
		Status:       common.ChannelStatusEnabled,
		Name:         "typesafe-official",
		Key:          "upstream-typesafe-key",
		Models:       "jev,jev-1.14.0,jev-1.13.0,jev-latest,jev-preview",
		ModelMapping: common.GetPointer(`{"jev":"jev-latest"}`),
		Setting:      common.GetPointer(`{"task_plugin_key":"typesafe"}`),
		Group:        "default",
		BaseURL:      &upstream.URL,
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))
	model.InitChannelCache()

	engine := gin.New()
	SetRelayRouter(engine)
	engine.NoRoute(SetPluginRouter(engine))
	return engine, upstream
}

func TestSystemOnePluginRoutesPreserveOrderAliasHeadersMultipartAndBilling(t *testing.T) {
	t.Run("vendor route jev-1.14.0 preserves state key order and does not retain result", func(t *testing.T) {
		var calls int
		var upstreamPath, upstreamAuth, rawUpstreamBody string
		upstreamResponse := `{"model":"jev-1.14.0","answers":{"q":{"type":"noul","answer":true}},"usage":{"input_tokens":100,"output_tokens":12}}`

		engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
			calls++
			upstreamPath = r.URL.Path
			upstreamAuth = r.Header.Get("Authorization")
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			rawUpstreamBody = string(raw)
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(upstreamResponse))
			require.NoError(t, err)
		})

		clientBody := `{"model":"jev-1.14.0","state":{"zeta":1,"alpha":2,"duration":3601,"count":999,"n":"No"},"questions":{"q":{"type":"noul"}}}`
		req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(clientBody))
		req.Header.Set("Authorization", "Bearer typesafetoken")
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, 1, calls)
		assert.Equal(t, "/v1/systemone", upstreamPath)
		assert.Equal(t, "Bearer upstream-typesafe-key", upstreamAuth)
		assert.Greater(t, strings.Index(rawUpstreamBody, `"alpha"`), strings.Index(rawUpstreamBody, `"zeta"`))
		assert.Contains(t, rawUpstreamBody, `"duration":3601`)
		assert.Contains(t, rawUpstreamBody, `"count":999`)
		assert.Contains(t, rawUpstreamBody, `"n":"No"`)
		assert.JSONEq(t, upstreamResponse, rec.Body.String())

		var user model.User
		require.NoError(t, model.DB.First(&user, 90).Error)
		assert.Equal(t, systemOneTestStartQuota-2, user.Quota)

		var task model.Task
		require.NoError(t, model.DB.First(&task).Error)
		assert.Equal(t, 2, task.Quota)
		assert.Empty(t, task.Data)
		assert.True(t, task.PrivateData.ResultDiscarded)
	})

	t.Run("legacy jev alias resolves to jev-latest via channel mapping", func(t *testing.T) {
		var received map[string]any
		engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, common.Unmarshal(raw, &received))
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"model":"jev-latest","answers":{"q":{"type":"noul","answer":false}},"usage":{"input_tokens":100000}}`))
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev","state":"hello","questions":{"q":{"type":"noul"}}}`))
		req.Header.Set("Authorization", "Bearer typesafetoken")
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "jev-latest", received["model"])

		var user model.User
		require.NoError(t, model.DB.First(&user, 90).Error)
		assert.Equal(t, systemOneTestStartQuota-2100, user.Quota)
	})

	t.Run("long-context header enforces higher reserve before upstream", func(t *testing.T) {
		var calls int
		engine, _ := setupTypeSafeRouterTest(t, 3000, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, "/typesafe/v1/systemone", strings.NewReader(`{"model":"jev-1.14.0","state":"long context check","questions":{"q":{"type":"noul"}}}`))
		req.Header.Set("Authorization", "Bearer typesafetoken")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("TypeSafe-Danger-Zone-Long-Context", "true")

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		assert.NotEqual(t, http.StatusOK, rec.Code)
		assert.Zero(t, calls)

		var user model.User
		require.NoError(t, model.DB.First(&user, 90).Error)
		assert.Equal(t, 3000, user.Quota)
	})

	t.Run("plugin-prefixed long-context request forwards header and strips client credentials", func(t *testing.T) {
		var calls int
		var upstreamPath, authHeader, longCtxHeader string
		var leakedCookie, leakedAPIKey, leakedGoogleKey string

		engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
			calls++
			upstreamPath = r.URL.Path
			authHeader = r.Header.Get("Authorization")
			longCtxHeader = r.Header.Get("TypeSafe-Danger-Zone-Long-Context")
			leakedCookie = r.Header.Get("Cookie")
			leakedAPIKey = r.Header.Get("X-Api-Key")
			leakedGoogleKey = r.Header.Get("X-Goog-Api-Key")
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"model":"jev-1.14.0","answers":{"q":{"type":"noul","answer":true}},"usage":{"input_tokens":200000}}`))
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodPost, "/typesafe/v1/systemone", strings.NewReader(`{"model":"jev-1.14.0","state":"long context ok","questions":{"q":{"type":"noul"}}}`))
		req.Header.Set("Authorization", "Bearer typesafetoken")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("TypeSafe-Danger-Zone-Long-Context", "true")
		req.Header.Set("Cookie", "session=secret")
		req.Header.Set("X-Api-Key", "client-secret")
		req.Header.Set("X-Goog-Api-Key", "google-secret")

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, 1, calls)
		assert.Equal(t, "/v1/systemone", upstreamPath)
		assert.Equal(t, "Bearer upstream-typesafe-key", authHeader)
		assert.Equal(t, "true", longCtxHeader)
		assert.Empty(t, leakedCookie)
		assert.Empty(t, leakedAPIKey)
		assert.Empty(t, leakedGoogleKey)

		var user model.User
		require.NoError(t, model.DB.First(&user, 90).Error)
		assert.Equal(t, systemOneTestStartQuota-4200, user.Quota)
	})

	t.Run("multipart upload with box question puts request first and sets images header", func(t *testing.T) {
		imageBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x01, 0x02}
		type uploadedPart struct {
			field    string
			filename string
			data     []byte
		}
		var parts []uploadedPart
		var imagesHeader string

		engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
			imagesHeader = r.Header.Get("TypeSafe-Danger-Zone-Images")
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			require.NoError(t, err)
			reader := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				data, err := io.ReadAll(part)
				require.NoError(t, err)
				parts = append(parts, uploadedPart{
					field:    part.FormName(),
					filename: part.FileName(),
					data:     data,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"model":"jev-1.14.0","answers":{"b":{"type":"box","answer":{"box":[10,20,30,40]}}},"usage":{"input_tokens":1000}}`))
			require.NoError(t, err)
		})

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("request", `{"model":"jev-1.14.0","state":"locate object","assets":{"img1":"img1"},"questions":{"b":{"type":"box"}}}`))
		fileWriter, err := writer.CreateFormFile("img1", "test.png")
		require.NoError(t, err)
		_, err = fileWriter.Write(imageBytes)
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		req := httptest.NewRequest(http.MethodPost, "/v1/systemone", &body)
		req.Header.Set("Authorization", "Bearer typesafetoken")
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "true", imagesHeader)
		require.Len(t, parts, 2)
		assert.Equal(t, "request", parts[0].field)
		var forwardedReq map[string]any
		require.NoError(t, common.Unmarshal(parts[0].data, &forwardedReq))
		assert.Equal(t, "jev-1.14.0", forwardedReq["model"])
		assert.Equal(t, "img1", parts[1].field)
		assert.Equal(t, "test.png", parts[1].filename)
		assert.Equal(t, imageBytes, parts[1].data)

		var respBody map[string]any
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &respBody))
		answers := respBody["answers"].(map[string]any)
		boxAnswer := answers["b"].(map[string]any)
		assert.Equal(t, "box", boxAnswer["type"])

		var user model.User
		require.NoError(t, model.DB.First(&user, 90).Error)
		assert.Equal(t, systemOneTestStartQuota-21, user.Quota)
	})

	t.Run("errors and fail-closed completion usage settlement", func(t *testing.T) {
		t.Run("unauthorized without token", func(t *testing.T) {
			var calls int
			engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
				calls++
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev-1.14.0","state":"hi","questions":{"q":{"type":"noul"}}}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			assert.Zero(t, calls)
		})

		t.Run("missing model returns native detail JSON", func(t *testing.T) {
			var calls int
			engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
				calls++
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"state":"hi","questions":{"q":{"type":"noul"}}}`))
			req.Header.Set("Authorization", "Bearer typesafetoken")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.JSONEq(t, `{"detail":"model is required"}`, rec.Body.String())
			assert.Zero(t, calls)

			var user model.User
			require.NoError(t, model.DB.First(&user, 90).Error)
			assert.Equal(t, systemOneTestStartQuota, user.Quota)
		})

		t.Run("upstream 422 preserves TypeSafe detail and refunds full quota", func(t *testing.T) {
			engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, err := w.Write([]byte(`{"detail":{"error_type":"max_tokens_exceeded","message":"limit exceeded"}}`))
				require.NoError(t, err)
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev-1.14.0","state":"hi","questions":{"q":{"type":"noul"}}}`))
			req.Header.Set("Authorization", "Bearer typesafetoken")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			var errBody map[string]any
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &errBody))
			detail, ok := errBody["detail"].(map[string]any)
			require.True(t, ok, rec.Body.String())
			assert.Equal(t, "max_tokens_exceeded", detail["error_type"])

			var user model.User
			require.Eventually(t, func() bool {
				return model.DB.First(&user, 90).Error == nil && user.Quota == systemOneTestStartQuota
			}, time.Second, 5*time.Millisecond, "upstream 422 must refund the pre-consumed quota")
			assert.Equal(t, systemOneTestStartQuota, user.Quota)
		})

		t.Run("missing usage.input_tokens fails closed and keeps standard reservation", func(t *testing.T) {
			engine, _ := setupTypeSafeRouterTest(t, systemOneTestStartQuota, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"model":"jev-1.14.0","answers":{"q":{"type":"noul","answer":true}}}`))
				require.NoError(t, err)
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(`{"model":"jev-1.14.0","state":"hi","questions":{"q":{"type":"noul"}}}`))
			req.Header.Set("Authorization", "Bearer typesafetoken")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var user model.User
			require.NoError(t, model.DB.First(&user, 90).Error)
			assert.Equal(t, systemOneTestStartQuota-1382, user.Quota)

			var task model.Task
			require.NoError(t, model.DB.First(&task).Error)
			assert.Equal(t, 1382, task.Quota)
		})
	})
}
