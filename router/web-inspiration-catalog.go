package router

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/gin-gonic/gin"
)

const (
	inspirationCatalogTTL          = 15 * time.Minute
	inspirationCatalogErrorBackoff = time.Minute
	inspirationCatalogMaxBytes     = 2 << 20
	inspirationCatalogHTTPTimeout  = 5 * time.Second
	inspirationModelsPrefix        = "/inspiration/models/"
)

var (
	errInspirationCatalogUnavailable = errors.New("inspiration catalog manifest unavailable")
	inspirationZoneSlugPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	inspirationDatePrefixPattern     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	inspirationSitemapLocPattern     = regexp.MustCompile(`<loc>([^<]+)</loc>`)
	inspirationSitemapLastmodPattern = regexp.MustCompile(`<lastmod>([^<]+)</lastmod>`)
	// Set to false by router test init() so tests never issue live network calls unless injected.
	inspirationCatalogAllowDefaultSiteFetch = true
	// inspirationCatalogFetch overrides manifest loading in tests.
	inspirationCatalogFetch func(siteURL string) ([]byte, error)
	// inspirationPricingModelExists checks whether a /pricing/<id> target is live on the gateway.
	inspirationPricingModelExists = func(modelID string) bool {
		if model.DB == nil {
			return true
		}
		return webPricingModelExists(modelID)
	}
	inspirationJSONReplacer = strings.NewReplacer(
		"<", `\u003c`,
		">", `\u003e`,
		"&", `\u0026`,
		"\u2028", `\u2028`,
		"\u2029", `\u2029`,
	)
)

type inspirationCatalogZone struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	MediaType   string   `json:"mediaType"`
	Count       int      `json:"count"`
	Quarantined int      `json:"quarantined"`
	Categories  []string `json:"categories"`
}

type inspirationCatalogManifest struct {
	Version     int                      `json:"version"`
	GeneratedAt string                   `json:"generatedAt"`
	Total       int                      `json:"total"`
	Zones       []inspirationCatalogZone `json:"zones"`
}

type inspirationCatalogSnapshot struct {
	zones   []inspirationCatalogZone
	byID    map[string]inspirationCatalogZone
	lastmod string
}

type inspirationCatalogStore struct {
	mu        sync.Mutex
	snapshot  *inspirationCatalogSnapshot
	expiresAt time.Time
	now       func() time.Time
	fetch     func(siteURL string) ([]byte, error)
}

func newInspirationCatalogStore() *inspirationCatalogStore {
	return &inspirationCatalogStore{
		now: time.Now,
	}
}

func (s *inspirationCatalogStore) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *inspirationCatalogStore) fetchManifest(siteURL string) ([]byte, error) {
	if s.fetch != nil {
		return s.fetch(siteURL)
	}
	return fetchInspirationCatalogManifest(siteURL)
}

func (s *inspirationCatalogStore) get(siteURL string) *inspirationCatalogSnapshot {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.currentTime()
	if !s.expiresAt.IsZero() && now.Before(s.expiresAt) {
		return s.snapshot
	}

	raw, err := s.fetchManifest(siteURL)
	if err == nil {
		if parsed, parseErr := parseInspirationCatalogManifest(raw); parseErr == nil {
			s.snapshot = parsed
			s.expiresAt = now.Add(inspirationCatalogTTL)
			return s.snapshot
		}
	}
	s.expiresAt = now.Add(inspirationCatalogErrorBackoff)
	return s.snapshot
}

func fetchInspirationCatalogManifest(siteURL string) ([]byte, error) {
	if inspirationCatalogFetch != nil {
		return inspirationCatalogFetch(siteURL)
	}
	if filePath := strings.TrimSpace(os.Getenv("INSPIRATION_CATALOG_MANIFEST_PATH")); filePath != "" {
		return readBoundedManifestFile(filePath)
	}
	rawURL := strings.TrimSpace(os.Getenv("INSPIRATION_CATALOG_MANIFEST_URL"))
	if rawURL == "" && inspirationCatalogAllowDefaultSiteFetch && strings.TrimSpace(siteURL) != "" {
		rawURL = strings.TrimRight(strings.TrimSpace(siteURL), "/") + "/inspiration-catalog/manifest.json"
	}
	if rawURL == "" {
		return nil, errInspirationCatalogUnavailable
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid inspiration catalog manifest URL")
	}
	client := &http.Client{Timeout: inspirationCatalogHTTPTimeout}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected catalog manifest status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, inspirationCatalogMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > inspirationCatalogMaxBytes {
		return nil, fmt.Errorf("inspiration catalog manifest exceeds size limit")
	}
	return data, nil
}

func readBoundedManifestFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, inspirationCatalogMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > inspirationCatalogMaxBytes {
		return nil, fmt.Errorf("inspiration catalog manifest exceeds size limit")
	}
	return data, nil
}

func parseInspirationCatalogManifest(raw []byte) (*inspirationCatalogSnapshot, error) {
	var manifest inspirationCatalogManifest
	if err := common.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	snapshot := &inspirationCatalogSnapshot{
		byID: make(map[string]inspirationCatalogZone, len(manifest.Zones)),
	}
	if len(manifest.GeneratedAt) >= 10 {
		prefix := manifest.GeneratedAt[:10]
		if inspirationDatePrefixPattern.MatchString(prefix) {
			snapshot.lastmod = prefix
		}
	}
	for _, zone := range manifest.Zones {
		id := strings.TrimSpace(zone.ID)
		if !inspirationZoneSlugPattern.MatchString(id) || zone.Count <= 0 {
			continue
		}
		if _, exists := snapshot.byID[id]; exists {
			continue
		}
		zone.ID = id
		zone.Label = strings.TrimSpace(zone.Label)
		if zone.Label == "" {
			zone.Label = id
		}
		snapshot.zones = append(snapshot.zones, zone)
		snapshot.byID[id] = zone
	}
	if len(snapshot.zones) == 0 {
		return nil, fmt.Errorf("inspiration catalog manifest has no indexable zones")
	}
	return snapshot, nil
}

func formatCommaInt(n int) string {
	if n < 0 {
		return "-" + formatCommaInt(-n)
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		cut = i
	}
	return strings.TrimSpace(s[:cut])
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	count := 0
	for i := range s {
		if count == maxRunes {
			return strings.TrimSpace(s[:i])
		}
		count++
	}
	return s
}

func inspirationModelMetaTitle(zone inspirationCatalogZone) string {
	withCount := fmt.Sprintf("%s Prompts (%s+) | OmniMux", zone.Label, formatCommaInt(zone.Count))
	if len(withCount) <= 60 {
		return withCount
	}
	withoutCount := fmt.Sprintf("%s Prompts | OmniMux", zone.Label)
	if len(withoutCount) <= 60 {
		return withoutCount
	}
	return truncateUTF8Bytes(withoutCount, 60)
}

