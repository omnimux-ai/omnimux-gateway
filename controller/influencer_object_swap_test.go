package controller

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func objectSwapClaimedRequest(t *testing.T, db *gorm.DB, user model.User, scanner *model.SystemTask) (*model.Task, map[string]any) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.File{}))
	driver := generationMaterialStore(t)
	out := map[string]any{
		"model": "queue-correction", "prompt": "old generic framing",
		"images":   []any{"https://8.8.4.4/subject.png", "https://8.8.4.4/detail.png", "https://8.8.4.4/product.png"},
		"duration": 5, "aspect_ratio": "9:16", "resolution": "480p", "generate_audio": false,
	}
	for _, field := range []string{"videos", "audios"} {
		key := "influencer-reference/frozen/" + field + ".bin"
		url, err := driver.Upload(context.Background(), key, []byte("accepted-original-"+field), "application/octet-stream")
		require.NoError(t, err)
		require.NoError(t, model.CreateFile(&model.File{FileId: "swap_" + field, UserId: user.Id, StorageKey: key, FileUrl: url, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}))
		out[field] = []any{url}
	}
	meta := map[string]any{
		"kind": "motion", "mode": "replace", "duration_sec": 5,
		"prompt":                    "只换靠右的容器，保持左侧倒水容器和人物不变。",
		"subject_image_tags":        []string{"主图", "补充细节", "商品"},
		"verified_subject_identity": "已核实的角色身份，仅在采用人物身份时使用",
		"prepared_video_sha256":     "accepted-source-digest",
	}
	generationMaterialQueue(t, user, out, meta, true)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	return claimed[0], out
}

func objectSwapStubPlanner(t *testing.T, planner func(context.Context, service.InfluencerObjectSwapPlanInput) (service.InfluencerObjectSwapPlan, error)) {
	t.Helper()
	previous := influencerObjectSwapPlanner
	influencerObjectSwapPlanner = planner
	t.Cleanup(func() { influencerObjectSwapPlanner = previous })
}

func TestObjectSwapPreparationUsesFrozenJointInputsAndDurablyReusesPrompt(t *testing.T) {
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("preparation must not submit a video")
	}))
	task, out := objectSwapClaimedRequest(t, db, user, scanner)
	originalBody, err := common.Marshal(out)
	require.NoError(t, err)
	originalInput := task.Properties.Input
	var calls int
	var seen service.InfluencerObjectSwapPlanInput
	const finalPrompt = "编辑@视频1，以@图片3替换画面右侧接水容器，保持倒水容器、人物、动作、镜头与原音。"
	objectSwapStubPlanner(t, func(_ context.Context, in service.InfluencerObjectSwapPlanInput) (service.InfluencerObjectSwapPlan, error) {
		calls++
		seen = in
		return service.InfluencerObjectSwapPlan{Prompt: finalPrompt, Audit: []byte(`{"status":"ready","edits":[{"images":[3],"target":"右侧接水容器"}]}`), Trace: service.InfluencerObjectSwapPlanTrace{Outcome: "completed", RequestID: "joint-request", Usage: []byte(`{"total_tokens":321}`)}}, nil
	})
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), task, out, scanner, "correction"))
	assert.Equal(t, out["videos"].([]any)[0], seen.VideoURL)
	assert.Equal(t, []string{"https://8.8.4.4/subject.png", "https://8.8.4.4/detail.png", "https://8.8.4.4/product.png"}, seen.ImageURLs)
	assert.Equal(t, []string{"主图", "补充细节", "商品"}, seen.ImageTags)
	assert.Equal(t, "只换靠右的容器，保持左侧倒水容器和人物不变。", seen.UserPrompt)
	assert.Equal(t, "已核实的角色身份，仅在采用人物身份时使用", seen.VerifiedIdentity)
	assert.Equal(t, 5, seen.DurationSec)
	assert.Equal(t, finalPrompt, out["prompt"])
	var expected map[string]any
	require.NoError(t, common.Unmarshal(originalBody, &expected))
	expected["prompt"] = finalPrompt
	actualBody, err := common.Marshal(out)
	require.NoError(t, err)
	expectedBody, err := common.Marshal(expected)
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedBody), string(actualBody), "only the final prompt may change; source media, audio and output settings stay accepted")
	assert.Equal(t, originalInput, task.Properties.Input)

	// A different in-memory task instance must reuse storage, including after a
	// worker resumes with the original pre-planning request body.
	var recovered model.Task
	require.NoError(t, db.First(&recovered, task.ID).Error)
	var resumed map[string]any
	require.NoError(t, common.Unmarshal(originalBody, &resumed))
	require.NoError(t, prepareInfluencerVideoPrompt(context.Background(), &recovered, resumed, scanner, "correction"))
	assert.Equal(t, 1, calls, "a completed paid understanding request must not be repeated")
	assert.Equal(t, finalPrompt, resumed["prompt"])
	require.NotNil(t, recovered.PrivateData.ObjectSwapPreparation)
	assert.Equal(t, model.GenerationObjectSwapReady, recovered.PrivateData.ObjectSwapPreparation.State)
	assert.Equal(t, finalPrompt, recovered.PrivateData.ObjectSwapPreparation.Prompt)
}

