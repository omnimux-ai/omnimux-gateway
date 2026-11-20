package model

import (
	"errors"
	"net/http"
	"slices"
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

const (
	MuseVideoDecoderModel = "muse-video"
	SoraVideoBindingModel = "sora-2"
	// Seedance25OpenAIDecoder selects the factory hook's economy request shape.
	// The public task and the upstream request still use their own model IDs.
	Seedance25OpenAIDecoder = "seedance-2-5-openai-decoder"
)

func IsFactorySoraVideoPlugin(plugin *jsplugin.LoadedPlugin) bool {
	return plugin != nil && plugin.IsFactory() && plugin.Meta.Key == "sora"
}

// Seedance25VideoGroupProfile identifies the request protocol for old callers
// that name a concrete group, and the resolution that group can default to.
// Both decisions use the same verified channels as the catalog and distributor.
func Seedance25VideoGroupProfile(rules *modelcontracts.VideoCapabilities, generation *jsplugin.RoutingGeneration, modelName, group string) (economyOnly bool, resolution string) {
	if group == "" || group == "auto" {
		return false, ""
	}
	var native, ambiguous bool
	for _, id := range rules.ChannelIDs() {
		identity, exists := rules.ChannelIdentity(modelName, id)
		if !exists || !IsChannelEnabledForGroupModel(group, modelName, id) {
			continue
		}
		channel, err := CacheGetChannel(id)
		if err != nil || !videoChannelQualified(rules, generation, channel, modelName) {
			continue
		}
		if identity.Protocol == modelcontracts.VideoProtocolGenerations {
			native = true
		} else if identity.Profile == modelcontracts.VideoProfileSeedance25OpenAI {
			ambiguous = ambiguous || resolution != "" && resolution != identity.FixedResolution
			resolution = identity.FixedResolution
		}
	}
	economyOnly = resolution != "" && !native
	if ambiguous {
		resolution = ""
	}
	return economyOnly, resolution
}

// projectSeedanceEconomyResolution supplies a catalog default when all of the
// group's eligible economy variants use the same resolution, as routing does.
// The variants are copies of the channel rules, local to this group.
func projectSeedanceEconomyResolution(operations []modelcontracts.Operation) {
	var resolution string
	var parameters []*modelcontracts.Parameter
	for _, operation := range operations {
		if operation.GatewayProtocol != modelcontracts.VideoProtocolOpenAI {
			continue
		}
		for i := range operation.Parameters {
			parameter := &operation.Parameters[i]
			if parameter.Name != "resolution" {
				continue
			}
			fixed, _ := parameter.Fixed.(string)
			if fixed == "" || resolution != "" && resolution != fixed {
				return
			}
			resolution = fixed
			parameters = append(parameters, parameter)
		}
	}
	for _, parameter := range parameters {
		parameter.Default = resolution
		parameter.Required = false
	}
}

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
	identity, _ := rules.ChannelIdentity(modelName, ch.Id)
	if request.Protocol != "" && identity.Protocol != request.Protocol {
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

	switch identity.Protocol {
	case modelcontracts.VideoProtocolGenerations:
		return ch.Type == constant.ChannelTypeApiMart
	case modelcontracts.VideoProtocolOpenAI:
		if identity.Profile == modelcontracts.VideoProfileSeedance25OpenAI {
			owner, _ := generation.GetByChannelType(ch.Type)
			return ch.Type == constant.ChannelTypeOpenAI && IsFactorySoraVideoPlugin(owner) &&
				slices.ContainsFunc(generation.LookupEndpointCandidates(http.MethodPost, "/v1/videos", SoraVideoBindingModel), func(candidate jsplugin.ProtocolBinding) bool {
					return candidate.Plugin == owner && candidate.Protocol == identity.Protocol
				})
		}
		if modelName != MuseVideoDecoderModel && upstream != MuseVideoDecoderModel {
			return false
		}
		candidates := generation.LookupEndpointCandidates(http.MethodPost, "/v1/videos", MuseVideoDecoderModel)
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
				return candidate.Protocol == identity.Protocol && IsFactorySoraVideoPlugin(candidate.Plugin)
			}
		}
	}
	return false
}
