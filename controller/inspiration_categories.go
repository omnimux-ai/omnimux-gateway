package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// fork: inspiration-categories
//
// Category dictionary endpoint for community inspiration discovery and prompt
// generation. Callers (agents, UI selectors, CLIs) query this dictionary to
// discover valid category keys, bilingual display names, and explicit
// required/optional field contracts for local publishing.

// InspirationCategoryItem represents one standard inspiration category.
type InspirationCategoryItem struct {
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	NameEn         string   `json:"name_en"`
	Group          string   `json:"group"`
	GroupKey       string   `json:"group_key"`
	Description    string   `json:"description"`
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
}

var standardPublishRequiredFields = []string{"category", "title", "prompt"}
var standardPublishOptionalFields = []string{
	"description",
	"model",
	"media",
	"cover",
	"media_type",
	"expire",
}

// inspirationCategories holds the curated list of inspiration categories.
var inspirationCategories = []InspirationCategoryItem{
	// ── 电商营销 (Ecommerce & Marketing) ──
	{
		Key:            "ecommerce-main-image",
		Name:           "电商主图",
		NameEn:         "E-commerce main image",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "适用于电商 Listing、独立站商品主图与白底/场景挂图设计",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "product-ads",
		Name:           "产品广告",
		NameEn:         "Product ads",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "快节奏带货商品展示短视频、痛点反转与核心卖点高光分镜",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "product-marketing",
		Name:           "产品营销",
		NameEn:         "Product marketing",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "全渠道品牌宣传、产品功能演示与转化导向的营销视觉素材",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "brand-commercial",
		Name:           "品牌商业广告",
		NameEn:         "Brand commercial",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "高质感品牌形象片、TVC 质感商业广告与品牌调性宣传",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "poster-flyer",
		Name:           "海报传单",
		NameEn:         "Poster & flyer",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "活动促销海报、社交媒体宣发单页与排版设计",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "character-product-consistency",
		Name:           "主体与产品一致性",
		NameEn:         "Subject & product consistency",
		Group:          "电商营销",
		GroupKey:       "ecommerce",
		Description:    "固定模特、固定商品 SKU 在多场景或多镜头下的连续性生成",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},

	// ── 短视频与短剧 (Short Video & Drama) ──
	{
		Key:            "ugc-talking-head",
		Name:           "UGC 口播视频",
		NameEn:         "UGC talking head",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "真人出镜感、第一人称真实种草与强情绪引导的口播短视频",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "ugc-vlog",
		Name:           "UGC Vlog",
		NameEn:         "UGC vlog",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "日常生活记录、开箱实测、沉浸式陪伴感生活方式 Vlog",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "cinematic-film",
		Name:           "电影级影片",
		NameEn:         "Cinematic film",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "宽银幕大片画质、戏剧性光影与专业导演级运镜视效",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "cinematic-story",
		Name:           "电影级叙事",
		NameEn:         "Cinematic story",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "连贯剧情脉络、人物对白互动与影视级情感张力刻画",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "cinematic-realistic",
		Name:           "电影级写实",
		NameEn:         "Cinematic realistic",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "超写实皮肤纹理、真实物理光照与沉浸式现场记录感",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "30s-stories-one-take",
		Name:           "30 秒一镜到底",
		NameEn:         "30s one-take story",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "单镜头长运镜穿越叙事，30 秒高密度视觉演进",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "camera-motion",
		Name:           "运镜特效",
		NameEn:         "Camera motion",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "推拉摇移、希区柯克变焦、FPV 穿越机等专业摄影机动态轨迹",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "first-frame-control",
		Name:           "首帧控制",
		NameEn:         "First-frame control",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "基于指定参考首帧的图生视频、动作延续与动态演化",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "dialogue-native-audio",
		Name:           "原生对白音频",
		NameEn:         "Dialogue with native audio",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "视频画面与多角色原声台词、音效、情绪声轨同步生成",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "wuxia-martial-arts",
		Name:           "武侠短剧",
		NameEn:         "Wuxia martial arts",
		Group:          "短视频与短剧",
		GroupKey:       "video",
		Description:    "刀光剑影、轻功武打分镜与东方古风江湖意境",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},

	// ── 生活与垂类行业 (Lifestyle & Vertical Industries) ──
	{
		Key:            "Health & Wellness",
		Name:           "健康生活",
		NameEn:         "Health & Wellness",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "养生保健、运动减脂、助眠舒缓与健康习惯类热门内容",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Relationships & Lifestyle",
		Name:           "情感社交",
		NameEn:         "Relationships & Lifestyle",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "男女情感、家庭日常、社交热梗与生活方式共鸣内容",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Personal Development",
		Name:           "个人成长",
		NameEn:         "Personal Development",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "职场认知、学习自律、思维认知与励志金句短视频",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Education & Knowledge",
		Name:           "教育知识",
		NameEn:         "Education & Knowledge",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "通俗科普、技能教学、干货拆解与信息可视化演示",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Travel",
		Name:           "旅游出行",
		NameEn:         "Travel",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "自然风光、城市地标巡礼、酒店度假与旅行攻略视觉",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Food & Cooking",
		Name:           "美食烹饪",
		NameEn:         "Food & Cooking",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "烹饪制作、食欲爆棚的近景特写、美食探店与饮品调配",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Fitness",
		Name:           "健身运动",
		NameEn:         "Fitness",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "力量训练、燃脂动作演示、运动装备展示与体态塑形",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "Home & Design",
		Name:           "家居设计",
		NameEn:         "Home & Design",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "室内软装搭配、智能家居实景、建筑美学与收纳改造",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "beauty_skincare",
		Name:           "美妆个护",
		NameEn:         "Beauty & Skincare",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "护肤步骤演示、彩妆妆效特写、成分科普与变美前后对比",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "fashion",
		Name:           "时尚穿搭",
		NameEn:         "Fashion",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "OOTD 街拍走秀、服饰穿搭合集与时装周高级审美画报",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "social-media-post",
		Name:           "社媒图文",
		NameEn:         "Social media post",
		Group:          "生活与垂类",
		GroupKey:       "lifestyle",
		Description:    "小红书图文、Instagram 九宫格、爆款社交配图与卡片排版",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},

	// ── 创意艺术与设计 (Creative Art & Design) ──
	{
		Key:            "anime-animation",
		Name:           "动漫动画",
		NameEn:         "Anime & animation",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "日漫二维赛璐璐、美漫国漫与三维立体角色动作动画",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "comic-storyboard",
		Name:           "漫画分镜",
		NameEn:         "Comic storyboard",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "连环画故事分镜、条漫画面衔接与动态漫叙事构图",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "character-design",
		Name:           "角色设计",
		NameEn:         "Character design",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "原创人物立绘、NPC 概念设定、三视图与角色服饰细节",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "fantasy",
		Name:           "奇幻魔法",
		NameEn:         "Fantasy",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "西幻神话、魔法粒子特效、巨龙与异域异想世界构建",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "sci-fi-cyberpunk",
		Name:           "科幻赛博",
		NameEn:         "Sci-fi & cyberpunk",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "霓虹雨夜、未来机械、太空飞船与赛博朋克都市景观",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "game-asset",
		Name:           "游戏资产",
		NameEn:         "Game asset",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "游戏道具、UI 图标、环境贴图与三维模型概念图",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "app-web-design",
		Name:           "应用网页设计",
		NameEn:         "App & web design",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "现代极简移动 App 界面、Landing Page 与科技感前端看板",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "infographic-edu-visual",
		Name:           "信息图视觉",
		NameEn:         "Infographic & visual",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "结构化数据图解、思维导图、流程步骤与知识图卡",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "music-video",
		Name:           "音乐与视效",
		NameEn:         "Music video & VFX",
		Group:          "创意艺术",
		GroupKey:       "creative",
		Description:    "卡点节拍、迷幻视觉效果、粒子流动与音乐氛围画面",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
	{
		Key:            "community-showcase",
		Name:           "社区精选",
		NameEn:         "Community showcase",
		Group:          "综合精选",
		GroupKey:       "general",
		Description:    "跨品类前沿探索、实验性视觉与社区高赞灵感创作",
		RequiredFields: standardPublishRequiredFields,
		OptionalFields: standardPublishOptionalFields,
	},
}

// InspirationListCategories GET /api/inspiration/v1/categories
// Public dictionary endpoint for available inspiration categories and parameter rules.
func InspirationListCategories(c *gin.Context) {
	groupFilter := strings.ToLower(strings.TrimSpace(c.Query("group")))

	var filtered []InspirationCategoryItem
	seenGroups := make(map[string]string)
	var orderedGroups []gin.H

	for _, item := range inspirationCategories {
		if _, exists := seenGroups[item.GroupKey]; !exists {
			seenGroups[item.GroupKey] = item.Group
			orderedGroups = append(orderedGroups, gin.H{
				"key":  item.GroupKey,
				"name": item.Group,
			})
		}

		if groupFilter != "" {
			if item.GroupKey != groupFilter && strings.ToLower(item.Group) != groupFilter {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":  len(filtered),
			"groups": orderedGroups,
			"items":  filtered,
			"rules": gin.H{
				"publish_required_fields": standardPublishRequiredFields,
				"publish_optional_fields": standardPublishOptionalFields,
				"note":                    "本地发布必须提供分类 (category)、标题 (title) 与提示词 (prompt)；媒体/封面若为本地文件，CLI 自动上传至 R2",
			},
		},
	})
}
