package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/apimart"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	motionTestImage       = "https://files.omnimux.ai/images/subject.png"
	motionTestVideo       = "https://files.omnimux.ai/influencer/motion/741a2220a4c89289.mp4"
	motionTestFrozenVideo = "https://files.omnimux.ai/influencer-reference/frozen/server-owned.mp4"
)

// motionPrepared mounts the prepare step in front of a probe that records what
// distribution and the submit handler would receive.
type motionPrepared struct {
	reached bool
	body    map[string]any
	meta    map[string]any
	group   string
}

func runMotionPrepare(t *testing.T, payload any, headers ...string) (*httptest.ResponseRecorder, *motionPrepared) {
	t.Helper()
	previous := service.InfluencerBrowserVideoPreparer
	service.InfluencerBrowserVideoPreparer = func(_ context.Context, _ string, _ int, metadata service.InfluencerVideoMetadata, _ func(string) bool) (service.PreparedInfluencerVideo, error) {
		return service.PreparedInfluencerVideo{Metadata: metadata, URL: motionTestFrozenVideo}, nil
	}
	defer func() { service.InfluencerBrowserVideoPreparer = previous }()
	if body, ok := payload.(map[string]any); ok {
		metadata, exists := body["video_metadata"].(service.InfluencerVideoMetadata)
		if exists {
			if ratio, ok := body["aspect_ratio"].(string); ok {
				switch strings.TrimSpace(ratio) {
				case "16:9":
					metadata.Width, metadata.Height = 1280, 720
				case "1:1":
					metadata.Width, metadata.Height = 800, 800
				case "4:3":
					metadata.Width, metadata.Height = 960, 720
				case "3:4":
					metadata.Width, metadata.Height = 960, 1280
				case "21:9":
					metadata.Width, metadata.Height = 1680, 720
				}
			}
			if duration, ok := body["_probe_duration"].(float64); ok {
				metadata.Duration, metadata.VideoDuration = duration, duration
			}
			if width, ok := body["_probe_width"].(int); ok {
				metadata.Width = width
			}
			if height, ok := body["_probe_height"].(int); ok {
				metadata.Height = height
			}
			body["video_metadata"] = metadata
		}
	}
	return runMotionPrepareWithInspector(t, payload, headers...)
}

