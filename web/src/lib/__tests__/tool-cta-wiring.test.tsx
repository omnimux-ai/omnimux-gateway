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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { LandingLangProvider } from '@/features/omnimux-landing/desktop/i18n'
import {
  ToolPhotoResult,
  ToolVideoResult,
} from '@/features/omnimux-landing/desktop/tools/components/tool-result'
import {
  TikTokToolPage,
  ToolRecreateBand,
} from '@/features/omnimux-landing/desktop/tools/index'
import type { ToolResult } from '@/features/omnimux-landing/desktop/tools/lib/api'

import {
  FIRST_TOUCH_STORAGE_KEY,
  GROWTH_CAMPAIGN,
  captureFirstTouch,
  resetToolCtaThrottle,
} from '../growth-tracking'

vi.mock('@/features/influencer/lib/tool-handoff', () => ({
  issueToolHandoff: vi.fn(() => Promise.reject(new Error('offline'))),
}))

// The parse form talks to the network and Turnstile; the CTAs do not need it.
vi.mock(
  '@/features/omnimux-landing/desktop/tools/components/tool-form',
  () => ({
    ToolForm: () => null,
  })
)

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  useRouter: () => ({ subscribe: () => () => {} }),
  Link: (props: {
    to: string
    children: React.ReactNode
    className?: string
    onClick?: (event: React.MouseEvent<HTMLAnchorElement>) => void
  }) => (
    <a
      href={props.to}
      className={props.className}
      onClick={(event) => {
        event.preventDefault()
        props.onClick?.(event)
      }}
    >
      {props.children}
    </a>
  ),
}))

const tracker = vi.fn()

const base = {
  id: '7470610564927032609',
  author: { unique_id: 'creator', nickname: 'C' },
  description: '',
  created_at: 1739404800,
  cover_url: '',
  quota: { used: 1, limit: 5 },
}
const videoResult = {
  ...base,
  kind: 'video',
  video: {
    duration_ms: 15000,
    width: 720,
    height: 1280,
    preview_url: '/api/tools/tiktok/file/video?u=v',
    download_url: '/api/tools/tiktok/file/video?u=v&dl=1',
  },
} as unknown as ToolResult
const photoResult = {
  ...base,
  kind: 'photo',
  images: [
    {
      index: 0,
      width: 720,
      height: 960,
      preview_url: '/api/tools/tiktok/file/image?u=a',
      download_url: '/api/tools/tiktok/file/image?u=a&dl=1',
    },
  ],
} as unknown as ToolResult

function inLanding(node: React.ReactNode, lang: 'en' | 'zh-CN' = 'en') {
  return render(<LandingLangProvider lang={lang}>{node}</LandingLangProvider>)
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, enabled: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <TikTokToolPage kind='video' locale='en' />
    </QueryClientProvider>
  )
}

function lastClick() {
  const calls = tracker.mock.calls.filter(([name]) => name === 'tool_cta_click')
  return calls.at(-1)?.[1]
}

beforeEach(() => {
  ;(globalThis as { __OMNIMUX_TRACK__?: unknown }).__OMNIMUX_TRACK__ = tracker
  tracker.mockReset()
  resetToolCtaThrottle()
  window.localStorage.clear()
})

afterEach(() => {
  delete (globalThis as { __OMNIMUX_TRACK__?: unknown }).__OMNIMUX_TRACK__
  window.localStorage.clear()
})

describe('tool page CTAs report tool_cta_click', () => {
  it('video result primary → workbench_recreate / result_primary', () => {
    inLanding(<ToolVideoResult result={videoResult} />, 'zh-CN')
    fireEvent.click(document.querySelector('a.btn-primary') as HTMLElement)
    expect(lastClick()).toMatchObject({
      tool: 'video',
      locale: 'zh-CN',
      cta_id: 'workbench_recreate',
      cta_variant: 'conservative',
      placement: 'result_primary',
    })
  })

  it('photo result primary → workbench_recreate / result_primary', () => {
    inLanding(<ToolPhotoResult result={photoResult} loggedIn />)
    fireEvent.click(document.querySelector('a.btn-primary') as HTMLElement)
    expect(lastClick()).toMatchObject({
      tool: 'photo',
      locale: 'en',
      cta_id: 'workbench_recreate',
      placement: 'result_primary',
    })
  })

  it('recreate band → workbench_recreate and desktop_download / inline_secondary', () => {
    inLanding(<ToolRecreateBand result={null} kind='photo' locale='en' />)
    fireEvent.click(document.querySelector('a.btn-primary') as HTMLElement)
    expect(lastClick()).toMatchObject({
      tool: 'photo',
      cta_id: 'workbench_recreate',
      placement: 'inline_secondary',
    })
    fireEvent.click(document.querySelector('button.btn-ghost') as HTMLElement)
    expect(lastClick()).toMatchObject({
      tool: 'photo',
      cta_id: 'desktop_download',
      placement: 'inline_secondary',
    })
  })

  it('studio related link → studio_fallback / footer; sibling tool link reports nothing', () => {
    renderPage()
    const studio = document.querySelector(
      '.tool-rel a[href="/ai-influencer-studio"]'
    ) as HTMLElement
    fireEvent.click(studio)
    expect(lastClick()).toMatchObject({
      tool: 'video',
      locale: 'en',
      cta_id: 'studio_fallback',
      placement: 'footer',
    })
    tracker.mockReset()
    const sibling = screen.getByText('TikTok Photo Downloader', {
      selector: '.tool-rel a',
    })
    fireEvent.click(sibling)
    expect(lastClick()).toBeUndefined()
  })

  it('adds no utm_* to CTA links and keeps the first touch untouched', () => {
    captureFirstTouch(
      `?utm_campaign=${GROWTH_CAMPAIGN}&utm_source=x&utm_medium=dm`
    )
    const before = window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)
    renderPage()
    for (const link of document.querySelectorAll('a')) {
      expect(link.getAttribute('href') ?? '').not.toContain('utm_')
    }
    fireEvent.click(
      document.querySelector('.tool-cta a.btn-primary') as HTMLElement
    )
    expect(window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)).toBe(before)
    expect(lastClick()).toMatchObject({
      acq_source: 'x',
      acq_medium: 'dm',
      acq_content: '',
    })
  })
})
