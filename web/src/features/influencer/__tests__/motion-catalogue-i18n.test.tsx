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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import viLocale from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { STATIC_I18N_KEYS } from '@/i18n/static-keys'

import { ExploreMotionGrid } from '../components/explore-motion-grid'
import { MotionPresetDialog } from '../components/motion-preset-dialog'
import {
  communityTrendPresets,
  motionExplorePresets,
} from '../lib/motion-explore'

const presets = [...motionExplorePresets(), ...communityTrendPresets()]
const locales = { en, zh, 'zh-TW': zhTW, fr, ja, ru, vi: viLocale }

beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
})

describe('motion catalogue title localization', () => {
  it.each(Object.entries(locales))(
    'provides all 59 registered catalogue titles in %s',
    (language, locale) => {
      const titles = new Set(presets.map((preset) => preset.title))
      expect(titles.size).toBe(59)
      const translation: Record<string, unknown> = locale.translation
      for (const title of titles) {
        expect(STATIC_I18N_KEYS, title).toContain(title)
        expect(translation[title], title).toBeTypeOf('string')
        expect(String(translation[title]).trim(), title).not.toBe('')
        if (language !== 'en') expect(translation[title], title).not.toBe(title)
      }
    }
  )

  it('updates shared card text and accessible names without changing the applied catalogue record', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
    const preset = presets[0]
    const original = structuredClone(preset)
    const onPreview = vi.fn()
    const onApply = vi.fn()
    render(
      <I18nextProvider i18n={i18n}>
        <ExploreMotionGrid
          presets={[preset]}
          onPreview={onPreview}
          onApply={onApply}
        />
      </I18nextProvider>
    )
    expect(screen.getByRole('button', { name: preset.title })).toBeVisible()
    await act(async () => {
      await i18n.changeLanguage('zh')
    })
    const localizedTitle = i18n.t(preset.title)
    expect(localizedTitle).not.toBe(preset.title)
    expect(screen.getByText(localizedTitle)).toBeVisible()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: localizedTitle }))
    expect(onPreview).toHaveBeenCalledWith(preset)
    await user.click(screen.getByRole('button', { name: i18n.t('Use') }))
    expect(onApply).toHaveBeenCalledWith(original)
    expect(preset).toEqual(original)
  })

  it('updates fullscreen heading and media accessible name while preserving the prompt and applied record', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
    const preset = presets.find((item) => item.mode === 'replace')
    if (!preset) throw new Error('Missing replacement catalogue fixture')
    const original = structuredClone(preset)
    const onApply = vi.fn()
    render(
      <I18nextProvider i18n={i18n}>
        <MotionPresetDialog
          preset={preset}
          onClose={vi.fn()}
          onApply={onApply}
        />
      </I18nextProvider>
    )
    expect(screen.getByRole('heading', { name: preset.title })).toBeVisible()
    expect(
      screen.getByLabelText(preset.title, { selector: 'video' })
    ).toHaveAttribute('src', preset.videoUrl)
    await act(async () => {
      await i18n.changeLanguage('zh')
    })
    const localizedTitle = i18n.t(preset.title)
    expect(localizedTitle).not.toBe(preset.title)
    expect(screen.getByRole('heading', { name: localizedTitle })).toBeVisible()
    const video = screen.getByLabelText(localizedTitle, { selector: 'video' })
    expect(video).toHaveAttribute('src', preset.videoUrl)
    fireEvent.loadedData(video)
    if (preset.description) {
      expect(screen.getByText(preset.description)).toBeVisible()
    }
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: i18n.t('Use this motion') }))
    expect(onApply).toHaveBeenCalledWith(original)
    expect(preset).toEqual(original)
  })
})
