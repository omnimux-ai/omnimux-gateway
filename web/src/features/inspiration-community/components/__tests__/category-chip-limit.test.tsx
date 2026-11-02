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
/**
 * The category facet row is capped on both levels of the prompt library.
 *
 * A merged-catalog zone carries 61 free-text category values (measured on
 * `seedance-2.0`), which rendered as one 62-chip row and a 5,600 px page. The
 * contract these tests pin: the row shows `FACET_CHIP_LIMIT` chips, the rest
 * are one in-place click away, and the category currently selected stays on
 * screen even when the cap would have hidden it — otherwise a visitor is
 * looking at a filtered listing with no visible way to undo the filter.
 */
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { FACET_CHIP_LIMIT } from '../../lib/catalog'
import type { PromptItem } from '../../types'
import { ModelPromptsPage } from '../model-prompts-page'

// The page chrome (site header, auth store, status query) is outside this
// feature; the cap contract lives in the page body.
vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
}))

const MODEL_KEY = 'seedance-2.0'
const MODEL_LABEL = 'Seedance 2.0'
const CATEGORY_COUNT = 61

/** `category-01` … `category-61`, as the manifest emits them by coverage. */
const CATEGORIES = Array.from(
  { length: CATEGORY_COUNT },
  (_, index) => `category-${String(index + 1).padStart(2, '0')}`
)
/** `category-61` -> `Category 61`, the label `facetDisplayName` falls back to. */
function categoryLabel(value: string): string {
  const words = value.replaceAll(/[_-]+/gu, ' ').trim()
  return words.charAt(0).toUpperCase() + words.slice(1)
}
const LAST_CATEGORY = CATEGORIES[CATEGORY_COUNT - 1]
const LAST_CATEGORY_LABEL = categoryLabel(LAST_CATEGORY)
const EXPAND_LABEL = `Show all ${CATEGORY_COUNT} categories`

function makeItem(id: string, category: string): PromptItem {
  return {
    id,
    slug: id,
    modelKey: MODEL_KEY,
    modelLabel: MODEL_LABEL,
    mediaType: 'video',
    title: `Prompt ${id}`,
    description: `Description ${id}`,
    prompt: `Prompt body ${id}`,
    category,
    useCases: ['cinematic-showcase'],
    styles: ['cinematic-realistic'],
    tags: ['cinematic-showcase'],
    previewImageUrl: `https://cdn.example.com/${id}.jpg`,
    previewVideoUrl: `https://cdn.example.com/${id}.mp4`,
    posterUrl: `https://cdn.example.com/${id}.jpg`,
    aspectRatio: '9 / 16',
    authorName: 'Loomi',
    sourcePlatform: 'x',
    sourceUrl: 'https://x.com/example/status/1',
    publishedAt: '2026-09-11T06:35:58',
    viewCount: 0,
    originalViewCount: 0,
  }
}

/** Rows covering the categories so the 61-category limit can be exercised with matching data. */
const PAGE_ITEMS = CATEGORIES.map((cat, index) => makeItem(`s${index}`, cat))

const MANIFEST = {
  version: 1,
  generatedAt: '2026-09-12T00:00:00.000Z',
  pageSize: 48,
  total: PAGE_ITEMS.length,
  quarantinedTotal: 0,
  sources: [
    { id: 'loomi', rows: PAGE_ITEMS.length, quarantined: 0, dropped: 0 },
  ],
  zones: [
    {
      id: MODEL_KEY,
      label: MODEL_LABEL,
      mediaType: 'video',
      count: PAGE_ITEMS.length,
      pages: 1,
      quarantined: 0,
      quarantinePages: 0,
      categories: CATEGORIES,
      styles: ['cinematic-realistic'],
      subjects: ['character'],
      useCases: ['cinematic-showcase'],
    },
  ],
}

const CATALOG: Record<string, unknown> = {
  'manifest.json': MANIFEST,
  [`${MODEL_KEY}/page-0000.json`]: {
    zone: MODEL_KEY,
    page: 0,
    pageSize: 48,
    total: PAGE_ITEMS.length,
    totalPages: 1,
    items: PAGE_ITEMS,
  },
}

