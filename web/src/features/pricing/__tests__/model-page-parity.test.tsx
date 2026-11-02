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
import { render } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { ReactElement } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { ModelPageAnchorNav } from '@/features/model-playground/components/model-page-anchor-nav'
import { ModelPlaygroundSection } from '@/features/model-playground/components/model-playground-section'

import { ModelHeader, ModelDetailsContent } from '../components/model-details'
import { ModelPagePricingPlan } from '../components/model-page-pricing-plan'
import type { PricingModel } from '../types'

// `model-details.tsx` reaches the chart stack, whose deep ESM entry points do
// not resolve under vitest. Stubbed exactly as the existing pricing tests do,
// so the header can still be verified in the DOM.
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

/**
 * Rendered-DOM half of the parity gate.
 *
 * `scripts/dev/check-parity-contracts.mjs` proves an id is *declared* somewhere
 * in our sources; this test proves it actually reaches the DOM. The distinction
 * matters: a parity id can sit in a lookup map, behind a condition that never
 * holds, or in a component nobody renders.
 */
const CHECKLIST_PATH = resolve(
  __dirname,
  '../../../../../docs/product/model-page-parity-checklist.json'
)

type Checklist = {
  checks: { id: string; area: string; compare: string; expect?: string }[]
}

const checklist = JSON.parse(
  readFileSync(CHECKLIST_PATH, 'utf8')
) as Checklist

/** Ids each component under test must render, from the shared checklist. */
const EXPECTED_BY_COMPONENT: Record<string, string[]> = {
  header: [
    'header.breadcrumb',
    'header.title',
    'header.summary',
    'header.meta',
    'capabilities.chips',
    'uptime.title',
    'uptime.badge',
    'uptime.note',
    'facts.input',
    'facts.output',
  ],
  nav: ['shell.anchor-nav'],
  playground: [
    'playground.intro.title',
    'playground.intro.desc',
    'playground.mode.switch',
    'playground.form.prompt',
    'playground.form.resolution',
    'playground.footer.reset',
    'playground.footer.submit',
    'playground.preview.switch',
    'playground.preview.empty',
    'playground.history.title',
    'playground.history.retention-ours',
    'playground.history.empty',
  ],
  pricing: [
    'pricing.intro.title',
    'pricing.intro.desc',
    'pricing.first-test.title',
    'pricing.first-test.card',
    'pricing.budget.title',
    'pricing.budget.rows',
  ],
}

function imageModel(): PricingModel {
  return {
    id: 1,
    model_name: 'gpt-image-2.5',
    description: 'Text-to-image and image editing.',
    vendor_name: 'OpenAI',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.01,
    enable_groups: ['default'],
    input_modalities: ['text', 'image'],
    output_modalities: ['image'],
  }
}

function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  )
}

function parityIds(container: HTMLElement): string[] {
  return [...container.querySelectorAll('[data-parity]')].map(
    (element) => element.getAttribute('data-parity') ?? ''
  )
}

describe('model page parity (rendered DOM)', () => {
  it('renders every header parity module for an image model', () => {
    const { container } = renderWithProviders(
      <ModelHeader
        model={imageModel()}
        priceRate={1}
        usdExchangeRate={1}
        showRechargePrice={false}
      />
    )

    expect(parityIds(container)).toEqual(
      expect.arrayContaining(EXPECTED_BY_COMPONENT.header)
    )
  })

  it('renders the anchor nav with one link per page section', () => {
    const { container } = renderWithProviders(
      <ModelPageAnchorNav activeId='playground' />
    )

    expect(parityIds(container)).toContain('shell.anchor-nav')
    for (const section of ['playground', 'pricing', 'about', 'api']) {
      expect(
        container.querySelector(`a[href="#${section}"]`)
      ).not.toBeNull()
    }
  })

  it('renders the playground section modules', () => {
    const { container } = renderWithProviders(
      <ModelPlaygroundSection model={imageModel()} authenticated={false} />
    )

    expect(parityIds(container)).toEqual(
      expect.arrayContaining(EXPECTED_BY_COMPONENT.playground)
    )
  })

  it('renders the pricing plan blocks for a priceable model', () => {
    const { container } = renderWithProviders(
      <ModelPagePricingPlan
        model={imageModel()}
        priceRate={1}
        usdExchangeRate={1}
      />
    )

    expect(parityIds(container)).toEqual(
      expect.arrayContaining(EXPECTED_BY_COMPONENT.pricing)
    )
  })

  it('assembles all four anchored sections with working anchor targets', () => {
    const { container } = renderWithProviders(
      <ModelDetailsContent
        model={imageModel()}
        groupRatio={{ default: 1 }}
        usableGroup={{ default: 'Default' }}
        endpointMap={{}}
        autoGroups={[]}
        priceRate={1}
        usdExchangeRate={1}
        tokenUnit='M'
      />
    )

    // Sections own the page-level parity ids that no single sub-component can
    // cover, so they are asserted here rather than in the per-component tests.
    const ids = parityIds(container)
    expect(ids).toContain('shell.section-separator')
    expect(ids).toEqual(
      expect.arrayContaining([
        'pricing.model-pricing.title',
        'pricing.model-pricing.table',
        'pricing.model-pricing.note',
      ])
    )

    for (const section of ['playground', 'pricing', 'about', 'api']) {
      expect(container.querySelector(`#${section}`)).not.toBeNull()
    }

    // Every in-page anchor must point at a section that exists, otherwise the
    // nav silently scrolls nowhere — the failure this restructure fixed.
    for (const link of container.querySelectorAll('a[href^="#"]')) {
      const target = link.getAttribute('href')?.slice(1)
      expect(target && container.querySelector(`#${target}`)).toBeTruthy()
    }
  })

  it('never renders a string the checklist forbids', () => {
    const forbidden = checklist.checks
      .filter((check) => check.compare === 'forbid')
      .map((check) => /MUST NOT appear:\s*(.+)$/.exec(check.expect ?? '')?.[1])
      .filter((needle): needle is string => Boolean(needle))

    expect(forbidden.length).toBeGreaterThan(0)

    const { container } = renderWithProviders(
      <>
        <ModelPlaygroundSection model={imageModel()} authenticated={false} />
        <ModelPagePricingPlan
          model={imageModel()}
          priceRate={1}
          usdExchangeRate={1}
        />
      </>
    )

    for (const needle of forbidden) {
      expect(container.textContent ?? '').not.toContain(needle)
    }
  })
})
