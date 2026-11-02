package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// fork: wan30-resolution-tier-group — a public video model keeps ONE model id
// per capability and its resolution quality is a routing/pricing tier, not a
// second model id. This middleware resolves that tier group from the request
// body before Distribute(), so channel selection and the billing context keys
// end up on the same group instead of a group chosen by an unrelated default.
//
// Only models declared in videoTierGroups are touched. For a declared model the
// resolution is not optional — it is what picks the tier — so a request that
// names no explicit group and declares no readable quality is rejected with an
// actionable 400 instead of falling through to a group that cannot serve it.
// Every other model, and every other request, keeps its existing behavior.

// videoTierGroups maps a declared public model to its resolution → group table.
// A model is absent from this table until its tier groups are provisioned.
var videoTierGroups = map[string]map[string]string{
	"wan-3.0": {
		"720p":  "wan-3.0-720p",
		"1080p": "wan-3.0-1080p",
	},
}

// videoTierExampleSizes spells one documented size form per tier. It exists only
// to make the missing-resolution error actionable.
var videoTierExampleSizes = map[string]string{
	"720p":  "1280x720",
	"1080p": "1920x1080",
}

// videoTierGroupRequest is the bounded slice of the submit body this middleware
// reads. Every other field is discarded rather than retained, so a large body
// does not become a large allocation here.
type videoTierGroupRequest struct {
	Model      string `json:"model"`
	Group      string `json:"group"`
	Resolution any    `json:"resolution"`
	Size       any    `json:"size"`
}

// SelectVideoTierGroup pins the resolution tier group for models that declare
// one. It runs after the task-plugin prepare step (which resolves the public
// model and an explicit model@group suffix) and before Distribute().
func SelectVideoTierGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		group, err := selectVideoTierGroup(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
			return
		}
		if group == "" {
			c.Next()
			return
		}
		// Distribution reads the group from the header and authorizes it against
		// the account's usable groups; the context keys are the same values the
		// native billing path reads, so routing and billing cannot disagree.
		c.Request.Header.Set("X-Omnimux-Group", group)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		c.Next()
	}
}

// videoTierResolutionRequiredError explains the one requirement the tier table
// imposes: without an explicit group, the resolution is what picks the tier, so
// a declared model cannot be routed without it.
func videoTierResolutionRequiredError(model string, tiers map[string]string) error {
	declared := make([]string, 0, len(tiers))
	examples := make([]string, 0, len(tiers))
	for tier := range tiers {
		declared = append(declared, strconv.Quote(tier))
		if example := videoTierExampleSizes[tier]; example != "" {
			examples = append(examples, strconv.Quote(example))
		}
	}
	slices.Sort(declared)
	slices.Sort(examples)
	message := fmt.Sprintf("resolution is required for %s: send %s", model, strings.Join(declared, " or "))
	if len(examples) > 0 {
		message += fmt.Sprintf(" (or an equivalent size such as %s)", strings.Join(examples, " or "))
	}
	return errors.New(message + ", or select a group explicitly")
}

