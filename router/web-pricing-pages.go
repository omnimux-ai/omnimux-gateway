package router

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const pricingModelsPrefix = "/pricing/"

var (
	pricingVersionSnapshotPattern = regexp.MustCompile(`-[0-9]+\.[0-9]+\.[0-9]+$`)
	pricingDigitsTokenPattern     = regexp.MustCompile(`^[0-9]+$`)

	// publicPricingCatalog returns anonymous-public catalog models eligible for
	// public model detail pages; injectable for tests. Guard nil DB so isolated
	// router tests without database setup fail open rather than panicking.
	publicPricingCatalog = func() []model.Pricing {
		if model.DB == nil {
			return nil
		}
		return filterPublicPricingRows(model.GetCatalogPricing(), service.GetUserUsableGroups(""))
	}
)

var pricingWordDisplayMap = map[string]string{
	"ai":       "AI",
	"api":      "API",
	"asr":      "ASR",
	"auc":      "AUC",
	"bigasr":   "BigASR",
	"bigmodel": "BigModel",
	"gpt":      "GPT",
	"h3":       "H3",
	"indextts": "IndexTTS",
	"jev":      "JEV",
	"llm":      "LLM",
	"minimax":  "MiniMax",
	"seedasr":  "SeedASR",
	"tts":      "TTS",
}

var pricingOperationDisplayMap = map[string]string{
	"text_to_video":     "Text-to-Video",
	"first_frame":       "Image-to-Video (First Frame)",
	"first_last_frame":  "First & Last Frame Video",
	"video_multi_ref":   "Multi-Reference Video",
	"video_edit":        "Video Editing",
	"video_extend":      "Video Extension",
	"document_to_video": "Document-to-Video",
	"webpage_to_video":  "Webpage-to-Video",
	"text_to_image":     "Text-to-Image",
	"image_to_image":    "Image-to-Image",
	"chat":              "Chat & Reasoning",
	"vision_chat":       "Vision Chat",
	"text_to_music":     "Music Generation",
	"text_to_speech":    "Text-to-Speech",
	"speech_to_text":    "Speech-to-Text",
}

var pricingToInspirationZoneMap = map[string][]struct {
	Slug  string
	Label string
}{
	"seedance-2-5":       {{Slug: "seedance-2.5", Label: "Seedance 2.5 Prompts"}},
	"seedance-2-5-flash": {{Slug: "seedance-2.5", Label: "Seedance 2.5 Prompts"}},
	"seedance-2-0":       {{Slug: "seedance-2.0", Label: "Seedance 2.0 Prompts"}},
	"wan-3.0":            {{Slug: "wan-3.0", Label: "Wan 3.0 Prompts"}},
	"gpt-image-2.5": {
		{Slug: "gpt-image-2.5", Label: "GPT Image 2.5 Prompts"},
		{Slug: "gpt-image-2", Label: "GPT Image 2 Prompts"},
	},
	"nano-banana-2-1":    {{Slug: "nano-banana-pro", Label: "Nano Banana Pro Prompts"}},
	"nano-banana-2-lite": {{Slug: "nano-banana-pro", Label: "Nano Banana Pro Prompts"}},
}

type pricingGuideLink struct {
	URL   string
	Label string
}

