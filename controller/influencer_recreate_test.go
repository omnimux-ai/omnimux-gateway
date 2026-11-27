package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/apimart"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recreateTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/pg/influencer/motion", nil)
	c.Set("id", 1)
	return c
}

func validRecreatePrompt() string {
	return "Reference image 1 is the hero product, a matte black perfume bottle; it defines the cap and the glass body. " +
		"A hand lifts it against a pale grey counter in one continuous take. The frame stays free of captions, lettering, logos and watermarks."
}

// The recreation submit body must carry images and the compiled prompt and must
// never carry the reference clip: that clip was decomposition material.
func TestPrepareInfluencerMotionRequestRecreateHasNoVideo(t *testing.T) {
	c := recreateTestContext()
	req := influencerMotionRequest{
		Mode:             "recreate",
		Prompt:           validRecreatePrompt(),
		SubjectImageURLs: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
		DurationSec:      4,
		Resolution:       "1080p",
	}
	out, meta, prepared, ok := prepareInfluencerMotionRequest(c, req, true)
	require.True(t, ok)
	require.NotNil(t, prepared)
	assert.NotContains(t, out, "video_urls")
	assert.NotContains(t, out, "audio_urls")
	assert.NotContains(t, out, "videos")
	assert.NotContains(t, out, "audios")
	assert.Equal(t, "magi-2", out["model"])
	assert.Equal(t, "default", out["group"])
	assert.Equal(t, 4, out["duration"])
	assert.Equal(t, "9:16", out["aspect_ratio"])
	assert.Equal(t, "1080p", out["resolution"])
	assert.Equal(t, true, out["generate_audio"])
	assert.Equal(t, req.SubjectImageURLs, out["image_urls"])
	assert.NotContains(t, out, "images")
	assert.NotContains(t, out, "seconds")
	assert.Equal(t, "recreate", meta["mode"])
	assert.Equal(t, req.SubjectImageURLs, meta["subject_image_urls"])
}

func TestPrepareInfluencerMotionRequestRecreateRejectsBadPrompt(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"video wording":  "Reference video 1 is the source. Reference image 1 is the product. The frame stays free of captions and logos.",
		"missing images": "A product rotates in a soft studio. The frame stays free of captions, lettering, logos and watermarks.",
		"too long":       validRecreatePrompt() + strings.Repeat("文字", 2500),
	}
	for name, prompt := range cases {
		t.Run(name, func(t *testing.T) {
			c := recreateTestContext()
			_, _, _, ok := prepareInfluencerMotionRequest(c, influencerMotionRequest{
				Mode:             "recreate",
				Prompt:           prompt,
				SubjectImageURLs: []string{"https://cdn.example.com/a.jpg"},
				DurationSec:      4,
				Resolution:       "1080p",
			}, true)
			assert.False(t, ok)
		})
	}
}

func TestInfluencerRecreatePromptGate(t *testing.T) {
	clean := validRecreatePrompt()
	assert.Empty(t, service.InfluencerRecreatePromptGate(clean, 2))

	assert.NotEmpty(t, service.InfluencerRecreatePromptGate(strings.Replace(clean, "image 1", "image 2", 1), 1), "image 2 does not exist")
	assert.NotEmpty(t, service.InfluencerRecreatePromptGate(strings.Replace(clean, "image 1", "image 4", 1), 2))
	assert.NotEmpty(t, service.InfluencerRecreatePromptGate(strings.Replace(clean, "Reference image 1", "The original clip", 1), 2))
	assert.NotEmpty(t, service.InfluencerRecreatePromptGate("Reference image 1 is the product.", 1), "needs the no-text constraint")
	assert.NotEmpty(t, service.InfluencerRecreatePromptGate(strings.Replace(clean, "free of captions, lettering, logos and watermarks", "with captions and logos", 1), 1))
}

