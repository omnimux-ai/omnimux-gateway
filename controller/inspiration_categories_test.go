package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspirationListCategories_ReturnsAllCategoriesAndRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/categories", InspirationListCategories)

	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var res struct {
		Success bool `json:"success"`
		Data    struct {
			Total  int                       `json:"total"`
			Groups []map[string]any          `json:"groups"`
			Items  []InspirationCategoryItem `json:"items"`
			Rules  struct {
				PublishRequiredFields []string `json:"publish_required_fields"`
				PublishOptionalFields []string `json:"publish_optional_fields"`
				Note                  string   `json:"note"`
			} `json:"rules"`
		} `json:"data"`
	}

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.True(t, res.Success)
	assert.GreaterOrEqual(t, res.Data.Total, 30)
	assert.Equal(t, res.Data.Total, len(res.Data.Items))
	assert.NotEmpty(t, res.Data.Groups)

	// Verify required and optional fields contracts
	assert.Equal(t, []string{"category", "title", "prompt"}, res.Data.Rules.PublishRequiredFields)
	assert.Contains(t, res.Data.Rules.PublishOptionalFields, "media")
	assert.Contains(t, res.Data.Rules.PublishOptionalFields, "cover")
	assert.Contains(t, res.Data.Rules.PublishOptionalFields, "model")
	assert.Contains(t, res.Data.Rules.PublishOptionalFields, "expire")

	// Verify an individual category item carries required/optional metadata
	first := res.Data.Items[0]
	assert.NotEmpty(t, first.Key)
	assert.NotEmpty(t, first.Name)
	assert.NotEmpty(t, first.NameEn)
	assert.NotEmpty(t, first.Group)
	assert.NotEmpty(t, first.GroupKey)
	assert.Equal(t, []string{"category", "title", "prompt"}, first.RequiredFields)
}

func TestInspirationListCategories_FiltersByGroupKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/categories", InspirationListCategories)

	// Filter by ecommerce group
	req := httptest.NewRequest(http.MethodGet, "/categories?group=ecommerce", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var res struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                       `json:"total"`
			Items []InspirationCategoryItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.True(t, res.Success)
	assert.Greater(t, res.Data.Total, 0)
	for _, item := range res.Data.Items {
		assert.Equal(t, "ecommerce", item.GroupKey)
	}

	// Filter by unknown group returns 0 items
	reqUnknown := httptest.NewRequest(http.MethodGet, "/categories?group=unknown_domain", nil)
	wUnknown := httptest.NewRecorder()
	r.ServeHTTP(wUnknown, reqUnknown)

	var resUnknown struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wUnknown.Body.Bytes(), &resUnknown))
	assert.Equal(t, 0, resUnknown.Data.Total)
}
