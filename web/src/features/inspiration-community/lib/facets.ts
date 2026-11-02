/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import i18next from 'i18next'

import { convertDetectedLanguage } from '@/i18n/languages'

import { humanizeFacet } from './format'

/** `key` -> display name per language. Keys come from the Topview/EvoLink crawls. */
const FACET_NAMES: Record<string, { zh: string; en: string }> = {
  // ── Categories (the single `categoryPrimary` of an item) ──────────────
  '30s-stories-one-take': { zh: '30 秒一镜到底故事', en: '30s one-take story' },
  action: { zh: '动作', en: 'Action' },
  'action-anime': { zh: '动作动漫', en: 'Action anime' },
  'anime-animation': { zh: '动漫动画', en: 'Anime animation' },
  'app-web-design': { zh: '应用与网页设计', en: 'App & web design' },
  'brand-commercial': { zh: '品牌商业广告', en: 'Brand commercial' },
  'camera-motion': { zh: '运镜', en: 'Camera motion' },
  'character-design': { zh: '角色设计', en: 'Character design' },
  'character-performance': { zh: '角色表演', en: 'Character performance' },
  cinematic: { zh: '电影感', en: 'Cinematic' },
  'cinematic-film': { zh: '电影级影片', en: 'Cinematic film' },
  'cinematic-showcase': { zh: '电影级展示', en: 'Cinematic showcase' },
  'cinematic-story': { zh: '电影级叙事', en: 'Cinematic story' },
  'comedy-humor': { zh: '喜剧幽默', en: 'Comedy & humor' },
  'comic-storyboard': { zh: '漫画分镜', en: 'Comic storyboard' },
  'community-showcase': { zh: '社区精选', en: 'Community showcase' },
  'dialogue-native-audio': {
    zh: '原生对白音频',
    en: 'Dialogue with native audio',
  },
  'ecommerce-main-image': { zh: '电商主图', en: 'E-commerce main image' },
  'explainer-tutorial': { zh: '讲解教程', en: 'Explainer tutorial' },
  fantasy: { zh: '奇幻', en: 'Fantasy' },
  fashion: { zh: '时尚', en: 'Fashion' },
  'first-frame-control': { zh: '首帧控制', en: 'First-frame control' },
  'game-asset': { zh: '游戏资产', en: 'Game asset' },
  'game-cinematic': { zh: '游戏电影化', en: 'Game cinematic' },
  'horror-dark': { zh: '恐怖暗黑', en: 'Horror & dark' },
  'infographic-edu-visual': {
    zh: '信息图与教学',
    en: 'Infographic & education',
  },
  'music-video': { zh: '音乐视频', en: 'Music video' },
  portraits: { zh: '人像', en: 'Portraits' },
  'poster-flyer': { zh: '海报传单', en: 'Poster & flyer' },
  'product-ads': { zh: '产品广告', en: 'Product ads' },
  'product-marketing': { zh: '产品营销', en: 'Product marketing' },
  'profile-avatar': { zh: '个人头像', en: 'Profile avatar' },
  'romance-drama': { zh: '爱情剧情', en: 'Romance drama' },
  'sci-fi-cyberpunk': { zh: '科幻赛博朋克', en: 'Sci-fi cyberpunk' },
  'short-film': { zh: '短片', en: 'Short film' },
  'social-lifestyle': { zh: '社交生活', en: 'Social lifestyle' },
  'social-media-post': { zh: '社媒图文', en: 'Social media post' },
  storytelling: { zh: '故事叙事', en: 'Storytelling' },
  'stylized-scenes': { zh: '风格化场景', en: 'Stylized scenes' },
  'ugc-talking-head': { zh: 'UGC 口播视频', en: 'UGC talking head' },
  'ugc-vlog': { zh: 'UGC Vlog', en: 'UGC vlog' },
  uncategorized: { zh: '未分类', en: 'Uncategorized' },
  'wuxia-martial-arts': { zh: '武侠', en: 'Wuxia martial arts' },
  'youtube-thumbnail': { zh: 'YouTube 封面', en: 'YouTube thumbnail' },
  'cinematic-realistic': { zh: '电影级写实', en: 'Cinematic realistic' },
  'channel-intro': { zh: '频道片头 / 栏目片头', en: 'Channel intro' },
  'official-seedance-2-5-showcase': {
    zh: '官方精选展示',
    en: 'Official showcase',
  },
  'native-audio-dialogue': {
    zh: '原生对白音频',
    en: 'Native dialogue & audio',
  },
  'product-ads-brand': { zh: '品牌产品广告', en: 'Brand product ads' },
  'vfx-video-editing': { zh: '特效与视频剪辑', en: 'VFX & video editing' },
  '30s-one-shot': { zh: '30 秒一镜到底', en: '30s one-shot' },
  'character-product-consistency': {
    zh: '主体与产品一致性',
    en: 'Subject consistency',
  },
  animation: { zh: '动画', en: 'Animation' },
  vfx: { zh: '视觉特效', en: 'VFX' },
  'music-dance': { zh: '音乐与舞蹈', en: 'Music & dance' },
  'sci-fi-worldbuilding': { zh: '科幻世界观', en: 'Sci-fi worldbuilding' },
  'video-editing': { zh: '视频剪辑', en: 'Video editing' },
  multilingual: { zh: '多语言表达', en: 'Multilingual' },
  'brand-film': { zh: '品牌大片', en: 'Brand film' },
  'motion-graphics': { zh: '动态图形', en: 'Motion graphics' },
  'product-ad': { zh: '产品广告', en: 'Product ad' },
  'twitter-community': { zh: '社区精选', en: 'Community showcase' },
  'cinematic-general': { zh: '电影级画面', en: 'Cinematic general' },
  'fashion-dance': { zh: '时尚舞蹈', en: 'Fashion & dance' },
  'fantasy-mythology': { zh: '奇幻神话', en: 'Fantasy & mythology' },
  'nature-animal': { zh: '自然动物', en: 'Nature & animal' },
  'pov-fpv': { zh: '第一人称与穿越机', en: 'POV & FPV' },
  'transformation-vfx': { zh: '变换与变身特效', en: 'Transformation VFX' },
  'epic-disaster': { zh: '史诗灾难', en: 'Epic disaster' },
  'commercial-product': { zh: '商业产品', en: 'Commercial product' },
  creative: { zh: '创意概念', en: 'Creative concept' },
  'sports-fitness': { zh: '运动健身', en: 'Sports & fitness' },
  'food-beverage': { zh: '美食饮品', en: 'Food & beverage' },
  'ip-character': { zh: 'IP 角色', en: 'IP character' },
  'historical-period': { zh: '历史与古装', en: 'Historical period' },
  'music-audio': { zh: '音乐音频', en: 'Music & audio' },
  'vehicle-transport': { zh: '载具交通', en: 'Vehicle & transport' },
  'advanced-camera-movements': {
    zh: '高阶运镜',
    en: 'Advanced camera movements',
  },
  'creative-visual-effects': {
    zh: '创意视觉特效',
    en: 'Creative visual effects',
  },
  'audio-voice-synthesis': { zh: '语音合成', en: 'Voice synthesis' },
  'story-development-extension': { zh: '剧情延伸', en: 'Story development' },
  'tutorial-prompt': { zh: '提示词教程', en: 'Tutorial & prompt' },
  'character-scene-consistency': {
    zh: '角色与场景一致性',
    en: 'Character & scene consistency',
  },
  animal: { zh: '动物', en: 'Animal' },
  'one-take-cinematography': {
    zh: '一镜到底摄影',
    en: 'One-take cinematography',
  },
  'ultra-realistic-video-generation': {
    zh: '超写实视频',
    en: 'Ultra-realistic video',
  },
  'video-editing-remixing': { zh: '视频混剪', en: 'Video remixing' },
  'music-synchronization': { zh: '音乐对齐卡点', en: 'Music synchronization' },
  'emotional-performance': { zh: '情绪表演', en: 'Emotional performance' },
  'cinematic-scenes': { zh: '电影级场景', en: 'Cinematic scenes' },
  'fantasy-scenes': { zh: '奇幻场景', en: 'Fantasy scenes' },
  vehicle: { zh: '载具', en: 'Vehicle' },
  poster: { zh: '海报', en: 'Poster' },
  portrait: { zh: '人像写真', en: 'Portrait' },
  ui: { zh: '界面设计', en: 'UI design' },
  comparison: { zh: '对比展示', en: 'Comparison' },
  'ad-creative': { zh: '广告创意', en: 'Ad creative' },
  ecommerce: { zh: '电商营销', en: 'E-commerce' },
  posters: { zh: '海报合集', en: 'Posters' },
  'ui-mockups': { zh: 'UI 样机展示', en: 'UI mockups' },
  'marketing-visuals': { zh: '营销视觉素材', en: 'Marketing visuals' },
  'product-photography': { zh: '产品摄影', en: 'Product photography' },
  'abstract-background': { zh: '抽象背景', en: 'Abstract background' },
  'diagram-chart': { zh: '图表与信息图', en: 'Diagram & chart' },
  'anime-manga': { zh: '动漫漫画', en: 'Anime & manga' },
  documentary: { zh: '纪录片', en: 'Documentary' },
  '2d-animation': { zh: '2D 动画', en: '2D animation' },
  'chinese-ink': { zh: '中国水墨', en: 'Chinese ink' },
  'stop-motion': { zh: '定格动画', en: 'Stop motion' },
  'cyberpunk-sci-fi': { zh: '赛博朋克 / 科幻', en: 'Cyberpunk sci-fi' },
  'ink-chinese-style': { zh: '国风水墨', en: 'Chinese ink style' },
  'sketch-line-art': { zh: '素描线稿', en: 'Sketch & line art' },
  watercolor: { zh: '水彩插画', en: 'Watercolor' },
  'comic-graphic-novel': { zh: '美漫图像小说', en: 'Comic graphic novel' },
  'chibi-q-style': { zh: 'Q 版卡通', en: 'Chibi Q style' },
  'oil-painting': { zh: '油画质感', en: 'Oil painting' },
  isometric: { zh: '2.5D 等轴测', en: 'Isometric' },
  'pixel-art': { zh: '像素艺术', en: 'Pixel art' },

  // ── Tags (the chip row at the foot of an L2 card) ─────────────────────
  '3d-cartoon': { zh: '3D 卡通', en: '3D cartoon' },
  '3d-render': { zh: '3D 渲染', en: '3D render' },
  'abstract-vfx-v': { zh: '抽象特效', en: 'Abstract VFX' },
  'animal-pet': { zh: '动物宠物', en: 'Animal & pet' },
  anime: { zh: '动漫', en: 'Anime' },
  'architecture-interior': { zh: '建筑室内', en: 'Architecture & interior' },
  character: { zh: '角色', en: 'Character' },
  'cinematic-film-still': { zh: '电影剧照', en: 'Cinematic film still' },
  'cityscape-street': { zh: '城市街景', en: 'Cityscape street' },
  'cityscape-urban': { zh: '城市景观', en: 'Cityscape urban' },
  'crowd-group': { zh: '人群与合影', en: 'Crowd & group' },
  'cyberpunk-scifi': { zh: '赛博朋克', en: 'Cyberpunk sci-fi' },
  dance: { zh: '舞蹈', en: 'Dance' },
  'fantasy-magical': { zh: '奇幻魔法', en: 'Fantasy & magical' },
  'fashion-item': { zh: '时尚单品', en: 'Fashion item' },
  'food-drink': { zh: '餐饮', en: 'Food & drink' },
  'food-drink-v': { zh: '餐饮', en: 'Food & drink' },
  illustration: { zh: '插画', en: 'Illustration' },
  'influencer-model': { zh: '达人与模特', en: 'Influencer & model' },
  interior: { zh: '室内', en: 'Interior' },
  korean: { zh: '韩系', en: 'Korean' },
  luxury: { zh: '奢华', en: 'Luxury' },
  macro: { zh: '微距', en: 'Macro' },
  minimalism: { zh: '极简', en: 'Minimalism' },
  'nature-landscape': { zh: '自然风景', en: 'Nature landscape' },
  'night-sky': { zh: '夜空', en: 'Night sky' },
  nightlife: { zh: '夜生活', en: 'Nightlife' },
  'person-character': { zh: '人物角色', en: 'Person & character' },
  photography: { zh: '摄影', en: 'Photography' },
  'portrait-selfie': { zh: '自拍人像', en: 'Portrait selfie' },
  product: { zh: '产品', en: 'Product' },
  'product-v': { zh: '产品展示', en: 'Product' },
  'retro-vintage': { zh: '复古', en: 'Retro vintage' },
  skincare: { zh: '护肤', en: 'Skincare' },
  'sports-action': { zh: '运动动作', en: 'Sports action' },
  'surreal-dreamlike': { zh: '超现实梦幻', en: 'Surreal dreamlike' },
  'talking-head': { zh: '口播', en: 'Talking head' },
  'text-typography': { zh: '文字排版', en: 'Text & typography' },
  ugc: { zh: 'UGC', en: 'UGC' },
  'vehicle-v': { zh: '载具', en: 'Vehicle' },
  'vintage-retro': { zh: '复古怀旧', en: 'Vintage retro' },
  vlog: { zh: 'Vlog', en: 'Vlog' },
  watch: { zh: '腕表', en: 'Watch' },
}