// Historical recreation tasks predate video_reference_url and must keep planning
// from their original video_url. Both canonical and legacy media keys are cleared.
func TestPrepareInfluencerVideoPromptRecreatePlansAndDropsVideo(t *testing.T) {
	const finalPrompt = "Reference image 1 is the hero product. The frame stays free of captions, lettering, logos and watermarks."
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	var seen service.InfluencerRecreatePlanInput
	influencerRecreatePlanner = func(_ context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		seen = in
		return service.InfluencerRecreatePlan{Prompt: finalPrompt, DurationSec: 8}, nil
	}

	input, err := common.Marshal(map[string]any{
		"mode":               "recreate",
		"prompt":             "keep it punchy",
		"video_url":          "https://cdn.example.com/reference.mp4",
		"subject_image_urls": []string{"https://cdn.example.com/hero.jpg", "https://cdn.example.com/detail.jpg"},
		"subject_image_tags": []string{"hero", "detail"},
		"resolution":         "1080p",
		"duration_sec":       4,
	})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(input)}}
	out := map[string]any{
		"prompt":     "",
		"image_urls": []any{"https://cdn.example.com/hero.jpg"},
		"video_urls": []any{"https://cdn.example.com/reference.mp4"},
		"audio_urls": []any{"https://cdn.example.com/reference.m4a"},
		"videos":     []any{"https://cdn.example.com/reference.mp4"},
		"audios":     []any{"https://cdn.example.com/reference.m4a"},
	}
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, out, nil, ""))

	assert.Equal(t, finalPrompt, out["prompt"])
	assert.Equal(t, 8, out["duration"])
	assert.Equal(t, []string{"https://cdn.example.com/hero.jpg", "https://cdn.example.com/detail.jpg"}, out["image_urls"])
	assert.NotContains(t, out, "video_urls")
	assert.NotContains(t, out, "audio_urls")
	assert.NotContains(t, out, "videos")
	assert.NotContains(t, out, "audios")
	assert.NotContains(t, out, "images")
	assert.NotContains(t, out, "seconds")
	assert.Equal(t, "https://cdn.example.com/reference.mp4", seen.VideoURL)
	assert.Equal(t, []string{"https://cdn.example.com/hero.jpg", "https://cdn.example.com/detail.jpg"}, seen.ImageURLs)
	assert.Equal(t, []string{"hero", "detail"}, seen.ImageTags)
	assert.Equal(t, "keep it punchy", seen.UserPrompt)
}

func TestPrepareInfluencerVideoPromptRecreatePreservesLegacySeedanceContract(t *testing.T) {
	const finalPrompt = "Reference image 1 is the hero product. The frame stays free of captions, lettering, logos and watermarks."
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	influencerRecreatePlanner = func(_ context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		assert.Equal(t, "480p", in.Resolution)
		return service.InfluencerRecreatePlan{Prompt: finalPrompt, DurationSec: 8}, nil
	}

	input, err := common.Marshal(map[string]any{
		"mode":               "recreate",
		"prompt":             "legacy inflight",
		"video_url":          "https://cdn.example.com/reference.mp4",
		"subject_image_urls": []string{"https://cdn.example.com/hero.jpg", "https://cdn.example.com/detail.jpg"},
		"resolution":         "480p",
		"duration_sec":       4,
	})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(input)}}
	out := map[string]any{
		"model":      "seedance-2-5",
		"group":      "seedance-2-5-480p",
		"resolution": "480p",
		"prompt":     "",
		"seconds":    "4",
		"images":     []any{"https://cdn.example.com/hero.jpg"},
		"videos":     []any{"https://cdn.example.com/reference.mp4"},
		"audios":     []any{"https://cdn.example.com/reference.m4a"},
	}
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, out, nil, ""))

	assert.Equal(t, finalPrompt, out["prompt"])
	assert.Equal(t, "8", out["seconds"])
	assert.Equal(t, []string{"https://cdn.example.com/hero.jpg", "https://cdn.example.com/detail.jpg"}, out["images"])
	assert.Equal(t, "480p", out["resolution"])
	assert.Equal(t, "seedance-2-5-480p", out["group"])
	assert.NotContains(t, out, "duration")
	assert.NotContains(t, out, "image_urls")
	assert.NotContains(t, out, "video_urls")
	assert.NotContains(t, out, "audio_urls")
	assert.NotContains(t, out, "videos")
	assert.NotContains(t, out, "audios")
}

