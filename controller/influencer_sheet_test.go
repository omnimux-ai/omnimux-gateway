package controller

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Exercises the real sheet handler, native task submission, GoEasy HTTP payload
// and post-submit metadata patch. Only the provider and database are local test
// boundaries; the queued provider response is not a generated-image assertion.
func TestPostInfluencerSheetFixesLandscapeSizeThroughGoEasy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Task{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousReferenceClient := influencerReferenceHTTPClient
	fetch := system_setting.GetFetchSetting()
	previousFetch := *fetch
	previousRedis, previousMemory, previousBatch, previousConsume := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	previousPrices := ratio_setting.ModelPrice2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	previousAutoGroups := setting.AutoGroups2JsonString()
	previousFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		influencerReferenceHTTPClient = previousReferenceClient
		*fetch = previousFetch
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = previousRedis, previousMemory, previousBatch, previousConsume
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = previousFreePreConsume
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAutoGroups))
		require.NoError(t, sqlDB.Close())
	})
	fetch.EnableSSRFProtection = false // Allow the local provider boundary; reference validation still requires public HTTPS.
	service.InitHttpClient()
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, false, false
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2.5":0}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-economy":1}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-economy"]`))
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "ratio"}, map[string]string{})
	previousAddress, previousPublic, previousSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = "https://gateway.example", "", "local-sheet-capability-secret"
	t.Cleanup(func() {
		system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = previousAddress, previousPublic, previousSecret
	})
	const userID = 107
	const group = "gpt-image-2.5-economy"
	const reference = "https://8.8.8.8/character.png" // A public IP avoids DNS and real network access during URL validation.
	var pngData bytes.Buffer
	require.NoError(t, png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	referenceFetches := 0
	influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(req *http.Request) (*http.Response, error) {
		referenceFetches++
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, reference, req.URL.String())
		return influencerImageTransport{data: pngData.Bytes()}.RoundTrip(req)
	})}
	require.NoError(t, db.Create(&model.User{Status: common.UserStatusEnabled, Id: userID, Username: "sheet-test", Group: group, Quota: 1_000_000}).Error)
	const legacyMeta = `{"tier":"normal","selection":{"gender":["female"]},"size":"1024x1536"}`
	require.NoError(t, db.Create(&model.Task{TaskID: "legacy-sheet", UserId: userID, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: legacyMeta}}).Error)

	type capturedRequest struct {
		path string
		body map[string]any
		err  error
	}
	requests := make(chan capturedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		decodeErr := common.DecodeJson(r.Body, &body)
		requests <- capturedRequest{path: r.URL.Path, body: body, err: decodeErr}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"local-provider-task","status":"queued"}`)
	}))
	t.Cleanup(upstream.Close)
	channel := model.Channel{Type: constant.ChannelTypeGoEasyAPI, Name: "local-sheet-provider", Key: "local-test-placeholder", BaseURL: &upstream.URL, Status: common.ChannelStatusEnabled, Models: "gpt-image-2.5", Group: group}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: group, Model: "gpt-image-2.5", ChannelId: channel.Id, Enabled: true}).Error)

	var wire map[string]any
	engine := gin.New()
	engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
		c.Set("id", userID)
		common.SetContextKey(c, constant.ContextKeyUserGroup, group)
		c.Next()
	}, PrepareInfluencerSheet, func(c *gin.Context) {
		c.Set("id", userID)
		c.Set("group", group)
		c.Set("using_group", group)
		c.Set("token_group", group)
		require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channel, "gpt-image-2.5"))
		PostInfluencerSheet(c)
		wire = nil
		require.NoError(t, common.UnmarshalBodyReusable(c, &wire))
		assert.Equal(t, group, c.GetHeader("X-Omnimux-Group"))
		assert.Equal(t, group, common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		assert.Equal(t, group, common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
	})

	engine.POST("/pg/influencer/multiview", func(c *gin.Context) {
		c.Set("id", userID)
		c.Set("group", group)
		c.Set("using_group", group)
		c.Set("token_group", group)
		require.Nil(t, middleware.SetupContextForSelectedChannel(c, &channel, "gpt-image-2.5"))
		PostInfluencerMultiView(c)
	})

	for _, tier := range []string{"normal", "freak", "total"} {
		for _, tc := range []struct {
			name string
			size *string
		}{
			{name: "omitted"},
			{name: "empty", size: common.GetPointer("")},
			{name: "whitespace", size: common.GetPointer("  ")},
			{name: "legacy ratio", size: common.GetPointer("9:16")},
			{name: "legacy pixels", size: common.GetPointer("1024x1536")},
			{name: "square", size: common.GetPointer("1024x1024")},
			{name: "landscape", size: common.GetPointer("16:9")},
		} {
			for _, withReference := range []bool{false, true} {
				mode := "generation"
				if withReference {
					mode = "edit"
				}
				t.Run(tier+"/"+tc.name+"/"+mode, func(t *testing.T) {
					const brief = "A red linen jacket and a calm smile"
					payload := map[string]any{"model": "gpt-image-2.5", "group": group, "tier": tier, "selection": map[string]any{"gender": []string{"female"}}, "brief": brief, "seed": 12345}
					if tc.size != nil {
						payload["size"] = *tc.size
					}
					if withReference {
						payload["image_url"] = "  " + reference + "  "
					}
					encoded, err := common.Marshal(payload)
					require.NoError(t, err)
					req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", bytes.NewReader(encoded))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-Omnimux-Group", group)
					recorder := httptest.NewRecorder()
					previousFetches := referenceFetches
					engine.ServeHTTP(recorder, req)
					require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					dispatched := dispatchGenerationQueueTest(t, userID, recorder.Body.Bytes())
					t.Cleanup(func() { require.NoError(t, db.Model(dispatched).Update("status", model.TaskStatusSuccess).Error) })
					require.Len(t, requests, 1, "a local provider request must actually occur")
					captured := <-requests
					require.NoError(t, captured.err)
					assert.Equal(t, "1920x1088", captured.body["size"])
					assert.Equal(t, "gpt-image-2.5-async", captured.body["model"], "fixed ratio must not select the high-resolution model")
					assert.Equal(t, wire["prompt"], captured.body["prompt"])
					assert.Contains(t, captured.body["prompt"], brief)
					assert.Contains(t, captured.body["prompt"], "Female")
					assert.Contains(t, captured.body["prompt"], "16:9 landscape canvas with exactly two side-by-side views")
					assert.Contains(t, captured.body["prompt"], "LEFT HALF: a large front-facing chest-up portrait")
					assert.Contains(t, captured.body["prompt"], "RIGHT HALF: a front-facing full-body standing view of the same single character")
					assert.NotContains(t, captured.body, "aspect_ratio")
					assert.NotContains(t, captured.body, "group", "routing groups are internal, not provider fields")
					assert.Equal(t, "1920x1088", wire["size"])
					assert.NotContains(t, wire, "group", "group survives in header/context, not the task payload")
					assert.Equal(t, map[string]any{"seed": float64(12345)}, wire["metadata"], "seed metadata survives controller rewriting without promising provider seed support")
					if withReference {
						assert.Equal(t, "/api/image-tasks/edits", captured.path)
						assert.Equal(t, []any{reference}, captured.body["images"])
						assert.Equal(t, reference, wire["image"])
						assert.Equal(t, previousFetches+2, referenceFetches, "the reference is validated before acceptance and dispatch")
						assert.Contains(t, captured.body["prompt"], "Explicit brief takes priority over selected traits; selected traits take priority over image inspiration")
						assert.Contains(t, captured.body["prompt"], "a different person")
						assert.NotContains(t, captured.body["prompt"], "exact same character")
					} else {
						assert.Equal(t, "/api/image-tasks/generations", captured.path)
						assert.NotContains(t, captured.body, "images")
						assert.NotContains(t, wire, "image")
						assert.Equal(t, previousFetches, referenceFetches, "generation has no reference fetch")
						assert.NotContains(t, captured.body["prompt"], "reference image only as inspiration")
					}

					var response struct {
						TaskID string `json:"task_id"`
					}
					require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
					require.NotEmpty(t, response.TaskID)
					task, exists, err := model.GetByTaskId(userID, response.TaskID)
					require.NoError(t, err)
					require.True(t, exists)
					assert.Equal(t, ActionInfluencerSheet, task.Action)
					var meta map[string]any
					require.NoError(t, common.UnmarshalJsonStr(task.Properties.Input, &meta))
					assert.Equal(t, "16:9", meta["size"])
					assert.Equal(t, "16:9", meta["size"], "supplier pixels must not replace the workbench composition metadata")
					assert.Equal(t, tier, meta["tier"])
					assert.Equal(t, map[string]any{"gender": []any{"female"}}, meta["selection"])
					assert.Equal(t, brief, meta["brief"])
					assert.Equal(t, float64(12345), meta["seed"])
					if withReference {
						assert.Equal(t, reference, meta["image_url"])
					} else {
						assert.NotContains(t, meta, "image_url")
					}
				})
			}
		}
	}
	for _, tier := range []string{"cats", "dogs"} {
		t.Run(tier+"/default animal", func(t *testing.T) {
			encoded, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "group": group, "tier": tier, "selection": map[string]any{}, "brief": ""})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", bytes.NewReader(encoded))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Omnimux-Group", group)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			dispatchGenerationQueueTest(t, userID, recorder.Body.Bytes())
			require.Len(t, requests, 1)
			captured := <-requests
			require.NoError(t, captured.err)
			assert.Contains(t, captured.body["prompt"], "anthropomorphic")
			assert.NotContains(t, captured.body["prompt"], "soles of both shoes")
			var response struct {
				TaskID string `json:"task_id"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			task, exists, err := model.GetByTaskId(userID, response.TaskID)
			require.NoError(t, err)
			require.True(t, exists)
			var meta map[string]any
			require.NoError(t, common.UnmarshalJsonStr(task.Properties.Input, &meta))
			assert.Equal(t, tier, meta["tier"])
			assert.Equal(t, map[string]any{}, meta["selection"])

			task.Status = model.TaskStatusSuccess
			task.PrivateData.ResultURL = "https://8.8.8.8/animal-result.png"
			require.NoError(t, task.Update())
			previousStorage := storage.GetStorageDriver()
			storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
			t.Cleanup(func() { storage.SetGlobalDriver(previousStorage) })
			_, err = service.PersistTaskArtifact(context.Background(), task, "image", "image/png", pngData.Bytes())
			require.NoError(t, err)
			artifactEngine := gin.New()
			artifactEngine.GET("/v1/tasks/:key/artifacts/:artifact_key/content", middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"), TaskArtifactContent)
			influencerReferenceHTTPClient = influencerReferenceLocalHTTPClient(t, artifactEngine)
			parentImage, err := service.BuildTaskArtifactContentURL(task.TaskID, "image")
			require.NoError(t, err)
			for _, invalidImage := range []string{
				"https://other.example/v1/tasks/" + task.TaskID + "/artifacts/image/content?access=invalid",
				"https://gateway.example/other?next=/v1/tasks/" + task.TaskID + "/artifacts/image/content",
				strings.Split(parentImage, "?")[0],
			} {
				invalidPayload, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "group": group, "parent_task_id": task.TaskID, "image_url": invalidImage})
				require.NoError(t, err)
				invalidRequest := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", bytes.NewReader(invalidPayload))
				invalidRequest.Header.Set("Content-Type", "application/json")
				invalidRequest.Header.Set("X-Omnimux-Group", group)
				invalidRecorder := httptest.NewRecorder()
				engine.ServeHTTP(invalidRecorder, invalidRequest)
				require.Equal(t, http.StatusBadRequest, invalidRecorder.Code, invalidRecorder.Body.String())
				require.Empty(t, requests, "invalid animal reference must not reach the provider boundary")
			}
			childPayload, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "group": group, "parent_task_id": task.TaskID, "image_url": parentImage})
			require.NoError(t, err)
			childRequest := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", bytes.NewReader(childPayload))
			childRequest.Header.Set("Content-Type", "application/json")
			childRequest.Header.Set("X-Omnimux-Group", group)
			childRecorder := httptest.NewRecorder()
			engine.ServeHTTP(childRecorder, childRequest)
			require.Equal(t, http.StatusOK, childRecorder.Code, childRecorder.Body.String())
			childDispatched := dispatchGenerationQueueTest(t, userID, childRecorder.Body.Bytes())
			t.Cleanup(func() { require.NoError(t, db.Model(childDispatched).Update("status", model.TaskStatusSuccess).Error) })
			require.Len(t, requests, 1)
			childCaptured := <-requests
			require.NoError(t, childCaptured.err)
			assert.Equal(t, "1920x1088", childCaptured.body["size"])
			assert.Equal(t, "gpt-image-2.5-async", childCaptured.body["model"])
			assert.Equal(t, []any{parentImage}, childCaptured.body["images"])
			assert.Equal(t, "/api/image-tasks/edits", childCaptured.path)
			assert.Contains(t, childCaptured.body["prompt"], "anthropomorphic")
			assert.NotContains(t, childCaptured.body["prompt"], "skin texture with visible pores")
			var childResponse struct {
				TaskID string `json:"task_id"`
			}
			require.NoError(t, common.Unmarshal(childRecorder.Body.Bytes(), &childResponse))
			child, exists, err := model.GetByTaskId(userID, childResponse.TaskID)
			require.NoError(t, err)
			require.True(t, exists)
			var childMeta map[string]any
			require.NoError(t, common.UnmarshalJsonStr(child.Properties.Input, &childMeta))
			assert.Equal(t, tier, childMeta["tier"])
			assert.Equal(t, task.TaskID, childMeta["parent_task_id"])
			assert.Equal(t, map[string]any{}, childMeta["selection"])
		})
	}
	legacy, exists, err := model.GetByTaskId(userID, "legacy-sheet")
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, legacyMeta, legacy.Properties.Input, "new submissions must not rewrite historical sheet dimensions")
}
