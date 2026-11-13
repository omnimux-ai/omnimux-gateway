package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	system_setting "github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// fork: inspiration-share-convergence
//
// One share capability, two sources. `POST /api/inspiration/v1/share` is the
// single entry; `/publish` and `/inspirations/:id/share` stay registered as
// compatibility aliases onto the same implementation, so a caller that already
// ships either flat payload keeps working unchanged.
//
// A share either copies a row that already lives in the inspiration library
// ("cloud") or carries data the caller brings along ("local"). Both land in
// inspiration_shares and answer with one `/s/<share_id>` link that opens the
// inspiration full-screen preview.
//
// The tier follows the source as well as the caller:
//   - cloud: permanent. The row lives in the inspiration library and is served
//     from there, so the link does not depend on anything this gateway holds.
//   - local: 72h for ordinary users, permanent for administrators, and the
//     `expire` keyword may override either way.

const (
	inspirationShareSourceCloud = model.InspirationShareSourceCloud
	inspirationShareSourceLocal = model.InspirationShareSourceLocal
)

// InspirationShareRequest is the payload of the unified share entry. It is the
// superset of both legacy payloads: the `/publish` alias simply never carries a
// source, and the `/inspirations/:id/share` alias takes its id from the path.
type InspirationShareRequest struct {
	// Source is "cloud" or "local". Omitted means inferred from the presence of
	// an inspiration id, so the common case needs no extra field.
	Source string `json:"source"`

	// InspirationId names a row in the inspiration library. `id` is accepted as
	// the spelling the share/replay path already used.
	InspirationId string `json:"inspiration_id"`
	Id            string `json:"id"`

	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model"`
	MediaType   string `json:"media_type"`
	MediaUrl    string `json:"media_url"`
	CoverUrl    string `json:"cover_url"`

	// Expire selects the tier: empty keeps the source's own default, "forever"
	// asks for permanent access, "3days" opts down to the protected window.
	Expire string `json:"expire"`
}

// upstreamId picks the cloud inspiration id out of either spelling; an empty
// result means the payload describes local data.
func (req *InspirationShareRequest) upstreamId() string {
	if id := strings.TrimSpace(req.InspirationId); id != "" {
		return id
	}
	return strings.TrimSpace(req.Id)
}

// inspirationResolveSource decides where the share's data comes from and
// reports a bad request instead of guessing when the payload is ambiguous.
func inspirationResolveSource(c *gin.Context, req *InspirationShareRequest) (source string, upstreamId string, ok bool) {
	upstreamId = req.upstreamId()
	switch strings.ToLower(strings.TrimSpace(req.Source)) {
	case "":
		if upstreamId != "" {
			return inspirationShareSourceCloud, upstreamId, true
		}
		return inspirationShareSourceLocal, "", true
	case inspirationShareSourceCloud:
		if upstreamId == "" {
			inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "云端分享必须提供灵感标识 (inspiration_id is required for a cloud share)")
			return "", "", false
		}
		return inspirationShareSourceCloud, upstreamId, true
	case inspirationShareSourceLocal:
		return inspirationShareSourceLocal, "", true
	default:
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "source 仅支持 cloud 或 local (source must be cloud or local)")
		return "", "", false
	}
}

// inspirationResolveShareTier maps the source, the caller's identity and the
// `expire` keyword onto the tier. Someone without the right to a permanent link
// who asks for one is refused instead of silently downgraded, so nobody ends up
// believing a temporary link is permanent.
func inspirationResolveShareTier(c *gin.Context, source string, isAdmin bool, expire string) (permanent bool, ok bool) {
	cloud := source == inspirationShareSourceCloud
	switch strings.ToLower(strings.TrimSpace(expire)) {
	case "", "default":
		return cloud || isAdmin, true
	case "forever", "permanent":
		if !cloud && !isAdmin {
			inspirationJSONError(c, http.StatusForbidden, "FORBIDDEN", "永久有效分享链接仅限管理员可用，普通用户请选择3天有效期")
			return false, false
		}
		return true, true
	case "3days", "3d", "72h":
		return false, true
	default:
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "expire 仅支持 3days 或 forever (expire must be 3days or forever)")
		return false, false
	}
}

