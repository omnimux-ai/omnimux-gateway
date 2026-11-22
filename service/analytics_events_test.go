package service

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFirstVideoMilestoneHistory(t *testing.T) {
	db := openAnalyticsFixture(t, sqlite.Open(filepath.Join(t.TempDir(), "history.db")))
	firstStart, firstSuccess, laterSuccess := seedVideoMilestones(t, db)

	start, err := earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, false)
	require.NoError(t, err)
	require.NotNil(t, start)
	assert.Equal(t, firstStart.ID, start.ID)

	success, err := earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, true)
	require.NoError(t, err)
	require.NotNil(t, success)
	assert.Equal(t, firstSuccess.ID, success.ID)
	assert.NotEqual(t, laterSuccess.ID, success.ID)

	// The older placeholder, non-video output, untyped URL, and discarded
	// result do not count towards the baseline.
	require.NoError(t, db.Where("id IN ?", []int64{firstSuccess.ID, laterSuccess.ID}).Delete(&model.Task{}).Error)
	success, err = earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, true)
	require.NoError(t, err)
	assert.Nil(t, success)
}

func TestFirstVideoMilestoneRecognizesVideoSubmissions(t *testing.T) {
	db := openAnalyticsFixture(t, sqlite.Open(filepath.Join(t.TempDir(), "operations.db")))
	tests := []struct {
		name, modelName, action, requestPath, plugin string
		wantStart                                    bool
	}{
		{"first frame", "seedance-2-5", "first_frame", "", "", true},
		{"multimodal references", "seedance-2-5", "video_multi_ref", "", "", true},
		{"native video edit", "grok-imagine-video", "video_edit", "/xai/v1/videos/edits", "xai", true},
		{"native video extension", "grok-imagine-video", "video_extension", "/xai/v1/videos/extensions", "xai", true},
		{"native dynamic video submit", "jimeng_vgfm_t2v_l20", "text_to_video", "/jimeng/", "jimeng", true},
		{"plugin responses video", "grok-imagine-video", "video_edit", "/v1/responses", "xai", true},
		{"OpenAI video with new action", "custom-video", "custom_generation", "/v1/videos", "", true},
		{"action without video provenance", "unknown-model", "video_edit", "/v1/responses", "", false},
		{"ambiguous plugin image action", "custom-image", constant.TaskActionGenerate, "/v1/responses", "sora", false},
		{"known image contract", "gpt-image-2.5", constant.TaskActionGenerate, "", "", false},
		{"recreation analysis", "seedance-2-5", "influencer_recreate_analysis", "", "", false},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := model.Task{
				TaskID: "task_operation_" + strconv.Itoa(i), UserId: 200 + i,
				Action: tc.action, Status: model.TaskStatusSubmitted,
				Properties:  model.Properties{OriginModelName: tc.modelName},
				PrivateData: model.TaskPrivateData{Execution: &model.TaskExecutionSnapshot{RequestPath: tc.requestPath}},
			}
			if tc.plugin != "" {
				task.Platform = constant.TaskPlatform(tc.plugin)
				task.PrivateData.Execution.TaskPlugin = &model.TaskPluginSnapshot{Key: tc.plugin, APIVersion: 1}
			}
			require.NoError(t, db.Create(&task).Error)
			result, err := earliestVideoMilestoneTask(t.Context(), db, task.UserId, false)
			require.NoError(t, err)
			if tc.wantStart {
				require.NotNil(t, result)
				assert.Equal(t, task.TaskID, result.TaskID)
			} else {
				assert.Nil(t, result)
			}
		})
	}

	t.Run("historic native submission after plugin disable", func(t *testing.T) {
		registry := jsplugin.NewRegistry()
		source, err := plugins.Source("kling")
		require.NoError(t, err)
		plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "kling"})
		require.NoError(t, err)
		previous := jsplugin.DefaultRegistry
		jsplugin.DefaultRegistry = registry
		t.Cleanup(func() { jsplugin.DefaultRegistry = previous })

		const userID = 299
		task := model.Task{
			UserId: userID, TaskID: "task_native_first", Platform: "kling", Action: constant.TaskActionTextToVideo,
			Status: model.TaskStatusFailure, Properties: model.Properties{OriginModelName: "kling-v1"},
			PrivateData: model.TaskPrivateData{Execution: &model.TaskExecutionSnapshot{
				RequestPath: "/kling/v1/videos/text2video",
				TaskPlugin: &model.TaskPluginSnapshot{
					Key: plugin.Meta.Key, APIVersion: plugin.Meta.APIVersion, Generation: registry.Generation().Number,
				},
			}},
		}
		require.NoError(t, db.Create(&task).Error)
		require.NoError(t, db.Create(&model.Task{
			UserId: userID, TaskID: "task_later_submit", Status: model.TaskStatusSubmitted,
			Action: constant.TaskActionInfluencerMotion,
		}).Error)
		before, err := earliestVideoMilestoneTask(t.Context(), db, userID, false)
		require.NoError(t, err)
		require.NotNil(t, before)
		require.Equal(t, task.TaskID, before.TaskID)

		registry.SetDisabledFactoryKeys([]string{"kling"})
		_, enabled := registry.Get("kling")
		require.False(t, enabled)
		after, err := earliestVideoMilestoneTask(t.Context(), db, userID, false)
		require.NoError(t, err)
		require.NotNil(t, after)
		assert.Equal(t, task.TaskID, after.TaskID)
	})
}

