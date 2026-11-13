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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shareUpstreamFixture mirrors the sidecar's /v1/inspirations/:id payload.
const shareUpstreamFixture = `{
  "success": true,
  "data": {
    "id": 2789,
    "type": "image",
    "title": "情绪共鸣型助眠歌单推广",
    "content": "Give it a try",
    "source_url": "https://www.tiktok.com/@creator/video/gxgen-aa5d20e5-729e",
    "cover_key": "/media/inspiration-covers/2789",
    "media_keys": ["/media/r2/publications/x/slide-1.jpg"],
    "country_code": "DE",
    "category": "Health & Wellness",
    "analysis": {
      "creator": {"name": "creator", "handle": "creator"},
      "target_goal": "捕捉失眠群体的深夜孤独感",
      "hook_highlight": "528Hz 频率、快速入睡",
      "visual_breakdown": "低饱和度、深夜色调"
    }
  }
}`

func shareUpstream(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "test-token",
	})
	prev := setInspirationClientForTest(client)
	t.Cleanup(func() { setInspirationClientForTest(prev) })
}

func shareRecorder(t *testing.T, userId, role int, shareId string, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/inspirations/:id/share", func(c *gin.Context) {
		c.Set("id", userId)
		c.Set("role", role)
		InspirationCreateShare(c)
	})
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/inspirations/"+shareId+"/share", reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type shareResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ShareId   string  `json:"share_id"`
		ShareUrl  string  `json:"share_url"`
		Expire    string  `json:"expire"`
		ExpireAt  *string `json:"expire_at"`
		ExpiresIn string  `json:"expires_in"`
		Category  string  `json:"category"`
		Title     string  `json:"title"`
	} `json:"data"`
}

func TestInspirationCreateShare_RequiresAuth(t *testing.T) {
	setupTestDB(t)
	w := shareRecorder(t, 0, common.RoleCommonUser, "2789", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// A cloud row already lives in the library and is served from there, so the
// permanent tier is not an administrator privilege on this path: asking for it
// is what the source default already is. The administrator-only refusal still
// guards local uploads (TestUnifiedShare_ForeverIsRefusedForAnOrdinaryLocalUpload).
func TestInspirationCreateShare_ForeverIsAllowedForACloudRow(t *testing.T) {
	setupTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := shareRecorder(t, 7, common.RoleCommonUser, "2789", `{"expire":"forever"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res shareResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "forever", res.Data.Expire)
	assert.Equal(t, "permanent", res.Data.ExpiresIn)
	assert.Nil(t, res.Data.ExpireAt)
}

func TestInspirationCreateShare_CommunityInspirationBecomesLink(t *testing.T) {
	setupTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/inspirations/2789", r.URL.Path)
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := shareRecorder(t, 7, common.RoleCommonUser, "2789", `{"expire":"3days"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res shareResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.True(t, res.Success)
	assert.NotEmpty(t, res.Data.ShareId)
	assert.Contains(t, res.Data.ShareUrl, "/s/"+res.Data.ShareId)
	assert.Equal(t, "3days", res.Data.Expire)
	assert.Equal(t, "72h", res.Data.ExpiresIn)
	require.NotNil(t, res.Data.ExpireAt)
	assert.Equal(t, "Health & Wellness", res.Data.Category)
	assert.Equal(t, "情绪共鸣型助眠歌单推广", res.Data.Title)

	// The share is real: it is reachable through the public lookup.
	stored, err := model.GetInspirationShareByShareId(res.Data.ShareId)
	require.NoError(t, err)
	assert.Equal(t, model.BucketOmnimuxFiles, stored.StorageBucket)
	assert.Contains(t, stored.Prompt, "目标与转化意图")
	// Shared links are opened anonymously, so the copied media must point at the
	// public projection rather than the authenticated media route.
	assert.Equal(t, "/api/inspiration/v1/public/media/r2/publications/x/slide-1.jpg", stored.MediaUrl)
}

func TestInspirationCreateShare_AdminGetsPermanentLink(t *testing.T) {
	setupTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := shareRecorder(t, 1, common.RoleAdminUser, "2789", `{"expire":"forever"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res shareResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "forever", res.Data.Expire)
	assert.Equal(t, "permanent", res.Data.ExpiresIn)
	assert.Nil(t, res.Data.ExpireAt)

	stored, err := model.GetInspirationShareByShareId(res.Data.ShareId)
	require.NoError(t, err)
	assert.Equal(t, model.BucketOmnimuxInspiration, stored.StorageBucket)
	assert.Nil(t, stored.ExpiresAt)
}

func TestInspirationCreateShare_ExistingShareIdIsIdempotent(t *testing.T) {
	setupTestDB(t)
	// No upstream stub: an already-published share must not need the sidecar.
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream must not be called for an existing share id, got %s", r.URL.Path)
	})

	existing := model.InspirationShare{
		UserId:   7,
		Category: "电影质感街景",
		Title:    "已发布的灵感",
		Prompt:   "prompt body",
	}
	require.NoError(t, model.CreateInspirationShare(&existing))

	w := shareRecorder(t, 7, common.RoleCommonUser, existing.ShareId, `{"expire":"3days"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res shareResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, existing.ShareId, res.Data.ShareId)
	assert.Contains(t, res.Data.ShareUrl, "/s/"+existing.ShareId)
}

func TestInspirationCreateShare_UnknownInspirationIsNotFabricated(t *testing.T) {
	setupTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"code":"NOT_FOUND"}`))
	})

	w := shareRecorder(t, 7, common.RoleCommonUser, "99999999", `{"expire":"3days"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "/s/", "a failed share must never answer with a link")

	// Nothing was written for a lookup that failed.
	var count int64
	require.NoError(t, model.DB.Model(&model.InspirationShare{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestInspirationCreateShare_MissingCategoryIsRejected(t *testing.T) {
	setupTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"title":"无分类灵感","content":"x","analysis":{}}}`))
	})

	w := shareRecorder(t, 7, common.RoleCommonUser, "1", `{"expire":"3days"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "缺少分类数据")
}
