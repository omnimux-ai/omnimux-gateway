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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'

import { PROMPT_PREVIEW_COUNT } from '../../lib/filter'
import type { PromptItem } from '../../types'
import { ModelPromptsPage } from '../model-prompts-page'
import { PromptGallery } from '../prompt-gallery'

// The page chrome (site header, auth store, status query) is outside this
// feature; the drill-down contract lives in the page body.
vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
}))

/* ── A tiny catalog served through a mocked `fetch` ─────────────────────── */

const MODEL_KEY = 'seedance-2.5'
const MODEL_LABEL = 'Seedance 2.5'
// The gallery navigates by 专區, so its section carries the group's key and
// label while the model page and the cards keep naming the model.
const SECTION_KEY = 'seedance'
const SECTION_LABEL = 'Seedance zone'
const OTHER_KEY = 'gpt-image-2'
const SECOND_PAGE_SIZE = 6
const FIRST_PAGE_SIZE = 24

function makeItem(
  id: string,
  modelKey: string,
  modelLabel: string,
  overrides: Partial<PromptItem> = {}
): PromptItem {
  return {
    id,
    slug: id,
    modelKey,
    modelLabel,
    mediaType: 'video',
    title: `Prompt ${id}`,
    description: `Description ${id}`,
    prompt: `Prompt body ${id}`,
    category: 'cinematic-showcase',
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
    ...overrides,
  }
}

const FIRST_PAGE = Array.from({ length: FIRST_PAGE_SIZE }, (_, index) =>
  makeItem(`s${index}`, MODEL_KEY, MODEL_LABEL)
)
const SECOND_PAGE = Array.from({ length: SECOND_PAGE_SIZE }, (_, index) =>
  makeItem(`s${FIRST_PAGE_SIZE + index}`, MODEL_KEY, MODEL_LABEL)
)
const QUARANTINE_PAGE = Array.from({ length: 4 }, (_, index) =>
  makeItem(`q${index}`, MODEL_KEY, MODEL_LABEL)
)
const OTHER_PAGE = Array.from({ length: FIRST_PAGE_SIZE }, (_, index) =>
  makeItem(`g${index}`, OTHER_KEY, 'GPT Image 2', {
    mediaType: 'image',
    category: 'social-media-post',
  })
)

const MANIFEST = {
  version: 1,
  generatedAt: '2026-09-12T00:00:00.000Z',
  pageSize: FIRST_PAGE_SIZE,
  total: 42,
  quarantinedTotal: 4,
  sources: [{ id: 'loomi', rows: 42, quarantined: 4, dropped: 0 }],
  zones: [
    {
      id: MODEL_KEY,
      label: MODEL_LABEL,
      mediaType: 'video',
      count: 30,
      pages: 2,
      quarantined: 4,
      quarantinePages: 1,
      categories: ['cinematic-showcase', 'product-ad'],
      styles: ['cinematic-realistic'],
      subjects: ['character'],
      useCases: ['cinematic-showcase'],
    },
    {
      id: OTHER_KEY,
      label: 'GPT Image 2',
      mediaType: 'image',
      count: 12,
      pages: 1,
      quarantined: 0,
      quarantinePages: 0,
      categories: ['social-media-post'],
      styles: [],
      subjects: [],
      useCases: [],
    },
  ],
}

const CATALOG: Record<string, unknown> = {
  'manifest.json': MANIFEST,
  [`${MODEL_KEY}/page-0000.json`]: {
    zone: MODEL_KEY,
    page: 0,
    pageSize: FIRST_PAGE_SIZE,
    total: 30,
    totalPages: 2,
    items: FIRST_PAGE,
  },
  [`${MODEL_KEY}/page-0001.json`]: {
    zone: MODEL_KEY,
    page: 1,
    pageSize: FIRST_PAGE_SIZE,
    total: 30,
    totalPages: 2,
    items: SECOND_PAGE,
  },
  [`${MODEL_KEY}/quarantine-0000.json`]: {
    zone: MODEL_KEY,
    page: 0,
    pageSize: FIRST_PAGE_SIZE,
    total: 4,
    totalPages: 1,
    items: QUARANTINE_PAGE,
  },
  [`${OTHER_KEY}/page-0000.json`]: {
    zone: OTHER_KEY,
    page: 0,
    pageSize: FIRST_PAGE_SIZE,
    total: 12,
    totalPages: 1,
    items: OTHER_PAGE,
  },
}

/** Every catalog URL the pages requested, in order. */
let requested: string[] = []

function jsonResponse(payload: unknown): Response {
  return {
    ok: true,
    status: 200,
    json: async () => payload,
  } as unknown as Response
}

/**
 * Serve the fake catalog from `fetch`. A URL outside it answers `404`, and an
 * aborted request rejects the way a real one does, so the hooks' cancellation
 * path is exercised instead of assumed.
 */
