package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"image"
	"image/png"
	"io"
	"strings"
)

// Sheet rejection is observed at the real handler, before native task submission.
type influencerImageTransport struct{ data []byte }

func (tr influencerImageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(tr.data)), Request: req}, nil
}

// Uses the native protected client and real TLS; only the socket is redirected to the local fixture.
func influencerReferenceLocalHTTPClient(t *testing.T, handler http.Handler) *http.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	client := service.NewInfluencerReferenceHTTPClient()
	transport := client.Transport.(*http.Transport)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, ServerName: "example.com"}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		server.Close()
	})
	return client
}

// Unknown provider responses retain the actual native reservation, not a quoted or refunded amount.
func assertInfluencerReferenceUnknownFunding(t *testing.T, task *model.Task, user *model.User, channelID int, expectedGroup string, expectedPrice float64, perCall bool) {
	t.Helper()
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Equal(t, model.TaskStatusDispatching, task.Status)
	assert.Equal(t, 50000, task.Quota)
	assert.Equal(t, 1000000-task.Quota, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Equal(t, channelID, task.ChannelId)
	assert.Equal(t, expectedGroup, task.Group)
	assert.Equal(t, "wallet", task.PrivateData.BillingSource)
	assert.Zero(t, task.PrivateData.SubscriptionId)
	assert.Zero(t, task.PrivateData.TokenId)
	require.NotNil(t, task.PrivateData.GenerationQueue)
	assert.NotZero(t, task.PrivateData.GenerationQueue.SendStartedAt)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.Equal(t, expectedPrice, task.PrivateData.BillingContext.ModelPrice)
	assert.Equal(t, float64(1), task.PrivateData.BillingContext.GroupRatio)
	assert.Equal(t, "gpt-image-2.5", task.PrivateData.BillingContext.OriginModelName)
	assert.Equal(t, perCall, task.PrivateData.BillingContext.PerCallBilling)
	assert.Contains(t, task.FailReason, "待核对")
	scanner := &model.SystemTask{TaskID: task.PrivateData.GenerationQueue.ClaimOwner}
	require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "test"))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	assert.Empty(t, claimed, "possible-send work must not be claimed again")
	dispatchInfluencerGeneration(context.Background(), task, scanner, "test")
	reloaded, exists, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, task.Status, reloaded.Status)
	assert.Equal(t, task.Quota, reloaded.Quota)
	assert.Equal(t, task.PrivateData.BillingContext, reloaded.PrivateData.BillingContext)
	require.NoError(t, model.DB.First(user, user.Id).Error)
	assert.Equal(t, 1000000-task.Quota, user.Quota)
}