// A browser task may carry both the original (possibly expiring) URL and the
// server-resolved stable reference; the worker must plan from the stable one.
func TestPrepareInfluencerVideoPromptRecreatePrefersStableReference(t *testing.T) {
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	var seen service.InfluencerRecreatePlanInput
	influencerRecreatePlanner = func(_ context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		seen = in
		return service.InfluencerRecreatePlan{Prompt: validRecreatePrompt(), DurationSec: 8}, nil
	}

	const (
		originalURL = "https://files.omnimux.ai/v1/tasks/history/artifacts/video/content?access=expiring"
		stableURL   = "https://files.omnimux.ai/influencer-reference/frozen/history.mp4"
	)
	input, err := common.Marshal(map[string]any{
		"mode":                "recreate",
		"video_url":           originalURL,
		"video_reference_url": stableURL,
		"subject_image_urls":  []string{"https://cdn.example.com/hero.jpg"},
		"resolution":          "1080p",
	})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(input)}}
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, map[string]any{"prompt": ""}, nil, ""))
	assert.Equal(t, stableURL, seen.VideoURL)
}

func TestPrepareInfluencerVideoPromptRecreateFailureStopsBeforeVideo(t *testing.T) {
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	influencerRecreatePlanner = func(context.Context, service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		return service.InfluencerRecreatePlan{}, service.ErrInfluencerRecreateTimeout
	}
	input, err := common.Marshal(map[string]any{
		"mode":               "recreate",
		"video_url":          "https://cdn.example.com/reference.mp4",
		"subject_image_urls": []string{"https://cdn.example.com/hero.jpg"},
		"resolution":         "1080p",
	})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(input)}}
	out := map[string]any{"prompt": "", "image_urls": []any{"https://cdn.example.com/hero.jpg"}}
	err = prepareInfluencerVideoPrompt(context.Background(), task, out, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超时")
	assert.Equal(t, "https://cdn.example.com/hero.jpg", out["image_urls"].([]any)[0])
	assert.Empty(t, out["prompt"], "a failed plan must not install a guessed prompt")
}

func TestPostInfluencerRecreateAnalyzeValidatesInput(t *testing.T) {
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	called := false
	influencerRecreatePlanner = func(ctx context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		called = true
		return service.InfluencerRecreatePlan{Prompt: validRecreatePrompt()}, nil
	}

	cases := map[string]map[string]any{
		"missing video":  {"image_urls": []string{"https://cdn.example.com/a.jpg"}},
		"insecure video": {"video_url": "http://cdn.example.com/a.mp4", "image_urls": []string{"https://cdn.example.com/a.jpg"}},
		"no images":      {"video_url": "https://cdn.example.com/a.mp4"},
		"too many":       {"video_url": "https://cdn.example.com/a.mp4", "image_urls": tooManyURLs(11)},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			raw, err := common.Marshal(body)
			require.NoError(t, err)
			c.Request = httptest.NewRequest("POST", "/pg/influencer/recreate/analyze", strings.NewReader(string(raw)))
			c.Request.Header.Set("Content-Type", "application/json")
			PostInfluencerRecreateAnalyze(c)
			assert.Equal(t, 400, recorder.Code)
			assert.False(t, called)
		})
	}
}

func TestPrepareInfluencerMotionRequestRecreateDurationContract(t *testing.T) {
	for _, seconds := range []int{4, 8, 15, 30} {
		c := recreateTestContext()
		out, _, _, ok := prepareInfluencerMotionRequest(c, influencerMotionRequest{
			Mode:             "recreate",
			Prompt:           validRecreatePrompt(),
			SubjectImageURLs: []string{"https://cdn.example.com/a.jpg"},
			DurationSec:      seconds,
			Resolution:       "1080p",
		}, true)
		require.True(t, ok, "seconds=%d", seconds)
		assert.Equal(t, seconds, out["duration"])
	}
	for _, seconds := range []int{0, 3, 31} {
		c := recreateTestContext()
		_, _, _, ok := prepareInfluencerMotionRequest(c, influencerMotionRequest{
			Mode:             "recreate",
			Prompt:           validRecreatePrompt(),
			SubjectImageURLs: []string{"https://cdn.example.com/a.jpg"},
			DurationSec:      seconds,
			Resolution:       "1080p",
		}, true)
		assert.False(t, ok, "seconds=%d must not be billable", seconds)
	}
}