var inspirationCategoryDisplayEN = map[string]string{
	"30s-stories-one-take":             "30s one-take story",
	"30s-one-shot":                     "30s one-shot",
	"2d-animation":                     "2D animation",
	"3d":                               "3D",
	"action":                           "Action",
	"action-anime":                     "Action anime",
	"ad-creative":                      "Ad creative",
	"advanced-camera-movements":        "Advanced camera movements",
	"animal":                           "Animal",
	"anime-animation":                  "Anime animation",
	"anime-manga":                      "Anime & manga",
	"app-web-design":                   "App & web design",
	"audio-voice-synthesis":            "Voice synthesis",
	"brand-commercial":                 "Brand commercial",
	"brand-film":                       "Brand film",
	"camera-motion":                    "Camera motion",
	"channel-intro":                    "Channel intro",
	"character-design":                 "Character design",
	"character-performance":            "Character performance",
	"character-product-consistency":    "Subject consistency",
	"character-scene-consistency":      "Character & scene consistency",
	"chibi-q-style":                    "Chibi Q style",
	"chinese-ink":                      "Chinese ink",
	"cinematic":                        "Cinematic",
	"cinematic-film":                   "Cinematic film",
	"cinematic-film-still":             "Cinematic film still",
	"cinematic-general":                "Cinematic general",
	"cinematic-realistic":              "Cinematic realistic",
	"cinematic-scenes":                 "Cinematic scenes",
	"cinematic-showcase":               "Cinematic showcase",
	"cinematic-story":                  "Cinematic story",
	"comedy-humor":                     "Comedy & humor",
	"comic-graphic-novel":              "Comic graphic novel",
	"comic-storyboard":                 "Comic storyboard",
	"commercial-product":               "Commercial product",
	"community-showcase":               "Community showcase",
	"comparison":                       "Comparison",
	"creative":                         "Creative concept",
	"creative-visual-effects":          "Creative visual effects",
	"cyberpunk-sci-fi":                 "Cyberpunk sci-fi",
	"diagram-chart":                    "Diagram & chart",
	"dialogue-native-audio":            "Dialogue with native audio",
	"documentary":                      "Documentary",
	"ecommerce":                        "E-commerce",
	"ecommerce-main-image":             "E-commerce main image",
	"emotional-performance":            "Emotional performance",
	"epic-disaster":                    "Epic disaster",
	"explainer-tutorial":               "Explainer tutorial",
	"fantasy":                          "Fantasy",
	"fantasy-mythology":                "Fantasy & mythology",
	"fantasy-scenes":                   "Fantasy scenes",
	"fashion":                          "Fashion",
	"fashion-dance":                    "Fashion & dance",
	"first-frame-control":              "First-frame control",
	"food-beverage":                    "Food & beverage",
	"game-asset":                       "Game asset",
	"game-cinematic":                   "Game cinematic",
	"historical-period":                "Historical period",
	"horror-dark":                      "Horror & dark",
	"illustration":                     "Illustration",
	"infographic-edu-visual":           "Infographic & education",
	"ink-chinese-style":                "Chinese ink style",
	"ip-character":                     "IP character",
	"isometric":                        "Isometric",
	"marketing-visuals":                "Marketing visuals",
	"motion-graphics":                  "Motion graphics",
	"multilingual":                     "Multilingual",
	"music-audio":                      "Music & audio",
	"music-dance":                      "Music & dance",
	"music-synchronization":            "Music synchronization",
	"music-video":                      "Music video",
	"native-audio-dialogue":            "Native dialogue & audio",
	"nature-animal":                    "Nature & animal",
	"official-seedance-2-5-showcase":   "Official showcase",
	"oil-painting":                     "Oil painting",
	"one-take-cinematography":          "One-take cinematography",
	"photography":                      "Photography",
	"pixel-art":                        "Pixel art",
	"portrait":                         "Portrait",
	"portraits":                        "Portraits",
	"poster":                           "Poster",
	"poster-flyer":                     "Poster & flyer",
	"posters":                          "Posters",
	"pov-fpv":                          "POV & FPV",
	"product-ad":                       "Product ad",
	"product-ads":                      "Product ads",
	"product-ads-brand":                "Brand product ads",
	"product-marketing":                "Product marketing",
	"product-photography":              "Product photography",
	"profile-avatar":                   "Profile avatar",
	"romance-drama":                    "Romance drama",
	"sci-fi-cyberpunk":                 "Sci-fi cyberpunk",
	"sci-fi-worldbuilding":             "Sci-fi worldbuilding",
	"short-film":                       "Short film",
	"sketch-line-art":                  "Sketch & line art",
	"social-lifestyle":                 "Social lifestyle",
	"social-media-post":                "Social media post",
	"sports-fitness":                   "Sports & fitness",
	"stop-motion":                      "Stop motion",
	"story-development-extension":      "Story development",
	"storytelling":                     "Storytelling",
	"stylized-scenes":                  "Stylized scenes",
	"text-typography":                  "Text & typography",
	"transformation-vfx":               "Transformation VFX",
	"tutorial-prompt":                  "Tutorial & prompt",
	"twitter-community":                "Community showcase",
	"ugc-talking-head":                 "UGC talking head",
	"ugc-vlog":                         "UGC vlog",
	"ui":                               "UI design",
	"ui-mockups":                       "UI mockups",
	"ultra-realistic-video-generation": "Ultra-realistic video",
	"vehicle":                          "Vehicle",
	"vehicle-transport":                "Vehicle & transport",
	"vfx":                              "VFX",
	"vfx-video-editing":                "VFX & video editing",
	"video-editing":                    "Video editing",
	"video-editing-remixing":           "Video remixing",
	"watercolor":                       "Watercolor",
	"wuxia-martial-arts":               "Wuxia martial arts",
	"youtube-thumbnail":                "YouTube thumbnail",
	"abstract-background":              "Abstract background",
	"animation":                        "Animation",
	// Folded Chinese facet labels mapped to canonical English display names.
	"电影级场景展示":  "Cinematic showcase",
	"电影级展示":    "Cinematic showcase",
	"vlog生活记录": "Social lifestyle",
	"短片":       "Short film",
	"品牌产品广告":   "Brand commercial",
	"品牌商业广告":   "Brand commercial",
	"社区精选":     "Community showcase",
	"音乐视频":     "Music video",
	"社交媒体帖子":   "Social media post",
	"社媒图文":     "Social media post",
	"产品营销":     "Product marketing",
	"个人资料头像":   "Profile avatar",
	"个人头像":     "Profile avatar",
	"漫画故事板":    "Comic storyboard",
	"漫画分镜":     "Comic storyboard",
	"海报传单":     "Poster & flyer",
	"信息图教育视觉图": "Infographic & education",
	"信息图与教学":   "Infographic & education",
	"游戏素材":     "Game asset",
	"游戏资产":     "Game asset",
	"动效":       "Motion graphics",
	"讲解":       "Explainer",
	"交互":       "Interactive",
	"动画":       "Animation",
	"动作":       "Action",
	"动作动漫":     "Action anime",
	"动漫动画":     "Anime animation",
	"运镜":       "Camera motion",
	"角色设计":     "Character design",
	"角色表演":     "Character performance",
	"电商主图":     "E-commerce main image",
	"产品广告":     "Product ads",
	"首帧控制":     "First-frame control",
	"人像":       "Portraits",
	"人像写真":     "Portrait",
	"奇幻":       "Fantasy",
	"时尚":       "Fashion",
}

