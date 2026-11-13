package controller

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// This crosses the real CLI process, UserAuth, scoped credential, durable queue,
// native provider/billing and artifact delivery. All accounts, storage and
// provider requests belong to this fixture; no configured account is loaded.
func TestInfluencerCLIProcessRecoversAcceptedCharacterAndDownloadsNativeArtifact(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the real CLI process acceptance test")
	}
	// UserAccessToken deliberately has a fixed table name, so this complete
	// account/process fixture must never use an environment-selected database.
	// Receipt transaction dialect coverage lives in the dedicated model tests.
	t.Setenv("TEST_TASK_DB_DIALECT", string(common.DatabaseTypeSQLite))
	db, dialect := openInfluencerCLITestDatabase(t, &model.User{}, &model.UserAccessToken{}, &model.AuditLog{}, &model.Option{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.Log{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.InfluencerGenerationRequest{})
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMemory, oldBatch, oldConsume, oldExport, oldRDB := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled, common.RDB
	oldSecret, oldArtifactURL, oldServerURL := common.CryptoSecret, system_setting.TaskPublicAddress, system_setting.ServerAddress
	oldStorage := storage.GetStorageDriver()
	localRedis := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: localRedis.Addr()})
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	// Initialize the native dialect-specific reserved-column handling without
	// opening an environment-selected database. Audit writes intentionally use
	// a literal table name, so logs get a separate isolated in-memory database.
	t.Setenv("LOG_SQL_DSN", "")
	oldMaster := common.IsMasterNode
	common.IsMasterNode = false
	logInitErr := model.InitLogDB()
	common.IsMasterNode = oldMaster
	require.NoError(t, logInitErr)
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlLogs, err := logDB.DB()
	require.NoError(t, err)
	sqlLogs.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlLogs.Close()) })
	require.NoError(t, logDB.AutoMigrate(&model.AuditLog{}, &model.Log{}))
	model.LOG_DB = logDB
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled, common.RDB = true, false, false, true, false, redisClient
	common.CryptoSecret = "influencer-cli-e2e-only-artifact-secret"
	storage.SetGlobalDriver(newInfluencerPresetTestDriver())
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return gopool.WorkerCount() == 0 }, 5*time.Second, time.Millisecond, "native account background work must drain before restoring globals")
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.DataExportEnabled, common.RDB = oldRedis, oldMemory, oldBatch, oldConsume, oldExport, oldRDB
		common.CryptoSecret, system_setting.TaskPublicAddress, system_setting.ServerAddress = oldSecret, oldArtifactURL, oldServerURL
		storage.SetGlobalDriver(oldStorage)
		require.NoError(t, redisClient.Close())
	})
	require.NoError(t, i18n.Init())
	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "true")
	allowPrivateTaskMediaTest(t)
	oldGroups, oldUsable := ratio_setting.GroupRatio2JSONString(), setting.UserUsableGroups2JSONString()
	oldAuto := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAuto))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":1,"gpt-image-2.5-sunburst-cli-e2e":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Original quote","gpt-image-2.5-sunburst-cli-e2e":"Fixture image route"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-cli-e2e"]`))
	withTieredBillingConfig(t, map[string]string{influencerImageModel: "ratio"}, map[string]string{})
	oldModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldModelPrice)) })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2.5-sunburst":0.01}`))

	// A genuine PNG at the reviewed canvas size protects the delivery contract.
	imageBytes := influencerPresetTestPNG(t, 2048, 1152)
	var providerSubmits, providerDownloads atomic.Int32
	var provider *httptest.Server
	provider = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Cookie"))
		switch r.URL.Path {
		case "/v1/images/generations":
			providerSubmits.Add(1)
			var body map[string]any
			assert.NoError(t, common.DecodeJson(r.Body, &body))
			assert.Equal(t, influencerImageModel, body["model"])
			assert.Contains(t, body["prompt"], "fixture character")
			assert.Equal(t, influencerImagePixels, body["size"])
			assert.Equal(t, "high", body["quality"])
			assert.Equal(t, "png", body["output_format"])
			assert.Equal(t, "url", body["response_format"])
			assert.Equal(t, float64(1), body["n"])
			w.Header().Set("Content-Type", "application/json")
			response, marshalErr := common.Marshal(gin.H{"created": common.GetTimestamp(), "data": []gin.H{{"url": provider.URL + "/content/sheet"}}, "private_provider_field": "not-public"})
			require.NoError(t, marshalErr)
			_, _ = w.Write(response)
		case "/content/sheet":
			providerDownloads.Add(1)
			assert.Empty(t, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(imageBytes)))
			_, _ = w.Write(imageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	// The fixture trusts only its local TLS issuer. Real HTTPS media delivery is
	// exercised without changing the application's certificate policy.
	relayClient, mediaClient := service.GetHttpClient(), service.GetSSRFProtectedHTTPClient()
	oldRelayTransport, oldMediaTransport := relayClient.Transport, mediaClient.Transport
	relayClient.Transport, mediaClient.Transport = provider.Client().Transport, provider.Client().Transport
	t.Cleanup(func() {
		relayClient.Transport, mediaClient.Transport = oldRelayTransport, oldMediaTransport
	})
	user := model.User{Username: "cli-e2e-owner", AffCode: "cli-e2e-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AuthVersion: 1, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	credential := model.AccessTokenPrefix + "fixture-cli-creation-permission-only"
	pat := model.UserAccessToken{Name: "fixture creation permission", TokenHash: model.AccessTokenFingerprint(credential), TokenHint: model.AccessTokenHint(credential)}
	require.NoError(t, pat.SetScopes([]string{"influencer_creation:read", "influencer_creation:write"}))
	require.NoError(t, model.CreateUserAccessToken(user.Id, &pat, service.AccessTokenMaxPerUser))
	channel := model.Channel{Name: "local CLI fixture provider", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "fixture-provider-key", Group: "gpt-image-2.5-sunburst-cli-e2e", Models: influencerImageModel, BaseURL: &provider.URL, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: channel.Group, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterInfluencerCLIRoutes(router.Group("/api/influencer/v1", middleware.UserAuth()))
	var contentReads atomic.Int32
	router.GET("/v1/tasks/:key/artifacts/:artifact_key/content", func(c *gin.Context) {
		contentReads.Add(1)
		assert.Empty(t, c.GetHeader("Authorization"), "capability downloads must not carry the account credential")
	}, middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"), TaskArtifactContent)
	var createPOSTs atomic.Int32
	var dropAcceptedReply atomic.Bool
	dropAcceptedReply.Store(true)
	apiServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/influencer/v1/characters" {
			createPOSTs.Add(1)
			if dropAcceptedReply.Swap(false) {
				// Lose only the HTTP response after the real handler commits its task.
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, r)
				if !assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String()) {
					w.WriteHeader(recorder.Code)
					_, _ = w.Write(recorder.Body.Bytes())
					return
				}
				connection, _, hijackErr := w.(http.Hijacker).Hijack()
				if assert.NoError(t, hijackErr) {
					_ = connection.Close()
				}
				return
			}
		}
		router.ServeHTTP(w, r)
	}))
	t.Cleanup(apiServer.Close)
	system_setting.TaskPublicAddress, system_setting.ServerAddress = apiServer.URL, apiServer.URL
	testDir := t.TempDir()
	certificate := filepath.Join(testDir, "fixture-ca.pem")
	require.NoError(t, os.WriteFile(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: apiServer.Certificate().Raw}), 0600))
	cliFile, err := filepath.Abs(filepath.Join("..", "cli", "src", "bin.js"))
	require.NoError(t, err)
	type cliEnvelope struct {
		OK    bool           `json:"ok"`
		Data  map[string]any `json:"data"`
		Error struct {
			Type        string `json:"type"`
			Subtype     string `json:"subtype"`
			OperationID string `json:"operation_id"`
		} `json:"error"`
	}
	invoke := func(configDirectory string, args ...string) (cliEnvelope, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, node, append([]string{cliFile}, args...)...)
		// Deliberately do not inherit environment credentials, config, TLS bypasses
		// or proxy settings. The only trusted CA is this fixture's certificate.
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + testDir, "OMNIMUX_CONFIG_DIR=" + configDirectory, "OMNIMUX_BASE_URL=" + apiServer.URL, "OMNIMUX_USER_ID=" + strconv.Itoa(user.Id), "OMNIMUX_ACCESS_TOKEN=" + credential, "OMNIMUX_AUTO_UPDATE=false", "NODE_EXTRA_CA_CERTS=" + certificate}
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		runErr := cmd.Run()
		require.NoError(t, ctx.Err(), "CLI child exceeded the bounded fixture deadline")
		exitCode := 0
		var envelope cliEnvelope
		if runErr != nil {
			var exitErr *exec.ExitError
			require.ErrorAs(t, runErr, &exitErr, "CLI child failed to start")
			exitCode = exitErr.ExitCode()
			require.NoError(t, common.Unmarshal(stderr.Bytes(), &envelope), "CLI error must retain the structured contract")
			assert.Empty(t, stdout.String())
		} else {
			require.NoError(t, common.Unmarshal(stdout.Bytes(), &envelope))
			assert.Empty(t, stderr.String())
		}
		assert.NotContains(t, stdout.String()+stderr.String(), credential)
		assert.NotContains(t, stdout.String()+stderr.String(), "fixture-provider-key")
		assert.NotContains(t, stdout.String()+stderr.String(), "private_provider_field")
		return envelope, exitCode
	}
	const requestID = "cli-e2e-character-accepted"
	createArgs := []string{"influencer", "character", "create", "--request-id", requestID, "--brief", "fixture character", "--yes"}
	configDir := filepath.Join(testDir, "first-account")
	lostReply, exitCode := invoke(configDir, createArgs...)
	require.Equal(t, 4, exitCode)
	assert.False(t, lostReply.OK)
	assert.Equal(t, "network", lostReply.Error.Type)
	assert.Equal(t, requestID, lostReply.Error.OperationID)
	var tasks []model.Task
	require.NoError(t, db.Find(&tasks).Error)
	require.Len(t, tasks, 1, "the accepted task survives loss of the HTTP response")
	assert.Equal(t, model.TaskStatusWaiting, tasks[0].Status)
	assert.Zero(t, tasks[0].Quota)
	assert.Zero(t, providerSubmits.Load())

	restored, exitCode := invoke(configDir, createArgs...)
	require.Zero(t, exitCode)
	require.True(t, restored.OK)
	taskID, ok := restored.Data["task_id"].(string)
	require.True(t, ok)
	assert.Equal(t, tasks[0].TaskID, taskID)
	assert.Equal(t, requestID, restored.Data["request_id"])
	assert.Equal(t, int32(1), createPOSTs.Load(), "recovery queries before attempting another creation POST")
	// A new local config lacks any receipt, proving server-side replay as well.
	replayed, exitCode := invoke(filepath.Join(testDir, "fresh-local-receipt"), createArgs...)
	require.Zero(t, exitCode)
	assert.Equal(t, taskID, replayed.Data["task_id"])
	assert.Equal(t, int32(2), createPOSTs.Load())

	// Revoking the submitting credential removes reads immediately without
	// deleting or cancelling an already accepted native queue task.
	require.NoError(t, db.Delete(&pat).Error)
	revoked, exitCode := invoke(configDir, "influencer", "task", "get", taskID)
	require.Equal(t, 2, exitCode)
	assert.Equal(t, "auth", revoked.Error.Type)
	credential = model.AccessTokenPrefix + "fixture-cli-renewed-creation-permission"
	pat = model.UserAccessToken{Name: "renewed fixture permission", TokenHash: model.AccessTokenFingerprint(credential), TokenHint: model.AccessTokenHint(credential)}
	require.NoError(t, pat.SetScopes([]string{"influencer_creation:read", "influencer_creation:write"}))
	require.NoError(t, model.CreateUserAccessToken(user.Id, &pat, service.AccessTokenMaxPerUser))

	scanner := &model.SystemTask{TaskID: "cli_e2e_scanner"}
	require.NoError(t, db.Create(&model.SystemTaskLock{Type: model.SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "cli-e2e-runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "cli-e2e-runner")
	var completed model.Task
	require.NoError(t, db.Where("task_id = ?", taskID).First(&completed).Error)
	require.EqualValues(t, model.TaskStatusSuccess, completed.Status, completed.FailReason)
	assert.Equal(t, ActionInfluencerSheet, completed.Action)
	assert.False(t, completed.PrivateData.ResultDiscarded)
	assert.Equal(t, int32(1), providerSubmits.Load())
	assert.Equal(t, 5000, completed.Quota)
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, 95000, after.Quota)
	var logs []model.Log
	require.NoError(t, logDB.Where("user_id = ? AND type = ?", user.Id, model.LogTypeConsume).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, completed.Quota, logs[0].Quota)

	finished, exitCode := invoke(configDir, "influencer", "task", "wait", taskID, "--timeout", "2")
	require.Zero(t, exitCode)
	assert.Equal(t, "succeeded", finished.Data["status"])
	assert.Equal(t, "ready", finished.Data["artifact_status"])
	assert.Equal(t, float64(0.1), finished.Data["credits"])
	history, exitCode := invoke(configDir, "influencer", "character", "list")
	require.Zero(t, exitCode)
	items, ok := history.Data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, taskID, items[0].(map[string]any)["task_id"])
	lookup, exitCode := invoke(configDir, "influencer", "request", "get", requestID)
	require.Zero(t, exitCode)
	assert.Equal(t, taskID, lookup.Data["task_id"])

	output := filepath.Join(testDir, "character.png")
	downloaded, exitCode := invoke(configDir, "influencer", "task", "download", taskID, "--output", output)
	require.Zero(t, exitCode)
	assert.Equal(t, float64(len(imageBytes)), downloaded.Data["bytes"])
	written, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, imageBytes, written)
	assert.Equal(t, int32(1), contentReads.Load())
	assert.Equal(t, int32(1), providerDownloads.Load())
	refusedOverwrite, exitCode := invoke(configDir, "influencer", "task", "download", taskID, "--output", output)
	assert.Equal(t, 3, exitCode)
	assert.Equal(t, "output_exists", refusedOverwrite.Error.Subtype)
	assert.Equal(t, int32(1), contentReads.Load())
	// The stored artifact remains downloadable when its provider disappears;
	// an explicit overwrite still completes atomically through the real CLI.
	provider.Close()
	cached, exitCode := invoke(configDir, "influencer", "task", "download", taskID, "--output", output, "--overwrite")
	require.Zero(t, exitCode)
	assert.Equal(t, float64(len(imageBytes)), cached.Data["bytes"])
	written, err = os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, imageBytes, written)
	assert.Equal(t, int32(2), contentReads.Load())
	assert.Equal(t, int32(1), providerDownloads.Load())
	// Disabling new CLI creation must not make a previously accepted identifier
	// start a second task or lose its recovery path.
	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "false")
	disabledReplay, exitCode := invoke(filepath.Join(testDir, "disabled-replay"), createArgs...)
	require.Zero(t, exitCode)
	assert.Equal(t, taskID, disabledReplay.Data["task_id"])

	var taskCount, receiptCount int64
	require.NoError(t, db.Model(&model.Task{}).Count(&taskCount).Error)
	require.NoError(t, db.Model(&model.InfluencerGenerationRequest{}).Count(&receiptCount).Error)
	assert.Equal(t, int64(1), taskCount)
	assert.Equal(t, int64(1), receiptCount)
	assert.Equal(t, int32(1), providerSubmits.Load())
	t.Logf("actual CLI process -> scoped UserAuth -> durable request/task -> native provider -> one wallet consumption -> signed artifact -> disk: task_rows=%d receipts=%d provider_submits=%d consume_logs=%d quota=%d bytes=%d", taskCount, receiptCount, providerSubmits.Load(), len(logs), completed.Quota, len(written))
}
