# Compliance Rewrite Guide

Load this reference when the user's image request risks triggering model safety filters: characters (especially female), gore/violence, real-name celebrities, brand logos, or any vocabulary the model treats as adult/restricted.

Goal: rewrite "desire words" into "aesthetic words" so the **English** prompt passes safety review while still expressing the user's creative intent. The base SKILL's `Step 3 CRITICAL: Always use English` is preserved — every replacement below is given in English.

---

## 1. Three-Color Vocabulary (CN ↔ EN)

### 🟢 Green — Stable, recommended

Use freely. These describe aesthetics, not desire:

| CN | EN |
|----|----|
| 成年女性角色 / 成熟气质 | adult female character, mature presence |
| 高级女性美 / 优雅从容 | refined feminine beauty, composed and graceful |
| 健康丰腴 / 自然曲线 | healthy and full figure, natural body curves |
| S 型姿态 / 体态协调 | natural S-curve posture, proportionate physique |
| 高级时尚感 / 商业人像 | high-end editorial, commercial portrait |
| 品牌 Lookbook / 时尚 editorial | brand lookbook, fashion editorial |
| 干净柔光 / 得体服装 | soft clean lighting, tasteful attire |
| 精致妆容 / 角色美术设计 | refined makeup, character art design |

### 🟡 Yellow — Use with care, translate first

These often pass but are unstable. **Replace before sending to the model:**

| CN (avoid) | EN replacement |
|----|----|
| 性感 / sexy | high-end feminine elegance, mature charm |
| 魅惑 / seductive | mysterious elegance, eyes with a story |
| 身材火辣 / hot body | proportionate full figure, naturally flowing curves |
| 妩媚 / sultry | warm and composed, confidently mature |
| 丰满 / voluptuous | healthy and full, mature physique |
| 撩人 / alluring | charming yet restrained |
| 贴身 / tight-fitting | tailored fit, body-conforming cut |
| 暧昧 / suggestive ambience | soft and dreamy, emotionally delicate |
| 床上 / on the bed | bright interior, soft bedroom set with poised posture |
| 泳装 / swimwear | swimwear brand lookbook / product showcase / vacation styling |

### 🔴 Red — Drop entirely

These are the most likely to be auto-rejected and are **never the high-end version of the same idea** — they only push the image toward gratuitous risk:

```text
seductive, provocative, flirtatious, exposed, nude, breast close-up,
butt close-up, cleavage, private parts, wet-body tease, slipping clothes,
suggestive pose, adult photoshoot, racy, erotic,
underage / loli / childlike sensuality
```

Replacement strategy: rewrite the whole sentence using Green vocabulary above, not a 1-to-1 swap.

---

## 2. Rewrite Formula

Apply in order:

```text
desire word
   ↓ replace with
aesthetic word (Green)
   ↓ then describe
whole-body posture (not body parts)
   ↓ make clothing
tailored / tasteful (not exposed)
   ↓ make scene
bright / clean / commercial (not private / suggestive)
   ↓ frame the shot
editorial / commercial angle (not low-angle / staring)
   ↓ append
safety negative-prompt boundary
```

---

## 3. Five Failure Causes — ❌ vs ✅ (English)

### 3.1 Age unclear

```text
❌ young girl, loli, childlike, youthful and seductive
✅ a 20-year-old woman, adult female character, mature natural face,
   gentle and refined, confident composure
```

### 3.2 Body parts singled out

```text
❌ emphasize chest, emphasize hips, large breasts, perky butt, body close-up
✅ healthy and full upper body, natural chest contour,
   overall figure proportionate and rounded,
   balanced ratio with soft shoulder and waist line
```

### 3.3 Suggestive pose

```text
❌ teasing pose, lying down, alluring gesture, body leaning forward
   to show figure, low-angle upward shot
✅ natural standing pose, composed seated posture, slight side turn,
   shoulders relaxed, graceful and unforced motion
```

### 3.4 Private scene

```text
❌ bed photoshoot, wet bathroom, dim suggestive room, private space
✅ bright interior space, soft bedroom set, refined home environment,
   light pastel background, clean natural light, warm restrained mood
```

### 3.5 Over-revealing wardrobe

```text
❌ revealing, ultra-short, low-cut, transparent, almost falling off
✅ refined cut, soft fabric, tailored fit, body-conforming yet tasteful,
   overall elegant and high-end
```

---

## 4. Three Bonus Scenarios

### 4.1 Swimwear

```text
❌ sexy swimsuit model, hot body, wet, low-angle
✅ high-end swimwear brand lookbook, adult female model, healthy and confident,
   composed natural posture, tasteful resort or sport swimwear,
   focus on swimwear cut, fabric, color, use scene, fresh summer mood
```

### 4.2 Maid / sweet-fantasy outfit

