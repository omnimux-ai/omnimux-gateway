package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Native routing reads the established abilities/channels names and scoped
// credentials have a fixed table name. Keep the full handler fixture isolated
// from environment-selected databases; receipt dialect tests stay separate.
func openInfluencerCLITestDatabase(t *testing.T, tables ...any) (*gorm.DB, common.DatabaseType) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(tables...))
	return db, common.DatabaseTypeSQLite
}

const influencerCLIRoutingBackup = "gpt-image-2.5-sunburst-reviewed-backup"

func setupInfluencerCLINativeRoutes(t *testing.T) (*gin.Engine, model.Channel, model.Channel) {
	t.Helper()
	engine, _ := setupInfluencerCLIAPI(t)
	oldAuto, oldRetry := setting.AutoGroups2JsonString(), common.RetryTimes
	common.RetryTimes = 1
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-reviewed-backup","gpt-image-2.5-sunburst-cheaper","gpt-image-2.5-sunburst-image"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Anchor","gpt-image-2.5-sunburst-reviewed-backup":"Backup","gpt-image-2.5-sunburst-cheaper":"Other price"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":0.2,"gpt-image-2.5-sunburst-reviewed-backup":0.2,"gpt-image-2.5-sunburst-cheaper":0.1}`))
	t.Cleanup(func() {
		common.RetryTimes = oldRetry
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
	})
	compatible := model.Channel{Name: "reviewed reference", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: influencerCLIRoutingBackup, Models: influencerImageModel, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference"],"sizes":["1024x576"]}}`}
	incompatible := model.Channel{Name: "unsupported neighbour", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: influencerCLIRoutingBackup, Models: influencerImageModel}
	for _, channel := range []*model.Channel{&compatible, &incompatible} {
		require.NoError(t, model.DB.Create(channel).Error)
		require.NoError(t, model.DB.Create(&model.Ability{Group: influencerCLIRoutingBackup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	}
	require.NoError(t, model.DB.Create(&model.Ability{Group: "gpt-image-2.5-sunburst-cheaper", Model: influencerImageModel, ChannelId: compatible.Id, Enabled: true}).Error)
	return engine, compatible, incompatible
}

func TestInfluencerCLINativeImageRoutingFreezesPolicyAndPixels(t *testing.T) {
	for _, kind := range []string{"character", "reference", "multiview"} {
		t.Run(kind, func(t *testing.T) {
			engine, _, incompatible := setupInfluencerCLINativeRoutes(t)
			oldClient := influencerReferenceHTTPClient
			influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{influencerPresetTestPNG(t, 16, 9)}}
			t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
			system_setting.ServerAddress = "https://8.8.8.8"
			path := "/characters"
			input := gin.H{"request_id": "native-image-" + kind, "brief": "a person in a red coat"}
			if kind == "reference" {
				input["image_url"] = createInfluencerCLIImageUpload(t, 41, "uploads/reference.png", influencerPresetTestPNG(t, 16, 9))
			} else if kind == "multiview" {
				parent := model.Task{TaskID: "native-parent", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"normal","brief":"a person in a red coat"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://upstream.example/parent.png"}}
				require.NoError(t, model.DB.Create(&parent).Error)
				path = "/characters/native-parent/multiview"
				delete(input, "brief")
			}
			recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, path, input)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			taskID := response["data"].(map[string]any)["task_id"].(string)
			var task model.Task
			require.NoError(t, model.DB.Where("task_id = ?", taskID).First(&task).Error)
			assert.Equal(t, influencerImageGroup, task.Group, "the original quote anchor is retained even when it has no channel")
			require.NotNil(t, task.PrivateData.GenerationQueue)
			require.NotNil(t, task.PrivateData.GenerationQueue.Routing)
			assert.Equal(t, model.GenerationRoutingAuto, task.PrivateData.GenerationQueue.Routing.Mode)
			assert.Equal(t, []string{influencerCLIRoutingBackup}, task.PrivateData.GenerationQueue.Routing.Groups, "a cheaper unrelated sale tier cannot replace the original quote")
			assert.Equal(t, 2, task.PrivateData.GenerationQueue.Routing.MaxAttempts)
			assert.Zero(t, task.Quota)
			var body, metadata map[string]any
			require.NoError(t, common.Unmarshal(task.PrivateData.GenerationQueue.Body, &body))
			require.NoError(t, common.UnmarshalJsonStr(task.Properties.Input, &metadata))
			assert.Equal(t, influencerImagePixels, body["size"])
			assert.Equal(t, influencerImageModel, body["model"])
			assert.Equal(t, "high", body["quality"])
			assert.Equal(t, "png", body["output_format"])
			assert.Equal(t, "url", body["response_format"])
			assert.Equal(t, float64(1), body["n"])
			require.NotNil(t, task.PrivateData.GenerationQueue.ImageInputs)
			expectedMode := "text"
			if kind != "character" {
				expectedMode = "reference"
				require.NotNil(t, task.PrivateData.GenerationQueue.ImageInputs.Source)
				assert.Equal(t, task.PrivateData.GenerationQueue.ImageInputs.Source.URL, body["image"])
				assert.NotEqual(t, input["image_url"], body["image"], "accepted media is an immutable server-owned copy")
			}
			assert.Equal(t, expectedMode, body["image_mode"])
			assert.Equal(t, expectedMode, task.PrivateData.GenerationQueue.ImageInputs.Mode)
			assert.Equal(t, "16:9", metadata["size"])
			assert.NotContains(t, recorder.Body.String(), influencerCLIRoutingBackup)
			worker, err := generationWorkerContext(context.Background(), &task, &model.SystemTask{}, "test-runner")
			require.NoError(t, err)
			assert.Equal(t, []string{influencerCLIRoutingBackup}, service.GetRequestAutoGroups(worker, "default"))
			assert.Contains(t, service.GetChannelConstraints(worker).Filters, dto.ChannelFilter{Kind: dto.FilterExcludeChannels, ExcludeChannelIDs: []int{incompatible.Id}})
			if kind != "character" {
				assert.NotEmpty(t, task.PrivateData.GenerationQueue.ReferencePreflight)
			}
			frozenBody, frozenRouting := string(task.PrivateData.GenerationQueue.Body), *task.PrivateData.GenerationQueue.Routing
			// Restoring the original receipt requires no live route or media and
			// cannot replace its frozen policy with a newly selected route.
			require.NoError(t, model.DB.Model(&model.Ability{}).Where("model = ?", influencerImageModel).Update("enabled", false).Error)
			t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "false")
			recorder, response = influencerCLIAPIRequest(t, engine, http.MethodPost, path, input)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, taskID, response["data"].(map[string]any)["task_id"])
			assert.Equal(t, true, response["data"].(map[string]any)["replayed"])
			var restored model.Task
			require.NoError(t, model.DB.Where("task_id = ?", taskID).First(&restored).Error)
			assert.Equal(t, frozenBody, string(restored.PrivateData.GenerationQueue.Body))
			assert.Equal(t, frozenRouting, *restored.PrivateData.GenerationQueue.Routing)
			_, err = generationWorkerContext(context.Background(), &restored, &model.SystemTask{}, "test-runner")
			assert.Error(t, err, "restoration must not expand a frozen policy after its routes disappear")
		})
	}
}

func TestInfluencerCLIOptionsNativeReferenceAvailabilityIsReadOnly(t *testing.T) {
	_, compatible, _ := setupInfluencerCLINativeRoutes(t)
	for _, enabled := range []bool{true, false} {
		require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", compatible.Id).Update("enabled", enabled).Error)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/influencer/v1/options", nil)
		c.Request.Header.Set("X-Omnimux-Group", "caller-choice")
		c.Set("group", "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		constraints := &dto.ChannelConstraints{Filters: []dto.ChannelFilter{{Kind: dto.FilterRequestPath, RequestPath: "/v1/images/generations"}}}
		common.SetContextKey(c, constant.ContextKeyChannelConstraints, constraints)
		GetInfluencerCLIOptions(c)
		assert.Equal(t, "caller-choice", c.GetHeader("X-Omnimux-Group"))
		assert.Len(t, constraints.Filters, 1, "capability checks must not append channel exclusions onto the real request")
		for _, key := range []string{ctxInfluencerRouting, ctxInfluencerQuoteGroup, string(constant.ContextKeyTokenAutoGroups), "influencer_reference_preflight", ctxInfluencerImageMode, ctxInfluencerImageInputs} {
			_, found := c.Get(key)
			assert.False(t, found, key)
		}
		assert.False(t, c.IsAborted())
		var response map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		availability := response["data"].(map[string]any)["availability"].(map[string]any)
		assert.Equal(t, enabled, availability["character"])
		assert.Equal(t, enabled, availability["reference_image"])
		assert.Equal(t, enabled, availability["multiview"])
		var count int64
		require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
		assert.Zero(t, count)
	}
}