/** Serve the fake catalog from `fetch`; anything outside it answers `404`. */
function serveCatalog(): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const payload =
        CATALOG[String(input).replace('/inspiration-catalog/', '')]
      if (payload === undefined) {
        return { ok: false, status: 404, json: async () => ({}) } as Response
      }
      return { ok: true, status: 200, json: async () => payload } as Response
    })
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

function renderAt(path: string, element: ReactElement) {
  const root = createRootRoute()
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => null,
  })
  const inspiration = createRoute({
    getParentRoute: () => root,
    path: '/inspiration',
    component: () => element,
  })
  const modelPrompts = createRoute({
    getParentRoute: () => root,
    path: '/inspiration/models/$modelSlug',
    component: () => element,
  })
  const router = createRouter({
    routeTree: root.addChildren([home, inspiration, modelPrompts]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })

  render(
    <TooltipProvider>
      <RouterProvider router={router} />
    </TooltipProvider>
  )
  return router
}

/**
 * The chips inside the category group. Chips are single-choice toggle
 * buttons (`aria-pressed`); the expand/collapse control beside them carries
 * `aria-expanded` instead, so it is not counted as a chip.
 */
function categoryChips(group: HTMLElement): HTMLElement[] {
  return within(group)
    .getAllByRole('button')
    .filter((button) => button.hasAttribute('aria-pressed'))
}

async function renderCategoryGroup(): Promise<HTMLElement> {
  renderAt(
    `/inspiration/models/${MODEL_KEY}`,
    <ModelPromptsPage modelSlug={MODEL_KEY} />
  )
  return screen.findByRole('group', { name: 'Category' })
}

describe('L2 category chip cap', () => {
  it('shows only the capped chips until the visitor asks for the rest', async () => {
    serveCatalog()
    const group = await renderCategoryGroup()

    // The capped chips plus the `All` chip that clears the filter.
    expect(categoryChips(group)).toHaveLength(FACET_CHIP_LIMIT + 1)
    expect(
      within(group).getByRole('button', { name: EXPAND_LABEL })
    ).toHaveAttribute('aria-expanded', 'false')
    expect(
      within(group).queryByRole('button', {
        name: categoryLabel(CATEGORIES[FACET_CHIP_LIMIT]),
      })
    ).not.toBeInTheDocument()
  })

  it('reveals every category chip on expand and hides them again on collapse', async () => {
    serveCatalog()
    const group = await renderCategoryGroup()

    await userEvent.click(
      within(group).getByRole('button', { name: EXPAND_LABEL })
    )

    expect(categoryChips(group)).toHaveLength(CATEGORY_COUNT + 1)
    expect(
      within(group).getByRole('button', { name: LAST_CATEGORY_LABEL })
    ).toBeInTheDocument()
    expect(
      within(group).getByRole('button', { name: 'Show fewer' })
    ).toHaveAttribute('aria-expanded', 'true')

    await userEvent.click(
      within(group).getByRole('button', { name: 'Show fewer' })
    )

    expect(categoryChips(group)).toHaveLength(FACET_CHIP_LIMIT + 1)
  })

  it('keeps the selected chip on screen after the row collapses', async () => {
    serveCatalog()
    const group = await renderCategoryGroup()

    await userEvent.click(
      within(group).getByRole('button', { name: EXPAND_LABEL })
    )
    await userEvent.click(
      within(group).getByRole('button', { name: LAST_CATEGORY_LABEL })
    )
    await userEvent.click(
      within(group).getByRole('button', { name: 'Show fewer' })
    )

    const selected = within(group).getByRole('button', {
      name: LAST_CATEGORY_LABEL,
    })
    expect(selected).toHaveAttribute('aria-pressed', 'true')
    // The capped chips, the `All` chip, and the selected chip the cap would
    // otherwise have hidden.
    expect(categoryChips(group)).toHaveLength(FACET_CHIP_LIMIT + 2)
  })
})