// The generate button's amount follows the motion surface's per-second quote:
// model price x resolution group ratio x seconds, then USD -> credits.
func TestInfluencerRecreatePricingUsesResolutionGroup(t *testing.T) {
	originalPrices, err := common.Marshal(ratio_setting.GetModelPriceMap())
	require.NoError(t, err)
	originalGroups, err := common.Marshal(ratio_setting.GetGroupRatioCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ratio_setting.UpdateModelPriceByJSONString(string(originalPrices))
		_ = ratio_setting.UpdateGroupRatioByJSONString(string(originalGroups))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"magi-2":0.11}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	c := recreateTestContext()
	c.Set("group", "default")
	pricing := influencerRecreatePricing(c, 11)
	require.NotNil(t, pricing)
	// 0.11 USD/s x 11 s x 1 x 10 credits/USD.
	assert.InDelta(t, 12.10, pricing["1080p"], 0.01)
}

func recreateAnalyzeContext(t *testing.T, body map[string]any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	raw, err := common.Marshal(body)
	require.NoError(t, err)
	c.Request = httptest.NewRequest("POST", "/pg/influencer/recreate/analyze", strings.NewReader(string(raw)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 7)
	return c, recorder
}

func validRecreateAnalyzeBody() map[string]any {
	return map[string]any{
		"video_url":  "https://cdn.example.com/a.mp4",
		"image_urls": []string{"https://cdn.example.com/a.jpg"},
		"resolution": "1080p",
	}
}

func TestInfluencerRecreateDailyLimitConfig(t *testing.T) {
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "")
	assert.Equal(t, 20, influencerRecreateDailyLimit())
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "5")
	assert.Equal(t, 5, influencerRecreateDailyLimit())
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "not-a-number")
	assert.Equal(t, 20, influencerRecreateDailyLimit())
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "0")
	assert.Equal(t, 0, influencerRecreateDailyLimit())
}

// The cap never touches a deployment without Redis; the request limiter still
// gates the route.
func TestInfluencerRecreateDailyAllowWithoutRedis(t *testing.T) {
	original := common.RedisEnabled
	t.Cleanup(func() { common.RedisEnabled = original })
	common.RedisEnabled = false
	c := recreateTestContext()
	assert.True(t, influencerRecreateDailyAllow(c))
}

// setupRecreateAnalysisDB installs a throwaway database with one claimed
// analysis job, mirroring what the queue hands to the worker.
func setupRecreateAnalysisDB(t *testing.T) (*model.Task, *model.User) {
	t.Helper()
	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Task{})
	oldDB, oldLog := model.DB, model.LOG_DB
	oldMain, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLog
		common.SetDatabaseTypes(oldMain, oldLogType)
		common.RedisEnabled = oldRedis
	})
	user := &model.User{Username: "recreate_worker", AffCode: "recreate_worker", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000}
	require.NoError(t, db.Create(user).Error)
	body, err := common.Marshal(influencerRecreateAnalysisSnapshot{
		VideoURL:   "https://cdn.example.com/a.mp4",
		ImageURLs:  []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
		Prompt:     "换我的产品",
		Resolution: "1080p",
	})
	require.NoError(t, err)
	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     user.Id,
		Group:      "default",
		Action:     ActionInfluencerRecreateAnalysis,
		Status:     model.TaskStatusClaimed,
		Progress:   "10%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{Input: string(body), OriginModelName: influencerMotionModel},
		PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{
			Path:       influencerRecreateAnalysisQueuePath,
			Body:       body,
			ClaimOwner: "scan_test",
		}},
	}
	require.NoError(t, task.InsertWithContext(context.Background()))
	return task, user
}