var pricingToBlogGuidesMap = map[string][]pricingGuideLink{
	"seedance-2-5": {
		{URL: "/blog/seedance-2-5-prompting-timestamps-and-camera-guide", Label: "Seedance 2.5 Prompting, Timestamps & Camera Guide"},
		{URL: "/blog/seedance-2-5-vs-seedance-2-0-guide", Label: "Seedance 2.5 vs Seedance 2.0 Workflow Comparison"},
	},
	"seedance-2-5-flash": {
		{URL: "/blog/seedance-2-5-prompting-timestamps-and-camera-guide", Label: "Seedance 2.5 Prompting, Timestamps & Camera Guide"},
		{URL: "/blog/tiktok-shop-product-video-9-shot-framework", Label: "TikTok Shop 9-Shot Product Video Framework"},
	},
	"seedance-2-0": {
		{URL: "/blog/seedance-2-5-vs-seedance-2-0-guide", Label: "Seedance 2.5 vs Seedance 2.0 Workflow Comparison"},
		{URL: "/blog/best-ai-video-models-for-ecommerce", Label: "Best AI Video Models for Ecommerce"},
	},
	"wan-3.0": {
		{URL: "/blog/wan-3-0-pricing-and-product-video-guide", Label: "Wan 3.0 Pricing & Product Video Guide"},
		{URL: "/blog/best-ai-video-models-for-ecommerce", Label: "Best AI Video Models for Ecommerce"},
	},
	"magi-2": {
		{URL: "/blog/magi-2-and-muse-video-guide", Label: "Magi 2 & Muse Video Production Guide"},
		{URL: "/blog/ai-video-upscaler-and-pre-publish-enhancement", Label: "AI Video Enhancement & Pre-Publish Pipeline"},
	},
	"muse-video": {
		{URL: "/blog/magi-2-and-muse-video-guide", Label: "Magi 2 & Muse Video Production Guide"},
		{URL: "/blog/tiktok-shop-product-video-9-shot-framework", Label: "TikTok Shop 9-Shot Product Video Framework"},
	},
	"minimax-h3-video": {
		{URL: "/blog/minimax-h3-video-pricing-and-workflow", Label: "MiniMax H3 Video Pricing & Workflow"},
		{URL: "/blog/best-ai-video-models-for-ecommerce", Label: "Best AI Video Models for Ecommerce"},
	},
	"gpt-image-2.5": {
		{URL: "/blog/seedream-5-5-vs-nano-banana-2-1-vs-gpt-image-2-5", Label: "Seedream 5.5 vs Nano Banana 2.1 vs GPT Image 2.5"},
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"nano-banana-2-1": {
		{URL: "/blog/seedream-5-5-vs-nano-banana-2-1-vs-gpt-image-2-5", Label: "Seedream 5.5 vs Nano Banana 2.1 vs GPT Image 2.5"},
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"nano-banana-2-lite": {
		{URL: "/blog/seedream-5-5-vs-nano-banana-2-1-vs-gpt-image-2-5", Label: "Seedream 5.5 vs Nano Banana 2.1 vs GPT Image 2.5"},
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"seedream-5-5": {
		{URL: "/blog/seedream-5-5-vs-nano-banana-2-1-vs-gpt-image-2-5", Label: "Seedream 5.5 vs Nano Banana 2.1 vs GPT Image 2.5"},
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"seedream-5-0-pro": {
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"muse-image": {
		{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
	},
	"seed-tts-3.0": {
		{URL: "/blog/indextts-2-vs-seed-tts-3-0-voiceover-guide", Label: "IndexTTS 2 vs Seed TTS 3.0 Voiceover Guide"},
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"indextts-2": {
		{URL: "/blog/indextts-2-vs-seed-tts-3-0-voiceover-guide", Label: "IndexTTS 2 vs Seed TTS 3.0 Voiceover Guide"},
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"bigasr-auc": {
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"seedasr-auc": {
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"doubao-asr-bigmodel": {
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"suno": {
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"suno-sounds": {
		{URL: "/blog/ai-image-voice-and-asr-models-for-ecommerce", Label: "AI Image, Voice & ASR Models for Ecommerce"},
	},
	"gemini-4-carbon": {
		{URL: "/blog/gemini-4-and-multimodal-llms-for-video-breakdown", Label: "Gemini 4 & Multimodal LLMs for Video Breakdown"},
	},
	"gemini-4-neon": {
		{URL: "/blog/gemini-4-and-multimodal-llms-for-video-breakdown", Label: "Gemini 4 & Multimodal LLMs for Video Breakdown"},
	},
	"gemini-4-hydrogen": {
		{URL: "/blog/gemini-4-and-multimodal-llms-for-video-breakdown", Label: "Gemini 4 & Multimodal LLMs for Video Breakdown"},
	},
	"gemini-4-argon": {
		{URL: "/blog/gemini-4-and-multimodal-llms-for-video-breakdown", Label: "Gemini 4 & Multimodal LLMs for Video Breakdown"},
	},
	"gemini-3.8-flash": {
		{URL: "/blog/gemini-4-and-multimodal-llms-for-video-breakdown", Label: "Gemini 4 & Multimodal LLMs for Video Breakdown"},
	},
	"jev": {
		{URL: "/blog/connect-omnimux-cli-and-skills-to-ai-agents", Label: "Connect OmniMux CLI & Skills to AI Agents"},
	},
	"seed-evolving": {
		{URL: "/blog/connect-omnimux-cli-and-skills-to-ai-agents", Label: "Connect OmniMux CLI & Skills to AI Agents"},
	},
}

func filterPublicPricingRows(rows []model.Pricing, usableGroups map[string]string) []model.Pricing {
	if len(rows) == 0 || len(usableGroups) == 0 {
		return nil
	}
	filtered := make([]model.Pricing, 0, len(rows))
	for _, item := range rows {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroups[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

// isIndexablePricingModel keeps only canonical, anonymous-public, live-contracted
// models out of dated/preview version snapshots and multi-segment slugs.
func isIndexablePricingModel(row model.Pricing) bool {
	name := row.ModelName
	if name == "" || strings.TrimSpace(name) != name || strings.Contains(name, "/") ||
		strings.HasPrefix(name, "$") || !webAppRouteSegment.MatchString(name) {
		return false
	}
	if row.Contract == nil || !row.Contract.IsLiveExecutable() {
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "-latest") || strings.HasSuffix(lower, "-preview") ||
		pricingVersionSnapshotPattern.MatchString(lower) {
		return false
	}
	return true
}

func indexablePricingModels() []model.Pricing {
	raw := publicPricingCatalog()
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]model.Pricing, 0, len(raw))
	for _, row := range raw {
		if !isIndexablePricingModel(row) {
			continue
		}
		if _, exists := seen[row.ModelName]; exists {
			continue
		}
		seen[row.ModelName] = struct{}{}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b model.Pricing) int {
		return strings.Compare(a.ModelName, b.ModelName)
	})
	return out
}

func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func formatPricingToken(tok string) string {
	lower := strings.ToLower(tok)
	if mapped, ok := pricingWordDisplayMap[lower]; ok {
		return mapped
	}
	if tok == "" {
		return ""
	}
	runes := []rune(tok)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func pricingModelDisplayName(row model.Pricing) string {
	raw := strings.Split(row.ModelName, "-")
	tokens := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		part := strings.TrimSpace(raw[i])
		if part == "" {
			continue
		}
		if i+1 < len(raw) && pricingDigitsTokenPattern.MatchString(part) &&
			pricingDigitsTokenPattern.MatchString(strings.TrimSpace(raw[i+1])) {
			tokens = append(tokens, part+"."+strings.TrimSpace(raw[i+1]))
			i++
			continue
		}
		tokens = append(tokens, formatPricingToken(part))
	}
	if len(tokens) == 0 {
		return row.ModelName
	}
	return strings.Join(tokens, " ")
}

func pricingModelBrand(row model.Pricing) string {
	if b := strings.TrimSpace(row.BrandName); b != "" && !containsCJK(b) {
		return b
	}
	if row.Contract != nil {
		if b := strings.TrimSpace(row.Contract.BrandName); b != "" && !containsCJK(b) {
			return b
		}
	}
	return ""
}

func pricingModelModality(row model.Pricing) (label, appCategory string) {
	if row.Contract != nil {
		for _, op := range row.Contract.Operations {
			switch op.ID {
			case "text_to_video", "first_frame", "first_last_frame", "video_multi_ref", "video_edit", "video_extend", "document_to_video", "webpage_to_video":
				return "Video Generation", "MultimediaApplication"
			case "text_to_image", "image_to_image":
				return "Image Generation", "MultimediaApplication"
			case "text_to_music", "text_to_speech", "speech_to_text", "audio_generation":
				return "Audio & Speech AI", "MultimediaApplication"
			}
		}
		switch strings.ToLower(strings.TrimSpace(row.Contract.Family)) {
		case "video":
			return "Video Generation", "MultimediaApplication"
		case "image":
			return "Image Generation", "MultimediaApplication"
		case "audio":
			return "Audio & Speech AI", "MultimediaApplication"
		case "data":
			return "Data API", "DeveloperApplication"
		}
		if strings.TrimSpace(row.Contract.AudioKind) != "" {
			return "Audio & Speech AI", "MultimediaApplication"
		}
	}
	lowerTags := strings.ToLower(row.Tags)
	switch {
	case strings.Contains(lowerTags, "video generation"):
		return "Video Generation", "MultimediaApplication"
	case strings.Contains(lowerTags, "image generation"):
		return "Image Generation", "MultimediaApplication"
	case strings.Contains(lowerTags, "audio generation"):
		return "Audio & Speech AI", "MultimediaApplication"
	default:
		return "Text & Multimodal AI", "DeveloperApplication"
	}
}

func formatPricingOperationLabel(id string) string {
	if mapped, ok := pricingOperationDisplayMap[id]; ok {
		return mapped
	}
	parts := strings.Split(id, "_")
	for i := range parts {
		parts[i] = formatPricingToken(parts[i])
	}
	return strings.Join(parts, " ")
}

func pricingModelOperationLabels(c *modelcontracts.Contract) []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(c.Operations))
	for _, op := range c.Operations {
		if !op.Executable || op.ID == "" {
			continue
		}
		label := formatPricingOperationLabel(op.ID)
		if _, ok := seen[label]; !ok {
			seen[label] = struct{}{}
			out = append(out, label)
		}
	}
	if len(out) == 0 {
		for _, id := range c.OperationIDs {
			if id == "" {
				continue
			}
			label := formatPricingOperationLabel(id)
			if _, ok := seen[label]; !ok {
				seen[label] = struct{}{}
				out = append(out, label)
			}
		}
	}
	return out
}

func formatContractDuration(d *modelcontracts.DurationEnvelope) string {
	if d == nil {
		return ""
	}
	if len(d.Allowed) > 0 {
		vals := make([]string, 0, len(d.Allowed))
		for _, v := range d.Allowed {
			vals = append(vals, fmt.Sprintf("%gs", v))
		}
		return strings.Join(vals, ", ")
	}
	if d.Min != nil && d.Max != nil {
		return fmt.Sprintf("%gs to %gs", *d.Min, *d.Max)
	}
	if d.Max != nil {
		return fmt.Sprintf("up to %gs", *d.Max)
	}
	return ""
}

func pricingModelCapabilityItems(row model.Pricing) (listItems []string, featureList []string) {
	c := row.Contract
	if c == nil {
		return nil, nil
	}
	if ops := pricingModelOperationLabels(c); len(ops) > 0 {
		listItems = append(listItems, "Supported operations: "+strings.Join(ops, ", "))
		featureList = append(featureList, ops...)
	}
	if len(c.Resolutions) > 0 {
		res := strings.Join(c.Resolutions, ", ")
		listItems = append(listItems, "Resolutions: "+res)
		featureList = append(featureList, "Resolutions: "+res)
	}
	if dur := formatContractDuration(c.DurationsSeconds); dur != "" {
		listItems = append(listItems, "Duration range: "+dur)
		featureList = append(featureList, "Duration: "+dur)
	}
	if len(c.AspectRatios) > 0 {
		listItems = append(listItems, "Aspect ratios: "+strings.Join(c.AspectRatios, ", "))
	}
	if c.AudioSupported != nil && *c.AudioSupported {
		listItems = append(listItems, "Native audio: Supported")
		featureList = append(featureList, "Native synchronized audio")
	}
	if len(c.OutputSizes) > 0 {
		sizes := strings.Join(c.OutputSizes, ", ")
		listItems = append(listItems, "Output sizes: "+sizes)
		featureList = append(featureList, "Output sizes: "+sizes)
	}
	if len(c.OutputFormats) > 0 {
		upper := make([]string, 0, len(c.OutputFormats))
		for _, f := range c.OutputFormats {
			if strings.TrimSpace(f) != "" {
				upper = append(upper, strings.ToUpper(strings.TrimSpace(f)))
			}
		}
		if len(upper) > 0 {
			listItems = append(listItems, "Output formats: "+strings.Join(upper, ", "))
		}
	}
	if c.ContextWindowTokens != nil && *c.ContextWindowTokens > 0 {
		ctx := formatCommaInt(*c.ContextWindowTokens) + " tokens"
		listItems = append(listItems, "Context window: "+ctx)
		featureList = append(featureList, ctx+" context")
	}
	if c.MaxOutputTokens != nil && *c.MaxOutputTokens > 0 {
		listItems = append(listItems, "Max output tokens: "+formatCommaInt(*c.MaxOutputTokens))
	}
	if c.ToolCalling != nil && *c.ToolCalling {
		listItems = append(listItems, "Tool calling: Supported")
		featureList = append(featureList, "Tool calling")
	}
	if c.Streaming != nil && *c.Streaming {
		listItems = append(listItems, "Streaming responses: Supported")
	}
	if c.ReferenceImages != nil && c.ReferenceImages.Max != nil && *c.ReferenceImages.Max > 0 {
		listItems = append(listItems, fmt.Sprintf("Reference images: up to %d", *c.ReferenceImages.Max))
	}
	if c.ReferenceVideos != nil && c.ReferenceVideos.Max != nil && *c.ReferenceVideos.Max > 0 {
		listItems = append(listItems, fmt.Sprintf("Reference videos: up to %d", *c.ReferenceVideos.Max))
	}
	if c.ReferenceAudios != nil && c.ReferenceAudios.Max != nil && *c.ReferenceAudios.Max > 0 {
		listItems = append(listItems, fmt.Sprintf("Reference audio clips: up to %d", *c.ReferenceAudios.Max))
	}
	return listItems, featureList
}

func pricingModelBillingUnitPhrase(row model.Pricing) string {
	if row.Contract == nil || row.Contract.Billing == nil {
		return ""
	}
	unit := strings.TrimSpace(row.Contract.Billing.Unit)
	if unit == "" || containsCJK(unit) {
		return ""
	}
	switch unit {
	case "per_second":
		return "per second"
	case "per_image":
		return "per image"
	case "per_video":
		return "per video"
	case "per_token":
		return "per 1M tokens"
	case "per_minute":
		return "per minute"
	case "per_request":
		return "per request"
	case "call":
		return "per call"
	default:
		return strings.ReplaceAll(unit, "_", " ")
	}
}

func pricingModelBillingSummary(row model.Pricing) string {
	if unit := pricingModelBillingUnitPhrase(row); unit != "" {
		return fmt.Sprintf(
			"Billed %s using pay-per-generation credits on OmniMux (no subscription required). Live tier, resolution, and group rates are displayed on this page.",
			unit,
		)
	}
	return "Pay-per-generation credits on OmniMux with no subscription required. Check live generation rates and active parameter tiers on this page."
}

func pricingModelMetaTitle(row model.Pricing) string {
	name := pricingModelDisplayName(row)
	for _, candidate := range []string{
		name + " API Pricing & Specs | OmniMux",
		name + " Pricing & Specs | OmniMux",
		name + " Pricing | OmniMux",
	} {
		if len(candidate) <= 60 {
			return candidate
		}
	}
	return truncateUTF8Bytes(name+" Pricing | OmniMux", 60)
}

func pricingModelMetaDescription(row model.Pricing) string {
	name := pricingModelDisplayName(row)
	brand := pricingModelBrand(row)
	modality, _ := pricingModelModality(row)
	prefix := name
	if brand != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(brand)) {
		prefix = name + " by " + brand
	}

	var specParts []string
	if row.Contract != nil {
		if len(row.Contract.Resolutions) > 0 {
			specParts = append(specParts, strings.Join(row.Contract.Resolutions, "/"))
		} else if len(row.Contract.OutputSizes) > 0 {
			specParts = append(specParts, strings.Join(row.Contract.OutputSizes[:min(3, len(row.Contract.OutputSizes))], "/"))
		}
		if dur := formatContractDuration(row.Contract.DurationsSeconds); dur != "" {
			specParts = append(specParts, dur)
		} else if row.Contract.ContextWindowTokens != nil && *row.Contract.ContextWindowTokens > 0 {
			specParts = append(specParts, formatCommaInt(*row.Contract.ContextWindowTokens)+" context")
		}
	}

	if len(specParts) > 0 {
		candidate := fmt.Sprintf(
			"%s %s API pricing and specs on OmniMux (%s). Pay-per-generation credits, no subscription required.",
			prefix,
			modality,
			strings.Join(specParts, ", "),
		)
		if len([]rune(candidate)) <= 160 {
			return candidate
		}
	}
	fallback := fmt.Sprintf(
		"%s %s API pricing and specs on OmniMux. Pay-per-generation credits, no subscription required.",
		prefix,
		modality,
	)
	return truncateRunes(fallback, 160)
}

func pricingModelHead(siteURL string, row model.Pricing) string {
	esc := htmlEscaper.Replace
	rawTitle := pricingModelMetaTitle(row)
	rawDesc := pricingModelMetaDescription(row)
	displayName := pricingModelDisplayName(row)
	title := esc(rawTitle)
	desc := esc(rawDesc)
	routePath := pricingModelsPrefix + url.PathEscape(row.ModelName)
	canonical := siteURL + routePath
	ogImage := siteURL + "/og.png"
	_, appCategory := pricingModelModality(row)
	_, featureList := pricingModelCapabilityItems(row)

	appGraph := map[string]any{
		"@type":               "SoftwareApplication",
		"@id":                 canonical + "#software",
		"name":                displayName,
		"description":         rawDesc,
		"url":                 canonical,
		"applicationCategory": appCategory,
		"operatingSystem":     "Web, API",
	}
	if brand := pricingModelBrand(row); brand != "" {
		appGraph["brand"] = map[string]any{
			"@type": "Brand",
			"name":  brand,
		}
	}
	if len(featureList) > 0 {
		appGraph["featureList"] = featureList
	}

	graph := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":       "WebPage",
				"@id":         canonical + "#webpage",
				"url":         canonical,
				"name":        rawTitle,
				"description": rawDesc,
				"isPartOf": map[string]any{
					"@type": "WebSite",
					"name":  "OmniMux",
					"url":   siteURL + "/",
				},
				"breadcrumb": map[string]any{"@id": canonical + "#breadcrumb"},
			},
			map[string]any{
				"@type": "BreadcrumbList",
				"@id":   canonical + "#breadcrumb",
				"itemListElement": []any{
					map[string]any{"@type": "ListItem", "position": 1, "name": "OmniMux", "item": siteURL + "/"},
					map[string]any{"@type": "ListItem", "position": 2, "name": "Model Pricing", "item": siteURL + "/pricing"},
					map[string]any{"@type": "ListItem", "position": 3, "name": displayName, "item": canonical},
				},
			},
			appGraph,
		},
	}

	return strings.Join([]string{
		`<title>` + title + `</title>`,
		`<meta name="title" content="` + title + `" />`,
		`<meta name="omnimux-seo-title" content="` + title + `" />`,
		`<meta name="omnimux-server-head-path" content="` + esc(routePath) + `" />`,
		`<meta name="description" content="` + desc + `" />`,
		`<meta name="robots" content="index,follow" />`,
		`<link rel="canonical" href="` + esc(canonical) + `" />`,
		`<meta property="og:type" content="website" />`,
		`<meta property="og:url" content="` + esc(canonical) + `" />`,
		`<meta property="og:title" content="` + title + `" />`,
		`<meta property="og:description" content="` + desc + `" />`,
		`<meta property="og:site_name" content="OmniMux" />`,
		`<meta property="og:image" content="` + esc(ogImage) + `" />`,
		`<meta property="og:locale" content="en_US" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`<meta name="twitter:title" content="` + title + `" />`,
		`<meta name="twitter:description" content="` + desc + `" />`,
		`<meta name="twitter:image" content="` + esc(ogImage) + `" />`,
		`<script type="application/ld+json">` + marshalStructuredDataJSON(graph) + `</script>`,
	}, "\n")
}

func pricingModelSummary(siteURL string, row model.Pricing) string {
	esc := htmlEscaper.Replace
	displayName := pricingModelDisplayName(row)
	brand := pricingModelBrand(row)
	modality, _ := pricingModelModality(row)
	desc := pricingModelMetaDescription(row)
	billingCopy := pricingModelBillingSummary(row)

	var overviewParts []string
	overviewParts = append(overviewParts, fmt.Sprintf(`<p>%s</p>`, esc(desc)))
	if englishDesc := strings.TrimSpace(row.Description); englishDesc != "" && !containsCJK(englishDesc) && englishDesc != desc {
		overviewParts = append(overviewParts, fmt.Sprintf(`<p>%s</p>`, esc(englishDesc)))
	}
	metaLine := "Modality: " + modality
	if brand != "" {
		metaLine = fmt.Sprintf("Provider: %s | Modality: %s", brand, modality)
	}
	overviewParts = append(overviewParts, fmt.Sprintf(`<p>%s</p>`, esc(metaLine)))
	overviewParts = append(overviewParts, fmt.Sprintf(`<p>%s</p>`, esc(billingCopy)))

	capItems, _ := pricingModelCapabilityItems(row)
	capBlock := ""
	if len(capItems) > 0 {
		var b strings.Builder
		b.WriteString("<h2>Verified Capabilities & Specifications</h2><ul>")
		for _, item := range capItems {
			b.WriteString("<li>")
			b.WriteString(esc(item))
			b.WriteString("</li>")
		}
		b.WriteString("</ul>")
		capBlock = b.String()
	}

	var links []string
	for _, zone := range pricingToInspirationZoneMap[row.ModelName] {
		links = append(links, fmt.Sprintf(`<a href="%s%s%s">%s</a>`, esc(siteURL), inspirationModelsPrefix, esc(zone.Slug), esc(zone.Label)))
	}
	for _, guide := range pricingToBlogGuidesMap[row.ModelName] {
		links = append(links, fmt.Sprintf(`<a href="%s%s">%s</a>`, esc(siteURL), esc(guide.URL), esc(guide.Label)))
	}
	links = append(links,
		fmt.Sprintf(`<a href="%s/pricing">All Model Pricing</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration">Prompt Inspiration</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/ai-influencer-studio">AI Influencer Studio</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/cli">OmniMux CLI</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/blog">OmniMux Blog</a>`, esc(siteURL)),
	)

	nav := strings.Join([]string{
		fmt.Sprintf(`<a href="%s/">Home</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/pricing">Pricing</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration">Inspiration</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/blog">Blog</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/guides">Guides</a>`, esc(siteURL)),
	}, " | ")

	return fmt.Sprintf(
		`<noscript><div id="geo-ssr"><header><p>OmniMux</p><nav>%s</nav></header><main><h1>%s</h1>%s%s<h2>Related Prompt Libraries & Guides</h2><p>%s</p></main></div></noscript>`,
		nav,
		esc(displayName+" Pricing & Specifications"),
		strings.Join(overviewParts, ""),
		capBlock,
		strings.Join(links, " | "),
	)
}

func (pages webSearchPages) servePricingModelPrivateOK(c *gin.Context) {
	middleware.SetHTMLCacheHeaders(c, false)
	c.Header("X-Robots-Tag", "noindex, follow")
	c.Data(http.StatusOK, "text/html; charset=utf-8", pages.private)
}

func (pages webSearchPages) servePricingModelPage(c *gin.Context) {
	rawSegment, ok := strings.CutPrefix(c.Request.URL.EscapedPath(), pricingModelsPrefix)
	if !ok || rawSegment == "" || strings.Contains(rawSegment, "/") {
		pages.servePricingModelPrivateOK(c)
		return
	}
	modelName, err := url.PathUnescape(rawSegment)
	if err != nil || modelName == "" {
		pages.servePricingModelPrivateOK(c)
		return
	}
	siteURL := searchPageCanonicalHost(pages.public["/"])
	if siteURL == "" {
		pages.servePricingModelPrivateOK(c)
		return
	}
	var matched model.Pricing
	found := false
	for _, row := range indexablePricingModels() {
		if row.ModelName == modelName {
			matched = row
			found = true
			break
		}
	}
	if !found {
		pages.servePricingModelPrivateOK(c)
		return
	}
	page, err := renderWebSearchPage(string(pages.private), webSearchPage{
		Head:    pricingModelHead(siteURL, matched),
		Summary: pricingModelSummary(siteURL, matched),
	})
	if err != nil {
		pages.servePricingModelPrivateOK(c)
		return
	}
	if isAPISubdomainHost(c.Request.Host, siteURL) {
		middleware.SetHTMLCacheHeaders(c, false)
		c.Header("X-Robots-Tag", "noindex, follow")
	} else {
		middleware.SetHTMLCacheHeaders(c, true)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", page)
}
