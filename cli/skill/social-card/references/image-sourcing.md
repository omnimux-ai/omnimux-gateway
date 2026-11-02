# Image Sourcing (Gxgen tools)

This file is read **only when** the Step 1 三选一 gate landed on **Option B (web-sourced)**. If the user supplied images (Option A) or picked AI generation (Option C), skip this file.

Policy: **grab first, disclose after, let the user decide on attribution.** Do not pre-filter sources by guessed license — the user is the rights holder of the final composition and decides what is acceptable. Once the user has picked Option B, **lock it** — do not re-prompt for A or C across subsequent turns.

## Priority list of image sources

All five below are free-tier libraries with no required licensing fees; we do not pull from paid stock sites (视觉中国 / Getty / 站酷海洛 etc.).

| # | Source | Strength | License posture |
|---|---|---|---|
| 1 | **Unsplash** | Outdoor / lifestyle / atmospheric backdrops. English keywords work best. | Permissive, verify case by case. |
| 2 | **Pexels** | **Native Chinese keyword search** — fills Unsplash's gap on 国内场景 (中文街景 / 国风物件 / 本地地名). Use first when the subject is China-specific or the keyword is Chinese. | Pexels License (free). |
| 3 | **Flickr CC pool** | Documentary realness: street photography, people-in-context, real interiors, non-styled scenes that Unsplash/Pexels lack. | Creative Commons only — preserve attribution if the user opts in. |
| 4 | **Wallhaven** | Game / anime / wallpaper themes. | User-uploaded; rights unverified. |
| 5 | **User-provided URLs / direct web search** | Specific subjects (a product render, a game still, a historical photo). | Verify before use. |

Internal first path: for generic page roles (texture, atmosphere, abstract), try `search-assets({ channel: "internal-gallery" })` before external channels. It is more reliable than external hotlinks and does not require an attribution decision.

**Editorial-mode picking order**: `search-assets({ locale: "zh" })` for Chinese keywords → `search-assets({ query: "..." })` for English → explicit `channel: "flickr"` for real-life feel → direct search.

**Swiss mode** rarely needs any of these — product renders, UI screenshots, and keyshot-style images should be user-supplied or AI-generated, not stock.

## URL pattern knowledge

Construct search and download URLs from these templates. Replace `<keyword>` with URL-encoded English (Unsplash / Flickr / Wallhaven) or Chinese (Pexels) terms.

```text
Unsplash search:        https://unsplash.com/s/photos/<keyword>
Pexels search (en):     https://www.pexels.com/search/<keyword>/
Pexels search (zh):     https://www.pexels.com/zh-cn/search/<keyword>/
Flickr CC search:       https://www.flickr.com/search/?text=<keyword>&license=2%2C3%2C4%2C5%2C6%2C9
Wallhaven search:       https://wallhaven.cc/search?q=<keyword>
```

Notes:

- Flickr `license=2,3,4,5,6,9` restricts results to Creative Commons photos.
- Unsplash photo pages: `https://unsplash.com/photos/<slug-or-id>`.
- Pexels photo pages: `https://www.pexels.com/photo/<slug>-<id>/`.
- Flickr photo pages: `https://www.flickr.com/photos/<user>/<photo-id>/`.
- Wallhaven wallpaper pages: `https://wallhaven.cc/w/<id>`.

## Tool workflow

The primary image sourcing tool is **`search-assets`** — a unified asset search tool that routes queries to Unsplash, Pexels, Flickr CC, Wallhaven, or the internal gallery through a single interface. Load it once via `search_tools` + `load_tool`, then call directly for every subsequent search.

### Quick-start examples

```js
// Chinese keyword — pass locale: "zh" to auto-prefer Pexels
search-assets({ query: "江南水乡", locale: "zh", minResolution: { width: 1080, height: 1440 }, orientation: "portrait" })

// English keyword — auto-routing picks the best channel
search-assets({ query: "autumn forest", minResolution: { width: 1080, height: 1440 } })

// Target a specific source
search-assets({ query: "vintage cafe", channel: "flickr", minResolution: { width: 1080, height: 1080 }, orientation: "square" })

// Internal gallery first (most reliable, no attribution)
search-assets({ query: "paper texture", channel: "internal-gallery" })
```

### Parameter guide

| Parameter | When to use | Notes |
|---|---|---|
| `query` | Always required | English for Unsplash/Wallhaven; Chinese supported on Pexels |
| `locale: "zh"` | Query is in Chinese | Auto-prefers Pexels (native Chinese search). Always pass this for Chinese keywords. |
| `locale: "en"` | Query is in English (default) | Auto-routing across all channels |
| `channel` | Need a specific source | Omit for auto-selection; pass to target one channel |
| `minResolution` | Always — avoids manual resolution checks | Use `1080×1440` for 3:4, `2100×900` for 21:9, `1080×1080` for 1:1 |
| `orientation` | Page layout needs specific ratio | `"portrait"` for 3:4, `"landscape"` for 21:9, `"square"` for 1:1 |
| `maxResults` | Need more/fewer candidates | Default 10; max 50 |

