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
import { type ReactNode, useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { modelSectionAnchorId } from '../constants'
import { useInspirationCatalogPreview } from '../hooks/use-inspiration-catalog'
import { catalogGroupSections } from '../lib/catalog'
import { zoneGroupById } from '../lib/zone-groups'
import {
  DEFAULT_PROMPT_FILTERS,
  PROMPT_PREVIEW_COUNT,
  filterPromptItems,
  groupPromptsByModel,
  sortPromptItems,
  type PromptFilterState,
} from '../lib/filter'
import { formatCount } from '../lib/format'
import { resolveInspirationPlaygroundModelId } from '../lib/model-mapping'
import {
  PREVIEW_COLUMN_STEPS,
  splitIntoColumns,
  useWaterfallColumnCount,
} from '../lib/waterfall'
import type { PromptItem, PromptSection, PromptSort } from '../types'
import { InspirationShell } from './inspiration-shell'
import { ModelAnchorBar } from './model-anchor-bar'
import { PromptPreviewModal } from './prompt-preview-modal'
import { PromptWaterfall } from './prompt-waterfall'

/** Shared target of every "View all" link on the page. */
const MODEL_PROMPTS_ROUTE = '/inspiration/models/$modelSlug' as const

/**
 * Page A, level 1 — prompt inspiration grouped by model (PRD section 3).
 *
 * L1 is a *preview* layer: each section prints the model's full total, shows
 * the first `PROMPT_PREVIEW_COUNT` items, and sends the visitor to the model
 * listing (L2) for the rest. Pagination is deliberately absent here.
 *
 * The rows come from the static catalog: the manifest supplies every model
 * total and the filter vocabulary, and one page per zone supplies the teaser
 * grid. Filtering and sorting therefore act on the loaded teaser rows, which
 * is what the section prints — the totals above them stay the model's real
 * library size.
 */
export interface PromptGalleryProps {
  /** Optional initial share ID from URL query to open the full-screen modal directly */
  initialShareId?: string
}

export function PromptGallery(props?: PromptGalleryProps) {
  const { t } = useTranslation()
  const initialShareId = props?.initialShareId
  const [filters, setFilters] = useState<PromptFilterState>(
    DEFAULT_PROMPT_FILTERS
  )
  const sort: PromptSort = 'latest'
  // The detail layer walks the tiles the visitor actually sees, so the open
  // list travels with the click instead of being re-derived from the section.
  const [preview, setPreview] = useState<{
    items: PromptItem[]
    index: number
  } | null>(null)

  useEffect(() => {
    if (!initialShareId) return
    let active = true
    const fetchSharedInspiration = async () => {
      try {
        const res = await fetch(
          `/api/inspiration/v1/share/${encodeURIComponent(initialShareId)}`
        )
        if (!res.ok) {
          if (active) {
            toast.error(
              t(
                'Inspiration not found or expired',
                '灵感资源不存在或已超过3天保护期失效'
              )
            )
          }
          return
        }
        const body = await res.json()
        if (active && body?.success && body?.data) {
          setPreview({ items: [body.data as PromptItem], index: 0 })
        } else if (active) {
          toast.error(
            t(
              'Inspiration not found or expired',
              '灵感资源不存在或已超过3天保护期失效'
            )
          )
        }
      } catch {
        // ignore network error
      }
    }
    fetchSharedInspiration()
    return () => {
      active = false
    }
  }, [initialShareId, t])
  const openPreview = useCallback((item: PromptItem, items: PromptItem[]) => {
    setPreview({ items, index: Math.max(0, items.indexOf(item)) })
  }, [])

  const catalog = useInspirationCatalogPreview()
  const manifest = catalog.manifest
  const sections = useMemo(
    () => (manifest ? catalogGroupSections(manifest) : []),
    [manifest]
  )

  const loadedItems = useMemo(
    () => Object.values(catalog.rowsByZone).flat(),
    [catalog.rowsByZone]
  )

  const groups = useMemo(() => {
    const filtered = filterPromptItems(loadedItems, filters)
    const sorted = sortPromptItems(filtered, sort)
    return groupPromptsByModel(sorted, sections)
  }, [loadedItems, filters, sort, sections])

  // The anchor pills carry the model's library size, the same number the
  // section header prints, so the two never disagree while filtering.
  const anchorSections = useMemo(
    () => groups.map((group) => group.section),
    [groups]
  )

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

  // A zone whose page failed is dropped from `groups` (empty sections never
  // render), so its label is reported in one line above the grids instead.
  const failedSections = useMemo(
    () => sections.filter((section) => catalog.zoneErrors[section.key]),
    [sections, catalog.zoneErrors]
  )

  let body: ReactNode
  if (catalog.error) {
    body = (
      <ErrorState
        className='mt-10'
        title={t('Failed to load')}
        description={t(
          'The prompt catalog could not be loaded. Retry, or come back in a moment.'
        )}
        onRetry={catalog.reload}
      />
    )
  } else if (catalog.loading) {
    body = <PromptGallerySkeleton />
  } else if (groups.length === 0) {
    body = (
      <EmptyState
        className='mt-10'
        bordered
        title={t('No prompts match these filters')}
        description={t(
          'Try another category, or switch the media type back to All.'
        )}
        action={
          <button
            type='button'
            className='text-foreground text-sm font-medium underline underline-offset-4'
            onClick={() => setFilters(DEFAULT_PROMPT_FILTERS)}
          >
            {t('Clear all')}
          </button>
        }
      />
    )
  } else {
    body = (
      <>
        {failedSections.length > 0 && (
          <p className='text-muted-foreground mt-8 flex flex-wrap items-center gap-3 text-sm'>
            {t('Some model sections could not be loaded: {{models}}', {
              models: failedSections.map((section) => section.label).join(', '),
            })}
            <Button
              variant='outline'
              size='sm'
              className='rounded-full'
              onClick={catalog.reload}
            >
              {t('Retry')}
            </Button>
          </p>
        )}

        <div className='flex flex-col gap-12 pt-8'>
          {groups.map((group) => (
            <ModelPromptSection
              key={group.section.key}
              section={group.section}
              items={group.items}
              onOpenPreview={openPreview}
            />
          ))}
        </div>
      </>
    )
  }

  return (
    <InspirationShell active='prompts' showHero={false}>
      <div className='mt-2 sm:mt-4'>
        <ModelAnchorBar
          sections={anchorSections}
          zones={manifest?.zones}
          selectedStyle={filters.style}
          onStyleChange={(style) =>
            setFilters((prev) => ({ ...prev, style }))
          }
          selectedSubject={filters.subject}
          onSubjectChange={(subject) =>
            setFilters((prev) => ({ ...prev, subject }))
          }
        />
      </div>

      {body}

      <PromptLibraryIndex sections={sections} />

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

interface ModelPromptSectionProps {
  section: PromptSection
  items: PromptItem[]
  onOpenPreview: (item: PromptItem, items: PromptItem[]) => void
}

/**
 * One model block on L1: header with the model's full total, a capped preview
 * row, and the centered "View all" pill sitting on the row's bottom fade.
 *
 * The preview renders the same loomi tile as the L2 listing — one button that
 * opens the detail layer, media sized by the item's own aspect ratio — through
 * the shared `PromptWaterfall`, on the denser L1 ladder.
 *
 * The fade is `pointer-events-none` so the cards underneath stay clickable;
 * only the pill re-enables pointer events. Its colour comes from the
 * `--background` token (not the reference's hard-coded `#101010`) so it reads
 * as a fade into the page in both themes.
 */
function ModelPromptSection(props: ModelPromptSectionProps) {
  const { t } = useTranslation()
  const anchorId = modelSectionAnchorId(props.section.key)
  const titleId = `${anchorId}-title`
  const previewItems = props.items.slice(0, PROMPT_PREVIEW_COUNT)

  return (
    <section id={anchorId} className='scroll-mt-32' aria-labelledby={titleId}>
      <div className='mb-4 px-1'>
        <h2
          id={titleId}
          className='text-[20px] font-semibold tracking-[-0.01em]'
        >
          {t(props.section.label)}
        </h2>
        <p className='text-muted-foreground mt-1 text-[12px]'>
          {t('Top prompts · {{count}} total', {
            count: formatCount(props.section.count),
          })}
        </p>
      </div>

      <div className='relative max-h-[560px] overflow-hidden'>
        <div
          style={{
            maskImage:
              'linear-gradient(to bottom, black 60%, transparent 100%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 60%, transparent 100%)',
          }}
        >
          <PromptWaterfall
            items={previewItems}
            onOpenPreview={props.onOpenPreview}
            ladder={PREVIEW_COLUMN_STEPS}
            gapClassName='gap-3'
            testId='prompt-section-grid'
          />
        </div>

        <div className='from-background via-background/85 pointer-events-none absolute inset-x-0 bottom-0 flex justify-center bg-gradient-to-t to-transparent pt-16 pb-1'>
          <Link
            to={MODEL_PROMPTS_ROUTE}
            params={{ modelSlug: modelSlugForSection(props.section) }}
            aria-label={t('View all {{model}} prompts', {
              model: t(props.section.label),
            })}
            className='bg-foreground text-background hover:bg-foreground/90 focus-visible:ring-ring pointer-events-auto rounded-full px-6 py-2.5 text-[14px] font-semibold shadow-[0_8px_24px_rgba(0,0,0,0.35)] transition focus-visible:ring-2 focus-visible:outline-none active:scale-[0.98]'
          >
            {t('View all')}
          </Link>
        </div>
      </div>
    </section>
  )
}

/** Site-wide model directory closing L1, mirroring the reference tail block. */
/**
 * Where a section's "View all" leads.
 *
 * A 专區 has no listing of its own yet, so its links lead with its first member
 * zone — a page that exists and is populated — rather than at a 专區 slug no
 * model page can resolve.
 */
function modelSlugForSection(section: PromptSection): string {
  return zoneGroupById(section.key)?.zoneIds.at(0) ?? section.key
}

function PromptLibraryIndex(props: { sections: PromptSection[] }) {
  const { t } = useTranslation()

  return (
    <section
      aria-labelledby='inspiration-prompt-libraries'
      className='mt-16 border-t pt-8'
    >
      <h2
        id='inspiration-prompt-libraries'
        className='text-lg font-semibold tracking-[-0.01em]'
      >
        {t('Explore prompt libraries')}
      </h2>
      <p className='text-muted-foreground mt-1.5 max-w-2xl text-sm leading-relaxed text-pretty'>
        {t(
          'Every model keeps its own library. Open one to browse all of its prompts, search them, and filter by category.'
        )}
      </p>
      <ul className='mt-4 flex flex-wrap gap-2'>
        {props.sections.map((section) => (
          <li key={section.key}>
            <Link
              to={MODEL_PROMPTS_ROUTE}
              params={{ modelSlug: modelSlugForSection(section) }}
              className='border-border bg-card hover:border-foreground/25 hover:bg-muted focus-visible:ring-ring inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:outline-none'
            >
              {section.label}
              <span className='text-muted-foreground font-mono text-xs'>
                {formatCount(section.count)}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** Skeleton shown while the gallery's data is not ready yet. */
export function PromptGallerySkeleton() {
  const columns = splitIntoColumns(
    Array.from({ length: PROMPT_PREVIEW_COUNT }, (_, index) => index),
    useWaterfallColumnCount(PREVIEW_COLUMN_STEPS)
  )

  return (
    <div
      className='flex items-start gap-3'
      aria-hidden='true'
      data-testid='prompt-skeleton'
    >
      {columns.map((column) => (
        <div key={column.id} className='flex min-w-0 flex-1 flex-col gap-3'>
          {column.items.map((cardIndex) => (
            <div
              key={cardIndex}
              className='flex flex-col overflow-hidden rounded-2xl border'
            >
              <Skeleton className='aspect-[4/3] w-full rounded-none' />
              <div className='flex flex-col gap-2 p-3'>
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
