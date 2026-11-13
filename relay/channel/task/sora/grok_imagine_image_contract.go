package sora

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const imageTaskContentTypeKey = "image_task_content_type"

// ImageTaskPlatformV1 binds newly frozen image requests to their native image
// adaptor during polling; historical Sora video/platform records stay intact.
const ImageTaskPlatformV1 constant.TaskPlatform = "image_task_v1"

// normalizedImageTaskInput adds public defaults to a local copy. In particular,
// a failed attempt never changes the canonical body another channel will read.
func normalizedImageTaskInput(input map[string]any) (map[string]any, error) {
	body := make(map[string]any, len(input)+6)
	for key, value := range input {
		switch key {
		case "model", "group", "prompt", "image_mode", "size", "aspect_ratio", "quality", "n", "output_format", "response_format", "image", "images", "image_urls", "input_reference", "mask_url", "metadata", "seed":
		default:
			return nil, fmt.Errorf("image task request contains an unsupported parameter: %s", key)
		}
		body[key] = value
	}
	if _, exists := body["seed"]; exists {
		return nil, fmt.Errorf("seed is not supported by the reviewed image task contract")
	}
	if value, exists := body["metadata"]; exists {
		metadata, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("image task metadata must contain supported record fields")
		}
		if _, exists := metadata["seed"]; exists {
			return nil, fmt.Errorf("seed is not supported by the reviewed image task contract")
		}
		if len(metadata) != 0 {
			return nil, fmt.Errorf("image task metadata contains an unsupported field")
		}
	}
	if ratio, exists := body["aspect_ratio"]; exists {
		if ratio != "16:9" {
			return nil, fmt.Errorf("image task request must use the reviewed 16:9 aspect ratio")
		}
		if size, exists := body["size"]; exists && size != "2048x1152" && size != "1024x576" {
			return nil, fmt.Errorf("image task size does not match its aspect ratio")
		}
		if _, exists := body["size"]; !exists {
			body["size"] = "2048x1152"
		}
		delete(body, "aspect_ratio")
	}
	for _, key := range []string{"images", "image_urls", "image", "input_reference"} {
		if value, exists := body[key]; exists {
			normalized, err := evoLinkImageReferenceValue(value)
			if err != nil {
				return nil, err
			}
			body[key] = normalized
		}
	}
	for key, value := range map[string]any{"quality": "high", "size": "2048x1152", "n": float64(1), "output_format": "png", "response_format": "url"} {
		if _, exists := body[key]; !exists {
			body[key] = value
		}
	}
	if _, exists := body["image_mode"]; !exists {
		mode := relaykitdto.ImageTaskModeText
		if len(referenceURLsFromBody(body)) > 0 {
			mode = relaykitdto.ImageTaskModeReference
		}
		if _, hasMask := body["mask_url"]; hasMask {
			mode = relaykitdto.ImageTaskModeMaskedEdit
		}
		body["image_mode"] = mode
	}
	return body, nil
}

// imageTaskContractFor only opts the new public Sunburst task contract in.
// Older model IDs retain their existing adaptor behavior on shared channels.
func imageTaskContractFor(info *relaycommon.RelayInfo) (*relaykitdto.ImageTaskContract, error) {
	if info == nil || info.ChannelMeta == nil || strings.TrimSpace(info.OriginModelName) != "gpt-image-2.5-sunburst" || info.ChannelOtherSettings.ImageTaskContract == nil {
		return nil, nil
	}
	if !helper.ImageTaskUpstreamMatchesContract(info.OriginModelName, info.GetUpstreamModelName(), info.ChannelOtherSettings.ImageTaskContract) {
		return nil, fmt.Errorf("image task contract requires the exact upstream model")
	}
	if err := info.ChannelOtherSettings.ValidateImageTaskContract(); err != nil {
		return nil, err
	}
	return info.ChannelOtherSettings.ImageTaskContract, nil
}

