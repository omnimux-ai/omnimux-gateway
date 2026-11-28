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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { LandingLangProvider } from '../../i18n'
import { ToolForm } from '../components/tool-form'
import { TranscriptInspirationStrip } from '../components/transcript-inspiration-strip'

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
const toastError = vi.hoisted(() => vi.fn())
const turnstile = vi.hoisted(() => ({
  verify: undefined as undefined | ((token: string) => void),
}))
const authState = vi.hoisted(() => ({
  user: null as { id: number } | null,
}))

vi.mock('@/lib/api', () => ({ api }))
vi.mock('sonner', () => ({ toast: { error: toastError } }))
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: (props: {
    to: string
    children: React.ReactNode
    className?: string
  }) => (
    <a href={props.to} className={props.className}>
      {props.children}
    </a>
  ),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({ auth: { user: authState.user } }),
}))
vi.mock('@/components/turnstile', () => ({
  Turnstile: (props: { onVerify: (token: string) => void }) => {
    turnstile.verify = props.onVerify
    return <div data-testid='turnstile' />
  },
}))

const VIDEO_URL =
  'https://www.tiktok.com/@mythicshadowws/video/7470610564927032609'

function config(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      success: true,
      data: {
        enabled: true,
        turnstile_site_key: 'site',
        pass_valid: true,
        quota_exhausted: false,
        logged_in: false,
        ...overrides,
      },
    },
  }
}

const videoResult = {
  data: {
    success: true,
    data: {
      kind: 'video',
      id: '7470610564927032609',
      author: { unique_id: 'mythicshadowws', nickname: 'M' },
      description: 'POV',
      created_at: 1739404800,
      cover_url: '/api/tools/tiktok/file/image?u=c',
      video: {
        duration_ms: 15000,
        width: 720,
        height: 1280,
        preview_url: '/api/tools/tiktok/file/video?u=v',
        download_url: '/api/tools/tiktok/file/video?u=v&dl=1',
      },
      quota_exhausted: false,
    },
  },
}

function rejectWith(status: number, code: string) {
  return Promise.reject({
    response: { status, data: { success: false, code } },
  })
}

function renderForm() {
  return render(
    <LandingLangProvider lang='en'>
      <ToolForm kind='video' locale='en' />
    </LandingLangProvider>
  )
}

function submit(url: string) {
  fireEvent.change(screen.getByRole('textbox'), { target: { value: url } })
  fireEvent.click(screen.getByRole('button', { name: /parse/i }))
}

