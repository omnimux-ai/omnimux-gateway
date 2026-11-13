package controller

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInfluencerSunburstCanonicalModesPreserveEditingIntent(t *testing.T) {
	for _, mode := range []string{"text", "reference", "edit", "masked_edit"} {
		t.Run(mode, func(t *testing.T) {
			req := influencerSheetRequest{Model: influencerImageModel, Tier: "normal", Brief: "a person wearing a blue coat", ImageMode: mode}
			if mode != "text" {
				req.ImageURL = "https://8.8.8.8/source.png"
			}
			if mode == "masked_edit" {
				req.MaskURL = "https://8.8.8.8/mask.png"
			}
			if mode == "edit" || mode == "masked_edit" {
				req.EditInstruction = "Change only the cup to green."
			}
			body, meta, code, err := buildInfluencerSheet(req)
			require.NoError(t, err, code)
			assert.Equal(t, influencerImageModel, body["model"])
			assert.Equal(t, mode, body["image_mode"])
			assert.Equal(t, "high", body["quality"])
			assert.Equal(t, influencerImagePixels, body["size"])
			assert.Equal(t, 1, body["n"])
			assert.Equal(t, "png", body["output_format"])
			assert.Equal(t, "url", body["response_format"])
			assert.Equal(t, mode, meta["image_mode"])
			if mode == "edit" || mode == "masked_edit" {
				assert.Equal(t, req.EditInstruction, body["prompt"])
				assert.Equal(t, req.EditInstruction, meta["edit_instruction"])
			} else {
				assert.NotEqual(t, req.Brief, body["prompt"], "character modes retain the established character composition")
			}
		})
	}
	for _, mutate := range []func(*influencerSheetRequest){
		func(r *influencerSheetRequest) { r.ImageMode = "unexpected" },
		func(r *influencerSheetRequest) { r.ImageMode = "text"; r.ImageURL = "https://8.8.8.8/source.png" },
		func(r *influencerSheetRequest) { r.ImageMode = "reference" },
		func(r *influencerSheetRequest) { r.ImageMode = "edit"; r.ImageURL = "https://8.8.8.8/source.png" },
		func(r *influencerSheetRequest) {
			r.ImageMode = "masked_edit"
			r.ImageURL = "https://8.8.8.8/source.png"
			r.EditInstruction = "edit"
		},
		func(r *influencerSheetRequest) { r.MaskURL = "https://8.8.8.8/mask.png" },
		func(r *influencerSheetRequest) { r.Size = "2688x1520" },
		func(r *influencerSheetRequest) { r.Quality = "medium" },
		func(r *influencerSheetRequest) { r.OutputFormat = "jpeg" },
		func(r *influencerSheetRequest) { r.ResponseFormat = "b64_json" },
		func(r *influencerSheetRequest) { r.N = common.GetPointer(2) },
		func(r *influencerSheetRequest) { r.Seed = 1 },
	} {
		req := influencerSheetRequest{Model: influencerImageModel, Brief: "a person"}
		mutate(&req)
		body, _, _, err := buildInfluencerSheet(req)
		assert.Error(t, err)
		assert.Nil(t, body)
	}
}

func TestInfluencerSunburstChannelCapabilityFailsClosed(t *testing.T) {
	contract := `{"image_task_contract":{"protocol":"evolink_json","modes":["text","reference","edit","masked_edit"]}}`
	ch := model.Channel{Type: constant.ChannelTypeOpenAI, OtherSettings: contract}
	for _, mode := range []string{"text", "reference", "edit", "masked_edit"} {
		assert.True(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, mode))
	}
	ch.OtherSettings = `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference","edit","masked_edit"]}}`
	for _, mode := range []string{"text", "reference", "edit", "masked_edit"} {
		assert.False(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, mode), "an impossible declaration is not a valid candidate")
	}
	ch.OtherSettings = `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference","edit"]}}`
	for _, mode := range []string{"text", "reference", "edit"} {
		assert.True(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, mode))
	}
	ch.OtherSettings = `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"]}}`
	assert.True(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, "text"))
	assert.False(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, "masked_edit"))
	ch.OtherSettings = ""
	assert.False(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, "text"))
	ch.OtherSettings = contract
	ch.ModelMapping = common.GetPointer(`{"gpt-image-2.5-sunburst":"gpt-image-2.5-flare"}`)
	assert.False(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, "text"))
	ch.ModelMapping = nil
	ch.ParamOverride = common.GetPointer(`{"quality":"low"}`)
	assert.False(t, helper.ImageTaskChannelSupportsMode(&ch, influencerImageModel, "text"))
}

