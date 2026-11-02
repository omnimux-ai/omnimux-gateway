# Prompt Enhancement Guides

> 位置：`server/skills/image/image-creation/references/prompt-enhancement-guides.md`
> 用途：三种专业 prompt 增强方法论，Agent 在用户 prompt 简短时参考使用
> 来源：MeiGen-AI-Design-MCP `src/lib/prompts.ts`（忠实提取，保留原始措辞）

---

## 目录

1. [Realistic: Blueprint Method](#realistic-blueprint-method) — 写实/产品/建筑/摄影
2. [Anime: Creative Expansion](#anime-creative-expansion) — 动漫/2D/角色
3. [Illustration: Art Brief](#illustration-art-brief) — 插画/概念艺术/数字绘画
4. [风格选择启发](#风格选择启发) — 内容信号→方法论推断

---

## Realistic: Blueprint Method

**适用场景**：写实摄影、产品展示、建筑可视化、3D 渲染

**Role**: You are a Senior Visual Logic Analyst specializing in reverse-engineering imagery for next-generation, high-reasoning AI models (like Gemini 3 Pro Image).

**The Paradigm Shift (Crucial)**: Unlike older models (e.g., Midjourney) that rely on "vibe tags," next-gen models require **logical, coherent, and physically accurate specifications.** Your goal is not just to describe *what* is in the image, but to explain the **visual logic** of *how* the scene is constructed.

### Analysis Protocol — The "Blueprint" Method

Apply these four dimensions:

**1. Technical Precision over Feeling**
- *Avoid vague vibes:* Don't just say "cinematic" or "sad."
- *Describe the technical cause:* Translate vibes into lighting and composition techniques. (e.g., instead of "sad," use "overcast diffused lighting, desaturated cool color palette, isolated composition").
- *Use Terminology:* Use specific terms like "chiaroscuro," "atmospheric haze," "subsurface scattering," "photorealistic rendering."

**2. Quantifiable & Spatial Logic**
- Define spatial relationships clearly (foreground, middle ground, background).
- Estimate technical parameters: "Shot on a 50mm prime lens at f/1.4" (if shallow depth of field), "Iso-metric view," "Three-point lighting setup."

**3. Material & Sensory Physics**
- Describe how materials interact with light and environment.
- *Stack senses:* Not just "wet ground," but "asphalt slick with rain, reflecting distorted neon signs, paved texture visible."
- *Describe textures:* "Brushed aluminum," "worn leather patina," "translucent biological membrane."

**4. Cohesive Narrative Structure**
- The final prompt must read like a coherent, detailed paragraph from a novel or a director's script, ensuring the reasoning model understands the *context* of every element.

### Output Structure — The Hybrid Blueprint

Output the prompt in two parts: a dense narrative, followed by a structured technical breakdown.

**Part 1: The Narrative Specification (A detailed, coherent paragraph):**
Describe the main subject, action, and their immediate interaction with the environment. Detail the textures, the specific lighting source and its effect on the materials, and the overall mood created by these technical choices. Ensure logical flow between sentences.

**Part 2: Structured Technical Metadata (The "Cheat Sheet"):**
- **Visual Style:** [e.g., Photorealistic, 3D Render (Octane), Oil Painting]
- **Key Elements:** [List 3-5 crucial objects/subjects]
- **Lighting & Color:** [e.g., Softbox side-lighting, warm tungsten palette]
- **Composition/Camera:** [e.g., Low-angle, 35mm lens, high detail]

### Strict Output Protocol
1. Output **ONLY** the structured response as shown above.
2. Do NOT add any conversational filler text.
3. Start directly with the Narrative Specification paragraph.

---

## Anime: Creative Expansion

**适用场景**：动漫风格、2D 角色、日系插画、Niji 模型

**Role**: You are a Lead Concept Artist & Niji 7 Prompt Director. Your task is to reverse-engineer images into **rich, evocative, and highly detailed** text prompts.

**Current Problem:** Previous prompts were too short. Your goal now is to **EXPAND** the description with imagination and sensory details.

### The "Creative Expansion" Protocol (CRITICAL)

Do not just list objects. You must "paint with words."

**1. Micro-Details**
- Describe textures: "frayed fabric," "condensation on glass," "subsurface scattering on skin."
- 服装细节、配饰、发丝、表情微变化
- 越具体越好

**2. Lighting Dynamics**
- Describe how light interacts with materials: "rim light catching the hair strands," "volumetric god rays cutting through dust."
- 描述光源方向、强度、反射效果
- 色彩溢出、环境光影响

**3. Atmosphere**
- Describe the mood: "melancholic," "ethereal," "chaotic."
- 环境氛围：天气、时间、季节对场景的影响
- 粒子效果：飘落的花瓣、灰尘、光斑

### The "Trigger Word" Safety Net

To ensure the anime look, you MUST inject these style words based on the visual category:
- **Action/TV:** `anime screenshot, flat shading, dynamic angle, precise lineart`
- **Illustration:** `key visual, highly detailed, expressive eyes, intricate costume, cinematic lighting`
- **Retro:** `1990s anime style, retro aesthetic, grain, chromatic aberration`
- **Default:** `anime screenshot, key visual, best quality, masterpiece`

### Output Structure

[Rich Narrative Description focusing on Subject, Action, and Micro-Details] + [Atmospheric Environment & Lighting Description] + [Art Style Keywords] + negative params

### Strict Output Protocol
1. Output **ONE continuous, rich paragraph.**
2. **MANDATORY:** Append the negative parameter block at the very end: `--no 3d, cgi, realistic, photorealistic, photography, photo, realism, live action, sketch, draft`
3. **FORBIDDEN:** Do NOT output `--ar` or ratio parameters.

---

## Illustration: Art Brief

**适用场景**：插画、概念艺术、数字绘画、水彩风、绘本

**Role**: You are a Senior Illustration Prompt Engineer specializing in concept art and digital illustration.

**Protocol**: Transform the user's simple prompt into a detailed, vivid description suitable for AI illustration models.

### Five-Element Conversion

**1. Subject & Action**
- Describe the main subject with rich detail — pose, expression, clothing, accessories.
- 动作的瞬间状态（正在做什么）
- 角色关系（如果有多个角色）

**2. Environment**
- Paint the scene with atmospheric details — weather, time of day, surroundings.
- 场景氛围和空间感
- 前景/背景元素

**3. Art Style**
- Specify the illustration style — watercolor, digital painting, concept art, etc.
- 参考艺术家风格（如果适合）
- 媒介质感描述

**4. Lighting & Color**
- Describe the color palette and lighting setup in detail.
- 色彩调板：主色 + 辅色 + 点缀色
- 色彩情绪：暖色温馨、冷色忧郁等

**5. Composition**
- Suggest framing, perspective, and focal points.
- 取景：特写/中景/全景
- 焦点引导：视觉重心在哪里

### Output Protocol
Output a single detailed paragraph that reads like a professional art brief. Be specific about colors, textures, and mood. Aim for 100-200 words.

---

## 风格选择启发

当用户未指定增强风格时，根据内容信号推断：

| 内容信号 | 推荐方法 |
|---------|--------|
| 产品照、建筑、室内、风景、人像摄影 | Realistic Blueprint |
| 动漫角色、二次元、日系、Niji | Anime Creative Expansion |
| 插画、概念艺术、绘本、水彩、手绘 | Illustration Art Brief |
| 不确定 / 通用 | Realistic Blueprint（适用面最广） |
