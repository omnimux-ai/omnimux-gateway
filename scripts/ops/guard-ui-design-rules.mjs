#!/usr/bin/env node
/**
 * scripts/ops/guard-ui-design-rules.mjs
 * DESIGN.md hard-rule inspection for the OmniMux frontend.
 *
 * Only rules that are (a) literally written as 严禁 / 必须 in DESIGN.md and
 * (b) clean against the current tree are enabled. See docs/ops/ui-design-hook-guard-zh.md
 * for the measured hit count that admitted or rejected each candidate rule.
 */

/**
 * UI surfaces this guard inspects. `className` lives in JSX files, plus the
 * `.ts` modules that sit beside components (variant maps, class helpers).
 * Every other module under web/src — API clients, stores, utils — passes through.
 */
export const UI_FILE_RE =
  /^web\/src\/.*\.(?:tsx|jsx)$|^web\/src\/(?:.+\/)?components\/.*\.ts$/

/** Test, story and generated files never carry product UI. */
export const EXEMPT_PATH_RE =
  /(?:^|\/)(?:__tests__|__mocks__|node_modules|dist|\.tmp)(?:\/|$)|\.(?:test|spec|stories)\.[jt]sx?$/

/** A line may opt out of every rule with `// exempt-ui <reason>`. */
export const EXEMPT_MARKER_RE = /exempt-ui(?:[-\w]*)?(?:\s+\S|$)/