type analyticsArtifactFixture struct {
	TaskPollingAdaptor
	plugin           *jsplugin.LoadedPlugin
	cancelProjection context.CancelFunc
}

func (a analyticsArtifactFixture) ListArtifactsWithContext(ctx context.Context, task *model.Task) ([]types.TaskArtifact, error) {
	var snapshot any
	if err := common.Unmarshal(task.Data, &snapshot); err != nil {
		return nil, err
	}
	var artifacts []types.TaskArtifact
	err := a.plugin.Engine.CallInto(ctx, &artifacts, "listArtifacts", map[string]any{
		"taskId": task.TaskID, "status": string(task.Status), "action": task.Action, "data": snapshot,
	})
	if a.cancelProjection != nil {
		a.cancelProjection()
	}
	return artifacts, err
}

func TestFirstVideoMilestoneUsesStoredArtifacts(t *testing.T) {
	db := openAnalyticsFixture(t, sqlite.Open(filepath.Join(t.TempDir(), "artifacts.db")))
	checkAnalyticsVideoArtifacts(t, db)
}

func checkAnalyticsVideoArtifacts(t *testing.T, db *gorm.DB) {
	t.Helper()
	const userID = 18
	tests := []struct {
		id, action, plugin, resultURL, snapshot string
	}{
		{"empty_sora", "first_frame", "sora", "https://example.com/v1/videos/task_empty_sora/content", `{"status":"completed"}`},
		{"sora_query_only", "first_frame", "sora", "https://example.com/download?format=sample.mp4", `{"status":"completed"}`},
		{"sora_upstream_proxy", "first_frame", "sora", "https://example.com/v1/videos/provider-task/content", `{"status":"completed","url":"https://example.com/v1/videos/provider-task/content"}`},
		{"sora_input_url", "first_frame", "sora", "https://example.com/download/input", `{"status":"completed","metadata":{"video_url":"https://example.com/download/input"}}`},
		{"sora_svg_preview", "first_frame", "sora", "https://example.com/preview.svg", `{"status":"completed","result_url":"https://example.com/preview.svg"}`},
		{"sora_image_snapshot", "first_frame", "sora", "https://example.com/cover.mp4", `{"status":"completed","type":"image","result_url":"https://example.com/cover.mp4"}`},
		{"sora_image_object", "first_frame", "sora", "https://example.com/image.mp4", `{"data":{"status":"completed","object":"image.generation.task","url":"https://example.com/image.mp4"}}`},
		{"moderated", "video_edit", "xai", "https://example.com/v1/videos/task_moderated/content", `{"status":"done","video":{"respect_moderation":false}}`},
		{"audio_with_video_projection", "video_edit", "xai", "https://example.com/v1/videos/task_audio_with_video_projection/content", `{"status":"done","video":{"url":"https://example.com/output.flac","respect_moderation":true}}`},
		{"last_frame_only", "video_multi_ref", "doubao", "", `{"status":"succeeded","content":{"last_frame_url":"https://example.com/frame.png"}}`},
		{"recreate_analysis", "influencer_recreate_analysis", "", "", `{"actions":[{"title":"sample"}]}`},
		{"ready", "video_extension", "xai", "https://example.com/v1/videos/task_ready/content", `{"status":"done","video":{"url":"https://example.com/download/opaque","respect_moderation":true}}`},
	}
	for i, tc := range tests {
		task := model.Task{
			TaskID: "task_" + tc.id, UserId: userID, Platform: constant.TaskPlatform(tc.plugin),
			Action: tc.action, Status: model.TaskStatusSuccess, FinishTime: int64(i + 1), Data: []byte(tc.snapshot),
			PrivateData: model.TaskPrivateData{ResultURL: tc.resultURL, UpstreamTaskID: "provider-task"},
		}
		if tc.plugin != "" {
			task.PrivateData.Execution = &model.TaskExecutionSnapshot{
				RequestPath: "/v1/videos", TaskPlugin: &model.TaskPluginSnapshot{Key: tc.plugin, APIVersion: 1},
			}
		}
		require.NoError(t, db.Create(&task).Error)
	}
	result, err := earliestVideoMilestoneTask(t.Context(), db, userID, true)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "task_ready", result.TaskID)

	const delivery = "https://example.com/download/opaque?signature=sample"
	const binaryDelivery = "https://example.com/output.bin?signature=sample"
	const mislabeledMP4 = "https://example.com/audio.mp4"
	for i, tc := range []struct {
		name, plugin, resultURL, snapshot string
		channelType                       int
		wantVideo                         bool
	}{
		{"sora nested video_url", "sora", delivery, `{"status":"completed","data":{"video_url":[null,{"url":["` + delivery + `"]}]}}`, 0, true},
		{"sora result_url", "sora", delivery, `{"status":"completed","result_url":"` + delivery + `"}`, 0, true},
		{"sora nested url", "sora", delivery, `{"data":{"status":"completed","url":"` + delivery + `"}}`, 0, true},
		{"sora mismatched url", "sora", delivery, `{"status":"completed","url":"https://example.com/download/different"}`, 0, false},
		{"doubao video_url", "doubao", delivery, `{"status":"succeeded","content":{"video_url":"` + delivery + `"}}`, 0, true},
		{"fal video url", "", delivery, `{"video":{"url":"` + delivery + `","content_type":"video/mp4"}}`, constant.ChannelTypeFalAI, true},
		{"fal confirmed MIME overrides flac suffix", "", "https://assets.example.test/output.flac", `{"status":"COMPLETED","video":{"url":"https://assets.example.test/output.flac","content_type":"video/mp4"}}`, constant.ChannelTypeFalAI, true},
		{"fal MIME contradicts mp4 suffix", "", mislabeledMP4, `{"video":{"url":"` + mislabeledMP4 + `","content_type":"audio/flac"}}`, constant.ChannelTypeFalAI, false},
		{"fal mismatched URL cannot override flac suffix", "", "https://assets.example.test/output.flac", `{"video":{"url":"` + delivery + `","content_type":"video/mp4"}}`, constant.ChannelTypeFalAI, false},
		{"fal without MIME cannot override flac suffix", "", "https://assets.example.test/output.flac", `{"video":{"url":"https://assets.example.test/output.flac"}}`, constant.ChannelTypeFalAI, false},
		{"fal without MIME excludes ogg", "", "https://assets.example.test/audio.ogg", `{"video":{"url":"https://assets.example.test/audio.ogg"}}`, constant.ChannelTypeFalAI, false},
		{"fal application octet-stream with mp4 suffix", "", mislabeledMP4, `{"video":{"url":"` + mislabeledMP4 + `","content_type":"application/octet-stream"}}`, constant.ChannelTypeFalAI, true},
		{"fal binary octet-stream with mp4 suffix", "", mislabeledMP4, `{"video":{"url":"` + mislabeledMP4 + `","content_type":"Binary/Octet-Stream; name=sample"}}`, constant.ChannelTypeFalAI, true},
		{"fal octet-stream with no suffix", "", delivery, `{"video":{"url":"` + delivery + `","content_type":"application/octet-stream"}}`, constant.ChannelTypeFalAI, false},
		{"fal octet-stream with unknown suffix", "", binaryDelivery, `{"video":{"url":"` + binaryDelivery + `","content_type":"binary/octet-stream"}}`, constant.ChannelTypeFalAI, false},
		{"fal no MIME with no suffix", "", delivery, `{"video":{"url":"` + delivery + `"}}`, constant.ChannelTypeFalAI, false},
		{"native videos array", "", delivery, `{"data":{"status":"completed","result":{"videos":[{"url":["` + delivery + `"]}]}}}`, constant.ChannelTypeApiMart, true},
		{"native video bin delivery", "", binaryDelivery, `{"data":{"status":"completed","result":{"videos":[{"url":"` + binaryDelivery + `"}]}}}`, constant.ChannelTypeApiMart, true},
		{"native images array", "", binaryDelivery, `{"data":{"status":"completed","result":{"images":[{"url":["` + binaryDelivery + `"]}]}}}`, constant.ChannelTypeApiMart, false},
		{"native images contradict mp4 suffix", "", "https://example.com/cover.mp4", `{"data":{"status":"completed","result":{"images":[{"url":"https://example.com/cover.mp4"}]}}}`, constant.ChannelTypeApiMart, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := model.Task{
				TaskID: "task_opaque_" + strconv.Itoa(i), UserId: 19 + i, Platform: constant.TaskPlatform(tc.plugin),
				Action: "video_multi_ref", Status: model.TaskStatusSuccess, FinishTime: 1, Data: []byte(tc.snapshot),
				PrivateData: model.TaskPrivateData{
					ResultURL: tc.resultURL,
					Execution: &model.TaskExecutionSnapshot{RequestPath: "/v1/videos"},
				},
			}
			if tc.channelType != 0 {
				task.Platform = constant.TaskPlatform(strconv.Itoa(tc.channelType))
				if tc.channelType == constant.ChannelTypeFalAI {
					task.Action = constant.TaskActionTextToVideo
					task.Properties.OriginModelName = "minimax-h3"
				}
			} else {
				task.PrivateData.Execution.TaskPlugin = &model.TaskPluginSnapshot{Key: tc.plugin, APIVersion: 1}
			}
			later := model.Task{
				TaskID: "task_later_" + strconv.Itoa(i), UserId: task.UserId,
				Action: constant.TaskActionInfluencerMotion, Status: model.TaskStatusSuccess, FinishTime: 2,
				PrivateData: model.TaskPrivateData{ResultURL: "https://assets.example.test/later.mp4"},
			}
			require.NoError(t, db.Create(&[]model.Task{task, later}).Error)
			result, err := earliestVideoMilestoneTask(t.Context(), db, task.UserId, true)
			require.NoError(t, err)
			require.NotNil(t, result)
			wantID := later.TaskID
			if tc.wantVideo {
				wantID = task.TaskID
			}
			assert.Equal(t, wantID, result.TaskID)
		})
	}

	t.Run("legacy persisted video delivery", func(t *testing.T) {
		task := model.Task{
			TaskID: "task_legacy_delivery", UserId: 101, Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeApiMart)),
			Action: constant.TaskActionGenerate, Status: model.TaskStatusSuccess, FinishTime: 1, FailReason: delivery,
			Data: []byte(`{"data":{"status":"completed","result":{"videos":[{"video_url":"` + delivery + `"}]}}}`),
		}
		require.NoError(t, db.Create(&task).Error)
		result, err := earliestVideoMilestoneTask(t.Context(), db, task.UserId, true)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, task.TaskID, result.TaskID)
	})

	t.Run("historical delivery with enabled and disabled plugins", func(t *testing.T) {
		checkAnalyticsPluginHistory(t, db)
	})
}