// The queue worker stores the plan on the task and the poll surface returns it
// to the owner; no provider request is involved.
func TestRecreateAnalysisWorkerStoresResultAndPollReadsIt(t *testing.T) {
	task, user := setupRecreateAnalysisDB(t)
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	influencerRecreatePlanner = func(ctx context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		assert.Equal(t, 2, len(in.ImageURLs))
		return service.InfluencerRecreatePlan{
			Prompt:           validRecreatePrompt(),
			AnalysisJSON:     `{"shots":[{"shot_id":"S01","start_s":0,"end_s":4.2,"narrative_job":"hook"}]}`,
			ImageJSON:        `[{"image_index":1,"kind":"product","summary":"香水瓶"}]`,
			ShotCount:        1,
			DurationSec:      11,
			ReferenceSeconds: 10.51,
			Hook:             "curiosity_gap",
			RiskFlags:        []string{"text_leak"},
		}, nil
	}
	runInfluencerRecreateAnalysis(context.Background(), task)

	reloaded, exists, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)
	assert.Contains(t, string(reloaded.Data), validRecreatePrompt()[:20])

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/pg/influencer/recreate/"+task.TaskID, nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", user.Id)
	GetInfluencerRecreateAnalysis(c)
	require.Equal(t, 200, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"status":"success"`)
	assert.Contains(t, body, `"duration_sec":11`)
	assert.Contains(t, body, `"reference_duration_sec":10.51`)
	assert.Contains(t, body, `"hook":"curiosity_gap"`)
}

// A plan failure becomes a terminal task the poll surface reports with its reason.
func TestRecreateAnalysisWorkerRecordsFailure(t *testing.T) {
	task, user := setupRecreateAnalysisDB(t)
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	influencerRecreatePlanner = func(ctx context.Context, in service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		return service.InfluencerRecreatePlan{}, service.ErrInfluencerRecreateTimeout
	}
	runInfluencerRecreateAnalysis(context.Background(), task)

	reloaded, exists, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Contains(t, reloaded.FailReason, "超时")

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/pg/influencer/recreate/"+task.TaskID, nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", user.Id)
	GetInfluencerRecreateAnalysis(c)
	require.Equal(t, 200, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"failed"`)
	assert.Contains(t, recorder.Body.String(), "超时")
}

// A poll by another account does not reveal the job.
func TestRecreateAnalysisPollIsOwnerScoped(t *testing.T) {
	task, _ := setupRecreateAnalysisDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/pg/influencer/recreate/"+task.TaskID, nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", 999999)
	GetInfluencerRecreateAnalysis(c)
	assert.Equal(t, 404, recorder.Code)
}

// The token-scoped acceptance route follows the command-line creation switch.
func TestPostInfluencerRecreateAnalyzeCLIWriteGate(t *testing.T) {
	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "false")
	c, recorder := recreateAnalyzeContext(t, validRecreateAnalyzeBody())
	c.Set("influencer_creation_api", true)
	PostInfluencerRecreateAnalyze(c)
	assert.Equal(t, 503, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "influencer_creation_disabled")
}

func tooManyURLs(count int) []string {
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		urls = append(urls, "https://cdn.example.com/image.jpg")
	}
	return urls
}

