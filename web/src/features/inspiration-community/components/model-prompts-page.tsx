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
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronDownIcon, ChevronUpIcon, SearchIcon } from 'lucide-react'
import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import {
  useCatalogZoneRows,
  findCatalogZone,
} from '../hooks/use-inspiration-catalog'
import { useInspirationManifest } from '../hooks/use-inspiration-manifest'
import {
  DEFAULT_CATALOG_PAGE_SIZE,
  catalogCategories,
  catalogSections,
  FACET_CHIP_LIMIT,
  visibleFacetValues,
  zoneRowCount,
} from '../lib/catalog'
import { facetDisplayName } from '../lib/facets'
import {
  FILTER_ALL,
  filterPromptItems,
  filterPromptItemsByQuery,
  resolveModelSection,
  sortPromptItems,
  type PromptFilterState,
} from '../lib/filter'
import { formatCount } from '../lib/format'
import { resolveInspirationPlaygroundModelId } from '../lib/model-mapping'
import {
  LISTING_COLUMN_STEPS,
  splitIntoColumns,
  useWaterfallColumnCount,
} from '../lib/waterfall'
import type { PromptItem, PromptMediaType } from '../types'
import { InspirationShell } from './inspiration-shell'
import { PromptPreviewModal } from './prompt-preview-modal'
import { PromptWaterfall } from './prompt-waterfall'

export interface ModelPromptsPageProps {
  /** `$modelSlug` path segment, e.g. `seedance-2.5`. */
  modelSlug: string
  /**
   * `?quarantine=1` — browse the quarantined rows instead of the displayable
   * ones, for data comparison only. Off by default: those rows are excluded
   * from the catalog totals and are never requested by a normal visit.
   */
  includeQuarantined?: boolean
}

/**
 * Page A, level 2 — the full prompt library of one model
 * (`/inspiration/models/$modelSlug`, PRD section 3.4).
 *
 * The visitor arrives here from an L1 "View all" link. This level owns the
 * real work: free-text search, category chips, a media-type shortcut row, and
 * a batched list revealed by "Load more".
 *
 * The rows come from the static catalog: the manifest supplies the model
 * label, the library size in the header and the category chips, while the
 * listing itself holds only the pages fetched so far. Search, category and
 * media type therefore narrow the *loaded* rows — "Load more" is what widens
 * the set — which is why the status line names both numbers.
 */
