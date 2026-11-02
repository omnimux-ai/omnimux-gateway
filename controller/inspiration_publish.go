package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// fork: inspiration-share-convergence
//
// The local half of the share capability, plus the public read that opens a
// share. `InspirationPublish` stays registered as a compatibility alias for a
// caller that already ships the flat payload; the documented entry is
// `POST /api/inspiration/v1/share`.

// InspirationPublishRequest is the legacy flat payload of `/publish`. The
// unified payload is a strict superset of it, so one type keeps both spellings
// valid without a second implementation.
type InspirationPublishRequest = InspirationShareRequest

// fork: inspiration-publish
// inspirationRequesterIsAdmin resolves the caller's administrator status from
// the account record. TokenAuth (gateway API keys) fills the context id but
// never the context role, so trusting the context alone reports every API-key
// caller as an ordinary user — including the administrator who owns the key.
func inspirationRequesterIsAdmin(c *gin.Context, userId int) bool {
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	return model.IsAdmin(userId)
}

// InspirationPublish handles POST /api/inspiration/v1/publish
// Compatibility alias for the unified share entry, kept for callers that
// already ship the flat payload. Ordinary users publish with the 72-hour
// protection window, administrators publish permanently.
func InspirationPublish(c *gin.Context) {
	var req InspirationShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "请求数据格式错误 (invalid JSON payload)")
		return
	}
	inspirationShareFromLocal(c, &req)
}

// InspirationGetShare handles GET /api/inspiration/v1/share/:id
// Publicly accessible via direct link without login.
// Returns the inspiration details shaped for PromptPreviewModal.
func InspirationGetShare(c *gin.Context) {
	shareId := strings.TrimSpace(c.Param("id"))
	if shareId == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "缺少分享标识 (missing share_id)")
		return
	}

	share, err := model.GetInspirationShareByShareId(shareId)
	if err != nil {
		inspirationJSONError(c, http.StatusNotFound, "NOT_FOUND", "灵感资源不存在或已超过3天保护期失效 (inspiration share not found or expired)")
		return
	}

	source := share.Source
	if source == "" {
		source = inspirationShareSourceLocal
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":                share.ShareId,
			"slug":              share.ShareId,
			"modelKey":          share.Model,
			"modelLabel":        share.Model,
			"mediaType":         share.MediaType,
			"title":             share.Title,
			"description":       share.Description,
			"prompt":            share.Prompt,
			"category":          share.Category,
			"previewImageUrl":   share.CoverUrl,
			"previewVideoUrl":   share.MediaUrl,
			"posterUrl":         share.CoverUrl,
			"storage_bucket":    share.StorageBucket,
			"source":            source,
			"expires_at":        share.ExpiresAt,
			"created_at":        share.CreatedAt,
			"is_admin":          share.IsAdmin,
			"authorName":        "OmniMux Creator",
			"aspectRatio":       "16 / 9",
			"useCases":          []string{share.Category},
			"styles":            []string{},
			"tags":              []string{share.Category},
			"sourcePlatform":    "omnimux",
			"sourceUrl":         "",
			"viewCount":         1,
			"originalViewCount": 1,
		},
	})
}