var inspirationAcronymMap = map[string]string{
	"ai": "AI", "3d": "3D", "2d": "2D", "ugc": "UGC", "vfx": "VFX",
	"ui": "UI", "pov": "POV", "fpv": "FPV", "ip": "IP", "pdp": "PDP",
}

func foldInspirationCategoryKey(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsSpace(r) || r == '/' || r == '／' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func containsInspirationCJK(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func normalizeInspirationCategoryEN(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	lower := strings.ToLower(trimmed)
	if lower == "uncategorized" || trimmed == "未分类" {
		return "", false
	}
	if en, ok := inspirationCategoryDisplayEN[lower]; ok {
		return en, true
	}
	if en, ok := inspirationCategoryDisplayEN[foldInspirationCategoryKey(trimmed)]; ok {
		return en, true
	}
	if containsInspirationCJK(trimmed) {
		return "", false
	}
	isSlug := strings.ContainsAny(trimmed, "-_") && !strings.ContainsRune(trimmed, ' ')
	var parts []string
	if isSlug {
		parts = strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == '-' || r == '_' || unicode.IsSpace(r)
		})
	} else {
		parts = strings.Fields(trimmed)
	}
	if len(parts) == 0 {
		return "", false
	}
	for i, part := range parts {
		pl := strings.ToLower(part)
		if acr, ok := inspirationAcronymMap[pl]; ok {
			parts[i] = acr
			continue
		}
		if !isSlug {
			if i == 0 {
				runes := []rune(part)
				runes[0] = unicode.ToUpper(runes[0])
				parts[i] = string(runes)
			}
			continue
		}
		if i == 0 {
			runes := []rune(pl)
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
			continue
		}
		parts[i] = pl
	}
	return strings.Join(parts, " "), true
}

func cleanInspirationCategories(raw []string, limit int) []string {
	if limit <= 0 || len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, min(limit, len(raw)))
	out := make([]string, 0, min(limit, len(raw)))
	for _, item := range raw {
		en, ok := normalizeInspirationCategoryEN(item)
		if !ok {
			continue
		}
		key := strings.ToLower(en)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, en)
		if len(out) >= limit {
			break
		}
	}
	return out
}

type inspirationPricingTarget struct {
	ContractID string
	PricingID  string
	Label      string
}