export function ModelPromptsPage(props: ModelPromptsPageProps) {
  const { t, i18n } = useTranslation()
  const quarantined = props.includeQuarantined === true

  const manifestState = useInspirationManifest()
  const manifest = manifestState.manifest

  const sections = useMemo(
    () => (manifest ? catalogSections(manifest) : []),
    [manifest]
  )
  const section = useMemo(
    () => resolveModelSection(sections, props.modelSlug),
    [sections, props.modelSlug]
  )
  const zone = useMemo(
    () => findCatalogZone(manifest, section?.key ?? null),
    [manifest, section]
  )

  const rows = useCatalogZoneRows(zone?.id ?? null, { quarantined })
  const loadMore = rows.loadMore
  const reloadRows = rows.reload

  const [query, setQuery] = useState('')
  const [category, setCategory] = useState<string>(FILTER_ALL)
  const [mediaType, setMediaType] = useState<
    PromptMediaType | typeof FILTER_ALL
  >(FILTER_ALL)
  const [preview, setPreview] = useState<{
    items: PromptItem[]
    index: number
  } | null>(null)
  // The detail layer walks the tiles the visitor actually sees: the filtered
  // list travels with the click, so the counter matches the page.
  const openPreview = useCallback((item: PromptItem, items: PromptItem[]) => {
    setPreview({ items, index: Math.max(0, items.indexOf(item)) })
  }, [])
  const [showAllCategories, setShowAllCategories] = useState(false)

  // Chips reflect the categories that actually exist in the currently loaded dataset,
  // guaranteeing that clicking any category chip will find real matching creatives.
  const categories = useMemo(() => {
    if (!rows.items || rows.items.length === 0) {
      return zone ? catalogCategories([zone]) : []
    }
    const present = new Set<string>()
    for (const item of rows.items) {
      if (item.category && item.category !== FILTER_ALL) {
        present.add(item.category)
      }
    }
    if (zone?.categories) {
      const filtered = zone.categories.filter((c) => present.has(c))
      for (const c of present) {
        if (!filtered.includes(c)) filtered.push(c)
      }
      return filtered
    }
    return [...present]
  }, [rows.items, zone])

  useEffect(() => {
    if (
      category !== FILTER_ALL &&
      categories.length > 0 &&
      !categories.includes(category)
    ) {
      setCategory(FILTER_ALL)
    }
  }, [categories, category])

  // A merged-catalog zone can carry 61 free-text category values, which is a
  // wall of chips rather than a filter row. The row is capped and the rest is
  // one click away; the selected chip is always rendered, so a visitor can see
  // and undo the filter they are looking at.
  const visibleCategories = useMemo(
    () =>
      showAllCategories
        ? categories
        : visibleFacetValues(categories, FACET_CHIP_LIMIT, category),
    [categories, category, showAllCategories]
  )

  const matched = useMemo(() => {
    const filters: PromptFilterState = { category, mediaType }
    const scoped = filterPromptItems(rows.items, filters)
    return sortPromptItems(filterPromptItemsByQuery(scoped, query), 'latest')
  }, [rows.items, category, mediaType, query])

  let navigate: ReturnType<typeof useNavigate> | null = null
  try {
    navigate = useNavigate()
  } catch {
    navigate = null
  }

  const handleUsePrompt = useCallback(
    (item: PromptItem) => {
      setPreview(null)
      const targetModelId = resolveInspirationPlaygroundModelId(
        item.modelKey,
        item.mediaType
      )
      if (navigate) {
        try {
          navigate({
            to: '/pricing/$modelId',
            params: { modelId: targetModelId },
            search: { prompt: item.prompt },
          })
          return
        } catch {
          // Fall back to window.location
        }
      }
      if (typeof window !== 'undefined') {
        window.location.assign(
          `/pricing/${encodeURIComponent(targetModelId)}?prompt=${encodeURIComponent(item.prompt)}`
        )
      }
    },
    [navigate]
  )

  const resetFilters = useCallback(() => {
    setQuery('')
    setCategory(FILTER_ALL)
    setMediaType(FILTER_ALL)
  }, [])

  if (manifestState.error) {
    return (
      <InspirationShell
        active='prompts'
        showSectionNav={false}
        title={t('Prompt inspiration')}
        subtitle={t(
          'Reusable prompts and finished results from real creators, organized by model.'
        )}
        stats={[]}
      >
        <ErrorState
          className='mt-10'
          title={t('Failed to load')}
          description={t(
            'The prompt catalog could not be loaded. Retry, or come back in a moment.'
          )}
          onRetry={manifestState.reload}
        />
      </InspirationShell>
    )
  }

  if (manifestState.loading) {
    return (
      <InspirationShell
        active='prompts'
        showSectionNav={false}
        title={t('Prompt inspiration')}
        subtitle={t(
          'Reusable prompts and finished results from real creators, organized by model.'
        )}
        stats={[]}
      >
        <div className='pt-8'>
          <ModelPromptsSkeleton />
        </div>
      </InspirationShell>
    )
  }

  if (!section || !zone) {
    return (
      <InspirationShell
        active='prompts'
        showSectionNav={false}
        title={t('Model not found')}
        subtitle={t(
          'This model is not in the prompt library yet. Browse the models that are.'
        )}
        stats={[]}
        breadcrumb={<ModelBreadcrumb label={t('Model not found')} />}
      >
        <EmptyState
          className='mt-10'
          bordered
          title={t('Model not found')}
          description={t(
            'The link may be out of date. Every model with a library is listed on the prompt inspiration page.'
          )}
          action={
            <Button
              variant='outline'
              size='lg'
              className='rounded-full'
              render={<Link to='/inspiration' />}
            >
              {t('Back to prompt inspiration')}
            </Button>
          }
        />
      </InspirationShell>
    )
  }

  // Every count on the page describes the set being browsed: the manifest's
  // library size by default, the quarantined set in comparison mode.
  const totalLabel = formatCount(zoneRowCount(zone, quarantined))

  return (
    <InspirationShell
      active='prompts'
      showSectionNav={false}
      title={t('{{model}} Prompts: {{count}}+ Examples & Guide', {
        model: section.label,
        count: totalLabel,
      })}
      subtitle={t('{{count}} prompts', { count: totalLabel })}
      stats={[]}
      breadcrumb={<ModelBreadcrumb label={section.label} />}
    >
      <div className='flex flex-col gap-5 pt-8'>
        {quarantined && (
          <div
            className='border-border bg-muted/40 flex flex-col gap-1.5 rounded-2xl border p-4'
            data-testid='quarantine-banner'
          >
            <p className='text-sm font-semibold'>
              {t('Quarantine comparison mode')}
            </p>
            <p className='text-muted-foreground text-sm leading-relaxed text-pretty'>
              {t(
                'Quarantine is an audit view: it lists rows the catalog holds back from normal browsing, and a regular visit never requests them.'
              )}
            </p>
            <p className='text-muted-foreground text-sm leading-relaxed text-pretty'>
              {t(
                'A row is held back when its poster cannot be fetched upstream, so its card has no media to render. The row itself is intact, which is why it is kept for comparison instead of being dropped.'
              )}
            </p>
            <p className='text-muted-foreground text-sm leading-relaxed text-pretty'>
              {t(
                'None of these {{count}} rows is counted in the default view: this mode is the only place they appear.',
                { count: formatCount(zone.quarantined) }
              )}
            </p>
            <Link
              to='/inspiration/models/$modelSlug'
              params={{ modelSlug: section.key }}
              className='text-foreground mt-1 self-start text-sm font-medium underline underline-offset-4'
            >
              {t('Back to the browsable prompts')}
            </Link>
          </div>
        )}

        <div className='flex flex-col gap-3 lg:flex-row lg:items-center'>
          <div className='relative w-full lg:max-w-sm'>
            <label htmlFor='model-prompt-search' className='sr-only'>
              {t('Search inspiration prompts')}
            </label>
            <SearchIcon
              aria-hidden='true'
              className='text-muted-foreground pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2'
            />
            <Input
              id='model-prompt-search'
              type='search'
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('Search inspiration prompts')}
              className='h-10 rounded-full ps-9'
            />
          </div>

          <div
            role='group'
            aria-label={t('Media type')}
            className='flex flex-wrap items-center gap-2 lg:ms-auto'
          >
            <QuickLink
              active={mediaType === FILTER_ALL}
              onClick={() => setMediaType(FILTER_ALL)}
            >
              {t('All prompts')}
            </QuickLink>
            {/* A library holds one media type, so offering the other one is a
                shortcut to an empty page. */}
            {zone.mediaType === 'image' && (
              <QuickLink
                active={mediaType === 'image'}
                onClick={() => setMediaType('image')}
              >
                {t('All Image Prompts')}
              </QuickLink>
            )}
            {zone.mediaType === 'video' && (
              <QuickLink
                active={mediaType === 'video'}
                onClick={() => setMediaType('video')}
              >
                {t('All Video Prompts')}
              </QuickLink>
            )}
          </div>
        </div>

        <div
          role='group'
          aria-label={t('Category')}
          className='flex flex-wrap gap-2'
        >
          <CategoryChip
            active={category === FILTER_ALL}
            onClick={() => setCategory(FILTER_ALL)}
          >
            {t('All')}
          </CategoryChip>
          {visibleCategories.map((entry) => (
            <CategoryChip
              key={entry}
              active={category === entry}
              onClick={() => setCategory(entry)}
            >
              {facetDisplayName(entry, i18n.language)}
            </CategoryChip>
          ))}
          {categories.length > FACET_CHIP_LIMIT && (
            <Button
              variant='outline'
              size='sm'
              aria-expanded={showAllCategories}
              onClick={() => setShowAllCategories((prev) => !prev)}
              className='gap-1 rounded-full px-3 font-medium'
            >
              {showAllCategories ? (
                <>
                  <ChevronUpIcon aria-hidden='true' className='size-3.5' />
                  {t('Show fewer')}
                </>
              ) : (
                <>
                  <ChevronDownIcon aria-hidden='true' className='size-3.5' />
                  {t('Show all {{count}} categories', {
                    count: formatCount(categories.length),
                  })}
                </>
              )}
            </Button>
          )}
        </div>

        {rows.error && rows.items.length > 0 && (
          <p className='text-muted-foreground flex flex-wrap items-center gap-3 text-sm'>
            {t('The next page could not be loaded.')}
            <Button
              variant='outline'
              size='sm'
              className='rounded-full'
              onClick={reloadRows}
            >
              {t('Retry')}
            </Button>
          </p>
        )}

        {rows.loading && rows.items.length === 0 && (
          <ModelPromptsSkeleton cardCount={manifest?.pageSize} />
        )}

        {rows.error && rows.items.length === 0 && (
          <ErrorState
            className='mt-6'
            title={t('Failed to load')}
            description={t(
              'This model’s prompts could not be loaded. Retry, or pick another model.'
            )}
            onRetry={reloadRows}
          />
        )}

        {!rows.loading && !rows.error && rows.items.length === 0 && (
          <EmptyState
            className='mt-6'
            bordered
            title={t('No prompts in this library yet')}
            description={t(
              'This model has no browsable prompts at the moment. Pick another model from the prompt inspiration page.'
            )}
          />
        )}

        {rows.items.length > 0 && matched.length === 0 && (
          <EmptyState
            className='mt-6'
            bordered
            title={t('No prompts match these filters')}
            description={t('Loosen a filter to see more creatives.')}
            action={
              <Button
                variant='outline'
                size='lg'
                className='rounded-full'
                onClick={resetFilters}
              >
                {t('Clear all')}
              </Button>
            }
          />
        )}

        {matched.length > 0 && (
          <>
            <PromptWaterfall
              items={matched}
              onOpenPreview={openPreview}
              ladder={LISTING_COLUMN_STEPS}
              gapClassName='gap-4'
              testId='model-prompt-grid'
            />

            <div className='flex flex-col items-center gap-2 pt-4'>
              {rows.hasMore && (
                <Button
                  variant='outline'
                  size='lg'
                  className='rounded-full px-6'
                  disabled={rows.loading}
                  onClick={loadMore}
                >
                  {rows.loading ? t('Loading...') : t('Load more')}
                </Button>
              )}
              {!rows.hasMore && (
                <p className='text-muted-foreground text-sm'>
                  {t('You have reached the end of this library.')}
                </p>
              )}
            </div>
          </>
        )}
      </div>

      <ModelPromptGuide modelLabel={section.label} />

      <PromptPreviewModal
        items={preview?.items ?? []}
        index={preview?.index ?? -1}
        onIndexChange={(index) =>
          setPreview((prev) => (prev ? { ...prev, index } : prev))
        }
        onClose={() => setPreview(null)}
        onUsePrompt={handleUsePrompt}
      />
    </InspirationShell>
  )
}

