# Production Workflow

## Recommended Folder Shape

Create a task folder under the current workspace:

```text
social-card-<slug>/
  index.html
  render.cjs
  assets/
  output/
```

Use descriptive slugs:

- `social-card-doubao-input`
- `social-card-hiking-outfit`
- `wechat-ai-card-skill-cover`

## HTML/CSS Rendering Pattern

For normal Gxgen project-chat social-card deliverables, use the seed-template structure and render a complete HTML string. Reserve the renderer's compact `cards` input for explicit raw-PNG-only simple drafts or when the HTML would exceed safe tool-call limits; if the task is still a Gxgen deliverable, persist the returned PNGs afterward.

Build one HTML file containing all frames:

```html
<main class="sheet">
  <section class="poster xhs" id="xhs-01">...</section>
  <section class="poster xhs" id="xhs-02">...</section>
  <section class="cover wechat wide" id="wechat-21x9">...</section>
  <section class="cover wechat square" id="wechat-1x1">...</section>
  <section class="wechat-pair-preview" id="wechat-pair-preview">
    <div class="preview-wide">...</div>
    <div class="preview-square">...</div>
  </section>
</main>
```

Each frame must have stable dimensions:

```css
.xhs {
  width: 1080px;
  height: 1440px;
}

.wechat.wide {
  width: 2100px;
  height: 900px;
}

.wechat.square {
  width: 1080px;
  height: 1080px;
}

.wechat-pair-preview {
  width: 2400px;
  min-height: 1180px;
}
```

Use `box-sizing:border-box`, fixed safe margins, and `overflow:hidden`.

For WeChat covers:

- Compose `wechat-21x9` and `wechat-1x1` as separate source frames.
- Also include `wechat-pair-preview` in the same HTML so the pair can be inspected together.
- Export the two real deliverables separately.
- Export the pair preview only when helpful for review.
- The `1:1` cover should use a simplified title derived from the long title, not a crop or text squeeze from the `21:9` frame.

For electronic-magazine pages, place background canvases inside each `.poster` instead of relying only on CSS color:

```html
<section class="poster magazine hero" id="xhs-01">
  <canvas class="mag-bg" data-bg="ink-flow"></canvas>
  <div class="grain"></div>
  <div class="content">...</div>
</section>
```

The skill includes a reusable helper at `assets/magazine-bg-webgl.js`. Reference it exactly as the seed template does — a `<script src="assets/magazine-bg-webgl.js"></script>` plus a `<canvas class="mag-bg" data-bg="ink-flow">` inside each `.poster`. You do NOT need to copy or inline it: the `render-html-to-png` tool stages this helper (and the local webfonts) next to your HTML before rendering, so the relative `assets/...` path resolves and `window.MagazineBg` mounts.

## Rendering

Use the `render-html-to-png` tool (Playwright headless Chrome under the hood):

1. For normal deliverables, pass a complete single-file HTML string in the `html` field.
2. For explicit raw-PNG-only simple drafts, or when the HTML would exceed safe tool-call limits, pass `cards: [{ title, lead, points, kicker, footer }]`.
3. Set `width`, `height`, and the expected `pages` count. `deviceScaleFactor` defaults to `1` (exact CSS pixels, e.g. 1080×1440); pass `2` only when you want a 2× supersampled export.
4. The tool stages the WebGL helper + local serif/sans CJK + mono webfonts beside your HTML, opens it via `file://`, waits for `document.fonts.ready` and the WebGL canvas mount, then screenshots each `<section class="poster">`. So: reference bundled assets by the seed template's relative paths, and reference user / generated / sourced images by **absolute URL** (the render tool does not stage user image files).
5. Output PNGs are saved in the returned output directory.
6. Verify dimensions on the returned files.

If using WebGL or procedural canvas backgrounds:

- Pass a `waitMs` parameter of at least 500-900ms before screenshots so the canvas has rendered.
- Use deterministic seeds or a fixed time when repeatability matters.
- On normal content pages, keep background opacity subtle.
- On cover, divider, pull-quote, and sparse pages, let the atmosphere show more strongly.

Example call to `render-html-to-png`:

```js
render-html-to-png({
  cards: [{
    kicker: "SOCIAL CARD / 01",
    title: "把重复运营交给智能体",
    lead: "团队把巡检、整理、发布前校对交给智能体，人只保留判断和创意。",
    points: ["重复动作自动跑", "异常结果交给人审", "沉淀成可复用流程"],
    footer: "AI TEAM OPERATIONS"
  }],
  width: 1080,
  height: 1440,
  deviceScaleFactor: 1,
  pages: 1,
});
```

