package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPreparedReferenceReclaimedWhenDistributionRefuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	path := filepath.Join(t.TempDir(), "reference.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=24", "-t", "4", "-an", "-c:v", "libx264", "-threads", "1", path).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	oldInspector, oldDB, oldDriver := service.InfluencerBrowserVideoPreparer, model.DB, storage.GetStorageDriver()
	settings := system_setting.GetFetchSetting()
	oldSettings := *settings
	oldAddress := system_setting.ServerAddress
	settings.EnableSSRFProtection = false
	system_setting.ServerAddress = "https://app.example"
	t.Setenv("FILE_PUBLIC_BASE_URL", "https://files.example")
	dir := t.TempDir()
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(dir))
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}, &model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "cleanup-owner", Status: 1}).Error)
	model.DB = db
	t.Cleanup(func() {
		service.InfluencerBrowserVideoPreparer = oldInspector
		model.DB = oldDB
		storage.SetGlobalDriver(oldDriver)
		*settings = oldSettings
		system_setting.ServerAddress = oldAddress
		sqlDB, e := db.DB()
		require.NoError(t, e)
		assert.NoError(t, sqlDB.Close())
	})
	// Replace only the external download URL; execute actual preparation, storage and DB paths.
	service.InfluencerBrowserVideoPreparer = func(ctx context.Context, _ string, owner int, metadata service.InfluencerVideoMetadata, valid func(string) bool) (service.PreparedInfluencerVideo, error) {
		return oldInspector(ctx, server.URL, owner, metadata, valid)
	}
	for _, tc := range []struct {
		name       string
		submitting bool
		status     int
		retained   int64
	}{
		{"refused-before-submission", false, http.StatusTooManyRequests, 0},
		{"uncertain-response-after-submission", true, http.StatusBadGateway, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/pg/influencer/motion", func(c *gin.Context) { c.Set("id", 41); c.Next() }, PrepareInfluencerMotion, func(c *gin.Context) {
				if tc.submitting {
					c.Set(ctxInfluencerMotionSubmitting, true)
				}
				c.AbortWithStatus(tc.status)
			})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/pg/influencer/motion", strings.NewReader(`{"mode":"motion","resolution":"720p","duration_sec":4,"aspect_ratio":"9:16","video_url":"https://files.omnimux.ai/influencer/motion/741a2220a4c89289.mp4","subject_image_url":"https://files.example/subject.png","video_metadata":{"duration":4,"video_duration":4,"width":720,"height":1280,"frame_rate":24,"video_codec":"h264","container":"mp4","size_bytes":1234}}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			var count int64
			require.NoError(t, db.Model(&model.File{}).Count(&count).Error)
			assert.Equal(t, tc.retained, count, "only a submission attempt may retain tracked references")
			objects := int64(0)
			require.NoError(t, filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					objects++
				}
				return nil
			}))
			assert.Equal(t, tc.retained, objects)
		})
	}
}