// selectVideoTierGroup reports the tier group to pin, if any. It returns an
// error only for a declared tier model that named no explicit group and
// declared no readable quality: there the resolution is what picks the tier, so
// the request cannot be routed and the caller gets an actionable 400.
func selectVideoTierGroup(c *gin.Context) (string, error) {
	// An explicit caller choice always wins and is never overridden: the header,
	// the legacy header, a body group field or a model@group suffix. A request
	// that selected a group is never rejected for a missing resolution.
	if strings.TrimSpace(c.GetHeader("X-Omnimux-Group")) != "" {
		return "", nil
	}
	if strings.TrimSpace(c.GetHeader("New-Api-Group")) != "" {
		return "", nil
	}

	// The task-plugin prepare step may already have folded the public model and
	// an explicit model@group suffix into its endpoint request. Prefer that
	// resolved view, since it is the model distribution will actually use.
	model := strings.TrimSpace(c.GetString("resolved_task_model"))
	if cached, exists := c.Get(contextKeyTaskPluginEndpointModel); exists {
		if endpoint, ok := cached.(ModelRequest); ok {
			if strings.TrimSpace(endpoint.Group) != "" {
				return "", nil
			}
			if _, suffix := parsePerRequestModelGroup(strings.TrimSpace(endpoint.Model)); suffix != "" {
				return "", nil
			}
			if model == "" {
				model = strings.TrimSpace(endpoint.Model)
			}
		}
	}

	// A model the context already identified that declares no tier can never be
	// pinned, and nothing else here depends on the body. Return before reading
	// it: common.UnmarshalBodyReusable re-decodes the whole payload on every
	// call, streaming it from disk for large bodies, and on the video routes
	// this is the common case — every model except a declared tier.
	if model != "" {
		if _, declared := videoTierGroups[model]; !declared {
			return "", nil
		}
	}

	var request videoTierGroupRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return "", nil
	}
	bodyModel, bodySuffix := parsePerRequestModelGroup(strings.TrimSpace(request.Model))
	if bodySuffix != "" || strings.TrimSpace(request.Group) != "" {
		return "", nil
	}
	if model == "" {
		model = bodyModel
	}

	tiers, declared := videoTierGroups[model]
	if !declared {
		return "", nil
	}
	tier, ok := videoTierFromResolution(request.Resolution, request.Size)
	if !ok {
		return "", videoTierResolutionRequiredError(model, tiers)
	}
	group := tiers[tier]
	if group == "" {
		// A quality this model does not offer is the same caller mistake as an
		// absent one: the error names the tiers the model does declare.
		return "", videoTierResolutionRequiredError(model, tiers)
	}
	// A tier group only exists once it is provisioned and usable for this
	// account. Until then the request keeps its existing group rather than
	// failing authorization downstream.
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if userGroup == "" {
		userGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if !service.IsUserSelectableGroup(userGroup, group) {
		return "", nil
	}
	return group, nil
}

// videoTierFromResolution reads the quality tier from the request's resolution
// or size declaration; the public video endpoint documents both, and a caller
// may send either. An unrecognized value returns false.
func videoTierFromResolution(resolution, size any) (string, bool) {
	for _, value := range []any{resolution, size} {
		if tier := videoTierFromValue(value); tier != "" {
			return tier, true
		}
	}
	return "", false
}

func videoTierFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return videoTierFromText(typed)
	case float64:
		if typed == float64(int(typed)) {
			return videoTierFromHeight(int(typed))
		}
	case int:
		return videoTierFromHeight(typed)
	}
	return ""
}

// videoTierFromText accepts the documented spellings: "720p"/"1080P", a bare
// tier height, and a WxH (or W*H, W×H) size. A WxH size is classified by its
// shorter side so portrait and landscape declarations of the same quality agree.
func videoTierFromText(text string) string {
	normalized := strings.NewReplacer("×", "x", "*", "x", "X", "x").Replace(strings.ToLower(strings.TrimSpace(text)))
	normalized = strings.ReplaceAll(normalized, " ", "")
	if width, height, found := strings.Cut(normalized, "x"); found {
		w, widthErr := strconv.Atoi(width)
		h, heightErr := strconv.Atoi(height)
		if widthErr != nil || heightErr != nil || w <= 0 || h <= 0 {
			return ""
		}
		return videoTierFromHeight(min(w, h))
	}
	height, err := strconv.Atoi(strings.TrimSuffix(normalized, "p"))
	if err != nil {
		return ""
	}
	return videoTierFromHeight(height)
}

// videoTierFromHeight maps a picture height to a declared tier. Only the
// provisioned tiers resolve; every other height is unknown, not a nearest tier.
func videoTierFromHeight(height int) string {
	switch height {
	case 720:
		return "720p"
	case 1080:
		return "1080p"
	}
	return ""
}