func TestPrepareInfluencerMotionRecreateQueuedUsesBrowserClipAndMetadata(t *testing.T) {
	req := map[string]any{
		"mode":               "recreate",
		"prompt":             "keep the hook but make it mine",
		"subject_image_urls": []string{motionTestImage},
		"video_url":          motionTestVideo,
		"duration_sec":       4,
		"resolution":         "1080p",
		"video_metadata": service.InfluencerVideoMetadata{
			Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24,
			VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 1234,
		},
	}
	rec, probe := runMotionPrepare(t, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.True(t, probe.reached)
	assert.NotContains(t, probe.body, "video_urls")
	assert.NotContains(t, probe.body, "videos")
	assert.Equal(t, "", probe.body["prompt"], "the worker owns prompt compilation")
	assert.Equal(t, float64(4), probe.body["duration"])
	assert.Equal(t, "9:16", probe.body["aspect_ratio"])
	assert.Equal(t, []any{motionTestImage}, probe.body["image_urls"])
	assert.Equal(t, "keep the hook but make it mine", probe.meta["prompt"])
	assert.Equal(t, motionTestVideo, probe.meta["video_url"])
	assert.Equal(t, "recreate", probe.meta["mode"])
	assert.NotNil(t, probe.meta["video_metadata"])
}

// The queued Web path keeps the synchronous /recreate/analyze intent budget
// instead of accepting the wider generic motion prompt limit.
func TestPrepareInfluencerMotionRecreateQueuedRejectsOverlongIntent(t *testing.T) {
	req := map[string]any{
		"mode":               "recreate",
		"prompt":             strings.Repeat("字", influencerRecreateMaxIntentRunes+1),
		"subject_image_urls": []string{motionTestImage},
		"video_url":          motionTestVideo,
		"duration_sec":       4,
		"resolution":         "1080p",
		"video_metadata": service.InfluencerVideoMetadata{
			Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24,
			VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 1234,
		},
	}
	rec, probe := runMotionPrepare(t, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.False(t, probe.reached)
}

func TestInfluencerRecreateDispatchFailureStopsBeforeVideoAndReleasesReservation(t *testing.T) {
	var videoCalls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		videoCalls.Add(1)
		_, _ = w.Write([]byte(`{"id":"must-not-be-submitted"}`))
	}))
	precharged := generationMaterialObservePrecharge(t, db)
	original := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = original })
	influencerRecreatePlanner = func(context.Context, service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		return service.InfluencerRecreatePlan{}, errors.New("reference video decomposition failed")
	}
	out := map[string]any{
		"model": "queue-correction", "prompt": "", "duration": 4,
		"aspect_ratio": "9:16", "resolution": "1080p",
		"image_urls": []any{"https://8.8.4.4/hero.png"},
	}
	meta := map[string]any{
		"kind": "motion", "mode": "recreate", "prompt": "make it mine",
		"video_url":          "https://cdn.example.com/reference.mp4",
		"subject_image_urls": []string{"https://8.8.4.4/hero.png"},
		"resolution":         "1080p", "duration_sec": 4,
	}
	task := generationMaterialQueue(t, user, out, meta, true)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "recreate-failure")

	assert.Zero(t, videoCalls.Load(), "a failed decomposition must never reach provider video POST")
	assert.False(t, precharged.Load(), "native video funds must never be reserved")
	var stored model.Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Contains(t, stored.FailReason, "decomposition failed")
	assert.Zero(t, stored.Quota)
	assert.Nil(t, stored.PrivateData.BillingContext)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
}

