package controller

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type standardImageReferenceTransport struct {
	data  []byte
	calls *atomic.Int32
}

// Rebuild the original P7 bytes from the original preparation algorithm. The
// fixed digest prevents future prompt-template changes from silently replacing
// the historical failing request with a different overlong string.
func historicalStandardImageP7Prompt(t *testing.T) string {
	t.Helper()
	prompt := influencer.BuildPrompt("normal", influencer.Selection{
		"aesthetic": {"casual"}, "age": {"adult"}, "gender": {"male"}, "hair": {"hair_short"},
	}, "")
	require.Equal(t, 3155, len(prompt))
	chinese := []rune(strings.Repeat("角色身着深蓝棉质外套浅灰长裤简洁运动鞋保留成年面部自然神态头发整洁衣料纹理真实配饰克制", 400))[:14000]
	english := strings.Repeat(" Keep the adult character calm and natural. Use a navy cotton jacket, light grey trousers and simple shoes. Preserve readable fabric texture, restrained accessories, balanced skin tones and consistent clothing details. ", 100)
	prompt += string(chinese) + english[:32000-3155-14000]
	require.Equal(t, 32000, utf8.RuneCountInString(prompt))
	require.Equal(t, 60000, len(prompt))
	require.Equal(t, "d96dc05e5d1f11f1f61ab118d74dabd99244e4aa37a5ad7de0df6f12eef7f802", fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))))
	return prompt
}

// This record contains fixture observations, not a manually declared pass. A
// collector must also require the enclosing test's PASS event and bind the
// complete go-test JSON log before using it as package-audit evidence.
func logStandardImagePromptRejection(t *testing.T, body []byte, prompt string, status int, errorCode string, posts, supplierGets, fundingUpdates, tokenUpdates int32, writes *nativeFundingWrites, user model.User, token model.Token, tasks, logs int64) {
	t.Helper()
	original, err := common.Marshal(map[string]any{
		"model": "gpt-image-2.5-flare", "prompt": prompt, "size": "1920x1088", "n": 1, "quality": "medium", "output_format": "png",
	})
	require.NoError(t, err)
	originalSHA := fmt.Sprintf("%x", sha256.Sum256(original))
	require.Equal(t, "2c35225c4dc40dc26d4acaf32732ab3c9a7c538da64ae666ca00a6af639bf3e7", originalSHA)
	sources := make([]map[string]string, 0, 7)
	sourceState := "committed_exact_sources"
	var testSHA string
	for _, file := range []string{"relay/helper/standard_image_request.go", "relay/helper/standard_image_request_test.go", "relay/relay_task.go", "controller/standard_image_contract_test.go", "controller/relay_generation_failover_billing_test.go", "modelcontracts/contracts.go", "docs/model-governance/specs/gpt-image-2.5.json"} {
		raw, err := os.ReadFile("../" + file)
		require.NoError(t, err)
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		sources = append(sources, map[string]string{"file": file, "sha256": digest})
		committed, err := exec.Command("git", "show", "HEAD:"+file).Output()
		if err != nil || !bytes.Equal(raw, committed) {
			sourceState = "uncommitted_exact_sources"
		}
		if file == "controller/standard_image_contract_test.go" {
			testSHA = digest
		}
	}
	revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	encoded, err := common.Marshal(map[string]any{
		"schema_version": 1, "kind": "standard-image-local-http-rejection", "case_id": "P7", "profile_id": "influencer-flare-public-standard-v1",
		"source_revision": strings.TrimSpace(string(revision)), "source_state": sourceState, "source_files": sources,
		"test":                    map[string]any{"file": "controller/standard_image_contract_test.go", "sha256": testSHA, "name": t.Name()},
		"original_request_sha256": originalSHA, "original_model": "gpt-image-2.5-flare", "original_group": nil,
		"prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))), "prompt_chars": utf8.RuneCountInString(prompt), "prompt_utf8_bytes": len(prompt),
		"public_model": "gpt-image-2.5", "public_group": influencerReferenceGroup, "local_request_sha256": fmt.Sprintf("%x", sha256.Sum256(body)),
		"contract_prompt_max_chars": 8192, "contract_prompt_max_utf8_bytes": 16384,
		"http_status": status, "error_code": errorCode, "supplier_posts": posts, "material_requests": supplierGets,
		"material_observation":      "pure-text input has no reference fields; local supplier fixture received zero GET requests",
		"precharge_funding_updates": fundingUpdates, "wallet_charge_calls": writes.walletDebits.Load(), "token_funding_updates": tokenUpdates,
		"refund_calls":              writes.walletCredits.Load() + writes.subscriptionReleases.Load(),
		"subscription_reservations": writes.subscriptionReservations.Load(), "funding_row_updates": fundingUpdates,
		"task_count": tasks, "consume_log_count": logs,
		"user_quota_before": 100000, "user_quota_after": user.Quota, "user_used_quota_after": user.UsedQuota,
		"token_quota_before": 100000, "token_quota_after": token.RemainQuota, "token_used_quota_after": token.UsedQuota,
	})
	require.NoError(t, err)
	fmt.Printf("STANDARD_IMAGE_PROMPT_REJECTION %s\n", encoded)
}

