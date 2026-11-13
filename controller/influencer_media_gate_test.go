package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareInfluencerMotionSuppliesOwnerAndExistingOriginIsolation(t *testing.T) {
	previous := service.InfluencerBrowserVideoPreparer
	oldServer := system_setting.ServerAddress
	oldTrusted := common.SessionCookieTrustedURLs
	t.Cleanup(func() {
		service.InfluencerBrowserVideoPreparer = previous
		system_setting.ServerAddress = oldServer
		common.SessionCookieTrustedURLs = oldTrusted
	})
	system_setting.ServerAddress = "https://app.example"
	common.SessionCookieTrustedURLs = nil
	service.InfluencerBrowserVideoPreparer = func(_ context.Context, raw string, userID int, metadata service.InfluencerVideoMetadata, validate func(string) bool) (service.PreparedInfluencerVideo, error) {
		assert.Equal(t, 41, userID)
		assert.Equal(t, motionTestVideo, raw)
		for _, unsafe := range []string{"http://files.example/a", "https://app.example/a", "https://app.example:444/a", "https://user@files.example/a", "https://files.example/a#fragment"} {
			assert.False(t, validate(unsafe), unsafe)
		}
		return service.PreparedInfluencerVideo{Metadata: service.InfluencerVideoMetadata{Duration: 5, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", Container: "mp4"}, URL: motionTestFrozenVideo}, nil
	}
	response, probe := runMotionPrepareWithInspector(t, validMotionRequest())
	require.Equal(t, http.StatusNoContent, response.Code)
	assert.NotContains(t, probe.body, "audios", "no separate audio reference is submitted")
	assert.NotContains(t, probe.meta, "audio_extracted")
	assert.Contains(t, probe.body["prompt"], influencerMotionAudioRule)
}

func TestPrepareInfluencerMotionRejectsMissingOwnerBeforeMediaOrBilling(t *testing.T) {
	previous := service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() { service.InfluencerBrowserVideoPreparer = previous })
	service.InfluencerBrowserVideoPreparer = func(context.Context, string, int, service.InfluencerVideoMetadata, func(string) bool) (service.PreparedInfluencerVideo, error) {
		t.Fatal("must not inspect without authenticated owner")
		return service.PreparedInfluencerVideo{}, nil
	}
	engine := gin.New()
	reached := false
	engine.POST("/pg/influencer/motion", PrepareInfluencerMotion, func(c *gin.Context) { reached = true; c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/pg/influencer/motion", strings.NewReader(`{"mode":"motion","resolution":"720p","duration_sec":5,"video_url":"https://files.omnimux.ai/videos/dance.mp4","subject_image_url":"https://files.omnimux.ai/images/subject.png"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.False(t, reached)
}

func TestPrepareInfluencerMotionNeverForwardsMutableOriginalURL(t *testing.T) {
	response, probe := runMotionPrepare(t, validMotionRequest())
	require.Equal(t, http.StatusNoContent, response.Code)
	require.True(t, probe.reached)
	assert.NotEqual(t, []any{motionTestVideo}, probe.body["videos"], "upstream must use the frozen object, not the original mutable URL")
	assert.Equal(t, motionTestVideo, probe.meta["video_url"], "history retains the original material identity")
}

func TestPrepareInfluencerMotionRejectsChangedReferenceFraming(t *testing.T) {
	for _, mode := range []string{"motion", "replace"} {
		t.Run(mode, func(t *testing.T) {
			request := validMotionRequest()
			request["mode"], request["aspect_ratio"] = mode, "16:9"
			response, probe := runMotionPrepareWithInspector(t, request)
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assert.False(t, probe.reached)
			assert.Contains(t, response.Body.String(), "Video aspect ratio must follow the reference video")
		})
	}
}

func TestPrepareInfluencerMotionRejectsCancelledPreparationBeforeDistribution(t *testing.T) {
	previous := service.InfluencerBrowserVideoPreparer
	t.Cleanup(func() { service.InfluencerBrowserVideoPreparer = previous })
	service.InfluencerBrowserVideoPreparer = func(context.Context, string, int, service.InfluencerVideoMetadata, func(string) bool) (service.PreparedInfluencerVideo, error) {
		return service.PreparedInfluencerVideo{}, context.Canceled
	}
	request := validMotionRequest()
	request["duration_sec"] = 5
	response, probe := runMotionPrepareWithInspector(t, request)
	assert.Equal(t, http.StatusRequestTimeout, response.Code)
	assert.False(t, probe.reached)
}
