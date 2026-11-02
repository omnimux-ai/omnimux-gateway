package controller

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func browserMotionRequest() map[string]any {
	request := validMotionRequest()
	request["video_url"] = "https://files.omnimux.ai/influencer/motion/741a2220a4c89289.mp4"
	request["video_metadata"] = service.InfluencerVideoMetadata{Duration: 5, VideoDuration: 5, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: 1234}
	return request
}

func TestBrowserMotionFollowsDeclaredReferenceBeforeNativeBilling(t *testing.T) {
	oldInspector, oldPreparer := service.InfluencerVideoInspector, service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() {
		service.InfluencerVideoInspector = oldInspector
		service.InfluencerBrowserVideoPreparer = oldPreparer
	})
	service.InfluencerVideoInspector = func(context.Context, string, int, func(string) bool) (service.PreparedInfluencerVideo, error) {
		t.Fatal("workbench must never fall back to server probe")
		return service.PreparedInfluencerVideo{}, nil
	}
	calls := 0
	service.InfluencerBrowserVideoPreparer = func(_ context.Context, raw string, owner int, metadata service.InfluencerVideoMetadata, valid func(string) bool) (service.PreparedInfluencerVideo, error) {
		calls++
		assert.Equal(t, 41, owner)
		assert.NotEmpty(t, raw)
		assert.False(t, valid("https://127.0.0.1/video.mp4"))
		return service.PreparedInfluencerVideo{Metadata: metadata, URL: motionTestFrozenVideo}, nil
	}
	request := browserMotionRequest()
	settings := system_setting.GetFetchSetting()
	oldSettings := *settings
	settings.ApplyIPFilterForDomain = false
	t.Cleanup(func() { *settings = oldSettings })
	delete(request, "duration_sec")
	response, probe := runMotionPrepareWithInspector(t, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	assert.Equal(t, "5", probe.body["seconds"])
	assert.Equal(t, "9:16", probe.body["aspect_ratio"])
	assert.NotContains(t, probe.body, "audios", "no separate audio reference is submitted")
	assert.NotContains(t, probe.meta, "audio_extracted")
	assert.Contains(t, probe.body["prompt"], influencerMotionAudioRule, "the prompt carries the audio-consistency instruction")
	assert.Equal(t, "browser", probe.meta["video_metadata_source"])
	assert.Contains(t, probe.meta, "video_metadata")
	assert.Equal(t, 1, calls)
}

func TestBrowserMotionRejectsMissingInvalidOrChangedDeclarationsBeforePreparation(t *testing.T) {
	previous := service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() { service.InfluencerBrowserVideoPreparer = previous })
	preparations := 0
	service.InfluencerBrowserVideoPreparer = func(_ context.Context, _ string, _ int, metadata service.InfluencerVideoMetadata, _ func(string) bool) (service.PreparedInfluencerVideo, error) {
		preparations++
		return service.PreparedInfluencerVideo{Metadata: metadata, URL: motionTestFrozenVideo}, nil
	}
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing", func(r map[string]any) { delete(r, "video_metadata") }},
		{"missing bytes", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.SizeBytes = 0
			r["video_metadata"] = m
		}},
		{"missing visual duration", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.VideoDuration = 0
			r["video_metadata"] = m
		}},
		{"source too long", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Duration = 31
			r["video_metadata"] = m
		}},
		{"visual duration cannot follow container", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.VideoDuration = 5.2
			r["video_metadata"] = m
		}},
		{"visual duration below four cannot extend", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Duration, m.VideoDuration = 4, 3.999
			r["duration_sec"] = 4
			r["video_metadata"] = m
		}},
		{"declarations differ beyond two frames", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Duration = 5.3
			m.VideoDuration = 5.2
			r["video_metadata"] = m
		}},
		{"just below four cannot extend", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Duration, m.VideoDuration = 3.999, 3.999
			r["video_metadata"] = m
		}},
		{"two seconds cannot extend", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Duration = 2
			m.VideoDuration = 2
			r["video_metadata"] = m
		}},
		{"request seconds differ", func(r map[string]any) { r["duration_sec"] = 6 }},
		{"request ratio differs", func(r map[string]any) { r["aspect_ratio"] = "16:9" }},
		{"source framing unsupported", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Width = 1000
			m.Height = 800
			r["video_metadata"] = m
		}},
		{"720p three four unsupported", func(r map[string]any) {
			m := r["video_metadata"].(service.InfluencerVideoMetadata)
			m.Width = 960
			m.Height = 1280
			r["video_metadata"] = m
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			preparations = 0
			request := browserMotionRequest()
			test.change(request)
			response, probe := runMotionPrepareWithInspector(t, request)
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assert.False(t, probe.reached)
			assert.Zero(t, preparations, "invalid declarations must stop before media preparation and billing")
		})
	}
}