func inspirationZonePricingTarget(zone inspirationCatalogZone) inspirationPricingTarget {
	switch strings.ToLower(strings.TrimSpace(zone.ID)) {
	case "seedance-2.5":
		return inspirationPricingTarget{ContractID: "seedance-2-5", PricingID: "seedance-2-5", Label: "Seedance 2.5"}
	case "seedance-2.0":
		return inspirationPricingTarget{ContractID: "seedance-2-0", PricingID: "seedance-2-0", Label: "Seedance 2.0"}
	case "wan-3.0":
		return inspirationPricingTarget{ContractID: "wan-3.0", PricingID: "wan-3.0", Label: "Wan 3.0"}
	case "gpt-image-2":
		return inspirationPricingTarget{ContractID: "", PricingID: "gpt-image-2.5", Label: "GPT Image 2.5"}
	case "gpt-image-2.5":
		return inspirationPricingTarget{ContractID: "gpt-image-2.5", PricingID: "gpt-image-2.5", Label: "GPT Image 2.5"}
	case "nano-banana-pro":
		return inspirationPricingTarget{ContractID: "nano-banana-pro", PricingID: "nano-banana-pro", Label: "Nano Banana Pro"}
	default:
		return inspirationPricingTarget{}
	}
}

type inspirationGuideLink struct {
	URL   string
	Label string
}

func inspirationZoneBlogGuides(zone inspirationCatalogZone) []inspirationGuideLink {
	switch strings.ToLower(strings.TrimSpace(zone.ID)) {
	case "seedance-2.5", "seedance-2.0":
		return []inspirationGuideLink{
			{URL: "/blog/seedance-2-5-prompting-timestamps-and-camera-guide", Label: "Seedance 2.5 Prompting, Timestamps & Camera Guide"},
			{URL: "/blog/tiktok-shop-product-video-9-shot-framework", Label: "TikTok Shop 9-Shot Product Video Framework"},
		}
	case "wan-3.0", "grok-imagine", "claude-opus-5.5":
		return []inspirationGuideLink{
			{URL: "/blog/tiktok-shop-product-video-9-shot-framework", Label: "TikTok Shop 9-Shot Product Video Framework"},
			{URL: "/blog/ai-video-upscaler-and-pre-publish-enhancement", Label: "AI Video Enhancement & Pre-Publish Pipeline"},
		}
	case "gpt-image-2", "gpt-image-2.5", "nano-banana-pro":
		return []inspirationGuideLink{
			{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"},
			{URL: "/blog/seedream-5-5-vs-nano-banana-2-1-vs-gpt-image-2-5", Label: "Seedream 5.5 vs Nano Banana 2.1 vs GPT Image 2.5"},
		}
	default:
		if strings.EqualFold(strings.TrimSpace(zone.MediaType), "image") {
			return []inspirationGuideLink{{URL: "/blog/ecommerce-product-image-set-pdp-and-carousels", Label: "Ecommerce Product Image Sets & Carousels"}}
		}
		return []inspirationGuideLink{{URL: "/blog/tiktok-shop-product-video-9-shot-framework", Label: "TikTok Shop 9-Shot Product Video Framework"}}
	}
}

func inspirationModelCapabilityList(c modelcontracts.Contract) []string {
	var items []string
	if strings.TrimSpace(c.BrandName) != "" {
		items = append(items, "Provider: "+strings.TrimSpace(c.BrandName))
	}
	if len(c.OperationIDs) > 0 {
		ops := make([]string, 0, len(c.OperationIDs))
		for _, id := range c.OperationIDs {
			ops = append(ops, strings.ReplaceAll(id, "_", " "))
		}
		items = append(items, "Operations: "+strings.Join(ops, ", "))
	}
	if len(c.Resolutions) > 0 {
		items = append(items, "Resolutions: "+strings.Join(c.Resolutions, ", "))
	}
	if c.DurationsSeconds != nil && c.DurationsSeconds.Min != nil && c.DurationsSeconds.Max != nil {
		items = append(items, fmt.Sprintf("Duration range: %gs to %gs", *c.DurationsSeconds.Min, *c.DurationsSeconds.Max))
	}
	if len(c.AspectRatios) > 0 {
		items = append(items, "Aspect ratios: "+strings.Join(c.AspectRatios, ", "))
	}
	if c.AudioSupported != nil && *c.AudioSupported {
		items = append(items, "Native audio: Supported")
	}
	if len(c.OutputSizes) > 0 {
		items = append(items, "Output sizes: "+strings.Join(c.OutputSizes, ", "))
	}
	if len(c.OutputFormats) > 0 {
		fmts := make([]string, 0, len(c.OutputFormats))
		for _, f := range c.OutputFormats {
			if strings.TrimSpace(f) != "" {
				fmts = append(fmts, strings.ToUpper(strings.TrimSpace(f)))
			}
		}
		if len(fmts) > 0 {
			items = append(items, "Output formats: "+strings.Join(fmts, ", "))
		}
	}
	return items
}

func inspirationModelMetaDescription(zone inspirationCatalogZone) string {
	mediaNoun := "AI"
	switch strings.ToLower(strings.TrimSpace(zone.MediaType)) {
	case "video":
		mediaNoun = "video"
	case "image":
		mediaNoun = "image"
	}
	cats := cleanInspirationCategories(zone.Categories, 3)
	if len(cats) > 0 {
		withCats := fmt.Sprintf(
			"Browse %s+ %s %s prompts covering %s on OmniMux. Copy any prompt or recreate it in the studio.",
			formatCommaInt(zone.Count),
			zone.Label,
			mediaNoun,
			strings.Join(cats, ", "),
		)
		if len([]rune(withCats)) <= 160 {
			return withCats
		}
	}
	desc := fmt.Sprintf(
		"Browse %s+ %s %s prompts with examples on OmniMux. Copy any prompt or recreate it with your own AI character in the studio.",
		formatCommaInt(zone.Count),
		zone.Label,
		mediaNoun,
	)
	return truncateRunes(desc, 160)
}

func marshalStructuredDataJSON(v any) string {
	raw, err := common.Marshal(v)
	if err != nil {
		return "{}"
	}
	return inspirationJSONReplacer.Replace(string(raw))
}

func inspirationModelHead(siteURL string, zone inspirationCatalogZone) string {
	esc := htmlEscaper.Replace
	rawTitle := inspirationModelMetaTitle(zone)
	rawDesc := inspirationModelMetaDescription(zone)
	title := esc(rawTitle)
	desc := esc(rawDesc)
	routePath := inspirationModelsPrefix + zone.ID
	canonical := siteURL + routePath
	ogImage := siteURL + "/og.png"

	graph := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":       "CollectionPage",
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
					map[string]any{"@type": "ListItem", "position": 2, "name": "Prompt inspiration", "item": siteURL + "/inspiration"},
					map[string]any{"@type": "ListItem", "position": 3, "name": zone.Label, "item": canonical},
				},
			},
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

