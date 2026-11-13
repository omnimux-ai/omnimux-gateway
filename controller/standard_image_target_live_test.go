package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

const standardTargetCampaign = "influencer-standard-target-preactivation-v1"
const standardTargetProxyURL = "http://127.0.0.1:18491"
const standardTargetGroup = "gpt-image-2.5-economy"
const standardTargetRequestSHA256 = "00444412350ab06fb136a995930db5a45cd4f5cd772c16b92f35d6253b74f11a"
const standardTargetManifestSHA256 = "89034b5584c676ce75e82de455018f8d7ff8958811a5b69722a20c187bae807b"
const standardTargetPrompt = "Create one photorealistic male character reference sheet on a pure white background. Show the same adult man in exactly two views: a chest-up portrait on the left and a complete head-to-toe standing view on the right. He is a 30-year-old man with short dark hair, brown eyes, an oval face, and a calm neutral expression. He wears a deep teal long-sleeved button-up shirt tucked into beige straight-leg trousers, a dark brown belt, and clean dark brown leather shoes. Keep his face, hairstyle, body proportions, clothing colors, and clothing details identical between both views. In the standing view, show his entire head and both shoes with white space around him, arms relaxed and both hands visible. Use realistic human anatomy, consistent soft studio lighting, sharp natural detail, and a clean two-view layout. No text, labels, logos, watermark, accessories, extra person, or extra panels. This is a new standard target channel acceptance image with the teal shirt and beige trousers."

type standardTargetBinding struct {
	Campaign                 string `json:"campaign"`
	TargetChannelID          int    `json:"target_channel_id"`
	SourceChannelID          int    `json:"source_channel_id"`
	Type                     int    `json:"type"`
	Status                   int    `json:"status"`
	ModelID                  string `json:"model_id"`
	Group                    string `json:"group"`
	UpstreamModel            string `json:"upstream_model"`
	BaseURL                  string `json:"base_url"`
	SameKeyVerified          bool   `json:"same_key_verified"`
	NoActiveAbilities        bool   `json:"no_active_abilities"`
	NoOverrides              bool   `json:"no_overrides"`
	BindingSHA256            string `json:"binding_sha256"`
	ManifestSHA256           string `json:"manifest_sha256"`
	RequestSHA256            string `json:"request_sha256"`
	SourceCommit             string `json:"source_commit"`
	AssociationReceiptSHA256 string `json:"association_receipt_sha256"`
	TransportSourceSHA256    string `json:"transport_source_sha256"`
}

type standardTargetTransportEvidence struct {
	Bind            standardTargetBinding `json:"bind"`
	Campaign        string                `json:"campaign"`
	TargetChannelID int                   `json:"target_channel_id"`
	PostCount       int                   `json:"post_count"`
	QueryCount      int                   `json:"query_count"`
	MediaDownloads  int                   `json:"media_downloads"`
	TaskSHA256      string                `json:"supplier_task_sha256"`
	RequestSHA256   string                `json:"request_sha256"`
	Image           struct {
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
		MIME   string `json:"mime"`
	} `json:"image"`
}