func validateImageTaskContractBody(body map[string]any, contract *relaykitdto.ImageTaskContract) (string, error) {
	mode, _ := body["image_mode"].(string)
	if !contract.SupportsMode(mode) {
		return "", fmt.Errorf("image task channel does not support the requested mode")
	}
	prompt, ok := body["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if err := helper.ValidateGPTImagePrompt("gpt-image-2.5-sunburst", prompt); err != nil {
		return "", err
	}
	size, err := helper.ImageTaskRequestSize(body)
	if err != nil {
		return "", err
	}
	if !contract.SupportsSize(size) {
		return "", fmt.Errorf("image task channel does not support the requested size")
	}
	refs := referenceURLsFromBody(body)
	mask := ""
	if value, exists := body["mask_url"]; exists {
		var ok bool
		mask, ok = value.(string)
		if !ok || strings.TrimSpace(mask) == "" {
			return "", fmt.Errorf("mask_url must be a nonempty URL")
		}
	}
	if mode == relaykitdto.ImageTaskModeText {
		if len(refs) != 0 || strings.TrimSpace(mask) != "" {
			return "", fmt.Errorf("text mode cannot contain image inputs")
		}
		return mode, nil
	}
	if len(refs) != 1 {
		return "", fmt.Errorf("image task mode requires one frozen source image")
	}
	if mode == relaykitdto.ImageTaskModeMaskedEdit {
		if strings.TrimSpace(mask) == "" {
			return "", fmt.Errorf("masked edit requires a frozen mask")
		}
	} else if strings.TrimSpace(mask) != "" {
		return "", fmt.Errorf("mask is only supported in masked edit mode")
	}
	return mode, nil
}

func validateExplicitImageTask(c *gin.Context, info *relaycommon.RelayInfo, contract *relaykitdto.ImageTaskContract) *dto.TaskError {
	if contract.DeliveryFormat() == "b64_json" {
		task, err := reservedInlineImageTask(c, info, false)
		if err == nil {
			_, err = service.InlineImageTaskContentURL(task)
		}
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_image_task_destination", http.StatusBadRequest)
		}
	}
	var body map[string]any
	if err := common.UnmarshalBodyReusable(c, &body); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	body, err := normalizedImageTaskInput(body)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	mode, err := validateImageTaskContractBody(body, contract)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if mode != relaykitdto.ImageTaskModeText {
		if err := freezeExplicitImageTaskInputs(c, info, body, mode, service.NewInfluencerReferenceHTTPClient()); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
	} else if err := retainImageTaskInputAudit(c, &model.GenerationImageInputs{Version: 1, Mode: mode}); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	encoded, err := common.Marshal(body)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	var req relaycommon.TaskSubmitReq
	if err := common.Unmarshal(encoded, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = constant.TaskActionGenerate
	if mode != relaykitdto.ImageTaskModeText {
		info.Action = constant.TaskActionImageEdit
	}
	c.Set("task_request", req)
	return nil
}