func TestObjectSwapPreparationFailurePreservesEvidenceAndNeverFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name, outcome, state string
		usage                []byte
	}{
		{name: "unconfirmed provider result", outcome: "unknown", state: model.GenerationObjectSwapUnknown},
		{name: "invalid completed response", outcome: "completed", state: model.GenerationObjectSwapFailed, usage: []byte(`{"total_tokens":99}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("failed preparation must not submit a video")
			}))
			task, out := objectSwapClaimedRequest(t, db, user, scanner)
			var calls int
			objectSwapStubPlanner(t, func(context.Context, service.InfluencerObjectSwapPlanInput) (service.InfluencerObjectSwapPlan, error) {
				calls++
				return service.InfluencerObjectSwapPlan{Trace: service.InfluencerObjectSwapPlanTrace{Outcome: tc.outcome, RequestID: "failed-joint-request", Usage: tc.usage}}, errors.New("provider response cannot establish a valid replacement")
			})
			err := prepareInfluencerVideoPrompt(context.Background(), task, out, scanner, "correction")
			require.Error(t, err)
			assert.Equal(t, "old generic framing", out["prompt"], "the untouched draft is not permission to continue generation")
			var stored model.Task
			require.NoError(t, db.First(&stored, task.ID).Error)
			require.NotNil(t, stored.PrivateData.ObjectSwapPreparation)
			assert.Equal(t, tc.state, stored.PrivateData.ObjectSwapPreparation.State)
			assert.Empty(t, stored.PrivateData.ObjectSwapPreparation.Prompt)
			var evidence service.InfluencerObjectSwapPlan
			require.NoError(t, common.Unmarshal(stored.PrivateData.ObjectSwapPreparation.Details, &evidence))
			assert.Equal(t, tc.outcome, evidence.Trace.Outcome)
			assert.Equal(t, "failed-joint-request", evidence.Trace.RequestID)
			if tc.usage == nil {
				assert.Empty(t, evidence.Trace.Usage, "missing usage is unknown; do not invent a zero charge")
			} else {
				assert.JSONEq(t, string(tc.usage), string(evidence.Trace.Usage))
			}
			require.Error(t, prepareInfluencerVideoPrompt(context.Background(), &stored, out, scanner, "correction"))
			assert.Equal(t, 1, calls, "an unconfirmed or failed call must not be silently repeated")
		})
	}
}

func TestObjectSwapDispatchFailureStopsBeforeVideoAndNativeCharge(t *testing.T) {
	var videoCalls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		videoCalls.Add(1)
		_, _ = w.Write([]byte(`{"id":"must-not-be-submitted"}`))
	}))
	precharged := generationMaterialObservePrecharge(t, db)
	task, _ := objectSwapClaimedRequest(t, db, user, scanner)
	var understandingCalls int
	objectSwapStubPlanner(t, func(context.Context, service.InfluencerObjectSwapPlanInput) (service.InfluencerObjectSwapPlan, error) {
		understandingCalls++
		return service.InfluencerObjectSwapPlan{Trace: service.InfluencerObjectSwapPlanTrace{Outcome: "unknown"}}, errors.New("understanding result is unconfirmed")
	})
	dispatchInfluencerGeneration(context.Background(), task, scanner, "correction")
	assert.Equal(t, 1, understandingCalls, "the test must reach understanding after real routing and frozen-media checks")
	assert.Zero(t, videoCalls.Load(), "failure must never dispatch the old generic prompt")
	assert.False(t, precharged.Load(), "native video funds must never be reserved")
	var stored model.Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Contains(t, stored.FailReason, "素材理解")
	assert.Zero(t, stored.Quota)
	assert.Nil(t, stored.PrivateData.BillingContext)
	require.NotNil(t, stored.PrivateData.ObjectSwapPreparation)
	assert.Equal(t, model.GenerationObjectSwapUnknown, stored.PrivateData.ObjectSwapPreparation.State)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
	var consumeCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeConsume).Count(&consumeCount).Error)
	assert.Zero(t, consumeCount)
	// Re-entering the queue worker after its terminal rejection cannot repeat
	// either model request.
	dispatchInfluencerGeneration(context.Background(), &stored, scanner, "correction")
	assert.Equal(t, 1, understandingCalls)
	assert.Zero(t, videoCalls.Load())
}

func TestObjectSwapDispatchSendsOnlyPlannedPromptAndKeepsAuditAfterNativeHandoff(t *testing.T) {
	var videoCalls atomic.Int32
	var submitted map[string]any
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		videoCalls.Add(1)
		require.NoError(t, common.DecodeJson(r.Body, &submitted))
		_, _ = w.Write([]byte(`{"id":"accepted-joint-video"}`))
	}))
	task, out := objectSwapClaimedRequest(t, db, user, scanner)
	originalInput := task.Properties.Input
	const finalPrompt = "编辑@视频1，根据@图片3替换右侧接水容器，保留其他内容与原音。"
	var understandingCalls int
	objectSwapStubPlanner(t, func(context.Context, service.InfluencerObjectSwapPlanInput) (service.InfluencerObjectSwapPlan, error) {
		understandingCalls++
		return service.InfluencerObjectSwapPlan{Prompt: finalPrompt, Audit: []byte(`{"status":"ready"}`), Trace: service.InfluencerObjectSwapPlanTrace{Outcome: "completed", RequestID: "accepted-joint-understanding", Usage: []byte(`{"total_tokens":456}`)}}, nil
	})
	dispatchInfluencerGeneration(context.Background(), task, scanner, "correction")
	assert.Equal(t, 1, understandingCalls)
	assert.Equal(t, int32(1), videoCalls.Load())
	out["prompt"] = finalPrompt
	expected, err := common.Marshal(out)
	require.NoError(t, err)
	actual, err := common.Marshal(submitted)
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), string(actual), "native plugin submission must retain every accepted setting and media reference")
	var stored model.Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Equal(t, "accepted-joint-video", stored.PrivateData.UpstreamTaskID)
	assert.Equal(t, originalInput, stored.Properties.Input)
	assert.Nil(t, stored.PrivateData.GenerationQueue)
	require.NotNil(t, stored.PrivateData.ObjectSwapPreparation)
	assert.Equal(t, model.GenerationObjectSwapReady, stored.PrivateData.ObjectSwapPreparation.State)
	assert.Equal(t, finalPrompt, stored.PrivateData.ObjectSwapPreparation.Prompt)
	var evidence service.InfluencerObjectSwapPlan
	require.NoError(t, common.Unmarshal(stored.PrivateData.ObjectSwapPreparation.Details, &evidence))
	assert.Equal(t, "accepted-joint-understanding", evidence.Trace.RequestID)
	assert.JSONEq(t, `{"total_tokens":456}`, string(evidence.Trace.Usage))
	assert.Equal(t, 5000, stored.Quota, "the native wallet continues to own the video charge")
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 95000, account.Quota)
}
