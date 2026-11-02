package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	multiViewTestUserID   = 100
	multiViewTestParentID = "task-parent"
	multiViewSheetMeta    = `{"tier":"normal","selection":{"gender":["female"]},"brief":"","size":"1024x1536"}`
	multiViewChildMeta    = `{"kind":"multiview","parent_task_id":"task-grandparent","size":"16:9","image_url":"https://omnimux.ai/x"}`
	multiViewParentImage  = "https://omnimux.ai/v1/tasks/task-parent/artifacts/image/content?access=abc"
)

// multiViewTestTask seeds one task row with the fields the handler inspects.
func multiViewTestTask(taskID string, userID int, action string, status model.TaskStatus, meta string) *model.Task {
	return &model.Task{
		TaskID:     taskID,
		UserId:     userID,
		Action:     action,
		Status:     status,
		Properties: model.Properties{Input: meta},
	}
}

// multiViewTestEngine mounts the handler behind an in-memory task table and
// injects the authenticated user the handler reads from the context.
func multiViewTestEngine(t *testing.T, seeded ...*model.Task) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	for _, task := range seeded {
		require.NoError(t, db.Create(task).Error)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/pg/influencer/multiview", func(c *gin.Context) {
		c.Set("id", multiViewTestUserID)
		c.Set("role", common.RoleCommonUser)
		PostInfluencerMultiView(c)
	})
	return engine, db
}

func postMultiView(t *testing.T, engine *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// A rejected request must never reach the relay chain, so it must leave the task
// table exactly as seeded — that is the observable form of "nothing was billed".
func TestPostInfluencerMultiViewRejectsInvalidRequestsWithoutCreatingTasks(t *testing.T) {
	validParent := multiViewTestTask(multiViewTestParentID, multiViewTestUserID, ActionInfluencerSheet, model.TaskStatusSuccess, multiViewSheetMeta)
	otherUserParent := multiViewTestTask("task-other", 999, ActionInfluencerSheet, model.TaskStatusSuccess, multiViewSheetMeta)
	notASheet := multiViewTestTask("task-chat", multiViewTestUserID, "chat", model.TaskStatusSuccess, "")
	noMeta := multiViewTestTask("task-nometa", multiViewTestUserID, ActionInfluencerSheet, model.TaskStatusSuccess, "")
	child := multiViewTestTask("task-child", multiViewTestUserID, ActionInfluencerSheet, model.TaskStatusSuccess, multiViewChildMeta)
	running := multiViewTestTask("task-running", multiViewTestUserID, ActionInfluencerSheet, model.TaskStatusQueued, multiViewSheetMeta)

	for _, tc := range []struct {
		name       string
		parent     *model.Task
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{
			name:   "model is required",
			parent: validParent,
			body: map[string]any{
				"parent_task_id": multiViewTestParentID,
				"image_url":      multiViewParentImage,
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "parent_task_id is required",
			parent: validParent,
			body: map[string]any{
				"model":     "gpt-image-2.5",
				"image_url": multiViewParentImage,
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "image_url is required",
			parent: validParent,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": multiViewTestParentID,
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "unknown parent",
			parent: validParent,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-missing",
				"image_url":      "https://omnimux.ai/v1/tasks/task-missing/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "task_not_found",
		},
		{
			name:   "parent belongs to another user",
			parent: otherUserParent,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-other",
				"image_url":      "https://omnimux.ai/v1/tasks/task-other/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "task_not_found",
		},
		{
			name:   "parent is not a character sheet",
			parent: notASheet,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-chat",
				"image_url":      "https://omnimux.ai/v1/tasks/task-chat/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "parent carries no sheet parameters",
			parent: noMeta,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-nometa",
				"image_url":      "https://omnimux.ai/v1/tasks/task-nometa/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "a multi-view board cannot parent another",
			parent: child,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-child",
				"image_url":      "https://omnimux.ai/v1/tasks/task-child/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "parent has not finished",
			parent: running,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": "task-running",
				"image_url":      "https://omnimux.ai/v1/tasks/task-running/artifacts/image/content?access=abc",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:   "reference image must be the parent's own artifact",
			parent: validParent,
			body: map[string]any{
				"model":          "gpt-image-2.5",
				"parent_task_id": multiViewTestParentID,
				"image_url":      "https://cdn.example.com/somebody-elses.png",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, db := multiViewTestEngine(t, tc.parent)
			rec := postMultiView(t, engine, tc.body)

			assert.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tc.wantCode)

			var count int64
			require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
			assert.EqualValues(t, 1, count, "a rejected request must not create a task")
		})
	}
}

// A malformed body must be reported as a request error, not reach the relay.
func TestPostInfluencerMultiViewRejectsMalformedBody(t *testing.T) {
	engine, db := multiViewTestEngine(t)

	req := httptest.NewRequest(http.MethodPost, "/pg/influencer/multiview", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "invalid_request")

	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

// The reference image is pinned to the parent's artifact route, so a task id
// that merely shares a prefix must not satisfy the containment check.
func TestParentArtifactPathIsExact(t *testing.T) {
	assert.Equal(t, "/v1/tasks/task-parent/artifacts/", parentArtifactPath(multiViewTestParentID))
	assert.NotContains(t, "https://omnimux.ai/v1/tasks/task-parent-2/artifacts/image/content", parentArtifactPath(multiViewTestParentID))
}