func (tr standardImageReferenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(tr.data)), Request: req}, nil
}

func TestStandardImagePublicEntryRejectsBeforeSupplierAndNativeFunding(t *testing.T) {
	require.NoError(t, i18n.Init())
	for _, tc := range []struct {
		name, extra, contentType string
		accepted                 bool
		path                     string
	}{
		{"reviewed standard", `,"n":1,"quality":"medium"`, "application/json", true, "/v1/images/generations"},
		{"native JSON group", `,"n":1,"quality":"medium","group":"gpt-image-2.5-economy"`, "application/json", true, "/v1/images/generations"},
		{"reviewed character boundary", `,"n":1,"quality":"medium"`, "application/json", true, "/v1/images/generations"},
		{"reviewed token boundary", `,"n":1,"quality":"medium"`, "application/json", true, "/v1/images/generations"},
		{"one token over boundary", `,"n":1,"quality":"medium"`, "application/json", false, "/v1/images/generations"},
		{"high token count within length limits", `,"n":1,"quality":"medium"`, "application/json", false, "/v1/images/generations"},
		{"one character over boundary", `,"n":1,"quality":"medium"`, "application/json", false, "/v1/images/generations"},
		{"one UTF8 byte over boundary", `,"n":1,"quality":"medium"`, "application/json", false, "/v1/images/generations"},
		{"historical P7 full prompt", `,"n":1,"quality":"medium","output_format":"png","group":"gpt-image-2.5-economy"`, "application/json", false, "/v1/images/generations"},
		{"multiple outputs", `,"n":2`, "application/json", false, "/v1/images/generations"},
		{"high quality", `,"quality":"high"`, "application/json", false, "/v1/images/generations"},
		{"old incompatible pixels", `,"size":"1024x1365"`, "application/json", false, "/v1/images/generations"},
		{"metadata override", `,"metadata":{"n":2}`, "application/json", false, "/v1/images/generations"},
		{"raw parameters", `,"parameters":{"quality":"high"}`, "application/json", false, "/v1/images/generations"},
		{"multiple reference fields", `,"image":"https://8.8.8.8/a.png","image_urls":["https://8.8.8.8/b.png"]`, "application/json", false, "/v1/images/generations"},
		{"mask", `,"mask_url":"https://8.8.8.8/mask.png"`, "application/json", false, "/v1/images/generations"},
		{"multipart URL", "url", "multipart/form-data", false, "/v1/images/edits"},
		{"multipart upload", "file", "multipart/form-data", false, "/v1/images/edits"},
		{"multipart mixed references", "both", "multipart/form-data", false, "/v1/images/edits"},
		{"generation multipart URL", "url", "multipart/form-data", false, "/v1/images/generations"},
		{"generation multipart upload", "file", "multipart/form-data", false, "/v1/images/generations"},
		{"generation multipart mixed references", "both", "multipart/form-data", false, "/v1/images/generations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path
			prompt, size := "a person", "16:9"
			switch tc.name {
			case "reviewed character boundary":
				prompt = strings.Repeat("字", 4096) + strings.Repeat("a", 4096)
			case "reviewed token boundary":
				prompt = strings.Repeat("字", 5000)
				require.Equal(t, 5000, tokenkit.Count("gpt-image-2.5", prompt))
			case "one token over boundary":
				prompt = strings.Repeat("字", 5001)
				require.Equal(t, 5001, tokenkit.Count("gpt-image-2.5", prompt))
			case "high token count within length limits":
				prompt = "Create one male character on a white background. Ignore the following decorative marks: " + strings.Repeat("\u07ff", 8100)
				require.Equal(t, 8188, utf8.RuneCountInString(prompt))
				require.Equal(t, 16288, len(prompt))
				require.Equal(t, 16216, tokenkit.Count("gpt-image-2.5", prompt))
				require.Equal(t, "05100c073888edb82e3ec869514d2087c3918498ff28f37c6aac094d335d6674", fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))))
			case "one character over boundary":
				prompt = strings.Repeat("a", 8193)
			case "one UTF8 byte over boundary":
				prompt = strings.Repeat("字", 5461) + "aa"
			case "historical P7 full prompt":
				prompt, size = historicalStandardImageP7Prompt(t), "1920x1088"
			}
			db := setupInfluencerRoutingTest(t)
			oldLogDB := model.LOG_DB
			oldRedis, oldBatch, oldLog, oldMaster := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode
			fetch := system_setting.GetFetchSetting()
			oldFetch := *fetch
			t.Cleanup(func() {
				model.LOG_DB = oldLogDB
				common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = oldRedis, oldBatch, oldLog, oldMaster
				*fetch = oldFetch
				service.InitHttpClient()
			})
			common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = false, false, false, false, false
			t.Setenv("LOG_SQL_DSN", "")
			require.NoError(t, model.InitLogDB())
			require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.Token{}))
			fetch.EnableSSRFProtection = false // The supplier is this local HTTP fixture.
			service.InitHttpClient()
			withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.013072)) * image_count`})
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-economy":1}`))
			user := model.User{Username: "standard-public", AffCode: "standard-public", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
			require.NoError(t, db.Create(&user).Error)
			token := model.Token{UserId: user.Id, Key: "standard-local-token", Name: "local", RemainQuota: 100000, Group: influencerReferenceGroup}
			require.NoError(t, db.Create(&token).Error)
			var posts, supplierGets, fundingUpdates, tokenUpdates atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					supplierGets.Add(1)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				posts.Add(1)
				assert.Equal(t, "/v1/images/generations", r.URL.Path)
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, "gpt-image-2.5-flare", body["model"])
				assert.Equal(t, "1920x1088", body["size"])
				assert.Equal(t, "medium", body["quality"])
				assert.Equal(t, prompt, body["prompt"], "accepted input must be sent in full without truncation")
				assert.NotContains(t, body, "group", "the existing native routing field is not a supplier option")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"local-standard-accepted","status":"pending"}`))
			}))
			defer upstream.Close()
			mapping := `{"gpt-image-2.5":"gpt-image-2.5-flare"}`
			ch := model.Channel{Name: "standard local supplier", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-test-only", Models: "gpt-image-2.5", Group: influencerReferenceGroup, BaseURL: &upstream.URL, ModelMapping: &mapping}
			require.NoError(t, db.Create(&ch).Error)
			require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
			writes := observeNativeFailoverFunding(t, &nativeFailoverFixture{db: db})
			require.NoError(t, db.Callback().Update().After("gorm:update").Register("observe_standard_prompt_funding", func(tx *gorm.DB) {
				if tx.Error == nil && tx.RowsAffected > 0 && (tx.Statement.Table == "users" || tx.Statement.Table == "tokens" || tx.Statement.Table == "user_subscriptions") {
					fundingUpdates.Add(1)
					if tx.Statement.Table == "tokens" {
						tokenUpdates.Add(1)
					}
				}
			}))
			engine := gin.New()
			engine.POST(path, func(c *gin.Context) {
				c.Set("id", user.Id)
				user.ToBaseUser().WriteContext(c)
				common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
				common.SetContextKey(c, constant.ContextKeyTokenKey, token.Key)
				common.SetContextKey(c, constant.ContextKeyTokenGroup, influencerReferenceGroup)
				common.SetContextKey(c, constant.ContextKeyUsingGroup, influencerReferenceGroup)
			}, middleware.Distribute(), RelayImage)
			encodedPrompt, err := common.Marshal(prompt)
			require.NoError(t, err)
			var requestBody map[string]any
			var body []byte
			if tc.contentType == "application/json" {
				require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-image-2.5","prompt":`+string(encodedPrompt)+`,"size":"`+size+`"`+tc.extra+`}`), &requestBody))
				body, err = common.Marshal(requestBody)
				require.NoError(t, err)
			}
			contentType := tc.contentType
			if strings.HasPrefix(tc.contentType, "multipart/") {
				var form bytes.Buffer
				writer := multipart.NewWriter(&form)
				require.NoError(t, writer.WriteField("model", "gpt-image-2.5"))
				require.NoError(t, writer.WriteField("prompt", "a person"))
				require.NoError(t, writer.WriteField("size", "16:9"))
				if tc.extra == "url" || tc.extra == "both" {
					require.NoError(t, writer.WriteField("input_reference", "https://8.8.8.8/a.png"))
				}
				if tc.extra == "file" || tc.extra == "both" {
					file, err := writer.CreateFormFile("image", "image.png")
					require.NoError(t, err)
					_, err = io.WriteString(file, "local unreviewed upload")
					require.NoError(t, err)
				}
				require.NoError(t, writer.Close())
				body, contentType = form.Bytes(), writer.FormDataContentType()
			}
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", contentType)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.NoError(t, db.First(&user, user.Id).Error)
			require.NoError(t, db.First(&token, token.Id).Error)
			var tasks int64
			require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
			if tc.accepted {
				assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				assert.Equal(t, int32(1), posts.Load())
				assert.EqualValues(t, 1, tasks)
				assert.Equal(t, int32(1), writes.walletDebits.Load())
				assert.Zero(t, writes.walletCredits.Load())
				assert.Less(t, user.Quota, 100000)
				assert.Equal(t, user.Quota, token.RemainQuota)
			} else {
				assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), "invalid_request")
				assert.Zero(t, posts.Load())
				assert.Zero(t, tasks)
				assert.Zero(t, writes.walletDebits.Load())
				assert.Zero(t, writes.walletCredits.Load())
				assert.Equal(t, 100000, user.Quota)
				assert.Zero(t, user.UsedQuota)
				assert.Equal(t, 100000, token.RemainQuota)
				assert.Zero(t, token.UsedQuota)
				assert.Zero(t, fundingUpdates.Load(), "rejected input cannot update user, token or subscription funds")
				assert.Zero(t, tokenUpdates.Load())
				if tc.name == "historical P7 full prompt" {
					var logs int64
					require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
					assert.Zero(t, logs)
					assert.Zero(t, supplierGets.Load())
					assert.Zero(t, writes.subscriptionReservations.Load())
					assert.Zero(t, writes.subscriptionReleases.Load())
					var response struct {
						Code string `json:"code"`
					}
					require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
					assert.Equal(t, "invalid_request", response.Code)
					if !t.Failed() {
						logStandardImagePromptRejection(t, body, prompt, rec.Code, response.Code, posts.Load(), supplierGets.Load(), fundingUpdates.Load(), tokenUpdates.Load(), writes, user, token, tasks, logs)
					}
				}
			}
			if tc.name == "reviewed token boundary" || tc.name == "one token over boundary" || tc.name == "high token count within length limits" {
				observation, err := common.Marshal(map[string]any{
					"case": tc.name, "prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))),
					"prompt_tokens": tokenkit.Count("gpt-image-2.5", prompt), "prompt_chars": utf8.RuneCountInString(prompt), "prompt_utf8_bytes": len(prompt),
					"http_status": rec.Code, "supplier_posts": posts.Load(), "supplier_gets": supplierGets.Load(), "tasks": tasks,
					"wallet_debits": writes.walletDebits.Load(), "wallet_refunds": writes.walletCredits.Load(),
					"funding_updates": fundingUpdates.Load(), "token_updates": tokenUpdates.Load(),
					"user_quota_before": 100000, "user_quota_after": user.Quota, "token_quota_before": 100000, "token_quota_after": token.RemainQuota,
				})
				require.NoError(t, err)
				fmt.Printf("STANDARD_IMAGE_TOKEN_BOUNDARY %s\n", observation)
			}
		})
	}
}

