package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerSunburstNewRequestUsesReviewed1KWithoutDowngradingExplicit2K(t *testing.T) {
	for _, size := range []string{"", "16:9", "1024x576"} {
		t.Run("accepted/"+size, func(t *testing.T) {
			body, _, _, err := buildInfluencerSunburstSheet(influencerSheetRequest{Model: influencerImageModel, Brief: "a blue kettle", Size: size})
			require.NoError(t, err)
			assert.Equal(t, "1024x576", body["size"])
			assert.Equal(t, "url", body["response_format"])
		})
	}
	body, _, _, err := buildInfluencerSunburstSheet(influencerSheetRequest{Model: influencerImageModel, Brief: "a blue kettle", Size: "2048x1152"})
	require.Error(t, err)
	assert.Nil(t, body, "new explicit legacy pixels must not silently become a smaller request")
}

func TestInfluencerSunburstQueueKeepsOriginalSizeAcrossSerialization(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "sunburst-spec-restore", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	for _, size := range []string{"2048x1152", "1024x576"} {
		t.Run(size, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2.5-sunburst","image_mode":"text","prompt":"a blue kettle","size":"` + size + `","quality":"high","n":1,"output_format":"png","response_format":"url"}`)
			task := &model.Task{TaskID: "task_original_" + size, UserId: user.Id, Action: ActionInfluencerSheet, Group: influencerImageGroup, Properties: model.Properties{OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: body, ImageInputs: &model.GenerationImageInputs{Version: 1, Mode: "text"}}}}
			require.NoError(t, db.Create(task).Error)
			var recovered model.Task
			require.NoError(t, db.First(&recovered, task.ID).Error)
			c, err := generationWorkerContext(context.Background(), &recovered, &model.SystemTask{}, "runner")
			require.NoError(t, err)
			actual, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			assert.Equal(t, body, actual)
			assert.Equal(t, 1, recovered.PrivateData.GenerationQueue.ImageInputs.Version)
		})
	}
}

func TestInfluencerSunburstQueuedSelectionCannotChangeTheFrozenSpecification(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "sunburst-spec-candidate", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	ids := make(map[string]int)
	for i, size := range []string{"2048x1152", "1024x576"} {
		contract := `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"]}}`
		if size == "1024x576" {
			contract = `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"],"delivery":"b64_json"}}`
		}
		priority := int64(200 - i*100)
		channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: influencerImageModel, Group: influencerImageGroup, Priority: &priority, OtherSettings: contract}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)
		ids[size] = channel.Id
	}
	model.InitChannelCache()
	for _, size := range []string{"2048x1152", "1024x576"} {
		t.Run(size, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2.5-sunburst","image_mode":"text","prompt":"a blue kettle","size":"` + size + `","quality":"high","n":1,"output_format":"png","response_format":"url"}`)
			task := &model.Task{ID: 7, TaskID: "task_specification_" + size, UserId: user.Id, Action: ActionInfluencerSheet, Group: influencerImageGroup, Properties: model.Properties{OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: body, ImageInputs: &model.GenerationImageInputs{Version: 1, Mode: "text"}, Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{influencerImageGroup}, MaxAttempts: 2}}}}
			c, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
			require.NoError(t, err)
			require.NoError(t, helper.AppendImageTaskModeFilter(c, influencerImageModel))
			selected, _, selectionError := service.SelectChannelForRequest(c, influencerImageModel, &service.RetryParam{Ctx: c, TokenGroup: influencerImageGroup, ModelName: influencerImageModel})
			require.Nil(t, selectionError)
			require.NotNil(t, selected)
			assert.Equal(t, ids[size], selected.Id)
			assert.Equal(t, body, []byte(task.PrivateData.GenerationQueue.Body))
			assert.Equal(t, 2, task.PrivateData.GenerationQueue.Routing.MaxAttempts)
		})
	}
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	assert.Zero(t, after.RequestCount)
}

func TestInfluencerSunburstQueueRejectsIncompleteOrDifferentCanonicalBody(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "sunburst-spec-invalid", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	for field, invalid := range map[string]any{"model": "gpt-image-2.5-sunburst-1k", "size": "16:9", "quality": "medium", "n": 2, "output_format": "jpeg", "response_format": "b64_json"} {
		for _, missing := range []bool{false, true} {
			name := field + "/changed"
			if missing {
				name = field + "/missing"
			}
			t.Run(name, func(t *testing.T) {
				body := map[string]any{"model": influencerImageModel, "image_mode": "text", "prompt": "a blue kettle", "size": "2048x1152", "quality": "high", "n": 1, "output_format": "png", "response_format": "url"}
				body[field] = invalid
				if missing {
					delete(body, field)
				}
				original, err := common.Marshal(body)
				require.NoError(t, err)
				task := &model.Task{ID: 7, TaskID: "task_invalid_spec", UserId: user.Id, Action: ActionInfluencerSheet, Group: influencerImageGroup, Properties: model.Properties{OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: original, ImageInputs: &model.GenerationImageInputs{Version: 1, Mode: "text"}, Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{influencerImageGroup}, MaxAttempts: 2}}}}
				_, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
				require.Error(t, err)
				assert.Equal(t, original, []byte(task.PrivateData.GenerationQueue.Body), "recovery must not rewrite an invalid original into today's specification")
			})
		}
	}
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	assert.Zero(t, after.RequestCount)
}

