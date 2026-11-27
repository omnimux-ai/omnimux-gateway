package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"gorm.io/gorm"
)

type PricingPluginVariant struct {
	PluginKey            string                               `json:"plugin_key"`
	PluginName           string                               `json:"plugin_name"`
	Icon                 string                               `json:"icon,omitempty"`
	BillingExpr          string                               `json:"billing_expr"`
	BillingMode          string                               `json:"billing_mode"`
	BillingUsageSchema   map[string]jsplugin.UsageFieldSchema `json:"billing_usage_schema"`
	BillingUsageExamples []jsplugin.UsageExample              `json:"billing_usage_examples,omitempty"`
}

type Pricing struct {
	Id                    int                    `json:"id,omitempty"`
	BillingPluginVariants []PricingPluginVariant `json:"billing_plugin_variants,omitempty"`
	ModelName             string                 `json:"model_name"`
	Description           string                 `json:"description,omitempty"`
	Icon                  string                 `json:"icon,omitempty"`
	Tags                  string                 `json:"tags,omitempty"`
	VendorID              int                    `json:"vendor_id,omitempty"`
	// fork: OmniMux — canonical cross-client brand identity projected from
	// modelcontracts. Strictly presentational; never feeds billing or routing.
	BrandID                string                               `json:"brand_id,omitempty"`
	BrandName              string                               `json:"brand_name,omitempty"`
	QuotaType              int                                  `json:"quota_type"`
	ModelRatio             float64                              `json:"model_ratio"`
	ModelPrice             float64                              `json:"model_price"`
	OwnerBy                string                               `json:"owner_by"`
	CompletionRatio        float64                              `json:"completion_ratio"`
	CacheRatio             *float64                             `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                             `json:"create_cache_ratio,omitempty"`
	ImageRatio             *float64                             `json:"image_ratio,omitempty"`
	AudioRatio             *float64                             `json:"audio_ratio,omitempty"`
	AudioCompletionRatio   *float64                             `json:"audio_completion_ratio,omitempty"`
	EnableGroup            []string                             `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType              `json:"supported_endpoint_types"`
	BillingMode            string                               `json:"billing_mode,omitempty"`
	BillingExpr            string                               `json:"billing_expr,omitempty"`
	BillingUsageSchema     map[string]jsplugin.UsageFieldSchema `json:"billing_usage_schema,omitempty"`
	BillingUsageExamples   []jsplugin.UsageExample              `json:"billing_usage_examples,omitempty"`
	PricingVersion         string                               `json:"pricing_version,omitempty"`
	CreatedTime            int64                                `json:"created_time,omitempty"`
	UpdatedTime            int64                                `json:"updated_time,omitempty"`

	// Contract publishes the model's declared capability envelope, projected
	// from the governance specs by package modelcontracts.
	//
	// fork: OmniMux — the catalog used to publish a price and a free-text `tags`
	// string and nothing else, so the public model page had to infer every
	// capability (duration, resolution, aspect ratio, audio support, reference
	// limits) and inferred wrong for 86 of 89 live models. The presence of a
	// live executable contract is the catalog publication gate; its values are
	// descriptive only and must never feed pricing, quota, precharge or
	// settlement. See FORK_CUSTOMIZATIONS.md.
	Contract       *modelcontracts.Contract `json:"contract,omitempty"`
	EffectiveVideo *EffectiveVideo          `json:"effective_video,omitempty"`

	// The shared pricing cache keeps channel variants for all served groups.
	// Only the caller-scoped projection is serialized.
	videoOperationsByGroup map[string][]modelcontracts.Operation
}

type EffectiveVideo struct {
	Version string                                `json:"version"`
	Groups  map[string][]modelcontracts.Operation `json:"groups"`
}