func TestInfluencerSunburstFrozenMaskRestoreRejectsChangedMaterialBeforeFunding(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, db.AutoMigrate(&model.File{}))
	user := model.User{Username: "sunburst-freeze", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	driver := generationMaterialStore(t)
	makePNG := func(mask bool) []byte {
		img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
		for y := 0; y < 2; y++ {
			for x := 0; x < 3; x++ {
				alpha := uint8(255)
				if mask && x == 0 {
					alpha = 0
				}
				img.SetNRGBA(x, y, color.NRGBA{R: 100, G: 180, A: alpha})
			}
		}
		var out bytes.Buffer
		require.NoError(t, png.Encode(&out, img))
		return out.Bytes()
	}
	source, mask := makePNG(false), makePNG(true)
	urls := map[string][]byte{}
	for key, data := range map[string][]byte{"uploads/source.png": source, "uploads/mask.png": mask} {
		raw, err := driver.Upload(context.Background(), key, data, "image/png")
		require.NoError(t, err)
		urls[raw] = data
		require.NoError(t, db.Create(&model.File{FileId: key, UserId: user.Id, FileUrl: raw, FileName: key, FileSize: int64(len(data)), MimeType: "image/png", StorageKey: key, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	}
	oldClient := influencerReferenceHTTPClient
	t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
	reads := 0
	influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(r *http.Request) (*http.Response, error) {
		reads++
		data := urls[r.URL.String()]
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}, nil
	})}
	req := influencerSheetRequest{Model: influencerImageModel, ImageMode: "masked_edit", ImageURL: driver.GetPublicURL("uploads/source.png"), MaskURL: driver.GetPublicURL("uploads/mask.png"), EditInstruction: "Change the selected area only."}
	out, meta, _, err := buildInfluencerSheet(req)
	require.NoError(t, err)
	c := influencerRoutingContext("default")
	c.Set("id", user.Id)
	require.True(t, prepareInfluencerImageInputs(c, req, out))
	assert.Equal(t, 2, reads)
	assert.NotEqual(t, req.ImageURL, out["image"])
	assert.NotEqual(t, req.MaskURL, out["mask_url"])
	task, err := buildInfluencerGenerationTask(user.Id, influencerImageGroup, "/pg/images/edits", ActionInfluencerSheet, out, meta, c.GetString("influencer_reference_preflight"))
	require.NoError(t, err)
	attachInfluencerImageInputs(c, task)
	require.NotNil(t, task.PrivateData.GenerationQueue.ImageInputs)
	restored, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, "masked_edit", restored.GetString(ctxInfluencerImageMode))
	assert.Equal(t, 2, reads, "restoration uses frozen bytes, not mutable original URLs")
	assert.Zero(t, task.Quota)
	task.PrivateData.GenerationQueue.ImageInputs.Mask.SHA256 = strings.Repeat("0", 64)
	_, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	assert.Error(t, err)
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	discardUnacceptedInfluencerImages(c)
}