func TestInfluencerReferenceOnlyUsesNativeGoEasyEditAndPersistsInput(t *testing.T) {
	for _, failFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "failed submit is not retried", false: "successful submit persists input"}[failFirst], func(t *testing.T) {
			previousDB, previousClient := model.DB, influencerReferenceHTTPClient
			previousPrices, previousGroups := ratio_setting.ModelPrice2JSONString(), ratio_setting.GroupRatio2JSONString()
			previousAuto, previousUsable := setting.AutoGroups2JsonString(), setting.UserUsableGroups2JSONString()
			require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-economy"]`))
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-image-2.5-economy":"Reference"}`))
			t.Cleanup(func() {
				require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAuto))
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
			})
			previousRedis := common.RedisEnabled
			common.RedisEnabled = false
			t.Cleanup(func() { common.RedisEnabled = previousRedis })
			previousLog := common.LogConsumeEnabled
			fetch := system_setting.GetFetchSetting()
			oldFetch := *fetch
			t.Cleanup(func() {
				model.DB = previousDB
				influencerReferenceHTTPClient = previousClient
				common.LogConsumeEnabled = previousLog
				*fetch = oldFetch
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousPrices))
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
			})
			fetch.EnableSSRFProtection = false // local upstream boundary only; feature URL checks remain strict.
			service.InitHttpClient()
			common.LogConsumeEnabled = false
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2.5":0.1}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-economy":1}`))
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			model.DB = db
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.UserSubscription{}, &model.Channel{}, &model.Ability{}, &model.SystemTask{}, &model.SystemTaskLock{}))
			user := model.User{Status: common.UserStatusEnabled, Username: "reference-test", AffCode: "reference-test", Quota: 1000000, Group: influencerReferenceGroup}
			require.NoError(t, db.Create(&user).Error)
			previousRetry, previousCache := common.RetryTimes, common.MemoryCacheEnabled
			common.RetryTimes, common.MemoryCacheEnabled = 1, false
			t.Cleanup(func() { common.RetryTimes, common.MemoryCacheEnabled = previousRetry, previousCache })
			calls := 0
			var captured map[string]any
			var path string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				require.NoError(t, common.DecodeJson(r.Body, &captured))
				w.Header().Set("Content-Type", "application/json")
				calls++
				if failFirst && calls == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"temporary upstream failure"}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"upstream-reference","status":"pending"}`))
			}))
			defer upstream.Close()
			ch := model.Channel{Type: constant.ChannelTypeGoEasyAPI, Name: "reference candidate", Status: common.ChannelStatusEnabled, Key: "test", BaseURL: &upstream.URL, Models: "gpt-image-2.5", Group: influencerReferenceGroup}
			require.NoError(t, db.Create(&ch).Error)
			require.NoError(t, db.Create(&model.Ability{Group: influencerReferenceGroup, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
			var pngData bytes.Buffer
			require.NoError(t, png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			influencerReferenceHTTPClient = influencerReferenceLocalHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(pngData.Bytes())
			}))
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, influencerReferenceGroup)
				common.SetContextKey(c, constant.ContextKeyUsingGroup, influencerReferenceGroup)
				common.SetContextKey(c, constant.ContextKeyTokenGroup, influencerReferenceGroup)
			}, PrepareInfluencerSheet, func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, influencerReferenceGroup)
				require.Nil(t, middleware.SetupContextForSelectedChannel(c, &ch, "gpt-image-2.5"))
			}, PostInfluencerSheet)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5@gpt-image-2.5-economy","tier":"total","selection":{},"brief":"","image_url":"https://8.8.8.8/reference.png","seed":42}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Omnimux-Group", influencerReferenceGroup)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			stored := dispatchGenerationQueueTest(t, user.Id, rec.Body.Bytes())
			assert.Equal(t, 1, calls)
			assert.Equal(t, "/api/image-tasks/edits", path)
			assert.Equal(t, "gpt-image-2.5-async", captured["model"])
			assert.Equal(t, []any{"https://8.8.8.8/reference.png"}, captured["images"])
			assert.Contains(t, captured["prompt"], "a different person")
			assert.Contains(t, captured["prompt"], "Extreme caricature")
			assert.NotContains(t, captured, "group")
			assert.NotContains(t, captured, "quality")
			var response map[string]any
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
			var task model.Task
			require.NoError(t, db.Where("task_id = ?", response["task_id"]).First(&task).Error)
			assert.Equal(t, ActionInfluencerSheet, task.Action)
			var input map[string]any
			require.NoError(t, common.UnmarshalJsonStr(task.Properties.Input, &input))
			assert.Equal(t, "total", input["tier"])
			assert.Equal(t, "https://8.8.8.8/reference.png", input["image_url"])
			assert.EqualValues(t, 42, input["seed"])
			assert.Empty(t, input["brief"])
			if failFirst {
				assertInfluencerReferenceUnknownFunding(t, stored, &user, ch.Id, influencerReferenceGroup, 0.1, true)
				assert.Equal(t, 1, calls, "a non-idempotent edit must not be submitted again")
				var tasks int64
				require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
				assert.EqualValues(t, 1, tasks)
			} else {
				require.NoError(t, db.First(&user, user.Id).Error)
				assert.Equal(t, 50000, stored.Quota)
				assert.Equal(t, 950000, user.Quota)
				assert.Equal(t, stored.Quota, user.UsedQuota)
			}

		})
	}
}

func TestInfluencerSheetRejectsGroupConflictsBeforeDistribution(t *testing.T) {
	for _, tc := range []struct{ name, body, header, legacy, code string }{
		{"missing", `{"model":"gpt-image-2.5","tier":"normal","brief":"a dancer"}`, "", "", "explicit_group_required"},
		{"automatic", `{"model":"gpt-image-2.5","group":"auto","tier":"normal","brief":"a dancer"}`, "", "", "explicit_group_required"},
		{"body header conflict", `{"model":"gpt-image-2.5","group":"economy","tier":"normal","brief":"a dancer"}`, "official", "", "group_conflict"},
		{"two headers conflict", `{"model":"gpt-image-2.5","tier":"normal","brief":"a dancer"}`, "economy", "official", "group_conflict"},
		{"unsupported reference", `{"model":"gpt-image-2.5","tier":"normal","image_url":"https://8.8.8.8/image"}`, "gpt-image-2.5-official", "", "reference_group_unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", PrepareInfluencerSheet, func(c *gin.Context) { reached = true })
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Omnimux-Group", tc.header)
			req.Header.Set("New-Api-Group", tc.legacy)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, 400, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.code)
			assert.False(t, reached)
		})
	}
}

