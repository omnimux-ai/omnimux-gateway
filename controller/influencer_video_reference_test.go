package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func referenceInspectionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}, &model.Task{}, &model.User{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		connection, err := db.DB()
		require.NoError(t, err)
		assert.NoError(t, connection.Close())
	})
	return db
}

func TestInfluencerVideoReferenceSourceRequiresOwnershipAndLiveUpload(t *testing.T) {
	db := referenceInspectionTestDB(t)
	for _, tc := range []struct {
		name          string
		owner, status int
		expires       time.Time
		allowed       bool
	}{
		{"owned", 41, model.FileStatusActive, time.Now().Add(time.Hour), true},
		{"other-owner", 42, model.FileStatusActive, time.Now().Add(time.Hour), false},
		{"pending", 41, model.FileStatusUploadPending, time.Now().Add(time.Hour), false},
		{"deleted", 41, model.FileStatusDeleted, time.Now().Add(time.Hour), false},
		{"expired", 41, model.FileStatusActive, time.Now().Add(-time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "https://8.8.8.8/videos/" + tc.name + ".mp4"
			record := model.File{FileId: tc.name, FileUrl: raw, UserId: tc.owner, Status: tc.status, ExpiresAt: tc.expires}
			require.NoError(t, db.Create(&record).Error)
			// A saved task must never revive a deleted/expired/foreign upload.
			input, err := common.Marshal(map[string]any{"video_url": raw})
			require.NoError(t, err)
			require.NoError(t, db.Create(&model.Task{TaskID: tc.name, UserId: 41, Action: ActionInfluencerMotion, Properties: model.Properties{Input: string(input)}}).Error)
			allowed, err := influencerVideoReferenceSource(context.Background(), raw, 41)
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, allowed)
		})
	}
	t.Run("duplicate records deny ambiguous URL", func(t *testing.T) {
		raw := "https://8.8.8.8/duplicate.mp4"
		for _, id := range []string{"duplicate-1", "duplicate-2"} {
			require.NoError(t, db.Create(&model.File{FileId: id, FileUrl: raw, UserId: 41, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
		}
		allowed, err := influencerVideoReferenceSource(context.Background(), raw, 41)
		require.NoError(t, err)
		assert.False(t, allowed)
	})
}

func TestInfluencerVideoReferenceSourceAcceptsOnlyControlledPresetAndOwnHistory(t *testing.T) {
	db := referenceInspectionTestDB(t)
	input, err := common.Marshal(map[string]any{"video_url": "https://8.8.8.8/legacy/original.mp4", "prepared_video_url": "https://8.8.8.8/frozen.mp4"})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Task{TaskID: "history", UserId: 41, Action: ActionInfluencerMotion, Properties: model.Properties{Input: string(input)}}).Error)
	for _, tc := range []struct {
		raw     string
		owner   int
		allowed bool
	}{
		{"https://files.omnimux.ai/influencer/motion/741a2220a4c89289.mp4", 41, true},
		{"https://files.omnimux.ai/influencer/motion/file_741a2220a4c89289.mp4", 41, false},
		{"https://files.omnimux.ai/influencer/motion/741a2220a4c89289.mp4?anything=1", 41, false},
		{"https://files.omnimux.ai.evil.example/influencer/motion/741a2220a4c89289.mp4", 41, false},
		{"https://8.8.8.8/legacy/original.mp4", 41, true},
		{"https://8.8.8.8/frozen.mp4", 41, true},
		{"https://8.8.8.8/legacy/original.mp4", 42, false},
		{"https://8.8.8.8/arbitrary.mp4", 41, false},
	} {
		allowed, err := influencerVideoReferenceSource(context.Background(), tc.raw, tc.owner)
		require.NoError(t, err)
		assert.Equal(t, tc.allowed, allowed, tc.raw)
	}
}

