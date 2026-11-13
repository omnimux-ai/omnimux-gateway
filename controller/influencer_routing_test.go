package controller

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const influencerRoutingBackup = "gpt-image-2.5-reviewed-backup"

func setupInfluencerRoutingTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Task{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	oldDB, oldMemory, oldRetry := model.DB, common.MemoryCacheEnabled, common.RetryTimes
	oldAuto, oldUsable := setting.AutoGroups2JsonString(), setting.UserUsableGroups2JSONString()
	oldRatios, oldSpecial := ratio_setting.GroupRatio2JSONString(), ratio_setting.GroupGroupRatio2JSONString()
	model.DB, common.MemoryCacheEnabled, common.RetryTimes = db, true, 1
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","gpt-image-2.5-economy","gpt-image-2.5-reviewed-backup","gpt-image-2.5-pricy","nanobanana-economy","gpt-image-2.5-disabled"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"Member","gpt-image-2.5-economy":"Primary","gpt-image-2.5-reviewed-backup":"Backup","gpt-image-2.5-pricy":"Price mismatch","nanobanana-economy":"Shared","gpt-image-2.5-unlisted":"Unlisted","gpt-image-2.5-disabled":"Disabled"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"gpt-image-2.5-economy":0.2,"gpt-image-2.5-reviewed-backup":0.2,"gpt-image-2.5-pricy":0.7,"nanobanana-economy":0.2,"gpt-image-2.5-unlisted":0.2,"gpt-image-2.5-disabled":0.2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))
	t.Cleanup(func() {
		model.DB, common.MemoryCacheEnabled, common.RetryTimes = oldDB, oldMemory, oldRetry
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(oldSpecial))
		if oldMemory && oldDB != nil && oldDB.Migrator().HasTable(&model.Channel{}) && oldDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func addInfluencerRoutingChannel(t *testing.T, db *gorm.DB, group string, status int) {
	t.Helper()
	channel := model.Channel{Type: constant.ChannelTypeGoEasyAPI, Name: group, Key: "local-routing-fixture", Group: group, Models: "gpt-image-2.5", Status: status}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: group, Model: "gpt-image-2.5", ChannelId: channel.Id, Enabled: true}).Error)
}

func influencerRoutingContext(group string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{}`))
	common.SetContextKey(c, constant.ContextKeyUserGroup, group)
	return c
}

func TestInfluencerRoutingUsesNativeOrderAndEffectiveMemberPrice(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	for _, group := range []string{influencerReferenceGroup, influencerRoutingBackup, "default", "gpt-image-2.5-pricy", "nanobanana-economy", "gpt-image-2.5-unlisted"} {
		addInfluencerRoutingChannel(t, db, group, common.ChannelStatusEnabled)
	}
	addInfluencerRoutingChannel(t, db, "gpt-image-2.5-disabled", common.ChannelStatusManuallyDisabled)
	groups, err := influencerImageAutoGroups(influencerRoutingContext("default"), "gpt-image-2.5", influencerReferenceGroup)
	require.NoError(t, err)
	assert.Equal(t, []string{influencerReferenceGroup, influencerRoutingBackup}, groups)
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"gpt-image-2.5-economy":0.1,"gpt-image-2.5-reviewed-backup":0.15}}`))
	groups, err = influencerImageAutoGroups(influencerRoutingContext("vip"), "gpt-image-2.5", influencerReferenceGroup)
	require.NoError(t, err)
	assert.Equal(t, []string{influencerReferenceGroup}, groups, "a member-specific price mismatch must exclude the backup")
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"gpt-image-2.5-economy":0.1,"gpt-image-2.5-reviewed-backup":0.1}}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-reviewed-backup","gpt-image-2.5-economy"]`))
	groups, err = influencerImageAutoGroups(influencerRoutingContext("vip"), "gpt-image-2.5", influencerReferenceGroup)
	require.NoError(t, err)
	assert.Equal(t, []string{influencerRoutingBackup, influencerReferenceGroup}, groups, "supplier order comes from native routing, not sale ratios")
}