// VideoCapabilitiesForGroups projects the currently served variants scoped to
// the same groups that the caller can select and see prices for.
func (p Pricing) VideoCapabilitiesForGroups(groupRatios map[string]float64) *EffectiveVideo {
	if p.Contract == nil || p.Contract.GatewayProtocol != modelcontracts.VideoProtocolGenerations &&
		!modelcontracts.ActiveVideoCapabilities().Managed(p.ModelName) {
		return nil
	}
	groups := make(map[string][]modelcontracts.Operation)
	for group, operations := range p.videoOperationsByGroup {
		if _, visible := groupRatios[group]; visible && group != "" && group != "auto" {
			groups[group] = operations
		}
	}
	content, err := common.Marshal(groups)
	if err != nil {
		common.SysLog(common.LogText("Could not encode video capabilities: %v", err))
		return nil
	}
	version := sha256.New()
	version.Write([]byte(p.Contract.ContractVersion))
	version.Write([]byte{0})
	version.Write(content)
	return &EffectiveVideo{Version: hex.EncodeToString(version.Sum(nil)[:8]), Groups: groups}
}

type PricingVendor struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

var (
	pricingMap             []Pricing
	vendorsList            []PricingVendor
	supportedEndpointMap   map[string]common.EndpointInfo
	lastGetPricingTime     time.Time
	updatePricingLock      sync.Mutex
	videoPricingGeneration atomic.Pointer[jsplugin.RoutingGeneration]

	// 缓存映射：模型名 -> 启用分组 / 计费类型
	modelEnableGroups     = make(map[string][]string)
	modelQuotaTypeMap     = make(map[string]int)
	modelEnableGroupsLock = sync.RWMutex{}

	contractGateUnavailableOnce sync.Once
)

var (
	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	modelSupportEndpointsLock = sync.RWMutex{}
)

func pricingCacheExpired() bool {
	if time.Since(lastGetPricingTime) > time.Minute || len(pricingMap) == 0 {
		return true
	}
	return modelcontracts.ActiveVideoCapabilities() != nil &&
		videoPricingGeneration.Load() != jsplugin.DefaultRegistry.Generation()
}

func GetPricing() []Pricing {
	if pricingCacheExpired() {
		updatePricingLock.Lock()
		defer updatePricingLock.Unlock()
		// Double check after acquiring the lock
		if pricingCacheExpired() {
			modelSupportEndpointsLock.Lock()
			defer modelSupportEndpointsLock.Unlock()
			updatePricing()
		}
	}
	return pricingMap
}

// fork: OmniMux — public catalog publication gate.
//
// GetCatalogPricing returns the public catalog subset of the served pricing
// rows. A served model is published only when its governance contract is live
// with at least one executable operation; a contractless model stays routable
// but unlisted.
//
// Self-use deployments and OMNIMUX_MODEL_CATALOG_ALLOW_UNCONTRACTED keep the
// legacy behavior. If the shipped contract payload is unreadable, the served
// catalog is left intact rather than blanked.
func GetCatalogPricing() []Pricing {
	return filterCatalogPricing(GetPricing())
}

// readCurrentCatalogPricing validates against current catalog inputs. It leaves
// the shared pricing cache and its legacy group ordering untouched.
func readCurrentCatalogPricing(ctx context.Context) ([]Pricing, error) {
	snapshot, err := readPricingSnapshot(DB.WithContext(ctx), true)
	if err != nil {
		return nil, err
	}
	return filterCatalogPricing(snapshot.rows), nil
}

func filterCatalogPricing(pricing []Pricing) []Pricing {
	if !catalogContractGateEnabled() {
		return pricing
	}
	if modelcontracts.Count() == 0 {
		contractGateUnavailableOnce.Do(func() {
			common.SysError("model catalog: contract payload is empty; publishing the served catalog without the contract gate")
		})
		return pricing
	}
	published := make([]Pricing, 0, len(pricing))
	for _, row := range pricing {
		if isCatalogPublished(row) {
			published = append(published, row)
		}
	}
	return published
}