```text
❌ sexy maid, teasing pose, childlike cute
✅ 20-year-old adult fantasy-styled female character,
   sweet dreamy pink-themed outfit, refined and cute overall styling,
   mature and gentle presence, composed natural posture,
   wardrobe leans toward character art and fantasy costume design
```

### 4.3 Look-back pose

```text
❌ sexy look-back, emphasize hips, low-angle
✅ body slightly side-turned, head naturally looking back toward camera,
   shoulder and neck line relaxed, graceful and composed,
   overall feel of high-end fashion portraiture
```

---

## 5. Negative Prompt Templates (append to `negativePrompt`)

### Standard
```text
nudity, suggestive pose, vulgar expression, body-part close-up,
low-angle staring shot, underage appearance, over-revealing clothing.
Keep the image as high-end character design, editorial portrait,
commercial visual, aesthetic expression.
```

### Short
```text
high-end, tasteful, restrained; no suggestive posing;
no body-part emphasis; no adult or vulgar expression.
```

### Strong-constraint
```text
focus on character presence, garment texture, posture beauty,
light and shadow layering, character design;
do not use body exposure as the visual focus.
```

---

## 6. 8-Item Self-Check (run before calling `generate-image`)

1. ✅ Prompt contains an explicit adult declaration (e.g., `adult, 20+ years old`)?
2. ✅ "Young / cute" never co-occurs with "sexy / seductive / alluring"?
3. ✅ No isolated emphasis on chest / hips / specific body parts?
4. ✅ No words from the 🔴 Red list (seductive, provocative, exposed, nude, etc.)?
5. ✅ No low-angle staring, body-gaze, or part close-up framing?
6. ✅ Wardrobe described by tailored / tasteful / fit, not by exposure level?
7. ✅ Scene described as bright / clean / commercial, not private / suggestive?
8. ✅ Visual focus is on presence, posture, wardrobe, light, character design — not flesh?

If all 8 pass, generation stability rises sharply.

---

## 7. Failure Retry Matrix

| Failure signal | Do this — do not just simplify |
|----------------|--------------------------------|
| Prompt rejected (content_policy / safety / NSFW response) | Drop every 🔴 Red word; rewrite using §1 Green vocab; add §5 negative template |
| Result too conservative / not pretty enough | Do NOT add "more sexy". Instead: `increase fashion expressiveness, enhance refined feminine presence, optimize body ratio and posture, make figure more proportionately full, improve tailoring fit, enhance soft lighting and skin texture` |
| Body deformed (extra fingers / twisted limbs) | Append: `proportionate body, shoulders / waist / leg length match real human anatomy, correct finger count, natural limbs, stable seated posture, avoid distortion` |
| Face too childlike | Append: `mature adult female face shape, not childlike, refined yet adult features, gentle confident presence` |
| Image still feels racy | Append: `return to high-end commercial portraiture and character design, reduce body exposure, reduce suggestive atmosphere, emphasize wardrobe, lighting, posture, character presence` |

---

## 8. Extended Coverage

### 8.1 Gore / Violence

```text
❌ blood, gore, dismemberment, graphic injury, wound close-up
✅ dramatic action scene, stylized impact, cinematic conflict moment,
   intense expression, dynamic motion — keep the focus on action and atmosphere,
   not anatomical injury
```

If the user explicitly wants horror / dark fantasy: shift to **stylized** vocabulary (ink-wash, charcoal sketch, comic shading) and describe **mood** (foreboding, eerie) instead of injury detail.

### 8.2 Real-name celebrities / public figures

GPT Image 2 and most managed image models refuse named-likeness prompts.

```text
❌ generate a photo of <real celebrity name>
✅ a person inspired by <general descriptor: era, profession, style>,
   for example: "a 1960s European screen-icon-style portrait,
   confident gaze, classic Hollywood lighting"
```

Rules:
- Never write the literal full name of a living public figure.
- For historical figures (>70 years deceased, public domain), still phrase as "in the style of" and decline if uncertain.
- If the user provides a reference image of the person, pass it via `referenceImageUrls` (per §Step 3 CRITICAL of SKILL.md) and **describe the visual qualities** in the prompt, not the identity.

### 8.3 Brand / Trademark / Logo

```text
❌ Nike logo on the shoe, Coca-Cola can, Apple iPhone screen
✅ generic athletic shoe with a minimal stripe motif,
   red carbonated-drink can with abstract typography,
   modern smartphone with clean glass front
```

Rules:
- Drop the literal brand name and trademark.
- Replace with the **product category + neutral visual descriptor**.
- If the user wants a brand-affiliated mockup, route them to a template / mockup tool — this skill should not synthesize trademarked marks.

---

## 9. Quick-Reference Card

> When in doubt, the principle:
> **Write less body stimulus, more high-end aesthetics.**
> **Write less body parts, more overall posture.**
> **Write less suggestive action, more natural composure.**
> **Write less racy atmosphere, more commercial quality.**

Truly attractive image generation is restrained, mature, natural, and textured — never racy.