func TestInfluencerRoutingFailsClosedForMissingPermissionsOrCandidates(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	addInfluencerRoutingChannel(t, db, influencerRoutingBackup, common.ChannelStatusEnabled)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	_, err := influencerImageAutoGroups(influencerRoutingContext("default"), "gpt-image-2.5", influencerReferenceGroup)
	assert.Error(t, err)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-economy":"Primary"}`))
	_, err = influencerImageAutoGroups(influencerRoutingContext("default"), "gpt-image-2.5", influencerReferenceGroup)
	assert.Error(t, err, "a broader global Auto list must not bypass candidate permissions")
}

func TestInfluencerRoutingFirstDistributionCanSkipUnavailableAnchor(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "routing-accept", Status: common.UserStatusEnabled, Group: "default", Role: common.RoleCommonUser, Quota: 10000}
	require.NoError(t, db.Create(&user).Error)
	addInfluencerRoutingChannel(t, db, influencerRoutingBackup, common.ChannelStatusEnabled)
	model.InitChannelCache()
	engine := gin.New()
	reached := false
	engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
		c.Set("id", user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	}, PrepareInfluencerSheet, middleware.Distribute(), func(c *gin.Context) {
		reached = true
		assert.Equal(t, influencerRoutingBackup, common.GetContextKeyString(c, constant.ContextKeyAutoGroup))
		assert.Equal(t, "auto", common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
		var req influencerSheetRequest
		require.NoError(t, common.UnmarshalBodyReusable(c, &req))
		group, ok := influencerSheetGroup(c, req.Group)
		require.True(t, ok)
		assert.Equal(t, influencerReferenceGroup, group)
		assert.Equal(t, influencerReferenceGroup, c.GetHeader("X-Omnimux-Group"))
		routing := generationRequestRouting(c, group, "/pg/images/generations")
		assert.Equal(t, model.GenerationRoutingAuto, routing.Mode)
		assert.Equal(t, []string{influencerRoutingBackup}, routing.Groups)
		PostInfluencerSheet(c)
	})
	body := `{"model":"gpt-image-2.5@gpt-image-2.5-economy","group":"gpt-image-2.5-economy","tier":"normal","brief":"person"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omnimux-Group", influencerReferenceGroup)
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, reached)
	var accepted model.Task
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&accepted).Error)
	assert.Equal(t, model.TaskStatusWaiting, accepted.Status)
	assert.Equal(t, influencerReferenceGroup, accepted.Group, "the receipt freezes the original quote group even when distribution skipped it")
	require.NotNil(t, accepted.PrivateData.GenerationQueue)
	require.NotNil(t, accepted.PrivateData.GenerationQueue.Routing)
	assert.Equal(t, []string{influencerRoutingBackup}, accepted.PrivateData.GenerationQueue.Routing.Groups)
	assert.Zero(t, accepted.Quota, "acceptance must not charge or send to a provider")
	var frozen, composition map[string]any
	require.NoError(t, common.Unmarshal(accepted.PrivateData.GenerationQueue.Body, &frozen))
	require.NoError(t, common.UnmarshalJsonStr(accepted.Properties.Input, &composition))
	assert.Equal(t, "1920x1088", frozen["size"], "new workbench routing must preserve the original supplier pixel specification")
	assert.Equal(t, "16:9", composition["size"])
	// A consumer-supplied Auto value has no trusted quote context.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(body))
	req.Header.Set("X-Omnimux-Group", "auto")
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInfluencerRoutingQueueRestoreKeepsMaterialsAndLegacyGroup(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "routing-restore", Status: common.UserStatusEnabled, Group: "default", Role: common.RoleCommonUser}
	require.NoError(t, db.Create(&user).Error)
	addInfluencerRoutingChannel(t, db, influencerReferenceGroup, common.ChannelStatusEnabled)
	addInfluencerRoutingChannel(t, db, influencerRoutingBackup, common.ChannelStatusEnabled)
	body := []byte(`{"model":"gpt-image-2.5","prompt":"person","size":"16:9","metadata":{"seed":23}}`)
	task := &model.Task{UserId: user.Id, Action: ActionInfluencerSheet, Group: influencerReferenceGroup, Properties: model.Properties{OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: body, Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingAuto, Groups: []string{influencerReferenceGroup, influencerRoutingBackup}, MaxAttempts: 2}}}}
	c, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, "auto", c.GetHeader("X-Omnimux-Group"))
	assert.Equal(t, []string{influencerReferenceGroup, influencerRoutingBackup}, service.GetRequestAutoGroups(c, "default"))
	restoredBody, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, restoredBody)
	// Old records have no routing policy and keep their original explicit group.
	task.PrivateData.GenerationQueue.Routing = nil
	c, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, influencerReferenceGroup, c.GetHeader("X-Omnimux-Group"))
	_, exists := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
	assert.False(t, exists)
}

func TestInfluencerRoutingVideoAndReferenceStayWithinAcceptedGroup(t *testing.T) {
	oldRetry := common.RetryTimes
	common.RetryTimes = 5
	t.Cleanup(func() { common.RetryTimes = oldRetry })
	for _, resolution := range []string{"480p", "720p"} {
		group := influencerMotionGroups[resolution]
		routing := generationRequestRouting(influencerRoutingContext("default"), group, "/pg/video/generations")
		assert.Equal(t, model.GenerationRoutingGroup, routing.Mode)
		assert.Equal(t, []string{group}, routing.Groups)
		assert.Equal(t, 2, routing.MaxAttempts)
	}
	routing := generationRequestRouting(influencerRoutingContext("default"), influencerReferenceGroup, "/pg/images/edits")
	assert.Equal(t, []string{influencerReferenceGroup}, routing.Groups)
	assert.Equal(t, 1, routing.MaxAttempts)
	common.RetryTimes = 0
	routing = generationRequestRouting(influencerRoutingContext("default"), influencerMotionGroups["720p"], "/pg/video/generations")
	assert.Equal(t, 1, routing.MaxAttempts)
}

func TestInfluencerReferenceAutoEntrySkipsIncompatibleMembersAndUnavailableAnchor(t *testing.T) {
	for _, tc := range []struct {
		upstreamModel     string
		retries, attempts int
	}{{"gpt-image-2.5-flare", 1, 2}, {"gpt-image-2.5-sunburst", 1, 2}, {"gpt-image-2.5-flare", 0, 1}} {
		t.Run(tc.upstreamModel+"/attempts="+strconv.Itoa(tc.attempts), func(t *testing.T) {
			db := setupInfluencerRoutingTest(t)
			common.RetryTimes = tc.retries
			oldClient := influencerReferenceHTTPClient
			t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
			var pngBytes bytes.Buffer
			require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{pngBytes.Bytes()}}
			user := model.User{Username: "reference-auto", Status: common.UserStatusEnabled, Group: "default", Quota: 100000}
			require.NoError(t, db.Create(&user).Error)
			mapping := `{"gpt-image-2.5":"alias","alias":"` + tc.upstreamModel + `"}`
			compatible := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "reviewed", Key: "local-only", Models: "gpt-image-2.5", Group: influencerRoutingBackup, ModelMapping: &mapping}
			require.NoError(t, db.Create(&compatible).Error)
			incompatible := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "unsupported", Key: "local-only", Models: "gpt-image-2.5", Group: influencerRoutingBackup, Priority: common.GetPointer(int64(100))}
			require.NoError(t, db.Create(&incompatible).Error)
			for _, channel := range []model.Channel{compatible, incompatible} {
				require.NoError(t, db.Create(&model.Ability{Group: influencerRoutingBackup, Model: "gpt-image-2.5", ChannelId: channel.Id, Enabled: true, Priority: channel.Priority}).Error)
			}
			model.InitChannelCache()
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
				c.Set("id", user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
			}, PrepareInfluencerSheet, middleware.Distribute(), func(c *gin.Context) {
				assert.Equal(t, compatible.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				assert.Equal(t, "https://8.8.8.8/reference.png", c.GetString("influencer_reference_preflight"))
				PostInfluencerSheet(c)
			})
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5","group":"gpt-image-2.5-economy","tier":"normal","image_url":"https://8.8.8.8/reference.png","seed":23}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var task model.Task
			require.NoError(t, db.Where("user_id = ?", user.Id).First(&task).Error)
			require.NotNil(t, task.PrivateData.GenerationQueue)
			assert.Equal(t, influencerReferenceGroup, task.Group)
			assert.Equal(t, "/pg/images/edits", task.PrivateData.GenerationQueue.Path)
			assert.Equal(t, model.GenerationRoutingAuto, task.PrivateData.GenerationQueue.Routing.Mode)
			assert.Equal(t, []string{influencerRoutingBackup}, task.PrivateData.GenerationQueue.Routing.Groups)
			assert.Equal(t, tc.attempts, task.PrivateData.GenerationQueue.Routing.MaxAttempts)
			assert.Zero(t, task.Quota)
			var body map[string]any
			require.NoError(t, common.Unmarshal(task.PrivateData.GenerationQueue.Body, &body))
			assert.Equal(t, "https://8.8.8.8/reference.png", body["image"])
			assert.Equal(t, "1920x1088", body["size"])
			assert.NotContains(t, body, "quality")
		})
	}
}

func TestInfluencerReferenceAutoRestoreRechecksCompatibilityAndFrozenIntent(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "reference-restore", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	addInfluencerRoutingChannel(t, db, influencerReferenceGroup, common.ChannelStatusEnabled)
	mapping := `{"gpt-image-2.5":"gpt-image-2.5-flare"}`
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: influencerRoutingBackup, Models: "gpt-image-2.5", ModelMapping: &mapping}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: influencerRoutingBackup, Model: "gpt-image-2.5", ChannelId: channel.Id, Enabled: true}).Error)
	body := []byte(`{"model":"gpt-image-2.5","prompt":"preserve this subject","size":"16:9","image":"https://8.8.8.8/reference.png"}`)
	task := &model.Task{UserId: user.Id, Action: ActionInfluencerSheet, Group: influencerReferenceGroup, Properties: model.Properties{OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/edits", Body: body, ReferencePreflight: "https://8.8.8.8/reference.png", Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingAuto, Groups: []string{influencerRoutingBackup}, MaxAttempts: 2}}}}
	c, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, []string{influencerRoutingBackup}, service.GetRequestAutoGroups(c, user.Group))
	storedBody, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	assert.Equal(t, body, storedBody)
	require.NoError(t, db.Model(&channel).Update("model_mapping", `{"gpt-image-2.5":"gpt-image-2-beta"}`).Error)
	_, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	assert.Error(t, err, "new incompatible mapping cannot retain a frozen candidate or expand to the anchor")
	task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{influencerReferenceGroup}, MaxAttempts: 1}
	c, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, influencerReferenceGroup, c.GetHeader("X-Omnimux-Group"))
	assert.Equal(t, model.GenerationRoutingGroup, task.PrivateData.GenerationQueue.Routing.Mode)
	assert.Equal(t, 1, task.PrivateData.GenerationQueue.Routing.MaxAttempts)
	task.PrivateData.GenerationQueue.Routing.Mode = model.GenerationRoutingAuto
	task.PrivateData.GenerationQueue.ReferencePreflight = "https://8.8.4.4/different.png"
	_, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	assert.Error(t, err, "automatic reference requests must retain their exact validated reference")
}

func TestInfluencerReferenceCapabilitiesRequireAnExactResolvedSupplierModel(t *testing.T) {
	for _, tc := range []struct {
		name, mapping, settings, override, publicModel string
		channelType                                    int
		want                                           bool
	}{
		{name: "existing GoEasy contract", channelType: constant.ChannelTypeGoEasyAPI, want: true},
		{name: "Flare", mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare"}`, want: true},
		{name: "Sunburst", mapping: `{"gpt-image-2.5":"gpt-image-2.5-sunburst"}`, want: true},
		{name: "mapped chain", mapping: `{"gpt-image-2.5":"alias","alias":"gpt-image-2.5-flare"}`, want: true},
		{name: "bare public ID"},
		{name: "Beta is incompatible", mapping: `{"gpt-image-2.5":"gpt-image-2-beta"}`},
		{name: "malformed mapping", mapping: `{broken`},
		{name: "mapping cycle", mapping: `{"gpt-image-2.5":"alias","alias":"gpt-image-2.5"}`},
		{name: "mapped beyond approved SKU", mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare","gpt-image-2.5-flare":"unsupported"}`},
		{name: "wrong channel type", channelType: constant.ChannelTypeAzure, mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare"}`},
		{name: "wrong public model", publicModel: "another-image", mapping: `{"another-image":"gpt-image-2.5-flare"}`},
		{name: "dynamic model rewrite", mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare"}`, settings: `{"advanced_custom":{"model_by_param":{"gpt-image-2.5":{"param":"size","rules":[{"any_of":["16:9"],"model":"unsupported"}]}}}}`},
		{name: "body override", mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare"}`, override: `{"model":"unsupported"}`},
		{name: "malformed settings", mapping: `{"gpt-image-2.5":"gpt-image-2.5-flare"}`, settings: `{broken`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channelType := tc.channelType
			if channelType == 0 {
				channelType = constant.ChannelTypeOpenAI
			}
			modelName := tc.publicModel
			if modelName == "" {
				modelName = "gpt-image-2.5"
			}
			ch := &model.Channel{Type: channelType, ModelMapping: &tc.mapping, OtherSettings: tc.settings, ParamOverride: &tc.override}
			assert.Equal(t, tc.want, influencerReferenceChannelSupportsInput(ch, modelName))
		})
	}
}

func TestInfluencerReferenceAutoRestoreRejectsUnvalidatedContracts(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "reference-contract", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	addInfluencerRoutingChannel(t, db, influencerReferenceGroup, common.ChannelStatusEnabled)
	for _, tc := range []struct{ name, action, path, body, preflight string }{
		{"missing preflight", ActionInfluencerSheet, "/pg/images/edits", `{"model":"gpt-image-2.5","image":"https://8.8.8.8/ref.png"}`, ""},
		{"reference mismatch", ActionInfluencerSheet, "/pg/images/edits", `{"model":"gpt-image-2.5","image":"https://8.8.8.8/other.png"}`, "https://8.8.8.8/ref.png"},
		{"wrong action", ActionInfluencerMotion, "/pg/images/edits", `{"model":"gpt-image-2.5","image":"https://8.8.8.8/ref.png"}`, "https://8.8.8.8/ref.png"},
		{"video path", ActionInfluencerSheet, "/pg/video/generations", `{"model":"gpt-image-2.5","image":"https://8.8.8.8/ref.png"}`, "https://8.8.8.8/ref.png"},
		{"unvalidated generation reference", ActionInfluencerSheet, "/pg/images/generations", `{"model":"gpt-image-2.5","images":["https://8.8.8.8/ref.png"]}`, ""},
		{"extra edit reference", ActionInfluencerSheet, "/pg/images/edits", `{"model":"gpt-image-2.5","image":"https://8.8.8.8/ref.png","image_urls":["https://8.8.8.8/other.png"]}`, "https://8.8.8.8/ref.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &model.Task{UserId: user.Id, Action: tc.action, Group: influencerReferenceGroup, Properties: model.Properties{OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: tc.path, Body: []byte(tc.body), ReferencePreflight: tc.preflight, Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingAuto, Groups: []string{influencerReferenceGroup}, MaxAttempts: 2}}}}
			_, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
			assert.Error(t, err)
		})
	}
}
