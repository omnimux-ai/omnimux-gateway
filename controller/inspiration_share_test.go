package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The unified share entry: one payload, two sources. These tests pin the source
// resolution and the tier rule that the convergence introduced — in particular
// that a cloud row is permanent for every caller, while a local upload keeps the
// caller's own default.

func unifiedShare(t *testing.T, userId int, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/share", func(c *gin.Context) {
		c.Set("id", userId)
		InspirationShare(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/share", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type unifiedShareResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ShareId       string  `json:"share_id"`
		ShareUrl      string  `json:"share_url"`
		Source        string  `json:"source"`
		Permanent     bool    `json:"permanent"`
		Expire        string  `json:"expire"`
		ExpireAt      *string `json:"expire_at"`
		ExpiresIn     string  `json:"expires_in"`
		StorageBucket string  `json:"storage_bucket"`
		IsAdmin       bool    `json:"is_admin"`
	} `json:"data"`
}

func decodeUnifiedShare(t *testing.T, w *httptest.ResponseRecorder) unifiedShareResponse {
	t.Helper()
	var res unifiedShareResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	return res
}

func shareRowCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.InspirationShare{}).Count(&count).Error)
	return count
}

// A cloud row lives in the inspiration library, which owns and serves it; the
// link therefore must not expire for an ordinary caller either.
func TestUnifiedShare_CloudSourceIsPermanentForOrdinaryUser(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := unifiedShare(t, 7, `{"inspiration_id":"2789"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUnifiedShare(t, w)
	assert.Equal(t, model.InspirationShareSourceCloud, res.Data.Source)
	assert.True(t, res.Data.Permanent)
	assert.Equal(t, "forever", res.Data.Expire)
	assert.Equal(t, "permanent", res.Data.ExpiresIn)
	assert.Equal(t, model.BucketOmnimuxInspiration, res.Data.StorageBucket)
	assert.False(t, res.Data.IsAdmin, "is_admin reports the publisher, not the tier")
	assert.Nil(t, res.Data.ExpireAt)
	assert.Contains(t, res.Data.ShareUrl, "/s/"+res.Data.ShareId)

	stored, err := model.GetInspirationShareByShareId(res.Data.ShareId)
	require.NoError(t, err)
	assert.Nil(t, stored.ExpiresAt)
	assert.Equal(t, model.InspirationShareSourceCloud, stored.Source)
}

// The source may be inferred, and `id` stays a valid spelling of the row id.
func TestUnifiedShare_CloudSourceIsInferredFromTheIdAlias(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := unifiedShare(t, 7, `{"id":"2789"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, model.InspirationShareSourceCloud, decodeUnifiedShare(t, w).Data.Source)
}

func TestUnifiedShare_LocalSourceKeepsTheProtectedWindow(t *testing.T) {
	setupAdminTierTestDB(t)

	w := unifiedShare(t, 7, `{"category":"街景","title":"t","prompt":"p"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUnifiedShare(t, w)
	assert.Equal(t, model.InspirationShareSourceLocal, res.Data.Source)
	assert.False(t, res.Data.Permanent)
	assert.Equal(t, "3days", res.Data.Expire)
	assert.Equal(t, "72h", res.Data.ExpiresIn)
	assert.Equal(t, model.BucketOmnimuxFiles, res.Data.StorageBucket)
	require.NotNil(t, res.Data.ExpireAt)
}

func TestUnifiedShare_LocalSourceIsPermanentForAdministrator(t *testing.T) {
	setupAdminTierTestDB(t)

	w := unifiedShare(t, 1, `{"category":"街景","title":"t","prompt":"p"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUnifiedShare(t, w)
	assert.Equal(t, model.InspirationShareSourceLocal, res.Data.Source)
	assert.True(t, res.Data.IsAdmin)
	assert.True(t, res.Data.Permanent)
	assert.Equal(t, model.BucketOmnimuxInspiration, res.Data.StorageBucket)
}

// The keyword still wins over the source default, in both directions.
func TestUnifiedShare_CloudSourceMayOptDownToTheProtectedWindow(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	w := unifiedShare(t, 7, `{"inspiration_id":"2789","expire":"3days"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUnifiedShare(t, w)
	assert.False(t, res.Data.Permanent)
	assert.Equal(t, "3days", res.Data.Expire)
	assert.Equal(t, model.BucketOmnimuxFiles, res.Data.StorageBucket)
}

// An explicit source wins over inference: a payload that carries local data and
// an unrelated id must not silently become a cloud copy.
func TestUnifiedShare_ExplicitLocalSourceIgnoresTheId(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream must not be called for a local share", http.StatusInternalServerError)
	})

	w := unifiedShare(t, 7, `{"source":"local","inspiration_id":"2789","category":"街景","title":"t","prompt":"p"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, model.InspirationShareSourceLocal, decodeUnifiedShare(t, w).Data.Source)
}

func TestUnifiedShare_RejectsUnknownSourceAndCloudWithoutId(t *testing.T) {
	setupAdminTierTestDB(t)
	before := shareRowCount(t)

	unknown := unifiedShare(t, 7, `{"source":"remote"}`)
	assert.Equal(t, http.StatusBadRequest, unknown.Code)
	assert.Contains(t, unknown.Body.String(), "source 仅支持")

	noId := unifiedShare(t, 7, `{"source":"cloud"}`)
	assert.Equal(t, http.StatusBadRequest, noId.Code)
	assert.Contains(t, noId.Body.String(), "云端分享必须提供灵感标识")

	assert.Equal(t, before, shareRowCount(t), "a rejected request writes no row")
}

func TestUnifiedShare_ForeverIsRefusedForAnOrdinaryLocalUpload(t *testing.T) {
	setupAdminTierTestDB(t)

	w := unifiedShare(t, 7, `{"category":"街景","title":"t","prompt":"p","expire":"forever"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "永久有效分享链接仅限管理员可用")
	assert.Zero(t, shareRowCount(t))
}

// A cloud id that the library does not know is a hard failure, never a link.
func TestUnifiedShare_MissingCloudRowIsReportedAndWritesNothing(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	w := unifiedShare(t, 7, `{"inspiration_id":"404404"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "灵感资源不存在")
	assert.Zero(t, shareRowCount(t))
}

// Replaying an existing share id answers with that share and its real tier, so a
// temporary link is never replayed as a permanent one.
func TestUnifiedShare_ReplayReportsTheStoredTier(t *testing.T) {
	setupAdminTierTestDB(t)

	existing := model.InspirationShare{
		UserId:    7,
		Category:  "街景",
		Title:     "t",
		Prompt:    "p",
		MediaType: "video",
	}
	require.NoError(t, model.CreateInspirationShare(&existing))
	require.NotNil(t, existing.ExpiresAt)

	w := unifiedShare(t, 7, `{"inspiration_id":"`+existing.ShareId+`","expire":"forever"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	res := decodeUnifiedShare(t, w)
	assert.Equal(t, existing.ShareId, res.Data.ShareId)
	assert.False(t, res.Data.Permanent, "the stored record is the authority")
	assert.Equal(t, "3days", res.Data.Expire)
}

// The expired window is still an access decision rather than a delete: the row
// stays, the link stops resolving.
func TestInspirationGetShare_ExpiredRowStopsResolving(t *testing.T) {
	setupAdminTierTestDB(t)

	past := time.Now().Add(-time.Hour)
	expired := model.InspirationShare{
		UserId:    7,
		Category:  "街景",
		Title:     "t",
		Prompt:    "p",
		MediaType: "video",
	}
	require.NoError(t, model.CreateInspirationShare(&expired))
	require.NoError(t, model.DB.Model(&model.InspirationShare{}).
		Where("id = ?", expired.Id).Update("expires_at", past).Error)

	_, err := model.GetInspirationShareByShareId(expired.ShareId)
	assert.Error(t, err)
	assert.Equal(t, int64(1), shareRowCount(t), "expiry hides the link, it does not delete the row")
}