func checkAnalyticsPluginHistory(t *testing.T, db *gorm.DB) {
	t.Helper()
	registry := jsplugin.NewRegistry()
	for _, key := range []string{"google", "hailuo", "kling", "jimeng", "vidu", "alibaba", "autodl"} {
		source, err := plugins.Source(key)
		require.NoError(t, err)
		_, err = registry.RegisterFactory(source, jsplugin.Options{Key: key})
		require.NoError(t, err)
	}
	previous := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(platform constant.TaskPlatform) TaskPollingAdaptor {
		plugin, enabled := registry.Get(string(platform))
		if !enabled {
			return nil
		}
		return analyticsArtifactFixture{plugin: plugin}
	}
	t.Cleanup(func() { GetTaskAdaptorFunc = previous })

	const opaque = "https://assets.example.test/download/opaque?signature=sample"
	const video = "https://assets.example.test/output.mp4"
	const audio = "https://assets.example.test/output.flac"
	tests := []struct {
		name, plugin, resultURL, snapshot string
		wantHistorical                    bool
	}{
		{"google URI behind proxy", "google", "", `{"done":true,"response":{"generateVideoResponse":{"generatedVideos":[{"video":{"uri":"` + opaque + `"}}]}}}`, true},
		{"hailuo historical file", "hailuo", "", `{"status":"Success","file_id":"fixture-file"}`, true},
		{"hailuo wrapped numeric file ID", "hailuo", "", `{"data":{"task_id":"example-task","data":{"status":"Success","file_id":9007199254740993}}}`, true},
		{"hailuo H3 content URL", "hailuo", "", `{"task":{"status":"succeeded","content":{"url":"` + opaque + `"}}}`, true},
		{"kling videos", "kling", opaque, `{"data":{"task_status":"succeed","task_result":{"videos":[{"url":"` + opaque + `"}]}}}`, true},
		{"jimeng wrapped video URL", "jimeng", opaque, `{"data":{"task_id":"example-task","data":{"code":10000,"data":{"status":"done","video_url":"` + opaque + `"}}}}`, true},
		{"vidu creations", "vidu", opaque, `{"state":"success","creations":[{"url":"` + opaque + `"}]}`, true},
		{"alibaba video URL", "alibaba", opaque, `{"output":{"task_status":"SUCCEEDED","video_url":"` + opaque + `"}}`, true},
		{"alibaba nested video behind proxy", "alibaba", "", `{"output":{"task_status":"SUCCEEDED","results":{"video_url":"` + opaque + `"}}}`, true},
		{"autodl stale stored URL", "autodl", "https://assets.example.test/download/older", `{"data":{"status":"SUCCESS","results":[null,{"file_url":"` + video + `"}]}}`, true},
		{"autodl explicitly typed video", "autodl", "", `{"status":"SUCCESS","results":{"url":"` + opaque + `","type":"video","file_type":"mp4"}}`, true},
		{"hailuo no file ID or URL", "hailuo", "", `{"status":"Success","file_id":" "}`, false},
		{"kling placeholder only", "kling", "", `{"data":{"task_status":"succeed","task_result":{"videos":[{}]}}}`, false},
		{"jimeng no video URL", "jimeng", "", `{"data":{"status":"done","image_url":"` + video + `"}}`, false},
		{"vidu audio behind proxy", "vidu", "", `{"state":"success","creations":[{"url":"` + audio + `"}]}`, false},
		{"alibaba image URL with mp4 suffix", "alibaba", video, `{"output":{"task_status":"SUCCEEDED","results":[{"url":"` + video + `"}]}}`, false},
		{"alibaba image in later choice", "alibaba", video, `{"output":{"choices":[{"message":{"content":[{"text":"preview"}]}},{"message":{"content":[{"image":"` + video + `"}]}}]}}`, false},
		{"autodl audio despite stale mp4", "autodl", video, `{"status":"SUCCESS","results":{"output":"` + audio + `"}}`, false},
		{"autodl audio type overrides mp4 suffix", "autodl", video, `{"data":{"status":"completed","results":[{"url":"` + video + `","type":"audio","file_type":"wav"}]}}`, false},
		{"autodl image file type overrides mp4 suffix", "autodl", video, `{"status":"SUCCESS","results":{"url":"` + video + `","file_type":"png"}}`, false},
		{"autodl ogg file type overrides mp4 suffix", "autodl", video, `{"status":"SUCCESS","results":{"url":"` + video + `","file_type":"ogg"}}`, false},
		{"autodl query suffix only", "autodl", "", `{"data":{"status":"SUCCESS","results":["https://assets.example.test/download?format=sample.mp4"]}}`, false},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := "task_historical_" + strconv.Itoa(i)
			if tc.name == "hailuo historical file" {
				id = "task_historical_file"
			}
			task := model.Task{
				UserId: 300 + i, TaskID: id, Platform: constant.TaskPlatform(tc.plugin),
				Action: constant.TaskActionTextToVideo, Status: model.TaskStatusSuccess, FinishTime: 1, Data: []byte(tc.snapshot),
				PrivateData: model.TaskPrivateData{
					ResultURL: tc.resultURL,
					Execution: &model.TaskExecutionSnapshot{
						RequestPath: "/v1/videos", TaskPlugin: &model.TaskPluginSnapshot{Key: tc.plugin, APIVersion: 1},
					},
				},
			}
			if task.PrivateData.ResultURL == "" {
				task.PrivateData.ResultURL = "https://gateway.example.test/v1/videos/" + task.TaskID + "/content"
			}
			require.NoError(t, db.Create(&task).Error)
			newer := model.Task{
				UserId: task.UserId, TaskID: "task_newer_video_" + strconv.Itoa(i), Action: constant.TaskActionInfluencerMotion,
				Status: model.TaskStatusSuccess, FinishTime: 2,
				PrivateData: model.TaskPrivateData{ResultURL: "https://assets.example.test/newer.mp4"},
			}
			require.NoError(t, db.Create(&newer).Error)
			wantID := newer.TaskID
			if tc.wantHistorical {
				wantID = task.TaskID
			}
			_, enabled := registry.Get(tc.plugin)
			require.True(t, enabled)
			before, err := earliestVideoMilestoneTask(t.Context(), db, task.UserId, true)
			require.NoError(t, err)
			require.NotNil(t, before)
			assert.Equal(t, wantID, before.TaskID, "with active plugin")

			registry.SetDisabledFactoryKeys([]string{tc.plugin})
			t.Cleanup(func() { registry.SetDisabledFactoryKeys(nil) })
			_, enabled = registry.Get(tc.plugin)
			require.False(t, enabled)
			after, err := earliestVideoMilestoneTask(t.Context(), db, task.UserId, true)
			require.NoError(t, err)
			require.NotNil(t, after)
			assert.Equal(t, wantID, after.TaskID, "with disabled plugin")
			assert.Equal(t, before.TaskID, after.TaskID, "plugin changes must preserve the earliest video")
		})
	}

	t.Run("custom plugin projection", func(t *testing.T) {
		checkAnalyticsCustomPluginArtifacts(t, db, registry)
	})
}