// GetCatalogGaps returns served model names that fail the publication gate.
// The list is sorted for deterministic logs and diagnostics.
func GetCatalogGaps() []string {
	if !catalogContractGateEnabled() || modelcontracts.Count() == 0 {
		return nil
	}
	gaps := make([]string, 0)
	for _, row := range GetPricing() {
		if !isCatalogPublished(row) {
			gaps = append(gaps, row.ModelName)
		}
	}
	sort.Strings(gaps)
	return gaps
}

func isCatalogPublished(row Pricing) bool {
	if row.Contract == nil || !row.Contract.IsLiveExecutable() {
		return false
	}
	// A newly declared native decoder profile needs runtime channel rules to
	// back its executable contract. The legacy catalog keeps its own behavior.
	return row.Contract.GatewayProtocol != modelcontracts.VideoProtocolGenerations ||
		modelcontracts.ActiveVideoCapabilities().Managed(row.ModelName)
}

// catalogContractGateEnabled reports whether the public catalog publication
// gate is active. Self-use mode serves bring-your-own catalogs, and the env
// var is an explicit emergency/BYO escape hatch.
func catalogContractGateEnabled() bool {
	if operation_setting.SelfUseModeEnabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OMNIMUX_MODEL_CATALOG_ALLOW_UNCONTRACTED"))) {
	case "1", "true", "yes":
		return false
	default:
		return true
	}
}

func InvalidatePricingCache() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	pricingMap = nil
	vendorsList = nil
	lastGetPricingTime = time.Time{}
}

// GetVendors 返回当前定价接口使用到的供应商信息
func GetVendors() []PricingVendor {
	if time.Since(lastGetPricingTime) > time.Minute*1 || len(pricingMap) == 0 {
		// 保证先刷新一次
		GetPricing()
	}
	return vendorsList
}

func GetModelSupportEndpointTypes(model string) []constant.EndpointType {
	if model == "" {
		return make([]constant.EndpointType, 0)
	}
	modelSupportEndpointsLock.RLock()
	defer modelSupportEndpointsLock.RUnlock()
	if endpoints, ok := modelSupportEndpointTypes[model]; ok {
		return endpoints
	}
	return make([]constant.EndpointType, 0)
}

func getPricingEndpointTypesForAbility(ability AbilityWithChannel, advancedCustomConfigs map[int]*dto.AdvancedCustomConfig) []constant.EndpointType {
	if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
		return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
	}
	if config := advancedCustomConfigs[ability.ChannelId]; config != nil {
		return config.SupportedEndpointTypesForModel(ability.Model)
	}
	return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
}

// loadPricingAdvancedCustomConfigs runs inside updatePricing while
// updatePricingLock is held, and nests channelSyncLock.RLock. This defines the
// global lock order updatePricingLock -> channelSyncLock: any code path holding
// channelSyncLock must release it before touching the pricing cache (see
// InitChannelCache / CacheUpdateChannel), otherwise it deadlocks.
// The returned configs are pointers shared with the channel cache; they are
// replaced wholesale on update and never mutated in place, so reading them after
// RUnlock is safe.
func loadPricingAdvancedCustomConfigs(db *gorm.DB, enableAbilities []AbilityWithChannel, requireComplete bool) (map[int]*dto.AdvancedCustomConfig, error) {
	channelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, ability := range enableAbilities {
		if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	configs := make(map[int]*dto.AdvancedCustomConfig, len(channelIDs))
	if requireComplete {
		// Read the stored settings directly. The legacy getter can repair
		// malformed settings in the database as a side effect of reading.
		var channels []Channel
		if err := db.Select("id", "settings").Where("id IN ? AND type = ?", channelIDs, constant.ChannelTypeAdvancedCustom).Find(&channels).Error; err != nil {
			return nil, fmt.Errorf("load advanced custom channel settings: %w", err)
		}
		for _, channel := range channels {
			if channel.OtherSettings == "" {
				continue
			}
			var settings dto.ChannelOtherSettings
			if err := common.UnmarshalJsonStr(channel.OtherSettings, &settings); err != nil {
				return nil, fmt.Errorf("decode advanced custom channel %d settings: %w", channel.Id, err)
			}
			if settings.AdvancedCustom != nil {
				configs[channel.Id] = settings.AdvancedCustom
			}
		}
		return configs, nil
	}
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		for _, channelID := range channelIDs {
			if config := channel2advancedCustomConfig[channelID]; config != nil {
				configs[channelID] = config
			}
		}
		return configs, nil
	}

	for _, channelID := range channelIDs {
		channel, err := CacheGetChannel(channelID)
		if err != nil {
			common.SysLog(common.LogText("load advanced custom channel settings error: channel_id=%d, error=%v", channelID, err))
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			configs[channelID] = config
		}
	}
	return configs, nil
}