func TestInfluencerSheetAcceptsMatchingExplicitGroupHeaders(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	addInfluencerRoutingChannel(t, db, influencerReferenceGroup, common.ChannelStatusEnabled)
	for _, body := range []string{`{"model":"gpt-image-2.5","tier":"normal","brief":"a dancer"}`, `{"model":"gpt-image-2.5","group":"gpt-image-2.5-economy","tier":"normal","brief":"a dancer"}`} {
		engine := gin.New()
		engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			c.Next()
		}, PrepareInfluencerSheet, func(c *gin.Context) {
			assert.Equal(t, "auto", c.GetHeader("X-Omnimux-Group"))
			assert.Equal(t, influencerReferenceGroup, c.GetString(ctxInfluencerQuoteGroup))
			assert.Empty(t, c.GetHeader("New-Api-Group"))
			group, ok := influencerSheetGroup(c, influencerReferenceGroup)
			require.True(t, ok)
			assert.Equal(t, influencerReferenceGroup, group)
			c.Status(204)
		})
		req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Omnimux-Group", influencerReferenceGroup)
		req.Header.Set("New-Api-Group", influencerReferenceGroup)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		assert.Equal(t, 204, rec.Code, rec.Body.String())
	}
}

func TestInfluencerReferenceRejectionsLeaveWalletAndTasksUnchanged(t *testing.T) {
	previousDB, previousClient := model.DB, influencerReferenceHTTPClient
	previousGroups := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-economy":1}`))
	previousAuto, previousUsable := setting.AutoGroups2JsonString(), setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-economy"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-image-2.5-economy":"Reference"}`))
	t.Cleanup(func() {
		model.DB = previousDB
		influencerReferenceHTTPClient = previousClient
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAuto))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
	})
	for _, tc := range []struct {
		name        string
		channelType int
		image       string
		data        []byte
		status      int
		code        string
	}{
		{"unknown route", 0, "https://8.8.8.8/a", nil, 503, "reference_route_unavailable"},
		{"Evolink incompatible", constant.ChannelTypeOpenAI, "https://8.8.8.8/a", nil, 503, "reference_route_unavailable"},
		{"private image", constant.ChannelTypeGoEasyAPI, "https://127.0.0.1/a", nil, 400, "invalid_reference_image"},
		{"undecodable image", constant.ChannelTypeGoEasyAPI, "https://8.8.8.8/a", []byte("bad"), 400, "invalid_reference_image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			model.DB = db
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.Task{}, &model.Channel{}, &model.Ability{}))
			user := model.User{Status: common.UserStatusEnabled, Username: tc.name, AffCode: tc.name, Quota: 1000000}
			require.NoError(t, db.Create(&user).Error)
			if tc.channelType != 0 {
				ch := model.Channel{Type: tc.channelType, Status: common.ChannelStatusEnabled}
				require.NoError(t, db.Create(&ch).Error)
				require.NoError(t, db.Create(&model.Ability{Group: influencerReferenceGroup, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
			}
			influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{tc.data}}
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", PrepareInfluencerSheet, PostInfluencerSheet)
			payload, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "group": influencerReferenceGroup, "tier": "normal", "image_url": tc.image})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tc.code)
			var tasks int64
			require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
			assert.Zero(t, tasks)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 1000000, user.Quota)
			assert.Zero(t, user.UsedQuota)
		})
	}
}

func TestInfluencerOnlyImageMultiViewPrechargeGates(t *testing.T) {
	oldDB, oldClient := model.DB, influencerReferenceHTTPClient
	oldSQLite, oldMaster := common.SQLitePath, common.IsMasterNode
	oldType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	common.SQLitePath, common.IsMasterNode = ":memory:", false
	require.NoError(t, model.InitDB())
	initSQL, err := model.DB.DB()
	require.NoError(t, err)
	require.NoError(t, initSQL.Close())
	oldAddress, oldPublic, oldSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	oldCache, oldRedis, oldRetry, oldLog := common.MemoryCacheEnabled, common.RedisEnabled, common.RetryTimes, common.LogConsumeEnabled
	oldAuto, oldUsable := setting.AutoGroups2JsonString(), setting.UserUsableGroups2JSONString()
	oldPrices, oldGroups := ratio_setting.ModelPrice2JSONString(), ratio_setting.GroupRatio2JSONString()
	fetch := system_setting.GetFetchSetting()
	oldFetch := *fetch
	t.Cleanup(func() {
		model.DB, influencerReferenceHTTPClient = oldDB, oldClient
		common.SQLitePath, common.IsMasterNode = oldSQLite, oldMaster
		common.SetDatabaseTypes(oldType, oldLogType)
		system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = oldAddress, oldPublic, oldSecret
		common.MemoryCacheEnabled, common.RedisEnabled, common.RetryTimes, common.LogConsumeEnabled = oldCache, oldRedis, oldRetry, oldLog
		*fetch = oldFetch
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
	})
	system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = "https://gateway.example/prefix", "", "local-test-secret"
	common.MemoryCacheEnabled, common.RedisEnabled, common.RetryTimes, common.LogConsumeEnabled = false, false, 1, false
	fetch.EnableSSRFProtection = false
	service.InitHttpClient()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2.5":0.1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-economy":1,"gpt-image-2.5-reviewed-backup":1,"other":1}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-economy","gpt-image-2.5-reviewed-backup"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-image-2.5-economy":"Primary","gpt-image-2.5-reviewed-backup":"Backup"}`))
	validURL, err := service.BuildTaskArtifactContentURL("parent-only-image", "image")
	require.NoError(t, err)
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	for _, tc := range []struct {
		name, imageURL, bodyGroup, headerGroup, selectedGroup, code, requestSize string
		channelType                                                              int
		badBytes, upstreamFailure, autoRoute, describedParent                    bool
		status                                                                   int
	}{
		{name: "query is not source", imageURL: "https://gateway.example/prefix/x?next=/v1/tasks/parent-only-image/artifacts/image/content", code: "invalid_request", status: 400},
		{name: "foreign origin", imageURL: strings.Replace(validURL, "gateway.example", "other.example", 1), code: "invalid_request", status: 400},
		{name: "foreign port", imageURL: strings.Replace(validURL, "gateway.example", "gateway.example:8443", 1), code: "invalid_request", status: 400},
		{name: "wrong artifact", imageURL: strings.Replace(validURL, "/image/content", "/video/content", 1), code: "invalid_request", status: 400},
		{name: "wrong parent", imageURL: strings.Replace(validURL, "parent-only-image/", "parent-only-image-2/", 1), code: "invalid_request", status: 400},
		{name: "http origin", imageURL: strings.Replace(validURL, "https:", "http:", 1), code: "invalid_request", status: 400},
		{name: "invalid access", imageURL: strings.Split(validURL, "?")[0] + "?access=bad", code: "invalid_request", status: 400},
		{name: "body header conflict", bodyGroup: "other", headerGroup: influencerReferenceGroup, code: "group_conflict", status: 400},
		{name: "selected group conflict", selectedGroup: "other", code: "group_conflict", status: 400},
		{name: "wrong provider", channelType: constant.ChannelTypeOpenAI, code: "reference_route_unavailable", status: 503},
		{name: "unreadable image", badBytes: true, code: "invalid_reference_image", status: 400},
		{name: "single failed submit", upstreamFailure: true, status: 200},
		{name: "real distribution succeeds", status: 200},
		{name: "automatic image-only parent skips absent anchor", autoRoute: true, status: 200},
		{name: "automatic explicit landscape preserves original pixels", autoRoute: true, requestSize: "16:9", status: 200},
		{name: "automatic unreviewed portrait rejected", autoRoute: true, requestSize: "9:16", code: "invalid_request", status: 400},
		{name: "legacy group portrait preserved", requestSize: "9:16", status: 200},
		{name: "automatic described parent skips absent anchor", autoRoute: true, describedParent: true, status: 200},
		{name: "automatic unknown submit never repeats", autoRoute: true, upstreamFailure: true, status: 200},
		{name: "automatic route still rejects invalid signature", autoRoute: true, imageURL: strings.Split(validURL, "?")[0] + "?access=bad", code: "invalid_request", status: 400},
		{name: "automatic route still rejects invalid bytes", autoRoute: true, badBytes: true, code: "invalid_reference_image", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			model.DB = db
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.UserSubscription{}, &model.Channel{}, &model.Ability{}, &model.SystemTask{}, &model.SystemTaskLock{}))
			user := model.User{Status: common.UserStatusEnabled, Username: tc.name, AffCode: tc.name, Quota: 1000000, Group: influencerReferenceGroup}
			require.NoError(t, db.Create(&user).Error)
			parent := model.Task{TaskID: "parent-only-image", UserId: user.Id, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"normal","image_url":"https://8.8.8.8/input.png"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/result.png"}}
			if tc.describedParent {
				parent.Properties.Input = `{"tier":"normal","selection":{"gender":["female"]},"brief":"a dancer"}`
			}
			require.NoError(t, db.Create(&parent).Error)
			previousStorage := storage.GetStorageDriver()
			driver := storage.NewLocalStorageDriver(t.TempDir())
			storage.SetGlobalDriver(driver)
			t.Cleanup(func() { storage.SetGlobalDriver(previousStorage) })
			artifactBytes := data.Bytes()
			if tc.badBytes {
				artifactBytes = []byte("bad image")
			}
			_, err = service.PersistTaskArtifact(context.Background(), &parent, "image", "image/png", artifactBytes)
			require.NoError(t, err)
			posts := 0
			var fetches atomic.Int32
			var captured map[string]any
			var path string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				path = r.URL.Path
				require.NoError(t, common.DecodeJson(r.Body, &captured))
				if tc.upstreamFailure {
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":"accepted outcome unknown"}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"multiview-upstream","status":"pending"}`))
			}))
			defer upstream.Close()
			channelType := tc.channelType
			if channelType == 0 {
				channelType = constant.ChannelTypeGoEasyAPI
			}
			ch := model.Channel{Type: channelType, Status: common.ChannelStatusEnabled, Key: "test", BaseURL: &upstream.URL, Models: "gpt-image-2.5", Group: influencerReferenceGroup}
			if tc.autoRoute {
				withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.1)) * image_count`})
				ch.Type = constant.ChannelTypeOpenAI
				ch.Group = influencerRoutingBackup
				ch.ModelMapping = common.GetPointer(`{"gpt-image-2.5":"gpt-image-2.5-flare"}`)
			}
			require.NoError(t, db.Create(&ch).Error)
			require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
			artifactEngine := gin.New()
			artifactEngine.GET("/prefix/v1/tasks/:key/artifacts/:artifact_key/content", middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"), TaskArtifactContent)
			influencerReferenceHTTPClient = influencerReferenceLocalHTTPClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fetches.Add(1)
				artifactEngine.ServeHTTP(w, r)
			}))
			engine := gin.New()
			engine.Use(gin.Recovery())
			auth := func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, influencerReferenceGroup)
				common.SetContextKey(c, constant.ContextKeyUsingGroup, influencerReferenceGroup)
				common.SetContextKey(c, constant.ContextKeyTokenGroup, influencerReferenceGroup)
			}
			if tc.autoRoute {
				engine.POST("/pg/influencer/multiview", auth, PrepareInfluencerMultiView, middleware.Distribute(), PostInfluencerMultiView)
			} else if tc.selectedGroup != "" || tc.channelType != 0 {
				engine.POST("/pg/influencer/multiview", auth, func(c *gin.Context) {
					require.Nil(t, middleware.SetupContextForSelectedChannel(c, &ch, "gpt-image-2.5"))
					if tc.selectedGroup != "" {
						common.SetContextKey(c, constant.ContextKeyUsingGroup, tc.selectedGroup)
					}
				}, PostInfluencerMultiView)
			} else {
				engine.POST("/pg/influencer/multiview", auth, middleware.Distribute(), PostInfluencerMultiView)
			}
			imageURL, group := tc.imageURL, tc.bodyGroup
			if imageURL == "" {
				imageURL = validURL
			}
			if group == "" {
				group = influencerReferenceGroup
			}
			payload, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "group": group, "parent_task_id": parent.TaskID, "image_url": imageURL, "size": tc.requestSize})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			header := tc.headerGroup
			if header == "" {
				header = influencerReferenceGroup
			}
			req.Header.Set("X-Omnimux-Group", header)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			var stored *model.Task
			if tc.status == 200 {
				if tc.autoRoute {
					var child model.Task
					require.NoError(t, db.Where("task_id <> ?", parent.TaskID).First(&child).Error)
					var frozen map[string]any
					require.NoError(t, common.Unmarshal(child.PrivateData.GenerationQueue.Body, &frozen))
					assert.Equal(t, "1920x1088", frozen["size"])
				}
				stored = dispatchGenerationQueueTest(t, user.Id, rec.Body.Bytes())
				if tc.autoRoute {
					assert.Equal(t, "1920x1088", captured["size"])
					assert.Equal(t, "/v1/images/generations", path)
					assert.Equal(t, "gpt-image-2.5-flare", captured["model"])
					assert.Equal(t, []any{validURL}, captured["image_urls"])
					assert.NotContains(t, captured, "image")
				} else {
					assert.Equal(t, "/api/image-tasks/edits", path)
					assert.Equal(t, "gpt-image-2.5-async", captured["model"])
					assert.Equal(t, []any{validURL}, captured["images"])
				}
				assert.NotContains(t, captured, "group")
				assert.NotContains(t, captured, "quality")
				var input map[string]any
				require.NoError(t, common.UnmarshalJsonStr(stored.Properties.Input, &input))
				assert.Equal(t, "multiview", input["kind"])
				assert.Equal(t, parent.TaskID, input["parent_task_id"])
				assert.Equal(t, validURL, input["image_url"])
				if tc.requestSize == "9:16" {
					assert.Equal(t, "9:16", input["size"])
					assert.Equal(t, "1088x1920", captured["size"], "legacy Group requests preserve their portrait input contract")
				} else {
					assert.Equal(t, "16:9", input["size"])
				}
				assert.Equal(t, user.Id, stored.UserId)
			}
			if tc.code != "" {
				assert.Contains(t, rec.Body.String(), tc.code)
			}
			var count int64
			require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
			require.NoError(t, db.First(&user, user.Id).Error)
			if tc.status != 200 || tc.upstreamFailure {
				if tc.upstreamFailure {
					assert.EqualValues(t, 2, count)
				} else {
					assert.EqualValues(t, 1, count)
				}
				if tc.upstreamFailure {
					if tc.autoRoute {
						assertInfluencerReferenceUnknownFunding(t, stored, &user, ch.Id, influencerRoutingBackup, 0, false)
						require.NotNil(t, stored.PrivateData.BillingContext.TieredSnapshot)
						assert.Equal(t, `tier("image", fixed(0.1)) * image_count`, stored.PrivateData.BillingContext.TieredSnapshot.ExprString)
					} else {
						assertInfluencerReferenceUnknownFunding(t, stored, &user, ch.Id, influencerReferenceGroup, 0.1, true)
					}
				} else {
					assert.Equal(t, 1000000, user.Quota)
				}
				assert.Zero(t, user.UsedQuota)
				if tc.upstreamFailure {
					assert.Equal(t, 1, posts)
					assert.EqualValues(t, 2, fetches.Load(), "acceptance and dispatch must read the current parent artifact")
				} else {
					assert.Zero(t, posts)
				}
			} else {
				assert.Equal(t, 1, posts)
				assert.EqualValues(t, 2, fetches.Load(), "acceptance and dispatch must read the current parent artifact")
				assert.EqualValues(t, 2, count)
				assert.Equal(t, 50000, stored.Quota)
				assert.Equal(t, 1000000-stored.Quota, user.Quota)
				assert.Equal(t, stored.Quota, user.UsedQuota)
				var child model.Task
				require.NoError(t, db.Where("task_id <> ?", parent.TaskID).First(&child).Error)
				var childInput map[string]any
				require.NoError(t, common.UnmarshalJsonStr(child.Properties.Input, &childInput))
				if tc.describedParent {
					assert.Equal(t, map[string]any{"gender": []any{"female"}}, childInput["selection"])
				} else {
					assert.Equal(t, map[string]any{}, childInput["selection"], "legacy image-only parents persist an empty selection object, not null")
				}
				assert.Equal(t, "normal", childInput["tier"])
				assert.Equal(t, parent.TaskID, childInput["parent_task_id"])
				child.Status = model.TaskStatusSuccess
				child.PrivateData.ResultURL = "https://8.8.8.8/child.png"
				// The reusable motion harness runs as user 41; this isolated fixture
				// gives the successful child that owner without altering the real handler.
				child.UserId = 41
				require.NoError(t, child.Update())
				childURL, err := service.BuildTaskArtifactContentURL(child.TaskID, "image")
				require.NoError(t, err)
				motion := validMotionRequest()
				motion["source_task_id"] = child.TaskID
				motion["subject_image_url"] = childURL
				motionRecorder, probe := runMotionPrepare(t, motion)
				require.Equal(t, http.StatusNoContent, motionRecorder.Code, motionRecorder.Body.String())
				assert.Equal(t, "human", probe.meta["subject_character_type"])
				assert.Equal(t, child.TaskID, probe.meta["source_task_id"])
			}
		})
	}
}

type referenceControllerFetchFunc func(*http.Request) (*http.Response, error)

func (f referenceControllerFetchFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInfluencerSheetNormalizesModelGroupSources(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	addInfluencerRoutingChannel(t, db, influencerReferenceGroup, common.ChannelStatusEnabled)
	for _, tc := range []struct {
		name, model, bodyGroup, header, legacy string
		status                                 int
	}{
		{"suffix only", "gpt-image-2.5@gpt-image-2.5-economy", "", "", "", 204},
		{"all matching", " gpt-image-2.5 @ gpt-image-2.5-economy ", influencerReferenceGroup, influencerReferenceGroup, influencerReferenceGroup, 204},
		{"suffix body conflict", "gpt-image-2.5@other", influencerReferenceGroup, "", "", 400},
		{"suffix header conflict", "gpt-image-2.5@other", "", influencerReferenceGroup, "", 400},
		{"suffix legacy conflict", "gpt-image-2.5@other", "", "", influencerReferenceGroup, 400},
		{"suffix auto", "gpt-image-2.5@auto", "", "", "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				c.Next()
			}, PrepareInfluencerSheet, func(c *gin.Context) {
				var normalized influencerSheetRequest
				require.NoError(t, common.UnmarshalBodyReusable(c, &normalized))
				assert.Equal(t, "gpt-image-2.5", normalized.Model)
				assert.Equal(t, "auto", c.GetHeader("X-Omnimux-Group"))
				assert.Equal(t, influencerReferenceGroup, normalized.Group)
				group, ok := influencerSheetGroup(c, normalized.Group)
				require.True(t, ok)
				assert.Equal(t, influencerReferenceGroup, group)
				c.Status(204)
			})
			body, err := common.Marshal(map[string]any{"model": tc.model, "group": tc.bodyGroup, "tier": "normal", "brief": "a dancer"})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Omnimux-Group", tc.header)
			req.Header.Set("New-Api-Group", tc.legacy)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.status == 400 && tc.name != "suffix auto" {
				assert.Contains(t, rec.Body.String(), "group_conflict")
			}
		})
	}
}

func TestInfluencerSheetRequiresExplicitGroupBeforeSubmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.POST("/pg/influencer/sheet", PostInfluencerSheet)
	req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", bytes.NewBufferString(`{"model":"gpt-image-2.5","tier":"normal","brief":"a dancer"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "explicit_group_required")
}

