package controller

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

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
		Resolution:       "480p",
	}
	out, meta, prepared, ok := prepareInfluencerMotionRequest(c, req, true)
	require.True(t, ok)
	require.NotNil(t, prepared)
	assert.NotContains(t, out, "videos")
	assert.NotContains(t, out, "audios")
	assert.Equal(t, "seedance-2-5", out["model"])
	assert.Equal(t, "seedance-2-5-480p", out["group"])
	assert.Equal(t, "4", out["seconds"])
	assert.Equal(t, "9:16", out["aspect_ratio"])
	assert.Equal(t, "480p", out["resolution"])
	assert.Equal(t, req.SubjectImageURLs, out["images"])
	assert.NotContains(t, out, "image_urls")
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
				Resolution:       "480p",
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

// A recreate task already carries its compiled prompt, so the queue dispatch must
// not run any planner turn before sending.
func TestPrepareInfluencerVideoPromptRecreateSkipsPlanning(t *testing.T) {
	input, err := common.Marshal(map[string]any{"mode": "recreate", "prompt": validRecreatePrompt(), "duration_sec": 4})
	require.NoError(t, err)
	task := &model.Task{Properties: model.Properties{Input: string(input)}}
	out := map[string]any{"prompt": "planned prompt stays"}
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, out, nil, ""))
	assert.Equal(t, "planned prompt stays", out["prompt"])
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
		"too many":       {"video_url": "https://cdn.example.com/a.mp4", "image_urls": tooManyURLs(31)},
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
			Resolution:       "480p",
		}, true)
		require.True(t, ok, "seconds=%d", seconds)
		assert.Equal(t, strconv.Itoa(seconds), out["seconds"])
	}
	for _, seconds := range []int{0, 3, 31} {
		c := recreateTestContext()
		_, _, _, ok := prepareInfluencerMotionRequest(c, influencerMotionRequest{
			Mode:             "recreate",
			Prompt:           validRecreatePrompt(),
			SubjectImageURLs: []string{"https://cdn.example.com/a.jpg"},
			DurationSec:      seconds,
			Resolution:       "480p",
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
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2-5":0.8}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"seedance-2-5-480p":0.524,"seedance-2-5-720p":0.7206}`))

	c := recreateTestContext()
	c.Set("group", "default")
	pricing := influencerRecreatePricing(c, 11)
	require.NotNil(t, pricing)
	// 0.8 USD/s x 11 s x 0.524 / 0.7206 x 10 credits/USD.
	assert.InDelta(t, 46.11, pricing["480p"], 0.05)
	assert.InDelta(t, 63.41, pricing["720p"], 0.1)
	assert.Greater(t, pricing["720p"], pricing["480p"])
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
		"resolution": "480p",
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
		Resolution: "480p",
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
