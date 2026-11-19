package model

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/reasoning"
)

var ErrVideoCapabilityMismatch = errors.New("no verified channel accepts this video request")

// MuseVideoTaskPlugin is the executing plugin of the verified OpenAI Video
// profile. A different plugin claiming the endpoint does not inherit its rules.
const MuseVideoTaskPlugin = "sora"

// videoChannelOperations returns the confirmed operations of an enabled
// channel. The same binding checks run on every routing attempt.
func videoChannelOperations(rules *modelcontracts.VideoCapabilities, generation *jsplugin.RoutingGeneration, ch *Channel, modelName string) []modelcontracts.Operation {
	if !videoChannelQualified(rules, generation, ch, modelName) {
		return nil
	}
	rule, _ := rules.ChannelRule(modelName, ch.Id)
	return rule.Operations
}

func videoChannelRejection(ch *Channel, modelName string, request *modelcontracts.VideoRequest) dto.ChannelFilterKind {
	rules := modelcontracts.ActiveVideoCapabilities()
	if request == nil || !videoChannelQualified(rules, jsplugin.DefaultRegistry.Generation(), ch, modelName) {
		return dto.FilterVideoCapabilityUnavailable
	}
	if !rules.Accepts(modelName, ch.Id, *request) {
		return dto.FilterVideoCapability
	}
	return ""
}

// The candidate may be owned by the channel cache. Decode settings without
// using the getters that repair malformed persisted settings on read.
func videoChannelQualified(rules *modelcontracts.VideoCapabilities, generation *jsplugin.RoutingGeneration, ch *Channel, modelName string) bool {
	if ch == nil || ch.Status != common.ChannelStatusEnabled {
		return false
	}
	identity, ok := rules.ChannelIdentity(modelName, ch.Id)
	if !ok || identity.ChannelType != ch.Type || identity.BaseURL != strings.TrimRight(ch.GetBaseURL(), "/") {
		return false
	}
	if ch.ParamOverride != nil && strings.TrimSpace(*ch.ParamOverride) != "" {
		var override map[string]any
		if common.UnmarshalJsonStr(*ch.ParamOverride, &override) != nil || len(override) != 0 {
			return false
		}
	}
	var other kitdto.ChannelOtherSettings
	if ch.OtherSettings != "" && common.UnmarshalJsonStr(ch.OtherSettings, &other) != nil {
		return false
	}
	if other.AdvancedCustom != nil && len(other.AdvancedCustom.ModelByParam) != 0 {
		return false
	}
	upstream := modelName
	if mapping := ch.GetModelMapping(); mapping != "" {
		var modelMap map[string]string
		if common.UnmarshalJsonStr(mapping, &modelMap) != nil {
			return false
		}
		var cyclic bool
		upstream, cyclic = followChannelModelMapping(modelMap, modelName)
		if cyclic || reasoning.BaseModelName(upstream) != upstream {
			return false
		}
	}
	if upstream != identity.UpstreamModel {
		// The native video adaptor emits the same wire model for the supported
		// dotted and hyphenated public spellings. Explicit vendor mappings
		// still have to agree with the configured target.
		if identity.Profile != "apimart_video" {
			return false
		}
		mappedContract, mapped := modelcontracts.For(upstream)
		configuredContract, configured := modelcontracts.For(identity.UpstreamModel)
		if !mapped || !configured || mappedContract.ModelID != configuredContract.ModelID {
			return false
		}
	}

	switch identity.Profile {
	case "apimart_video":
		return ch.Type == constant.ChannelTypeApiMart
	case "openai_video":
		contract, ok := modelcontracts.For(modelName)
		if !ok || contract.GatewayProtocol != identity.Profile {
			return false
		}
		candidates := generation.LookupEndpointCandidates(http.MethodPost, "/v1/videos", contract.ModelID)
		var settings kitdto.ChannelSettings
		if ch.Setting != nil && *ch.Setting != "" && common.UnmarshalJsonStr(*ch.Setting, &settings) != nil {
			return false
		}
		for _, candidate := range candidates {
			if candidate.Plugin == nil {
				continue
			}
			bound := false
			switch ch.Type {
			case constant.ChannelTypeTaskPlugin:
				bound = settings.TaskPluginKey == candidate.Plugin.Meta.Key
			case constant.ChannelTypeNewAPI:
				bound = settings.BindsTaskPlugin(candidate.Plugin.Meta.Key)
			case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
				owner, _ := generation.GetByChannelType(ch.Type)
				bound = owner == candidate.Plugin
			}
			if bound {
				return candidate.Protocol == identity.Profile && candidate.Plugin.Meta.Key == MuseVideoTaskPlugin && candidate.Plugin.IsFactory()
			}
		}
	}
	return false
}