/**
 * English names for the canonical Chinese facet labels.
 *
 * The browse surfaces do not show raw crawl slugs: the anchor bar is handed the
 * Chinese label whenever the vocabulary carries one, so those labels — not the
 * slugs above — are what a visitor actually reads. They are product-defined (see
 * `STYLE_ALIASES` / `SUBJECT_ALIASES` in `filter.ts`) and mostly do NOT coincide
 * with the `zh` column, so without their own English names an English reader
 * would be shown Chinese chips.
 */
const FACET_NAMES_BY_LABEL: Record<string, string> = {
  电影级写实风格: 'Cinematic realism',
  '复古 / 怀旧胶片': 'Retro film',
  '超现实 / 梦境': 'Surreal dream',
  极简主义: 'Minimalism',
  水彩: 'Watercolor',
  漫画小说: 'Graphic novel',
  '水墨 / 国风': 'Ink wash',
  'Q版 / 萌系': 'Chibi',
  等轴测: 'Isometric',
  油画: 'Oil painting',
  像素画: 'Pixel art',
  '人物 / 角色': 'Person & character',
  '运动 / 动作': 'Sports & action',
  '城市 / 街景': 'City & street',
  '自然 / 风景': 'Nature & landscape',
  '人群 / 群体': 'Crowd & group',
  '建筑 / 室内': 'Architecture & interior',
  '动物 / 宠物': 'Animal & pet',
  '美食 / 饮品': 'Food & drink',
  产品: 'Product',
  交通工具: 'Vehicle',
  '抽象 / VFX': 'Abstract & VFX',
  '文字 / Logo': 'Text & logo',
  '人像 / 自拍': 'Portrait & selfie',
  '网红 / 模特': 'Influencer & model',
  '风景 / 自然': 'Landscape & nature',
  '团体 / 情侣': 'Group & couple',
  '动物 / 生物': 'Animal & creature',
  图表: 'Charts',
  '食物 / 饮品': 'Food & drink',
}

