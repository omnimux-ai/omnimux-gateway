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
import type { IconName } from '../components/cn/data'

export type LandingLang = 'zh-CN' | 'zh-TW' | 'en' | 'ja' | 'id' | 'es'

export type LandingLangOption = {
  code: LandingLang
  /** Native name shown in the switcher menu. */
  label: string
  /** Compact code shown on the closed switcher trigger. */
  short: string
  /** Value applied to <html lang>. */
  htmlLang: string
}

export type ModeCopy = {
  /** Line shown next to the send button (model hint); omitted when the mode picks its own tool. */
  model?: string
  prompts: readonly string[]
}

export type ReelItem = { title: string; tag: string }

export type TrendingCopy = {
  who: string
  views: string
  length: string
  title: string
  likes: string
  saves: string
}

export type PostCopy = {
  title: string
  platformLabel: string
  account: string
  time: string
  state: string
}

export type FlowStepCopy = {
  kind: string
  title: string
  desc: string
  chips?: readonly string[]
  primary: string
  secondary?: string
}

export type ToolGroupCopy = {
  title: string
  lead: string
  items: readonly { icon: IconName; label: string }[]
}

export type ModelCopy = { mark: string; name: string; kind: string }

export type FooterColumnCopy = {
  title: string
  links: readonly { label: string; href: string; external?: boolean }[]
}

/**
 * Interface strings of the TikTok downloader tool pages. Indexable page text
 * (H1, steps, FAQ) lives in `tools/content.ts` instead, shared with the
 * static SEO HTML.
 */
export type ToolPageCopy = {
  crumbAria: string
  crumbTools: string
  kindAria: string
  kindVideo: string
  kindPhoto: string
  inputLabel: string
  placeholderVideo: string
  placeholderPhoto: string
  paste: string
  parse: string
  parsing: string
  verifying: string
  /** `{remaining}` and `{limit}` are replaced. */
  quotaAnon: string
  /** `{remaining}` and `{limit}` are replaced. */
  quotaUser: string
  turnstileAria: string
  unavailable: string
  downloadMp4: string
  recreateMine: string
  breakdown: string
  /** `{count}` is replaced. */
  photoCount: string
  /** `{n}` is replaced. */
  imageAlt: string
  /** `{n}` is replaced. */
  downloadImage: string
  downloadZip: string
  zipLocked: string
  recreatePhotos: string
  ctaTitle: string
  ctaBody: string
  ctaStart: string
  ctaRef: string
  ctaMine: string
  relatedAria: string
  errors: {
    invalidLink: string
    wrongKindPhoto: string
    toPhoto: string
    wrongKindVideo: string
    toVideo: string
    unavailable: string
    serviceUnavailable: string
    upstreamFailed: string
    quotaAnon: string
    quotaUser: string
    busy: string
    rateLimited: string
    turnstileFailed: string
    turnstileUnavailable: string
    generic: string
  }
}

/**
 * Every user-visible string on the desktop landing page (the omnimux.ai
 * homepage since 2026-10-27), keyed by
 * section. Each locale file must satisfy this type so a missing key fails
 * typecheck instead of silently shipping English fallback text.
 */
export interface LandingCopy {
  meta: { title: string }
  skip: string

  nav: {
    brand: string
    home: string
    models: string
    tools: string
    ext: string
    pricing: string
    resources: string
    download: string
    login: string
    menuOpen: string
    langTitle: string
    brandAria: string
    navAria: string
    toolsMenu: {
      aria: string
      group: string
      video: string
      photo: string
      agentGroup: string
      cli: string
      skills: string
    }
    resourcesMenu: {
      aria: string
      inspiration: string
      docs: string
    }
    account: {
      aria: string
      dashboard: string
      profile: string
      security: string
      wallet: string
      systemSettings: string
      signOut: string
    }
  }

  signOut: {
    title: string
    body: string
    cancel: string
    confirm: string
    busy: string
    failed: string
    done: string
    close: string
  }

  hero: {
    titleA: string
    titleB: string
    subPre: string
    subHighlight: string
    subPost: string
    tabsAria: string
    tabs: { agent: string; video: string; image: string }
    inputLabel: string
    attachAria: string
    sendAria: string
    dlMac: string
    dlWin: string
    dlMenuAria: string
    ghStar: string
    dlMacArm: string
    dlMacIntel: string
    dlWinX64: string
    skillsLabel: string
    skillsAria: string
    skills: readonly { icon: IconName; label: string; fill: string }[]
    modes: {
      agent: ModeCopy
      video: ModeCopy
      image: ModeCopy
    }
    reelAria: string
    reel: readonly ReelItem[]
  }