function serveCatalog(): void {
  requested = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      requested.push(url)
      if (init?.signal?.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }
      const payload = CATALOG[url.replace('/inspiration-catalog/', '')]
      if (payload === undefined) {
        return { ok: false, status: 404, json: async () => ({}) } as Response
      }
      return jsonResponse(payload)
    })
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

/* ── Router harness ────────────────────────────────────────────────────── */

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
 * Model keys contain dots (`seedance-2.5`), which cannot be expressed as an
 * id selector, so the section is looked up by attribute like the anchor bar
 * does in production.
 */
function modelSection(key: string): HTMLElement {
  const section = document.querySelector<HTMLElement>(
    `[id="inspiration-model-${key}"]`
  )
  if (!section) throw new Error(`section ${key} not rendered`)
  return section
}

/** Cards only: the guide block below the grid also renders `<article>`s. */
async function gridCards(): Promise<HTMLElement[]> {
  const grid = await screen.findByTestId('model-prompt-grid')
  return within(grid).getAllByRole('article')
}

function renderModelListing(slug = MODEL_KEY) {
  renderAt(`/inspiration/models/${slug}`, <ModelPromptsPage modelSlug={slug} />)
}

/**
 * Report exactly one media query as matching, so the waterfall ladder can be
 * exercised without a real viewport. The shared test setup answers every query
 * with `matches: false`, which is the narrowest rung of the ladder.
 */
function matchOnlyViewport(query: string) {
  const originalMatchMedia = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((candidate) => ({
    ...originalMatchMedia(candidate),
    matches: candidate === query,
  }))
}

describe('L1 prompt gallery drill-down', () => {
  it('prints the manifest totals, not the loaded row count', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    const section = modelSection(SECTION_KEY)
    expect(
      within(section).getByText(`Top prompts · 30 total`)
    ).toBeInTheDocument()
    expect(within(section).getAllByRole('article')).toHaveLength(
      PROMPT_PREVIEW_COUNT
    )
  })

  it('loads the manifest once and one first page per zone', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    expect(requested).toEqual([
      '/inspiration-catalog/manifest.json',
      `/inspiration-catalog/${MODEL_KEY}/page-0000.json`,
      `/inspiration-catalog/${OTHER_KEY}/page-0000.json`,
    ])
  })

  it('prints the model total and links the section tail to the model listing', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    const viewAll = within(modelSection(SECTION_KEY)).getByRole('link', {
      name: `View all ${SECTION_LABEL} prompts`,
    })
    // A 专區 has no listing of its own yet, so the tail leads with the group's
    // first member zone instead of a slug no model page can resolve.
    expect(viewAll).toHaveAttribute(
      'href',
      `/inspiration/models/${MODEL_KEY}`
    )
  })

  it('renders no numeric pagination on the preview level', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    expect(screen.queryByLabelText('Previous page')).toBeNull()
    expect(screen.queryByLabelText('Next page')).toBeNull()
    expect(screen.queryByLabelText('Page number')).toBeNull()
  })

  it('lays the preview out as a waterfall on the denser L1 ladder', async () => {
    serveCatalog()
    matchOnlyViewport('(min-width: 1024px)')
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    const section = modelSection(SECTION_KEY)
    const grid = section.querySelector<HTMLElement>(
      '[data-testid="prompt-section-grid"]'
    )
    if (!grid) throw new Error('section grid not rendered')

    // The aspect-driven tile rules out a CSS grid row: the batch is cut into
    // waterfall columns at the level's own ladder instead.
    expect(grid.className).toContain('items-start')
    expect(grid.className).toContain('gap-3')

    const columns = [...grid.children]
    expect(columns).toHaveLength(5)
    for (const column of columns) {
      expect(column.className).toContain('min-w-0')
      expect(column.className).toContain('flex-1')
    }

    expect(within(grid).getAllByRole('article')).toHaveLength(
      PROMPT_PREVIEW_COUNT
    )
  })

  it('opens the detail layer from a section tile', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    const tile = modelSection(SECTION_KEY).querySelector('article')
    if (!tile) throw new Error('section tile not rendered')

    // The tile is the browsing surface only: one control, no author link and
    // no media-type line of its own.
    expect(within(tile).getAllByRole('button')).toHaveLength(1)
    await userEvent.click(within(tile).getByRole('button'))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('centres the "View all" pill on a bottom fade that lets card clicks through', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    const section = modelSection(SECTION_KEY)
    // The simplified tail link is gone: the section is header + grid/fade only.
    expect(section.children).toHaveLength(2)

    const viewAll = within(section).getByRole('link', {
      name: `View all ${SECTION_LABEL} prompts`,
    })

    const fade = viewAll.parentElement
    if (!fade) throw new Error('fade wrapper not rendered')

    expect(fade.className).toContain('pointer-events-none')
    expect(fade.className).toContain('absolute inset-x-0 bottom-0')
    expect(fade).toHaveClass('justify-center', 'pt-16', 'pb-1')
    expect(fade.className).toContain('from-background')
    expect(fade.className).toContain('via-background/85')
    expect(fade.className).toContain('to-transparent')
    expect(viewAll.className).toContain('pointer-events-auto')
  })

  it('offers the style and subject facet options of the zone vocabulary from the manifest', async () => {
    serveCatalog()
    renderAt('/inspiration', <PromptGallery />)
    await screen.findByRole('heading', { name: SECTION_LABEL })

    // Style and Subject facet rows are offered directly on the clean page.
    expect(screen.getByText('Style')).toBeInTheDocument()
    expect(screen.getByText('Subject')).toBeInTheDocument()
  })
})

