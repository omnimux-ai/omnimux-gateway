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

// Issue #252: a gateway API key authenticates through TokenAuth, which fills the
// context id but never the context role. Reading the role from the context alone
// therefore served the administrator who owns the key the ordinary 3-day tier.
// The account record has to be the authority.

func setupAdminTierTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.InspirationShare{}))
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "admin",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "affadmin",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       7,
		Username: "creator",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "affcreator",
	}).Error)
}

// publishViaApiKey mirrors TokenAuth: the id is set, the role is not.
func publishViaApiKey(t *testing.T, userId int, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/publish", func(c *gin.Context) {
		c.Set("id", userId)
		InspirationPublish(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type publishResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ShareId       string  `json:"share_id"`
		ShareUrl      string  `json:"share_url"`
		Expire        string  `json:"expire"`
		ExpireAt      *string `json:"expire_at"`
		ExpiresIn     string  `json:"expires_in"`
		StorageBucket string  `json:"storage_bucket"`
		IsAdmin       bool    `json:"is_admin"`
	} `json:"data"`
}

const adminTierBody = `{"category":"电影质感街景","title":"管理员发布","prompt":"prompt body","media_url":"https://example.com/a.mp4"}`

func TestPublishViaApiKey_AdminOwnerGetsPermanentTier(t *testing.T) {
	setupAdminTierTestDB(t)

	w := publishViaApiKey(t, 1, adminTierBody)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res publishResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.True(t, res.Data.IsAdmin, "the account role decides, not the missing context role")
	assert.Equal(t, "forever", res.Data.Expire)
	assert.Equal(t, "permanent", res.Data.ExpiresIn)
	assert.Equal(t, model.BucketOmnimuxInspiration, res.Data.StorageBucket)
	assert.Nil(t, res.Data.ExpireAt)

	stored, err := model.GetInspirationShareByShareId(res.Data.ShareId)
	require.NoError(t, err)
	assert.Nil(t, stored.ExpiresAt)
	assert.Equal(t, model.BucketOmnimuxInspiration, stored.StorageBucket)
}

func TestPublishViaApiKey_OrdinaryOwnerKeepsThreeDayTier(t *testing.T) {
	setupAdminTierTestDB(t)

	w := publishViaApiKey(t, 7, adminTierBody)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res publishResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.False(t, res.Data.IsAdmin)
	assert.Equal(t, "3days", res.Data.Expire)
	assert.Equal(t, "72h", res.Data.ExpiresIn)
	assert.Equal(t, model.BucketOmnimuxFiles, res.Data.StorageBucket)
	require.NotNil(t, res.Data.ExpireAt)
}

func TestPublishViaApiKey_ForeverIsRefusedForOrdinaryOwner(t *testing.T) {
	setupAdminTierTestDB(t)

	w := publishViaApiKey(t, 7, `{"category":"街景","title":"t","prompt":"p","expire":"forever"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "永久有效分享链接仅限管理员可用")

	var count int64
	require.NoError(t, model.DB.Model(&model.InspirationShare{}).Count(&count).Error)
	assert.Zero(t, count, "a refused publish must not write a row")
}

func TestPublishViaApiKey_AdminMayOptDownToThreeDays(t *testing.T) {
	setupAdminTierTestDB(t)

	w := publishViaApiKey(t, 1, `{"category":"街景","title":"t","prompt":"p","expire":"3days"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res publishResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "3days", res.Data.Expire)
	assert.Equal(t, model.BucketOmnimuxFiles, res.Data.StorageBucket)
}

func TestPublishViaApiKey_UnknownExpireKeywordIsRejected(t *testing.T) {
	setupAdminTierTestDB(t)

	w := publishViaApiKey(t, 1, `{"category":"街景","title":"t","prompt":"p","expire":"never"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "expire 仅支持")
}

func TestCreateShareViaApiKey_AdminOwnerGetsPermanentLink(t *testing.T) {
	setupAdminTierTestDB(t)
	shareUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(shareUpstreamFixture))
	})

	// shareUpstream shares the recorder helper with the publish tests; drive the
	// share endpoint directly so the TokenAuth shape (id only) is exercised.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/inspirations/:id/share", func(c *gin.Context) {
		c.Set("id", 1)
		InspirationCreateShare(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/inspirations/2789/share", bytes.NewReader([]byte(`{"expire":"forever"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res publishResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, "forever", res.Data.Expire)
	assert.Equal(t, "permanent", res.Data.ExpiresIn)
	assert.Equal(t, model.BucketOmnimuxInspiration, res.Data.StorageBucket)
}
