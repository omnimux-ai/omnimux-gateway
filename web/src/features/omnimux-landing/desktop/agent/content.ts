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
// fork: shared shapes, paths and locale-independent data for the Agent entry
// pages (/cli, /skills and their per-language mirrors).
//
// Copy lives in ./locales/<lang>.ts so one language can be edited without
// touching the others. This module stays plain data: no React/JSX and no '@/…'
// alias imports, the same contract as tools/content.ts.

export type AgentKind = 'cli' | 'skills'

/** The six languages the desktop landing page ships. */
export type AgentLocale = 'en' | 'zh-CN' | 'zh-TW' | 'ja' | 'es' | 'id'

/** The three installation steps share one shape on both pages. */
export interface AgentStep {
  title: string
  /** One-line hint under the command. */
  body: string
  /** Copyable command; rendered in a mono block with a copy button. */
  command?: string
  /** Step 2/3 carry a link instead of a command. */
  action?: { label: string; href: string }
}

/** Waveform bars for the avatar card; static, so it carries no state. */
export const AGENT_VOICE_BARS = [
  { id: 'b1', height: 10 },
  { id: 'b2', height: 18 },
  { id: 'b3', height: 26 },
  { id: 'b4', height: 14 },
  { id: 'b5', height: 22 },
  { id: 'b6', height: 30 },
  { id: 'b7', height: 12 },
  { id: 'b8', height: 20 },
  { id: 'b9', height: 8 },
  { id: 'b10', height: 16 },
  { id: 'b11', height: 24 },
  { id: 'b12', height: 12 },
]

/**
 * A capability card's preview. Each variant is a small DOM mock of the real
 * product surface; `type` selects the renderer in agent-cards.tsx.
 */
export type AgentMock =
  | {
      type: 'checklist'
      rows: { label: string; value: string; accent?: boolean }[]
    }
  | { type: 'prompt'; text: string; icon: 'video' | 'img' }
  | { type: 'voice' }
  | { type: 'chips'; chips: string[] }
  | {
      type: 'publish'
      rows: {
        title: string
        meta: string
        status: string
        tone?: 'ok' | 'busy'
      }[]
      chips: string[]
    }

export interface AgentCard {
  title: string
  body: string
  mock: AgentMock
}

export interface AgentPageContent {
  path: string
  /** SEO <title>. */
  title: string
  /** SEO meta description. */
  description: string
  badge: string
  h1: string
  subtitle: string
  steps: AgentStep[]
  stepLabel: string
  compatTitle: string
  compatLead: string
  /** Column captions under the flow diagram. */
  flowLabels: { agents: string; hub: string; caps: string }
  hubTitle: string
  hubSub: string
  flowCaps: { title: string; body: string }[]
  coreTitle: string
  coreLead: string
  cards: AgentCard[]
  tryTitle: string
  tryHeading: string
  tryBody: string
  tryPrompts: { label: string; text: string }[]
  tryChat: {
    prompt: string
    done: { label: string; value: string }[]
    follow: string
  }
  faqTitle: string
  faq: { q: string; a: string }[]
  ctaTitle: string
  ctaBody: string
  ctaButton: string
  copy: string
  copied: string
  kindAria: string
  kindCli: string
  kindSkills: string
}

/** The Agent names the CLI/Skills support, in the reference page's order. */
export const AGENT_NAMES = [
  'Codex',
  'Claude Code',
  'OpenClaw',
  'Hermes',
  'DouBao',
  'WorkBuddy',
] as const

/**
 * One URL per language, matching the landing page's own convention: English
 * sits at the root, every other language under its own prefix. `zh-CN` keeps
 * `/zh` because the TikTok tool pages already publish it.
 */
export const AGENT_PATHS: Record<AgentLocale, Record<AgentKind, string>> = {
  en: { cli: '/cli', skills: '/skills' },
  'zh-CN': { cli: '/zh/cli', skills: '/zh/skills' },
  'zh-TW': { cli: '/zh-tw/cli', skills: '/zh-tw/skills' },
  ja: { cli: '/ja/cli', skills: '/ja/skills' },
  es: { cli: '/es/cli', skills: '/es/skills' },
  id: { cli: '/id/cli', skills: '/id/skills' },
}

export const AGENT_LOCALES: AgentLocale[] = [
  'en',
  'zh-CN',
  'zh-TW',
  'ja',
  'es',
  'id',
]

/** `hreflang` and Open Graph locale for each page language. */
export const AGENT_LOCALE_TAGS: Record<
  AgentLocale,
  { hreflang: string; og: string }
> = {
  en: { hreflang: 'en', og: 'en_US' },
  'zh-CN': { hreflang: 'zh-Hans', og: 'zh_CN' },
  'zh-TW': { hreflang: 'zh-Hant', og: 'zh_TW' },
  ja: { hreflang: 'ja', og: 'ja_JP' },
  es: { hreflang: 'es', og: 'es_ES' },
  id: { hreflang: 'id', og: 'id_ID' },
}
