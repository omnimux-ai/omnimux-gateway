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

import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { assert, describe, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { MotionStyleRow } from '../components/motion-style-row'
import { MOTION_PRESETS } from '../lib/motion-presets'
import { MOTION_VISUAL_STYLES } from '../lib/motion-visual-styles'

describe('MotionStyleRow visual descriptions', () => {
  it('keeps a selected replacement shelf description visible when its generation supplement is empty', () => {
    const preset = MOTION_PRESETS.find((item) => item.mode === 'replace')
    assert(preset, 'The replacement shelf fixture must exist')
    render(
      <MotionStyleRow presets={[preset]} style={preset} onOpen={vi.fn()} />
    )

    expect(
      screen.getByText('Explore this object replacement preset.')
    ).toBeVisible()
  })

  it('updates the selected name and summary when the language changes', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
    render(
      <I18nextProvider i18n={i18n}>
        <MotionStyleRow
          presets={MOTION_VISUAL_STYLES}
          style={MOTION_VISUAL_STYLES[0]}
          onOpen={vi.fn()}
        />
      </I18nextProvider>
    )
    const style = MOTION_VISUAL_STYLES[0]
    expect(screen.getByText(i18n.t(style.nameKey))).toBeVisible()
    await act(async () => {
      await i18n.changeLanguage('zh')
    })
    expect(i18n.t(style.nameKey)).not.toBe(style.nameKey)
    expect(screen.getByText(i18n.t(style.nameKey))).toBeVisible()
    expect(screen.getByText(i18n.t(style.summaryKey))).toBeVisible()
  })

  it('preserves the legacy replacement title, thumbnail and instruction', () => {
    const legacy = {
      id: 'replace-scene',
      nameKey: 'Scene Recast',
      mode: 'replace' as const,
      prompt: 'Keep the original scene',
      durationSec: 5,
      thumbUrl: 'https://cdn.example/scene.webp',
    }
    render(
      <MotionStyleRow
        presets={[...MOTION_VISUAL_STYLES, legacy]}
        style={legacy}
        onOpen={vi.fn()}
      />
    )
    expect(screen.getByText('Scene Recast')).toBeVisible()
    expect(screen.getByText('Keep the original scene')).toBeVisible()
    expect(document.querySelector('img')).toHaveAttribute(
      'src',
      'https://cdn.example/scene.webp'
    )
  })

  it('opens the picker from the unselected entry by keyboard', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    render(<MotionStyleRow presets={[]} style={null} onOpen={onOpen} />)
    const entry = screen.getByRole('button', {
      name: /Style preset.*Apply style/,
    })
    entry.focus()
    await user.keyboard('{Enter}')
    expect(onOpen).toHaveBeenCalled()
  })

  it('shows the selected cover and summary without a video or the generation prompt', () => {
    render(
      <MotionStyleRow
        presets={MOTION_VISUAL_STYLES}
        style={MOTION_VISUAL_STYLES[5]}
        onOpen={vi.fn()}
      />
    )

    const style = MOTION_VISUAL_STYLES[5]
    expect(screen.getByText(style.summaryKey)).toBeVisible()
    expect(screen.queryByText(style.prompt)).not.toBeInTheDocument()
    expect(screen.getByRole('button')).toHaveAttribute(
      'aria-haspopup',
      'dialog'
    )
    expect(document.querySelector('img')).toHaveAttribute(
      'src',
      style.previewImageUrl
    )
    expect(document.querySelectorAll('img')).toHaveLength(3)
    expect(document.querySelector('video')).toBeNull()
  })

  it('keeps the entry available when the selected cover fails', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    const style = MOTION_VISUAL_STYLES[0]
    render(<MotionStyleRow presets={[style]} style={style} onOpen={onOpen} />)
    fireEvent.error(document.querySelector('img') as HTMLImageElement)
    expect(document.querySelector('img')).toBeNull()
    expect(screen.getByText(style.nameKey)).toBeVisible()
    await user.click(screen.getByRole('button'))
    expect(onOpen).toHaveBeenCalledOnce()
  })
})