func checkAnalyticsCustomPluginArtifacts(t *testing.T, db *gorm.DB, registry *jsplugin.Registry) {
	t.Helper()
	const pluginKey = "review-video"
	plugin, err := registry.Register(`
export const meta = {
  apiVersion: 1, key: "review-video", name: "Review Video", version: "1.0.0",
  author: {name: "Test"}, models: ["review-video-model"], fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
export function listArtifacts(task) {
  if (task.data.projection_error) throw new Error("fixture snapshot is unsupported");
  if (task.data.artifacts) return task.data.artifacts;
  if (!task.data.asset_id) return [];
  const artifact = {key: "video", type: "video"};
  if (task.data.mime !== null) artifact.mimeType = task.data.mime ?? "video/mp4";
  return [artifact];
}
export function buildContentRequest() { throw new Error("fixture has no media server"); }
`, jsplugin.Options{Key: pluginKey})
	require.NoError(t, err)

	const mp4 = "https://assets.example.test/output.mp4"
	const poster = "https://assets.example.test/thumbnail.jpg"
	const posterOnly = `{"status":"SUCCESS","artifacts":[{"key":"poster","type":"image","mimeType":"image/jpeg"}]}`
	const projectionError = `{"status":"SUCCESS","projection_error":true}`
	const errorWithMP4 = "projection error with mp4 URL"
	tests := []struct {
		name, resultURL, snapshot string
		withPlugin, withoutPlugin bool
	}{
		{"video MIME behind a proxy", "", `{"status":"SUCCESS","asset_id":"fixture-file"}`, true, false},
		{"video type without MIME behind a proxy", "", `{"status":"SUCCESS","asset_id":"fixture-file","mime":null}`, true, false},
		{"video artifact alongside a thumbnail URL", poster, `{"status":"SUCCESS","asset_id":"fixture-file"}`, true, false},
		{"audio MIME despite mp4 URL", mp4, `{"status":"SUCCESS","asset_id":"fixture-file","mime":"audio/flac"}`, false, true},
		{"empty projection behind a proxy", "", `{"status":"SUCCESS"}`, false, false},
		{"poster projection with mp4 URL", mp4, posterOnly, true, true},
		{"poster projection behind a proxy", "", posterOnly, false, false},
		{"poster projection with image URL", poster, posterOnly, false, false},
		{errorWithMP4, mp4, projectionError, true, true},
		{"projection error behind a proxy", "", projectionError, false, false},
		{"first video artifact MIME takes precedence", mp4, `{"status":"SUCCESS","artifacts":[{"key":"poster","type":"image","mimeType":"image/jpeg"},{"key":"primary","type":"video","mimeType":"audio/flac"},{"key":"alternate","type":"video","mimeType":"video/mp4"}]}`, false, true},
	}
	originals := make([]model.Task, len(tests))
	for i, tc := range tests {
		original := model.Task{
			TaskID: "task_custom_original_" + strconv.Itoa(i), UserId: 400 + i,
			Platform: constant.TaskPlatform(pluginKey), Action: constant.TaskActionTextToVideo,
			Status: model.TaskStatusSuccess, FinishTime: 1, Data: []byte(tc.snapshot),
			PrivateData: model.TaskPrivateData{
				ResultURL: tc.resultURL,
				Execution: &model.TaskExecutionSnapshot{
					RequestPath: "/v1/videos",
					TaskPlugin:  &model.TaskPluginSnapshot{Key: pluginKey, APIVersion: 1, Version: plugin.Meta.Version},
				},
			},
		}
		if original.PrivateData.ResultURL == "" {
			original.PrivateData.ResultURL = "https://gateway.example.test/v1/videos/" + original.TaskID + "/content"
		}
		later := model.Task{
			TaskID: "task_custom_later_" + strconv.Itoa(i), UserId: original.UserId,
			Action: constant.TaskActionInfluencerMotion, Status: model.TaskStatusSuccess, FinishTime: 2,
			PrivateData: model.TaskPrivateData{ResultURL: mp4},
		}
		require.NoError(t, db.Create(&[]model.Task{original, later}).Error)
		originals[i] = original
	}
	for _, state := range []struct {
		name      string
		installed bool
	}{{"installed", true}, {"uninstalled", false}} {
		t.Run(state.name, func(t *testing.T) {
			if !state.installed {
				require.NoError(t, registry.Unregister(pluginKey))
			}
			_, installed := registry.Get(pluginKey)
			require.Equal(t, state.installed, installed)
			for i, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					original := originals[i]
					if state.installed && tc.name == errorWithMP4 {
						t.Run("context canceled during projection", func(t *testing.T) {
							ctx, cancel := context.WithCancel(t.Context())
							defer cancel()
							previous := GetTaskAdaptorFunc
							GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor {
								return analyticsArtifactFixture{plugin: plugin, cancelProjection: cancel}
							}
							t.Cleanup(func() { GetTaskAdaptorFunc = previous })
							result, err := earliestVideoMilestoneTask(ctx, db, original.UserId, true)
							require.ErrorIs(t, err, context.Canceled)
							assert.Nil(t, result)
						})
					}
					result, err := earliestVideoMilestoneTask(t.Context(), db, original.UserId, true)
					require.NoError(t, err)
					require.NotNil(t, result)
					wantID := "task_custom_later_" + strconv.Itoa(i)
					if (state.installed && tc.withPlugin) || (!state.installed && tc.withoutPlugin) {
						wantID = original.TaskID
					}
					assert.Equal(t, wantID, result.TaskID)
				})
			}
		})
	}
}

