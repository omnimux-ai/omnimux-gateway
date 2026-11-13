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
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { LandingLangProvider } from '../../i18n'
import { ToolPhotoResult, ToolVideoResult } from '../components/tool-result'
import type { ToolResult } from '../lib/api'

// Breakdown is launched in this file only.
vi.mock('../lib/workbench-entry', () => ({ TOOL_BREAKDOWN_ENABLED: true }))

// The video entries also ask for a handoff; keep that off the network here
// (workbench-handoff.test.tsx covers it).
vi.mock('@/features/influencer/lib/tool-handoff', () => ({
  issueToolHandoff: vi.fn(() => Promise.reject(new Error('offline'))),
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  useRouter: () => ({}),
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

const base = {
  id: '7470610564927032609',
  author: { unique_id: 'mythicshadowws', nickname: 'M' },
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

function inLanding(node: React.ReactNode) {
  return render(<LandingLangProvider lang='en'>{node}</LandingLangProvider>)
}

describe('tool page breakdown buttons once breakdown is launched', () => {
  it('shows the video breakdown button next to Recreate with my character', () => {
    inLanding(<ToolVideoResult result={videoResult} />)

    expect(
      screen.getByRole('link', { name: /recreate with my character/i })
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /break down/i })).toHaveAttribute(
      'href',
      '/influencer'
    )
  })

  it('shows the photo breakdown button next to Recreate this photo set', () => {
    inLanding(<ToolPhotoResult result={photoResult} loggedIn />)

    expect(
      screen.getByRole('link', { name: /recreate this photo set/i })
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /break down/i })).toHaveAttribute(
      'href',
      '/influencer'
    )
  })
})
