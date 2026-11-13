package helper

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var ErrImageTaskDurableEntryRequired = errors.New("this image model is currently available through the character workbench and creation commands only")

// ImageTaskRequestSizeContextKey is set only from a server-built canonical
// request or the strictly validated original body of a durable queued task.
const ImageTaskRequestSizeContextKey = "influencer_image_request_size"

// ImageTaskRequestSize validates explicit canonical specifications. In
// particular, recovery must never fill a missing size from today's defaults.
func ImageTaskRequestSize(body map[string]any) (string, error) {
	size, _ := body["size"].(string)
	n := false
	switch count := body["n"].(type) {
	case int:
		n = count == 1
	case float64:
		n = count == 1
	}
	if body["model"] != "gpt-image-2.5-sunburst" || (size != relaykitdto.ImageTaskSize1K && size != relaykitdto.ImageTaskSize2K) || body["quality"] != "high" || !n || body["output_format"] != "png" || body["response_format"] != "url" {
		return "", errors.New("image request is outside its frozen contract")
	}
	return size, nil
}

func ParseImageTaskRequestSize(body []byte) (string, error) {
	var request map[string]any
	if err := common.Unmarshal(body, &request); err != nil {
		return "", errors.New("image request is outside its frozen contract")
	}
	return ImageTaskRequestSize(request)
}

// ImageTaskUpstreamMatchesContract only recognizes exact model identities.
// Channel qualification, protocol, modes and input formats remain separate gates.
func ImageTaskUpstreamMatchesContract(publicModel, upstreamModel string, contract *relaykitdto.ImageTaskContract) bool {
	if publicModel != "gpt-image-2.5-sunburst" || contract == nil {
		return false
	}
	return upstreamModel == "gpt-image-2.5-sunburst" || upstreamModel == "gpt-image-2.5-sunburst-1k"
}

// ImageTaskChannelSupportsMode validates the declared supplier input contract
// before native priority selection; a shared channel type alone is insufficient.
func ImageTaskChannelSupportsMode(ch *model.Channel, modelName, mode string, formats ...string) bool {
	return imageTaskChannelContract(ch, modelName, mode, formats...) != nil
}

// ImageTaskChannelSupportsRequest qualifies the same canonical request on every
// candidate. A 1K-only channel cannot become a legacy 2K task's fallback.
func ImageTaskChannelSupportsRequest(ch *model.Channel, modelName, mode, size string, formats ...string) bool {
	contract := imageTaskChannelContract(ch, modelName, mode, formats...)
	return contract != nil && contract.SupportsSize(size)
}