// External verification uses task-owned databases plus two independent Redis
// clients. Ordinary go test still exercises metadata and history on SQLite.
func TestFirstVideoMilestoneDatabaseMatrix(t *testing.T) {
	redisURL := os.Getenv("E03_TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set E03_TEST_REDIS_URL and task-owned database DSNs to run the integration matrix")
	}
	redisOpts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	requireAnalyticsFixtureAddress(t, redisOpts.Addr)

	mysqlDSN := os.Getenv("E03_TEST_MYSQL_DSN")
	mysqlConfig, err := mysqldriver.ParseDSN(mysqlDSN)
	require.NoError(t, err)
	require.Equal(t, "tcp", mysqlConfig.Net)
	require.True(t, strings.HasPrefix(mysqlConfig.DBName, "__test_e03_"))
	requireAnalyticsFixtureAddress(t, mysqlConfig.Addr)

	postgresDSN := os.Getenv("E03_TEST_POSTGRES_DSN")
	postgresURL, err := url.Parse(postgresDSN)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(postgresURL.Path, "/__test_e03_"))
	requireAnalyticsFixtureAddress(t, postgresURL.Host)

	for _, fixture := range []struct {
		name    string
		dialect gorm.Dialector
	}{
		{"sqlite", sqlite.Open(filepath.Join(t.TempDir(), "matrix.db"))},
		{"mysql", mysql.Open(mysqlDSN)},
		{"postgres", postgres.Open(postgresDSN)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			db := openAnalyticsFixture(t, fixture.dialect)
			if !t.Run("artifact evidence", func(t *testing.T) { checkAnalyticsVideoArtifacts(t, db) }) {
				return
			}
			firstStart, firstSuccess, laterSuccess := seedVideoMilestones(t, db)
			workerA, workerB := redis.NewClient(redisOpts), redis.NewClient(redisOpts)
			t.Cleanup(func() { _ = workerA.Close() })
			t.Cleanup(func() { _ = workerB.Close() })
			site := "fixture:" + filepath.Base(t.TempDir()) + ":" + fixture.name
			// An isolated Redis process can exercise AOF replay of the actual claims.
			if os.Getenv("E03_TEST_KEEP_REDIS_CLAIMS") != "1" {
				t.Cleanup(func() {
					user := strconv.Itoa(firstStart.UserId)
					for _, name := range []string{analytics.EventFirstVideoStart, analytics.EventFirstVideoSuccess} {
						require.NoError(t, workerA.Del(context.Background(), "analytics:e03:"+site+":"+name+":"+user).Err())
					}
				})
			}

			start, reason := firstVideoMilestone(t.Context(), db, workerA, site, firstStart.UserId, firstStart.ID, analytics.EventFirstVideoStart, "api")
			assert.Empty(t, reason)
			assert.Equal(t, analytics.EventFirstVideoStart, start.Name)
			assert.Equal(t, firstStart.TaskID, start.Data["task_id"])

			later, reason := firstVideoMilestone(t.Context(), db, workerB, site, laterSuccess.UserId, laterSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.Empty(t, reason)
			assert.Empty(t, later.Name, "a later task must respect the existing first success")

			type outcome struct {
				event  analytics.Event
				reason string
			}
			results := make(chan outcome, 2)
			ready := make(chan struct{})
			var concurrent sync.WaitGroup
			for _, worker := range []*redis.Client{workerA, workerB} {
				concurrent.Go(func() {
					<-ready
					event, reason := firstVideoMilestone(t.Context(), db, worker, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
					results <- outcome{event, reason}
				})
			}
			close(ready)
			concurrent.Wait()
			close(results)
			count := 0
			for result := range results {
				assert.Empty(t, result.reason)
				if result.event.Name != "" {
					count++
					assert.Equal(t, firstSuccess.TaskID, result.event.Data["task_id"])
					assert.Equal(t, int64(5250), result.event.Data["duration_ms"])
					assert.Equal(t, int64(123_456), result.event.Data["file_size"])
				}
			}
			assert.Equal(t, 1, count)

			restarted := redis.NewClient(redisOpts)
			t.Cleanup(func() { _ = restarted.Close() })
			replay, reason := firstVideoMilestone(t.Context(), db, restarted, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.Empty(t, reason)
			assert.Empty(t, replay.Name)

			disabled, reason := firstVideoMilestone(t.Context(), db, nil, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.NotEmpty(t, reason)
			assert.Empty(t, disabled.Name)
		})
	}
}

func openAnalyticsFixture(t *testing.T, dialect gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialect, &gorm.Config{})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	return db
}

func requireAnalyticsFixtureAddress(t *testing.T, addr string) {
	t.Helper()
	host, textPort, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	port, err := strconv.Atoi(textPort)
	require.NoError(t, err)
	require.GreaterOrEqual(t, port, 49152)
}

func seedVideoMilestones(t *testing.T, db *gorm.DB) (firstStart, firstSuccess, laterSuccess *model.Task) {
	t.Helper()
	const userID = 92_101
	t.Cleanup(func() { require.NoError(t, db.Where("user_id = ?", userID).Delete(&model.Task{}).Error) })

	// More than one page of ambiguous legacy actions shares a finish time.
	images := make([]model.Task, 33)
	for i := range images {
		images[i] = model.Task{UserId: userID, TaskID: "task_legacy_image_" + strconv.Itoa(i),
			Action: constant.TaskActionGenerate, Status: model.TaskStatusSuccess, FinishTime: 10,
			Properties:  model.Properties{OriginModelName: "gpt-image-2.5"},
			PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/thumbnail.jpg"}}
	}
	require.NoError(t, db.Create(&images).Error)
	video := model.Properties{OriginModelName: "seedance-2-5"}
	firstStart = &model.Task{UserId: userID, TaskID: "task_failed_video", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusFailure, FinishTime: 5, Properties: video}
	placeholder := &model.Task{UserId: userID, TaskID: "task_placeholder", Action: constant.TaskActionImageToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/v1/videos/task_placeholder/content"}}
	discarded := &model.Task{UserId: userID, TaskID: "task_discarded", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.mp4", ResultDiscarded: true}}
	nonVideo := &model.Task{UserId: userID, TaskID: "task_audio_output", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.flac"}}
	untyped := &model.Task{UserId: userID, TaskID: "task_untyped_output", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/download?format=mp4"}}
	metadata, err := common.Marshal(dto.InfluencerVideoMetadata{VideoDuration: 5.25, Width: 480, Height: 854, SizeBytes: 123_456})
	require.NoError(t, err)
	firstSuccess = &model.Task{UserId: userID, TaskID: "task_workbench_video", Action: constant.TaskActionInfluencerMotion,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video, OutputVideoMetadata: metadata,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/v1/videos/task_workbench_video/content"}}
	laterSuccess = &model.Task{UserId: userID, TaskID: "task_later_video", Action: constant.TaskActionReferenceToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.webm"}}
	for _, task := range []*model.Task{firstStart, placeholder, discarded, nonVideo, untyped, firstSuccess, laterSuccess} {
		require.NoError(t, db.Create(task).Error)
	}
	return firstStart, firstSuccess, laterSuccess
}