func inspirationModelSummary(siteURL string, zone inspirationCatalogZone) string {
	esc := htmlEscaper.Replace
	cleanedCats := cleanInspirationCategories(zone.Categories, 12)
	var cats strings.Builder
	for _, cat := range cleanedCats {
		cats.WriteString("<li>")
		cats.WriteString(esc(cat))
		cats.WriteString("</li>")
	}
	categoryBlock := ""
	if cats.Len() > 0 {
		categoryBlock = "<h2>Prompt Categories</h2><ul>" + cats.String() + "</ul>"
	}

	capabilityBlock := ""
	target := inspirationZonePricingTarget(zone)
	hasPricingPage := target.PricingID != "" && inspirationPricingModelExists(target.PricingID)
	if target.ContractID != "" {
		if contract, ok := modelcontracts.For(target.ContractID); ok && contract.IsLiveExecutable() {
			if capItems := inspirationModelCapabilityList(contract); len(capItems) > 0 {
				var capHTML strings.Builder
				capHTML.WriteString("<h2>Model Capabilities on OmniMux</h2><ul>")
				for _, item := range capItems {
					capHTML.WriteString("<li>")
					capHTML.WriteString(esc(item))
					capHTML.WriteString("</li>")
				}
				capHTML.WriteString("</ul>")
				if hasPricingPage {
					capHTML.WriteString(fmt.Sprintf(
						`<p><a href="%s/pricing/%s">View %s API pricing and specifications</a></p>`,
						esc(siteURL),
						esc(target.PricingID),
						esc(target.Label),
					))
				}
				capabilityBlock = capHTML.String()
			}
		}
	}

	checklist := []string{
		"Define subject, product selling point, and scene setting in concrete visual terms.",
		"Specify camera movement, pacing beats, and timestamp transitions when recreating product clips.",
		"Match output aspect ratio, duration, and reference inputs to the target placement before rendering.",
	}
	if strings.EqualFold(strings.TrimSpace(zone.MediaType), "image") {
		checklist = []string{
			"Lead with exact product geometry, material texture, and core visual selling points.",
			"Keep lighting direction, lens framing, and background palette consistent across PDP and carousel sets.",
			"Lock aspect ratio and resolution tier before batch variations.",
		}
	}
	var adaptHTML strings.Builder
	adaptHTML.WriteString("<h2>How to Adapt These Prompts</h2><ul>")
	for _, bullet := range checklist {
		adaptHTML.WriteString("<li>")
		adaptHTML.WriteString(esc(bullet))
		adaptHTML.WriteString("</li>")
	}
	adaptHTML.WriteString("</ul>")

	heading := fmt.Sprintf("%s Prompts (%s+ Examples)", zone.Label, formatCommaInt(zone.Count))
	desc := inspirationModelMetaDescription(zone)
	nav := strings.Join([]string{
		fmt.Sprintf(`<a href="%s/">Home</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration">Prompt inspiration</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration/tiktok">TikTok Ad Library</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/pricing">Pricing</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/guides">Guides</a>`, esc(siteURL)),
	}, " | ")

	var relatedLinks []string
	if hasPricingPage {
		relatedLinks = append(relatedLinks, fmt.Sprintf(`<a href="%s/pricing/%s">%s API Pricing</a>`, esc(siteURL), esc(target.PricingID), esc(target.Label)))
	}
	for _, guide := range inspirationZoneBlogGuides(zone) {
		relatedLinks = append(relatedLinks, fmt.Sprintf(`<a href="%s%s">%s</a>`, esc(siteURL), esc(guide.URL), esc(guide.Label)))
	}
	relatedLinks = append(relatedLinks,
		fmt.Sprintf(`<a href="%s/inspiration">All prompt libraries</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration/tiktok">TikTok Ad Library</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/tools/tiktok-transcript-generator">TikTok Transcript Generator</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/ai-influencer-studio">AI Influencer Studio</a>`, esc(siteURL)),
	)

	return fmt.Sprintf(
		`<noscript><div id="geo-ssr"><header><p>OmniMux</p><nav>%s</nav></header><main><h1>%s</h1><p>%s</p><p>%s prompts indexed in the public library.</p>%s%s%s<h2>Related Pricing &amp; Guides</h2><p>%s</p></main></div></noscript>`,
		nav,
		esc(heading),
		esc(desc),
		esc(formatCommaInt(zone.Count)),
		categoryBlock,
		capabilityBlock,
		adaptHTML.String(),
		strings.Join(relatedLinks, " | "),
	)
}

