package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fork: influencer-preset-publish
//
// These tests drive the real handlers over a real Gin engine, with the session
// identity set directly: the auth middleware that produces it is covered by its
// own suite, and what matters here is that a published character keeps its
// parameters and its bytes.

const (
	influencerPresetTestUser  = 7
	influencerPresetTestOther = 99
)

type influencerPresetTestObject struct {
	body        []byte
	contentType string
}

// influencerPresetTestDriver is an in-memory StorageDriver so the assertions can
// see exactly which objects a publish or a withdrawal created and removed.
type influencerPresetTestDriver struct {
	objects   map[string]influencerPresetTestObject
	deleted   []string
	uploadErr error
}

func newInfluencerPresetTestDriver() *influencerPresetTestDriver {
	return &influencerPresetTestDriver{objects: map[string]influencerPresetTestObject{}}
}

func (d *influencerPresetTestDriver) Upload(_ context.Context, key string, data []byte, contentType string) (string, error) {
	if d.uploadErr != nil {
		return "", d.uploadErr
	}
	d.objects[key] = influencerPresetTestObject{body: data, contentType: contentType}
	return d.GetPublicURL(key), nil
}

func (d *influencerPresetTestDriver) Get(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	object, ok := d.objects[key]
	if !ok {
		return nil, "", 0, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(object.body)), object.contentType, int64(len(object.body)), nil
}

func (d *influencerPresetTestDriver) Delete(_ context.Context, key string) error {
	delete(d.objects, key)
	d.deleted = append(d.deleted, key)
	return nil
}

func (d *influencerPresetTestDriver) GetPublicURL(key string) string {
	return "https://cdn.example/" + strings.TrimPrefix(key, "/")
}

func (d *influencerPresetTestDriver) IsR2() bool { return false }

func (d *influencerPresetTestDriver) PresignPut(context.Context, string, string, time.Duration) (*storage.PresignResult, error) {
	return nil, storage.ErrPresignUnsupported
}

// setupInfluencerPresetTest points the model layer at a fresh in-memory database
// and the storage layer at a fresh in-memory driver.
func setupInfluencerPresetTest(t *testing.T) *influencerPresetTestDriver {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousTaskSchedule := taskImageThumbnailSchedule
	taskImageThumbnailSchedule = func(*model.Task, string) bool { return false }
	t.Cleanup(func() { taskImageThumbnailSchedule = previousTaskSchedule })
	previousEnqueue := enqueueInfluencerPresetThumbnail
	enqueueInfluencerPresetThumbnail = func(int64) bool { return false }
	t.Cleanup(func() { enqueueInfluencerPresetThumbnail = previousEnqueue })

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.InfluencerPreset{}))
	t.Cleanup(func() { model.DB = previousDB })

	driver := newInfluencerPresetTestDriver()
	previousDriver := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previousDriver) })

	return driver
}

// influencerPresetTestEngine mounts the four handlers behind a fixed session
// identity, so each test can act as a specific account.
func influencerPresetTestEngine(userID int) *gin.Engine {
	engine := gin.New()
	asUser := func(c *gin.Context) { c.Set("id", userID) }
	engine.GET("/api/influencer/presets", asUser, ListInfluencerPresets)
	engine.POST("/api/influencer/presets", asUser, PublishInfluencerPreset)
	engine.DELETE("/api/influencer/presets/:id", asUser, DeleteInfluencerPreset)
	engine.GET("/api/influencer/presets/:id/asset/:role", InfluencerPresetAsset)
	return engine
}

// influencerPresetTestPNG encodes a PNG of the requested pixel size, so the
// stored dimensions are a real decode result rather than a guess.
func influencerPresetTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))))
	return buffer.Bytes()
}

