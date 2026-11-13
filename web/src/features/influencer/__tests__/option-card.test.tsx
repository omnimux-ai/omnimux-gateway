import { readFileSync } from 'node:fs'

import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { OptionCard } from '../components/option-card'

const css = readFileSync('src/features/influencer/influencer.css', 'utf8')
let style: HTMLStyleElement

beforeEach(() => {
  style = document.createElement('style')
  style.textContent = css
  document.head.append(style)
})
afterEach(() => style.remove())

describe('character option presentation', () => {
  it.each(['animal_breed', 'coat_pattern'])(
    'keeps the text-only %s label visible without a separate dark strip',
    (categoryId) => {
      render(
        <div className='influencer-page'>
          <OptionCard
            categoryId={categoryId}
            kind='text'
            option={{
              id: 'natural',
              label_en: 'Natural',
              visibleIn: ['cats', 'dogs'],
            }}
            selected={false}
            onClick={() => {}}
          />
        </div>
      )
      const label = within(
        screen.getByRole('button', { name: 'Natural' })
      ).getByText('Natural')
      expect(getComputedStyle(label).opacity).toBe('1')
      expect(getComputedStyle(label).transform).toBe('none')
      expect(getComputedStyle(label).backgroundColor).toBe('rgba(0, 0, 0, 0)')
    }
  )
  it.each(['media', 'color', 'text'] as const)(
    'hides the default %s name without removing its accessible name',
    (kind) => {
      render(
        <div className='influencer-page'>
          <OptionCard
            categoryId='body_type'
            kind={kind}
            option={{
              id: 'feature',
              label_en: 'Feature',
              visibleIn: ['total'],
              imageUrl: '/feature.webp',
              swatch: '#fff',
            }}
            selected={false}
            onClick={() => {}}
          />
        </div>
      )
      const button = screen.getByRole('button', { name: 'Feature' })
      const label = within(button).getByText('Feature')
      expect(getComputedStyle(label).opacity).toBe('0')
      expect(getComputedStyle(label).backgroundImage).toBe('none')
      expect(getComputedStyle(label).backgroundColor).toBe('rgba(0, 0, 0, 0)')
      expect(label).toHaveTextContent('Feature')
    }
  )

  it.each(['tier', 'gender'])(
    'keeps the %s name visible without hovering',
    (categoryId) => {
      render(
        <div className='influencer-page'>
          <OptionCard
            categoryId={categoryId}
            kind='media'
            option={{ id: 'choice', label_en: 'Choice', visibleIn: ['total'] }}
            selected={false}
            onClick={() => {}}
          />
        </div>
      )
      const button = screen.getByRole('button', { name: 'Choice' })
      expect(getComputedStyle(within(button).getByText('Choice')).opacity).toBe(
        '1'
      )
      if (categoryId === 'gender') {
        expect(getComputedStyle(button).backgroundColor).toBe(
          'rgba(0, 0, 0, 0)'
        )
      }
    }
  )

  it.each(['age', 'skin_tone', 'eye_color', 'hair_colour'])(
    'keeps the %s name visible before and after selection without duplicating it',
    (categoryId) => {
      const option = {
        id: 'choice',
        label_en: 'Deep brown',
        visibleIn: ['total'],
        swatch: '#333',
      }
      const props = {
        categoryId,
        kind: categoryId === 'age' ? ('text' as const) : ('color' as const),
        option,
        onClick: () => {},
      }
      const { rerender } = render(
        <div className='influencer-page'>
          <OptionCard {...props} selected={false} />
        </div>
      )
      const button = screen.getByRole('button', { name: 'Deep brown' })
      expect(within(button).getAllByText('Deep brown')).toHaveLength(1)
      expect(
        getComputedStyle(within(button).getByText('Deep brown')).opacity
      ).toBe('1')
      rerender(
        <div className='influencer-page'>
          <OptionCard {...props} selected />
        </div>
      )
      expect(button).toHaveAttribute('aria-pressed', 'true')
      expect(
        getComputedStyle(within(button).getByText('Deep brown')).transform
      ).toBe('none')
    }
  )

  it('keeps a selected ordinary name hidden and allows keyboard selection', async () => {
    const onClick = vi.fn()
    const user = userEvent.setup()
    render(
      <div className='influencer-page'>
        <OptionCard
          categoryId='unknown_color_group'
          kind='color'
          option={{
            id: 'dark',
            label_en: 'Dark',
            visibleIn: ['total'],
            swatch: '#333',
          }}
          selected
          onClick={onClick}
        />
      </div>
    )
    const button = screen.getByRole('button', { name: 'Dark' })
    expect(button).toHaveAttribute('aria-pressed', 'true')
    expect(getComputedStyle(within(button).getByText('Dark')).opacity).toBe('0')
    await user.tab()
    expect(button).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onClick).toHaveBeenCalledOnce()
  })
})