func TestInfluencerSunburstMaskByteLimitRejectsBeforeFundingAndSurvivesRestore(t *testing.T) {
	for _, restored := range []bool{false, true} {
		name := "new HTTP request"
		if restored {
			name = "restored unsent queue"
		}
		t.Run(name, func(t *testing.T) {
			db := setupInfluencerRoutingTest(t)
			require.NoError(t, db.AutoMigrate(&model.File{}, &model.Log{}))
			user := model.User{Username: "mask-byte-limit", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
			require.NoError(t, db.Create(&user).Error)
			var fundingWrites, posts atomic.Int32
			require.NoError(t, db.Callback().Update().After("gorm:update").Register("observe_mask_limit_funding", func(tx *gorm.DB) {
				if tx.Statement.Table == "users" {
					if values, ok := tx.Statement.Dest.(map[string]interface{}); ok {
						for _, field := range []string{"quota", "used_quota", "request_count"} {
							if _, exists := values[field]; exists {
								fundingWrites.Add(1)
							}
						}
					}
				}
			}))
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer upstream.Close()
			require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-image"]`))
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":0.2}`))
			channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "mask-limit-fixture-only", Group: influencerImageGroup, Models: influencerImageModel, BaseURL: &upstream.URL, OtherSettings: `{"image_task_contract":{"protocol":"evolink_json","modes":["masked_edit"],"input_formats":["png"]}}`}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
			model.InitChannelCache()
			driver := generationMaterialStore(t)
			materials := map[string][]byte{}
			for _, isMask := range []bool{false, true} {
				img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
				for y := 0; y < 2; y++ {
					for x := 0; x < 2; x++ {
						alpha := uint8(255)
						if isMask && x == 0 {
							alpha = 0
						}
						img.SetNRGBA(x, y, color.NRGBA{R: 120, A: alpha})
					}
				}
				var out bytes.Buffer
				require.NoError(t, png.Encode(&out, img))
				data, key := out.Bytes(), "uploads/source.png"
				if isMask {
					key = "uploads/mask.png"
					chunk := make([]byte, 4000000-len(data))
					binary.BigEndian.PutUint32(chunk[:4], uint32(len(chunk)-12))
					copy(chunk[4:8], "tEXt")
					copy(chunk[8:], "Comment\x00")
					binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
					data = append(append(append([]byte{}, data[:len(data)-12]...), chunk...), data[len(data)-12:]...)
					require.Len(t, data, 4000000)
				}
				raw, err := driver.Upload(context.Background(), key, data, "image/png")
				require.NoError(t, err)
				materials[raw] = data
				require.NoError(t, db.Create(&model.File{FileId: key, UserId: user.Id, FileUrl: raw, FileName: key, FileSize: int64(len(data)), MimeType: "image/png", StorageKey: key, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
			}
			oldClient := influencerReferenceHTTPClient
			t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
			influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(r *http.Request) (*http.Response, error) {
				data, ok := materials[r.URL.String()]
				require.True(t, ok, "only original material reads are substituted")
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: r}, nil
			})}
			req := influencerSheetRequest{Model: influencerImageModel, Group: influencerImageGroup, ImageMode: "masked_edit", ImageURL: driver.GetPublicURL("uploads/source.png"), MaskURL: driver.GetPublicURL("uploads/mask.png"), EditInstruction: "Change only the selected pixels."}
			if !restored {
				engine := gin.New()
				engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
					c.Set("id", user.Id)
					common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				}, PrepareInfluencerSheet, middleware.Distribute(), PostInfluencerSheet)
				server := httptest.NewServer(engine)
				defer server.Close()
				body, err := common.Marshal(map[string]any{"model": req.Model, "group": req.Group, "image_mode": req.ImageMode, "image_url": req.ImageURL, "mask_url": req.MaskURL, "edit_instruction": req.EditInstruction})
				require.NoError(t, err)
				response, err := server.Client().Post(server.URL+"/pg/influencer/sheet", "application/json", bytes.NewReader(body))
				require.NoError(t, err)
				defer response.Body.Close()
				content, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, response.StatusCode, string(content))
				assert.Contains(t, string(content), "4,000,000")
			} else {
				// Model a pre-upgrade, validated-and-frozen WAITING task without
				// calling the newly tightened mask validator during fixture setup.
				inputs := &model.GenerationImageInputs{Version: 1, Mode: "masked_edit"}
				for _, item := range []struct {
					raw    string
					target **model.GenerationImageReference
				}{{req.ImageURL, &inputs.Source}, {req.MaskURL, &inputs.Mask}} {
					frozen, err := service.FreezeInfluencerImage(context.Background(), user.Id, item.raw, time.Now().Add(time.Hour), influencerReferenceHTTPClient)
					require.NoError(t, err)
					*item.target = &frozen.InfluencerFrozenImage
				}
				out, meta, _, err := buildInfluencerSheet(req)
				require.NoError(t, err)
				c := influencerRoutingContext("default")
				c.Set(ctxInfluencerImageInputs, inputs)
				applyInfluencerImageInputs(c, out, inputs)
				task, err := buildInfluencerGenerationTask(user.Id, influencerImageGroup, "/pg/images/edits", ActionInfluencerSheet, out, meta, inputs.Source.URL)
				require.NoError(t, err)
				attachInfluencerImageInputs(c, task)
				require.NoError(t, task.Insert())
				scanner := &model.SystemTask{TaskID: "mask-limit-scanner"}
				require.NoError(t, db.Create(&model.SystemTaskLock{Type: model.SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
				claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "runner"))
				claimed, err = model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				_, err = generationWorkerContext(context.Background(), claimed[0], scanner, "runner")
				require.ErrorContains(t, err, "4,000,000")
				dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "runner")
				dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "runner")
				var saved model.Task
				require.NoError(t, db.First(&saved, task.ID).Error)
				assert.EqualValues(t, model.TaskStatusFailure, saved.Status)
				assert.Zero(t, saved.Quota)
				assert.Zero(t, saved.PrivateData.GenerationQueue.SendStartedAt)
				assert.Equal(t, inputs.Mask.SHA256, saved.PrivateData.GenerationQueue.ImageInputs.Mask.SHA256)
			}
			var after model.User
			require.NoError(t, db.First(&after, user.Id).Error)
			assert.Equal(t, user.Quota, after.Quota)
			assert.Zero(t, after.UsedQuota)
			assert.Zero(t, after.RequestCount)
			assert.Zero(t, fundingWrites.Load())
			assert.Zero(t, posts.Load())
			var logs, tasks, files int64
			require.NoError(t, db.Model(&model.Log{}).Count(&logs).Error)
			require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
			require.NoError(t, db.Model(&model.File{}).Where("status = ?", model.FileStatusActive).Count(&files).Error)
			assert.Zero(t, logs)
			if restored {
				assert.EqualValues(t, 1, tasks)
				assert.EqualValues(t, 4, files, "restoration preserves the existing original and frozen materials")
			} else {
				assert.Zero(t, tasks)
				assert.EqualValues(t, 2, files, "only newly prepared rejected copies are discarded")
			}
			t.Logf("restored=%t supplier_posts=%d funding_writes=%d quota=%d logs=%d tasks=%d", restored, posts.Load(), fundingWrites.Load(), after.Quota, logs, tasks)
		})
	}
}

