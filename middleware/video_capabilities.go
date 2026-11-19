package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay/channel/task/apimart"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// prepareVideoCapabilityFilter reads the same request shape the chosen
// adaptor will submit. Downloading or decoding media bytes stays downstream.
func prepareVideoCapabilityFilter(c *gin.Context, modelName string) *types.NewAPIError {
	if c.Request.Method != http.MethodPost || !modelcontracts.ActiveVideoCapabilities().Managed(modelName) {
		return nil
	}
	switch c.Request.URL.Path {
	case "/v1/videos", "/v1/video/generations", "/pg/video/generations":
	default:
		return nil
	}

	var intent modelcontracts.VideoRequest
	var err error
	contract, _ := modelcontracts.For(modelName)
	if contract.GatewayProtocol == "openai_video" {
		value, _ := c.Get(jsplugin.ContextKeyPinnedEndpoint)
		pinned, _ := value.(jsplugin.PinnedEndpoint)
		if pinned.Protocol != contract.GatewayProtocol || !pinned.Plugin.IsFactory() || pinned.Plugin.Meta.Key != model.MuseVideoTaskPlugin {
			return videoCapabilityUnavailable(c, modelName)
		}
		intent, err = orderedImageVideoIntent(c)
	} else {
		var raw map[string]any
		if _, claimed := c.Get(jsplugin.ContextKeyPinnedEndpoint); claimed {
			return videoCapabilityUnavailable(c, modelName)
		}
		if err = common.UnmarshalBodyReusable(c, &raw); err == nil {
			intent, err = apimart.VideoRoutingRequest(modelName, raw)
		}
	}
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	service.GetChannelConstraints(c).AddFilter(dto.ChannelFilter{Kind: dto.FilterVideoCapability, VideoRequest: &intent})
	return nil
}

func orderedImageVideoIntent(c *gin.Context) (modelcontracts.VideoRequest, error) {
	intent := modelcontracts.VideoRequest{
		Operation:  c.GetString("task_action"),
		Slots:      make(map[string]int, 2),
		Parameters: make(map[string]any, 3),
	}
	value, _ := c.Get("task_request")
	body, ok := value.(map[string]any)
	if !ok || intent.Operation == "" {
		return intent, errors.New("the video request could not be normalized")
	}
	if prompt, _ := body["prompt"].(string); strings.TrimSpace(prompt) != "" {
		intent.Slots["prompt"] = 1
	}
	switch intent.Operation {
	case "first_frame":
		if image, _ := body["image"].(string); image != "" {
			intent.Slots["first_frame"] = 1
		}
	case "video_multi_ref":
		if images, ok := body["images"].([]any); ok {
			intent.Slots["reference_images"] = len(images)
		}
	}
	for _, key := range []string{"duration", "output_format", "aspect_ratio"} {
		if value := body[key]; value != nil {
			intent.Parameters[key] = value
		}
	}
	if size, exists := body["size"]; exists && size != nil && size != "" {
		// The protocol decoder converts the two documented frame sizes to the
		// same ratios the catalog publishes. Conflicting fields have no single
		// certified interpretation for channel selection.
		var ratio string
		switch size {
		case "1280x720":
			ratio = "16:9"
		case "720x1280":
			ratio = "9:16"
		case "1:1", "adaptive":
			ratio, _ = size.(string)
		default:
			return intent, errors.New("use a video size or aspect ratio declared by the model contract")
		}
		if value, exists := intent.Parameters["aspect_ratio"]; exists && value != ratio {
			return intent, errors.New("video size and aspect_ratio must agree")
		}
		intent.Parameters["aspect_ratio"] = ratio
	}
	return intent, nil
}

// ValidateSelectedVideoChannel also covers a channel pinned by an origin task
// after initial distribution, and rechecks channel bindings before a retry.
func ValidateSelectedVideoChannel(c *gin.Context, ch *model.Channel, modelName string) *types.NewAPIError {
	constraints, ok := common.GetContextKeyType[*dto.ChannelConstraints](c, constant.ContextKeyChannelConstraints)
	if !ok {
		return nil
	}
	filter, exists := constraints.VideoFilter()
	if !exists {
		return nil
	}
	ok, failure := model.ChannelSatisfiesFilters(ch, modelName, []dto.ChannelFilter{filter})
	if ok {
		return nil
	}
	if failure == dto.FilterVideoCapability {
		err := errors.New(i18n.T(c, i18n.MsgDistributorVideoCapabilityMismatch))
		return types.NewErrorWithStatusCode(err, types.ErrorCode(failure), http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	return videoCapabilityUnavailable(c, modelName)
}

func videoCapabilityUnavailable(c *gin.Context, modelName string) *types.NewAPIError {
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	err := errors.New(i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": group, "Model": modelName}))
	return types.NewErrorWithStatusCode(err, types.ErrorCodeModelNotFound, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
}