func TestPostInfluencerMotionRecreateReturnsTaskIDImmediately(t *testing.T) {
	settings := system_setting.GetFetchSetting()
	previousSettings := *settings
	settings.ApplyIPFilterForDomain = false
	t.Cleanup(func() { *settings = previousSettings })
	var providerCalls atomic.Int32
	_, user, _ := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerCalls.Add(1)
		_, _ = w.Write([]byte(`{"id":"must-not-be-submitted"}`))
	}))
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/pg/influencer/motion", func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("group", "default")
	}, PrepareInfluencerMotion, PostInfluencerMotion)
	body := map[string]any{
		"mode":               "recreate",
		"prompt":             "keep the hook but make it mine",
		"subject_image_urls": []string{motionTestImage},
		"video_url":          motionTestVideo,
		"duration_sec":       4,
		"resolution":         "1080p",
		"video_metadata": service.InfluencerVideoMetadata{
			Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24,
			VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 1234,
		},
	}
	raw, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/pg/influencer/motion", strings.NewReader(string(raw)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		TaskID string `json:"task_id"`
		ID     string `json:"id"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	taskID := response.TaskID
	if taskID == "" {
		taskID = response.ID
	}
	require.NotEmpty(t, taskID)
	stored, exists, err := model.GetByTaskId(user.Id, taskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, ActionInfluencerMotion, stored.Action)
	assert.Equal(t, model.TaskStatus(model.TaskStatusWaiting), stored.Status)
	assert.Zero(t, providerCalls.Load(), "queue acceptance must not call the provider")
	var meta struct {
		Mode     string `json:"mode"`
		VideoURL string `json:"video_url"`
		Prompt   string `json:"prompt"`
	}
	require.NoError(t, common.UnmarshalJsonStr(stored.Properties.Input, &meta))
	assert.Equal(t, "recreate", meta.Mode)
	assert.Equal(t, motionTestVideo, meta.VideoURL)
	assert.Equal(t, "keep the hook but make it mine", meta.Prompt)
}

// The Web queued path consumes the same daily analysis budget as the
// synchronous /recreate/analyze route: an exhausted budget must be rejected
// before a second task row is created.
func TestPostInfluencerMotionRecreateDailyAnalysisBudget(t *testing.T) {
	settings := system_setting.GetFetchSetting()
	previousSettings := *settings
	settings.ApplyIPFilterForDomain = false
	t.Cleanup(func() { *settings = previousSettings })

	db, user, _ := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("queue acceptance must not call the provider")
	}))
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = true
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "1")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/pg/influencer/motion", func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("group", "default")
	}, PrepareInfluencerMotion, PostInfluencerMotion)

	body := map[string]any{
		"mode":               "recreate",
		"prompt":             "keep the hook but make it mine",
		"subject_image_urls": []string{motionTestImage},
		"video_url":          motionTestVideo,
		"duration_sec":       4,
		"resolution":         "1080p",
		"video_metadata": service.InfluencerVideoMetadata{
			Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24,
			VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 1234,
		},
	}
	raw, err := common.Marshal(body)
	require.NoError(t, err)
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/pg/influencer/motion", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		return recorder
	}

	first := request()
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := request()
	require.Equal(t, http.StatusTooManyRequests, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "recreate_analysis_limit")

	var count int64
	require.NoError(t, db.Model(&model.Task{}).Where("user_id = ? AND action = ?", user.Id, ActionInfluencerMotion).Count(&count).Error)
	assert.EqualValues(t, 1, count, "an over-budget request must not create a task")
}

// The queued recreation worker must write planner output into the request body
// storage used by native adaptors, and the native adaptor must pass MAGI-2
// validation and build the reference-to-video request without video_urls.
func TestInfluencerRecreateWorkerBodyFeedsAPIMartPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const finalPrompt = "Reference image 1 is the hero product. The frame stays free of captions, lettering, logos and watermarks."

	out := map[string]any{
		"model":          "magi-2",
		"prompt":         "",
		"duration":       4,
		"aspect_ratio":   "9:16",
		"resolution":     "1080p",
		"generate_audio": true,
		"image_urls":     []string{"https://cdn.example.com/hero.png"},
	}
	initialBody, err := common.Marshal(out)
	require.NoError(t, err)
	storage, err := common.CreateBodyStorage(initialBody)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(string(initialBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyBodyStorage, storage)

	originalPlanner := influencerRecreatePlanner
	t.Cleanup(func() { influencerRecreatePlanner = originalPlanner })
	influencerRecreatePlanner = func(context.Context, service.InfluencerRecreatePlanInput) (service.InfluencerRecreatePlan, error) {
		return service.InfluencerRecreatePlan{Prompt: finalPrompt, DurationSec: 4}, nil
	}
	meta, err := common.Marshal(map[string]any{
		"mode":               "recreate",
		"prompt":             "make it mine",
		"video_url":          "https://cdn.example.com/reference.mp4",
		"subject_image_urls": []string{"https://cdn.example.com/hero.png"},
		"resolution":         "1080p",
	})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(meta)}}

	parsed := map[string]any{}
	require.NoError(t, common.Unmarshal(initialBody, &parsed))
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, parsed, nil, ""))
	require.NoError(t, writeInfluencerMotionTaskRequestBody(c, parsed))

	info := &relaycommon.RelayInfo{
		OriginModelName: "magi-2",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	adaptor := &apimart.TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info), "the worker body must pass APIMart magi-2 validation")
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	raw, err := io.ReadAll(reader)
	require.NoError(t, err)
	var providerBody map[string]any
	require.NoError(t, common.Unmarshal(raw, &providerBody))
	assert.Equal(t, "magi-2-reference-to-video", providerBody["model"])
	assert.Equal(t, finalPrompt, providerBody["prompt"])
	assert.Equal(t, []any{"https://cdn.example.com/hero.png"}, providerBody["image_urls"])
	assert.Equal(t, float64(4), providerBody["duration"])
	assert.Equal(t, "1080p", providerBody["quality"])
	assert.Equal(t, "9:16", providerBody["aspect_ratio"])
	assert.Equal(t, true, providerBody["generate_audio"])
	assert.NotContains(t, providerBody, "images")
	assert.NotContains(t, providerBody, "seconds")
	assert.NotContains(t, providerBody, "resolution")
	assert.NotContains(t, providerBody, "size")
	assert.NotContains(t, providerBody, "video_urls")
	assert.NotContains(t, providerBody, "videos")
}
