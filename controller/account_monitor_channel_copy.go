package controller

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// fork: only an explicit scope may narrow a saved channel for account monitoring,
// public tool pages, or TikTok Shop selection. An absent body retains the original
// channel-copy contract.
func parseChannelCopyScope(body io.Reader) (string, error) {
	if body == nil {
		return "", nil
	}
	const maxBodyBytes = 4096
	invalid := errors.New("invalid channel copy scope")
	raw, err := io.ReadAll(io.LimitReader(body, maxBodyBytes+1))
	if err != nil || len(raw) > maxBodyBytes {
		return "", invalid
	}
	if len(raw) == 0 {
		return "", nil
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || len(fields) != 1 {
		return "", invalid
	}
	var scope string
	if err := common.Unmarshal(fields["scope"], &scope); err != nil ||
		(scope != "account-monitor" && scope != "tiktok-tool" && scope != "tiktok-shop") {
		return "", invalid
	}
	return scope, nil
}

type tikHubRouteContract struct {
	model          string
	path           string
	policy         string
	fields         []string
	requiredFields []string
}

type tikHubScopedCopySpec struct {
	name           string
	errMessage     string
	disableAutoBan bool
	contracts      []tikHubRouteContract
}

// narrowTikHubChannelCopy never calls mutating saved-setting getters. Only the
// declared fixed GET contracts and their server-side credentials survive the copy.
func narrowTikHubChannelCopy(origin *model.Channel, spec tikHubScopedCopySpec) (*model.Channel, error) {
	invalid := errors.New(spec.errMessage)
	if origin == nil || origin.Type != constant.ChannelTypeAdvancedCustom || origin.BaseURL == nil || *origin.BaseURL != "https://api.tikhub.io" {
		return nil, invalid
	}
	switch origin.Status {
	case common.ChannelStatusUnknown, common.ChannelStatusManuallyDisabled, common.ChannelStatusAutoDisabled:
	default:
		return nil, invalid
	}
	var transport dto.ChannelSettings
	if origin.Setting != nil && *origin.Setting != "" {
		if err := common.Unmarshal([]byte(*origin.Setting), &transport); err != nil || len(transport.TaskPluginBindings()) > 0 {
			return nil, invalid
		}
	}
	var settings dto.ChannelOtherSettings
	if err := common.Unmarshal([]byte(origin.OtherSettings), &settings); err != nil || settings.AdvancedCustom == nil {
		return nil, invalid
	}
	selected := make(map[string]dto.AdvancedCustomRoute, len(spec.contracts))
	for _, route := range settings.AdvancedCustom.Routes {
		if len(route.Models) == 0 {
			return nil, invalid
		}
		for _, rawName := range route.Models {
			name := strings.TrimSpace(rawName)
			if name == "" || strings.HasPrefix(name, "re:") {
				return nil, invalid
			}
			for _, contract := range spec.contracts {
				if name != contract.model {
					continue
				}
				if _, duplicate := selected[name]; duplicate || len(route.Models) != 1 || rawName != name ||
					route.IncomingPath != "/v1/chat/completions" || route.UpstreamPath != contract.path ||
					route.UpstreamMethod != http.MethodGet || route.Converter != "none" ||
					!route.ShouldPassthroughResponse() || route.PassThroughBodyEnabled {
					return nil, invalid
				}
				query := make(map[string]string, len(contract.fields))
				for _, field := range contract.fields {
					if source, exists := route.QueryMapping[field]; exists {
						if source != field {
							return nil, invalid
						}
						query[field] = field
					}
				}
				hasRequired := false
				for _, required := range contract.requiredFields {
					if query[required] != "" {
						hasRequired = true
						break
					}
				}
				if !hasRequired {
					return nil, invalid
				}
				selected[name] = dto.AdvancedCustomRoute{
					IncomingPath: "/v1/chat/completions", UpstreamPath: contract.path, Converter: "none", Models: []string{name},
					UpstreamMethod: http.MethodGet, QueryMapping: query, Auth: route.Auth,
					PassthroughResponse: common.GetPointer(true), ResponseValidation: contract.policy,
				}
			}
		}
	}
	config := &dto.AdvancedCustomConfig{Routes: make([]dto.AdvancedCustomRoute, 0, len(spec.contracts))}
	models := make([]string, 0, len(spec.contracts))
	originModels := origin.GetModels()
	for _, contract := range spec.contracts {
		route, found := selected[contract.model]
		if !found || !slices.Contains(originModels, contract.model) {
			return nil, invalid
		}
		config.Routes = append(config.Routes, route)
		models = append(models, contract.model)
	}
	if err := config.Validate(); err != nil {
		return nil, invalid
	}
	encoded, err := common.Marshal(dto.ChannelOtherSettings{AdvancedCustom: config})
	if err != nil {
		return nil, invalid
	}
	clone := *origin
	clone.Name = spec.name
	clone.Status = common.ChannelStatusManuallyDisabled
	clone.Group = "social-data-tikhub"
	clone.Tag = common.GetPointer("data-api")
	if spec.disableAutoBan {
		clone.AutoBan = common.GetPointer(0)
	}
	clone.Models = strings.Join(models, ",")
	clone.OtherSettings = string(encoded)
	clone.Setting = nil
	clone.OpenAIOrganization = nil
	clone.TestModel = nil
	clone.ModelMapping = nil
	clone.ParamOverride = nil
	clone.HeaderOverride = nil
	clone.StatusCodeMapping = nil
	clone.Other = ""
	clone.OtherInfo = ""
	clone.Remark = nil
	clone.Balance = 0
	clone.BalanceUpdatedTime = 0
	clone.UsedQuota = 0
	return &clone, nil
}

// accountMonitorChannelCopy never calls mutating saved-setting getters. Only the
// four fixed GET contracts and their server-side credentials survive the copy.
func accountMonitorChannelCopy(origin *model.Channel) (*model.Channel, error) {
	return narrowTikHubChannelCopy(origin, tikHubScopedCopySpec{
		name:       "TikHub 账号监控",
		errMessage: "channel does not support account monitor copying",
		contracts: []tikHubRouteContract{
			{"x-user", "/api/v1/twitter/web/fetch_user_profile", dto.AdvancedCustomResponseValidationTikHubUser, []string{"screen_name", "rest_id"}, []string{"screen_name", "rest_id"}},
			{"x-posts", "/api/v1/twitter/web/fetch_user_post_tweet", dto.AdvancedCustomResponseValidationTikHubPosts, []string{"screen_name", "rest_id", "cursor"}, []string{"screen_name", "rest_id"}},
			{"tiktok-user", "/api/v1/tiktok/web/fetch_user_profile", dto.AdvancedCustomResponseValidationTikHubUser, []string{"uniqueId", "secUid"}, []string{"uniqueId", "secUid"}},
			{"tiktok-posts", "/api/v1/tiktok/app/v3/fetch_user_post_videos", dto.AdvancedCustomResponseValidationTikHubPosts, []string{"unique_id", "sec_user_id", "count", "max_cursor"}, []string{"unique_id", "sec_user_id"}},
		},
	})
}

// fork: tiktokShopChannelCopy narrows a disabled TikHub source channel to the
// six core TikTok Shop / creator GET routes under social-data-tikhub, keeping
// credentials server-side, status ManuallyDisabled, Tag = "data-api", and AutoBan = 0.
func tiktokShopChannelCopy(origin *model.Channel) (*model.Channel, error) {
	return narrowTikHubChannelCopy(origin, tikHubScopedCopySpec{
		name:           "TikHub TikTok Shop 选品",
		errMessage:     "channel does not support tiktok shop copying",
		disableAutoBan: true,
		contracts: []tikHubRouteContract{
			{"tiktok-shop-search-v2", "/api/v1/tiktok/shop/web/fetch_search_products_list_v2", "", []string{"search_word", "offset", "page_token", "region"}, []string{"search_word"}},
			{"tiktok-shop-product-v3", "/api/v1/tiktok/shop/web/fetch_product_detail_v3", "", []string{"product_id", "region"}, []string{"product_id"}},
			{"tiktok-shop-reviews-v2", "/api/v1/tiktok/shop/web/fetch_product_reviews_v2", "", []string{"product_id", "page_start", "sort_rule", "filter_type", "filter_value", "region"}, []string{"product_id"}},
			{"tiktok-creator-search-videos", "/api/v1/tiktok/app/v3/fetch_creator_search_insights_videos", "", []string{"keyword", "offset", "count"}, []string{"keyword"}},
			{"tiktok-shop-product-link", "/api/v1/tiktok/shop/web/fetch_product_id_by_share_link", "", []string{"share_link"}, []string{"share_link"}},
			{"tiktok-shop-product", "/api/v1/tiktok/shop/web/fetch_product_detail", "", []string{"product_id", "seller_id", "region"}, []string{"product_id"}},
		},
	})
}

// fork: tiktokToolChannelCopy narrows a disabled TikHub source to the single
// tiktok-video GET route used by the public tool pages. The copy stays
// disabled and lives in its own group, so it never serves relay traffic for
// ordinary groups; credentials stay server-side.
func tiktokToolChannelCopy(origin *model.Channel) (*model.Channel, error) {
	invalid := errors.New("channel does not support tiktok tool copying")
	if origin == nil || origin.Type != constant.ChannelTypeAdvancedCustom || origin.BaseURL == nil || *origin.BaseURL != "https://api.tikhub.io" {
		return nil, invalid
	}
	switch origin.Status {
	case common.ChannelStatusUnknown, common.ChannelStatusManuallyDisabled, common.ChannelStatusAutoDisabled:
	default:
		return nil, invalid
	}
	if !slices.Contains(origin.GetModels(), tiktokToolModel) {
		return nil, invalid
	}
	var settings dto.ChannelOtherSettings
	if err := common.Unmarshal([]byte(origin.OtherSettings), &settings); err != nil || settings.AdvancedCustom == nil {
		return nil, invalid
	}
	var selected *dto.AdvancedCustomRoute
	for _, route := range settings.AdvancedCustom.Routes {
		if !slices.Contains(route.Models, tiktokToolModel) {
			continue
		}
		if selected != nil || len(route.Models) != 1 || route.UpstreamPath != "/api/v1/tiktok/app/v3/fetch_one_video" ||
			route.UpstreamMethod != http.MethodGet || route.QueryMapping["aweme_id"] != "aweme_id" {
			return nil, invalid
		}
		selected = &dto.AdvancedCustomRoute{
			IncomingPath: "/v1/chat/completions", UpstreamPath: route.UpstreamPath, Converter: "none",
			Models: []string{tiktokToolModel}, UpstreamMethod: http.MethodGet,
			QueryMapping: map[string]string{"aweme_id": "aweme_id"}, Auth: route.Auth,
			PassthroughResponse: common.GetPointer(true),
		}
	}
	if selected == nil {
		return nil, invalid
	}
	config := &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{*selected}}
	if err := config.Validate(); err != nil {
		return nil, invalid
	}
	encoded, err := common.Marshal(dto.ChannelOtherSettings{AdvancedCustom: config})
	if err != nil {
		return nil, invalid
	}
	clone := *origin
	clone.Name = "TikHub TikTok 工具页"
	clone.Status = common.ChannelStatusManuallyDisabled
	clone.Group = "tiktok-tool"
	clone.Tag = common.GetPointer("data-api")
	clone.Models = tiktokToolModel
	clone.OtherSettings = string(encoded)
	clone.Setting = nil
	clone.OpenAIOrganization = nil
	clone.TestModel = nil
	clone.ModelMapping = nil
	clone.ParamOverride = nil
	clone.HeaderOverride = nil
	clone.StatusCodeMapping = nil
	clone.Other = ""
	clone.OtherInfo = ""
	clone.Remark = nil
	clone.Balance = 0
	clone.BalanceUpdatedTime = 0
	clone.UsedQuota = 0
	return &clone, nil
}
