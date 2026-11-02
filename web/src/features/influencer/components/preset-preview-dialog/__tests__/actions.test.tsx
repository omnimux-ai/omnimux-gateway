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

import { act, cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, describe, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import type { ExplorePreset, InfluencerTaxonomy } from '../../../types'
import { PresetPreviewDialog } from '../../preset-preview-dialog'

const preset: ExplorePreset = {
  id: 'p1',
  name: 'Character',
  tier: 'normal',
  selection: { hair: ['bowl'] },
  brief: 'Direction',
  published: true,
  preview: { path: '/preview.png', width: 400, height: 600 },
  sheet: { path: '/sheet.png', width: 1200, height: 700 },
}
const taxonomy: InfluencerTaxonomy = {
  version: 1,
  source: 'test',
  prompt_map: { tiers: {}, categories: {} },
  tier_group: {
    options: [{ id: 'normal', label_en: 'Average', visibleIn: ['normal'] }],
    comingSoonImageUrls: [],
  },
  category_priority: ['hair'],
  categories: [
    {
      id: 'hair',
      label_en: 'Hair',
      kind: 'media',
      max: 1,
      options: [{ id: 'bowl', label_en: 'Bowl cut', visibleIn: ['normal'] }],
    },
  ],
}
afterEach(cleanup)
describe('inspiration full-screen preview actions', () => {
  it('provides a real accessible title and preserves the recreate callback', async () => {
    const apply = vi.fn()
    render(
      <PresetPreviewDialog
        preset={preset}
        taxonomy={taxonomy}
        onClose={vi.fn()}
        onApply={apply}
      />
    )
    expect(screen.getByRole('heading', { name: 'Character' })).toHaveAttribute(
      'data-slot',
      'dialog-title'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Recreate' }))
    expect(apply).toHaveBeenCalledWith(preset)
  })
  it('keeps unavailable presets disabled and restricts removal to administrators', () => {
    render(
      <PresetPreviewDialog
        preset={preset}
        taxonomy={null}
        onClose={vi.fn()}
        onApply={vi.fn()}
        onRemove={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: 'Recreate' })).toBeDisabled()
    expect(
      screen.queryByRole('button', { name: 'Remove' })
    ).not.toBeInTheDocument()
  })
  it('requires confirmation before administrator removal', async () => {
    const remove = vi.fn()
    render(
      <PresetPreviewDialog
        preset={preset}
        taxonomy={taxonomy}
        onClose={vi.fn()}
        onApply={vi.fn()}
        onRemove={remove}
        isAdmin
      />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(remove).not.toHaveBeenCalled()
    await userEvent.click(
      within(screen.getByRole('alertdialog')).getByRole('button', {
        name: 'Remove',
      })
    )
    expect(remove).toHaveBeenCalledWith(preset)
  })
})

it('translates controlled preset parameters on language change while preserving authored content', async () => {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en, zh } })
  const character: ExplorePreset = {
    ...preset,
    name: 'Male',
    brief: 'Athletic',
    selection: { gender: ['male'], build: ['athletic'], hair: ['mullet'] },
  }
  const categories: InfluencerTaxonomy = {
    ...taxonomy,
    category_priority: ['gender', 'build', 'hair'],
    categories: [
      {
        id: 'gender',
        label_en: 'Gender',
        kind: 'text',
        max: 1,
        options: [{ id: 'male', label_en: 'Male', visibleIn: ['normal'] }],
      },
      {
        id: 'build',
        label_en: 'Build',
        kind: 'text',
        max: 1,
        options: [
          { id: 'athletic', label_en: 'Athletic', visibleIn: ['normal'] },
        ],
      },
      {
        id: 'hair',
        label_en: 'Hair',
        kind: 'text',
        max: 1,
        options: [{ id: 'mullet', label_en: 'Mullet', visibleIn: ['normal'] }],
      },
    ],
  }
  render(
    <I18nextProvider i18n={i18n}>
      <PresetPreviewDialog
        preset={character}
        taxonomy={categories}
        onClose={vi.fn()}
        onApply={vi.fn()}
      />
    </I18nextProvider>
  )
  const parameters = screen.getByRole('list')
  expect(within(parameters).getByText('Male')).toBeVisible()
  expect(within(parameters).getByText('Athletic')).toBeVisible()
  expect(within(parameters).getByText('Mullet')).toBeVisible()
  await act(async () => {
    await i18n.changeLanguage('zh')
  })
  expect(within(parameters).getByText('男性')).toBeVisible()
  expect(within(parameters).getByText('健美')).toBeVisible()
  expect(within(parameters).getByText('鲻鱼头')).toBeVisible()
  expect(within(parameters).queryByText('Male')).not.toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'Male' })).toBeVisible()
  expect(screen.getByText('Athletic')).toBeVisible()
  await act(async () => {
    await i18n.changeLanguage('en')
  })
  expect(within(parameters).getByText('Male')).toBeVisible()
  expect(within(parameters).getByText('Athletic')).toBeVisible()
  expect(within(parameters).getByText('Mullet')).toBeVisible()
})