func isInspirationQuarantineQuery(values url.Values) bool {
	v := strings.TrimSpace(strings.ToLower(values.Get("quarantine")))
	return v == "1" || v == "true"
}

func (pages webSearchPages) serveInspirationModelNotFound(c *gin.Context) {
	middleware.SetHTMLCacheHeaders(c, false)
	c.Header("X-Robots-Tag", "noindex, follow")
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusNotFound)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", pages.private)
}

func (pages webSearchPages) serveInspirationModelPrivateOK(c *gin.Context) {
	middleware.SetHTMLCacheHeaders(c, false)
	c.Header("X-Robots-Tag", "noindex, follow")
	c.Data(http.StatusOK, "text/html; charset=utf-8", pages.private)
}

func (pages webSearchPages) serveInspirationModelPage(c *gin.Context, urlPath string) {
	if strings.HasSuffix(urlPath, "/") && urlPath != inspirationModelsPrefix {
		location := strings.TrimRight(urlPath, "/")
		if location != "/inspiration/models" {
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
	}
	slug := strings.TrimPrefix(urlPath, inspirationModelsPrefix)
	if slug == "" || strings.Contains(slug, "/") {
		pages.serveInspirationModelNotFound(c)
		return
	}

	siteURL := searchPageCanonicalHost(pages.public["/"])
	snapshot := pages.catalog.get(siteURL)
	if snapshot == nil {
		// Fail open when the runtime catalog cannot be reached so outages do not emit 404s.
		pages.serveInspirationModelPrivateOK(c)
		return
	}

	if lower := strings.ToLower(slug); lower != slug {
		if _, ok := snapshot.byID[lower]; ok {
			location := inspirationModelsPrefix + lower
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
	}

	zone, found := snapshot.byID[slug]
	if !found {
		pages.serveInspirationModelNotFound(c)
		return
	}
	if isInspirationQuarantineQuery(c.Request.URL.Query()) {
		pages.serveInspirationModelPrivateOK(c)
		return
	}

	page, err := renderWebSearchPage(string(pages.private), webSearchPage{
		Head:    inspirationModelHead(siteURL, zone),
		Summary: inspirationModelSummary(siteURL, zone),
	})
	if err != nil {
		pages.serveInspirationModelPrivateOK(c)
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

func renderDynamicSitemap(staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot) []byte {
	return renderDynamicSitemapWithModels(staticSitemap, siteURL, snapshot, nil)
}

func renderDynamicSitemapWithModels(staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot, pricingModels []model.Pricing) []byte {
	site := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	hasZones := snapshot != nil && len(snapshot.zones) > 0
	if len(staticSitemap) == 0 || site == "" || (!hasZones && len(pricingModels) == 0) {
		return staticSitemap
	}
	raw := string(staticSitemap)
	closeTag := "</urlset>"
	closeIndex := strings.Index(raw, closeTag)
	if closeIndex < 0 {
		return staticSitemap
	}

	existing := make(map[string]bool)
	for _, match := range inspirationSitemapLocPattern.FindAllStringSubmatch(raw, -1) {
		existing[match[1]] = true
	}
	fallbackLastmod := ""
	if match := inspirationSitemapLastmodPattern.FindStringSubmatch(raw); len(match) > 1 {
		fallbackLastmod = match[1]
	}

	esc := htmlEscaper.Replace
	var extra strings.Builder
	appendEntry := func(loc, lastmod string) {
		if existing[loc] {
			return
		}
		existing[loc] = true
		extra.WriteString("  <url><loc>")
		extra.WriteString(esc(loc))
		extra.WriteString("</loc>")
		if lastmod != "" {
			extra.WriteString("<lastmod>")
			extra.WriteString(esc(lastmod))
			extra.WriteString("</lastmod>")
		}
		extra.WriteString("</url>\n")
	}

	if hasZones {
		zoneLastmod := snapshot.lastmod
		if zoneLastmod == "" {
			zoneLastmod = fallbackLastmod
		}
		for _, zone := range snapshot.zones {
			appendEntry(site+inspirationModelsPrefix+zone.ID, zoneLastmod)
		}
	}
	for _, row := range pricingModels {
		if !isIndexablePricingModel(row) {
			continue
		}
		lastmod := fallbackLastmod
		if row.UpdatedTime > 0 {
			lastmod = time.Unix(row.UpdatedTime, 0).UTC().Format("2006-01-02")
		}
		appendEntry(site+pricingModelsPrefix+url.PathEscape(row.ModelName), lastmod)
	}
	if extra.Len() == 0 {
		return staticSitemap
	}
	before := raw[:closeIndex]
	if before != "" && !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	return []byte(before + extra.String() + raw[closeIndex:])
}

func serveDynamicSitemap(c *gin.Context, staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot) {
	serveDynamicSitemapWithModels(c, staticSitemap, siteURL, snapshot, nil)
}

func serveDynamicSitemapWithModels(c *gin.Context, staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot, pricingModels []model.Pricing) {
	body := renderDynamicSitemapWithModels(staticSitemap, siteURL, snapshot, pricingModels)
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "application/xml; charset=utf-8")
		c.Status(http.StatusOK)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
}