  agentDemo: {
    h2: string
    lead: string
    windowAria: string
    windowTitle: string
    windowStatus: string
    replay: string
    replayAria: string
    side: readonly string[]
    me: string
    msgPre: string
    msgProducts: readonly [string, string, string]
    msgPost: string
    linkCard: string
    doneBreakdown: string
    kv: readonly { k: string; v: string }[]
    doneProducts: string
    doneScripts: string
    confirmTitle: string
    confirmNeed: string
    confirmCost: string
    confirmGo: string
    confirmEdit: string
    inputPh: string
    resultsTitle: string
    resultsMeta: string
    generating: string
    vidNames: readonly [string, string, string]
    vidHooks: readonly [string, string, string]
    schedTitle: string
    schedOk: string
    schedPlatform: string
    schedTimes: readonly [string, string, string]
    schedAll: string
  }

  flow: {
    h2: string
    lineAria: string
    steps: readonly FlowStepCopy[]

    discover: {
      title: string
      meta: string
      savesPrefix: string
      items: readonly TrendingCopy[]
    }

    breakdown: {
      tabsAria: string
      tabs: { video: string; account: string; category: string }
      statsViews: string
      statsLikes: string
      why: string
      rows: readonly { label: string; time: string; desc: string }[]
      reuse: string
      chain: readonly [string, string, string]
      accountName: string
      accountMeta: string
      thumbs: readonly string[]
      metrics: readonly { label: string; value: string }[]
      insights: readonly { k: string; v: string }[]
      accountCta: string
      catTitle: string
      catMeta: string
      bars: readonly { label: string; value: string }[]
      catInsights: readonly { k: string; v: string }[]
      catCta: string
    }

    make: {
      tabsAria: string
      tabs: { generate: string; replicate: string }
      capProduct: string
      capResult: string
      capRef: string
      capMine: string
      badgeGenerated: string
      badgeRef: string
      arrowReplicateA: string
      arrowReplicateB: string
      promptLabel: string
      promptGenerate: string
      intentLabel: string
      promptReplicate: string
      altProduct: string
      altRef: string
      altCan: string
    }

    publish: {
      title: string
      meta: string
      posts: readonly PostCopy[]
      feats: readonly { title: string; desc: string }[]
    }

    review: {
      title: string
      synced: string
      meta: string
      kpis: readonly { label: string; value: string; delta: string }[]
      days: readonly string[]
      splitTitle: string
      splitLegs: readonly [string, string]
      note: string
      cta: string
    }
  }

  create: {
    tag: string
    h2: string
    lead: string
    msgProductCard: string
    msgAsk: string
    doneAnalyze: string
    donePlan: string
    doneConsistent: string
    confirmTitle: string
    confirmNeed: string
    paramCount: string
    confirmGo: string
    confirmEdit: string
    doneResult: string
    tabsAria: string
    tabs: {
      canvas: string
      assets: string
      inspiration: string
      products: string
    }
    groupLabel: string
    setCaptions: readonly string[]
    libAssets: readonly { label: string; sub: string }[]
    libInspiration: readonly { label: string; sub: string }[]
    libProducts: readonly { name: string; sub: string; count: string }[]
    capAgentTitle: string
    capAgentDesc: string
    capAgentCta: string
    capCanvasTitle: string
    capCanvasDesc: string
    capCanvasCta: string
    canvasAria: string
    nodeImage: string
    nodeText: string
    nodeTextBody: string
    nodeAIImage: string
    nodeAIVideo: string
    nodeCut: string
    nodeDone: string
    nodeCutBodyA: string
    nodeCutBodyB: string
  }

  surfaces: {
    h2: string
    lead: string
    desktop: {
      title: string
      desc: string
      input: string
      cta: string
    }
    cli: {
      title: string
      desc: string
      cta: string
    }
    ext: {
      title: string
      desc: string
      capSave: string
      capScan: string
      toast: string
      cta: string
    }
  }

  tools: {
    h2: string
    lead: string
    centerLabel: string
    groups: readonly ToolGroupCopy[]
    modelsTitle: string
    modelsAria: string
    models: readonly ModelCopy[]
    chipsAria: string
    capabilityRows: readonly { icon: IconName; label: string }[][]
  }

  workspace: {
    h2: string
    lead: string
    list: readonly { name: string; desc: string }[]
    note: string
    tableTitle: string
    addProduct: string
    cols: readonly [string, string, string]
    rows: readonly { name: string; category: string; points: string }[]
  }

  proof: {
    h2: string
    lead: string
    facts: readonly { value: string; label: string }[]
    platformsAria: string
    platforms: readonly string[]
    badges: readonly string[]
  }

  faq: {
    h2: string
    lead: string
    items: readonly { q: string; a: string }[]
  }

  finalCta: {
    h2: string
    lead: string
    dlMac: string
    dlWin: string
    note: string
  }

  download: {
    title: string
    desc: string
    close: string
    macTitle: string
    macSub: string
    winTitle: string
    winSub: string
    recommended: string
    notePre: string
    noteMore: string
    noteRun: string
    notePost: string
  }

  footer: {
    brand: string
    tagline: string
    columns: readonly FooterColumnCopy[]
    powered: string
    source: string
  }

  toolPage: ToolPageCopy
}