func TestInfluencerReferenceAutoUsesNativePreSendFailoverWithoutLosingMaterial(t *testing.T) {
	for _, first := range []string{"not sent", "accepted", "unknown service error", "legacy group not sent"} {
		t.Run(first, func(t *testing.T) {
			db := setupInfluencerRoutingTest(t)
			oldClient, oldLogDB := influencerReferenceHTTPClient, model.LOG_DB
			oldRedis, oldBatch, oldLog, oldMaster := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode
			fetch := system_setting.GetFetchSetting()
			oldFetch := *fetch
			t.Cleanup(func() {
				influencerReferenceHTTPClient, model.LOG_DB = oldClient, oldLogDB
				common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = oldRedis, oldBatch, oldLog, oldMaster
				*fetch = oldFetch
			})
			common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = false, false, false, false, false
			t.Setenv("LOG_SQL_DSN", "")
			require.NoError(t, model.InitLogDB())
			require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}))
			fetch.EnableSSRFProtection = false // All supplier sockets below are local fixtures.
			service.InitHttpClient()
			withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.013072)) * image_count`})
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-economy":0.112455,"gpt-image-2.5-reviewed-backup":0.112455}`))
			var imageBytes bytes.Buffer
			require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{imageBytes.Bytes()}}
			user := model.User{Username: "reference-native", AffCode: "reference-native", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
			require.NoError(t, db.Create(&user).Error)
			var primaryCalls, backupCalls atomic.Int32
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				primaryCalls.Add(1)
				assert.Equal(t, "/api/image-tasks/edits", r.URL.Path)
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, []any{"https://8.8.8.8/reference.png"}, body["images"])
				assert.Equal(t, "1920x1088", body["size"])
				assert.NotContains(t, body, "quality")
				w.Header().Set("Content-Type", "application/json")
				if first == "unknown service error" {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = io.WriteString(w, `{"error":"result unknown"}`)
					return
				}
				_, _ = io.WriteString(w, `{"id":"reference-primary-accepted","status":"pending","quality":"auto"}`)
			}))
			defer primary.Close()
			backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backupCalls.Add(1)
				assert.Equal(t, "/v1/images/generations", r.URL.Path)
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, "gpt-image-2.5-flare", body["model"])
				assert.Equal(t, []any{"https://8.8.8.8/reference.png"}, body["image_urls"])
				assert.Equal(t, "1920x1088", body["size"])
				assert.Contains(t, body["prompt"], "a different person")
				assert.Equal(t, map[string]any{"seed": float64(23)}, body["metadata"])
				assert.NotContains(t, body, "quality", "an earlier provider must not replace the original omitted quality")
				assert.NotContains(t, body, "image")
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"reference-backup-accepted","status":"pending"}`)
			}))
			defer backup.Close()
			primaryURL := primary.URL
			if first == "not sent" || first == "legacy group not sent" {
				primaryURL = ":bad-url"
			}
			mapping := `{"gpt-image-2.5":"gpt-image-2.5-flare"}`
			a := model.Channel{Name: "primary", Type: constant.ChannelTypeGoEasyAPI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: "gpt-image-2.5", Group: influencerReferenceGroup, BaseURL: &primaryURL}
			b := model.Channel{Name: "backup", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: "gpt-image-2.5", Group: influencerRoutingBackup, BaseURL: &backup.URL, ModelMapping: &mapping}
			for _, ch := range []*model.Channel{&a, &b} {
				require.NoError(t, db.Create(ch).Error)
				require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
			}
			writes := observeNativeFailoverFunding(t, &nativeFailoverFixture{db: db})
			engine := gin.New()
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
				c.Set("id", user.Id)
				user.ToBaseUser().WriteContext(c)
			}, PrepareInfluencerSheet, middleware.Distribute(), PostInfluencerSheet)
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5","group":"gpt-image-2.5-economy","tier":"normal","image_url":"https://8.8.8.8/reference.png","seed":23}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var queued model.Task
			require.NoError(t, db.Where("user_id = ?", user.Id).First(&queued).Error)
			require.NotNil(t, queued.PrivateData.GenerationQueue.Routing)
			assert.Equal(t, model.GenerationRoutingAuto, queued.PrivateData.GenerationQueue.Routing.Mode)
			assert.Equal(t, 2, queued.PrivateData.GenerationQueue.Routing.MaxAttempts)
			assert.Equal(t, []string{influencerReferenceGroup, influencerRoutingBackup}, queued.PrivateData.GenerationQueue.Routing.Groups)
			if first == "legacy group not sent" {
				// Restore a pre-upgrade explicit-group record with its original
				// single-attempt contract despite a compatible live backup.
				queued.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{influencerReferenceGroup}, MaxAttempts: 1}
				require.NoError(t, db.Model(&queued).Update("private_data", queued.PrivateData).Error)
			}
			stored := dispatchGenerationQueueTest(t, user.Id, rec.Body.Bytes())
			require.NoError(t, db.First(&user, user.Id).Error)
			if first == "legacy group not sent" {
				assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
				assert.Zero(t, stored.Quota)
				assert.Equal(t, 100000, user.Quota)
				assert.Equal(t, int32(1), writes.walletDebits.Load())
				assert.Equal(t, int32(1), writes.walletCredits.Load())
				assert.Zero(t, primaryCalls.Load())
				assert.Zero(t, backupCalls.Load(), "legacy reference records must not gain the new Auto fallback")
				return
			}
			assert.Equal(t, 735, stored.Quota)
			assert.Equal(t, 100000-735, user.Quota)
			assert.Equal(t, int32(1), writes.walletDebits.Load())
			assert.Zero(t, writes.walletCredits.Load())
			var count int64
			require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
			assert.EqualValues(t, 1, count)
			if first == "unknown service error" {
				assert.Equal(t, model.TaskStatusDispatching, stored.Status)
				assert.Equal(t, int32(1), primaryCalls.Load())
				assert.Zero(t, backupCalls.Load())
			} else {
				require.NotNil(t, stored.PrivateData.GenerationRouting)
				if first == "not sent" {
					assert.Zero(t, primaryCalls.Load())
					assert.Equal(t, int32(1), backupCalls.Load())
					assert.Equal(t, b.Id, stored.ChannelId)
					require.Len(t, stored.PrivateData.GenerationRouting.Attempts, 2)
					assert.Equal(t, model.GenerationAttemptRejected, stored.PrivateData.GenerationRouting.Attempts[0].Outcome)
				} else {
					assert.Equal(t, int32(1), primaryCalls.Load())
					assert.Zero(t, backupCalls.Load())
					assert.Equal(t, a.Id, stored.ChannelId)
					require.Len(t, stored.PrivateData.GenerationRouting.Attempts, 1)
				}
			}
		})
	}
}

func TestPrepareInfluencerMultiViewRejectsMissingInputsBeforeDistribution(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"invalid JSON", `{broken`, "invalid_request"},
		{"missing model", `{"parent_task_id":"parent","image_url":"https://8.8.8.8/ref.png"}`, "invalid_request"},
		{"missing parent", `{"model":"gpt-image-2.5","image_url":"https://8.8.8.8/ref.png"}`, "invalid_request"},
		{"missing reference", `{"model":"gpt-image-2.5","parent_task_id":"parent"}`, "invalid_request"},
		{"missing group", `{"model":"gpt-image-2.5","parent_task_id":"parent","image_url":"https://8.8.8.8/ref.png"}`, "explicit_group_required"},
		{"conflicting group", `{"model":"gpt-image-2.5@other","group":"gpt-image-2.5-economy","parent_task_id":"parent","image_url":"https://8.8.8.8/ref.png"}`, "group_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			engine := gin.New()
			engine.POST("/pg/influencer/multiview", PrepareInfluencerMultiView, func(c *gin.Context) { reached = true })
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.code)
			assert.False(t, reached)
		})
	}
}
