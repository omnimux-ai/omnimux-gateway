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
import { startCompletion } from '@codemirror/autocomplete'
import { EditorView } from '@codemirror/view'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { createRef, useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { MotionPanel } from '../components/motion-panel'
import type { MotionForm } from '../hooks/use-motion'
import { motionModeMeta, type VideoValidationSuccess } from '../lib/motion'
import {
  communityTrendPresets,
  exploreToMotionPreset,
  motionExplorePresets,
} from '../lib/motion-explore'

beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
})

const videoMetadata: VideoValidationSuccess = {
  ok: true,
  rawDurationSec: 17,
  durationSec: 17,
  width: 720,
  height: 1280,
  aspectRatio: '9:16',
}

vi.mock('../api', () => ({
  uploadReferenceAsset: vi.fn(),
  fetchMotionOptions: vi.fn().mockResolvedValue(null),
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

/**
 * The panel is a pure view over the form state, so the test supplies the state
 * directly instead of standing up the hook's upload path.
 */
function stubMotion(overrides: Partial<MotionForm> = {}): MotionForm {
  const mode = overrides.mode ?? 'motion'
  const characters =
    overrides.characters ?? (overrides.character ? [overrides.character] : [])
  return {
    mode,
    setMode: vi.fn(),
    meta: motionModeMeta(mode),
    character: characters[0] ?? null,
    characters,
    uploadedCharacters: [],
    uploadedVideos: [],
    characterUploading: false,
    characterUploadProgress: undefined,
    characterUploads: [],
    videoUploadProgress: undefined,
    selectUploadedVideo: vi.fn(),
    setCharacters: vi.fn(),
    addCharacter: vi.fn(),
    removeCharacter: vi.fn(),
    toggleCharacter: vi.fn(),
    characterPickerOpen: false,
    openCharacterPicker: vi.fn(),
    closeCharacterPicker: vi.fn(),
    videoPickerOpen: false,
    openVideoPicker: vi.fn(),
    closeVideoPicker: vi.fn(),
    pickHistoryVideo: vi.fn(),
    videoPreparing: false,
    videoPreparationPhase: null,
    preparingTaskId: undefined,
    cancelVideoSelection: vi.fn(),
    isVideoPreparing: vi.fn(() => false),
    video: null,
    prompt: '',
    setPrompt: vi.fn(),
    promptEnabled: false,
    setPromptEnabled: vi.fn(),
    durationSec: 4,
    videoMetadataLoading: false,
    videoValidation: null,
    resolution: '720p',
    setResolution: vi.fn(),
    outputAspect: '9:16',
    outputCapabilities: undefined,
    setOutputCapabilities: vi.fn(),
    pickCharacter: vi.fn().mockResolvedValue(undefined),
    pickCharacters: vi.fn().mockResolvedValue({
      added: 0,
      failed: 0,
      skipped: 0,
    }),
    retryCharacterUpload: vi.fn().mockResolvedValue({
      added: 0,
      failed: 0,
      skipped: 0,
    }),
    dismissCharacterUpload: vi.fn(),
    isCharacterUploading: vi.fn(() => false),
    pickVideo: vi.fn().mockResolvedValue(videoMetadata),
    setRemoteVideo: vi.fn(),
    style: null,
    applyStyle: vi.fn(),
    effectivePrompt: '',
    recreateImages: [],
    recreateResult: null,
    recreatePrompt: '',
    setRecreatePrompt: vi.fn(),
    recreatePromptRunes: 0,
    recreateIntentPrompt: '',
    setRecreateIntentPrompt: vi.fn(),
    recreateIntentPromptEnabled: false,
    setRecreateIntentPromptEnabled: vi.fn(),
    recreateSubmitDuration: 0,
    videoRawDurationSec: 0,
    recreateQuote: null,
    recreateAnalyzing: false,
    analyzeRecreate: vi.fn().mockResolvedValue(null),
    resetRecreateAnalysis: vi.fn(),
    clearCharacter: vi.fn(),
    clearVideo: vi.fn(),
    reset: vi.fn(),
    restoreTask: vi.fn(),
    canSubmit: false,
    ...overrides,
  }
}

describe('MotionPanel mode preview', () => {
  it('updates the image and description when switching modes and back', async () => {
    function Harness() {
      const [mode, setMode] = useState<MotionForm['mode']>('motion')
      return <MotionPanel motion={stubMotion({ mode, setMode })} />
    }
    const user = userEvent.setup()
    render(<Harness />)
    expect(screen.getByRole('img', { name: 'Motion Control' })).toHaveAttribute(
      'src',
      '/images/influencer/motion-transfer-preview.webp'
    )
    expect(
      screen
        .getByRole('figure', { name: 'Motion Control' })
        .compareDocumentPosition(
          screen.getByRole('radiogroup', { name: 'Mode' })
        ) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    await user.click(screen.getByRole('radio', { name: 'Object swap' }))
    expect(screen.getByRole('img', { name: 'Object Swap' })).toHaveAttribute(
      'src',
      '/images/influencer/object-swap-preview.webp'
    )
    expect(
      screen.getByText(
        'Replace only what you choose. Keep the scene and motion.'
      )
    ).toBeVisible()
    expect(screen.queryByRole('heading', { name: 'Motion Control' })).toBeNull()
    await user.click(screen.getByRole('radio', { name: 'Motion transfer' }))
    expect(
      screen.getByRole('heading', { name: 'Motion Control' })
    ).toBeVisible()
    expect(
      screen.getByText('Keep the motion. Reimagine the character and scene.')
    ).toBeVisible()
  })
})

describe('MotionPanel mode wording', () => {
  it.each(['motion', 'replace'] as const)(
    'places video before inclusive reference images in %s mode',
    (mode) => {
      render(<MotionPanel motion={stubMotion({ mode })} />)
      const video = screen.getByRole('button', {
        name: 'Add a reference video to extract motion',
      })
      const images = screen.getByRole('button', {
        name: 'Add character, product or clothing images',
      })
      expect(
        video.compareDocumentPosition(images) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
      expect(screen.getByText('Up to 30 reference images')).toBeVisible()
    }
  )

  it('asks for a motion reference video in motion transfer', () => {
    render(<MotionPanel motion={stubMotion({ mode: 'motion' })} />)
    expect(
      screen.getByRole('button', {
        name: 'Add a reference video to extract motion',
      })
    ).toBeInTheDocument()
  })

  it('asks for a motion reference video in object swap', () => {
    render(<MotionPanel motion={stubMotion({ mode: 'replace' })} />)
    expect(
      screen.getByRole('button', {
        name: 'Add a reference video to extract motion',
      })
    ).toBeInTheDocument()
  })

  it('changes the prompt placeholder with the mode', () => {
    const { unmount } = render(
      <MotionPanel
        motion={stubMotion({ mode: 'motion', promptEnabled: true })}
      />
    )
    expect(screen.getByRole('textbox', { name: 'Prompt' })).toHaveAttribute(
      'aria-placeholder',
      expect.stringMatching(/Leave empty for a same-style remake/)
    )
    unmount()

    render(
      <MotionPanel
        motion={stubMotion({ mode: 'replace', promptEnabled: true })}
      />
    )
    expect(screen.getByRole('textbox', { name: 'Prompt' })).toHaveAttribute(
      'aria-placeholder',
      expect.stringMatching(/Describe the scene to keep/)
    )
  })

  it('hides the prompt box until the switch turns it on', async () => {
    const user = userEvent.setup()
    const setPromptEnabled = vi.fn()
    render(<MotionPanel motion={stubMotion({ setPromptEnabled })} />)

    // Off by default: only the compact label row, no textarea rendered.
    expect(
      screen.queryByPlaceholderText(/Leave empty for a same-style remake/)
    ).toBeNull()

    await user.click(screen.getByRole('switch'))
    // Base UI passes (checked, eventDetails); the flag is the first arg.
    expect(setPromptEnabled.mock.calls[0][0]).toBe(true)
  })

  it('renders the prompt box once the switch is on', () => {
    render(<MotionPanel motion={stubMotion({ promptEnabled: true })} />)
    expect(screen.getByRole('textbox', { name: 'Prompt' })).toHaveAttribute(
      'aria-placeholder',
      expect.stringMatching(/Leave empty for a same-style remake/)
    )
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true')
  })

  it('offers the mode switch as a radio group', () => {
    render(<MotionPanel motion={stubMotion({ mode: 'motion' })} />)
    const transfer = screen.getByRole('radio', { name: 'Motion transfer' })
    const swap = screen.getByRole('radio', { name: 'Object swap' })
    expect(transfer).toHaveAttribute('aria-checked', 'true')
    expect(swap).toHaveAttribute('aria-checked', 'false')
  })

  // The 288px track leaves 52px per segment, so the full English names wrap.
  // Each segment shows a compact label while the full name still drives the
  // accessible name and the hover title; Chinese already fits on one line.
  it('shows compact switch labels while keeping the full mode name accessible', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
    render(
      <I18nextProvider i18n={i18n}>
        <MotionPanel motion={stubMotion({ mode: 'motion' })} />
      </I18nextProvider>
    )

    const expected = [
      ['Motion transfer', 'Transfer'],
      ['Object swap', 'Swap'],
      ['Viral recreate', 'Recreate'],
    ] as const

    for (const [fullName, compactLabel] of expected) {
      const segment = screen.getByRole('radio', { name: fullName })
      expect(segment.textContent).toBe(compactLabel)
      expect(segment).toHaveAttribute('title', fullName)
    }

    await act(async () => {
      await i18n.changeLanguage('zh')
    })
    expect(screen.getByRole('radio', { name: '运动交换' }).textContent).toBe(
      '运动交换'
    )
  })
})

describe('MotionPanel styles', () => {
  it('keeps the styles behind the picker instead of listing them inline', async () => {
    const user = userEvent.setup()
    render(<MotionPanel motion={stubMotion()} />)

    expect(
      screen.queryByRole('option', { name: 'Hand-Painted Cel Animation' })
    ).toBeNull()

    await user.click(screen.getByRole('button', { name: /Style preset/ }))

    expect(
      screen.getByRole('option', { name: 'Hand-Painted Cel Animation' })
    ).toBeInTheDocument()
  })

  it('hands the picked style back only once apply is pressed', async () => {
    const user = userEvent.setup()
    const applyStyle = vi.fn()
    render(<MotionPanel motion={stubMotion({ applyStyle })} />)

    await user.click(screen.getByRole('button', { name: /Style preset/ }))
    await user.click(
      screen.getByRole('option', { name: 'Hand-Painted Cel Animation' })
    )

    expect(applyStyle).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Apply style' }))

    expect(applyStyle).toHaveBeenCalledTimes(1)
    expect(applyStyle.mock.calls[0][0]).toMatchObject({
      id: 'motion-cel-animation',
    })
  })

  it('clears the style when the no-style tile is applied', async () => {
    const user = userEvent.setup()
    const applyStyle = vi.fn()
    render(
      <MotionPanel
        motion={stubMotion({
          applyStyle,
          style: {
            id: 'motion-cel-animation',
            nameKey: 'Hand-Painted Cel Animation',
            mode: 'motion',
            prompt: 'keep the umbrella',
            durationSec: 5,
          },
        })}
      />
    )

    await user.click(screen.getByRole('button', { name: /Style preset/ }))
    await user.click(screen.getByRole('option', { name: 'No style' }))
    await user.click(screen.getByRole('button', { name: 'Apply style' }))

    expect(applyStyle).toHaveBeenCalledWith(null)
  })

  it('only offers the styles of the active mode', async () => {
    const user = userEvent.setup()
    render(<MotionPanel motion={stubMotion({ mode: 'replace' })} />)

    await user.click(screen.getByRole('button', { name: /Style preset/ }))

    expect(
      screen.getByRole('option', { name: 'Countryside Duo' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('option', { name: 'Hand-Painted Cel Animation' })
    ).toBeNull()
  })
})

describe('MotionPanel slots', () => {
  it('routes a multi-file character selection through the form as one batch', async () => {
    const user = userEvent.setup()
    const pickCharacters = vi.fn().mockResolvedValue({
      added: 0,
      failed: 0,
      skipped: 0,
    })
    const { container } = render(
      <MotionPanel motion={stubMotion({ pickCharacters })} />
    )
    const input = container.querySelector(
      'input[accept="image/png,image/jpeg,image/webp"]'
    )
    if (!(input instanceof HTMLInputElement)) {
      throw new Error('the character slot rendered no file input')
    }
    expect(input.multiple).toBe(true)
    const files = [
      new File(['one'], 'one.png', { type: 'image/png' }),
      new File(['two'], 'two.png', { type: 'image/png' }),
    ]

    await user.upload(input, files)

    expect(pickCharacters).toHaveBeenCalledWith(files)
  })

  it('clears only the video slot from its own remove control', async () => {
    const user = userEvent.setup()
    const clearVideo = vi.fn()
    const clearCharacter = vi.fn()
    render(
      <MotionPanel
        motion={stubMotion({
          video: {
            kind: 'upload',
            url: '/clip.mp4',
            fileName: 'clip.mp4',
            metadata: videoMetadata,
          },
          videoValidation: videoMetadata,
          durationSec: 17,
          clearVideo,
          clearCharacter,
        })}
      />
    )

    await user.click(screen.getByRole('button', { name: /Remove/ }))

    expect(clearVideo).toHaveBeenCalledTimes(1)
    expect(clearCharacter).not.toHaveBeenCalled()
  })
})

describe('MotionPanel reference parameters', () => {
  it.each(['motion', 'replace'] as const)(
    'keeps time and frame read-only and omits the reference copy in %s mode',
    (mode) => {
      render(<MotionPanel motion={stubMotion({ mode })} />)
      expect(screen.queryByRole('spinbutton')).toBeNull()
      expect(screen.queryByRole('radiogroup', { name: 'Frame' })).toBeNull()
      expect(screen.queryByText(/Follows reference video/)).toBeNull()
      expect(screen.queryByText(/Generated duration/)).toBeNull()
      expect(
        screen.queryByText(
          'Add a reference video to detect duration and frame automatically.'
        )
      ).toBeNull()
      expect(
        screen.getByRole('radiogroup', { name: 'Resolution' })
      ).toBeVisible()
    }
  )
  it.each(['motion', 'replace'] as const)(
    'keeps the reference duration badge while omitting the duration copy in %s mode',
    (mode) => {
      render(
        <MotionPanel
          motion={stubMotion({
            mode,
            video: {
              kind: 'upload',
              url: '/clip.mp4',
              fileName: 'clip.mp4',
              metadata: videoMetadata,
            },
            videoValidation: videoMetadata,
          })}
        />
      )
      expect(screen.getByText('17s')).toBeVisible()
      expect(screen.queryByText(/Follows reference video/)).toBeNull()
      expect(screen.queryByText(/Generated duration/)).toBeNull()
      expect(screen.queryByText(/9:16/)).toBeNull()
      expect(screen.queryByRole('spinbutton')).toBeNull()
      expect(screen.queryByRole('radiogroup', { name: 'Frame' })).toBeNull()
    }
  )
  it('does not show a zero-second duration while reading the reference', () => {
    render(
      <MotionPanel
        motion={stubMotion({
          video: {
            kind: 'remote',
            url: 'https://cdn.example/clip.mp4',
            name: 'Reference',
          },
          videoMetadataLoading: true,
        })}
      />
    )
    expect(screen.queryByText('0s')).toBeNull()
    expect(screen.queryByRole('spinbutton')).toBeNull()
    expect(screen.queryByText(/Follows reference video/)).toBeNull()
  })
  it('keeps resolution selectable', async () => {
    const setResolution = vi.fn()
    render(<MotionPanel motion={stubMotion({ setResolution })} />)
    await userEvent.setup().click(screen.getByRole('radio', { name: '480P' }))
    expect(setResolution).toHaveBeenCalledWith('480p')
  })
})

describe('MotionPanel video feedback', () => {
  it.each(['motion', 'replace'] as const)(
    'omits the duplicate preparation message while allowing task inspection and locking file edits in %s mode',
    async (mode) => {
      const openVideoPicker = vi.fn()
      const view = render(
        <MotionPanel
          motion={stubMotion({ mode, videoPreparing: true, openVideoPicker })}
        />
      )
      expect(screen.queryByText('Preparing reference video…')).toBeNull()
      const entry = screen.getByRole('button', {
        name: 'Add a reference video to extract motion',
      })
      expect(entry).toBeEnabled()
      await userEvent.setup().click(entry)
      expect(openVideoPicker).toHaveBeenCalledOnce()
      expect(
        view.container.querySelector(
          'input[accept="video/mp4,video/quicktime"]'
        )
      ).toBeDisabled()
    }
  )

  it.each(['duration', 'corrupt'] as const)(
    'does not duplicate a %s failure already reported by the selection flow',
    async (reason) => {
      vi.mocked(toast.error).mockClear()
      const pickVideo = vi.fn().mockResolvedValue({ ok: false, reason })
      const { container } = render(
        <MotionPanel motion={stubMotion({ pickVideo })} />
      )
      const input = container.querySelector(
        'input[accept="video/mp4,video/quicktime"]'
      ) as HTMLInputElement
      const file = new File(['bytes'], 'clip.mp4', { type: 'video/mp4' })
      await userEvent.setup().upload(input, file)
      expect(pickVideo).toHaveBeenCalledWith(file)
      expect(toast.error).not.toHaveBeenCalled()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    }
  )

  it('shows localized upload failure feedback instead of a Chinese server message', async () => {
    vi.mocked(toast.error).mockClear()
    const user = userEvent.setup()
    const pickVideo = vi.fn().mockRejectedValue({
      response: {
        status: 500,
        data: { success: false, msg: '公开存储地址必须使用独立于本站的域名' },
      },
    })
    const { container } = render(
      <MotionPanel motion={stubMotion({ pickVideo })} />
    )
    const input = container.querySelector(
      'input[accept="video/mp4,video/quicktime"]'
    )
    if (!(input instanceof HTMLInputElement)) {
      throw new Error('the video slot rendered no file input')
    }

    await user.upload(
      input,
      new File(['bytes'], 'clip.mp4', { type: 'video/mp4' })
    )

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledExactlyOnceWith(
        'Upload failed. Please try again.'
      )
    )
    expect(
      screen.getByRole('button', {
        name: 'Add a reference video to extract motion',
      })
    ).toBeEnabled()
  })

  it('accepts a successful structured video result without an error', async () => {
    vi.mocked(toast.error).mockClear()
    const user = userEvent.setup()
    const { container } = render(<MotionPanel motion={stubMotion()} />)
    const input = container.querySelector(
      'input[accept="video/mp4,video/quicktime"]'
    )
    if (!(input instanceof HTMLInputElement)) {
      throw new Error('the video slot rendered no file input')
    }

    await user.upload(
      input,
      new File(['bytes'], 'clip.mp4', { type: 'video/mp4' })
    )

    await waitFor(() =>
      expect(
        screen.getByRole('button', {
          name: 'Add a reference video to extract motion',
        })
      ).toBeEnabled()
    )
    expect(toast.error).not.toHaveBeenCalled()
  })
})

describe('MotionPanel catalogue title localization', () => {
  const replacement = motionExplorePresets().find(
    (preset) => preset.mode === 'replace'
  )
  if (!replacement) throw new Error('Missing replacement catalogue fixture')
  const item = replacement
  const preset = exploreToMotionPreset(item)

  it('localizes replacement picker and selected captions with real resources and live language changes', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'zh', fallbackLng: 'en', resources: { en, zh } })
    const localizedTitle = i18n.t(item.title)
    expect(localizedTitle).not.toBe(item.title)
    const applyStyle = vi.fn()
    function Harness() {
      const [style, setStyle] = useState<MotionForm['style']>(null)
      return (
        <MotionPanel
          motion={stubMotion({
            mode: 'replace',
            style,
            video: style
              ? {
                  kind: 'remote',
                  name: item.title,
                  url: item.referenceVideoUrl ?? item.videoUrl,
                  previewUrl: item.videoUrl,
                }
              : null,
            applyStyle: (choice) => {
              applyStyle(choice)
              setStyle(choice)
            },
          })}
        />
      )
    }
    render(
      <I18nextProvider i18n={i18n}>
        <Harness />
      </I18nextProvider>
    )
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('button', { name: new RegExp(i18n.t('Style preset')) })
    )
    for (const replacement of motionExplorePresets().filter(
      (entry) => entry.mode === 'replace'
    )) {
      expect(
        screen.getByRole('option', { name: i18n.t(replacement.title) })
      ).toBeVisible()
    }
    await user.click(screen.getByRole('option', { name: localizedTitle }))
    await user.click(
      screen.getByRole('button', { name: i18n.t('Apply style') })
    )
    expect(applyStyle).toHaveBeenCalledWith(preset)
    expect(screen.getAllByText(localizedTitle)).toHaveLength(2)
    expect(screen.queryByText(item.title)).toBeNull()
    await act(async () => {
      await i18n.changeLanguage('en')
    })
    expect(screen.getAllByText(item.title)).toHaveLength(2)
    expect(screen.queryByText(localizedTitle)).toBeNull()
  })

  it('localizes a selected community clip without changing its source when language changes', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
    const community = communityTrendPresets()[0]
    render(
      <I18nextProvider i18n={i18n}>
        <MotionPanel
          motion={stubMotion({
            video: {
              kind: 'remote',
              name: community.title,
              url: community.referenceVideoUrl ?? community.videoUrl,
              previewUrl: community.videoUrl,
            },
          })}
        />
      </I18nextProvider>
    )
    expect(screen.getByText(community.title)).toBeVisible()
    await act(async () => {
      await i18n.changeLanguage('zh')
    })
    expect(i18n.t(community.title)).not.toBe(community.title)
    expect(screen.getByText(i18n.t(community.title))).toBeVisible()
    expect(screen.queryByText(community.title)).toBeNull()
  })

  it.each(['Download', 'Countryside Duo', '我的视频.mp4'])(
    'never shows the uploaded file name %s in the video slot',
    async (fileName) => {
      const i18n = createInstance()
      await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
      render(
        <I18nextProvider i18n={i18n}>
          <MotionPanel
            motion={stubMotion({
              video: { kind: 'upload', url: '/user.mp4', fileName },
            })}
          />
        </I18nextProvider>
      )
      expect(screen.queryByText(fileName)).toBeNull()
      await act(async () => {
        await i18n.changeLanguage('zh')
      })
      expect(screen.queryByText(fileName)).toBeNull()
    }
  )

  it.each(['Download', 'Countryside Duo', 'my-video.mp4'])(
    'does not show uncatalogued remote file name %s',
    async (name) => {
      const i18n = createInstance()
      await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
      render(
        <I18nextProvider i18n={i18n}>
          <MotionPanel
            motion={stubMotion({
              video: {
                kind: 'remote',
                url: 'https://user.example/clip.mp4',
                name,
              },
            })}
          />
        </I18nextProvider>
      )
      expect(screen.queryByText(name)).toBeNull()
      await act(async () => {
        await i18n.changeLanguage('zh')
      })
      expect(screen.queryByText(name)).toBeNull()
    }
  )
})