// loadPricingVideoOperations uses the same model/group membership and channel
// rules as selection. The lock order matches loadPricingAdvancedCustomConfigs.
func loadPricingVideoOperations(db *gorm.DB, enableAbilities []AbilityWithChannel, generation *jsplugin.RoutingGeneration, useCache bool) (map[string]map[string][]modelcontracts.Operation, error) {
	rules := modelcontracts.ActiveVideoCapabilities()
	if rules == nil {
		return nil, nil
	}
	var managedAbilities []AbilityWithChannel
	channelIDs := make(map[int]struct{})
	for _, ability := range enableAbilities {
		if rules.Managed(ability.Model) {
			managedAbilities = append(managedAbilities, ability)
			channelIDs[ability.ChannelId] = struct{}{}
		}
	}
	if len(managedAbilities) == 0 {
		return nil, nil
	}

	var channelsByID map[int]*Channel
	if useCache {
		channelSyncLock.RLock()
		channelsByID = channelsIDM
	} else {
		var channels []*Channel
		if err := db.Select("id", "type", "status", "base_url", "model_mapping", "param_override", "settings", "setting").
			Where("id IN ? AND status = ?", slices.Collect(maps.Keys(channelIDs)), common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
			return nil, err
		}
		channelsByID = make(map[int]*Channel, len(channels))
		for _, channel := range channels {
			channelsByID[channel.Id] = channel
		}
	}

	operationsByModel := make(map[string]map[string][]modelcontracts.Operation)
	for _, ability := range managedAbilities {
		channel := channelsByID[ability.ChannelId]
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if useCache && !slices.Contains(group2model2channels[ability.Group][ability.Model], ability.ChannelId) {
			continue
		}
		groups := operationsByModel[ability.Model]
		if groups == nil {
			groups = make(map[string][]modelcontracts.Operation)
			operationsByModel[ability.Model] = groups
		}
		groups[ability.Group] = append(groups[ability.Group], videoChannelOperations(rules, generation, channel, ability.Model)...)
	}
	if useCache {
		channelSyncLock.RUnlock()
	}

	for modelName, groups := range operationsByModel {
		for group, operations := range groups {
			if rules.HasSeedance25OpenAI(modelName) || rules.HasSeedanceMultimodalOpenAI(modelName) {
				projectSeedanceEconomyResolution(operations)
			}
			unique := make(map[string]modelcontracts.Operation, len(operations))
			for _, operation := range operations {
				encoded, err := common.Marshal(operation)
				if err == nil {
					unique[string(encoded)] = operation
				}
			}
			ordered := make([]modelcontracts.Operation, 0, len(unique))
			for _, key := range slices.Sorted(maps.Keys(unique)) {
				ordered = append(ordered, unique[key])
			}
			groups[group] = ordered
		}
	}
	return operationsByModel, nil
}

