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
import { renderToStaticMarkup } from 'react-dom/server'
import type { LinkNode } from 'stream-markdown-parser'
import { describe, expect, test, vi } from 'vitest'

import { renderLink } from '@/components/ai-elements/response-renderer-inline'
import { Source } from '@/components/ai-elements/sources'
import { listWaffoPancakeCatalog } from '@/features/system-settings/integrations/waffo-pancake-api'
import { api } from '@/lib/api'
import {
  isSafeExternalHttpUrl,
  openSafeExternalUrl,
  sanitizeExternalHttpUrl,
  sanitizeSafeHref,
} from '@/lib/safe-url'

describe('safe-url helpers', () => {
  test('sanitizeSafeHref allows safe link schemes, fragments, and relative paths', () => {
    expect(sanitizeSafeHref('https://docs.omnimux.ai/guide')).toBe(
      'https://docs.omnimux.ai/guide'
    )
    expect(sanitizeSafeHref('http://localhost:3000/test')).toBe(
      'http://localhost:3000/test'
    )
    expect(sanitizeSafeHref('mailto:security@omnimux.ai')).toBe(
      'mailto:security@omnimux.ai'
    )
    expect(sanitizeSafeHref('#section-1')).toBe('#section-1')
    expect(sanitizeSafeHref('/console/wallet?tab=topup')).toBe(
      '/console/wallet?tab=topup'
    )
  })

  test('sanitizeSafeHref rejects dangerous schemes, protocol-relative paths, and control characters', () => {
    const unsafe: unknown[] = [
      undefined,
      null,
      '',
      '   ',
      'javascript:alert(1)',
      'JAVASCRIPT:alert(1)',
      'vbscript:msgbox(1)',
      'data:text/html,<script>alert(1)</script>',
      'file:///etc/passwd',
      '//evil.example/path',
      '/\\evil.example/path',
      'https://example.com/\r\nSet-Cookie:a=1',
      'https://example.com/\t/path',
    ]

    for (const candidate of unsafe) {
      expect(sanitizeSafeHref(candidate)).toBeNull()
    }
  })

  test('sanitizeExternalHttpUrl and isSafeExternalHttpUrl enforce http(s) only', () => {
    expect(sanitizeExternalHttpUrl(' https://checkout.stripe.com/pay ')).toBe(
      'https://checkout.stripe.com/pay'
    )
    expect(isSafeExternalHttpUrl('http://pay.example.com/ok')).toBe(true)
    expect(isSafeExternalHttpUrl('mailto:pay@example.com')).toBe(false)
    expect(isSafeExternalHttpUrl('/relative/pay')).toBe(false)
    expect(isSafeExternalHttpUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeExternalHttpUrl('https://pay.example.com/\u0000')).toBe(false)
  })

  test('openSafeExternalUrl enforces protocol check and opener isolation', () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    expect(openSafeExternalUrl('javascript:alert(1)')).toBe(false)
    expect(openSpy).not.toHaveBeenCalled()

    expect(openSafeExternalUrl('https://checkout.stripe.com/c/pay/cs_1')).toBe(
      true
    )
    expect(openSpy).toHaveBeenCalledWith(
      'https://checkout.stripe.com/c/pay/cs_1',
      '_blank',
      'noopener,noreferrer'
    )
  })
})

describe('AI element link sanitization', () => {
  test('renderLink downgrades unsafe protocols to inert span and keeps safe links with noopener noreferrer', () => {
    const unsafeNode: LinkNode = {
      type: 'link',
      href: 'javascript:alert(1)',
      text: 'Click here',
      title: null,
      children: [{ type: 'text', content: 'Click here', raw: 'Click here' }],
      raw: '[Click here](javascript:alert(1))',
    }
    const unsafeHtml = renderToStaticMarkup(
      <>{renderLink(unsafeNode, 'k1', () => 'Click here')}</>
    )
    expect(unsafeHtml).toBe('<span>Click here</span>')

    const safeNode: LinkNode = {
      type: 'link',
      href: 'https://docs.omnimux.ai/api',
      text: 'Docs',
      title: null,
      children: [{ type: 'text', content: 'Docs', raw: 'Docs' }],
      raw: '[Docs](https://docs.omnimux.ai/api)',
    }
    const safeHtml = renderToStaticMarkup(
      <>{renderLink(safeNode, 'k2', () => 'Docs')}</>
    )
    expect(safeHtml).toContain('href="https://docs.omnimux.ai/api"')
    expect(safeHtml).toContain('rel="noopener noreferrer"')
    expect(safeHtml).toContain('target="_blank"')
  })

  test('Source downgrades unsafe href to inert span and enforces noopener noreferrer on safe links', () => {
    const unsafeHtml = renderToStaticMarkup(
      <Source href='javascript:alert(1)' title='Malicious Source' />
    )
    expect(unsafeHtml).not.toContain('<a')
    expect(unsafeHtml).toContain('Malicious Source')

    const safeHtml = renderToStaticMarkup(
      <Source href='https://docs.omnimux.ai/ref' title='Safe Source' />
    )
    expect(safeHtml).toContain('<a')
    expect(safeHtml).toContain('href="https://docs.omnimux.ai/ref"')
    expect(safeHtml).toContain('rel="noopener noreferrer"')
    expect(safeHtml).toContain('target="_blank"')
  })
})

describe('Waffo Pancake catalog transport alignment', () => {
  test('listWaffoPancakeCatalog posts credentials in JSON body instead of query string', async () => {
    const getSpy = vi.spyOn(api, 'get')
    const postSpy = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { data: { stores: [] } } })

    const result = await listWaffoPancakeCatalog('mch_123', 'PRIVATE_KEY_DATA')

    expect(result).toEqual({ data: { stores: [] } })
    expect(getSpy).not.toHaveBeenCalled()
    expect(postSpy).toHaveBeenCalledWith('/api/option/waffo-pancake/catalog', {
      merchant_id: 'mch_123',
      private_key: 'PRIVATE_KEY_DATA',
    })
  })
})