## Gxgen Template Assembly

For project-chat requests that ask for 小红书图文 / social-card / 社交图文卡片 / 图文模板, rendering is not the final step. Use the PNG URLs returned by `render-html-to-png` as page assets, then load and call `workflow-slideshow-persist`.

Use one `pages[]` item and one `resolvedImages[]` item per rendered PNG:

```js
workflow-slideshow-persist({
  pages: [
    {
      pageIndex: 1,
      title: "把重复运营交给智能体",
      body: "团队把巡检、整理、发布前校对交给智能体，人只保留判断和创意。",
      imageUrl: "/api/social-card-renders/<renderId>/page-001.png",
      visualHint: "Rendered social-card page"
    }
  ],
  resolvedImages: [
    {
      index: 1,
      url: "/api/social-card-renders/<renderId>/page-001.png",
      sourceType: "rendered-social-card"
    }
  ],
  aspectRatio: "3:4",
  targetPlatform: "rednote",
  slideType: "educational",
  imageSourceMode: "ai",
  replicationMode: "user_skipped"
});
```

Use `replicationMode: "user_skipped"` when there is no source template being replicated. Do not call the broader slideshow bootstrap tools just to persist already-rendered social-card pages. Stop after rendering only when the user explicitly asks for raw PNG files only, no Gxgen template, no project template creation, or no persistence.

Do not print the HTML in chat. If the tool-call JSON becomes too long, use `cards` instead of a full HTML string.

If a local dev server is needed for assets or font loading, start it and tell the user the URL. If `file://` rendering works, no server is required.

## Verification Commands

After rendering, inspect outputs with:

```bash
sips -g pixelWidth -g pixelHeight output/*.png
```

Then run the `validate-social-deck` tool against the task folder to auto-check:

- R1 overflow / R2 footer collision / R3 Swiss bold display / R4 minimum font size / R5 4-band density / R6 `.h-xl` line caps / R7 figure margin drift.

For visual inspection:

- Show final PNGs inline with absolute paths.

## Screenshot Treatment

Programmatic framing is preferred for user-provided screenshots:

- Create a clean target-ratio frame.
- Add plain white, refined grey, or paper background. Do not add page-wide grid/dot backgrounds unless the user explicitly asks for a technical blueprint look.
- If the capture contains a floating window/card over unrelated UI, crop to the foreground subject before placing it.
- Place screenshot inside with safe padding.
- Preserve readable text.
- Do not redraw the screenshot unless the user asked for redesign.
- Do not add perspective, skew, rotation, or mockup tilt unless the user explicitly asks for a scene mockup.

For Swiss:

- Straight corners.
- No shadow by default.
- Hairline only if the screenshot edge disappears.

For editorial magazine:

- Small radius or subtle shadow is allowed, but avoid SaaS marketing-card styling.

## Generated Images

When generating missing visuals:

- Generate only the raw visual asset.
- Keep text out of generated images.
- Match the page role and style mode.
- Reference the generated / sourced image by the **absolute URL** the image tool returns, directly in the HTML `<img src="https://…">`. The render tool stages only its own asset kit (WebGL helper + fonts), so relative `assets/<file>` image paths will not resolve — always use absolute URLs for images.
- Generate only the pages that need it, usually 1-2 images for a set.

## Accessibility And Readability

- Use strong contrast for all text.
- Do not place long text over busy photos.
- If text must sit over photo, use a solid ink/paper block or a high-contrast strip, not a blur blob.
- Avoid negative letter spacing in Chinese body text.
- Use line-height that lets Chinese breathe: roughly 1.08-1.22 for big titles, 1.35-1.55 for body.

## Common Fixes

- Cover feels empty: enlarge title, enlarge image, or add a functional bottom strip.
- Screenshot too small: reduce side text, give screenshot 55%-70% of the canvas.
- Lower area empty: read `portrait-fill.md`; merge pages, add a full-height ledger, use a larger evidence image, add a marginal quote column, or switch to an atmospheric thesis page.
- Style feels generic: add issue metadata, better type hierarchy, a stronger evidence image, richer WebGL/ink atmosphere, or more intentional content dividers.
- Text overflows: shorten copy before shrinking type.