func appendPricingEndpoint(endpoints []string, endpoint string) []string {
	if endpoint == "" || common.StringsContains(endpoints, endpoint) {
		return endpoints
	}
	return append(endpoints, endpoint)
}

type pricingSnapshot struct {
	rows          []Pricing
	vendors       []PricingVendor
	endpointTypes map[string][]constant.EndpointType
	endpointMap   map[string]common.EndpointInfo
	generation    *jsplugin.RoutingGeneration
}

func readPricingSnapshot(db *gorm.DB, requireComplete bool) (pricingSnapshot, error) {
	enableAbilities, err := getAllEnableAbilityWithChannels(db)
	if err != nil {
		return pricingSnapshot{}, fmt.Errorf("load enabled abilities: %w", err)
	}
	// 预加载模型元数据与供应商一次，避免循环查询
	var allMeta []Model
	if err := db.Find(&allMeta).Error; err != nil && requireComplete {
		return pricingSnapshot{}, fmt.Errorf("load model metadata: %w", err)
	}
	names := make([]string, 0, len(enableAbilities))
	for _, ability := range enableAbilities {
		names = append(names, ability.Model)
	}
	metaMap := resolveModelMetadata(allMeta, names)

	// 预加载供应商
	var vendors []Vendor
	if err := db.Find(&vendors).Error; err != nil && requireComplete {
		return pricingSnapshot{}, fmt.Errorf("load catalog vendors: %w", err)
	}
	vendorMap := make(map[int]*Vendor)
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	// 初始化默认供应商映射
	initDefaultVendorMapping(metaMap, vendorMap, enableAbilities)

	// 构建对前端友好的供应商列表
	vendorList := make([]PricingVendor, 0, len(vendorMap))
	for _, v := range vendorMap {
		vendorList = append(vendorList, PricingVendor{
			ID:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			Icon:        v.Icon,
		})
	}

	modelGroupsMap := make(map[string]*types.Set[string])

	for _, ability := range enableAbilities {
		groups, ok := modelGroupsMap[ability.Model]
		if !ok {
			groups = types.NewSet[string]()
			modelGroupsMap[ability.Model] = groups
		}
		groups.Add(ability.Group)
	}

	//这里使用切片而不是Set，因为一个模型可能支持多个端点类型，并且第一个端点是优先使用端点
	modelSupportEndpointsStr := make(map[string][]string)
	advancedCustomConfigs, err := loadPricingAdvancedCustomConfigs(db, enableAbilities, requireComplete)
	if err != nil {
		return pricingSnapshot{}, err
	}
	pluginGeneration := jsplugin.DefaultRegistry.Generation()
	videoOperationsByModel, err := loadPricingVideoOperations(db, enableAbilities, pluginGeneration, common.MemoryCacheEnabled && !requireComplete)
	if err != nil {
		if requireComplete {
			return pricingSnapshot{}, fmt.Errorf("load channels for video capabilities: %w", err)
		}
		common.SysLog(common.LogText("Could not load channels for video capabilities: %v", err))
	}

	// 先根据已有能力填充原生端点
	for _, ability := range enableAbilities {
		endpoints := modelSupportEndpointsStr[ability.Model]
		channelTypes := getPricingEndpointTypesForAbility(ability, advancedCustomConfigs)
		for _, channelType := range channelTypes {
			if !common.StringsContains(endpoints, string(channelType)) {
				endpoints = append(endpoints, string(channelType))
			}
		}
		modelSupportEndpointsStr[ability.Model] = endpoints
	}

	// 再补充模型自定义端点：若配置有效则追加到已有推断，不再裁剪渠道真实能力
	for modelName, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			endpoints := modelSupportEndpointsStr[modelName]
			for k, v := range raw {
				switch v.(type) {
				case string, map[string]any:
					endpoints = appendPricingEndpoint(endpoints, k)
				}
			}
			if len(endpoints) > 0 {
				modelSupportEndpointsStr[modelName] = endpoints
			}
		}
	}

	endpointTypes := make(map[string][]constant.EndpointType)
	for model, endpoints := range modelSupportEndpointsStr {
		supportedEndpoints := make([]constant.EndpointType, 0)
		for _, endpointStr := range endpoints {
			endpointType := constant.EndpointType(endpointStr)
			supportedEndpoints = append(supportedEndpoints, endpointType)
		}
		endpointTypes[model] = supportedEndpoints
	}

	// 构建全局 endpointMap（默认 + 自定义覆盖）
	endpointMap := make(map[string]common.EndpointInfo)
	// 1. 默认端点
	for _, endpoints := range endpointTypes {
		for _, et := range endpoints {
			if info, ok := common.GetDefaultEndpointInfo(et); ok {
				if _, exists := endpointMap[string(et)]; !exists {
					endpointMap[string(et)] = info
				}
			}
		}
	}
	// 2. 自定义端点（models 表）覆盖默认
	for _, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			for k, v := range raw {
				switch val := v.(type) {
				case string:
					endpointMap[k] = common.EndpointInfo{Path: val, Method: "POST"}
				case map[string]any:
					ep := common.EndpointInfo{Method: "POST"}
					if p, ok := val["path"].(string); ok {
						ep.Path = p
					}
					if m, ok := val["method"].(string); ok {
						ep.Method = strings.ToUpper(m)
					}
					endpointMap[k] = ep
				default:
					// ignore unsupported types
				}
			}
		}
	}

	rows := make([]Pricing, 0)
	for model, groups := range modelGroupsMap {
		enabledGroups := groups.Items()
		slices.Sort(enabledGroups)
		pricing := Pricing{
			ModelName:              model,
			EnableGroup:            enabledGroups,
			SupportedEndpointTypes: endpointTypes[model],
			videoOperationsByGroup: videoOperationsByModel[model],
		}

		// fork: OmniMux — publish the declared capability contract next to the
		// price. A model without a governance spec simply keeps the field absent,
		// which leaves the row byte-identical to before this field existed.
		if contract, ok := modelcontracts.For(model); ok {
			pricing.Contract = &contract
			pricing.BrandID = contract.BrandID
			pricing.BrandName = contract.BrandName
		}

		// 补充模型元数据（描述、标签、供应商、状态、入库更新时间）
		if meta, ok := metaMap[model]; ok {
			// 若模型被禁用(status!=1)，则直接跳过，不返回给前端
			if meta.Status != 1 {
				continue
			}
			pricing.Id = meta.Id
			pricing.Description = meta.Description
			pricing.Icon = meta.Icon
			pricing.Tags = meta.Tags
			pricing.VendorID = meta.VendorID
			pricing.CreatedTime = meta.CreatedTime
			pricing.UpdatedTime = meta.UpdatedTime
		}
		modelPrice, findPrice := ratio_setting.GetModelPrice(model, false)
		if findPrice {
			pricing.ModelPrice = modelPrice
			pricing.QuotaType = 1
		} else {
			modelRatio, _, _ := ratio_setting.GetModelRatio(model)
			pricing.ModelRatio = modelRatio
			pricing.CompletionRatio = ratio_setting.GetCompletionRatio(model)
			pricing.QuotaType = 0
		}
		if cacheRatio, ok := ratio_setting.GetCacheRatio(model); ok {
			pricing.CacheRatio = &cacheRatio
		}
		if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(model); ok {
			pricing.CreateCacheRatio = &createCacheRatio
		}
		if imageRatio, ok := ratio_setting.GetImageRatio(model); ok {
			pricing.ImageRatio = &imageRatio
		}
		if ratio_setting.ContainsAudioRatio(model) {
			audioRatio := ratio_setting.GetAudioRatio(model)
			pricing.AudioRatio = &audioRatio
		}
		if ratio_setting.ContainsAudioCompletionRatio(model) {
			audioCompletionRatio := ratio_setting.GetAudioCompletionRatio(model)
			pricing.AudioCompletionRatio = &audioCompletionRatio
		}
		billingMode := billing_setting.GetBillingMode(model)
		if billingMode == billing_setting.BillingModeTieredExpr {
			if expr, ok := billing_setting.GetBillingExpr(model); ok && strings.TrimSpace(expr) != "" {
				pricing.BillingMode = billingMode
				pricing.BillingExpr = expr
			}
			// fork: OmniMux — expose the native image service expression as a read-only
			// legacy-field projection. A broken service expression has no public quote.
			if model == "gpt-image-2.5-sunburst" && !projectSunburstImagePrice(&pricing) {
				continue
			}
		} else if !requireComplete {
			if target, resolved := ResolveTaskModelAlias(pluginGeneration, model); resolved && target.Declared != "" {
				if tailMode := billing_setting.GetBillingMode(target.Declared); tailMode == billing_setting.BillingModeTieredExpr {
					if expr, ok := billing_setting.GetBillingExpr(target.Declared); ok && strings.TrimSpace(expr) != "" {
						pricing.BillingMode = tailMode
						pricing.BillingExpr = expr
					}
				}
			}
		}
		if requireComplete {
			// Alias and plugin price metadata only enrich the public response.
			// Resolving aliases can rebuild a cache via repairing channel getters.
			rows = append(rows, pricing)
			continue
		}
		usageModel := model
		plugin, ok := pluginGeneration.GetByModel(model)
		if !ok {
			if target, resolved := ResolveTaskModelAlias(pluginGeneration, model); resolved {
				plugin, ok = pluginGeneration.Get(target.PluginKey)
				usageModel = target.Declared
			}
		}
		if ok && plugin != nil {
			usageSchema, usageExamples := plugin.Meta.UsageForModel(usageModel)
			pricing.BillingUsageSchema = jsplugin.CloneUsageSchema(usageSchema)
			pricing.BillingUsageExamples = jsplugin.CloneUsageExamples(usageExamples)
		}
		providers := pluginGeneration.PluginsByModel(model)
		hasProviderOverride := false
		for _, provider := range providers {
			if _, configured := billing_setting.GetPluginBillingExpr(provider.Meta.Key, model); configured {
				hasProviderOverride = true
				break
			}
		}
		if hasProviderOverride || (len(providers) >= 2 && pricing.BillingMode == billing_setting.BillingModeTieredExpr) {
			for _, provider := range providers {
				schema, examples := provider.Meta.UsageForModel(model)
				if schema == nil {
					schema = map[string]jsplugin.UsageFieldSchema{}
				}
				expression, hasExpression := billing_setting.ResolveTaskBillingExpr(provider.Meta.Key, model, "")
				mode := billing_setting.BillingModeRatio
				if hasExpression || billing_setting.GetBillingMode(model) == billing_setting.BillingModeTieredExpr {
					mode = billing_setting.BillingModeTieredExpr
				}
				if mode == billing_setting.BillingModeTieredExpr && !billing_setting.TaskExprCompatible(expression, schema) {
					expression = ""
				}
				pricing.BillingPluginVariants = append(pricing.BillingPluginVariants, PricingPluginVariant{
					PluginKey: provider.Meta.Key, PluginName: provider.Meta.Name, Icon: provider.Meta.Icon,
					BillingExpr: expression, BillingMode: mode,
					BillingUsageSchema: jsplugin.CloneUsageSchema(schema), BillingUsageExamples: jsplugin.CloneUsageExamples(examples),
				})
			}
		}
		rows = append(rows, pricing)
	}

	// 按更新时间、创建时间、ID 倒序，完全相同则按模型名升序。
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].UpdatedTime != rows[j].UpdatedTime {
			return rows[i].UpdatedTime > rows[j].UpdatedTime
		}
		if rows[i].CreatedTime != rows[j].CreatedTime {
			return rows[i].CreatedTime > rows[j].CreatedTime
		}
		if rows[i].Id != rows[j].Id {
			return rows[i].Id > rows[j].Id
		}
		return rows[i].ModelName < rows[j].ModelName
	})

	// 防止大更新后数据不通用
	if len(rows) > 0 {
		rows[0].PricingVersion = "5a90f2b86c08bd983a9a2e6d66c255f4eaef9c4bc934386d2b6ae84ef0ff1f1f"
	}

	return pricingSnapshot{
		rows: rows, vendors: vendorList, endpointTypes: endpointTypes,
		endpointMap: endpointMap, generation: pluginGeneration,
	}, nil
}

