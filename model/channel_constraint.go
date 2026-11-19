package model

import (
	"slices"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

var filterEvalOrder = []dto.ChannelFilterKind{
	dto.FilterExcludeChannels,
	dto.FilterRequestPath,
	dto.FilterTaskPluginIdentity,
	dto.FilterResponsesWebSocket,
	dto.FilterVideoCapability,
}

// ChannelSatisfiesFilters reports whether ch passes every filter.
// On false, it returns the kind of the first violated filter (request_path
// then task_plugin_identity) for error attribution.
func ChannelSatisfiesFilters(ch *Channel, modelName string, filters []dto.ChannelFilter) (bool, dto.ChannelFilterKind) {
	if ch == nil {
		return false, ""
	}
	for _, kind := range filterEvalOrder {
		for _, filter := range filters {
			if filter.Kind != kind {
				continue
			}
			if kind == dto.FilterVideoCapability {
				if rejected := videoChannelRejection(ch, modelName, filter.VideoRequest); rejected != "" {
					return false, rejected
				}
				continue
			}
			if !channelMatchesFilter(ch, modelName, filter) {
				return false, kind
			}
		}
	}
	return true, ""
}

// filterCandidateIDs applies filters to a cached candidate id list.
// Caller must hold channelSyncLock (read lock). The input slice is never mutated.
// A missing id in channelsIDM is kept for request_path (downstream consistency
// error) and dropped for task_plugin_identity, matching the previous filters.
func filterCandidateIDs(ids []int, modelName string, filters []dto.ChannelFilter) (kept []int, emptiedBy dto.ChannelFilterKind) {
	if len(ids) == 0 {
		return ids, ""
	}
	kept = ids
	for _, kind := range filterEvalOrder {
		kindFilters := filtersByKind(filters, kind)
		if len(kindFilters) == 0 {
			continue
		}
		next := make([]int, 0, len(kept))
		rejectedBy := kind
		if kind == dto.FilterVideoCapability {
			rejectedBy = dto.FilterVideoCapabilityUnavailable
		}
		for _, id := range kept {
			channel, exists := channelsIDM[id]
			failure := candidateFilterRejection(channel, exists, modelName, kind, kindFilters)
			if failure == "" {
				next = append(next, id)
			} else if failure == dto.FilterVideoCapability {
				rejectedBy = failure
			}
		}
		if len(kept) > 0 && len(next) == 0 {
			return next, rejectedBy
		}
		kept = next
	}
	return kept, ""
}

func filtersByKind(filters []dto.ChannelFilter, kind dto.ChannelFilterKind) []dto.ChannelFilter {
	var matched []dto.ChannelFilter
	for _, filter := range filters {
		if filter.Kind == kind {
			matched = append(matched, filter)
		}
	}
	return matched
}

func candidateFilterRejection(ch *Channel, exists bool, modelName string, kind dto.ChannelFilterKind, filters []dto.ChannelFilter) dto.ChannelFilterKind {
	if kind == dto.FilterRequestPath && !exists {
		return ""
	}
	if kind == dto.FilterVideoCapability {
		for _, filter := range filters {
			if rejected := videoChannelRejection(ch, modelName, filter.VideoRequest); rejected != "" {
				return rejected
			}
		}
		return ""
	}
	if !exists || ch == nil {
		return kind
	}
	for _, filter := range filters {
		if !channelMatchesFilter(ch, modelName, filter) {
			return kind
		}
	}
	return ""
}

func channelMatchesFilter(ch *Channel, modelName string, filter dto.ChannelFilter) bool {
	switch filter.Kind {
	case dto.FilterExcludeChannels:
		return !slices.Contains(filter.ExcludeChannelIDs, ch.Id)
	case dto.FilterRequestPath:
		if filter.RequestPath == "" {
			return true
		}
		if !constant.IsAdvancedCustomChannel(ch.Type) {
			return true
		}
		config := ch.GetOtherSettings().AdvancedCustom
		return config != nil && config.SupportsPathForModel(filter.RequestPath, modelName)
	case dto.FilterTaskPluginIdentity:
		if filter.TaskPluginExclude {
			filter.TaskPluginExclude = false
			return !channelMatchesFilter(ch, modelName, filter)
		}
		if filter.TaskPluginKey == "" {
			return ch.Type != constant.ChannelTypeTaskPlugin
		}
		if ch.Type == constant.ChannelTypeTaskPlugin || ch.Type == constant.ChannelTypeNewAPI {
			// A New API channel serves every plugin it is extended with; the
			// pinned plugin or any shared-model candidate may execute there.
			setting := ch.GetSetting()
			return setting.BindsTaskPlugin(filter.TaskPluginKey) || slices.ContainsFunc(filter.TaskPluginKeys, setting.BindsTaskPlugin)
		}
		return slices.Contains(filter.TaskPluginChannelTypes, ch.Type)
	case dto.FilterResponsesWebSocket:
		if !ch.GetSetting().ResponsesWebSocketEnabled {
			return false
		}
		switch ch.Type {
		case constant.ChannelTypeOpenAI, constant.ChannelTypeCodex, constant.ChannelTypeSub2API, constant.ChannelTypeNewAPI:
			return true
		case constant.ChannelTypeAdvancedCustom:
			// The session forwards native Responses events without protocol
			// conversion, so only a converter-free /v1/responses route qualifies.
			route, ok := ch.GetOtherSettings().AdvancedCustom.MatchPathForModel("/v1/responses", modelName)
			return ok && route.IsNative()
		default:
			return false
		}
	default:
		return true
	}
}
