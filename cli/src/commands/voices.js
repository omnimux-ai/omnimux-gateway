/**
 * omnimux voices — search and list available voice timbres and IDs.
 *
 * Supports filtering by keyword (--query), scene/category (--scene),
 * language (--language), resource ID or gateway model compatibility (--model).
 */

const { printJson } = require("../lib/output");
const VOICES_DATA = require("../data/voices.json");

const SCENE_ALIASES = {
  电商带货: ["广告", "带货", "营销", "促销", "解说", "活力", "剪映同款", "抖音同款"],
  电商: ["广告", "带货", "营销", "促销", "解说", "活力"],
  带货: ["广告", "带货", "营销", "促销", "解说", "活力"],
  广告营销: ["广告", "营销", "促销", "解说", "活力"],
  影视解说: ["解说", "悬疑", "视频配音", "TVB", "译制片", "有声阅读"],
  解说: ["解说", "悬疑", "播客", "故事"],
  出海: ["外语", "英语", "美式", "英式", "外语音色"],
  跨境: ["外语", "英语", "美式", "英式", "外语音色"],
  外语: ["外语音色", "外语", "英语"],
  角色扮演: ["角色扮演", "视频配音", "趣味口音"],
  有声阅读: ["有声阅读", "小说", "故事", "播客", "绘本"],
  客服: ["客服场景", "客服"],
};

function matchScene(v, sceneInput) {
  if (!sceneInput) return true;
  const target = sceneInput.trim().toLowerCase();

  // 1. Direct text include check
  const inCategory = v.category && v.category.toLowerCase().includes(target);
  const inTags = Array.isArray(v.tags) && v.tags.some((t) => String(t).toLowerCase().includes(target));
  const inSection = v.section && v.section.toLowerCase().includes(target);
  const inName = (v.name && v.name.toLowerCase().includes(target)) ||
                (v.display_name && v.display_name.toLowerCase().includes(target));

  if (inCategory || inTags || inSection || inName) return true;

  // 2. Semantic aliases check
  for (const [aliasKey, keywords] of Object.entries(SCENE_ALIASES)) {
    if (target.includes(aliasKey.toLowerCase()) || aliasKey.toLowerCase().includes(target)) {
      const matchAny = keywords.some((kw) => {
        const k = kw.toLowerCase();
        return (
          (v.name && v.name.toLowerCase().includes(k)) ||
          (v.display_name && v.display_name.toLowerCase().includes(k)) ||
          (v.category && v.category.toLowerCase().includes(k)) ||
          (v.section && v.section.toLowerCase().includes(k)) ||
          (Array.isArray(v.tags) && v.tags.some((t) => String(t).toLowerCase().includes(k)))
        );
      });
      if (matchAny) return true;
    }
  }

  return false;
}

function filterVoices(allVoices, flags = {}) {
  const query = String(flags.query || flags.q || "").trim().toLowerCase();
  const scene = String(flags.scene || flags.category || "").trim().toLowerCase();
  const language = String(flags.language || flags.lang || "").trim().toLowerCase();
  const resourceId = String(flags.resourceId || flags["resource-id"] || "").trim().toLowerCase();
  const model = String(flags.model || "").trim().toLowerCase();

  let filtered = allVoices.filter((v) => {
    if (query) {
      const matchName = v.name && v.name.toLowerCase().includes(query);
      const matchDisplay = v.display_name && v.display_name.toLowerCase().includes(query);
      const matchType = v.voice_type && v.voice_type.toLowerCase().includes(query);
      const matchTags = Array.isArray(v.tags) && v.tags.some((t) => String(t).toLowerCase().includes(query));
      const matchSection = v.section && v.section.toLowerCase().includes(query);
      if (!matchName && !matchDisplay && !matchType && !matchTags && !matchSection) {
        return false;
      }
    }
    if (scene && !matchScene(v, scene)) {
      return false;
    }
    if (language) {
      const matchLang = v.language && v.language.toLowerCase().includes(language);
      if (!matchLang) return false;
    }
    if (resourceId) {
      const matchRes = v.resource_id && v.resource_id.toLowerCase().includes(resourceId);
      if (!matchRes) return false;
    }
    if (model) {
      if (model.includes("seed-tts-3") || model.includes("doubao-seed-tts-3") || model.includes("3.0")) {
        if (v.resource_id !== "doubao-seed-tts-3.0") return false;
      } else {
        return false;
      }
    }
    return true;
  });

  // Attach recommended_model for clarity
  filtered = filtered.map((v) => ({
    name: v.name,
    display_name: v.display_name || v.name,
    voice_type: v.voice_type,
    category: v.category || "",
    language: v.language || "",
    resource_id: v.resource_id || "",
    recommended_model: "seed-tts-3.0",
    capabilities: Array.isArray(v.capabilities) ? v.capabilities : [],
    tags: Array.isArray(v.tags) ? v.tags : [],
    section: v.section || "",
  }));

  if (flags.limit !== undefined && flags.limit !== null && flags.limit !== "") {
    const limitNum = Number(flags.limit);
    if (Number.isFinite(limitNum) && limitNum > 0) {
      filtered = filtered.slice(0, limitNum);
    }
  }

  return filtered;
}

async function run(ctx) {
  const items = filterVoices(VOICES_DATA, ctx.flags || {});
  printJson(
    {
      total: VOICES_DATA.length,
      count: items.length,
      items,
    },
    ctx.flags
  );
}

module.exports = {
  run,
  filterVoices,
  matchScene,
};