func buildExplicitImageTaskBody(c *gin.Context, info *relaycommon.RelayInfo, contract *relaykitdto.ImageTaskContract) (io.Reader, error) {
	if contract.DeliveryFormat() == "b64_json" {
		task, err := reservedInlineImageTask(c, info, false)
		if err != nil {
			return nil, err
		}
		if _, err := service.InlineImageTaskContentURL(task); err != nil {
			return nil, err
		}
	}
	var input map[string]any
	if err := common.UnmarshalBodyReusable(c, &input); err != nil {
		return nil, err
	}
	input, err := normalizedImageTaskInput(input)
	if err != nil {
		return nil, err
	}
	mode, err := validateImageTaskContractBody(input, contract)
	if err != nil {
		return nil, err
	}
	// Rebuild from canonical input for every attempt; never reuse a previous
	// supplier's body, multipart boundary, or Content-Type.
	body := make(map[string]any, 8)
	for _, key := range []string{"prompt", "n", "size", "quality", "output_format", "response_format"} {
		if value, exists := input[key]; exists {
			body[key] = value
		}
	}
	body["model"] = info.GetUpstreamModelName()
	body["response_format"] = contract.DeliveryFormat()
	c.Set(imageTaskContentTypeKey, "application/json")
	if mode != relaykitdto.ImageTaskModeText {
		// The reference/edit image-file contract has a real supplier receipt.
		// A successful image file does not establish that masks are honored.
		if contract.Protocol == relaykitdto.ImageTaskProtocolOpenAIMultipart && mode == relaykitdto.ImageTaskModeMaskedEdit {
			return nil, fmt.Errorf("image task multipart mask contract is not enabled")
		}
		refs := referenceURLsFromBody(input)
		source, sourceBytes, exists, err := readImageTaskFrozenInput(c, info, "influencer_frozen_image", refs[0])
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("frozen source image does not match the request")
		}
		if !contract.SupportsInputFormat(source.Format) {
			return nil, fmt.Errorf("image task channel does not support the source image format")
		}
		if contract.Protocol == relaykitdto.ImageTaskProtocolOpenAIMultipart {
			var buffer bytes.Buffer
			writer := multipart.NewWriter(&buffer)
			for _, key := range []string{"model", "prompt", "size", "quality", "n", "output_format", "response_format"} {
				if err := writer.WriteField(key, fmt.Sprint(body[key])); err != nil {
					return nil, err
				}
			}
			extension := source.Format
			if extension == "jpeg" {
				extension = "jpg"
			}
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="source.%s"`, extension))
			header.Set("Content-Type", "image/"+source.Format)
			part, err := writer.CreatePart(header)
			if err != nil {
				return nil, err
			}
			if _, err := part.Write(sourceBytes); err != nil {
				return nil, err
			}
			if err := writer.Close(); err != nil {
				return nil, err
			}
			c.Set(imageTaskContentTypeKey, writer.FormDataContentType())
			return bytes.NewReader(buffer.Bytes()), nil
		}
		body["image_urls"] = []string{source.URL}
		if mode == relaykitdto.ImageTaskModeMaskedEdit {
			rawMask, _ := input["mask_url"].(string)
			mask, _, exists, err := readImageTaskFrozenInput(c, info, "influencer_frozen_mask", rawMask)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("frozen mask does not match the request")
			}
			if mask.Format != "png" || mask.Width != source.Width || mask.Height != source.Height {
				return nil, fmt.Errorf("image task mask must be PNG on the source canvas")
			}
			body["mask_url"] = mask.URL
		}
	}
	encoded, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(encoded), nil
}

func readImageTaskFrozenInput(c *gin.Context, info *relaycommon.RelayInfo, key, originalURL string) (service.InfluencerFrozenImage, []byte, bool, error) {
	value, exists := c.Get(key)
	if !exists {
		return service.InfluencerFrozenImage{}, nil, false, nil
	}
	snapshot, ok := value.(service.InfluencerFrozenImage)
	if !ok || (snapshot.URL != originalURL && c.GetString(key+"_original_url") != originalURL) {
		return service.InfluencerFrozenImage{}, nil, false, fmt.Errorf("frozen image does not match the request")
	}
	content, err := service.ReadInfluencerFrozenImage(c.Request.Context(), info.UserId, snapshot)
	if err != nil {
		return service.InfluencerFrozenImage{}, nil, false, err
	}
	return snapshot, content, true, nil
}

func prepareImageTaskFrozenInput(c *gin.Context, info *relaycommon.RelayInfo, key, originalURL string, client *http.Client) (service.InfluencerFrozenImage, *service.PreparedInfluencerImage, error) {
	snapshot, _, exists, err := readImageTaskFrozenInput(c, info, key, originalURL)
	if err != nil || exists {
		return snapshot, nil, err
	}
	expiry, err := service.ResolveInfluencerImageSourceExpiry(c.Request.Context(), info.UserId, originalURL)
	if err != nil {
		return service.InfluencerFrozenImage{}, nil, err
	}
	prepared, err := service.FreezeInfluencerImage(c.Request.Context(), info.UserId, originalURL, expiry, client)
	if err != nil {
		return service.InfluencerFrozenImage{}, nil, err
	}
	return prepared.InfluencerFrozenImage, &prepared, nil
}

func freezeExplicitImageTaskInputs(c *gin.Context, info *relaycommon.RelayInfo, body map[string]any, mode string, client *http.Client) error {
	originalSource := referenceURLsFromBody(body)[0]
	source, preparedSource, err := prepareImageTaskFrozenInput(c, info, "influencer_frozen_image", originalSource, client)
	if err != nil {
		return err
	}
	var preparedMask *service.PreparedInfluencerImage
	retained := false
	defer func() {
		if !retained {
			if preparedSource != nil {
				preparedSource.Discard()
			}
			if preparedMask != nil {
				preparedMask.Discard()
			}
		}
	}()
	if !info.ChannelOtherSettings.ImageTaskContract.SupportsInputFormat(source.Format) {
		return fmt.Errorf("image task channel does not support the source image format")
	}
	var mask service.InfluencerFrozenImage
	originalMask := ""
	if mode == relaykitdto.ImageTaskModeMaskedEdit {
		originalMask, _ = body["mask_url"].(string)
		mask, preparedMask, err = prepareImageTaskFrozenInput(c, info, "influencer_frozen_mask", originalMask, client)
		if err != nil {
			return err
		}
		if err := service.ValidateInfluencerFrozenImageMask(c.Request.Context(), info.UserId, source, mask); err != nil {
			return err
		}
	}
	inputs := &model.GenerationImageInputs{Version: 1, Mode: mode, Source: &source}
	if mode == relaykitdto.ImageTaskModeMaskedEdit {
		inputs.Mask = &mask
	}
	if err := retainImageTaskInputAudit(c, inputs); err != nil {
		return err
	}
	c.Set("influencer_frozen_image", source)
	c.Set("influencer_frozen_image_original_url", originalSource)
	if mode == relaykitdto.ImageTaskModeMaskedEdit {
		c.Set("influencer_frozen_mask", mask)
		c.Set("influencer_frozen_mask_original_url", originalMask)
	}
	// Once validated, retain these native-expiry file records across rejection,
	// retry, accepted and unknown outcomes. The existing file expiry service
	// reclaims them; deleting here could invalidate an in-flight supplier read.
	retained = true
	return nil
}

func retainImageTaskInputAudit(c *gin.Context, inputs *model.GenerationImageInputs) error {
	if value, exists := c.Get("influencer_image_inputs"); exists {
		original, ok := value.(*model.GenerationImageInputs)
		if !ok || original == nil || original.Version != inputs.Version || original.Mode != inputs.Mode || (original.Source == nil) != (inputs.Source == nil) || (original.Mask == nil) != (inputs.Mask == nil) {
			return fmt.Errorf("image task inputs do not match their private snapshot")
		}
		if original.Source != nil && *original.Source != *inputs.Source || original.Mask != nil && *original.Mask != *inputs.Mask {
			return fmt.Errorf("image task inputs do not match their private snapshot")
		}
		return nil
	}
	c.Set("influencer_image_inputs", inputs)
	return nil
}

func explicitImageTaskResponseURL(body []byte) (string, error) {
	var response struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &response); err != nil || len(response.Data) != 1 {
		return "", fmt.Errorf("image task response must contain one retrievable image")
	}
	if err := validateExplicitImageTaskResultURL(response.Data[0].URL); err != nil {
		return "", err
	}
	return response.Data[0].URL, nil
}

func validateExplicitImageTaskResultURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || strings.Contains(rawURL, "#") {
		return fmt.Errorf("image task response has no retrievable image URL")
	}
	return nil
}

func validateExplicitImageTaskPollResult(body []byte, resultURL string) error {
	if err := validateExplicitImageTaskResultURL(resultURL); err != nil {
		return err
	}
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return err
	}
	containers := []map[string]any{envelope}
	if data, ok := envelope["data"].(map[string]any); ok {
		containers = append(containers, data)
	}
	baseContainerCount := len(containers)
	for _, container := range append([]map[string]any(nil), containers...) {
		if result, ok := container["result"].(map[string]any); ok {
			containers = append(containers, result)
		}
	}
	// These are the image collections already understood by the shared poll
	// parser. A second image or a second contradictory URL is not silently lost.
	genuineURLFound := false
	for index, container := range containers {
		if index >= baseContainerCount {
			for _, key := range []string{"url", "image_url", "image"} {
				if value, ok := container[key].(string); ok && value != "" {
					if value != resultURL {
						return fmt.Errorf("image task response contains conflicting image results")
					}
					genuineURLFound = true
				}
			}
		}
		for _, key := range []string{"images", "results", "result_data", "image_urls"} {
			value, exists := container[key]
			if !exists || value == nil {
				continue
			}
			list, ok := value.([]any)
			if !ok || len(list) > 1 {
				return fmt.Errorf("image task response must contain one retrievable image")
			}
			if len(list) > 0 {
				if url := firstURLFromAnyList(list); url == "" || url != resultURL {
					return fmt.Errorf("image task response contains conflicting image results")
				}
				genuineURLFound = true
			}
			for _, item := range list {
				if object, ok := item.(map[string]any); ok {
					for _, field := range []string{"url", "image_url", "image"} {
						switch value := object[field].(type) {
						case string:
							if value != "" && value != resultURL {
								return fmt.Errorf("image task response contains conflicting image results")
							}
						case []any:
							if len(value) > 1 || len(value) == 1 && value[0] != resultURL {
								return fmt.Errorf("image task response must contain one retrievable image")
							}
						}
					}
				}
			}
		}
	}
	if !genuineURLFound {
		return fmt.Errorf("image task response has no retrievable image URL")
	}
	return nil
}

// reservedInlineImageTask binds storage to the server-reserved current task.
// Neither a supplier id nor the originating parent image can supply this key.
func reservedInlineImageTask(c *gin.Context, info *relaycommon.RelayInfo, sent bool) (*model.Task, error) {
	if c == nil || c.Request == nil || info == nil || info.TaskRelayInfo == nil || info.ChannelMeta == nil || info.PublicTaskID == "" || info.OriginTaskID == info.PublicTaskID {
		return nil, fmt.Errorf("inline image task identity is unavailable")
	}
	// The durable send snapshot is read back from SQL and replaces the queue
	// value. Its frozen input identity must still match by value; pointer equality
	// belongs only to the original durable-entry admission check.
	inputValue, _ := c.Get("influencer_image_inputs")
	inputs, prepared := inputValue.(*model.GenerationImageInputs)
	if !prepared || inputs == nil || inputs.Version != 1 || c.GetString("influencer_image_mode") != inputs.Mode || info.OriginModelName != "gpt-image-2.5-sunburst" || c.Request.Method != http.MethodPost {
		return nil, fmt.Errorf("inline image task has no prepared input identity")
	}
	switch inputs.Mode {
	case relaykitdto.ImageTaskModeText, relaykitdto.ImageTaskModeReference, relaykitdto.ImageTaskModeEdit, relaykitdto.ImageTaskModeMaskedEdit:
	default:
		return nil, fmt.Errorf("inline image task has an unsupported frozen mode")
	}
	switch c.Request.URL.Path {
	case "/v1/images/generations", "/v1/images/edits", "/pg/images/generations", "/pg/images/edits":
		if (inputs.Mode == relaykitdto.ImageTaskModeText) != strings.HasSuffix(c.Request.URL.Path, "/images/generations") {
			return nil, fmt.Errorf("inline image task path does not match its frozen mode")
		}
	default:
		return nil, fmt.Errorf("inline image task requires the durable image entry")
	}
	value, _ := c.Get("generation_queue_reserved_task")
	task, ok := value.(*model.Task)
	if !ok || task == nil || task.ID <= 0 || task.TaskID != info.PublicTaskID || task.UserId <= 0 || task.UserId != info.UserId || task.UserId != c.GetInt("id") || task.Status != model.TaskStatusDispatching || task.Action != "influencer_sheet" || task.Group != "gpt-image-2.5-sunburst-image" || task.Properties.OriginModelName != "gpt-image-2.5-sunburst" || task.PrivateData.GenerationQueue == nil {
		return nil, fmt.Errorf("inline image task identity does not match its reservation")
	}
	queue := task.PrivateData.GenerationQueue
	if queue.ImageInputs == nil || queue.ImageInputs.Version != 1 {
		return nil, fmt.Errorf("inline image task has no frozen queue input identity")
	}
	if err := retainImageTaskInputAudit(c, queue.ImageInputs); err != nil {
		return nil, err
	}
	scannerValue, _ := c.Get("generation_queue_scanner")
	scanner, ok := scannerValue.(*model.SystemTask)
	if !ok || scanner == nil || scanner.TaskID == "" || scanner.TaskID != queue.ClaimOwner || c.GetString("generation_queue_runner") == "" || queue.ReleaseStartedAt != 0 {
		return nil, fmt.Errorf("inline image task dispatch ownership is unavailable")
	}
	if sent {
		if queue.SendStartedAt <= 0 || !c.GetBool("generation_queue_attempt_started") {
			return nil, fmt.Errorf("inline image task send fence is unavailable")
		}
		if routing := queue.Routing; routing != nil {
			if len(routing.Attempts) == 0 {
				return nil, fmt.Errorf("inline image task has no live attempt")
			}
			attempt := routing.Attempts[len(routing.Attempts)-1]
			if attempt.ChannelID != info.GetChannelID() || attempt.Outcome != model.GenerationAttemptPending {
				return nil, fmt.Errorf("inline image task response does not match its live attempt")
			}
		}
	}
	return &model.Task{TaskID: task.TaskID, UserId: task.UserId}, nil
}

func explicitInlineImageTaskResponse(c *gin.Context, info *relaycommon.RelayInfo, body []byte) ([]byte, string, error) {
	task, err := reservedInlineImageTask(c, info, true)
	if err != nil {
		return nil, "", err
	}
	var response struct {
		Created int64 `json:"created"`
		Data    []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &response); err != nil || len(response.Data) != 1 || response.Data[0].B64JSON == "" || response.Data[0].URL != "" {
		return nil, "", fmt.Errorf("inline image task response must contain one PNG and no alternate URL")
	}
	result, err := service.PersistInlineImageTask(c.Request.Context(), task, response.Data[0].B64JSON)
	if err != nil {
		return nil, "", err
	}
	normalized, err := common.Marshal(map[string]any{
		"id": info.PublicTaskID, "task_id": info.PublicTaskID, "created": response.Created,
		"data": []map[string]any{{"url": result.URL, "width": result.Width, "height": result.Height, "bytes": result.Bytes, "sha256": fmt.Sprintf("%x", result.SHA256)}},
	})
	return normalized, result.URL, err
}