/** `Home / Prompt inspiration / {model}` — the middle level returns to L1. */
function ModelBreadcrumb(props: { label: string }) {
  const { t } = useTranslation()

  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink
            render={<Link to='/' activeOptions={{ exact: true }} />}
          >
            {t('Home')}
          </BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>
          {/* `exact` keeps the ancestor link from claiming `aria-current`:
              only the trail's last item is the current page. */}
          <BreadcrumbLink
            render={<Link to='/inspiration' activeOptions={{ exact: true }} />}
          >
            {t('Prompt inspiration')}
          </BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>
          <BreadcrumbPage>{props.label}</BreadcrumbPage>
        </BreadcrumbItem>
      </BreadcrumbList>
    </Breadcrumb>
  )
}

interface FilterControlProps {
  active: boolean
  onClick: () => void
  children: ReactNode
}

/** Single-choice shortcut pill, e.g. `All Video Prompts`. */
function QuickLink(props: FilterControlProps) {
  return (
    <Button
      variant={props.active ? 'default' : 'outline'}
      size='lg'
      aria-pressed={props.active}
      onClick={props.onClick}
      className='rounded-full px-4'
    >
      {props.children}
    </Button>
  )
}

/** Single-choice category chip. */
function CategoryChip(props: FilterControlProps) {
  return (
    <Button
      variant={props.active ? 'default' : 'outline'}
      size='sm'
      aria-pressed={props.active}
      onClick={props.onClick}
      className={cn('rounded-full px-3', props.active && 'font-semibold')}
    >
      {props.children}
    </Button>
  )
}