/**
 * Whitespace and slashes in a Chinese label are presentation, not identity: the
 * crawl writes `奇幻 / 魔法` where a table entry may write `奇幻魔法`, and the two
 * name the same facet. Folding both sides is what lets one index serve every
 * spelling the data actually ships.
 */
function foldFacetKey(value: string): string {
  return value.replaceAll(/[\s/]+/gu, '').toLowerCase()
}

/**
 * Chinese label -> English name, folded.
 *
 * Two sources feed it: the `zh` column of `FACET_NAMES`, and the canonical
 * labels in `FACET_NAMES_BY_LABEL`. Either spelling then resolves to the same
 * English name.
 */
const FACET_EN_BY_ZH = new Map<string, string>()
for (const entry of Object.values(FACET_NAMES)) {
  FACET_EN_BY_ZH.set(foldFacetKey(entry.zh), entry.en)
}
for (const [label, english] of Object.entries(FACET_NAMES_BY_LABEL)) {
  FACET_EN_BY_ZH.set(foldFacetKey(label), english)
}

const CJK_PATTERN = /[\u4e00-\u9fa5]/u

/** Resolve curated facet names through the interface translations. */
export function facetDisplayName(key: string, language: string): string {
  const chinese = language.toLowerCase().startsWith('zh')
  const translate = (english: string, fallback: string): string =>
    i18next.t(english, {
      lng: convertDetectedLanguage(language),
      defaultValue: fallback,
    }) || fallback
  const entry = FACET_NAMES[key]
  if (entry) return translate(entry.en, chinese ? entry.zh : entry.en)
  if (CJK_PATTERN.test(key)) {
    const english = FACET_EN_BY_ZH.get(foldFacetKey(key))
    if (!english) return key
    return translate(english, chinese ? key : english)
  }
  return humanizeFacet(key)
}
