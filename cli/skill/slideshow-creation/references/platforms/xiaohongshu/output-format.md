# 小红书结构化输出规范（历史参考）

> 位置：`server/skills/slideshow/slideshow-creation/references/platforms/xiaohongshu/output-format.md`
> 用途：保留旧版小红书专用工具的输出合同，作为历史参考；当前 Agent 主链已由 `slideshow-creation` 接管

---

# Xiaohongshu Content Legacy Contract

## 概述
根据用户主题生成符合小红书平台调性的图文内容，包含吸睛标题、结构化正文（封面页、内容页、总结页）。支持联网搜索获取最新资讯辅助创作。

<capabilities>
1. 生成 4-12 页图文内容，自动规划封面、内容页、总结页结构
2. 支持 4 种内容风格：轻松口语、专业严谨、爆款风格、科普教育
3. 可选联网搜索，获取最新热点资讯融入内容
4. 每页包含标题、正文、配图建议
</capabilities>

## 行为约束

<rules>
REQUIRED: 标题必须包含 emoji 和数字（如"5个护肤技巧"）
REQUIRED: 正文使用短句、分段、emoji 点缀
REQUIRED: 每页必须包含 imageHint 配图建议
PREFER: 语气亲切、口语化，避免书面语
PREFER: 内容有具体的数据、事实或案例支撑
AVOID: 单页内容不超过 1000 字
AVOID: 负面或争议性表达
FORBIDDEN: 虚假宣传或违规内容
</rules>

## 内容框架

<content_frameworks>
### Hook-Body-CTA 结构（爆款公式）

**Hook（开头吸引）**：
- 数字型：「5个方法」「3分钟学会」
- 痛点型：「90%的人都做错了」
- 悬念型：「你绝对想不到...」
- 对比型：「从XX到XX」

**Body（正文结构）**：
- 问题引入 → 解决方案 → 案例佐证
- 步骤拆解：1️⃣ 2️⃣ 3️⃣ 清晰分点
- 个人经验 + 专业知识融合

**CTA（行动号召）**：
- 互动引导：「你们觉得呢？」「评论区告诉我」
- 关注引导：「关注我，更多干货」
- 收藏引导：「先收藏，不然刷着刷着就找不到了」
</content_frameworks>

## 参考资源

<references>
- `references/platforms/xiaohongshu/viral-frameworks.json`：爆款内容结构参考素材，仅用于灵感启发，不要求严格套用
</references>

## 风格说明

<style_descriptions>
- **casual**（轻松口语）：姐妹聊天式，亲切自然，适合生活分享
- **professional**（专业严谨）：专家视角，数据支撑，适合知识科普
- **viral**（爆款风格）：强情绪、强对比、强互动，适合快速传播
- **educational**（科普教育）：深入浅出，通俗易懂，适合教程分享
</style_descriptions>

## 页面类型

<page_types>
- **cover**：封面页，大标题 + 核心卖点
- **content**：内容页，正文 + 配图建议
- **summary**：总结页，要点回顾 + CTA
</page_types>

## 输出格式

<output_format>
{
  "topic": "主题",
  "pages": [
    {
      "pageIndex": 0,
      "type": "cover | content | summary",
      "title": "页面标题",
      "content": "页面正文",
      "imageHint": "配图建议"
    }
  ]
}
</output_format>

## 示例

<examples>
  <example>
    <input>
      topic: "护肤步骤"
      pageCount: 6
      style: "casual"
    </input>
    <output>
      {
        "topic": "护肤步骤",
        "pages": [
          {"pageIndex": 0, "type": "cover", "title": "🌸 早晚护肤的正确顺序，90%的人都做错了！", "content": "...", "imageHint": "护肤品摆放整齐的平铺图"},
          {"pageIndex": 1, "type": "content", "title": "第一步：清洁", "content": "洗脸不是随便洗洗就行...", "imageHint": "洗面奶泡沫特写"},
          ...
        ]
      }
    </output>
  </example>
</examples>

## 当前状态

- `generateXiaohongshuContent` 旧工具已移除，不再作为 Agent 模式入口
- 当前小红书图文统一走 `slideshow-creation` 主链，再按需叠加平台 overlay
