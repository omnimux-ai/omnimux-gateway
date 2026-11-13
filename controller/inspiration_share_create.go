package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// fork: inspiration-share-convergence
//
// The cloud half of the unified share capability: the row already lives in the
// inspiration library, so the share copies it into inspiration_shares and links
// to it. A cloud row is permanent by construction — the library owns the data
// and serves it, so the link does not depend on anything this gateway stores.
//
// The upstream lookup is what makes the inspiration reachable: the sidecar owns
// the row, this gateway owns the share. A missing upstream row is reported as a
// hard error rather than answered with a fabricated link, so a caller can never
// mistake a failed share for a working one.

// inspirationSharePromptLabels maps upstream analysis keys onto readable
// sections, in the order they are printed inside the share's prompt block.
var inspirationSharePromptLabels = []struct {
	Key   string
	Label string
}{
	{"prompt", "提示词"},
	{"prompt_text", "提示词"},
	{"target_goal", "目标与转化意图"},
	{"hook_highlight", "钩子与核心卖点"},
	{"visual_breakdown", "画面与视觉拆解"},
	{"narrative_strategy", "叙事与结构策略"},
}

// InspirationCreateShare POST /api/inspiration/v1/inspirations/:id/share
// Compatibility alias for the unified share entry: the path carries the
// inspiration id the unified payload would put in the body.
func InspirationCreateShare(c *gin.Context) {
	rawId := strings.TrimSpace(c.Param("id"))
	if rawId == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "缺少灵感标识 (missing inspiration id)")
		return
	}

	var body struct {
		Expire string `json:"expire"`
	}
	_ = c.ShouldBindJSON(&body)

	inspirationShareFromCloud(c, rawId, body.Expire)
}

// inspirationShareFromCloud copies a row that already lives in the inspiration
// library into inspiration_shares and answers with its direct link.
func inspirationShareFromCloud(c *gin.Context, rawId string, expire string) {
	userId := c.GetInt("id")
	if userId <= 0 {
		inspirationJSONError(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录或提供有效凭证 (authentication required)")
		return
	}

	isAdmin := inspirationRequesterIsAdmin(c, userId)
	permanent, ok := inspirationResolveShareTier(c, inspirationShareSourceCloud, isAdmin, expire)
	if !ok {
		return
	}

	// Idempotent: the caller already holds a share id, so hand back its link.
	if existing, err := model.GetInspirationShareByShareId(rawId); err == nil && existing != nil {
		inspirationShareRespond(c, existing, "分享链接已生成")
		return
	}

	// The id names a community inspiration: copy it across before linking.
	if !inspirationReady(c) {
		return
	}
	raw, status, err := inspirationUpstream.doJSON(c, http.MethodGet, "/v1/inspirations/"+url.PathEscape(rawId), nil, true)
	if err != nil {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UNAVAILABLE", "灵感服务暂时不可用，请稍后重试 (inspiration upstream unavailable)")
		return
	}
	if status == http.StatusNotFound {
		inspirationJSONError(c, http.StatusNotFound, "NOT_FOUND", "灵感资源不存在 (inspiration not found)")
		return
	}
	if status < 200 || status >= 300 {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "灵感服务返回异常，请稍后重试 (inspiration upstream error)")
		return
	}

	var envelope struct {
		Success bool                    `json:"success"`
		Data    inspirationUpstreamItem `json:"data"`
	}
	if err := common.Unmarshal(raw, &envelope); err != nil || !envelope.Success {
		inspirationJSONError(c, http.StatusServiceUnavailable, "INSPIRATION_UPSTREAM", "灵感服务返回数据无法解析 (inspiration upstream response invalid)")
		return
	}
	item := envelope.Data

	// Category and prompt are the share's mandatory data. The upstream row
	// supplies both, so a row missing them is reported instead of published
	// half-empty.
	category := strings.TrimSpace(item.Category)
	if category == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "该灵感缺少分类数据，无法生成分享链接 (inspiration has no category)")
		return
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = strings.TrimSpace(item.Content)
	}
	if title == "" {
		inspirationJSONError(c, http.StatusBadRequest, "INVALID_PARAM", "该灵感缺少标题数据，无法生成分享链接 (inspiration has no title)")
		return
	}
	prompt := inspirationSharePromptFromAnalysis(item)
	if prompt == "" {
		prompt = strings.TrimSpace(item.Content)
	}
	if prompt == "" {
		prompt = title
	}

	mediaType := strings.TrimSpace(item.Type)
	if mediaType == "" {
		mediaType = "video"
	}

	share := model.InspirationShare{
		UserId:      userId,
		IsAdmin:     isAdmin,
		Source:      inspirationShareSourceCloud,
		Permanent:   permanent,
		Category:    category,
		Title:       title,
		Description: strings.TrimSpace(item.Content),
		Prompt:      prompt,
		Model:       inspirationShareModelFromAnalysis(item),
		MediaType:   mediaType,
		MediaUrl:    inspirationShareFirstMediaURL(item.MediaKeys),
		CoverUrl:    inspirationShareCoverURL(item.CoverKey),
	}
	if err := model.CreateInspirationShare(&share); err != nil {
		inspirationJSONError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "生成分享链接失败: "+err.Error())
		return
	}

	inspirationShareRespond(c, &share, "分享链接已生成")
}

// inspirationSharePromptFromAnalysis renders the upstream analysis sections as
// the share's prompt block; unknown keys are skipped rather than dumped raw.
func inspirationSharePromptFromAnalysis(item inspirationUpstreamItem) string {
	sections := make([]string, 0, len(inspirationSharePromptLabels))
	seen := make(map[string]struct{}, len(inspirationSharePromptLabels))
	for _, field := range inspirationSharePromptLabels {
		raw, ok := item.Analysis[field.Key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(inspirationPublicString(raw))
		if text == "" {
			continue
		}
		if _, dup := seen[field.Label]; dup {
			continue
		}
		seen[field.Label] = struct{}{}
		sections = append(sections, "【"+field.Label+"】\n"+text)
	}
	return strings.Join(sections, "\n\n")
}

// inspirationShareModelFromAnalysis reads the generating model when the crawl
// recorded one; the share falls back to a neutral label otherwise.
func inspirationShareModelFromAnalysis(item inspirationUpstreamItem) string {
	for _, key := range []string{"model", "model_name", "modelLabel"} {
		if text := strings.TrimSpace(inspirationPublicString(item.Analysis[key])); text != "" {
			return text
		}
	}
	return "OmniMux Inspiration"
}

func inspirationShareFirstMediaURL(keys []string) string {
	for _, key := range keys {
		if mapped := inspirationPublicMediaURL(key); mapped != nil {
			return *mapped
		}
	}
	return ""
}

func inspirationShareCoverURL(key string) string {
	if mapped := inspirationPublicMediaURL(key); mapped != nil {
		return *mapped
	}
	return ""
}