func (bind standardTargetBinding) validate(sourceCommit string) error {
	if bind.Campaign != standardTargetCampaign || bind.TargetChannelID <= 0 || bind.TargetChannelID >= 2147483646 || bind.TargetChannelID == 37 || bind.SourceChannelID != 37 || bind.Type != constant.ChannelTypeOpenAI || bind.Status != common.ChannelStatusManuallyDisabled || bind.ModelID != "gpt-image-2.5" || bind.Group != standardTargetGroup || bind.UpstreamModel != "gpt-image-2.5-flare" || bind.BaseURL != "https://api.evolink.ai" || !bind.SameKeyVerified || !bind.NoActiveAbilities || !bind.NoOverrides || bind.ManifestSHA256 != standardTargetManifestSHA256 || bind.RequestSHA256 != standardTargetRequestSHA256 {
		return fmt.Errorf("standard_target_binding_invalid")
	}
	metadataSHA, err := bind.metadataSHA256()
	if err != nil || metadataSHA != bind.BindingSHA256 {
		return fmt.Errorf("standard_target_metadata_digest_mismatch")
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(sourceCommit) || bind.SourceCommit != sourceCommit {
		return fmt.Errorf("standard_target_source_commit_mismatch")
	}
	for _, digest := range []string{bind.BindingSHA256, bind.AssociationReceiptSHA256, bind.TransportSourceSHA256} {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(digest) {
			return fmt.Errorf("standard_target_binding_digest_invalid")
		}
	}
	return nil
}

func (bind standardTargetBinding) metadataSHA256() (string, error) {
	metadata, err := common.Marshal(map[string]any{"campaign": bind.Campaign, "source_channel_id": bind.SourceChannelID, "target_channel_id": bind.TargetChannelID, "type": bind.Type, "status": bind.Status, "model_id": bind.ModelID, "group": bind.Group, "upstream_model": bind.UpstreamModel, "base_url": bind.BaseURL, "same_key_verified": bind.SameKeyVerified, "no_active_abilities": bind.NoActiveAbilities, "no_overrides": bind.NoOverrides})
	if err != nil {
		return "", err
	}
	return standardTargetHash(metadata), nil
}

// Default builds and test runs cannot reach the paid branch. The remote
// transport owns the durable, once-only supplier intent; Go never has its key.
func TestStandardImageTargetNativeOptIn(t *testing.T) {
	if os.Getenv("OMNIMUX_STANDARD_TARGET_REAL_ACCEPTANCE") != "1" {
		t.Skip("explicit authorization and the prepared disabled target are required")
	}
	require.Equal(t, "^TestStandardImageTargetNativeOptIn$", flag.Lookup("test.run").Value.String(), "only_the_exact_paid_test_may_be_selected")
	require.Equal(t, "1", flag.Lookup("test.count").Value.String(), "paid_test_must_not_repeat")
	require.Equal(t, standardTargetProxyURL, os.Getenv("OMNIMUX_STANDARD_TARGET_PROXY_URL"), "fixed_loopback_transport_required")
	require.Equal(t, standardTargetCampaign, os.Getenv("OMNIMUX_STANDARD_TARGET_CAMPAIGN"), "fixed_campaign_required")
	commitBytes, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	commit := strings.TrimSpace(string(commitBytes))
	dirty, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	require.NoError(t, err)
	require.Empty(t, dirty, "committed_clean_source_required")
	committedHarness, err := exec.Command("git", "show", "HEAD:controller/standard_image_target_live_test.go").Output()
	require.NoError(t, err, "harness_must_be_present_in_source_commit")
	currentHarness, err := os.ReadFile("standard_image_target_live_test.go")
	require.NoError(t, err)
	require.Equal(t, standardTargetHash(committedHarness), standardTargetHash(currentHarness), "harness_must_match_source_commit")
	output := os.Getenv("OMNIMUX_STANDARD_TARGET_EVIDENCE_DIR")
	require.True(t, filepath.IsAbs(output), "absolute_evidence_directory_required")
	require.NoError(t, os.Mkdir(output, 0700), "fresh_evidence_directory_required_no_resume_or_resubmit")
	bind := readStandardTargetJSON[standardTargetBinding](t, standardTargetProxyURL+"/__standard-target-bind")
	require.NoError(t, bind.validate(commit))
	require.Equal(t, bind.TransportSourceSHA256, os.Getenv("OMNIMUX_STANDARD_TARGET_TRANSPORT_SHA256"), "reviewed_transport_source_required")
	require.Equal(t, bind.AssociationReceiptSHA256, os.Getenv("OMNIMUX_STANDARD_TARGET_ASSOCIATION_RECEIPT_SHA256"), "verified_association_receipt_required")
	intent, err := common.Marshal(map[string]any{"campaign": standardTargetCampaign, "target_channel_id": bind.TargetChannelID, "source_commit": commit, "state": "started_no_automatic_resubmit"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(output, "execution.intent.json"), intent, 0600))
	result := runStandardTargetNative(t, standardTargetProxyURL, bind, "default", 37500, false, true, output, 5*time.Minute, 5*time.Second)
	encoded, err := common.Marshal(result)
	require.NoError(t, err)
	t.Logf("STANDARD_TARGET_NATIVE_RESULT %s", encoded)
}

func TestStandardImageTargetBindingFailsClosed(t *testing.T) {
	bind := offlineStandardTargetBinding()
	require.NoError(t, bind.validate(bind.SourceCommit))
	for _, mutate := range []func(*standardTargetBinding){
		func(b *standardTargetBinding) { b.TargetChannelID = 37 },
		func(b *standardTargetBinding) { b.TargetChannelID = 0 },
		func(b *standardTargetBinding) { b.Status = common.ChannelStatusEnabled },
		func(b *standardTargetBinding) { b.UpstreamModel = "gpt-image-2.5-beta" },
		func(b *standardTargetBinding) { b.SameKeyVerified = false },
		func(b *standardTargetBinding) { b.NoActiveAbilities = false },
		func(b *standardTargetBinding) { b.NoOverrides = false },
		func(b *standardTargetBinding) { b.ManifestSHA256 = strings.Repeat("e", 64) },
		func(b *standardTargetBinding) { b.BindingSHA256 = strings.Repeat("f", 64) },
		func(b *standardTargetBinding) { b.SourceCommit = strings.Repeat("b", 40) },
		func(b *standardTargetBinding) { b.AssociationReceiptSHA256 = "" },
	} {
		invalid := bind
		mutate(&invalid)
		require.Error(t, invalid.validate(bind.SourceCommit))
	}
}

func offlineStandardTargetBinding() standardTargetBinding {
	bind := standardTargetBinding{Campaign: standardTargetCampaign, TargetChannelID: 987, SourceChannelID: 37, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusManuallyDisabled, ModelID: "gpt-image-2.5", Group: standardTargetGroup, UpstreamModel: "gpt-image-2.5-flare", BaseURL: "https://api.evolink.ai", SameKeyVerified: true, NoActiveAbilities: true, NoOverrides: true, ManifestSHA256: standardTargetManifestSHA256, RequestSHA256: standardTargetRequestSHA256, SourceCommit: strings.Repeat("a", 40), AssociationReceiptSHA256: strings.Repeat("b", 64), TransportSourceSHA256: strings.Repeat("c", 64)}
	bind.BindingSHA256, _ = bind.metadataSHA256()
	return bind
}

func TestStandardImageTargetNativeOffline(t *testing.T) {
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 1920, 1088))))
	for _, tier := range []struct {
		group string
		quota int
	}{{"default", 37500}, {"sub-basic", 33750}, {"sub-pro", 30000}} {
		var firstBillingSHA any
		for _, firstSucceeds := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/first_success_%t", tier.group, firstSucceeds), func(t *testing.T) {
				bind := offlineStandardTargetBinding()
				var posts, polls, media atomic.Int32
				var upstream *httptest.Server
				upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
						posts.Add(1)
						assert.Equal(t, "Bearer backup-test-only", r.Header.Get("Authorization"))
						var body map[string]any
						require.NoError(t, common.DecodeJson(r.Body, &body))
						assert.Equal(t, standardTargetRequest(t, true), body)
						_, _ = io.WriteString(w, `{"id":"offline-standard-target-original","status":"pending","object":"image.generation.task","type":"image"}`)
					case r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/offline-standard-target-original":
						polls.Add(1)
						_, _ = fmt.Fprintf(w, `{"id":"offline-standard-target-original","model":"gpt-image-2.5-flare","status":"completed","type":"image","results":[%q]}`, upstream.URL+"/native-media/image")
					case r.Method == http.MethodGet && r.URL.Path == "/native-media/image":
						media.Add(1)
						assert.Empty(t, r.Header.Get("Authorization"))
						w.Header().Set("Content-Type", "image/png")
						_, _ = w.Write(pixels.Bytes())
					case r.Method == http.MethodGet && r.URL.Path == "/__native-evidence":
						assert.Equal(t, "Bearer backup-test-only", r.Header.Get("Authorization"))
						evidence := standardTargetTransportEvidence{Bind: bind, Campaign: standardTargetCampaign, TargetChannelID: bind.TargetChannelID, PostCount: int(posts.Load()), QueryCount: int(polls.Load()), MediaDownloads: int(media.Load()), TaskSHA256: standardTargetHash([]byte("offline-standard-target-original")), RequestSHA256: standardTargetJSONHash(t, standardTargetRequest(t, true))}
						evidence.Image.SHA256, evidence.Image.Bytes, evidence.Image.MIME = standardTargetHash(pixels.Bytes()), pixels.Len(), "image/png"
						encoded, err := common.Marshal(evidence)
						require.NoError(t, err)
						_, _ = w.Write(encoded)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer upstream.Close()
				output := filepath.Join(t.TempDir(), "evidence")
				require.NoError(t, os.Mkdir(output, 0700))
				result := runStandardTargetNative(t, upstream.URL, bind, tier.group, tier.quota, firstSucceeds, false, output, 5*time.Second, time.Millisecond)
				assert.False(t, result["paid_supplier"].(bool))
				assert.Equal(t, int32(1), posts.Load())
				assert.Equal(t, int32(1), polls.Load())
				assert.Equal(t, int32(1), media.Load())
				if firstSucceeds {
					firstBillingSHA = result["billing_snapshot_sha256"]
				} else {
					assert.Equal(t, firstBillingSHA, result["billing_snapshot_sha256"])
				}
				for _, file := range []string{"native-task.json", "native-funding-events.json", "server-bind.json", "supplier-transport.json", "request.public.json", "native-acceptance.json", "output.png"} {
					info, err := os.Stat(filepath.Join(output, file))
					require.NoError(t, err)
					assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
				}
				encoded, err := common.Marshal(result)
				require.NoError(t, err)
				t.Logf("STANDARD_TARGET_OFFLINE_RESULT %s", encoded)
			})
		}
	}
}