func TestInfluencerSunburstTextQueueUsesOnlyCompatibleNativeCandidates(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-image"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":0.2}`))
	user := model.User{Username: "sunburst-text", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	for _, settings := range []string{`{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"]}}`, ``} {
		channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "fixture-only", Group: influencerImageGroup, Models: influencerImageModel, OtherSettings: settings}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	}
	c := influencerRoutingContext("default")
	c.Set("id", user.Id)
	out, meta, _, err := buildInfluencerSheet(influencerSheetRequest{Model: influencerImageModel, Brief: "a person"})
	require.NoError(t, err)
	require.True(t, prepareInfluencerImageInputs(c, influencerSheetRequest{Model: influencerImageModel}, out))
	require.True(t, prepareInfluencerSheetRouting(c, influencerImageModel, influencerImageGroup, false))
	task, err := buildInfluencerGenerationTask(user.Id, influencerImageGroup, "/pg/images/generations", ActionInfluencerSheet, out, meta, "")
	require.NoError(t, err)
	task.PrivateData.GenerationQueue.Routing = generationRequestRouting(c, influencerImageGroup, "/pg/images/generations")
	attachInfluencerImageInputs(c, task)
	restored, err := generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	require.NoError(t, err)
	assert.Equal(t, "text", restored.GetString(ctxInfluencerImageMode))
	assert.Equal(t, 2, task.PrivateData.GenerationQueue.Routing.MaxAttempts)
	model.InitChannelCache()
	engine := gin.New()
	engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
		c.Set("id", user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	}, PrepareInfluencerSheet, middleware.Distribute(), PostInfluencerSheet)
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","brief":"a person"}`))
	httpRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, httpRequest)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var accepted model.Task
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&accepted).Error)
	require.NotNil(t, accepted.PrivateData.GenerationQueue.ImageInputs)
	assert.Equal(t, "text", accepted.PrivateData.GenerationQueue.ImageInputs.Mode)
	assert.Equal(t, model.TaskStatusWaiting, accepted.Status)
	assert.Zero(t, accepted.Quota)
	assert.NotContains(t, accepted.Properties.Input, "sha256")
	task.PrivateData.GenerationQueue.ImageInputs = nil
	_, err = generationWorkerContext(context.Background(), task, &model.SystemTask{}, "runner")
	assert.Error(t, err, "historical or missing snapshots must never silently enter the new contract")
}

