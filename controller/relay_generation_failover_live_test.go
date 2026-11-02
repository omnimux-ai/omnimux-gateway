package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"
)

// This extra acceptance is opt-in because its loopback proxy may submit one real
// supplier job. It never changes production configuration or uses a real key in Go.
func TestNativeGenerationFailoverRealAlternateOptIn(t *testing.T) {
	if os.Getenv("OMNIMUX_NATIVE_REAL_ACCEPTANCE") != "1" {
		t.Skip("one paid acceptance requires an explicitly authorized local proxy")
	}
	proxyURL := os.Getenv("OMNIMUX_NATIVE_REAL_PROXY_URL")
	parsed, err := url.Parse(proxyURL)
	require.True(t, err == nil && parsed != nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.Port() != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "", "loopback_proxy_required")
	evidence := runNativeAlternateAcceptance(t, proxyURL, 5*time.Minute, 5*time.Second, "default", false, 735)
	encoded, err := common.Marshal(evidence)
	require.NoError(t, err)
	t.Logf("NATIVE_REAL_RESULT %s", encoded)
}

// The same acceptance runs through a fake supplier first: no real credentials,
// external requests or funds. It includes async polling and artifact retrieval.
func TestNativeGenerationFailoverRealHarnessOffline(t *testing.T) {
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 1024, 1024))))
	for _, member := range []struct {
		group string
		quota int
	}{{"default", 735}, {"sub-basic", 662}, {"sub-pro", 588}} {
		var firstBillingSHA any
		for _, primarySucceeds := range []bool{true, false} {
			route := "fallback"
			if primarySucceeds {
				route = "first_success"
			}
			t.Run(member.group+"/"+route, func(t *testing.T) {
				var posts, polls, mediaReads atomic.Int32
				var upstream *httptest.Server
				upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.Method == http.MethodPost && (r.URL.Path == "/v1/images/generations" || r.URL.Path == "/api/image-tasks/generations"):
						posts.Add(1)
						var body map[string]any
						require.NoError(t, common.DecodeJson(r.Body, &body))
						assert.Equal(t, "low", body["quality"])
						assert.Equal(t, "1024x1024", body["size"])
						if primarySucceeds {
							assert.Len(t, body, 4)
							assert.Equal(t, "gpt-image-2.5-async", body["model"])
							assert.Equal(t, "Bearer primary-test-only", r.Header.Get("Authorization"))
						} else {
							assert.Len(t, body, 5)
							assert.Equal(t, "gpt-image-2.5-flare", body["model"])
							assert.Equal(t, float64(1), body["n"])
							assert.Equal(t, "Bearer backup-test-only", r.Header.Get("Authorization"))
						}
						_, _ = io.WriteString(w, `{"id":"offline-native-original","status":"pending","object":"image.generation.task","type":"image"}`)
					case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/offline-native-original":
						polls.Add(1)
						_, _ = fmt.Fprintf(w, `{"id":"offline-native-original","model":"gpt-image-2.5-flare","status":"completed","object":"image.generation.task","type":"image","results":[%q],"result_data":[{"url":%q}]}`, upstream.URL+"/native-media/image", upstream.URL+"/native-media/image")
					case r.Method == http.MethodGet && r.URL.Path == "/api/image-tasks" && r.URL.Query().Get("ids") == "offline-native-original":
						polls.Add(1)
						_, _ = fmt.Fprintf(w, `{"items":[{"id":"offline-native-original","status":"success","data":[{"url":%q}]}],"missing_ids":[]}`, upstream.URL+"/native-media/image")
					case r.Method == http.MethodGet && r.URL.Path == "/native-media/image":
						mediaReads.Add(1)
						assert.Empty(t, r.Header.Get("Authorization"))
						w.Header().Set("Content-Type", "image/png")
						_, _ = w.Write(pixels.Bytes())
					case r.Method == http.MethodGet && r.URL.Path == "/__native-evidence":
						taskSHA := sha256.Sum256([]byte("offline-native-original"))
						_, _ = fmt.Fprintf(w, `{"post_count":%d,"query_count":%d,"media_downloads":%d,"supplier_task_sha256":"%x"}`, posts.Load(), polls.Load(), mediaReads.Load(), taskSHA)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer upstream.Close()
				evidence := runNativeAlternateAcceptance(t, upstream.URL, 5*time.Second, 10*time.Millisecond, member.group, primarySucceeds, member.quota)
				assert.Equal(t, int32(1), posts.Load())
				assert.Equal(t, int32(1), polls.Load())
				if primarySucceeds {
					firstBillingSHA = evidence["billing_snapshot_sha256"]
				} else {
					assert.Equal(t, firstBillingSHA, evidence["billing_snapshot_sha256"])
				}
				encoded, err := common.Marshal(evidence)
				require.NoError(t, err)
				t.Logf("NATIVE_OFFLINE_RESULT %s", encoded)
			})
		}
	}
}

