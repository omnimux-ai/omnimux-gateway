package helper

import (
	"errors"
	"fmt"

	rootcommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hostreasoning "github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := rootcommon.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := info.OriginModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			mappedModel, exists := modelMap[currentModel]
			baseModel := hostreasoning.BaseModelName(currentModel)
			if (!exists || mappedModel == "") && baseModel != currentModel {
				mappedModel, exists = modelMap[baseModel]
			}
			if exists && mappedModel != "" {
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						}

						info.IsModelMapped = true
						break
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	applyModelByParam(info, request)
	return nil
}

// applyModelByParam lets an explicitly configured channel pick the upstream
// model from a request parameter (image `size`, for example): one public ID can
// then fan out to the SKU that matches the requested resolution instead of
// splitting the public catalog.
//
// It runs after model_mapping, so a matching rule wins over the channel alias,
// and it only rewrites the upstream name. OriginModelName — the billing and
// display identity — stays untouched, and IsModelMapped keeps its meaning of
// "the channel mapping renamed the model". Channels without the config are not
// affected in any way.
func applyModelByParam(info *relaycommon.RelayInfo, request dto.Request) {
	if request == nil {
		return
	}
	imageRequest, ok := request.(*dto.ImageRequest)
	if !ok {
		return
	}
	if upstream, changed := ApplyModelByParamLookup(info, func(param string) string {
		if param == "size" {
			return imageRequest.Size
		}
		return ""
	}); changed {
		request.SetModelName(upstream)
	}
}

// ApplyModelByParamLookup is the request-object-free half of the rule above, for
// callers that already hold the parameters but not a dto.ImageRequest (the
// task/task relay path reads them from the reusable body). It reports the
// upstream name it applied, or false when nothing changed.
func ApplyModelByParamLookup(info *relaycommon.RelayInfo, lookup func(param string) string) (string, bool) {
	if info == nil || info.ChannelMeta == nil || lookup == nil {
		return "", false
	}
	custom := info.ChannelMeta.ChannelOtherSettings.AdvancedCustom
	if custom == nil || len(custom.ModelByParam) == 0 {
		return "", false
	}
	upstream, matched := custom.ResolveModelByParam(info.OriginModelName, lookup)
	if !matched || upstream == info.UpstreamModelName {
		return "", false
	}
	info.UpstreamModelName = upstream
	return upstream, true
}