// InspirationShare POST /api/inspiration/v1/share
// Accessible by authenticated users via session or API token.
func InspirationShare(c *gin.Context) {
	var req InspirationShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "请求数据格式错误 (invalid JSON payload)")
		return
	}

	source, upstreamId, ok := inspirationResolveSource(c, &req)
	if !ok {
		return
	}
	if source == inspirationShareSourceCloud {
		inspirationShareFromCloud(c, upstreamId, req.Expire)
		return
	}
	inspirationShareFromLocal(c, &req)
}

// inspirationShareFromLocal stores data the caller brought along: category and
// the core fields are mandatory, and the tier is the caller's own default
// unless the keyword says otherwise.
func inspirationShareFromLocal(c *gin.Context, req *InspirationShareRequest) {
	userId := c.GetInt("id")
	if userId <= 0 {
		inspirationJSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录或提供有效凭证 (authentication required)")
		return
	}

	isAdmin := inspirationRequesterIsAdmin(c, userId)
	permanent, ok := inspirationResolveShareTier(c, inspirationShareSourceLocal, isAdmin, req.Expire)
	if !ok {
		return
	}

	category := strings.TrimSpace(req.Category)
	if category == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "必须包含灵感分类才能发布 (category is required)")
		return
	}

	title := strings.TrimSpace(req.Title)
	prompt := strings.TrimSpace(req.Prompt)
	if title == "" || prompt == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "必须包含灵感核心数据(标题及提示词)才能发布 (title and prompt are required)")
		return
	}

	mediaType := strings.TrimSpace(req.MediaType)
	if mediaType == "" {
		mediaType = "video"
	}

	mediaUrl := strings.TrimSpace(req.MediaUrl)
	coverUrl := strings.TrimSpace(req.CoverUrl)
	if coverUrl == "" && mediaUrl != "" && (strings.HasSuffix(strings.ToLower(mediaUrl), ".jpg") || strings.HasSuffix(strings.ToLower(mediaUrl), ".png") || strings.HasSuffix(strings.ToLower(mediaUrl), ".webp")) {
		coverUrl = mediaUrl
	}

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = "Universal"
	}

	share := model.InspirationShare{
		UserId:      userId,
		IsAdmin:     isAdmin,
		Source:      inspirationShareSourceLocal,
		Permanent:   permanent,
		Category:    category,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Prompt:      prompt,
		Model:       modelName,
		MediaType:   mediaType,
		MediaUrl:    mediaUrl,
		CoverUrl:    coverUrl,
	}

	if err := model.CreateInspirationShare(&share); err != nil {
		inspirationJSONError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "发布灵感失败: "+err.Error())
		return
	}

	inspirationShareRespond(c, &share, "灵感发布成功")
}

// inspirationShareRespond writes the payload both entries answer with: the
// share_url plus the tier and source the stored record actually carries. The
// record is the authority, so an idempotent replay reports the real tier of the
// existing share rather than whatever the new request asked for.
func inspirationShareRespond(c *gin.Context, share *model.InspirationShare, message string) {
	baseURL := system_setting.ServerAddress
	if baseURL == "" {
		baseURL = "https://omnimux.ai"
	}
	shareURL := fmt.Sprintf("%s/s/%s", strings.TrimRight(baseURL, "/"), share.ShareId)

	permanent := share.ExpiresAt == nil
	expireMode := "3days"
	expiresDesc := "72h"
	if permanent {
		expireMode = "forever"
		expiresDesc = "permanent"
	}

	source := share.Source
	if source == "" {
		source = inspirationShareSourceLocal
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"id":             share.ShareId,
			"share_id":       share.ShareId,
			"share_url":      shareURL,
			"source":         source,
			"permanent":      permanent,
			"expire":         expireMode,
			"expire_at":      share.ExpiresAt,
			"expires_at":     share.ExpiresAt,
			"expires_in":     expiresDesc,
			"storage_bucket": share.StorageBucket,
			"is_admin":       share.IsAdmin,
			"category":       share.Category,
			"title":          share.Title,
			"created_at":     share.CreatedAt,
		},
	})
}