func influencerPresetTestTask(t *testing.T, taskID string, userID int) *model.Task {
	t.Helper()
	task := &model.Task{
		TaskID:   taskID,
		UserId:   userID,
		Status:   model.TaskStatusSuccess,
		Action:   ActionInfluencerSheet,
		Platform: "openai",
		Properties: model.Properties{
			Input: `{"tier":"freak","selection":{"hair":["hair_bowl"],"age":["adult"]},"brief":"street portrait","seed":4242}`,
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

// publishRequest builds the multipart body the workbench sends.
func publishRequest(t *testing.T, fields map[string]string, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	for name, content := range files {
		part, err := writer.CreateFormFile(name, name+".png")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/influencer/presets", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func influencerPresetTestPublish(t *testing.T, engine *gin.Engine, fields map[string]string, files map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, publishRequest(t, fields, files))
	return recorder
}

func influencerPresetCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.InfluencerPreset{}).Count(&count).Error)
	return count
}

func TestInfluencerPresetPublishStoresParametersAndAssets(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	preview := influencerPresetTestPNG(t, 3, 5)
	sheet := influencerPresetTestPNG(t, 9, 4)
	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet",
		"name":    "街头银发",
	}, map[string][]byte{"preview": preview, "sheet": sheet})

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Id        string              `json:"id"`
			Name      string              `json:"name"`
			Tier      string              `json:"tier"`
			Selection map[string][]string `json:"selection"`
			Brief     string              `json:"brief"`
			Seed      int64               `json:"seed"`
			Published bool                `json:"published"`
			Preview   struct {
				Path   string `json:"path"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"preview"`
			Sheet struct {
				Path   string `json:"path"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"sheet"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))

	assert.True(t, response.Success)
	assert.Equal(t, "街头银发", response.Data.Name)
	assert.Equal(t, "freak", response.Data.Tier)
	assert.Equal(t, map[string][]string{"hair": {"hair_bowl"}, "age": {"adult"}}, response.Data.Selection)
	assert.Equal(t, "street portrait", response.Data.Brief)
	assert.Equal(t, int64(4242), response.Data.Seed)
	assert.True(t, response.Data.Published)
	assert.Equal(t, "/api/influencer/presets/1/asset/preview", response.Data.Preview.Path)
	assert.Equal(t, "/api/influencer/presets/1/asset/sheet", response.Data.Sheet.Path)
	assert.Equal(t, 3, response.Data.Preview.Width)
	assert.Equal(t, 5, response.Data.Preview.Height)
	assert.Equal(t, 9, response.Data.Sheet.Width)
	assert.Equal(t, 4, response.Data.Sheet.Height)

	// Both artworks were persisted under keys this row owns, with the sniffed
	// content type, and the row survives without the task's upstream URL.
	require.Len(t, driver.objects, 2)
	var stored model.InfluencerPreset
	require.NoError(t, model.DB.First(&stored).Error)
	assert.NotEqual(t, stored.PreviewKey, stored.SheetKey)
	assert.Equal(t, "image/png", stored.PreviewMime)
	assert.Contains(t, stored.PreviewKey, "influencer/presets/")
	assert.Equal(t, influencerPresetTestUser, stored.CreatedBy)
	assert.Equal(t, "task_sheet", stored.SourceTaskID)
	assert.Equal(t, preview, driver.objects[stored.PreviewKey].body)
	assert.Equal(t, sheet, driver.objects[stored.SheetKey].body)
}

func TestInfluencerPresetPublishReusesOneObjectWhenArtworkMatches(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	artwork := influencerPresetTestPNG(t, 4, 4)
	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet",
		"name":    "同一张图",
	}, map[string][]byte{"preview": artwork, "sheet": artwork})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var stored model.InfluencerPreset
	require.NoError(t, model.DB.First(&stored).Error)
	assert.Equal(t, stored.PreviewKey, stored.SheetKey)
	assert.Len(t, driver.objects, 1)
}

func TestInfluencerPresetPublishRejectsRecordsWithoutParameters(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	// A turnaround board is derived from its parent and carries no parameters of
	// its own, so it must not seed a library entry.
	board := &model.Task{
		TaskID: "task_board", UserId: influencerPresetTestUser,
		Status: model.TaskStatusSuccess, Action: ActionInfluencerSheet,
		Properties: model.Properties{Input: `{"kind":"multiview","parent_task_id":"task_sheet"}`},
	}
	require.NoError(t, model.DB.Create(board).Error)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_board",
		"name":    "设定板",
	}, map[string][]byte{"preview": influencerPresetTestPNG(t, 2, 2)})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_not_publishable")
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)
}

func TestInfluencerPresetPublishRejectsAnotherUsersRecord(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestOther)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet",
		"name":    "别人的形象",
	}, map[string][]byte{"preview": influencerPresetTestPNG(t, 2, 2)})

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_not_found")
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)
}

func TestInfluencerPresetPublishRejectsDuplicatePublication(t *testing.T) {
	setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	artwork := influencerPresetTestPNG(t, 2, 2)

	first := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "第一次",
	}, map[string][]byte{"preview": artwork})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	second := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "第二次",
	}, map[string][]byte{"preview": artwork})

	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Contains(t, second.Body.String(), "already_published")
	assert.Equal(t, int64(1), influencerPresetCount(t))
}

func TestInfluencerPresetPublishRejectsUnsupportedArtwork(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet",
		"name":    "不是图片",
	}, map[string][]byte{"preview": []byte("<html>not an image</html>")})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unsupported artwork type")
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)
}

func TestInfluencerPresetPublishRequiresNameAndTask(t *testing.T) {
	setupInfluencerPresetTest(t)
	// The task exists so the artwork check rejects the last case, rather than the
	// ownership lookup rejecting it first.
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	artwork := influencerPresetTestPNG(t, 2, 2)

	missingName := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet",
	}, map[string][]byte{"preview": artwork})
	assert.Equal(t, http.StatusBadRequest, missingName.Code)

	missingTask := influencerPresetTestPublish(t, engine, map[string]string{
		"name": "没有任务",
	}, map[string][]byte{"preview": artwork})
	assert.Equal(t, http.StatusBadRequest, missingTask.Code)

	missingArtwork := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "没有素材",
	}, nil)
	assert.Equal(t, http.StatusBadRequest, missingArtwork.Code)

	assert.Equal(t, int64(0), influencerPresetCount(t))
}

func TestInfluencerPresetPublishReportsStorageFailure(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	driver.uploadErr = errors.New("r2 unavailable")
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	recorder := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "存储故障",
	}, map[string][]byte{"preview": influencerPresetTestPNG(t, 2, 2)})

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "storage_failed")
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)
}

func TestInfluencerPresetListProjectsLibraryShape(t *testing.T) {
	setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_older", influencerPresetTestUser)
	influencerPresetTestTask(t, "task_newer", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	for _, taskID := range []string{"task_older", "task_newer"} {
		recorder := influencerPresetTestPublish(t, engine, map[string]string{
			"task_id": taskID, "name": taskID,
		}, map[string][]byte{"preview": influencerPresetTestPNG(t, 2, 2)})
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/influencer/presets", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				Id        string              `json:"id"`
				Name      string              `json:"name"`
				Selection map[string][]string `json:"selection"`
				Published bool                `json:"published"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data.Items, 2)
	// Newest first, and the identity is the published slug rather than the row id.
	assert.Equal(t, "pub-2", response.Data.Items[0].Id)
	assert.Equal(t, "task_newer", response.Data.Items[0].Name)
	assert.Equal(t, "pub-1", response.Data.Items[1].Id)
	assert.Equal(t, map[string][]string{"hair": {"hair_bowl"}, "age": {"adult"}}, response.Data.Items[0].Selection)
	assert.True(t, response.Data.Items[0].Published)
}