func TestStandardImageWorkbenchReferenceSurvivesQueueAndNativeGate(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	oldClient, oldLogDB := influencerReferenceHTTPClient, model.LOG_DB
	oldRedis, oldBatch, oldLog, oldMaster := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode
	fetch := system_setting.GetFetchSetting()
	oldFetch := *fetch
	t.Cleanup(func() {
		influencerReferenceHTTPClient, model.LOG_DB = oldClient, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = oldRedis, oldBatch, oldLog, oldMaster
		*fetch = oldFetch
		service.InitHttpClient()
	})
	common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = false, false, false, false, false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}))
	fetch.EnableSSRFProtection = false // The supplier is this local HTTP fixture.
	service.InitHttpClient()
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.013072)) * image_count`})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-economy":1}`))
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	var referenceFetches, posts atomic.Int32
	influencerReferenceHTTPClient = &http.Client{Transport: standardImageReferenceTransport{imageBytes.Bytes(), &referenceFetches}}
	user := model.User{Username: "standard-reference", AffCode: "standard-reference", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		assert.Equal(t, "gpt-image-2.5-flare", body["model"])
		assert.Equal(t, "1920x1088", body["size"])
		assert.Equal(t, []any{"https://8.8.8.8/reference.png"}, body["image_urls"])
		assert.Equal(t, map[string]any{"seed": float64(23)}, body["metadata"])
		assert.Contains(t, body["prompt"], "a different person")
		assert.NotContains(t, body, "quality", "the original omitted quality must retain the supplier medium default")
		assert.NotContains(t, body, "image")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"local-standard-reference","status":"pending"}`)
	}))
	defer upstream.Close()
	mapping := `{"gpt-image-2.5":"gpt-image-2.5-flare"}`
	ch := model.Channel{Name: "standard reference supplier", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-test-only", Models: "gpt-image-2.5", Group: influencerReferenceGroup, BaseURL: &upstream.URL, ModelMapping: &mapping}
	require.NoError(t, db.Create(&ch).Error)
	require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true}).Error)
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
	assert.Equal(t, int32(1), referenceFetches.Load(), "acceptance performs the existing protected material preflight")
	assert.Zero(t, posts.Load(), "queued acceptance cannot create a supplier job")
	var queued model.Task
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&queued).Error)
	require.NotNil(t, queued.PrivateData.GenerationQueue)
	require.NotNil(t, queued.PrivateData.GenerationQueue.Routing)
	assert.Equal(t, []string{influencerReferenceGroup}, queued.PrivateData.GenerationQueue.Routing.Groups)
	stored := dispatchGenerationQueueTest(t, user.Id, rec.Body.Bytes())
	assert.Equal(t, int32(2), referenceFetches.Load(), "dispatch revalidates material once; the new standard gate must not download it a third time")
	assert.Equal(t, int32(1), posts.Load())
	assert.Equal(t, ch.Id, stored.ChannelId)
	assert.Equal(t, "local-standard-reference", stored.GetUpstreamTaskID())
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100000-stored.Quota, user.Quota)
	assert.Positive(t, stored.Quota)
	assert.Equal(t, int32(1), writes.walletDebits.Load())
	assert.Zero(t, writes.walletCredits.Load())
	var tasks int64
	require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
	assert.EqualValues(t, 1, tasks)
}

func TestStandardImageRestoredWorkbenchQueueRejectsHighTokenPromptBeforeFundingAndSend(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	oldLogDB := model.LOG_DB
	oldRedis, oldBatch, oldLog, oldMaster := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode
	fetch := system_setting.GetFetchSetting()
	oldFetch := *fetch
	t.Cleanup(func() {
		model.LOG_DB = oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = oldRedis, oldBatch, oldLog, oldMaster
		*fetch = oldFetch
		service.InitHttpClient()
	})
	common.MemoryCacheEnabled, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.IsMasterNode = false, false, false, false, false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.Token{}))
	fetch.EnableSSRFProtection = false // Both suppliers are local HTTP fixtures.
	service.InitHttpClient()
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.013072)) * image_count`})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-economy":1}`))
	user := model.User{Username: "standard-restored", AffCode: "standard-restored", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "standard-restored-test-only", Name: "untouched token", RemainQuota: 100000, Group: influencerReferenceGroup}
	require.NoError(t, db.Create(&token).Error)
	var primaryPosts, backupPosts, fundingUpdates, tokenUpdates atomic.Int32
	for index, counter := range []*atomic.Int32{&primaryPosts, &backupPosts} {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"must-not-create-restored-image","status":"pending"}`)
		}))
		t.Cleanup(upstream.Close)
		mapping, priority := `{"gpt-image-2.5":"gpt-image-2.5-flare"}`, int64(20-index*10)
		ch := model.Channel{Name: fmt.Sprintf("restored local supplier %d", index), Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-test-only", Models: "gpt-image-2.5", Group: influencerReferenceGroup, BaseURL: &upstream.URL, ModelMapping: &mapping, Priority: &priority}
		require.NoError(t, db.Create(&ch).Error)
		require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: "gpt-image-2.5", ChannelId: ch.Id, Enabled: true, Priority: &priority}).Error)
	}
	prompt := "Create one male character on a white background. Ignore the following decorative marks: " + strings.Repeat("\u07ff", 8100)
	require.Equal(t, 8188, utf8.RuneCountInString(prompt))
	require.Equal(t, 16288, len(prompt))
	require.Equal(t, 16216, tokenkit.Count("gpt-image-2.5", prompt))
	require.Equal(t, "05100c073888edb82e3ec869514d2087c3918498ff28f37c6aac094d335d6674", fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))))
	body, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "prompt": prompt, "size": "1920x1088", "n": 1, "quality": "medium"})
	require.NoError(t, err)
	// Restore a previously accepted, unsent workbench row. Its public record
	// already exists; rejection must retain that row rather than claim none did.
	queued := model.Task{TaskID: "task_standard_restored_high_token", UserId: user.Id, Group: influencerReferenceGroup, Action: ActionInfluencerSheet, Status: model.TaskStatusWaiting, Progress: "0%",
		Properties:  model.Properties{OriginModelName: "gpt-image-2.5", Input: `{"kind":"sheet","tier":"normal"}`},
		PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: body, Routing: &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingAuto, Groups: []string{influencerReferenceGroup}, MaxAttempts: 2}}}}
	require.NoError(t, queued.Insert())
	writes := observeNativeFailoverFunding(t, &nativeFailoverFixture{db: db})
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("observe_restored_standard_funding", func(tx *gorm.DB) {
		if tx.Error != nil || tx.RowsAffected == 0 {
			return
		}
		values, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		for _, key := range []string{"quota", "used_quota", "remain_quota", "amount_used"} {
			if _, exists := values[key]; exists && (tx.Statement.Table == "users" || tx.Statement.Table == "tokens" || tx.Statement.Table == "user_subscriptions") {
				fundingUpdates.Add(1)
				if tx.Statement.Table == "tokens" {
					tokenUpdates.Add(1)
				}
				return
			}
		}
	}))
	receipt, err := common.Marshal(map[string]any{"task_id": queued.TaskID})
	require.NoError(t, err)
	stored := dispatchGenerationQueueTest(t, user.Id, receipt)
	assert.Equal(t, queued.ID, stored.ID)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	assert.Empty(t, stored.PrivateData.UpstreamTaskID)
	require.NotNil(t, stored.PrivateData.GenerationQueue)
	require.NotNil(t, stored.PrivateData.GenerationQueue.Routing)
	assert.Equal(t, body, []byte(stored.PrivateData.GenerationQueue.Body), "restore and rejection cannot shorten the frozen request")
	assert.Zero(t, stored.PrivateData.GenerationQueue.SendStartedAt)
	assert.Empty(t, stored.PrivateData.GenerationQueue.Routing.Attempts)
	assert.Zero(t, primaryPosts.Load())
	assert.Zero(t, backupPosts.Load())
	assert.Zero(t, writes.walletDebits.Load())
	assert.Zero(t, writes.walletCredits.Load())
	assert.Zero(t, writes.subscriptionReservations.Load())
	assert.Zero(t, writes.subscriptionReleases.Load())
	assert.Zero(t, fundingUpdates.Load())
	assert.Zero(t, tokenUpdates.Load())
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 100000, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Equal(t, 100000, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)
	var tasks, logs int64
	require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
	require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
	assert.EqualValues(t, 1, tasks, "only the restored public queue row remains")
	assert.Zero(t, logs)
	if !t.Failed() {
		observation, err := common.Marshal(map[string]any{"case": "restored workbench high token prompt", "scope": "existing_unsent_queue_row", "prompt_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))), "prompt_tokens": tokenkit.Count("gpt-image-2.5", prompt), "status": stored.Status, "tasks_before": 1, "tasks_after": tasks, "request_body_unchanged": bytes.Equal(body, stored.PrivateData.GenerationQueue.Body), "primary_posts": primaryPosts.Load(), "backup_posts": backupPosts.Load(), "wallet_debits": writes.walletDebits.Load(), "wallet_refunds": writes.walletCredits.Load(), "funding_updates": fundingUpdates.Load(), "token_updates": tokenUpdates.Load()})
		require.NoError(t, err)
		fmt.Printf("STANDARD_IMAGE_RESTORED_QUEUE_TOKEN_BOUNDARY %s\n", observation)
	}
}