func runNativeAlternateAcceptance(t *testing.T, proxyURL string, pollBudget, pollInterval time.Duration, userGroup string, primarySucceeds bool, wantQuota int) map[string]any {
	t.Helper()
	// Native failure logs can contain a supplier ID or signed URL. Keep them out
	// of the test output; only the explicit whitelist below is emitted.
	oldDebug, oldWriter, oldErrorWriter, oldLog := common.DebugEnabled, gin.DefaultWriter, gin.DefaultErrorWriter, log.Writer()
	common.DebugEnabled = false
	common.LogWriterMu.Lock()
	gin.DefaultWriter, gin.DefaultErrorWriter = io.Discard, io.Discard
	common.LogWriterMu.Unlock()
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
		common.LogWriterMu.Lock()
		gin.DefaultWriter, gin.DefaultErrorWriter = oldWriter, oldErrorWriter
		common.LogWriterMu.Unlock()
		log.SetOutput(oldLog)
	})
	t.Setenv("TEST_TASK_DB_DIALECT", "sqlite")
	oldQueryLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 100
	t.Cleanup(func() { constant.TaskQueryLimit = oldQueryLimit })
	primaryURL := ":bad-url"
	if primarySucceeds {
		primaryURL = proxyURL
	}
	f := newNativeFailoverFixture(t, primaryURL, proxyURL, false)
	f.db.Logger = f.db.Logger.LogMode(gormlogger.Silent)
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.013072)) * image_count`})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":0.112455,"backup":0.112455}`))
	configureNativeFailoverBilling(t, f, userGroup, "wallet")
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"sub-basic":{"default":0.1012095,"backup":0.1012095},"sub-pro":{"default":0.089964,"backup":0.089964}}`))
	writes := observeNativeFailoverFunding(t, f)
	f.backup.ModelMapping = common.GetPointer(`{"gpt-image-2.5":"gpt-image-2.5-flare"}`)
	require.NoError(t, f.db.Model(&f.backup).Update("model_mapping", f.backup.ModelMapping).Error)
	const body = `{"model":"gpt-image-2.5","prompt":"A single red apple on a plain white background.","size":"1024x1024","n":1,"quality":"low"}`
	f.task.PrivateData.GenerationQueue.Body = []byte(body)
	require.NoError(t, f.db.Save(f.task).Error)
	f.c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body)).WithContext(context.Background())
	f.c.Request.Header.Set("Content-Type", "application/json")
	f.c.Set("generation_queue_reserved_task", f.task)
	common.CleanupBodyStorage(f.c)
	f.c.Set(common.KeyRequestBody, []byte(body))

	oldRegistry := pluginruntime.DefaultRegistry
	registry := pluginruntime.NewRegistry()
	source, err := os.ReadFile("../plugins/tasks/sora/plugin.js")
	require.NoError(t, err)
	_, err = registry.RegisterFactory(string(source), pluginruntime.Options{Key: "sora"})
	require.NoError(t, err)
	pluginruntime.DefaultRegistry = registry
	t.Cleanup(func() { pluginruntime.DefaultRegistry = oldRegistry })
	oldFactory := service.GetTaskAdaptorFunc
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
	t.Cleanup(func() { service.GetTaskAdaptorFunc = oldFactory })
	oldStorage := storage.GetStorageDriver()
	storage.SetGlobalDriver(newInfluencerPresetTestDriver())
	t.Cleanup(func() { storage.SetGlobalDriver(oldStorage) })
	allowPrivateTaskMediaTest(t)

	firstNotSent := false
	var attemptQuotes []*generationSubmissionQuote
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
		result, submitErr := relay.RelayTaskSubmit(c, info)
		platform := constant.TaskPlatform("62")
		if result != nil {
			platform = result.Platform
		}
		snapshot, snapshotErr := taskFundingSnapshot(c, info, platform)
		require.NoError(t, snapshotErr)
		attemptQuotes = append(attemptQuotes, &generationSubmissionQuote{Quota: snapshot.Quota, Billing: snapshot.PrivateData.BillingContext})
		if info.ChannelId == f.primary.Id {
			firstNotSent = submitErr != nil && submitErr.NoTaskCreated
		}
		return result, submitErr
	})
	require.True(t, taskErr == nil, "native_submit_failed_before_acceptance")
	require.NotNil(t, outcome)
	wantAttempts := 2
	wantChannel := f.backup.Id
	if primarySucceeds {
		wantAttempts, wantChannel = 1, f.primary.Id
	}
	require.Len(t, attemptQuotes, wantAttempts)
	if !primarySucceeds {
		require.True(t, attemptQuotes[0].Matches(attemptQuotes[1].Quota, attemptQuotes[1].Billing), "cross_protocol_native_quote_changed")
	}
	require.Equal(t, !primarySucceeds, firstNotSent, "primary_pre_send_evidence")
	require.Equal(t, wantChannel, outcome.Task.ChannelId)
	require.Equal(t, wantQuota, outcome.Task.Quota)
	require.False(t, outcome.Task.Status == model.TaskStatusSuccess, "real_async_receipt_must_not_be_faked_as_delivery")
	beforeBilling := outcome.Task.PrivateData.BillingContext
	require.NotNil(t, beforeBilling)
	deadline := time.Now().Add(pollBudget)
	var saved model.Task
	for {
		service.RunTaskPollingOnce(context.Background(), nil)
		require.NoError(t, f.db.First(&saved, f.task.ID).Error)
		if saved.Status == model.TaskStatusSuccess || saved.Status == model.TaskStatusFailure {
			break
		}
		require.True(t, time.Now().Before(deadline), "native_poll_deadline_unknown_no_resubmit")
		time.Sleep(pollInterval)
	}
	require.True(t, saved.Status == model.TaskStatusSuccess, "native_terminal_failure_no_resubmit")
	require.Greater(t, saved.FinishTime, int64(0))
	require.True(t, reflect.DeepEqual(beforeBilling, saved.PrivateData.BillingContext), "native_billing_snapshot_changed")
	require.NotNil(t, saved.PrivateData.GenerationRouting)
	require.Len(t, saved.PrivateData.GenerationRouting.Attempts, wantAttempts)
	if primarySucceeds {
		assert.Equal(t, model.GenerationAttemptAccepted, saved.PrivateData.GenerationRouting.Attempts[0].Outcome)
	} else {
		assert.Equal(t, model.GenerationAttemptRejected, saved.PrivateData.GenerationRouting.Attempts[0].Outcome)
		assert.Equal(t, model.GenerationAttemptAccepted, saved.PrivateData.GenerationRouting.Attempts[1].Outcome)
		assert.NotEqual(t, saved.PrivateData.GenerationRouting.Attempts[0].ChannelID, saved.PrivateData.GenerationRouting.Attempts[1].ChannelID)
	}
	var rows int64
	require.NoError(t, f.db.Model(&model.Task{}).Count(&rows).Error)
	assert.Equal(t, int64(1), rows)
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Equal(t, 100000-wantQuota, account.Quota)
	assert.Equal(t, wantQuota, account.UsedQuota)
	assert.Equal(t, int32(1), writes.walletDebits.Load())
	assert.Zero(t, writes.walletCredits.Load())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.TaskArtifactAccessContextKey, true)
	c.Params = gin.Params{{Key: "key", Value: saved.TaskID}, {Key: "artifact_key", Value: "image"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+saved.TaskID+"/artifacts/image/content", nil)
	TaskArtifactContent(c)
	require.Equal(t, http.StatusOK, recorder.Code, "native_artifact_read_failed")
	decoded, _, err := image.Decode(bytes.NewReader(recorder.Body.Bytes()))
	require.True(t, err == nil, "native_artifact_not_decodable")
	assert.Equal(t, 1024, decoded.Bounds().Dx())
	assert.Equal(t, 1024, decoded.Bounds().Dy())
	imageSHA := sha256.Sum256(recorder.Body.Bytes())
	taskSHA := sha256.Sum256([]byte(saved.GetUpstreamTaskID()))
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get(proxyURL + "/__native-evidence")
	require.True(t, err == nil, "proxy_evidence_unavailable")
	defer response.Body.Close()
	var proxyEvidence struct {
		PostCount      int    `json:"post_count"`
		QueryCount     int    `json:"query_count"`
		MediaDownloads int    `json:"media_downloads"`
		TaskSHA        string `json:"supplier_task_sha256"`
	}
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, common.DecodeJson(response.Body, &proxyEvidence))
	require.Equal(t, 1, proxyEvidence.PostCount)
	require.Greater(t, proxyEvidence.QueryCount, 0)
	require.Equal(t, 1, proxyEvidence.MediaDownloads)
	require.True(t, proxyEvidence.TaskSHA == fmt.Sprintf("%x", taskSHA), "proxy_task_association_changed")
	billingBytes, encodeErr := common.Marshal(saved.PrivateData.BillingContext)
	require.NoError(t, encodeErr)
	billingSHA := sha256.Sum256(billingBytes)
	return map[string]any{"primary_pre_send_rejected": firstNotSent, "attempt_count": wantAttempts, "user_group": userGroup,
		"native_quote_preserved": true, "billing_snapshot_sha256": fmt.Sprintf("%x", billingSHA),
		"supplier_post_count": proxyEvidence.PostCount, "supplier_query_count": proxyEvidence.QueryCount,
		"customer_raw_quota": saved.Quota, "wallet_debits": writes.walletDebits.Load(), "wallet_credits": writes.walletCredits.Load(),
		"task_rows": rows, "task_status": string(saved.Status), "supplier_task_sha256": fmt.Sprintf("%x", taskSHA),
		"image_sha256": fmt.Sprintf("%x", imageSHA), "image_bytes": recorder.Body.Len(), "width": decoded.Bounds().Dx(), "height": decoded.Bounds().Dy()}
}