/** DESIGN.md §7 Don'ts 2: neon greens that must never act as brand or beam color. */
const NEON_GREEN_RE = /(?:#C8F135|#D4FF38)/i

/** DESIGN.md §7 Don'ts 7: legacy cold blue tints that must never be used in dark surfaces. */
const DARK_BLUE_TINT_RE = /(?:#151A26|#111520|#14161C|#1A1D25)\b/i

/** DESIGN.md §7 Don'ts 1 + §1.2: violet is reserved for Beam / logo / plan badges, never a solid CTA fill. */
const VIOLET_FILL_RE = /\b(?:bg|fill)-(?:violet|purple|indigo)-(?:500|600|700)\b(?![-\w])/
const VIOLET_GRADIENT_RE = /\b(?:from|via|to)-(?:violet|purple|indigo)-(?:500|600|700)\b(?![-\w])/
const CTA_ELEMENT_RE = /<(?:button|Button|a|Link)\b[^>]*$|\brole\s*=\s*(?:"button"|'button'|{"button"})[^>]*$/s
/** The same rule for hand-written CSS: a button-like selector whose background is a solid violet. */
const CSS_CTA_SELECTOR_RE = /\bbtn\b|\bbutton\b|\bcta\b|-cta\b|primary|\bsend\b|submit/i
const CSS_VIOLET_TOKEN_RE = /var\(--(?:violet|purple|indigo|brand|omx-brand|beam)[\w-]*\)/i
/** Translucent washes, glows and borders in the brand family are not a fill. */
const CSS_VIOLET_WASH_TOKEN_RE = /tint|glow|subtle|shadow|border|wash|--omx-brand-soft/i

/**
 * DESIGN.md §3.2 营销字阶白名单 + §3.3 微字阶下限：公开营销面可用的全部字号。
 * 本数组必须与 DESIGN.md §3.2「公开面字号白名单」逐值一致——改一处必须同时改另一处。
 */
export const TYPE_SCALE = [64, 44, 32, 28, 24, 20, 18, 16, 14, 12, 11]
const ARBITRARY_SIZE_RE = /\btext-\[(\d+(?:\.\d+)?)(px|rem)\]/g
const PUBLIC_SURFACE_RE = /web\/src\/(?:features\/omnimux-landing|components\/public-|features\/inspiration)\//
/** Site chrome (header/footer nav) sits outside §3.2, which governs content typography. */
const CHROME_PATH_RE = /(?:landing|public)-(?:header|footer)\.tsx$/

/** DESIGN.md §3.2/§3.3/§5.2/§5.3/§6 also govern the public marketing CSS. */
export const CSS_FILE_RE = /^web\/src\/(?:features\/omnimux-landing|features\/inspiration)\/.*\.css$/
/** §3.2 营销字阶 ∪ §3.3 微字阶（模拟图与控制台用）。 */
const CSS_TYPE_SCALE = [64, 44, 32, 28, 24, 20, 18, 16, 14, 13, 12, 11, 10]
/** §3.4 字重阶梯。 */
const CSS_WEIGHTS = [400, 500, 600, 700]
/** §5.3 圆角阶梯（0 / 50% / 999px / 9999px 另行放行）。 */
const CSS_RADII = [8, 16, 20, 24, 36]
/** §5.2 间距阶梯。 */
const CSS_GAPS = [2, 4, 8, 12, 16, 24, 32, 40, 48, 64, 80, 100, 120]
const CSS_TOKEN_RE = /var\(|clamp\(|calc\(|inherit|initial|unset|revert/
/** DESIGN.md §7 Don'ts 4: light theme must not wash media cards out to white. */
const MEDIA_CARD_PATH_RE = /media-(?:poster|card|frame)|poster-accent|video-frame|first-last-frame/i
const WHITE_WASH_RE = /\bbg-white\b(?![-\w])/

/** DESIGN.md §4/§5: design-system directories build their own controls. */
const DESIGN_SYSTEM_PATH_RE = /web\/src\/components\/ui\//
const NATIVE_SELECT_RE = /<select\b(?![^>]*className)[^>]*>/

/**
 * A solid violet in the brand family (hue 235–300°). Translucent tints below
 * 50% alpha are badge washes, not a fill, and stay allowed.
 * @param {string} literal `#rgb[a]` / `#rrggbb[aa]` / `rgb[a](…)`
 */
export function isSolidViolet(literal) {
  let r, g, b, a = 1
  const hex = /^#([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i.exec(literal)
  if (hex) {
    const h = hex[1].length <= 4 ? [...hex[1]].map((c) => c + c).join('') : hex[1]
    ;[r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
    if (h.length === 8) a = parseInt(h.slice(6, 8), 16) / 255
  } else {
    const fn = /^rgba?\(([^)]*)\)$/i.exec(literal)
    if (!fn) return false
    const parts = fn[1].split(/[\s,/]+/).filter(Boolean)
    ;[r, g, b] = parts.slice(0, 3).map(Number)
    if (parts[3] !== undefined) a = parts[3].endsWith('%') ? parseFloat(parts[3]) / 100 : Number(parts[3])
  }
  if (![r, g, b, a].every(Number.isFinite) || a < 0.5) return false
  const max = Math.max(r, g, b) / 255
  const min = Math.min(r, g, b) / 255
  const d = max - min
  if (d === 0) return false
  const l = (max + min) / 2
  const s = d / (1 - Math.abs(2 * l - 1))
  const rn = r / 255, gn = g / 255, bn = b / 255
  let hue
  if (max === rn) hue = (((gn - bn) / d) % 6) * 60
  else if (max === gn) hue = ((bn - rn) / d + 2) * 60
  else hue = ((rn - gn) / d + 4) * 60
  if (hue < 0) hue += 360
  return hue >= 235 && hue <= 300 && s >= 0.35 && l >= 0.25 && l <= 0.85
}

function lineOf(text, index) {
  return text.slice(0, index).split('\n').length
}

function isExemptLine(line) {
  return EXEMPT_MARKER_RE.test(line)
}

function push(violations, content, index, rule, message) {
  const line = lineOf(content, index)
  if (isExemptLine(content.split('\n')[line - 1] || '')) return
  violations.push({ rule, line, message })
}

/** Rem, the type scale's rem half (64/16 = 4 is the only rem value in use). */
function remToPx(value, unit) {
  return unit === 'rem' ? Math.round(value * 16) : value
}

/**
 * Inspect one code payload (full file for `write`, the edited span for `edit`).
 * @param {string} code Source text about to be written.
 * @param {string} filePath Repository-relative POSIX path.
 * @returns {{rule: string, line: number, message: string}[]}
 */
export function inspectUICode(code, filePath) {
  const violations = []
  const text = String(code || '')
  if (!text) return violations

  const isPublicSurface = PUBLIC_SURFACE_RE.test(filePath) && !CHROME_PATH_RE.test(filePath)

  // 1. Neon green used as a brand color (class, style object or inline CSS).
  const neon = NEON_GREEN_RE.exec(text)
  if (neon) {
    push(violations, text, neon.index, 'NEON_GREEN', '禁止引入高饱和荧光绿 #C8F135 / #D4FF38 充当品牌色或连线色（DESIGN.md §7 禁忌 2）')
  }

  // 2. Solid violet fill on a CTA. These patterns are not global: one test per
  // pattern, and only a confirmed CTA goes on to locate its position.
  for (const re of [VIOLET_FILL_RE, VIOLET_GRADIENT_RE]) {
    if (!re.test(text)) continue
    const found = re.exec(text)
    if (!found) continue
    const openTag = text.slice(Math.max(0, found.index - 400), found.index)
    if (!CTA_ELEMENT_RE.test(openTag)) continue
    const kind = re === VIOLET_FILL_RE ? '实心紫色填充' : '紫色渐变'
    push(violations, text, found.index, 'VIOLET_CTA', `严禁在主按钮（Primary CTA）上使用${kind} [${found[0]}]；主行动点一律黑白墨水化（DESIGN.md §7 禁忌 1）`)
  }

  // 3. Type scale on public marketing surfaces.
  if (isPublicSurface) {
    ARBITRARY_SIZE_RE.lastIndex = 0
    let m
    const reported = new Set()
    while ((m = ARBITRARY_SIZE_RE.exec(text)) !== null) {
      const px = remToPx(Number(m[1]), m[2])
      if (TYPE_SCALE.includes(px)) continue
      if (reported.has(px)) continue
      reported.add(px)
      push(
        violations,
        text,
        m.index,
        'OFF_TYPE_SCALE',
        `落地页/公开面字号 ${m[0]}（${px}px）不在字阶表内；§3.2 白名单 [${TYPE_SCALE.join(', ')}]px，或改用 §3.3 的 --omx-type-* 角色变量`,
      )
    }
  }

  // 4. Media cards washed to white.
  if (MEDIA_CARD_PATH_RE.test(filePath)) {
    const wash = WHITE_WASH_RE.exec(text)
    if (wash) {
      push(violations, text, wash.index, 'MEDIA_CARD_WHITEWASH', '严禁在浅色主题下将视听生成卡片漂白（bg-white）；媒体卡保持暗房底色 #11141D（DESIGN.md §7 禁忌 4）')
    }
  }

  // 5. Native controls inside the design-system layer.
  if (DESIGN_SYSTEM_PATH_RE.test(filePath)) {
    const native = NATIVE_SELECT_RE.exec(text)
    if (native) {
      push(violations, text, native.index, 'NATIVE_CONTROL', '设计系统层禁止裸用原生 <select>；改用共享 Select/DropdownSelect 组件（DESIGN.md §4 组件规范）')
    }
  }

  // 6. Legacy cold blue tints in dark surfaces (DESIGN.md §7 Don'ts 7).
  const blueTint = DARK_BLUE_TINT_RE.exec(text)
  if (blueTint) {
    push(
      violations,
      text,
      blueTint.index,
      'DARK_BLUE_TINT',
      `严禁使用旧版冷调发蓝杂色 [${blueTint[0]}]；深色模式必须使用纯正中性深石墨灰阶（Neutral Charcoal / Zinc，如 var(--card) / var(--background) / #09090b / #141416）（DESIGN.md §7 禁忌 7）`
    )
  }


  // 7. Violet CTA fills written as plain CSS (DESIGN.md §7 禁忌 1 + §2.2 紫色预算).
  // Comments are blanked to spaces so selectors stay clean and indices still map
  // back to the original lines, where `exempt-ui` whitelists a §2.2 entry.
  if (/\.css$/.test(filePath)) {
    const css = text.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '))
    for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const selector = rule[1].trim()
      if (selector.startsWith('@') || !CSS_CTA_SELECTOR_RE.test(selector)) continue
      const bodyStart = rule.index + rule[0].indexOf('{') + 1
      for (const decl of rule[2].matchAll(/(?<![-\w])background(?:-color|-image)?\s*:\s*([^;]+)/g)) {
        const value = decl[1]
        const found = CSS_VIOLET_TOKEN_RE.exec(value)
        const token = found && !CSS_VIOLET_WASH_TOKEN_RE.test(found[0]) ? found : null
        const literal = token ? null : (value.match(/#[0-9a-f]{3,8}\b|rgba?\([^)]*\)/gi) || []).find(isSolidViolet)
        if (!token && !literal) continue
        push(
          violations,
          text,
          bodyStart + decl.index,
          'VIOLET_CTA_CSS',
          `严禁在按钮类选择器 [${selector.replace(/\s+/g, ' ').slice(0, 60)}] 上使用实心紫色背景 [${token ? token[0] : literal}]；主行动点一律黑白墨水化（DESIGN.md §7 禁忌 1）。§2.2 白名单内的紫色动作键在同行加 exempt-ui 注释`,
        )
      }
    }
  }

  // 6. Public marketing CSS carries the same scale rules (DESIGN.md §3.2/§3.4/§5.2/§5.3/§6).
  if (CSS_FILE_RE.test(filePath)) {
    const decl = (prop, value) => { const i = text.indexOf(prop + ":" + value); return i }
    // 6a. font-size
    for (const m of text.matchAll(/(?<![-\w])font-size:\s*([^;]+);/g)) {
      const v = m[1].trim()
      if (CSS_TOKEN_RE.test(v)) continue
      const px = /^(\d+(?:\.\d+)?)px$/.exec(v)
      if (px && !CSS_TYPE_SCALE.includes(Number(px[1]))) {
        push(violations, text, m.index, "OFF_TYPE_SCALE_CSS", `CSS 字号 ${v} 不在 §3.2/§3.3 白名单 [${CSS_TYPE_SCALE.join(", ")}]px 内`)
      }
    }
    // 6a-2. 畸形 var 残留：var(--x) 紧邻十六进制字符，必是替换脚本的前缀碰撞
    for (const m of text.matchAll(/var\(--[\w-]+\)[0-9a-fA-F]{2}\b/g)) {
      push(violations, text, m.index, "MALFORMED_VAR_CSS", `畸形 CSS 值 ${m[0]}：var() 后残留十六进制字符；多半是「短 hex 是长 hex 前缀」导致的替换碰撞（如 #ffffff 命中 #ffffff66）`)
    }
    // 6b. font-weight
    for (const m of text.matchAll(/(?<![-\w])font-weight:\s*([^;]+);/g)) {
      const v = m[1].trim()
      if (CSS_TOKEN_RE.test(v)) continue
      if (!CSS_WEIGHTS.includes(Number(v))) {
        push(violations, text, m.index, "OFF_WEIGHT_CSS", `CSS 字重 ${v} 不在 §3.4 四档 [400, 500, 600, 700] 内`)
      }
    }
    // 6c. border-radius（逐角校验，0 与 % 放行）
    for (const m of text.matchAll(/(?<![-\w])border-radius:\s*([^;]+);/g)) {
      const v = m[1].trim()
      if (CSS_TOKEN_RE.test(v)) continue
      const parts = v.split(/\s+/).filter(Boolean)
      const bad = parts.some((p) => p !== "0" && !p.endsWith("%") && !["999px", "9999px"].includes(p) && !/(\d+(?:\.\d+)?)px/.exec(p)?.length)
        || parts.some((p) => { const n = /^(\d+(?:\.\d+)?)px$/.exec(p); return n && !["999", "9999"].includes(n[1]) && !CSS_RADII.includes(Number(n[1])) })
      if (bad) push(violations, text, m.index, "OFF_RADIUS_CSS", `CSS 圆角 ${v} 不在 §5.3 阶梯 [8, 16, 20, 24, 36, 999px, 50%] 内`)
    }
    // 6d. gap
    for (const m of text.matchAll(/(?<![-\w])gap:\s*([^;]+);/g)) {
      const v = m[1].trim()
      if (CSS_TOKEN_RE.test(v)) continue
      const bad = v.split(/\s+/).filter(Boolean).some((p) => { const n = /^(\d+(?:\.\d+)?)px$/.exec(p); return n && !CSS_GAPS.includes(Number(n[1])) })
      if (bad) push(violations, text, m.index, "OFF_GAP_CSS", `CSS gap ${v} 不在 §5.2 阶梯 [${CSS_GAPS.join(", ")}]px 内`)
    }
    // 6e. box-shadow 必须是 token 或 none / inset 描边
    for (const m of text.matchAll(/(?<![-\w])box-shadow:\s*([^;]+);/g)) {
      const v = m[1].trim().replace(/\s+/g, " ")
      if (v === "none" || v.startsWith("none ") || v.startsWith("inset") || v.startsWith("var(--shadow-")) continue
      push(violations, text, m.index, "SHADOW_LITERAL_CSS", `CSS 阴影必须用 §6 的 var(--shadow-sm|md|brand)，不要写字面量：${v.slice(0, 60)}`)
    }
  }
  return violations.sort((a, b) => a.line - b.line)
}

/**
 * @param {string} filePath Absolute or repo-relative path.
 * @param {string} repoRoot Absolute repository root (the guard's own checkout).
 * @returns {string} Repository-relative POSIX path, or '' when the target is
 * outside this checkout — a path that merely contains `web/src` belongs to
 * another workspace and must not be inspected by this repo's gate.
 */
export function toRepoRelative(filePath, repoRoot) {
  const normalized = String(filePath || '').replace(/\\/g, '/')
  const root = String(repoRoot || '').replace(/\\/g, '/').replace(/\/+$/, '')
  if (normalized.startsWith('/')) {
    if (root && normalized.startsWith(root + '/')) return normalized.slice(root.length + 1)
    return ''
  }
  return normalized.replace(/^\.\//, '')
}

/**
 * @param {string} relativePath Repository-relative POSIX path.
 * @returns {boolean} Whether the guard inspects this file at all.
 */
export function isTargetUIFile(relativePath) {
  if (EXEMPT_PATH_RE.test(relativePath)) return false
  // 公开营销面 CSS 与 UI 代码同受 DESIGN.md 约束；全站 CSS 同样受核心色彩与禁忌约束。
  return (
    UI_FILE_RE.test(relativePath) ||
    CSS_FILE_RE.test(relativePath) ||
    /^web\/src\/.*\.css$/.test(relativePath)
  )
}
