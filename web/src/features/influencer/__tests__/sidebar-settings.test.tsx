import { act, render, renderHook, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { OptionCard } from '../components/option-card'
import { CharacterSettingsSummary } from '../components/settings-summary'
import { useBuilder } from '../hooks/use-builder'
import type { InfluencerTaxonomy } from '../types'

const taxonomy: InfluencerTaxonomy = {
  version: 1,
  source: 'test',
  category_priority: ['hair'],
  tier_group: { options: [], comingSoonImageUrls: [] },
  categories: [
    {
      id: 'hair',
      label_en: 'Hair',
      kind: 'media',
      max: 2,
      options: [
        {
          id: 'long',
          label_en: 'Long hair',
          visibleIn: ['total', 'normal'],
          imageUrl: '/long.webp',
        },
        { id: 'bold', label_en: 'Bold hair', visibleIn: ['total'] },
      ],
    },
  ],
  prompt_map: { tiers: {}, categories: {} },
}

describe('sidebar selected settings', () => {
  it('hides empty features without presenting the default tier as a choice', () => {
    render(
      <CharacterSettingsSummary
        taxonomy={taxonomy}
        tier='total'
        selection={{}}
        onReset={() => {}}
      />
    )
    expect(screen.queryByLabelText('Selected settings')).not.toBeInTheDocument()
  })

  it('filters unknown and tier-hidden choices while retaining valid multiple choices', () => {
    const view = render(
      <CharacterSettingsSummary
        taxonomy={taxonomy}
        tier='total'
        selection={{ hair: ['long', 'bold', 'missing'], unknown: ['long'] }}
        onReset={() => {}}
      />
    )
    expect(screen.getByLabelText('Selected settings')).toHaveTextContent(
      'Long hair · Bold hair'
    )
    expect(screen.getByLabelText('Selected settings')).not.toHaveTextContent(
      'missing'
    )
    view.rerender(
      <CharacterSettingsSummary
        taxonomy={taxonomy}
        tier='normal'
        selection={{ hair: ['bold'] }}
        onReset={() => {}}
      />
    )
    expect(screen.queryByLabelText('Selected settings')).not.toBeInTheDocument()
  })

  it('expands and collapses the summary through an accessible disclosure', async () => {
    render(
      <CharacterSettingsSummary
        taxonomy={taxonomy}
        tier='total'
        selection={{ hair: ['long', 'bold'] }}
        onReset={() => {}}
      />
    )
    const user = userEvent.setup()
    expect(screen.getByRole('button', { name: 'Show more' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
    await user.click(screen.getByRole('button', { name: 'Show more' }))
    expect(screen.getByRole('button', { name: 'Show less' })).toHaveAttribute(
      'aria-expanded',
      'true'
    )
    await user.click(screen.getByRole('button', { name: 'Show less' }))
    expect(screen.getByRole('button', { name: 'Show more' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })

  it('resetting only features preserves image, seed, tier and model while ignoring a legacy brief', () => {
    const { result } = renderHook(() => useBuilder(taxonomy))
    act(() =>
      result.current.applyLook({
        tier: 'normal',
        values: { hair: ['long'] },
        seed: 42,
        image_url: 'https://files.example/reference.png',
        model: 'image-model',
      })
    )
    act(() => result.current.resetFeatures())
    expect(result.current.state).toMatchObject({
      selection: {},
      brief: '',
      seed: '42',
      tier: 'normal',
      imageUrl: 'https://files.example/reference.png',
      model: 'image-model',
    })
  })

  it('does not allow feature reset when generation is busy', async () => {
    let resets = 0
    render(
      <CharacterSettingsSummary
        taxonomy={taxonomy}
        tier='total'
        selection={{ hair: ['long'] }}
        disabled
        onReset={() => {
          resets += 1
        }}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Reset features' })
    ).toBeDisabled()
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Reset features' }))
    expect(resets).toBe(0)
  })

  it.each(['media', 'color', 'text'] as const)(
    'keeps the full %s option name in its own label without an image overlay',
    async (kind) => {
      const label =
        'An exceptionally long character feature name that remains readable'
      render(
        <OptionCard
          kind={kind}
          option={{
            id: 'long-name',
            label_en: label,
            visibleIn: ['total'],
            imageUrl: '/long.webp',
            swatch: '#fff',
          }}
          selected={false}
          onClick={() => {}}
        />
      )
      const button = screen.getByRole('button', { name: label })
      expect(button).toHaveTextContent(label)
      expect(button.querySelector('.inf-overlay')).toBeNull()
      expect(button.querySelector('.inf-lbl')).toHaveTextContent(label)
      expect(button).toHaveAttribute('aria-pressed', 'false')
    }
  )

  it.each([
    { id: 'st_fair', label_en: 'Fair', expected: '白皙肤色' },
    { id: 'st_light', label_en: 'Light', expected: '浅肤色' },
  ])(
    'uses the Chinese skin meaning for $label_en in both the option and summary',
    async (skin) => {
      const instance = i18next.createInstance()
      await instance.init({ lng: 'zh', resources: { en, zh } })
      const option = {
        id: skin.id,
        label_en: skin.label_en,
        visibleIn: ['normal', 'freak', 'total'],
      }
      const skinTaxonomy: InfluencerTaxonomy = {
        ...taxonomy,
        categories: [
          ...taxonomy.categories,
          {
            id: 'skin_tone',
            label_en: 'Skin color',
            kind: 'color',
            max: 1,
            options: [option],
          },
        ],
      }
      render(
        <I18nextProvider i18n={instance}>
          <OptionCard
            categoryId='skin_tone'
            kind='color'
            option={option}
            selected
            onClick={() => {}}
          />
          <CharacterSettingsSummary
            taxonomy={skinTaxonomy}
            tier='total'
            selection={{
              skin_tone: [skin.id, 'missing'],
              hair: ['bold', 'long'],
              unknown: ['missing'],
            }}
            onReset={() => {}}
          />
        </I18nextProvider>
      )
      expect(
        screen.getByRole('button', { name: skin.expected })
      ).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByLabelText('已选设置')).toHaveTextContent(
        `Bold hair · 长发 · ${skin.expected}`
      )
      expect(screen.getByLabelText('已选设置')).not.toHaveTextContent('missing')
      await act(async () => {
        await instance.changeLanguage('en')
      })
      expect(
        screen.getByRole('button', { name: `${skin.label_en} skin` })
      ).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByLabelText('Selected settings')).toHaveTextContent(
        `Bold hair · Long hair · ${skin.label_en} skin`
      )
    }
  )

  it.each(['Fair', 'Light'])(
    'keeps the existing %s translation outside skin tone',
    async (label) => {
      const instance = i18next.createInstance()
      await instance.init({ lng: 'zh', resources: { en, zh } })
      const option = { id: 'theme', label_en: label, visibleIn: ['total'] }
      const themeTaxonomy: InfluencerTaxonomy = {
        ...taxonomy,
        categories: [
          {
            id: 'theme',
            label_en: 'Theme',
            kind: 'text',
            max: 1,
            options: [option],
          },
        ],
      }
      render(
        <I18nextProvider i18n={instance}>
          <OptionCard
            categoryId='theme'
            kind='text'
            option={option}
            selected
            onClick={() => {}}
          />
          <CharacterSettingsSummary
            taxonomy={themeTaxonomy}
            tier='total'
            selection={{ theme: ['theme'] }}
            onReset={() => {}}
          />
        </I18nextProvider>
      )
      expect(
        screen.getByRole('button', {
          name: zh.translation[label as 'Fair' | 'Light'],
        })
      ).toBeInTheDocument()
      expect(screen.getByLabelText('已选设置')).toHaveTextContent(
        zh.translation[label as 'Fair' | 'Light']
      )
    }
  )

  it('updates option names and summary labels when the language changes', async () => {
    const instance = i18next.createInstance()
    await instance.init({
      lng: 'en',
      resources: {
        en: { translation: { 'Long hair': 'Long hair' } },
        zh: {
          translation: { 'Long hair': '长发', 'Selected settings': '已选设置' },
        },
      },
    })
    render(
      <I18nextProvider i18n={instance}>
        <OptionCard
          kind='media'
          option={taxonomy.categories[0].options[0]}
          selected
          onClick={() => {}}
        />
        <CharacterSettingsSummary
          taxonomy={taxonomy}
          tier='total'
          selection={{ hair: ['long'] }}
          onReset={() => {}}
        />
      </I18nextProvider>
    )
    expect(screen.getByRole('button', { name: 'Long hair' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    await act(async () => {
      await instance.changeLanguage('zh')
    })
    expect(screen.getByRole('button', { name: '长发' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByLabelText('已选设置')).toHaveTextContent('长发')
  })
})
