package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type frozenTestDriver struct {
	objects    map[string][]byte
	failUpload bool
	mismatch   bool
	failDelete bool
	onUpload   func(context.Context)
}

func (d *frozenTestDriver) Upload(ctx context.Context, key string, data []byte, mime string) (string, error) {
	d.objects[key] = bytes.Clone(data)
	if d.onUpload != nil {
		d.onUpload(ctx)
	}
	if d.failUpload {
		return "", errors.New("partial upload")
	}
	if d.mismatch {
		return "https://other.example/wrong", nil
	}
	return d.GetPublicURL(key), nil
}
func (d *frozenTestDriver) Get(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	data := d.objects[key]
	return io.NopCloser(bytes.NewReader(data)), "", int64(len(data)), nil
}
func (d *frozenTestDriver) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.failDelete {
		return errors.New("delete unavailable")
	}
	delete(d.objects, key)
	return nil
}
func (d *frozenTestDriver) GetPublicURL(key string) string { return "https://files.example/" + key }
func (d *frozenTestDriver) IsR2() bool                     { return true }
func (d *frozenTestDriver) PresignPut(context.Context, string, string, time.Duration) (*storage.PresignResult, error) {
	return nil, storage.ErrPresignUnsupported
}

func frozenTestEnvironment(t *testing.T, media []byte) (*frozenTestDriver, string, *atomic.Int32) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}, &model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "freeze-owner", Status: 1}).Error)
	oldDB := model.DB
	oldDriver := storage.GetStorageDriver()
	oldClient := httpClient
	setting := system_setting.GetFetchSetting()
	oldSetting := *setting
	model.DB = db
	driver := &frozenTestDriver{objects: map[string][]byte{}}
	storage.SetGlobalDriver(driver)
	// Replace only the external HTTP boundary with a real local mutable server.
	// Production SSRF policy and dialer remain unchanged.
	setting.EnableSSRFProtection = false
	gets := &atomic.Int32{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gets.Add(1) == 1 {
			_, _ = w.Write(media)
		} else {
			_, _ = w.Write([]byte("changed-invalid-media"))
		}
	}))
	httpClient = server.Client()
	t.Cleanup(func() {
		server.Close()
		model.DB = oldDB
		storage.SetGlobalDriver(oldDriver)
		httpClient = oldClient
		*setting = oldSetting
		sqlDB, err := db.DB()
		require.NoError(t, err)
		assert.NoError(t, sqlDB.Close())
	})
	return driver, server.URL + "/mutable.mp4", gets
}

func frozenRealVideo(t *testing.T, seconds string, audio bool) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "original.mp4")
	args := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=24"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-c:a", "aac")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-t", seconds, "-c:v", "libx264", "-threads", "1", path)
	output, err := exec.Command("ffmpeg", args...).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
func frozenPublicURL(raw string) bool {
	return strings.HasPrefix(raw, "https://files.example/influencer-reference/frozen/")
}

func TestInfluencerVideoFreezesSingleDownloadOriginalBytes(t *testing.T) {
	original := frozenRealVideo(t, "4", true)
	driver, source, gets := frozenTestEnvironment(t, original)
	prepared, err := InfluencerVideoInspector(context.Background(), source, 41, frozenPublicURL)
	require.NoError(t, err)
	require.NotEmpty(t, prepared.URL)
	assert.NotEqual(t, source, prepared.URL)
	assert.Equal(t, int32(1), gets.Load())
	assert.Equal(t, original, driver.objects[strings.TrimPrefix(prepared.URL, "https://files.example/")])
	digest := sha256.Sum256(original)
	assert.Equal(t, fmt.Sprintf("%x", digest), prepared.SHA256)
	assert.Len(t, driver.objects, 1, "the frozen video is the only stored reference; no separate audio attachment")
	var records []model.File
	require.NoError(t, model.DB.Find(&records).Error)
	require.Len(t, records, 1)
	for _, record := range records {
		assert.Equal(t, 41, record.UserId)
		assert.Equal(t, model.FileStatusActive, record.Status)
		assert.Equal(t, model.DefaultFileExpireDuration, record.ExpiresAt.Sub(record.CreatedAt))
	}
	prepared.Discard()
	assert.Empty(t, driver.objects)
	var count int64
	require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestInfluencerVideoPreparationReclaimsFailures(t *testing.T) {
	original := frozenRealVideo(t, "4", false)
	for _, failure := range []string{"partial-upload", "wrong-url", "cancel-upload", "record-failure", "confirm-failure", "invalid-video", "unsafe-url", "user-zero"} {
		t.Run(failure, func(t *testing.T) {
			data := original
			if failure == "invalid-video" {
				data = []byte("not a video")
			}
			driver, source, gets := frozenTestEnvironment(t, data)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			validation := frozenPublicURL
			userID := 41
			switch failure {
			case "partial-upload":
				driver.failUpload = true
			case "wrong-url":
				driver.mismatch = true
			case "cancel-upload":
				driver.onUpload = func(context.Context) { cancel() }
			case "record-failure":
				require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("fail", func(tx *gorm.DB) { tx.AddError(errors.New("record failure")) }))
			case "confirm-failure":
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("fail-confirm", func(tx *gorm.DB) { tx.AddError(errors.New("confirm failure")) }))
			case "unsafe-url":
				validation = func(string) bool { return false }
			case "user-zero":
				userID = 0
			}
			prepared, err := InfluencerVideoInspector(ctx, source, userID, validation)
			require.Error(t, err)
			assert.Empty(t, prepared.URL)
			assert.Empty(t, driver.objects)
			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.Zero(t, count)
			if failure == "user-zero" {
				assert.Zero(t, gets.Load())
			} else {
				assert.Equal(t, int32(1), gets.Load())
			}
			if failure == "cancel-upload" {
				assert.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func TestInfluencerVideoFailedDeleteRetainsExpiryRecord(t *testing.T) {
	driver, source, _ := frozenTestEnvironment(t, frozenRealVideo(t, "4", false))
	driver.failUpload = true
	driver.failDelete = true
	_, err := InfluencerVideoInspector(context.Background(), source, 41, frozenPublicURL)
	require.Error(t, err)
	require.Len(t, driver.objects, 1)
	var record model.File
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, model.FileStatusUploadPending, record.Status)
	assert.Equal(t, model.DefaultFileExpireDuration, record.ExpiresAt.Sub(record.CreatedAt))
	// No permanently orphaned object: the existing expiration sweep can retry it.
	require.NoError(t, model.DB.Model(&record).Update("expires_at", time.Now().Add(-time.Second)).Error)
	expired, err := model.GetExpiredFiles(10)
	require.NoError(t, err)
	require.Len(t, expired, 1)
}

// A reference longer than the retired 15-second audio window is still frozen as
// a single original object: its own audio track travels inside the video.
func TestInfluencerVideoLongReferenceStoresOnlyFrozenVideo(t *testing.T) {
	original := frozenRealVideo(t, "16", true)
	driver, source, gets := frozenTestEnvironment(t, original)
	prepared, err := InfluencerVideoInspector(context.Background(), source, 41, frozenPublicURL)
	require.NoError(t, err)
	assert.Len(t, driver.objects, 1)
	assert.Equal(t, original, driver.objects[strings.TrimPrefix(prepared.URL, "https://files.example/")])
	assert.Equal(t, int32(1), gets.Load())
	prepared.Discard()
}