describe('TikTok tool resolve flow', () => {
  beforeEach(() => {
    turnstile.verify = undefined
    authState.user = null
  })
  afterEach(() => {
    // reset, not clear: a leftover implementation would make the next case
    // depend on this one's server answer.
    vi.resetAllMocks()
  })

  it('renders the video result with an attachment download link when the pass is valid', async () => {
    api.get.mockResolvedValue(config())
    api.post.mockResolvedValue(videoResult)
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit(VIDEO_URL)

    const download = await screen.findByRole('link', { name: /download mp4/i })
    expect(download).toHaveAttribute(
      'href',
      '/api/tools/tiktok/file/video?u=v&dl=1'
    )
    expect(api.post).toHaveBeenCalledWith(
      '/api/tools/tiktok/resolve',
      { url: VIDEO_URL, kind: 'video' },
      expect.anything()
    )
    expect(screen.queryByText(/\d+\/\d+/)).not.toBeInTheDocument()
    expect(screen.queryByText(/today/i)).not.toBeInTheDocument()
  })

  it('shows Turnstile on verification_required and retries resolve after the token is verified', async () => {
    api.get.mockResolvedValue(config({ pass_valid: true }))
    api.post.mockImplementation((path: string) => {
      if (path.endsWith('/verify')) {
        return Promise.resolve({
          data: { success: true, data: { pass_valid: true } },
        })
      }
      return api.post.mock.calls.filter((call) =>
        String(call[0]).endsWith('/resolve')
      ).length === 1
        ? rejectWith(403, 'verification_required')
        : Promise.resolve(videoResult)
    })
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit(VIDEO_URL)
    await screen.findByTestId('turnstile')
    turnstile.verify?.('cf-token')

    expect(
      await screen.findByRole('link', { name: /download mp4/i })
    ).toBeInTheDocument()
    expect(api.post).toHaveBeenCalledWith(
      '/api/tools/tiktok/verify',
      { token: 'cf-token' },
      expect.anything()
    )
    expect(toastError).not.toHaveBeenCalled()
  })

  it('marks the input invalid and toasts once for a non-TikTok link', async () => {
    api.get.mockResolvedValue(config())
    api.post.mockImplementation(() => rejectWith(400, 'invalid_link'))
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit('https://www.youtube.com/watch?v=abc')

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })

  it('disables Parse with a reason when today’s quota is used up', async () => {
    api.get.mockResolvedValue(config({ quota_exhausted: true }))
    renderForm()

    const parse = await screen.findByRole('button', { name: /parse/i })
    await waitFor(() => expect(parse).toHaveAttribute('aria-disabled', 'true'))
    expect(parse.getAttribute('title')).toBeTruthy()
    expect(api.post).not.toHaveBeenCalled()
  })

  it('disables Parse after resolve reports quota_exhausted without showing counts', async () => {
    api.get.mockResolvedValue(config({ quota_exhausted: false }))
    api.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          ...videoResult.data.data,
          quota_exhausted: true,
        },
      },
    })
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit(VIDEO_URL)

    expect(
      await screen.findByRole('link', { name: /download mp4/i })
    ).toBeInTheDocument()
    const parse = screen.getByRole('button', { name: /parse/i })
    await waitFor(() => expect(parse).toHaveAttribute('aria-disabled', 'true'))
    expect(parse.getAttribute('title')).toMatch(/used up/i)
    expect(document.querySelector('#tool-quota')).toBeNull()
    expect(screen.queryByText(/\d+\/\d+/)).not.toBeInTheDocument()
  })

  it('keeps a closed tool focusable and explains it on click', async () => {
    api.get.mockResolvedValue(config({ enabled: false }))
    renderForm()

    const parse = await screen.findByRole('button', {
      name: /temporarily unavailable/i,
    })
    expect(parse).toHaveAttribute('aria-disabled', 'true')
    expect(parse).not.toBeDisabled()
    expect(parse.getAttribute('title')).toBeTruthy()

    fireEvent.click(parse)

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(api.post).not.toHaveBeenCalled()
  })

  it('closes the tool after the server reports it is switched off', async () => {
    api.get.mockResolvedValue(config({ pass_valid: true }))
    api.post.mockImplementation(() => rejectWith(503, 'unavailable_service'))
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit(VIDEO_URL)

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(toastError.mock.calls[0][0]).toMatch(/not open/i)
    const parse = screen.getByRole('button', {
      name: /temporarily unavailable/i,
    })
    expect(parse).toHaveAttribute('aria-disabled', 'true')
  })

  it('fails closed with turnstile_unavailable when no site key is configured', async () => {
    api.get.mockResolvedValue(
      config({ pass_valid: false, turnstile_site_key: '' })
    )
    renderForm()
    await act(async () => {
      await Promise.resolve()
    })

    submit(VIDEO_URL)

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(screen.queryByTestId('turnstile')).not.toBeInTheDocument()
    expect(api.post).not.toHaveBeenCalled()
  })

  it('reports a plain failure instead of blaming verification when config never loads', async () => {
    api.get.mockRejectedValue(new Error('offline'))
    renderForm()
    await screen.findByRole('button', { name: /parse/i })

    submit(VIDEO_URL)

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(toastError.mock.calls[0][0]).toMatch(/couldn't parse/i)
    expect(screen.queryByTestId('turnstile')).not.toBeInTheDocument()
  })

  it('extracts native captions, switches tracks, toggles timestamps, searches, copies, and exports TXT/SRT/VTT', async () => {
    api.get.mockResolvedValue(config({ pass_valid: true }))
    api.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          kind: 'transcript',
          id: '7470610564927032609',
          author: { unique_id: 'mythicshadowws', nickname: 'M' },
          description: 'Morning skincare demo desc',
          created_at: 1739404800,
          cover_url: '/api/tools/tiktok/file/image?u=c',
          video: {
            duration_ms: 15000,
            width: 720,
            height: 1280,
            preview_url: '/api/tools/tiktok/file/video?u=v',
            download_url: '/api/tools/tiktok/file/video?u=v&dl=1',
          },
          transcript: {
            has_captions: true,
            tracks: [
              {
                id: 'en-auto',
                lang: 'eng-US',
                language_code: 'en',
                is_auto_generated: true,
                cues: [
                  { start_ms: 0, end_ms: 3200, text: 'First hook line' },
                  { start_ms: 3200, end_ms: 7000, text: 'Second step serum' },
                ],
              },
              {
                id: 'es',
                lang: 'spa-ES',
                language_code: 'es',
                is_auto_generated: false,
                cues: [{ start_ms: 0, end_ms: 3200, text: 'Primera linea' }],
              },
            ],
          },
          quota_exhausted: false,
        },
      },
    })

    const writeText = vi.fn().mockResolvedValue(undefined)
    const origClipboard = Object.getOwnPropertyDescriptor(
      navigator,
      'clipboard'
    )
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    const origCreateObjectURL = URL.createObjectURL
    const origRevokeObjectURL = URL.revokeObjectURL
    const exportedContents: string[] = []
    URL.createObjectURL = vi.fn((blob: Blob) => {
      void blob.text().then((text) => exportedContents.push(text))
      return 'blob:mock-transcript'
    }) as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn()

    try {
      render(
        <LandingLangProvider lang='en'>
          <ToolForm kind='transcript' locale='en' />
        </LandingLangProvider>
      )
      await act(async () => {
        await Promise.resolve()
      })
      fireEvent.change(screen.getByRole('textbox'), {
        target: { value: VIDEO_URL },
      })
      fireEvent.click(screen.getByRole('button', { name: /get transcript/i }))

      await screen.findByText('First hook line')
      expect(api.post).toHaveBeenCalledWith(
        '/api/tools/tiktok/resolve',
        expect.objectContaining({ kind: 'transcript', url: VIDEO_URL }),
        expect.anything()
      )

      const videoEl = screen.getByLabelText(
        '@mythicshadowws'
      ) as HTMLVideoElement
      expect(videoEl.tagName).toBe('VIDEO')
      const playMock = vi.fn().mockResolvedValue(undefined)
      videoEl.play = playMock
      fireEvent.click(screen.getByText('Second step serum'))
      expect(videoEl.currentTime).toBeCloseTo(3.2, 3)

      fireEvent.error(videoEl)
      const coverImg = screen.getByRole('img', { name: '@mythicshadowws' })
      fireEvent.error(coverImg)
      expect(
        screen.queryByRole('img', { name: '@mythicshadowws' })
      ).not.toBeInTheDocument()
      expect(document.querySelector('.tx-cover-empty')).toBeInTheDocument()

      const esTrack = screen.getByRole('button', { name: /spanish/i })
      expect(esTrack).toHaveAttribute('aria-pressed', 'false')
      fireEvent.click(esTrack)
      expect(esTrack).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByText('Primera linea')).toBeInTheDocument()

      const enTrack = screen.getByRole('button', { name: /english/i })
      fireEvent.click(enTrack)
      expect(screen.getByText('First hook line')).toBeInTheDocument()

      const plainBtn = screen.getByRole('button', { name: /plain/i })
      fireEvent.click(plainBtn)
      expect(
        screen.getByText('First hook line Second step serum')
      ).toBeInTheDocument()

      const timeBtn = screen.getByRole('button', { name: /timestamp/i })
      fireEvent.click(timeBtn)

      const searchInput = screen.getByRole('searchbox')
      fireEvent.change(searchInput, { target: { value: 'serum' } })
      expect(screen.queryByText('First hook line')).not.toBeInTheDocument()
      expect(
        screen.getByText('serum', { selector: 'mark' })
      ).toBeInTheDocument()

      fireEvent.change(searchInput, { target: { value: '' } })
      const copyButtons = screen.getAllByRole('button', {
        name: /^copy text$/i,
      })
      const mainCopyButton = copyButtons.at(-1)
      expect(mainCopyButton).toBeDefined()
      if (mainCopyButton) fireEvent.click(mainCopyButton)
      expect(writeText).toHaveBeenCalledWith(
        '[00:00] First hook line\n[00:03] Second step serum'
      )
      expect(
        await screen.findByRole('button', { name: /^copied$/i })
      ).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: /download txt/i }))
      fireEvent.click(screen.getByRole('button', { name: /download srt/i }))
      fireEvent.click(screen.getByRole('button', { name: /download vtt/i }))

      await waitFor(() => expect(exportedContents).toHaveLength(3))
      expect(exportedContents[0]).toBe(
        '[00:00] First hook line\n[00:03] Second step serum'
      )
      expect(exportedContents[1]).toContain(
        '1\n00:00:00,000 --> 00:00:03,200\nFirst hook line\n'
      )
      expect(exportedContents[2]).toMatch(
        /^WEBVTT\n\n00:00:00\.000 --> 00:00:03\.200\nFirst hook line\n/
      )
    } finally {
      URL.createObjectURL = origCreateObjectURL
      URL.revokeObjectURL = origRevokeObjectURL
      if (origClipboard) {
        Object.defineProperty(navigator, 'clipboard', origClipboard)
      } else {
        delete (navigator as { clipboard?: unknown }).clipboard
      }
    }
  })

  it('shows honest no-captions fallback when TikTok post has no caption tracks', async () => {
    api.get.mockResolvedValue(config({ pass_valid: true }))
    api.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          kind: 'transcript',
          id: '7470610564927032609',
          author: { unique_id: 'mythicshadowws', nickname: 'M' },
          description: 'Morning skincare demo desc',
          created_at: 1739404800,
          cover_url: '/api/tools/tiktok/file/image?u=c',
          video: {
            duration_ms: 15000,
            width: 720,
            height: 1280,
            preview_url: '/api/tools/tiktok/file/video?u=v',
            download_url: '/api/tools/tiktok/file/video?u=v&dl=1',
          },
          transcript: {
            has_captions: false,
            tracks: [],
          },
          quota_exhausted: false,
        },
      },
    })

    render(
      <LandingLangProvider lang='en'>
        <ToolForm kind='transcript' locale='en' />
      </LandingLangProvider>
    )
    await act(async () => {
      await Promise.resolve()
    })
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: VIDEO_URL },
    })
    fireEvent.click(screen.getByRole('button', { name: /get transcript/i }))

    await screen.findByRole('link', { name: /download mp4/i })
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(
      screen.getAllByText('Morning skincare demo desc').length
    ).toBeGreaterThanOrEqual(1)
    expect(screen.getByRole('link', { name: /download mp4/i })).toHaveAttribute(
      'href',
      '/api/tools/tiktok/file/video?u=v&dl=1'
    )
  })

  it('offers Photo Downloader toast action when photo URL is submitted on transcript tool', async () => {
    api.get.mockResolvedValue(config({ pass_valid: true }))
    api.post.mockImplementation(() => rejectWith(409, 'wrong_kind_photo'))

    render(
      <LandingLangProvider lang='en'>
        <ToolForm kind='transcript' locale='en' />
      </LandingLangProvider>
    )
    await act(async () => {
      await Promise.resolve()
    })
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: VIDEO_URL },
    })
    fireEvent.click(screen.getByRole('button', { name: /get transcript/i }))

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(toastError.mock.calls[0][1]?.action?.label).toBe(
      'Go to Photo Downloader'
    )
  })

  it('preloads interactive Demo offline with synced tabs and Demo translation without POST calls', async () => {
    api.get.mockResolvedValue(config())

    render(
      <LandingLangProvider lang='en'>
        <ToolForm kind='transcript' locale='en' />
      </LandingLangProvider>
    )
    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText('@omnimux_studio')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Wait until you see how calm my morning skin looks after one week.'
      )
    ).toBeInTheDocument()
    expect(api.post).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: /^translate$/i }))
    expect(
      await screen.findByText(
        'Espera a ver lo calmada que luce mi piel por la mañana tras una semana.'
      )
    ).toBeInTheDocument()
    expect(api.post).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('tab', { name: /^breakdown$/i }))
    expect(
      await screen.findByRole('button', { name: /copy breakdown/i })
    ).toBeInTheDocument()
    expect(api.post).not.toHaveBeenCalled()
  })

  it('blocks anonymous AI translate and breakdown on parsed videos while serving signed-in AI calls', async () => {
    const transcriptPayload = {
      data: {
        success: true,
        data: {
          kind: 'transcript',
          id: '7470610564927032609',
          author: { unique_id: 'mythicshadowws', nickname: 'M' },
          description: 'Morning skincare demo desc',
          created_at: 1739404800,
          cover_url: '/api/tools/tiktok/file/image?u=c',
          video: {
            duration_ms: 15000,
            width: 720,
            height: 1280,
            preview_url: '/api/tools/tiktok/file/video?u=v',
            download_url: '/api/tools/tiktok/file/video?u=v&dl=1',
          },
          transcript: {
            has_captions: true,
            tracks: [
              {
                id: 'en-auto',
                lang: 'eng-US',
                language_code: 'en',
                is_auto_generated: true,
                cues: [{ start_ms: 0, end_ms: 3200, text: 'First hook line' }],
              },
            ],
          },
          quota_exhausted: false,
        },
      },
    }

    api.get.mockResolvedValue(config({ pass_valid: true, logged_in: false }))
    api.post.mockResolvedValueOnce(transcriptPayload)

    const { unmount } = render(
      <LandingLangProvider lang='en'>
        <ToolForm kind='transcript' locale='en' />
      </LandingLangProvider>
    )
    await act(async () => {
      await Promise.resolve()
    })
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: VIDEO_URL },
    })
    fireEvent.click(screen.getByRole('button', { name: /get transcript/i }))
    await screen.findByText('First hook line')

    fireEvent.click(screen.getByRole('button', { name: /^translate$/i }))
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(api.post).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('tab', { name: /^breakdown$/i }))
    expect(
      await screen.findByRole('button', { name: /sign in to analyze/i })
    ).toBeInTheDocument()
    expect(api.post).toHaveBeenCalledTimes(1)
    unmount()

    vi.clearAllMocks()
    authState.user = { id: 42 }
    api.get.mockResolvedValue(config({ pass_valid: true, logged_in: true }))
    api.post.mockImplementation((path: string) => {
      if (path.endsWith('/resolve')) return Promise.resolve(transcriptPayload)
      if (path.endsWith('/translate')) {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              target_lang: 'zh-CN',
              cues: [{ start_ms: 0, end_ms: 3200, text: '第一句开场钩子' }],
            },
          },
        })
      }
      if (path.endsWith('/breakdown')) {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              summary: 'Opening hook plus single benefit.',
              sections: [
                {
                  stage: 'hook',
                  label: 'Opening Contrast',
                  start_ms: 0,
                  end_ms: 3200,
                  quote: 'First hook line',
                  analysis: 'Stops scroll in first 3 seconds.',
                },
              ],
            },
          },
        })
      }
      return Promise.reject(new Error(`Unexpected POST ${path}`))
    })

    render(
      <LandingLangProvider lang='en'>
        <ToolForm kind='transcript' locale='en' />
      </LandingLangProvider>
    )
    await act(async () => {
      await Promise.resolve()
    })
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: VIDEO_URL },
    })
    fireEvent.click(screen.getByRole('button', { name: /get transcript/i }))
    await screen.findByText('First hook line')

    fireEvent.click(screen.getByRole('button', { name: /^translate$/i }))
    expect(await screen.findByText('第一句开场钩子')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: /^breakdown$/i }))
    expect(
      await screen.findByText('Opening hook plus single benefit.')
    ).toBeInTheDocument()
    expect(screen.getByText('Opening Contrast')).toBeInTheDocument()
  })

  it('loads public inspiration examples and sends picked TikTok URL to callback', async () => {
    const onPickUrl = vi.fn()
    api.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [
            {
              id: 7,
              title: 'Peptide Mist Hook Study',
              caption: 'Morning glow test caption',
              coverUrl: '/media/cover.jpg',
              sourceUrl: VIDEO_URL,
              category: 'Skincare',
              duration: 18,
              views: 124000,
              creator: { handle: 'glowlab', name: 'Glow Lab' },
            },
            {
              id: 8,
              title: 'Instagram Reel Sample',
              caption: 'Non-TikTok post should not expose TikTok load button',
              coverUrl: '/media/ig.jpg',
              sourceUrl: 'https://www.instagram.com/reel/abc123/',
              category: 'Beauty',
              duration: 21,
              views: 88000,
              creator: { handle: 'studio', name: 'Studio' },
            },
          ],
        },
      },
    })

    render(
      <LandingLangProvider lang='en'>
        <TranscriptInspirationStrip onPickUrl={onPickUrl} />
      </LandingLangProvider>
    )

    const loadBtns = await screen.findAllByRole('button', {
      name: /load link/i,
    })
    expect(loadBtns).toHaveLength(1)
    expect(screen.getByText('Peptide Mist Hook Study')).toBeInTheDocument()
    expect(screen.getByText('Instagram Reel Sample')).toBeInTheDocument()
    fireEvent.click(loadBtns[0])
    expect(onPickUrl).toHaveBeenCalledWith(VIDEO_URL)
  })
})