/**
 * Guide block closing the model listing. Written for OmniMux rather than
 * copied from the reference page: it explains how to reuse a crawled prompt
 * and what makes one work.
 */
function ModelPromptGuide(props: { modelLabel: string }) {
  const { t } = useTranslation()

  return (
    <section
      aria-labelledby='model-prompt-guide'
      className='mt-16 border-t pt-8'
    >
      <h2
        id='model-prompt-guide'
        className='text-xl font-semibold tracking-[-0.02em] sm:text-2xl'
      >
        {t('Prompting guide: {{model}}', { model: props.modelLabel })}
      </h2>

      <div className='mt-6 grid gap-8 md:grid-cols-3'>
        <GuideTopic title={t('Start from the reference')}>
          <p>
            {t(
              'Every prompt below produced a real result. Open a card, read the full prompt, then copy it and swap in your own subject, product, or character.'
            )}
          </p>
          <p>
            {t(
              'Keep the camera, lighting, and style wording that came with the card. Rewriting all of it is what loses the look you picked the card for.'
            )}
          </p>
        </GuideTopic>

        <GuideTopic title={t('The prompt formula')}>
          <p>
            {t(
              'A prompt that works usually reads in four parts: subject and action, then camera and framing, then light and mood, then the finish you want.'
            )}
          </p>
          <p>
            {t(
              'Match the frame to the shot. A vertical close-up, a wide establishing shot, and a product macro need different instructions for the same subject.'
            )}
          </p>
        </GuideTopic>

        <GuideTopic title={t('Common mistakes')}>
          <p>
            {t(
              'Stacking several unrelated ideas into one prompt makes the model average them. One shot, one idea.'
            )}
          </p>
          <p>
            {t(
              'Empty quality words add nothing. Describe what should be visible in the frame instead of asking for "best quality".'
            )}
          </p>
        </GuideTopic>
      </div>

      <div className='mt-8'>
        <h3 className='text-sm font-semibold'>
          {t('What changes between a weak and a strong prompt')}
        </h3>
        <div className='mt-3 grid gap-4 md:grid-cols-2'>
          <div className='border-border bg-muted/30 rounded-2xl border p-4'>
            <p className='text-muted-foreground text-xs font-semibold tracking-[0.08em] uppercase'>
              {t('Weak')}
            </p>
            <code className='text-muted-foreground mt-2 block font-mono text-xs leading-relaxed break-words'>
              {t('a woman walking in a city')}
            </code>
            <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
              {t(
                'No subject detail, no camera, no light. The model fills every gap for you, so the result changes on every run.'
              )}
            </p>
          </div>
          <div className='border-border bg-card rounded-2xl border p-4'>
            <p className='text-foreground text-xs font-semibold tracking-[0.08em] uppercase'>
              {t('Strong')}
            </p>
            <code className='mt-2 block font-mono text-xs leading-relaxed break-words'>
              {t(
                'medium shot, woman in a beige trench coat walking through a rain-soaked neon street at night, reflections on wet asphalt, 50mm lens, shallow depth of field'
              )}
            </code>
            <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
              {t(
                'Subject, action, camera, and light are all pinned down, so the same prompt gives you the same shot again.'
              )}
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}

function GuideTopic(props: { title: string; children: ReactNode }) {
  return (
    <article className='flex flex-col gap-2'>
      <h3 className='text-base font-semibold tracking-[-0.01em]'>
        {props.title}
      </h3>
      <div className='text-muted-foreground flex flex-col gap-2 text-sm leading-relaxed text-pretty'>
        {props.children}
      </div>
    </article>
  )
}

/**
 * Skeleton for the model listing while its first page is on the wire. It runs
 * on the listing ladder so the placeholder columns match the tiles that replace
 * them.
 */
export function ModelPromptsSkeleton(props: { cardCount?: number }) {
  const cardCount = props.cardCount ?? DEFAULT_CATALOG_PAGE_SIZE
  const columns = splitIntoColumns(
    Array.from({ length: cardCount }, (_, index) => index),
    useWaterfallColumnCount(LISTING_COLUMN_STEPS)
  )

  return (
    <div
      className='flex items-start gap-4'
      aria-hidden='true'
      data-testid='model-prompts-skeleton'
    >
      {columns.map((column) => (
        <div key={column.id} className='flex min-w-0 flex-1 flex-col gap-4'>
          {column.items.map((cardIndex) => (
            <div
              key={cardIndex}
              className='flex flex-col overflow-hidden rounded-2xl border'
            >
              <Skeleton className='aspect-[4/3] w-full rounded-none' />
              <div className='flex flex-col gap-2 p-4'>
                <Skeleton className='h-2.5 w-12' />
                <Skeleton className='h-3.5 w-full' />
                <Skeleton className='h-3.5 w-2/3' />
              </div>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
