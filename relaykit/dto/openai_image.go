package dto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// MaxImageN caps the image generation count. Without this bound a huge or
// wrapped-negative n overflows quota calculation into a negative charge.
const MaxImageN = 128

// ImageBillingParameters contains only the provider scalars parsed by request
// validation. Keep this separate from the complete provider request payload.
type ImageBillingParameters struct {
	N            *uint `json:"n,omitempty"`
	PromptExtend *bool `json:"prompt_extend,omitempty"`
}

type ImageRequest struct {
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt" binding:"required"`
	N                 *uint           `json:"n,omitempty"`
	Size              string          `json:"size,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	ResponseFormat    string          `json:"response_format,omitempty"`
	Style             json.RawMessage `json:"style,omitempty"`
	User              json.RawMessage `json:"user,omitempty"`
	ExtraFields       json.RawMessage `json:"extra_fields,omitempty"`
	Background        json.RawMessage `json:"background,omitempty"`
	Moderation        json.RawMessage `json:"moderation,omitempty"`
	OutputFormat      json.RawMessage `json:"output_format,omitempty"`
	OutputCompression json.RawMessage `json:"output_compression,omitempty"`
	PartialImages     json.RawMessage `json:"partial_images,omitempty"`
	Stream            *bool           `json:"stream,omitempty"`
	Images            json.RawMessage `json:"images,omitempty"`
	Mask              json.RawMessage `json:"mask,omitempty"`
	InputFidelity     json.RawMessage `json:"input_fidelity,omitempty"`
	Watermark         *bool           `json:"watermark,omitempty"`
	// zhipu 4v
	WatermarkEnabled json.RawMessage `json:"watermark_enabled,omitempty"`
	UserId           json.RawMessage `json:"user_id,omitempty"`
	Image            json.RawMessage `json:"image,omitempty"`
	// 用匿名参数接收额外参数
	Extra map[string]json.RawMessage `json:"-"`
	// OptionalParams 是需要原样带回上游的可选参数，键不存在时不序列化。
	// 图像路径仅在需要透传渠道专属字段时填充，例如 Midjourney 的
	// model_params / image_urls；未填充时序列化结果与改动前完全一致。
	OptionalParams    map[string]json.RawMessage `json:"-"`
	BillingParameters *ImageBillingParameters    `json:"-"`
}

// ImageCount resolves the validated request quantity. Top-level zero retains
// its legacy default of one; an explicit provider count must be positive.
func (i *ImageRequest) ImageCount(useProviderParameters bool) (int, error) {
	n := uint(1)
	if i.N != nil && *i.N != 0 {
		n = *i.N
	}
	if n > MaxImageN {
		return 0, fmt.Errorf("n must be an integer between 1 and %d", MaxImageN)
	}
	if parameters := i.BillingParameters; parameters != nil && parameters.N != nil {
		if *parameters.N > MaxImageN || useProviderParameters && *parameters.N == 0 {
			return 0, fmt.Errorf("parameters.n must be an integer between 1 and %d", MaxImageN)
		}
		if useProviderParameters {
			n = *parameters.N
		}
	}
	return int(n), nil
}

func (i *ImageRequest) UnmarshalJSON(data []byte) error {
	// 先解析成 map[string]interface{}
	var rawMap map[string]json.RawMessage
	if err := kitutil.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 用 struct tag 获取所有已定义字段名
	knownFields := GetJSONFieldNames(reflect.TypeFor[ImageRequest]())

	// 再正常解析已定义字段
	type Alias ImageRequest
	var known Alias
	if err := kitutil.Unmarshal(data, &known); err != nil {
		return err
	}
	*i = ImageRequest(known)

	// 提取多余字段
	i.Extra = make(map[string]json.RawMessage)
	for k, v := range rawMap {
		if _, ok := knownFields[k]; !ok {
			i.Extra[k] = v
		}
	}
	return nil
}

// 序列化时需要重新把字段平铺
func (r ImageRequest) MarshalJSON() ([]byte, error) {
	// 将已定义字段转为 map
	type Alias ImageRequest
	alias := Alias(r)
	base, err := kitutil.Marshal(alias)
	if err != nil {
		return nil, err
	}

	var baseMap map[string]json.RawMessage
	if err := kitutil.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}

	// 不能合并ExtraFields！！！！！！！！
	// 合并 ExtraFields
	//for k, v := range r.Extra {
	//	if _, exists := baseMap[k]; !exists {
	//		baseMap[k] = v
	//	}
	//}

	// OptionalParams 只登记键（值可能为空），真实取值来自 Extra。这里用
	// MergedExtra 拿到「登记且请求里确实带了」的键值，已声明字段优先。
	for k, v := range r.MergedExtra() {
		existing, exists := baseMap[k]
		if !exists || isNullOrEmptyRaw(existing) {
			baseMap[k] = v
		}
	}

	return kitutil.Marshal(baseMap)
}

// isNullOrEmptyRaw reports whether a raw JSON value carries no data.
func isNullOrEmptyRaw(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// MergedExtra 从「未知字段」集合里挑出显式登记的可选参数，返回可直接合并进
// 出站请求体的键值对（值保持原始 JSON）。白名单之外的可选参数不予透传，避免
// 客户端用未知字段扩权。没有任何命中时返回 nil。
func (r *ImageRequest) MergedExtra() map[string]json.RawMessage {
	if r == nil || len(r.Extra) == 0 || len(r.OptionalParams) == 0 {
		return nil
	}
	merged := make(map[string]json.RawMessage, len(r.OptionalParams))
	for key := range r.OptionalParams {
		raw, ok := r.Extra[key]
		if !ok || len(raw) == 0 {
			continue
		}
		merged[key] = raw
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func GetJSONFieldNames(t reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过匿名字段（例如 ExtraFields）
		if field.Anonymous {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}

		// 取逗号前字段名（排除 omitempty 等）
		name := tag
		if commaIdx := indexComma(tag); commaIdx != -1 {
			name = tag[:commaIdx]
		}
		fields[name] = struct{}{}
	}
	return fields
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (i *ImageRequest) GetTokenCountMeta() *types.TokenCountMeta {
	imageN := uint(1)
	if i.N != nil && *i.N > 0 {
		imageN = *i.N
	}

	// Keep n separate from ImagePriceRatio so size/quality and count remain
	// independent billing dimensions. Fixed-price pre-consume stores this on
	// PriceData, and image settlement reuses or replaces the same "n" ratio.
	return &types.TokenCountMeta{
		CombineText:     i.Prompt,
		MaxTokens:       1584,
		ImagePriceRatio: i.legacyDallePriceRatio(),
		BillingRatios:   map[string]float64{"n": float64(imageN)},
	}
}

func (i *ImageRequest) IsStream(c *http.Request) bool {
	return i.Stream != nil && *i.Stream
}

func (i *ImageRequest) SetModelName(modelName string) {
	if modelName != "" {
		i.Model = modelName
	}
}

type ImageResponse struct {
	Data     []ImageData     `json:"data"`
	Created  int64           `json:"created"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
type ImageData struct {
	Url           string `json:"url"`
	B64Json       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
}