func TestInspectInfluencerMotionVideoReturnsMetadataWithoutGenerationSideEffects(t *testing.T) {
	db := referenceInspectionTestDB(t)
	raw := "https://8.8.8.8/videos/upload.mp4"
	require.NoError(t, db.Create(&model.File{FileId: "own", FileUrl: raw, UserId: 41, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "inspect-owner", Quota: 12345}).Error)
	previousInspector := influencerVideoReferenceInspector
	t.Cleanup(func() { influencerVideoReferenceInspector = previousInspector })
	calls := 0
	influencerVideoReferenceInspector = func(_ context.Context, url, mode string) (service.InfluencerVideoMetadata, error) {
		calls++
		assert.Equal(t, raw, url)
		assert.Equal(t, "motion", mode)
		return service.InfluencerVideoMetadata{Duration: 2.125, VideoDuration: 2.125, Width: 800, Height: 640, FrameRate: 24, VideoCodec: "hevc", Container: "mov", SizeBytes: 4567}, nil
	}
	engine := gin.New()
	engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(`{"mode":"motion","video_url":"`+raw+`"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Success bool                            `json:"success"`
		Data    service.InfluencerVideoMetadata `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success)
	assert.Equal(t, 2.125, body.Data.Duration)
	assert.Equal(t, 800, body.Data.Width)
	assert.Equal(t, "hevc", body.Data.VideoCodec)
	assert.Equal(t, int64(4567), body.Data.SizeBytes)
	assert.Equal(t, 1, calls)
	var files, tasks int64
	require.NoError(t, db.Model(&model.File{}).Count(&files).Error)
	require.NoError(t, db.Model(&model.Task{}).Count(&tasks).Error)
	assert.Equal(t, int64(1), files)
	assert.Zero(t, tasks)
	var user model.User
	require.NoError(t, db.First(&user, 41).Error)
	assert.Equal(t, 12345, user.Quota)
}

func TestInspectInfluencerMotionVideoRejectsBeforeDownloadingAndPreservesErrorStatus(t *testing.T) {
	db := referenceInspectionTestDB(t)
	previousInspector := influencerVideoReferenceInspector
	previousAddress, previousTrusted := system_setting.ServerAddress, common.SessionCookieTrustedURLs
	t.Cleanup(func() {
		influencerVideoReferenceInspector = previousInspector
		system_setting.ServerAddress, common.SessionCookieTrustedURLs = previousAddress, previousTrusted
	})
	system_setting.ServerAddress = "https://app.example"
	common.SessionCookieTrustedURLs = nil
	influencerVideoReferenceInspector = func(context.Context, string, string) (service.InfluencerVideoMetadata, error) {
		t.Fatal("uncontrolled input must not be downloaded")
		return service.InfluencerVideoMetadata{}, nil
	}
	for _, tc := range []struct {
		name, raw, mode string
		owner, status   int
	}{
		{"unauthenticated", "https://8.8.8.8/a.mp4", "motion", 0, http.StatusUnauthorized},
		{"unknown-mode", "https://8.8.8.8/a.mp4", "other", 41, http.StatusBadRequest},
		{"application-origin", "https://app.example/a.mp4", "motion", 41, http.StatusBadRequest},
		{"userinfo", "https://user@8.8.8.8/a.mp4", "motion", 41, http.StatusBadRequest},
		{"private-ip", "https://127.0.0.1/a.mp4", "motion", 41, http.StatusBadRequest},
		{"insecure", "http://8.8.8.8/a.mp4", "motion", 41, http.StatusBadRequest},
		{"unowned", "https://8.8.8.8/a.mp4", "motion", 41, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/inspect", func(c *gin.Context) { c.Set("id", tc.owner) }, InspectInfluencerMotionVideo)
			response := httptest.NewRecorder()
			payload, err := common.Marshal(map[string]any{"video_url": tc.raw, "mode": tc.mode})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
	raw := "https://8.8.8.8/own.mp4"
	require.NoError(t, db.Create(&model.File{FileId: "own", FileUrl: raw, UserId: 41, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}).Error)
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"busy", service.ErrInfluencerVideoBusy, http.StatusServiceUnavailable},
		{"canceled", context.Canceled, http.StatusRequestTimeout},
		{"invalid", errors.New("视频实际时长为1秒，必须在2至30秒之间"), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			influencerVideoReferenceInspector = func(context.Context, string, string) (service.InfluencerVideoMetadata, error) {
				return service.InfluencerVideoMetadata{}, tc.err
			}
			engine := gin.New()
			engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(`{"video_url":"`+raw+`","mode":"motion"}`))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
			assert.Contains(t, response.Body.String(), tc.err.Error())
		})
	}
}