func updatePricing() {
	snapshot, err := readPricingSnapshot(DB, false)
	if err != nil {
		common.SysLog(common.LogText("Could not load pricing: %v", err))
		return
	}
	pricingMap = snapshot.rows
	vendorsList = snapshot.vendors
	modelSupportEndpointTypes = snapshot.endpointTypes
	supportedEndpointMap = snapshot.endpointMap

	// 刷新缓存映射，供高并发快速查询
	modelEnableGroupsLock.Lock()
	modelEnableGroups = make(map[string][]string)
	modelQuotaTypeMap = make(map[string]int)
	for _, p := range pricingMap {
		modelEnableGroups[p.ModelName] = p.EnableGroup
		modelQuotaTypeMap[p.ModelName] = p.QuotaType
	}
	modelEnableGroupsLock.Unlock()

	lastGetPricingTime = time.Now()
	videoPricingGeneration.Store(snapshot.generation)
}

// GetSupportedEndpointMap 返回全局端点到路径的映射
func GetSupportedEndpointMap() map[string]common.EndpointInfo {
	return supportedEndpointMap
}

// projectSunburstImagePrice accepts only the published single-image service
// shape. It does not write ModelPrice/ModelRatio or change billing selection.
func projectSunburstImagePrice(pricing *Pricing) bool {
	_, body := billingexpr.ParseExprVersion(pricing.BillingExpr)
	tree, err := parser.Parse(body)
	if err != nil {
		return false
	}
	product, ok := tree.Node.(*ast.BinaryNode)
	if !ok || product.Operator != "*" {
		return false
	}
	quantity, ok := product.Right.(*ast.IdentifierNode)
	if !ok || quantity.Value != "image_count" {
		return false
	}
	tier, ok := product.Left.(*ast.CallNode)
	if !ok || len(tier.Arguments) != 2 {
		return false
	}
	tierName, ok := tier.Callee.(*ast.IdentifierNode)
	if !ok || tierName.Value != "tier" {
		return false
	}
	label, ok := tier.Arguments[0].(*ast.StringNode)
	if !ok || label.Value != "image" {
		return false
	}
	fixed, ok := tier.Arguments[1].(*ast.CallNode)
	if !ok || len(fixed.Arguments) != 1 {
		return false
	}
	fixedName, ok := fixed.Callee.(*ast.IdentifierNode)
	if !ok || fixedName.Value != "fixed" {
		return false
	}
	switch fixed.Arguments[0].(type) {
	case *ast.IntegerNode, *ast.FloatNode:
	default:
		return false
	}
	cost, trace, err := billingexpr.RunExpr(pricing.BillingExpr, billingexpr.TokenParams{})
	if err != nil || trace.BillingUnit != billingexpr.BillingUnitRequest || trace.FixedPrice == nil ||
		math.IsNaN(cost) || math.IsInf(cost, 0) || cost < 0 {
		return false
	}
	price := cost / 1_000_000
	if _, clamp := common.QuotaRoundChecked(price * common.QuotaPerUnit); clamp != nil {
		return false
	}
	pricing.QuotaType = 1
	pricing.ModelPrice = price
	pricing.ModelRatio = 0
	return true
}