func TestStandardImageTargetAcceptedOrUnknownNeverSendsBackup(t *testing.T) {
	for _, scenario := range []string{"accepted", "missing_receipt", "disconnected"} {
		t.Run(scenario, func(t *testing.T) {
			var first, backup atomic.Int32
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				if scenario == "disconnected" {
					conn, _, err := w.(http.Hijacker).Hijack()
					require.NoError(t, err)
					_ = conn.Close()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if scenario == "accepted" {
					_, _ = io.WriteString(w, `{"id":"offline-target-accepted","status":"pending","type":"image"}`)
				} else {
					_, _ = io.WriteString(w, `{"status":"pending","type":"image"}`)
				}
			}))
			defer primary.Close()
			alternate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { backup.Add(1) }))
			defer alternate.Close()
			f := standardTargetFixture(t, primary.URL, offlineStandardTargetBinding(), "default", true)
			f.backup.BaseURL = &alternate.URL
			require.NoError(t, f.db.Model(&f.backup).Update("base_url", alternate.URL).Error)
			writes := observeNativeFailoverFunding(t, f)
			outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
			var saved model.Task
			require.NoError(t, f.db.First(&saved, f.task.ID).Error)
			if scenario == "accepted" {
				require.Nil(t, taskErr)
				require.NotNil(t, outcome)
				require.Equal(t, "offline-target-accepted", saved.GetUpstreamTaskID())
				require.NotNil(t, saved.PrivateData.GenerationRouting)
				require.Len(t, saved.PrivateData.GenerationRouting.Attempts, 1)
			} else {
				require.NotNil(t, taskErr)
				require.False(t, taskErr.NoTaskCreated)
				require.NotNil(t, saved.PrivateData.GenerationQueue)
				require.Len(t, saved.PrivateData.GenerationQueue.Routing.Attempts, 1)
				require.Equal(t, model.GenerationAttemptUnknown, saved.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
			}
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, backup.Load())
			assert.Equal(t, int32(1), writes.walletDebits.Load())
			assert.Zero(t, writes.walletCredits.Load(), "unknown or accepted reservation stays protected")
		})
	}
}