func TestInfluencerPresetDeleteRemovesRowAndObjects(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	published := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "待下架",
	}, map[string][]byte{
		"preview": influencerPresetTestPNG(t, 3, 3),
		"sheet":   influencerPresetTestPNG(t, 6, 2),
	})
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	require.Len(t, driver.objects, 2)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/influencer/presets/1", nil))

	assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)
	assert.Len(t, driver.deleted, 2)

	missing := httptest.NewRecorder()
	engine.ServeHTTP(missing, httptest.NewRequest(http.MethodDelete, "/api/influencer/presets/1", nil))
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestInfluencerPresetAssetServesStoredBytes(t *testing.T) {
	setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	artwork := influencerPresetTestPNG(t, 5, 7)

	published := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "看图",
	}, map[string][]byte{"preview": artwork})
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())

	for _, role := range []string{"preview", "sheet"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/influencer/presets/1/asset/"+role, nil))
		assert.Equal(t, http.StatusOK, recorder.Code, role)
		assert.Equal(t, artwork, recorder.Body.Bytes(), role)
		assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"), role)
		assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"), role)
		assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"), role)
	}

	unknownRole := httptest.NewRecorder()
	engine.ServeHTTP(unknownRole, httptest.NewRequest(http.MethodGet, "/api/influencer/presets/1/asset/thumb", nil))
	assert.Equal(t, http.StatusNotFound, unknownRole.Code)

	unknownId := httptest.NewRecorder()
	engine.ServeHTTP(unknownId, httptest.NewRequest(http.MethodGet, "/api/influencer/presets/42/asset/preview", nil))
	assert.Equal(t, http.StatusNotFound, unknownId.Code)
}

// The grid hands the client the identifier it must feed back ("pub-<row>"), so
// the withdraw and asset routes accept that form as well as the bare row id.
func TestInfluencerPresetRoutesAcceptThePublicIdentifier(t *testing.T) {
	driver := setupInfluencerPresetTest(t)
	influencerPresetTestTask(t, "task_sheet", influencerPresetTestUser)
	engine := influencerPresetTestEngine(influencerPresetTestUser)

	published := influencerPresetTestPublish(t, engine, map[string]string{
		"task_id": "task_sheet", "name": "公开编号",
	}, map[string][]byte{
		"preview": influencerPresetTestPNG(t, 4, 4),
	})
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())

	asset := httptest.NewRecorder()
	engine.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/api/influencer/presets/pub-1/asset/preview", nil))
	require.Equal(t, http.StatusOK, asset.Code, asset.Body.String())
	assert.Equal(t, "image/png", asset.Header().Get("Content-Type"))

	removed := httptest.NewRecorder()
	engine.ServeHTTP(removed, httptest.NewRequest(http.MethodDelete, "/api/influencer/presets/pub-1", nil))
	assert.Equal(t, http.StatusOK, removed.Code, removed.Body.String())
	assert.Equal(t, int64(0), influencerPresetCount(t))
	assert.Empty(t, driver.objects)

	invalid := httptest.NewRecorder()
	engine.ServeHTTP(invalid, httptest.NewRequest(http.MethodDelete, "/api/influencer/presets/pub-", nil))
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}