func runMotionPrepareWithInspector(t *testing.T, payload any, headers ...string) (*httptest.ResponseRecorder, *motionPrepared) {
	t.Helper()
	settings := system_setting.GetFetchSetting()
	previousSettings := *settings
	settings.ApplyIPFilterForDomain = false
	t.Cleanup(func() { *settings = previousSettings })
	gin.SetMode(gin.TestMode)
	probe := &motionPrepared{}
	engine := gin.New()
	engine.POST("/pg/influencer/motion", func(c *gin.Context) { c.Set("id", 41); c.Next() }, PrepareInfluencerMotion, func(c *gin.Context) {
		probe.reached = true
		probe.group = c.GetHeader("X-Omnimux-Group")
		assert.Empty(t, c.GetHeader("New-Api-Group"))
		raw, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &probe.body))
		metaValue, _ := c.Get(ctxInfluencerMotionMeta)
		probe.meta, _ = metaValue.(map[string]any)
		c.Status(http.StatusNoContent)
	})
	var raw []byte
	switch v := payload.(type) {
	case string:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(v)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(http.MethodPost, "/pg/influencer/motion", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec, probe
}

func validMotionRequest() map[string]any {
	return map[string]any{
		"mode":              "motion",
		"prompt":            "smile at the end",
		"subject_image_url": motionTestImage,
		"video_url":         motionTestVideo,
		"duration_sec":      5,
		"resolution":        "1080p",
		"video_metadata":    service.InfluencerVideoMetadata{Duration: 5, VideoDuration: 5, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: 1234},
	}
}

// A rejected form must stop before distribution, which is where a channel is
// picked and billing begins.
func TestPrepareInfluencerMotionRejectsInvalidForms(t *testing.T) {
	cases := map[string]func(map[string]any){
		"unknown mode":    func(r map[string]any) { r["mode"] = "dance" },
		"480p":            func(r map[string]any) { r["resolution"] = "480p" },
		"720p":            func(r map[string]any) { r["resolution"] = "720p" },
		"too short":       func(r map[string]any) { r["duration_sec"] = 3 },
		"too long":        func(r map[string]any) { r["duration_sec"] = 31 },
		"prompt too long": func(r map[string]any) { r["prompt"] = strings.Repeat("字", 4001) },
		"missing image":   func(r map[string]any) { delete(r, "subject_image_url") },
		"http image":      func(r map[string]any) { r["subject_image_url"] = "http://files.omnimux.ai/a.png" },
		"data url image":  func(r map[string]any) { r["subject_image_url"] = "data:image/png;base64,AAAA" },
		"too many images": func(r map[string]any) {
			urls := make([]string, 11)
			for i := range urls {
				urls[i] = motionTestImage
			}
			r["subject_image_urls"] = urls
		},
		"http in image list": func(r map[string]any) {
			r["subject_image_urls"] = []string{motionTestImage, "http://bad.png"}
		},
		"blob video":         func(r map[string]any) { r["video_url"] = "blob:https://omnimux.ai/123" },
		"relative video":     func(r map[string]any) { r["video_url"] = "/api/v1/files/download/1" },
		"missing video":      func(r map[string]any) { delete(r, "video_url") },
		"missing resolution": func(r map[string]any) { delete(r, "resolution") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validMotionRequest()
			mutate(req)
			rec, probe := runMotionPrepare(t, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.False(t, probe.reached)
		})
	}

	rec, probe := runMotionPrepare(t, "{not json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, probe.reached)
}

// The resolution decides the pinned tier group; the caller cannot pick another.
func TestPrepareInfluencerMotionRewritesToVideoSubmitBody(t *testing.T) {
	req := validMotionRequest()
	req["resolution"] = "1080p"
	req["group"] = "other-group"
	req["model"] = "something-else"
	rec, probe := runMotionPrepare(t, req, "X-Omnimux-Group", "other-group", "New-Api-Group", "other-group")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.True(t, probe.reached)

	assert.Equal(t, "magi-2", probe.body["model"])
	assert.Equal(t, "default", probe.body["group"])
	assert.Equal(t, "default", probe.group, "distribution reads the group from this header")
	assert.Equal(t, float64(5), probe.body["duration"])
	assert.Equal(t, "9:16", probe.body["aspect_ratio"])
	assert.Equal(t, "1080p", probe.body["resolution"])
	assert.Equal(t, true, probe.body["generate_audio"])
	assert.Equal(t, []any{motionTestImage}, probe.body["image_urls"])
	assert.Equal(t, []any{motionTestFrozenVideo}, probe.body["video_urls"])
	assert.NotContains(t, probe.body, "seconds")
	assert.NotContains(t, probe.body, "images")
	assert.NotContains(t, probe.body, "videos")
	assert.NotContains(t, probe.body, "audios")

	assert.Equal(t, "motion", probe.meta["kind"])
	assert.Equal(t, "smile at the end", probe.meta["prompt"])
	assert.Equal(t, "1080p", probe.meta["resolution"])
}

// Browser requests must follow source framing within the 1080p MAGI-2 output set.
func TestPrepareInfluencerMotionAspectRatio(t *testing.T) {
	allowedRatios := []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
	blockedRatios := []string{"5:4", "auto", "adaptive"}
	for _, allowed := range allowedRatios {
		t.Run("1080p/motion/allowed-"+allowed, func(t *testing.T) {
			req := validMotionRequest()
			req["resolution"], req["mode"] = "1080p", "motion"
			req["aspect_ratio"] = allowed
			rec, probe := runMotionPrepare(t, req)
			require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
			require.True(t, probe.reached)
			assert.Equal(t, allowed, probe.body["aspect_ratio"])
			assert.Equal(t, allowed, probe.meta["size"])
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			setInfluencerMotionTaskRequest(c, probe.body)
			value, exists := c.Get("task_request")
			require.True(t, exists)
			request, ok := value.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, allowed, request["aspect_ratio"])
			assert.NotContains(t, request, influencerMotionGroupField)
		})
	}
	for _, blocked := range blockedRatios {
		t.Run("1080p/motion/blocked-"+blocked, func(t *testing.T) {
			req := validMotionRequest()
			req["resolution"], req["mode"] = "1080p", "motion"
			req["aspect_ratio"] = blocked
			rec, probe := runMotionPrepare(t, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.False(t, probe.reached, "invalid ratios stop before distribution")
		})
	}
	for name, input := range map[string]any{"empty": "", "whitespace": "   ", "malformed": "16/9", "wrong type": 169} {
		t.Run("invalid/"+name, func(t *testing.T) {
			req := validMotionRequest()
			req["aspect_ratio"] = input
			rec, probe := runMotionPrepare(t, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.False(t, probe.reached)
		})
	}
}

// Reference input validity does not imply source-preserving output support.
func TestPrepareInfluencerMotionNonStandardReferenceRatio(t *testing.T) {
	for _, mode := range []string{"motion", "replace"} {
		t.Run(mode, func(t *testing.T) {
			request := validMotionRequest()
			request["mode"] = mode
			request["_probe_width"], request["_probe_height"] = 800, 640
			response, probe := runMotionPrepare(t, request)
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assert.False(t, probe.reached)
			assert.Contains(t, response.Body.String(), "Current generation service cannot preserve this reference video aspect ratio")
		})
	}
}

// The options surface tells the form exactly which output framings the bound
// lines evidence per resolution; it exposes no channel or vendor details.
func TestInfluencerMotionOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/pg/influencer/motion/options", InfluencerMotionOptions)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pg/influencer/motion/options", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			InputConstraints map[string]service.InfluencerVideoInputConstraints `json:"input_constraints"`
			Resolutions      map[string]struct {
				AspectRatios []string `json:"aspect_ratios"`
			} `json:"resolutions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	assert.Len(t, body.Data.Resolutions, 1)
	assert.Equal(t, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, body.Data.Resolutions["1080p"].AspectRatios)
	assert.Equal(t, 2.0, body.Data.InputConstraints["motion"].MinDurationSeconds)
	assert.Equal(t, 4.0, body.Data.InputConstraints["replace"].MinDurationSeconds)
	assert.Equal(t, 4.0, body.Data.InputConstraints["recreate"].MinDurationSeconds)
	assert.Equal(t, 0.4, body.Data.InputConstraints["replace"].MinAspectRatio)
	assert.Equal(t, 2.5, body.Data.InputConstraints["replace"].MaxAspectRatio)
}

func TestPrepareInfluencerMotionMultiImages(t *testing.T) {
	req := validMotionRequest()
	urls := make([]string, 10)
	for i := range urls {
		urls[i] = motionTestImage
	}
	req["subject_image_urls"] = urls
	rec, probe := runMotionPrepare(t, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.True(t, probe.reached)
	assert.Len(t, probe.body["image_urls"], 10)
	assert.Equal(t, motionTestImage, probe.meta["subject_image_url"])
	assert.Len(t, probe.meta["subject_image_urls"], 10)
}

func TestPrepareInfluencerMotionFramesPromptPerMode(t *testing.T) {
	motion := validMotionRequest()
	motion["subject_character_type"] = "human"
	_, probe := runMotionPrepare(t, motion)
	prompt, _ := probe.body["prompt"].(string)
	assert.True(t, strings.HasPrefix(prompt, influencerMotionPrompts["motion"]))
	assert.True(t, strings.HasSuffix(prompt, "\n\nsmile at the end"))

	replace := validMotionRequest()
	replace["subject_character_type"] = "human"
	replace["mode"] = "replace"
	replace["prompt"] = ""
	_, probe = runMotionPrepare(t, replace)
	assert.Equal(t, influencerMotionPrompts["replace"], probe.body["prompt"])
}

func TestInfluencerMotionPromptDescribesVariedSceneForEveryCharacterType(t *testing.T) {
	for _, characterType := range []string{"", "human", "cat", "dog"} {
		t.Run("type="+characterType, func(t *testing.T) {
			prompt := influencerCharacterMotionPrompt("motion", characterType)
			assert.Contains(t, prompt, influencerMotionScene)
			assert.Contains(t, prompt, "only as a motion reference")
			assert.Contains(t, prompt, influencerMotionAudioRule, "no separate audio reference, so the prompt owns the audio contract")
			assert.NotContains(t, prompt, "camera framing")
			lower := strings.ToLower(prompt)
			for _, editWord := range []string{"replace", "modify", "change", "add ", "remove", "delete", "edit"} {
				assert.NotContains(t, lower, editWord)
			}
			// The framing must hold for any clip: no clip-specific objects or
			// camera styles, and the shot's structure, light and time stay put.
			for _, specific := range []string{"window", "seat", "handheld", "sunset"} {
				assert.NotContains(t, lower, specific)
			}
			for _, kept := range []string{"spatial layout", "time of day", "same number, position, size and distance", "only focal point"} {
				assert.Contains(t, prompt, kept)
			}
		})
	}
}

func TestPrepareInfluencerReplaceUsesAutonomousRulesForAllImageTypes(t *testing.T) {
	for _, characterType := range []string{"", "human", "cat", "dog"} {
		t.Run("type="+characterType, func(t *testing.T) {
			request := validMotionRequest()
			request["mode"], request["prompt"] = "replace", ""
			request["subject_character_type"] = characterType
			images := []string{motionTestImage, "https://files.omnimux.ai/images/product-side.png"}
			request["subject_image_urls"] = images
			recorder, probe := runMotionPrepare(t, request)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
			require.True(t, probe.reached)
			prompt, ok := probe.body["prompt"].(string)
			require.True(t, ok)
			assert.Equal(t, influencerMotionPrompts["replace"], prompt)
			for _, rule := range []string{"综合全部参考图片的核心内容及其与原片的语义、功能和交互关系", "自主确定合理的替换目标与范围", "图片顺序不预设用途，用户补充的明确意图优先", "参考图片定义替换后目标的完整外观与可辨识特征", "原视频定义事件进程、动作时序、空间关系和镜头组织", "完整实现替换，消除与参考目标冲突的原有特征", "不引入图片中无关内容", "只修改完成替换所必需的部分，其余画面与原音沿用原片", "允许自然融合所需的透视、遮挡、接触、光影和运动响应调整", "保持原有交互逻辑与全片连续性", "多张图片可互补同一目标或分别对应不同目标", "对应关系贯穿全片稳定一致", "不重复增添或混合冲突形象，缺乏合理对应的内容不强行加入", "声音与参考视频1保持一致", "不新增、不替换、不重写任何声音"} {
				assert.Contains(t, prompt, rule)
			}
			assert.NotContains(t, prompt, "PRIORITY:")
			for _, specific := range []string{"人物", "角色", "服饰", "衣物", "衣服", "袖长", "裤装", "商品", "类别", "同类", "透明度"} {
				assert.NotContains(t, prompt, specific, "the default instruction must apply without a fixed object taxonomy")
			}
			assert.LessOrEqual(t, utf8.RuneCountInString(prompt), 524)
			assert.NotContains(t, prompt, "已保存角色", "an unverified type cannot assert a saved source")
			assert.NotContains(t, prompt, "用户补充：")
			assert.NotContains(t, prompt, influencerMotionScene)
			assert.Equal(t, []any{images[0], images[1]}, probe.body["image_urls"])
			assert.Equal(t, []any{motionTestFrozenVideo}, probe.body["video_urls"])
			assert.Equal(t, "", probe.meta["prompt"])
		})
	}
}

func TestPrepareInfluencerReplacePreservesFullUserSupplementLast(t *testing.T) {
	request := validMotionRequest()
	request["mode"] = "replace"
	const instruction = "只换衣服，保留原人物的脸。"
	supplement := strings.Repeat("字", influencerMotionMaxPromptLen-utf8.RuneCountInString(instruction)) + instruction
	require.Equal(t, influencerMotionMaxPromptLen, utf8.RuneCountInString(supplement))
	request["prompt"] = supplement
	recorder, probe := runMotionPrepare(t, request)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	prompt, ok := probe.body["prompt"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasSuffix(prompt, "\n\n用户补充：\n"+supplement))
	assert.Equal(t, supplement, probe.meta["prompt"])
	assert.LessOrEqual(t, utf8.RuneCountInString(prompt), 4600, "the existing 4000-character supplement fits the current line")
}

// The queued body feeds the native video adaptor directly, so every MAGI-2
// field must pass adaptor validation and omit internal routing keys.
func TestSetInfluencerMotionTaskRequestKeepsReferences(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	out := map[string]any{
		"model":          influencerMotionModel,
		"group":          "default",
		"prompt":         "p",
		"duration":       5,
		"aspect_ratio":   influencerMotionAspectRatio,
		"resolution":     "1080p",
		"generate_audio": true,
		"image_urls":     []string{motionTestImage},
		"video_urls":     []string{motionTestVideo},
	}
	setInfluencerMotionTaskRequest(c, out)
	value, ok := c.Get("task_request")
	require.True(t, ok)
	request, ok := value.(map[string]any)
	require.True(t, ok)
	want := make(map[string]any, len(out))
	for key, field := range out {
		want[key] = field
	}
	// The tier group rides in the X-Omnimux-Group header. Providers reject an
	// unknown top-level group field, so it must not reach the provider body.
	delete(want, influencerMotionGroupField)
	assert.Equal(t, want, request)
	assert.NotContains(t, request, influencerMotionGroupField)

	rawReq, err := common.Marshal(request)
	require.NoError(t, err)
	storage, err := common.CreateBodyStorage(rawReq)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })
	adaptorCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	adaptorCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(rawReq)))
	adaptorCtx.Request.Header.Set("Content-Type", "application/json")
	adaptorCtx.Set(common.KeyBodyStorage, storage)
	info := &relaycommon.RelayInfo{OriginModelName: "magi-2", TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &apimart.TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(adaptorCtx, info))
	reader, err := adaptor.BuildRequestBody(adaptorCtx, info)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(reader)
	require.NoError(t, err)
	var providerBody map[string]any
	require.NoError(t, common.Unmarshal(bodyBytes, &providerBody))
	assert.Equal(t, "magi-2-reference-to-video", providerBody["model"])
	assert.Equal(t, "1080p", providerBody["quality"])
	assert.Equal(t, "9:16", providerBody["aspect_ratio"])
	assert.Equal(t, float64(5), providerBody["duration"])
	assert.Equal(t, true, providerBody["generate_audio"])
	assert.Equal(t, []any{motionTestImage}, providerBody["image_urls"])
	assert.Equal(t, []any{motionTestVideo}, providerBody["video_urls"])

	request["prompt"] = "changed"
	assert.Equal(t, "p", out["prompt"], "the task request is a copy")
}

func TestPrepareInfluencerMotionRejectsIndependentOutputDuration(t *testing.T) {
	request := validMotionRequest()
	request["duration_sec"] = 12
	response, probe := runMotionPrepare(t, request)
	assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	assert.False(t, probe.reached)
	assert.Contains(t, response.Body.String(), "Video duration must follow the reference video")
}

func TestPrepareInfluencerReplaceFollowsIntegerReferenceAndAccurateSourceMetadata(t *testing.T) {
	request := validMotionRequest()
	request["mode"], request["duration_sec"], request["aspect_ratio"] = "replace", 10, "16:9"
	request["_probe_duration"] = 10.000001
	response, probe := runMotionPrepare(t, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.True(t, probe.reached)
	assert.Equal(t, float64(10), probe.body["duration"])
	assert.Equal(t, "16:9", probe.body["aspect_ratio"])
	assert.Equal(t, 10.000001, probe.meta["source_duration_sec"])
	assert.Equal(t, 10.0, probe.meta["request_duration_sec"])
	assert.NotContains(t, probe.body, "auto_duration")
}

func TestPrepareInfluencerReplaceRejectsActualShortReference(t *testing.T) {
	req := validMotionRequest()
	req["mode"], req["duration_sec"] = "replace", 0
	req["_probe_duration"] = 3.999
	rec, probe := runMotionPrepare(t, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.False(t, probe.reached)
	assert.Contains(t, rec.Body.String(), "3.999")
	assert.Contains(t, rec.Body.String(), "4至30")
}

func TestPrepareInfluencerReplaceToleratesOneFrameContainerDifference(t *testing.T) {
	for _, tc := range []struct {
		source  float64
		seconds int
	}{
		{4, 4},
		{9.000001, 9},
		{9.041666667, 9},
		{30, 30},
	} {
		t.Run(strconv.Itoa(tc.seconds), func(t *testing.T) {
			req := validMotionRequest()
			req["mode"], req["duration_sec"] = "replace", 0
			req["_probe_duration"] = tc.source
			rec, probe := runMotionPrepare(t, req)
			require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
			require.True(t, probe.reached)
			assert.Equal(t, float64(tc.seconds), probe.body["duration"])
			assert.Equal(t, float64(tc.seconds), probe.meta["request_duration_sec"])
			assert.Equal(t, tc.source, probe.meta["source_duration_sec"])
		})
	}
	req := validMotionRequest()
	req["mode"], req["duration_sec"] = "replace", 0
	req["_probe_duration"] = 30.001
	rec, probe := runMotionPrepare(t, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.False(t, probe.reached)
}

func TestPrepareInfluencerReplaceMatchesOnlySupportedSourceFraming(t *testing.T) {
	for _, tc := range []struct {
		name, resolution, ratio string
		width, height           int
		allowed                 bool
	}{
		{"landscape-16x9", "1080p", "16:9", 1280, 720, true},
		{"portrait-9x16", "1080p", "9:16", 720, 1280, true},
		{"square-1x1", "1080p", "1:1", 800, 800, true},
		{"classic-4x3", "1080p", "4:3", 960, 720, true},
		{"three-four-3x4", "1080p", "3:4", 960, 1280, true},
		{"ultrawide-21x9", "1080p", "21:9", 1680, 720, true},
		{"legacy-pixel-tolerance", "1080p", "16:9", 854, 480, true},
		{"outside-legacy-pixel-tolerance", "1080p", "", 855, 480, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validMotionRequest()
			req["mode"], req["duration_sec"] = "replace", 0
			req["resolution"] = tc.resolution
			req["_probe_duration"] = 10.000001
			req["_probe_width"], req["_probe_height"] = tc.width, tc.height
			rec, probe := runMotionPrepare(t, req)
			if !tc.allowed {
				assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				assert.False(t, probe.reached)
				assert.Contains(t, rec.Body.String(), "Current generation service cannot preserve this reference video aspect ratio")
				return
			}
			require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
			require.True(t, probe.reached)
			assert.Equal(t, tc.ratio, probe.body["aspect_ratio"])
			assert.Equal(t, float64(10), probe.body["duration"])
			assert.NotContains(t, probe.body, "auto_duration")
			assert.NotContains(t, probe.body, "omni_reference_task_type")
		})
	}
}

func TestLegacyInspectionAcceptsFiveByFourBeforeBrowserCapabilityRejection(t *testing.T) {
	db := referenceInspectionTestDB(t)
	const source = "https://8.8.8.8/own-five-four-reference.mp4"
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "five-four-owner", Quota: 12345}).Error)
	require.NoError(t, db.Create(&model.File{FileId: "five-four-reference", FileUrl: source, UserId: 41, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	previousInspector := influencerVideoReferenceInspector
	t.Cleanup(func() { influencerVideoReferenceInspector = previousInspector })
	influencerVideoReferenceInspector = func(_ context.Context, raw, mode string) (service.InfluencerVideoMetadata, error) {
		assert.Equal(t, source, raw)
		assert.Equal(t, "replace", mode)
		return service.InfluencerVideoMetadata{Duration: 5, Width: 800, Height: 640, FrameRate: 24, VideoCodec: "h264", Container: "mp4"}, nil
	}
	engine := gin.New()
	engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
	payload, err := common.Marshal(map[string]any{"video_url": source, "mode": "replace"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/inspect", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	selection := httptest.NewRecorder()
	engine.ServeHTTP(selection, request)
	require.Equal(t, http.StatusOK, selection.Code, selection.Body.String())

	submit := validMotionRequest()
	submit["mode"], submit["duration_sec"] = "replace", 0
	submit["video_url"] = source
	submit["_probe_width"], submit["_probe_height"] = 800, 640
	rejected, probe := runMotionPrepare(t, submit)
	assert.Equal(t, http.StatusBadRequest, rejected.Code, rejected.Body.String())
	assert.Contains(t, rejected.Body.String(), "Current generation service cannot preserve this reference video aspect ratio")
	assert.False(t, probe.reached, "the capability rejection precedes distribution and billing")
	var user model.User
	require.NoError(t, db.First(&user, 41).Error)
	assert.Equal(t, 12345, user.Quota)
	var files, tasks int64
	require.NoError(t, db.Model(&model.File{}).Count(&files).Error)
	require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
	assert.Equal(t, int64(1), files, "the selected source upload is retained")
	assert.Zero(t, tasks)
}