describe('L2 model prompt listing', () => {
  it('renders the breadcrumb, heading and prompt count from the manifest', async () => {
    serveCatalog()
    renderModelListing()

    const breadcrumb = await screen.findByRole('navigation', {
      name: 'breadcrumb',
    })
    expect(within(breadcrumb).getByText('Home')).toHaveAttribute('href', '/')
    expect(within(breadcrumb).getByText('Prompt inspiration')).toHaveAttribute(
      'href',
      '/inspiration'
    )
    expect(within(breadcrumb).getByText(MODEL_LABEL)).toHaveAttribute(
      'aria-current',
      'page'
    )
    // Only the last trail item is the current page; the middle level is a link
    // back to L1 and must not claim `aria-current` as an active ancestor.
    expect(breadcrumb.querySelectorAll('[aria-current="page"]')).toHaveLength(1)

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: `${MODEL_LABEL} Prompts: 30+ Examples & Guide`,
      })
    ).toBeInTheDocument()
    expect(screen.getByText('30 prompts')).toBeInTheDocument()
    // The segmented L1 tab switcher (Prompt inspiration / Marketing Studio) is omitted in L2.
    expect(
      screen.queryByRole('navigation', { name: 'Inspiration Community' })
    ).toBeNull()
  })

  it('shows only the first loaded page and no loaded-row bookkeeping', async () => {
    serveCatalog()
    renderModelListing()

    expect(await gridCards()).toHaveLength(FIRST_PAGE_SIZE)
  })

  it('fetches the next zone page when Load more is activated', async () => {
    serveCatalog()
    renderModelListing()

    await userEvent.click(await screen.findByRole('button', { name: 'Load more' }))

    expect(await gridCards()).toHaveLength(
      FIRST_PAGE_SIZE + SECOND_PAGE_SIZE
    )
    expect(requested).toContain(
      `/inspiration-catalog/${MODEL_KEY}/page-0001.json`
    )
    expect(await gridCards()).toHaveLength(30)
    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull()
    expect(
      screen.getByText('You have reached the end of this library.')
    ).toBeInTheDocument()
  })

  it('keeps the whole batch in one waterfall column below the md breakpoint', async () => {
    serveCatalog()
    renderModelListing()

    const grid = await screen.findByTestId('model-prompt-grid')

    expect(grid.className).toContain('items-start')
    expect(grid.children).toHaveLength(1)
    expect(grid.children[0].children).toHaveLength(FIRST_PAGE_SIZE)
  })

  it('cuts the batch into four waterfall columns at the 2xl breakpoint', async () => {
    serveCatalog()
    matchOnlyViewport('(min-width: 1536px)')
    renderModelListing()

    const grid = await screen.findByTestId('model-prompt-grid')
    const columns = [...grid.children]

    expect(columns).toHaveLength(4)
    for (const column of columns) {
      expect(column.className).toContain('min-w-0')
      expect(column.className).toContain('flex-1')
      expect(column.children).toHaveLength(FIRST_PAGE_SIZE / 4)
    }
  })

  it('cuts the batch into three waterfall columns at the xl breakpoint', async () => {
    serveCatalog()
    matchOnlyViewport('(min-width: 1280px)')
    renderModelListing()

    const grid = await screen.findByTestId('model-prompt-grid')

    expect(grid.children).toHaveLength(3)
    expect(grid.children[0].children).toHaveLength(
      Math.ceil(FIRST_PAGE_SIZE / 3)
    )
  })

  it('filters the listing by the search box', async () => {
    serveCatalog()
    renderModelListing()

    const target = FIRST_PAGE[0]
    if (!target) throw new Error('fixture page is empty')

    fireEvent.change(
      await screen.findByLabelText('Search inspiration prompts'),
      { target: { value: target.title } }
    )

    expect(await gridCards()).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: target.title })
    ).toBeInTheDocument()
  })

  it('shows the empty state when the search matches nothing', async () => {
    serveCatalog()
    renderModelListing()

    await userEvent.type(
      await screen.findByLabelText('Search inspiration prompts'),
      'zzzz-not-a-prompt'
    )

    expect(screen.queryByTestId('model-prompt-grid')).toBeNull()
    expect(
      screen.getByText('No prompts match these filters')
    ).toBeInTheDocument()
  })

  it('offers only the media shortcut the library can actually serve', async () => {
    serveCatalog()
    renderModelListing()

    expect(
      await screen.findByRole('button', { name: 'All Video Prompts' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'All Image Prompts' })
    ).not.toBeInTheDocument()
  })

  it('narrows the listing to one category chip', async () => {
    serveCatalog()
    renderModelListing()

    await screen.findByTestId('model-prompt-grid')
    await userEvent.click(
      screen.getByRole('button', { name: 'Cinematic showcase' })
    )

    // Every fixture row carries the facet, so the count is unchanged; the point
    // is that the chip narrows the loaded rows instead of requesting the zone.
    expect(await gridCards()).toHaveLength(FIRST_PAGE_SIZE)
    expect(requested).not.toContain(
      `/inspiration-catalog/${MODEL_KEY}/page-0001.json`
    )
  })

  it('never requests the quarantined pages on a normal visit', async () => {
    serveCatalog()
    renderModelListing()

    await screen.findByTestId('model-prompt-grid')

    expect(requested.some((url) => url.includes('quarantine'))).toBe(false)
    expect(
      screen.queryByText('Quarantine comparison mode')
    ).not.toBeInTheDocument()
  })

  it('loads the quarantined rows only in comparison mode', async () => {
    serveCatalog()
    renderAt(
      `/inspiration/models/${MODEL_KEY}`,
      <ModelPromptsPage modelSlug={MODEL_KEY} includeQuarantined />
    )

    expect(await gridCards()).toHaveLength(QUARANTINE_PAGE.length)
    expect(requested).toContain(
      `/inspiration-catalog/${MODEL_KEY}/quarantine-0000.json`
    )
    expect(screen.getByText('Quarantine comparison mode')).toBeInTheDocument()
    expect(
      screen.getByRole('heading', {
        level: 1,
        name: `${MODEL_LABEL} Prompts: 4+ Examples & Guide`,
      })
    ).toBeInTheDocument()
  })

  it('explains the audit mode in the banner: what it is, why, and how to leave', async () => {
    serveCatalog()
    renderAt(
      `/inspiration/models/${MODEL_KEY}`,
      <ModelPromptsPage modelSlug={MODEL_KEY} includeQuarantined />
    )

    const banner = await screen.findByTestId('quarantine-banner')

    expect(
      within(banner).getByText(
        'Quarantine is an audit view: it lists rows the catalog holds back from normal browsing, and a regular visit never requests them.'
      )
    ).toBeInTheDocument()
    expect(
      within(banner).getByText(
        'A row is held back when its poster cannot be fetched upstream, so its card has no media to render. The row itself is intact, which is why it is kept for comparison instead of being dropped.'
      )
    ).toBeInTheDocument()
    expect(
      within(banner).getByText(
        'None of these 4 rows is counted in the default view: this mode is the only place they appear.'
      )
    ).toBeInTheDocument()
    expect(
      within(banner).getByRole('link', { name: 'Back to the browsable prompts' })
    ).toHaveAttribute('href', `/inspiration/models/${MODEL_KEY}`)
  })

  it('keeps the held-back line off the default listing', async () => {
    serveCatalog()
    renderModelListing(OTHER_KEY)

    expect(await gridCards()).toHaveLength(OTHER_PAGE.length)
    expect(
      screen.queryByText(/rows are held back from this library/)
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'Compare held-back rows' })
    ).not.toBeInTheDocument()
  })

  it('reports an unknown model slug instead of an empty grid', async () => {
    serveCatalog()
    renderModelListing('kling-3.0')

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Model not found' })
    ).toBeInTheDocument()
    expect(screen.queryByTestId('model-prompt-grid')).toBeNull()
    expect(
      screen.getByRole('button', { name: 'Back to prompt inspiration' })
    ).toHaveAttribute('href', '/inspiration')
    // An unknown slug must not request a page that cannot exist.
    expect(requested).toEqual(['/inspiration-catalog/manifest.json'])
  })

  it('surfaces a page failure with a retry action', async () => {
    serveCatalog()
    vi.mocked(fetch).mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      requested.push(url)
      if (url.endsWith('manifest.json')) return jsonResponse(MANIFEST)
      return { ok: false, status: 500, json: async () => ({}) } as Response
    })
    renderModelListing()

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })
})