### Workflow steps

1. **Gallery-first check** — for generic page roles (texture, atmosphere, abstract backdrop), call `search-assets({ query: "...", channel: "internal-gallery" })` first. If the result fits, use it — no external hotlinking, no attribution question. Skip this step when the page needs a specific real-world subject (cityscape, product, person).
2. **Search with locale** — call `search-assets` with the keyword and appropriate `locale`. For Chinese subjects, pass `locale: "zh"` and the tool auto-prefers Pexels. Always include `minResolution` matching the target ratio so the tool filters out undersized results automatically.
3. **Review results** — the tool returns absolute URLs, thumbnails, dimensions, and attribution metadata. Pick the best candidate by composition and relevance. If results are unsatisfactory, try a different `channel` or refine the `query`.
4. **Use the absolute URL directly in the HTML** — reference the image as `<img src="https://…">`. The `render-html-to-png` tool stages only its own asset kit (WebGL helper + fonts); it does **not** stage downloaded image files, so a relative `assets/<file>` image path will not resolve. Track each image by its source URL plus a short purpose label for the provenance note.

Legacy fallback (only when `search-assets` is unavailable or a channel returns no results after retries):

- Call `research-execute-web-search` with a search-page URL to find candidate page URLs.
- Call `scrape-webpage-with-apify` on the photo page to extract the absolute full-resolution image URL.
- Use `fetch-url-content` as a lightweight alternative when you only need the page HTML / metadata.

Reliability boundary:

- Stop after two external candidate pages fail to produce a usable absolute image URL or sufficient resolution.
- If external sourcing stalls, ask the user whether to switch to AI generation, user-uploaded images, or a typography-only composition.
- Do not keep looping web search, and do not silently fall back from the user's chosen source mode to a different mode.

## Attribution & disclosure

Always record provenance even if the user later declines visible attribution.

**1. Track each image's source** as `purpose ← source URL`, one line per image:

```text
hero-mountain ← https://unsplash.com/photos/abc123
ui-pulse-card ← https://www.pexels.com/photo/xyz-456789/
```

Keep this list so you can surface it to the user (next step) and repeat it in the final delivery summary, letting the human author audit the deck later.

**2. Surface the sources to the user before finalizing**:

```text
我从 <site> 取了这些图：
- hero-mountain — <url>
- ui-pulse-card — <url>

⚠️ 版权未经核实。请你判断是否可用。
是否需要在图文中标注来源？
- 要：我把 "Photo · <site> · @<author>" 加到对应页脚 / 角标。
- 不要：原样使用，不加注释。
```

**3. Apply the user's choice consistently**:

- 标注 → add a small `mono` caption (Swiss: `.t-meta` 18-20px in corner; Editorial: `.label` next to the image well). Never crowd the caption into the layout's focal area.
- 不标注 → proceed silently. Provenance still goes in the delivery summary.
- If an image is one of many in a composite (e.g. one of nine photos in a matrix), the user may reasonably skip attribution. Do not force a credit label that breaks the layout.

**4. Mention the attribution decision** in the final delivery summary alongside the source URL and site name.

## Generated image policy

This is the **C** branch of Step 1's 三选一 gate. Once the user picks AI generation, lock it — do not re-ask for A or B across subsequent turns.

- Call `generate-image` (9 models available) only where the bitmap **adds editorial value**, usually 1-2 pages out of a 5-9 page set.
- Generate images that match the page's visual role, not generic decoration.
- **Never embed inside the generated image**: titles, page numbers, logos, watermarks, fake UI labels, fake browser chrome. Composition layers (text, frames, captions) must be added in HTML/CSS over the raw bitmap.
- Prompt should be short and role-specific; reference `references/content-planning.md` for the prompt patterns used for Editorial vs Swiss generated images.
- Reference the generated image by the **absolute media URL** `generate-image` returns, placing it in the HTML as `<img src="https://…">` the same way as web-sourced images — never a relative `assets/` path.

## Cross-references

- Full text-on-image qualification & subject-mapping rules: `references/image-overlay.md`.
- Crop discipline (`object-position` per `<img>`): `references/components.md`.
- Editorial / Swiss screenshot framing: `references/screenshot-treatment.md`.
- The intake-time 三选一 gate copy and one-shot rule: `SKILL.md` Step 1.
