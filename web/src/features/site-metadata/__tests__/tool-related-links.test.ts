import { describe, expect, it } from 'vitest'

import {
  TOOL_PAGES,
  type ToolKind,
  type ToolLocale,
} from '../../omnimux-landing/desktop/tools/content'

const TOOL_KINDS: ToolKind[] = ['video', 'photo']
const TOOL_LOCALES: ToolLocale[] = ['en', 'zh-CN']

describe('tool page related links', () => {
  it('does not point related links at /influencer', () => {
    for (const locale of TOOL_LOCALES) {
      for (const kind of TOOL_KINDS) {
        const hrefs = TOOL_PAGES[locale][kind].related.map((link) => link.href)
        expect(hrefs, `${locale} ${kind}`).not.toContain('/influencer')
      }
    }
  })

  it('keeps related-link hrefs unique on each page', () => {
    for (const locale of TOOL_LOCALES) {
      for (const kind of TOOL_KINDS) {
        const hrefs = TOOL_PAGES[locale][kind].related.map((link) => link.href)
        expect(hrefs, `${locale} ${kind}`).toEqual([...new Set(hrefs)])
      }
    }
  })

  it('labels the studio related link to match the landing page', () => {
    for (const kind of TOOL_KINDS) {
      expect(
        TOOL_PAGES.en[kind].related.map((link) => [link.label, link.href])
      ).toContainEqual(['AI Influencer Studio', '/ai-influencer-studio'])
      expect(
        TOOL_PAGES['zh-CN'][kind].related.map((link) => [link.label, link.href])
      ).toContainEqual(['AI 网红工作室', '/ai-influencer-studio'])
    }
  })
})