describe('MotionPanel viral recreate', () => {
  it('offers the third mode as one generate form without an analyze action', () => {
    render(<MotionPanel motion={stubMotion({ mode: 'recreate' })} />)

    expect(
      screen.getByRole('heading', { name: 'Viral Recreate' })
    ).toBeVisible()
    expect(
      screen.getByRole('radio', { name: 'Viral recreate' })
    ).toHaveAttribute('aria-checked', 'true')
    expect(
      screen.getByRole('button', { name: 'Add a reference video' })
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Add reference images' })
    ).toBeVisible()
    // Generation is the only action; decomposition is an internal step.
    expect(
      screen.queryByRole('button', { name: 'Analyze reference video' })
    ).toBeNull()
    expect(
      screen.queryByText(
        'Only used to decompose narrative and pacing; never sent to the generation model.'
      )
    ).toBeNull()
    expect(
      screen.getByText('Video duration: 4 to 30 seconds')
    ).toBeVisible()
    expect(screen.getByText('Up to 30 reference images')).toBeVisible()
    expect(screen.queryByText('Decomposition only')).toBeNull()
    // Output quality now lives in the form, not in a confirmation step.
    expect(
      screen.getByRole('radiogroup', { name: 'Resolution' })
    ).toBeVisible()
    expect(screen.getByRole('radio', { name: '480P' })).toBeVisible()
    expect(screen.getByRole('radio', { name: '720P' })).toBeVisible()
    // The prompt uses the same switch card as the two movement modes.
    expect(screen.getByRole('switch', { name: 'Prompt' })).not.toBeChecked()
    expect(screen.queryByRole('textbox', { name: 'Prompt' })).toBeNull()
    // Both inputs live in one Assets card; the grid carries no order badges.
    expect(screen.getByText('Assets')).toBeVisible()
    expect(screen.queryByText('Reference video')).toBeNull()
    expect(document.querySelector('.inf-slot-badge')).toBeNull()
  })

  it('shows the intent rune counter instead of an inline over-limit error', () => {
    render(
      <MotionPanel
        motion={stubMotion({
          mode: 'recreate',
          recreateIntentPromptEnabled: true,
          recreateIntentPrompt: 'x'.repeat(1205),
        })}
      />
    )

    expect(screen.getByText(/1205 \/ 1200/)).toBeVisible()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('anchors the shared reference picker to the recreate assets card', () => {
    const assetsRef = createRef<HTMLElement>()
    const { container } = render(
      <MotionPanel
        motion={stubMotion({ mode: 'recreate' })}
        assetsRef={assetsRef}
      />
    )

    // The desktop picker closes itself when the anchor is missing, so the
    // recreation Assets card must be the measured anchor.
    expect(assetsRef.current).toBe(container.querySelector('.inf-acc'))
  })

  it('reveals a prompt editor with image-only mentions in the recreate mode', async () => {
    render(
      <MotionPanel
        motion={stubMotion({
          mode: 'recreate',
          recreateIntentPromptEnabled: true,
          recreateIntentPrompt: '结尾收在 @',
          recreateImages: [
            { url: 'https://cdn.example.com/a.jpg', fileName: 'a.jpg' },
            { url: 'https://cdn.example.com/b.jpg', fileName: 'b.jpg' },
          ],
        })}
      />
    )

    const textbox = screen.getByRole('textbox', { name: 'Prompt' })
    expect(textbox).toBeVisible()
    const view = EditorView.findFromDOM(textbox)
    if (!view) throw new Error('Missing editor')
    act(() => {
      view.focus()
      view.dispatch({ selection: { anchor: view.state.doc.length } })
      startCompletion(view)
    })
    const options = await screen.findAllByRole('option')
    const labels = options.map((option) => option.textContent ?? '')
    expect(labels.some((label) => label.includes('Image 1'))).toBe(true)
    // The clip is decomposition material only: no video mention is offered.
    expect(labels.some((label) => label.includes('Video'))).toBe(false)
  })

  it('keeps the form visible but inert while the decomposition runs', () => {
    render(
      <MotionPanel
        motion={stubMotion({
          mode: 'recreate',
          recreateAnalyzing: true,
          canSubmit: false,
        })}
      />
    )

    expect(screen.getByText('About 2–3 minutes')).toBeVisible()
    expect(screen.getByText('Keep this page open')).toBeVisible()
    // Inputs stay readable but are removed from interaction.
    expect(
      screen.getByRole('button', { name: 'Add a reference video' })
    ).toBeInTheDocument()
    expect(document.querySelector('[inert]')).not.toBeNull()
    expect(screen.queryByRole('button', { name: 'Generate video' })).toBeNull()
  })

  it('reuses a kept decomposition without showing a confirmation step', () => {
    render(
      <MotionPanel
        motion={stubMotion({
          mode: 'recreate',
          canSubmit: true,
          recreatePrompt: 'Reference image 1 is the hero product.',
          recreatePromptRunes: 38,
          recreateResult: {
            prompt: 'Reference image 1 is the hero product.',
            analysis: {
              video_overview: {
                one_line: 'One continuous close-up with a slow reveal.',
              },
              shots: [
                {
                  shot_id: 'S01',
                  start_s: 0,
                  end_s: 4.5,
                  shot_size: 'close',
                  narrative_job: 'hook',
                },
              ],
            },
            imageAnalysis: [],
            meta: {
              shot_count: 1,
              duration_sec: 11,
              pricing: { '480p': 6.47, '720p': 13.89 },
            },
            durationSec: 11,
            referenceDurationSec: 10.6,
          },
        })}
      />
    )

    // The kept result only feeds the next submit; it never becomes a page.
    expect(screen.queryByText('Decomposition result')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Re-analyze' })).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'Prompt text' })).toBeNull()
    expect(
      screen.getByRole('button', { name: 'Add a reference video' })
    ).toBeVisible()
  })
})