func imageTaskChannelContract(ch *model.Channel, modelName, mode string, formats ...string) *relaykitdto.ImageTaskContract {
	if modelName != "gpt-image-2.5-sunburst" || ch == nil || ch.Type != constant.ChannelTypeOpenAI {
		return nil
	}
	var settings relaykitdto.ChannelOtherSettings
	if ch.OtherSettings == "" || common.UnmarshalJsonStr(ch.OtherSettings, &settings) != nil || settings.ImageTaskContract == nil || settings.ValidateImageTaskContract() != nil || !settings.ImageTaskContract.SupportsMode(mode) || (settings.AdvancedCustom != nil && len(settings.AdvancedCustom.ModelByParam) > 0) {
		return nil
	}
	for _, format := range formats {
		if !settings.ImageTaskContract.SupportsInputFormat(format) {
			return nil
		}
	}
	var overrides map[string]any
	if ch.ParamOverride != nil && strings.TrimSpace(*ch.ParamOverride) != "" && (common.UnmarshalJsonStr(*ch.ParamOverride, &overrides) != nil || len(overrides) != 0) {
		return nil
	}
	mapped := &gin.Context{}
	mapped.Set("model_mapping", ch.GetModelMapping())
	info := &relaycommon.RelayInfo{OriginModelName: modelName, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
	if ModelMappedHelper(mapped, info, nil) != nil || !ImageTaskUpstreamMatchesContract(modelName, info.GetUpstreamModelName(), settings.ImageTaskContract) {
		return nil
	}
	return settings.ImageTaskContract
}

// ValidateImageTaskDurableEntry applies the same generation boundary to HTTP
// dispatch and transports that select channels per event. Only registered
// original-task fetch routes are read-only; a GET can also start generation.
func ValidateImageTaskDurableEntry(c *gin.Context, modelName string) error {
	if modelName != "gpt-image-2.5-sunburst" || imageTaskReadOnlyRequest(c) {
		return nil
	}
	if c.Request.Method != http.MethodPost {
		return ErrImageTaskDurableEntryRequired
	}
	value, _ := c.Get("influencer_image_inputs")
	inputs, prepared := value.(*model.GenerationImageInputs)
	if !prepared || inputs == nil || inputs.Version != 1 || c.GetInt("id") <= 0 || c.GetString("influencer_image_mode") != inputs.Mode {
		return ErrImageTaskDurableEntryRequired
	}
	mode := inputs.Mode
	if mode != "text" && mode != "reference" && mode != "edit" && mode != "masked_edit" {
		return errors.New("image_mode is unsupported")
	}
	switch c.Request.URL.Path {
	case "/pg/influencer/sheet", "/pg/influencer/multiview":
		value, _ := c.Get("influencer_generation_routing")
		routing, ok := value.(*model.GenerationRoutingPolicy)
		if !ok || routing == nil || len(routing.Groups) == 0 || routing.MaxAttempts < 1 || routing.MaxAttempts > 2 {
			return ErrImageTaskDurableEntryRequired
		}
	case "/v1/images/generations", "/v1/images/edits", "/pg/images/generations", "/pg/images/edits":
		value, _ := c.Get("generation_queue_reserved_task")
		task, queued := value.(*model.Task)
		if !queued || task == nil || task.ID <= 0 || task.Action != "influencer_sheet" || task.UserId != c.GetInt("id") || task.Group != "gpt-image-2.5-sunburst-image" || task.Properties.OriginModelName != modelName || task.PrivateData.GenerationQueue == nil || task.PrivateData.GenerationQueue.ImageInputs != inputs {
			return ErrImageTaskDurableEntryRequired
		}
		if (mode == "text") != strings.HasSuffix(c.Request.URL.Path, "/images/generations") {
			return errors.New("image path does not match its frozen mode")
		}
	default:
		return ErrImageTaskDurableEntryRequired
	}
	return nil
}

func imageTaskReadOnlyRequest(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	taskID := c.Param("task_id")
	if taskID == "" || strings.Contains(taskID, "/") {
		return false
	}
	switch c.Request.URL.Path {
	case "/v1/videos/" + taskID, "/v1/video/generations/" + taskID,
		"/v1/images/generations/" + taskID, "/v1/images/tasks/" + taskID:
		return true
	default:
		return false
	}
}

// AppendImageTaskModeFilter narrows existing native selection only after the
// server-created workbench preparation or durable worker context is verified.
// A model ID on another generation surface cannot bypass the durable boundary.
func AppendImageTaskModeFilter(c *gin.Context, modelName string) error {
	if err := ValidateImageTaskDurableEntry(c, modelName); err != nil {
		return err
	}
	if modelName != "gpt-image-2.5-sunburst" || imageTaskReadOnlyRequest(c) {
		return nil
	}
	mode := c.GetString("influencer_image_mode")
	size := c.GetString(ImageTaskRequestSizeContextKey)
	if size != relaykitdto.ImageTaskSize1K && size != relaykitdto.ImageTaskSize2K {
		return errors.New("image request is outside its frozen contract")
	}
	if model.DB == nil {
		return errors.New("image channel capabilities are unavailable")
	}
	var ids []int
	if err := model.DB.WithContext(c.Request.Context()).Model(&model.Ability{}).Where("model = ? AND enabled = ?", modelName, true).Distinct("channel_id").Pluck("channel_id", &ids).Error; err != nil {
		return errors.New("image channel capabilities are unavailable")
	}
	if len(ids) == 0 {
		return nil
	}
	var channels []model.Channel
	if err := model.DB.WithContext(c.Request.Context()).Select("id", "type", "status", "model_mapping", "param_override", "settings").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return errors.New("image channel capabilities are unavailable")
	}
	for i := range channels {
		ch := &channels[i]
		if ch.Status != common.ChannelStatusEnabled || !ImageTaskChannelSupportsRequest(ch, modelName, mode, size, ImageTaskFrozenFormats(c)...) {
			service.GetChannelConstraints(c).AddFilter(dto.ChannelFilter{Kind: dto.FilterExcludeChannels, ExcludeChannelIDs: []int{ch.Id}})
		}
	}
	return nil
}

// ImageTaskFrozenFormats uses only server-created, decoded input identity.
// Clients cannot establish format compatibility by naming a file extension.
func ImageTaskFrozenFormats(c *gin.Context) []string {
	var formats []string
	for _, key := range []string{"influencer_frozen_image", "influencer_frozen_mask"} {
		if value, exists := c.Get(key); exists {
			if snapshot, ok := value.(service.InfluencerFrozenImage); ok {
				formats = append(formats, snapshot.Format)
			}
		}
	}
	return formats
}