func TestInfluencerSunburstEarlyDistributionUsesServer1KInsteadOfBusinessBody(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-image"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":0.2}`))
	user := model.User{Username: "sunburst-spec-prepare", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	var wantedID int
	for i, contract := range []string{
		`{"protocol":"openai_multipart","modes":["text"]}`,
		`{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"],"delivery":"b64_json"}`,
	} {
		priority := int64(200 - i*100)
		channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: influencerImageModel, Group: influencerImageGroup, Priority: &priority, OtherSettings: `{"image_task_contract":` + contract + `}`}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)
		if i == 1 {
			wantedID = channel.Id
		}
	}
	model.InitChannelCache()
	for _, rawSize := range []string{"", `,"size":"16:9"`} {
		t.Run(rawSize, func(t *testing.T) {
			reached := false
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
				c.Set("id", user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			}, PrepareInfluencerSheet, middleware.Distribute(), func(c *gin.Context) {
				reached = true
				assert.Equal(t, wantedID, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				var business map[string]any
				require.NoError(t, common.UnmarshalBodyReusable(c, &business))
				assert.NotContains(t, business, "quality", "early routing must not require the later canonical rewrite")
				assert.NotContains(t, business, "response_format")
				assert.NoError(t, helper.AppendImageTaskModeFilter(c, influencerImageModel))
				PostInfluencerSheet(c)
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","brief":"a blue kettle"`+rawSize+`}`))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.True(t, reached)
		})
	}
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	assert.Zero(t, after.RequestCount)
}

func TestInfluencerSunburstReferenceAndMultiViewPrepareKeepSourceAnd1KSpecification(t *testing.T) {
	for _, kind := range []string{"reference", "multiview"} {
		t.Run(kind, func(t *testing.T) {
			oldMemory := common.MemoryCacheEnabled
			common.MemoryCacheEnabled = true
			t.Cleanup(func() { common.MemoryCacheEnabled = oldMemory })
			engine, _ := setupInfluencerCLIAPI(t)
			system_setting.ServerAddress = "https://8.8.8.8"
			pngBytes := influencerPresetTestPNG(t, 16, 9)
			oldClient := influencerReferenceHTTPClient
			influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{pngBytes}}
			t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
			legacy := `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference"]}}`
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 1).Updates(map[string]any{"settings": legacy, "priority": 200}).Error)
			require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 1).Update("priority", 200).Error)
			priority := int64(100)
			channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: influencerImageModel, Group: "gpt-image-2.5-sunburst-test", Priority: &priority, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["reference"],"sizes":["1024x576"],"delivery":"b64_json"}}`}
			require.NoError(t, model.DB.Create(&channel).Error)
			require.NoError(t, model.DB.Create(&model.Ability{Group: channel.Group, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)
			model.InitChannelCache()
			imageURL := createInfluencerCLIImageUpload(t, 41, "uploads/spec-reference.png", pngBytes)
			path := "/pg/influencer/sheet"
			prepare, submit := PrepareInfluencerSheet, PostInfluencerSheet
			input := map[string]any{"model": influencerImageModel, "group": influencerImageGroup, "image_url": imageURL, "size": "16:9"}
			if kind == "multiview" {
				parent := model.Task{TaskID: "task_spec_parent", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"normal","brief":"a blue kettle"}`, OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.4.4/parent.png"}}
				require.NoError(t, model.DB.Create(&parent).Error)
				var err error
				input["image_url"], err = service.BuildTaskArtifactContentURL(parent.TaskID, "image")
				require.NoError(t, err)
				input["parent_task_id"] = parent.TaskID
				path, prepare, submit = "/pg/influencer/multiview", PrepareInfluencerMultiView, PostInfluencerMultiView
			}
			reached := false
			engine.POST(path, func(c *gin.Context) {
				c.Set("id", 41)
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			}, prepare, middleware.Distribute(), func(c *gin.Context) {
				reached = true
				assert.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				_, valid := influencerReferenceSelectedChannel(c)
				require.True(t, valid, "the selected-channel reference check must use the same 1K specification")
				submit(c)
			})
			data, err := common.Marshal(input)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.True(t, reached)
			var task model.Task
			require.NoError(t, model.DB.Where("status = ? AND user_id = ?", model.TaskStatusWaiting, 41).First(&task).Error)
			require.NotNil(t, task.PrivateData.GenerationQueue)
			var body map[string]any
			require.NoError(t, common.Unmarshal(task.PrivateData.GenerationQueue.Body, &body))
			assert.Equal(t, "1024x576", body["size"])
			assert.Equal(t, "reference", body["image_mode"])
			assert.Equal(t, "url", body["response_format"])
			source := task.PrivateData.GenerationQueue.ImageInputs.Source
			require.NotNil(t, source)
			frozen, err := service.ReadInfluencerFrozenImage(context.Background(), 41, *source)
			require.NoError(t, err)
			assert.Equal(t, pngBytes, frozen, "the smaller output specification must not resize or re-encode the reference")
			assert.Zero(t, task.Quota)
		})
	}
}