func TestInfluencerSunburstForeignUploadIsRejectedBeforeFetch(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, db.AutoMigrate(&model.File{}))
	driver := generationMaterialStore(t)
	raw := driver.GetPublicURL("uploads/foreign.png")
	require.NoError(t, db.Create(&model.File{FileId: "foreign", UserId: 99, FileUrl: raw, StorageKey: "uploads/foreign.png", Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	c := influencerRoutingContext("default")
	c.Set("id", 41)
	_, err := service.ResolveInfluencerImageSourceExpiry(c.Request.Context(), c.GetInt("id"), raw)
	assert.Error(t, err)
}

func TestInfluencerSunburstQueuedSelectionSkipsIncompatiblePreferredChannel(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "sunburst-mode-selection", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	var compatibleID int
	for i, contract := range []string{
		`{"protocol":"openai_multipart","modes":["text","reference","edit","masked_edit"]}`,
		`{"protocol":"evolink_json","modes":["text"]}`,
		`{"protocol":"evolink_json","modes":["text","reference","edit","masked_edit"],"input_formats":["jpeg"]}`,
		`{"protocol":"evolink_json","modes":["text","reference","edit","masked_edit"]}`,
	} {
		priority := int64(200 - i*50)
		channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", Models: influencerImageModel, Group: influencerImageGroup, Priority: &priority, OtherSettings: `{"image_task_contract":` + contract + `}`}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true, Priority: &priority}).Error)
		if i == 3 {
			compatibleID = channel.Id
		}
	}
	model.InitChannelCache()
	c := influencerRoutingContext("default")
	c.Set("id", user.Id)
	c.Set("influencer_frozen_image", service.InfluencerFrozenImage{Format: "png"})
	c.Set("influencer_frozen_mask", service.InfluencerFrozenImage{Format: "png"})
	inputs := &model.GenerationImageInputs{Version: 1, Mode: "masked_edit"}
	c.Set(ctxInfluencerImageInputs, inputs)
	c.Set(ctxInfluencerImageMode, inputs.Mode)
	c.Set(helper.ImageTaskRequestSizeContextKey, "2048x1152")
	c.Set("generation_queue_reserved_task", &model.Task{ID: 1, UserId: user.Id, Group: influencerImageGroup, Action: ActionInfluencerSheet, Properties: model.Properties{OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{ImageInputs: inputs}}})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(`{"model":"gpt-image-2.5-sunburst","prompt":"change the selected area","image":"https://8.8.4.4/source.png","mask_url":"https://8.8.4.4/mask.png"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, influencerImageGroup)
	require.NoError(t, helper.AppendImageTaskModeFilter(c, influencerImageModel))
	channel, _, err := service.SelectChannelForRequest(c, influencerImageModel, &service.RetryParam{Ctx: c, TokenGroup: influencerImageGroup, ModelName: influencerImageModel})
	require.Nil(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, compatibleID, channel.Id, "implemented protocol, mode and actual frozen PNG format narrow native priority before selection")
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	assert.Zero(t, after.RequestCount)
}

func TestInfluencerSunburstUnimplementedContractStopsBeforeDistributionAndFunding(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-image"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":0.2}`))
	user := model.User{Username: "sunburst-invalid-contract", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	posts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-only", BaseURL: common.GetPointer(upstream.URL), Group: influencerImageGroup, Models: influencerImageModel, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference","edit","masked_edit"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	model.InitChannelCache()
	engine := gin.New()
	distributed := false
	engine.POST("/pg/influencer/sheet", func(c *gin.Context) {
		c.Set("id", user.Id)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	}, PrepareInfluencerSheet, func(c *gin.Context) { distributed = true }, middleware.Distribute(), PostInfluencerSheet)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","brief":"a blue kettle"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), "generation_route_unavailable")
	assert.False(t, distributed)
	assert.Zero(t, posts)
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.Zero(t, count)
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	assert.Zero(t, after.RequestCount)
}

func TestInfluencerSunburstRejectsExplicitUnsupportedFieldsBeforeFunding(t *testing.T) {
	db := setupInfluencerRoutingTest(t)
	user := model.User{Username: "image-feature-guard", Group: "default", Status: common.UserStatusEnabled, Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	for _, extra := range []string{`"seed":0`, `"metadata":{"seed":0}`, `"background":"transparent"`, `"quality":"medium"`, `"n":2`} {
		t.Run(extra, func(t *testing.T) {
			engine := gin.New()
			reached := false
			engine.POST("/pg/influencer/sheet", func(c *gin.Context) { c.Set("id", user.Id) }, PrepareInfluencerSheet, func(c *gin.Context) { reached = true })
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/pg/influencer/sheet", strings.NewReader(`{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","brief":"a blue kettle",`+extra+`}`))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, reached)
			var after model.User
			require.NoError(t, db.First(&after, user.Id).Error)
			assert.Equal(t, user.Quota, after.Quota)
			assert.Zero(t, after.UsedQuota)
			var tasks int64
			require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
			assert.Zero(t, tasks)
		})
	}
}

func TestInfluencerSunburstRawEntryIsClosedBeforeSelectionFundingOrSend(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupInfluencerRoutingTest(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","gpt-image-2.5-sunburst-image":"Images"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":1}`))
	user := model.User{Username: "raw-image-closed", Status: common.UserStatusEnabled, Group: "default", Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	channel := model.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "fixture-only", Models: influencerImageModel, Group: influencerImageGroup, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	model.InitChannelCache()
	for _, path := range []string{"/v1/images/generations", "/v1/images/edits", "/pg/images/generations", "/pg/images/edits", "/v1/videos", "/v1/video/generations", "/pg/video/generations", "/v1/chat/completions", "/pg/influencer/sheet", "/pg/influencer/multiview"} {
		t.Run(path, func(t *testing.T) {
			engine := gin.New()
			reached := false
			engine.POST(path, func(c *gin.Context) {
				c.Set("id", user.Id)
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			}, middleware.Distribute(), func(c *gin.Context) { reached = true })
			recorder := httptest.NewRecorder()
			// Body and headers cannot forge typed, server-created preparation.
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","prompt":"a kettle","task_type":"image_task_v1","influencer_image_inputs":{"version":1,"mode":"text"},"generation_queue_reserved_task":{"user_id":1}}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Omnimux-Group", influencerImageGroup)
			req.Header.Set("X-Task-Type", "image_task_v1")
			engine.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), helper.ErrImageTaskDurableEntryRequired.Error())
			assert.False(t, reached)
		})
	}
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	assert.Equal(t, user.Quota, after.Quota)
	assert.Zero(t, after.UsedQuota)
	var tasks int64
	require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
	assert.Zero(t, tasks)
}

func TestInfluencerSunburstUnverifiedResultIsVisibleWithoutPrivateDetails(t *testing.T) {
	task := setupGenericTaskTest(t)
	task.Status = model.TaskStatusInProgress
	task.Action = ActionInfluencerSheet
	task.Properties.OriginModelName = influencerImageModel
	task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
	task.PrivateData.ImageResultUnverified = true
	task.PrivateData.UpstreamTaskID = "private-upstream-identity"
	require.NoError(t, model.DB.Save(task).Error)
	view := relay.TaskModel2Dto(task)
	assert.True(t, view.ResultUnverified)
	assert.Equal(t, "reconciling", influencerCLITaskStatus(task))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", task.UserId)
	c.Params = gin.Params{{Key: "key", Value: task.TaskID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+task.TaskID, nil)
	GetTask(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, true, body["result_unverified"])
	assert.NotContains(t, recorder.Body.String(), "private-upstream-identity")
	assert.NotContains(t, recorder.Body.String(), "image_inputs")
}