func TestBrowserMotionRetainsControlledSourceOwnershipWithoutProbing(t *testing.T) {
	db := referenceInspectionTestDB(t)
	source := "https://8.8.8.8/other-user.mp4"
	require.NoError(t, db.Create(&model.File{FileId: "other-reference", FileUrl: source, UserId: 42, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	previous := service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() { service.InfluencerBrowserVideoPreparer = previous })
	service.InfluencerBrowserVideoPreparer = func(context.Context, string, int, service.InfluencerVideoMetadata, func(string) bool) (service.PreparedInfluencerVideo, error) {
		t.Fatal("must not read another user's media")
		return service.PreparedInfluencerVideo{}, nil
	}
	request := browserMotionRequest()
	request["video_url"] = source
	response, probe := runMotionPrepareWithInspector(t, request)
	assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	assert.False(t, probe.reached)
}

func TestBrowserVideoOutputToleratesOneFrameAcrossContainerAndPicture(t *testing.T) {
	for _, durations := range []struct{ container, picture float64 }{
		{5, 5 + 1.0/24},
		{5 + 1.0/24, 5},
	} {
		metadata := browserUploadMetadata(1234)
		metadata.Duration, metadata.VideoDuration = durations.container, durations.picture
		seconds, ratio, err := influencerBrowserVideoOutput(influencerMotionRequest{Mode: "motion", VideoMetadata: &metadata}, "480p")
		require.NoError(t, err)
		assert.Equal(t, 5, seconds)
		assert.Equal(t, "9:16", ratio)
	}
}

func TestBrowserMotionAcceptsFractionalProductionClipBeforePreparation(t *testing.T) {
	previous := service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() { service.InfluencerBrowserVideoPreparer = previous })
	preparations := 0
	service.InfluencerBrowserVideoPreparer = func(_ context.Context, _ string, owner int, metadata service.InfluencerVideoMetadata, _ func(string) bool) (service.PreparedInfluencerVideo, error) {
		preparations++
		assert.Equal(t, 41, owner)
		return service.PreparedInfluencerVideo{Metadata: metadata, URL: motionTestFrozenVideo}, nil
	}
	request := browserMotionRequest()
	delete(request, "duration_sec")
	request["resolution"] = "720p"
	request["video_metadata"] = service.InfluencerVideoMetadata{Duration: 5.717007, VideoDuration: 5.708333, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 2863940}
	response, probe := runMotionPrepareWithInspector(t, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	assert.Equal(t, 1, preparations)
	assert.Equal(t, "6", probe.body["seconds"])
	assert.Equal(t, "9:16", probe.body["aspect_ratio"])
	assert.Equal(t, 6, probe.meta["duration_sec"])
	assert.Equal(t, 6.0, probe.meta["request_duration_sec"])
	assert.Equal(t, 5.717007, probe.meta["source_duration_sec"])

	request["duration_sec"] = 7
	response, rejected := runMotionPrepareWithInspector(t, request)
	assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	assert.False(t, rejected.reached)
	assert.Equal(t, 1, preparations, "an independent request duration must stop before preparation and billing")
}

func TestBrowserMotionPreservesAudioPaddedDeclarationsWithoutProbing(t *testing.T) {
	oldInspector, oldPreparer := service.InfluencerVideoInspector, service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() {
		service.InfluencerVideoInspector = oldInspector
		service.InfluencerBrowserVideoPreparer = oldPreparer
	})
	service.InfluencerVideoInspector = func(context.Context, string, int, func(string) bool) (service.PreparedInfluencerVideo, error) {
		t.Fatal("browser declarations must never fall back to a server probe")
		return service.PreparedInfluencerVideo{}, nil
	}
	for _, sample := range []struct {
		name     string
		metadata service.InfluencerVideoMetadata
		seconds  int
		ratio    string
	}{
		{"reported seven second clip", service.InfluencerVideoMetadata{Duration: 7.072, VideoDuration: 7.041667, Width: 480, Height: 854, FrameRate: 24, VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 6650581}, 7, "9:16"},
		{"four second output with audio tail", service.InfluencerVideoMetadata{Duration: 4.064, VideoDuration: 4.041667, Width: 854, Height: 480, FrameRate: 24, VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 1234}, 4, "16:9"},
	} {
		for _, mode := range []string{"motion", "replace"} {
			t.Run(sample.name+"/"+mode, func(t *testing.T) {
				calls := 0
				service.InfluencerBrowserVideoPreparer = func(_ context.Context, _ string, owner int, metadata service.InfluencerVideoMetadata, _ func(string) bool) (service.PreparedInfluencerVideo, error) {
					calls++
					assert.Equal(t, 41, owner)
					assert.Equal(t, sample.metadata, metadata, "media preparation receives the original declaration")
					return service.PreparedInfluencerVideo{Metadata: metadata, URL: motionTestFrozenVideo}, nil
				}
				request := browserMotionRequest()
				request["mode"] = mode
				request["resolution"] = "480p"
				request["video_metadata"] = sample.metadata
				delete(request, "duration_sec")
				response, probe := runMotionPrepareWithInspector(t, request)
				require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
				assert.Equal(t, 1, calls)
				assert.Equal(t, strconv.Itoa(sample.seconds), probe.body["seconds"])
				assert.Equal(t, sample.ratio, probe.body["aspect_ratio"])
				assert.Equal(t, sample.seconds, probe.meta["duration_sec"])
				assert.Equal(t, sample.metadata.Duration, probe.meta["source_duration_sec"])
				assert.Equal(t, sample.metadata, probe.meta["video_metadata"], "persist both raw fractional durations unchanged")
				assert.Equal(t, "browser", probe.meta["video_metadata_source"])

				request["duration_sec"] = sample.seconds + 1
				response, rejected := runMotionPrepareWithInspector(t, request)
				assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
				assert.False(t, rejected.reached)
				assert.Equal(t, 1, calls, "changed request duration must stop before preparation")
				delete(request, "duration_sec")
				request["aspect_ratio"] = "3:4"
				response, rejected = runMotionPrepareWithInspector(t, request)
				assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
				assert.False(t, rejected.reached)
				assert.Equal(t, 1, calls, "changed request framing must stop before preparation")
			})
		}
	}
}

func TestBrowserVideoOutputBoundsVisualDurationAndAudioPadding(t *testing.T) {
	for _, test := range []struct {
		name                 string
		container, picture   float64
		audio, containerType string
		accept               bool
	}{
		{"AAC positive container tail", 7.072, 7.041667, "aac", "mp4", true},
		{"four second AAC positive container tail", 4.064, 4.041667, "aac", "mp4", true},
		{"supported MOV PCM positive tail", 7.072, 7.041667, "pcm", "mov", true},
		{"supported MP3 positive tail", 7.072, 7.041667, "mp3", "mp4", true},
		{"prior reverse record discrepancy", 7.04, 6.96, "", "mp4", true},
		{"prior one frame without audio", 7 + 1.0/24, 7, "", "mp4", true},
		{"production fractional clip", 5.717007, 5.708333, "aac", "mp4", true},
		{"consistent fractional picture follows rounded duration", 7.2, 7.2, "aac", "mp4", true},
		{"consistent small fractional picture", 5.2, 5.2, "aac", "mp4", true},
		{"container cannot hide fractional picture", 7, 7.2, "aac", "mp4", false},
		{"large positive tail", 7.2, 7, "aac", "mp4", false},
		{"tail just above one frame stays inside two frames", 7.084334, 7.041667, "aac", "mp4", true},
		{"tail above two frames", 7.127001, 7.041667, "aac", "mp4", false},
		{"positive container tail without audio within two frames", 7.072, 7.041667, "", "mp4", true},
		{"unsupported audio cannot allow tail", 7.072, 7.041667, "opus", "mp4", false},
		{"reverse container rounding within two frames", 6.927, 6.958333, "aac", "mp4", true},
		{"container below hard minimum", 3.999, 4, "aac", "mp4", false},
		{"picture below hard minimum", 4, 3.999, "aac", "mp4", false},
		{"container above hard maximum", 30.001, 30, "aac", "mp4", false},
		{"picture above hard maximum", 30, 30.001, "aac", "mp4", false},
		{"picture missing", 7, 0, "aac", "mp4", false},
		{"picture nonfinite", 7, math.NaN(), "aac", "mp4", false},
	} {
		for _, mode := range []string{"motion", "replace"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				metadata := browserUploadMetadata(1234)
				metadata.Duration, metadata.VideoDuration = test.container, test.picture
				metadata.AudioCodec, metadata.Container = test.audio, test.containerType
				seconds, ratio, err := influencerBrowserVideoOutput(influencerMotionRequest{Mode: mode, VideoMetadata: &metadata}, "480p")
				if !test.accept {
					require.Error(t, err)
					assert.Zero(t, seconds)
					assert.Empty(t, ratio)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, int(math.Round(test.picture)), seconds)
				assert.Equal(t, "9:16", ratio)
			})
		}
	}
}