func TestStandardImageTargetBudgetStopsAtTwoDistinctAttempts(t *testing.T) {
	var thirdCalls atomic.Int32
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { thirdCalls.Add(1) }))
	defer third.Close()
	bind := offlineStandardTargetBinding()
	f := standardTargetFixture(t, ":invalid-backup-before-transport", bind, "default", false)
	last := model.Channel{Id: bind.TargetChannelID + 2, Name: "unattempted third", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "backup-test-only", Group: standardTargetGroup, Models: "gpt-image-2.5", BaseURL: &third.URL, Priority: common.GetPointer(int64(0)), ModelMapping: common.GetPointer(`{"gpt-image-2.5":"gpt-image-2.5-flare"}`)}
	require.NoError(t, f.db.Create(&last).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: standardTargetGroup, Model: "gpt-image-2.5", ChannelId: last.Id, Enabled: true, Priority: last.Priority}).Error)
	writes := observeNativeFailoverFunding(t, f)
	var attempted []int
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
		result, err := relay.RelayTaskSubmit(c, info)
		attempted = append(attempted, info.ChannelId)
		require.NotNil(t, err)
		require.True(t, err.NoTaskCreated)
		return result, err
	})
	require.NotNil(t, taskErr)
	require.True(t, taskErr.NoTaskCreated)
	var saved model.Task
	require.NoError(t, f.db.First(&saved, f.task.ID).Error)
	require.Len(t, attempted, 2)
	assert.Equal(t, []int{f.primary.Id, f.backup.Id}, attempted)
	assert.NotEqual(t, attempted[0], attempted[1])
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), saved.Status)
	assert.Zero(t, thirdCalls.Load())
	assert.Equal(t, int32(1), writes.walletDebits.Load())
	assert.Equal(t, int32(1), writes.walletCredits.Load())
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
}

func standardTargetRequest(t *testing.T, upstream bool) map[string]any {
	t.Helper()
	modelID := "gpt-image-2.5"
	if upstream {
		modelID = "gpt-image-2.5-flare"
	}
	return map[string]any{"model": modelID, "prompt": standardTargetPrompt, "size": "1920x1088", "n": float64(1), "quality": "medium", "output_format": "png"}
}

