package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.InspirationShare{}))
}

func TestInspirationPublish_ValidationFailures(t *testing.T) {
	setupTestDB(t)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/publish", func(c *gin.Context) {
		c.Set("id", 100)
		c.Set("role", common.RoleCommonUser)
		InspirationPublish(c)
	})

	// Case 1: Missing category
	{
		body, _ := json.Marshal(map[string]any{
			"title":  "Test Title",
			"prompt": "Test Prompt",
		})
		req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "必须包含灵感分类才能发布")
	}

	// Case 2: Missing title
	{
		body, _ := json.Marshal(map[string]any{
			"category": "街景",
			"prompt":   "Test Prompt",
		})
		req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "必须包含灵感核心数据")
	}

	// Case 3: Missing prompt
	{
		body, _ := json.Marshal(map[string]any{
			"category": "街景",
			"title":    "Test Title",
		})
		req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "必须包含灵感核心数据")
	}
}

func TestInspirationPublish_UserStorageTieringAndShareAccess(t *testing.T) {
	setupTestDB(t)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/publish/user", func(c *gin.Context) {
		c.Set("id", 100)
		c.Set("role", common.RoleCommonUser)
		InspirationPublish(c)
	})
	r.POST("/publish/admin", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", common.RoleAdminUser)
		InspirationPublish(c)
	})
	r.GET("/share/:id", InspirationGetShare)

	// 1. Common user publish -> omnimux-files, 72h protection
	var userShareId string
	{
		payload := map[string]any{
			"category":    "电影质感街景",
			"title":       "90年代曼哈顿街景",
			"prompt":      `{"format":"16:9","style":"cinematic"}`,
			"model":       "Seedance 2.5",
			"media_url":   "https://example.com/manhattan.mp4",
			"description": "30秒电影感视频",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/publish/user", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res struct {
			Success bool `json:"success"`
			Data    struct {
				ShareId       string     `json:"share_id"`
				ShareUrl      string     `json:"share_url"`
				StorageBucket string     `json:"storage_bucket"`
				IsAdmin       bool       `json:"is_admin"`
				ExpiresAt     *time.Time `json:"expires_at"`
				ExpiresIn     string     `json:"expires_in"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.True(t, res.Success)
		assert.NotEmpty(t, res.Data.ShareId)
		assert.Contains(t, res.Data.ShareUrl, "/s/"+res.Data.ShareId)
		assert.Equal(t, model.BucketOmnimuxFiles, res.Data.StorageBucket)
		assert.False(t, res.Data.IsAdmin)
		assert.Equal(t, "72h", res.Data.ExpiresIn)
		require.NotNil(t, res.Data.ExpiresAt)
		assert.True(t, res.Data.ExpiresAt.After(time.Now()))

		userShareId = res.Data.ShareId
	}

	// 2. Admin publish -> omnimux-inspiration, permanent
	var adminShareId string
	{
		payload := map[string]any{
			"category":  "官方标杆案例",
			"title":     "官方精选电影镜头",
			"prompt":    `{"camera":"dolly zoom"}`,
			"model":     "Seedance 2.5",
			"media_url": "https://example.com/official.mp4",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/publish/admin", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res struct {
			Success bool `json:"success"`
			Data    struct {
				ShareId       string     `json:"share_id"`
				StorageBucket string     `json:"storage_bucket"`
				IsAdmin       bool       `json:"is_admin"`
				ExpiresAt     *time.Time `json:"expires_at"`
				ExpiresIn     string     `json:"expires_in"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.True(t, res.Success)
		assert.Equal(t, model.BucketOmnimuxInspiration, res.Data.StorageBucket)
		assert.True(t, res.Data.IsAdmin)
		assert.Equal(t, "permanent", res.Data.ExpiresIn)
		assert.Nil(t, res.Data.ExpiresAt)

		adminShareId = res.Data.ShareId
	}

	// 3. Public direct link access
	{
		req := httptest.NewRequest(http.MethodGet, "/share/"+userShareId, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "90年代曼哈顿街景")
		assert.Contains(t, w.Body.String(), model.BucketOmnimuxFiles)
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/share/"+adminShareId, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "官方精选电影镜头")
		assert.Contains(t, w.Body.String(), model.BucketOmnimuxInspiration)
	}

	// 4. Expired share returns 404
	{
		// Manually set userShare's expires_at to 1 hour ago
		past := time.Now().Add(-1 * time.Hour)
		require.NoError(t, model.DB.Model(&model.InspirationShare{}).
			Where("share_id = ?", userShareId).
			Update("expires_at", past).Error)

		req := httptest.NewRequest(http.MethodGet, "/share/"+userShareId, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "已超过3天保护期失效")
	}

	// 5. Non-existent share returns 404
	{
		req := httptest.NewRequest(http.MethodGet, "/share/insp_not_exist", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	}
}