func standardTargetFixture(t *testing.T, proxyURL string, bind standardTargetBinding, userGroup string, firstSucceeds bool) *nativeFailoverFixture {
	t.Helper()
	t.Setenv("TEST_TASK_DB_DIALECT", "sqlite")
	f := newNativeFailoverFixture(t, ":invalid-before-transport", proxyURL, false)
	require.Equal(t, "sqlite", f.db.Dialector.Name())
	require.Equal(t, common.DatabaseTypeSQLite, common.MainDatabaseType())
	f.db.Logger = f.db.Logger.LogMode(gormlogger.Silent)
	require.NoError(t, f.db.Where("1 = 1").Delete(&model.Ability{}).Error)
	require.NoError(t, f.db.Where("1 = 1").Delete(&model.Channel{}).Error)
	f.primary.Id, f.backup.Id = bind.TargetChannelID+1, bind.TargetChannelID
	f.primary.Group, f.backup.Group = standardTargetGroup, standardTargetGroup
	f.primary.Type = constant.ChannelTypeOpenAI
	f.primary.ModelMapping = common.GetPointer(`{"gpt-image-2.5":"gpt-image-2.5-flare"}`)
	f.backup.ModelMapping = common.GetPointer(`{"gpt-image-2.5":"gpt-image-2.5-flare"}`)
	f.primary.Key, f.backup.Key = "backup-test-only", "backup-test-only"
	if firstSucceeds {
		f.primary.Id, f.backup.Id = bind.TargetChannelID, bind.TargetChannelID+1
		f.primary.BaseURL = &proxyURL
	}
	for _, channel := range []*model.Channel{&f.primary, &f.backup} {
		require.NoError(t, f.db.Create(channel).Error)
		require.NoError(t, f.db.Create(&model.Ability{Group: standardTargetGroup, Model: "gpt-image-2.5", ChannelId: channel.Id, Enabled: true, Priority: channel.Priority}).Error)
	}
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.044314)) * image_count`})
	ratio := 0.075 / 0.044314
	groupJSON, err := common.Marshal(map[string]float64{standardTargetGroup: ratio})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(groupJSON)))
	configureNativeFailoverBilling(t, f, userGroup, "wallet")
	memberJSON, err := common.Marshal(map[string]map[string]float64{"sub-basic": {standardTargetGroup: 0.9 * ratio}, "sub-pro": {standardTargetGroup: 0.8 * ratio}})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(string(memberJSON)))
	f.task.Group = standardTargetGroup
	f.task.TaskID = fmt.Sprintf("task_standard_target_pre_activation_v1_%d", bind.TargetChannelID)
	f.task.PrivateData.GenerationQueue.Path = "/v1/images/generations"
	f.task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{standardTargetGroup}, MaxAttempts: 2}
	body, err := common.Marshal(standardTargetRequest(t, false))
	require.NoError(t, err)
	f.task.PrivateData.GenerationQueue.Body = body
	require.NoError(t, f.db.Save(f.task).Error)
	f.c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body)).WithContext(context.Background())
	f.c.Request.Header.Set("Content-Type", "application/json")
	common.CleanupBodyStorage(f.c)
	f.c.Set(common.KeyRequestBody, body)
	f.info.TaskRelayInfo.PublicTaskID = f.task.TaskID
	common.SetContextKey(f.c, constant.ContextKeyTokenAutoGroups, []string{standardTargetGroup})
	common.SetContextKey(f.c, constant.ContextKeyAutoGroup, standardTargetGroup)
	f.info.TokenGroup, f.info.UsingGroup = standardTargetGroup, standardTargetGroup
	require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, "gpt-image-2.5"))
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
	return f
}

type standardTargetFundingEvent struct {
	Sequence     int    `json:"sequence"`
	Field        string `json:"field"`
	Operation    string `json:"operation"`
	Amount       int    `json:"amount"`
	Unit         string `json:"unit"`
	RowsAffected int64  `json:"rows_affected"`
}

// This observes successful native database updates and then reconciles them to
// actual account and Task rows. It does not invent a ledger or transaction ID.
func observeStandardTargetFunding(t *testing.T, f *nativeFailoverFixture) (*[]standardTargetFundingEvent, *sync.Mutex) {
	t.Helper()
	events, mu := &[]standardTargetFundingEvent{}, &sync.Mutex{}
	require.NoError(t, f.db.Callback().Update().After("gorm:update").Register("observe_standard_target_funding", func(tx *gorm.DB) {
		if tx.Error != nil || tx.RowsAffected == 0 || !strings.HasSuffix(tx.Statement.Table, "users") {
			return
		}
		values, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		for _, field := range []string{"quota", "used_quota", "request_count"} {
			expr, ok := values[field].(clause.Expr)
			if !ok || len(expr.Vars) != 1 {
				continue
			}
			amount, ok := expr.Vars[0].(int)
			if !ok {
				continue
			}
			operation := "increase"
			if strings.Contains(expr.SQL, " -") {
				operation = "decrease"
			}
			unit := "raw_quota"
			if field == "request_count" {
				unit = "request_count"
			}
			mu.Lock()
			*events = append(*events, standardTargetFundingEvent{Sequence: len(*events) + 1, Field: field, Operation: operation, Amount: amount, Unit: unit, RowsAffected: tx.RowsAffected})
			mu.Unlock()
		}
	}))
	return events, mu
}

func runStandardTargetNative(t *testing.T, proxyURL string, bind standardTargetBinding, userGroup string, wantQuota int, firstSucceeds, paidSupplier bool, output string, pollBudget, pollInterval time.Duration) map[string]any {
	t.Helper()
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
	oldLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 100
	t.Cleanup(func() { constant.TaskQueryLimit = oldLimit })
	f := standardTargetFixture(t, proxyURL, bind, userGroup, firstSucceeds)
	events, mu := observeStandardTargetFunding(t, f)
	writes := observeNativeFailoverFunding(t, f)
	completed := false
	t.Cleanup(func() {
		if completed {
			return
		}
		var saved model.Task
		var account model.User
		if f.db.First(&saved, f.task.ID).Error != nil || f.db.First(&account, f.user.Id).Error != nil {
			return
		}
		mu.Lock()
		observed := append([]standardTargetFundingEvent(nil), (*events)...)
		mu.Unlock()
		// Preserve a safe checkpoint on failure/timeout. Its presence never
		// authorizes another send, and it is not a successful acceptance.
		supplierSHA := ""
		if saved.PrivateData.UpstreamTaskID != "" {
			supplierSHA = standardTargetHash([]byte(saved.PrivateData.UpstreamTaskID))
		}
		checkpoint := map[string]any{"acceptance_state": "incomplete_do_not_resubmit", "execution_scope": "isolated_native_pre_activation", "native_db": "isolated_sqlite", "funding": "virtual_user_wallet", "paid_supplier": paidSupplier, "campaign": standardTargetCampaign, "target_channel_id": bind.TargetChannelID, "source_commit": bind.SourceCommit, "native_task_sha256": standardTargetHash([]byte(saved.TaskID)), "supplier_task_sha256": supplierSHA, "task_status": string(saved.Status), "raw_quota": saved.Quota, "routing": saved.PrivateData.GenerationRouting, "wallet_after": account.Quota, "used_quota_after": account.UsedQuota, "events": observed, "wallet_debits": writes.walletDebits.Load(), "wallet_credits": writes.walletCredits.Load()}
		encoded, err := common.Marshal(checkpoint)
		if err != nil || os.WriteFile(filepath.Join(output, "native-incomplete.json"), encoded, 0600) != nil {
			t.Error("native_incomplete_checkpoint_write_failed")
		}
	})
	firstNotSent := false
	var quotes []*generationSubmissionQuote
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
		result, submitErr := relay.RelayTaskSubmit(c, info)
		snapshot, snapshotErr := taskFundingSnapshot(c, info, constant.TaskPlatform("1"))
		require.NoError(t, snapshotErr)
		quotes = append(quotes, &generationSubmissionQuote{Quota: snapshot.Quota, Billing: snapshot.PrivateData.BillingContext})
		if info.ChannelId == f.primary.Id {
			firstNotSent = submitErr != nil && submitErr.NoTaskCreated
		}
		return result, submitErr
	})
	require.True(t, taskErr == nil, "native_submit_failed_no_automatic_resubmit")
	require.NotNil(t, outcome)
	wantAttempts := 2
	if firstSucceeds {
		wantAttempts = 1
	}
	require.Len(t, quotes, wantAttempts)
	if !firstSucceeds {
		require.True(t, quotes[0].Matches(quotes[1].Quota, quotes[1].Billing), "native_frozen_quote_changed")
	}
	require.Equal(t, !firstSucceeds, firstNotSent)
	require.Equal(t, bind.TargetChannelID, outcome.Task.ChannelId)
	require.Equal(t, wantQuota, outcome.Task.Quota)
	require.NotEqual(t, model.TaskStatusSuccess, outcome.Task.Status, "receipt_is_not_delivery")
	billingBefore := outcome.Task.PrivateData.BillingContext
	deadline := time.Now().Add(pollBudget)
	var saved model.Task
	for {
		service.RunTaskPollingOnce(context.Background(), nil)
		require.NoError(t, f.db.First(&saved, f.task.ID).Error)
		if saved.Status == model.TaskStatusSuccess || saved.Status == model.TaskStatusFailure {
			break
		}
		require.True(t, time.Now().Before(deadline), "native_poll_deadline_unknown_do_not_resubmit")
		time.Sleep(pollInterval)
	}
	require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), saved.Status, "native_terminal_failure_do_not_resubmit")
	require.Equal(t, bind.TargetChannelID, saved.ChannelId)
	require.Equal(t, standardTargetGroup, saved.Group)
	require.Equal(t, "gpt-image-2.5", saved.Properties.OriginModelName)
	require.Equal(t, "gpt-image-2.5-flare", saved.Properties.UpstreamModelName)
	require.Equal(t, "wallet", saved.PrivateData.BillingSource)
	require.True(t, reflect.DeepEqual(billingBefore, saved.PrivateData.BillingContext))
	require.NotNil(t, saved.PrivateData.GenerationRouting)
	require.Len(t, saved.PrivateData.GenerationRouting.Attempts, wantAttempts)
	require.Equal(t, 2, saved.PrivateData.GenerationRouting.MaxAttempts)
	require.Equal(t, []string{standardTargetGroup}, saved.PrivateData.GenerationRouting.Groups)
	require.Equal(t, bind.TargetChannelID, saved.PrivateData.GenerationRouting.Attempts[wantAttempts-1].ChannelID)
	if !firstSucceeds {
		require.Equal(t, model.GenerationAttemptRejected, saved.PrivateData.GenerationRouting.Attempts[0].Outcome)
		require.NotEqual(t, saved.PrivateData.GenerationRouting.Attempts[0].ChannelID, saved.PrivateData.GenerationRouting.Attempts[1].ChannelID)
	}
	require.Equal(t, model.GenerationAttemptAccepted, saved.PrivateData.GenerationRouting.Attempts[wantAttempts-1].Outcome)
	var rows int64
	require.NoError(t, f.db.Model(&model.Task{}).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	require.Equal(t, 100000-wantQuota, account.Quota)
	require.Equal(t, wantQuota, account.UsedQuota)
	require.Equal(t, 1, account.RequestCount)
	require.Equal(t, int32(1), writes.walletDebits.Load())
	require.Zero(t, writes.walletCredits.Load())
	mu.Lock()
	fundingEvents := append([]standardTargetFundingEvent(nil), (*events)...)
	mu.Unlock()
	require.Contains(t, fundingEvents, standardTargetFundingEvent{Sequence: 1, Field: "quota", Operation: "decrease", Amount: wantQuota, Unit: "raw_quota", RowsAffected: 1})
	require.Len(t, fundingEvents, 3, "native_reservation_and_used_quota_request_count_only")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.TaskArtifactAccessContextKey, true)
	c.Params = gin.Params{{Key: "key", Value: saved.TaskID}, {Key: "artifact_key", Value: "image"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+saved.TaskID+"/artifacts/image/content", nil)
	TaskArtifactContent(c)
	require.Equal(t, http.StatusOK, recorder.Code, "native_artifact_unavailable")
	decoded, err := png.Decode(bytes.NewReader(recorder.Body.Bytes()))
	require.NoError(t, err, "native_artifact_invalid_png")
	require.Equal(t, 1920, decoded.Bounds().Dx())
	require.Equal(t, 1088, decoded.Bounds().Dy())
	transport := readStandardTargetJSON[standardTargetTransportEvidence](t, proxyURL+"/__native-evidence")
	require.Equal(t, bind, transport.Bind)
	require.Equal(t, standardTargetCampaign, transport.Campaign)
	require.Equal(t, bind.TargetChannelID, transport.TargetChannelID)
	require.Equal(t, 1, transport.PostCount)
	require.Greater(t, transport.QueryCount, 0)
	require.Equal(t, 1, transport.MediaDownloads)
	require.NotEmpty(t, saved.PrivateData.UpstreamTaskID)
	require.NotEqual(t, saved.TaskID, saved.PrivateData.UpstreamTaskID)
	supplierTaskSHA := standardTargetHash([]byte(saved.PrivateData.UpstreamTaskID))
	require.Equal(t, supplierTaskSHA, transport.TaskSHA256)
	require.Equal(t, standardTargetJSONHash(t, standardTargetRequest(t, true)), transport.RequestSHA256)
	require.Equal(t, standardTargetHash(recorder.Body.Bytes()), transport.Image.SHA256)
	require.Equal(t, recorder.Body.Len(), transport.Image.Bytes)
	require.Equal(t, "image/png", transport.Image.MIME)
	billingSHA := standardTargetJSONHash(t, saved.PrivateData.BillingContext)
	taskProjection := map[string]any{"execution_scope": "isolated_native_pre_activation", "native_db": "isolated_sqlite", "source_commit": bind.SourceCommit, "row_id": saved.ID, "native_task_sha256": standardTargetHash([]byte(saved.TaskID)), "supplier_task_sha256": supplierTaskSHA, "target_channel_id": saved.ChannelId, "model_id": saved.Properties.OriginModelName, "upstream_model": saved.Properties.UpstreamModelName, "group": saved.Group, "status": string(saved.Status), "action": saved.Action, "raw_quota": saved.Quota, "submit_time": saved.SubmitTime, "finish_time": saved.FinishTime, "billing_source": saved.PrivateData.BillingSource, "billing_snapshot_sha256": billingSHA, "routing": saved.PrivateData.GenerationRouting}
	fundingProjection := map[string]any{"execution_scope": "isolated_native_pre_activation", "native_db": "isolated_sqlite", "funding": "virtual_user_wallet", "observation": "successful_native_database_updates_reconciled_to_account_and_task_rows_no_external_transaction_id", "user_group": userGroup, "wallet_before": 100000, "wallet_after": account.Quota, "used_quota_after": account.UsedQuota, "request_count_after": account.RequestCount, "events": fundingEvents, "wallet_debits": writes.walletDebits.Load(), "wallet_refunds": writes.walletCredits.Load(), "native_task_sha256": standardTargetHash([]byte(saved.TaskID)), "target_channel_id": saved.ChannelId}
	taskRowSHA := standardTargetWriteJSON(t, output, "native-task.json", taskProjection)
	fundingSHA := standardTargetWriteJSON(t, output, "native-funding-events.json", fundingProjection)
	bindSHA := standardTargetWriteJSON(t, output, "server-bind.json", bind)
	standardTargetWriteJSON(t, output, "supplier-transport.json", transport)
	standardTargetWriteJSON(t, output, "request.public.json", standardTargetRequest(t, false))
	require.NoError(t, os.WriteFile(filepath.Join(output, "output.png"), recorder.Body.Bytes(), 0600))
	source, err := os.ReadFile("standard_image_target_live_test.go")
	require.NoError(t, err)
	result := map[string]any{"schema_version": 1, "campaign": standardTargetCampaign, "execution_scope": "isolated_native_pre_activation", "native_db": "isolated_sqlite", "funding": "virtual_user_wallet", "paid_supplier": paidSupplier, "source_commit": bind.SourceCommit, "harness_source_sha256": standardTargetHash(source), "server_bind_sha256": bindSHA, "target_channel_id": bind.TargetChannelID, "production_target_status": bind.Status, "request_sha256": transport.RequestSHA256, "gateway_request_sha256": standardTargetJSONHash(t, standardTargetRequest(t, false)), "supplier_task_sha256": supplierTaskSHA, "native_task_sha256": standardTargetHash([]byte(saved.TaskID)), "native_task_row_sha256": taskRowSHA, "funding_events_sha256": fundingSHA, "billing_snapshot_sha256": billingSHA, "customer_raw_quota": saved.Quota, "user_group": userGroup, "refund_raw_quota": 0, "wallet_debits": writes.walletDebits.Load(), "wallet_credits": writes.walletCredits.Load(), "supplier_post_count": transport.PostCount, "supplier_query_count": transport.QueryCount, "primary_pre_send_rejected": firstNotSent, "attempt_count": wantAttempts, "task_status": string(saved.Status), "settlement_status": "terminal_settled", "task_rows": rows, "image_sha256": standardTargetHash(recorder.Body.Bytes()), "image_bytes": recorder.Body.Len(), "width": decoded.Bounds().Dx(), "height": decoded.Bounds().Dy(), "image_format": "png", "native_quote_preserved": true}
	standardTargetWriteJSON(t, output, "native-acceptance.json", result)
	completed = true
	return result
}

func readStandardTargetJSON[T any](t *testing.T, url string) T {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer backup-test-only")
	response, err := client.Do(request)
	require.True(t, err == nil, "local_transport_evidence_unavailable")
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode, "local_transport_evidence_unavailable")
	var value T
	require.NoError(t, common.DecodeJson(io.LimitReader(response.Body, 65536), &value))
	return value
}

func standardTargetHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func standardTargetJSONHash(t *testing.T, value any) string {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return standardTargetHash(data)
}

func standardTargetWriteJSON(t *testing.T, output, name string, value any) string {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(output, name), data, 0600))
	return standardTargetHash(data)
}
