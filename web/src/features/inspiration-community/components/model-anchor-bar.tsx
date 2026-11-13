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
import { Link } from '@tanstack/react-router'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { modelSectionAnchorId } from '../constants'
import type { CatalogZone } from '../lib/catalog'
import { facetDisplayName } from '../lib/facets'
import {
  groupFacetValues,
  preferredFacetLabels,
  ZONE_GROUPS,
  zoneGroupById,
} from '../lib/zone-groups'
import type { PromptSection } from '../types'

/**
 * Resolve a model section element. An attribute selector is used instead of
 * `getElementById` because model keys contain dots (`seedance-2.5`), and it
 * keeps a single DOM lookup style across the feature.
 */
function queryModelSectionAnchor(modelKey: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(
    `[id="${modelSectionAnchorId(modelKey)}"]`
  )
}

export interface ModelAnchorBarProps {
  sections: PromptSection[]
  zones?: readonly CatalogZone[]
  selectedStyle?: string
  onStyleChange?: (style: string) => void
  selectedSubject?: string
  onSubjectChange?: (subject: string) => void
  className?: string
}

/**
 * Top-level category navigation and style/subject facet filters.
 *
 * Reproduces the reference layout: clean text tabs for each 专區 with
 * Marketing Studio linked alongside, plus two facet rows (Style and Subject)
 * for the active 专區 with solid pill selection.
 */
export function ModelAnchorBar(props: ModelAnchorBarProps) {
  const { t, i18n } = useTranslation()
  const [activeKey, setActiveKey] = useState<string | null>(
    props.sections[0]?.key ?? null
  )
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (props.sections.length === 0) return
    if (typeof IntersectionObserver === 'undefined') return

    const elements = props.sections
      .map((section) => ({
        key: section.key,
        element: queryModelSectionAnchor(section.key),
      }))
      .filter((entry): entry is { key: string; element: HTMLElement } =>
        Boolean(entry.element)
      )

    if (elements.length === 0) return

    const visible = new Map<string, boolean>()
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const key = entry.target.getAttribute('data-model-key')
          if (key) visible.set(key, entry.isIntersecting)
        }
        const next = elements.find((entry) => visible.get(entry.key))
        if (next) setActiveKey(next.key)
      },
      // Treat the region just below the sticky header as the "active" band.
      { rootMargin: '-96px 0px -70% 0px', threshold: 0 }
    )

    for (const entry of elements) {
      entry.element.setAttribute('data-model-key', entry.key)
      observer.observe(entry.element)
    }
    return () => observer.disconnect()
  }, [props.sections])

  // Keep the highlighted tab inside the horizontal scroller without scrolling the window.
  useEffect(() => {
    if (!activeKey) return
    const container = containerRef.current
    if (!container) return
    const pill = container.querySelector<HTMLElement>(
      `[data-anchor-key="${activeKey}"]`
    )
    if (!pill) return
    const pillLeft = pill.offsetLeft
    const pillWidth = pill.offsetWidth
    const containerWidth = container.clientWidth
    const currentScroll = container.scrollLeft

    if (pillLeft < currentScroll) {
      container.scrollTo({ left: pillLeft, behavior: 'smooth' })
    } else if (pillLeft + pillWidth > currentScroll + containerWidth) {
      container.scrollTo({
        left: pillLeft + pillWidth - containerWidth,
        behavior: 'smooth',
      })
    }
  }, [activeKey])

  const handleJump = useCallback((key: string) => {
    setActiveKey(key)
    queryModelSectionAnchor(key)?.scrollIntoView({
      behavior: 'smooth',
      block: 'start',
    })
  }, [])

  const currentGroup = useMemo(
    () => (activeKey ? zoneGroupById(activeKey) : undefined) ?? ZONE_GROUPS[0],
    [activeKey]
  )

  const styles = useMemo(() => {
    if (!props.zones || props.zones.length === 0 || !currentGroup) return []
    return preferredFacetLabels(
      groupFacetValues(currentGroup, props.zones, 'styles')
    )
  }, [currentGroup, props.zones])

  const subjects = useMemo(() => {
    if (!props.zones || props.zones.length === 0 || !currentGroup) return []
    return preferredFacetLabels(
      groupFacetValues(currentGroup, props.zones, 'subjects')
    )
  }, [currentGroup, props.zones])

  const hasFacets = styles.length > 0 || subjects.length > 0

  if (props.sections.length === 0) return null

  return (
    <nav
      aria-label={t('Models')}
      data-surface='inspiration.anchor-bar.v2'
      className={cn(
        'bg-card/95 border-border/80 shadow-xs relative rounded-[24px] border px-5 py-4 backdrop-blur-md sm:px-7 sm:py-4.5',
        props.className
      )}
    >
      {/* ── Row 1: 一级分类 (专区 tabs + 营销工作室) ── */}
      <div
        ref={containerRef}
        className='flex items-center gap-6 overflow-x-auto pb-2.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:gap-8'
      >
        {props.sections.map((section) => {
          const isActive = section.key === activeKey
          const group = zoneGroupById(section.key)
          return (
            <button
              key={section.key}
              type='button'
              data-anchor-key={section.key}
              aria-current={isActive ? 'true' : undefined}
              onClick={() => handleJump(section.key)}
              className={cn(
                'shrink-0 text-base whitespace-nowrap transition-colors',
                'focus-visible:ring-ring rounded-md px-1 py-0.5 focus-visible:ring-2 focus-visible:outline-none',
                isActive
                  ? 'text-foreground font-bold'
                  : 'text-muted-foreground hover:text-foreground font-medium'
              )}
            >
              <span className='inline-flex items-center gap-1.5'>
                {group ? getLobeIcon(group.iconName, 16) : null}
                {t(section.label)}
              </span>
            </button>
          )
        })}

        {/* 营销工作室（原：TikTok 广告库） */}
        <Link
          to='/inspiration/tiktok'
          className='text-muted-foreground hover:text-foreground focus-visible:ring-ring shrink-0 rounded-md px-1 py-0.5 text-base font-medium whitespace-nowrap transition-colors focus-visible:ring-2 focus-visible:outline-none'
        >
          <span className='inline-flex items-center gap-1.5'>
            {getLobeIcon('TikTok', 16)}
            {t('TikTok Ad Library')}
          </span>
        </Link>
      </div>

      {/* ── Row 2 & 3: 风格与主体 Facets ── */}
      {hasFacets && (
        <div className='border-border/40 flex flex-col gap-2 border-t pt-2.5'>
          {/* 风格 */}
          {styles.length > 0 && (
            <div className='flex items-center gap-3 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'>
              <span className='text-muted-foreground shrink-0 text-xs font-medium sm:text-sm'>
                {t('Style')}
              </span>
              <div className='flex items-center gap-2'>
                <button
                  type='button'
                  onClick={() => props.onStyleChange?.('all')}
                  className={cn(
                    'shrink-0 rounded-md px-2.5 py-1 text-xs font-medium transition-colors sm:text-sm',
                    (props.selectedStyle ?? 'all') === 'all'
                      ? 'bg-muted text-foreground font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t('All')}
                </button>
                {styles.map((style) => {
                  const isSelected = props.selectedStyle === style
                  return (
                    <button
                      key={style}
                      type='button'
                      onClick={() => props.onStyleChange?.(style)}
                      className={cn(
                        'shrink-0 rounded-md px-2.5 py-1 text-xs font-medium whitespace-nowrap transition-colors sm:text-sm',
                        isSelected
                          ? 'bg-muted text-foreground font-semibold'
                          : 'text-muted-foreground hover:text-foreground'
                      )}
                    >
                      {facetDisplayName(style, i18n.language)}
                    </button>
                  )
                })}
              </div>
            </div>
          )}

          {/* 主体 */}
          {subjects.length > 0 && (
            <div className='flex items-center gap-3 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'>
              <span className='text-muted-foreground shrink-0 text-xs font-medium sm:text-sm'>
                {t('Subject')}
              </span>
              <div className='flex items-center gap-2'>
                <button
                  type='button'
                  onClick={() => props.onSubjectChange?.('all')}
                  className={cn(
                    'shrink-0 rounded-md px-2.5 py-1 text-xs font-medium transition-colors sm:text-sm',
                    (props.selectedSubject ?? 'all') === 'all'
                      ? 'bg-muted text-foreground font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t('All')}
                </button>
                {subjects.map((subject) => {
                  const isSelected = props.selectedSubject === subject
                  return (
                    <button
                      key={subject}
                      type='button'
                      onClick={() => props.onSubjectChange?.(subject)}
                      className={cn(
                        'shrink-0 rounded-md px-2.5 py-1 text-xs font-medium whitespace-nowrap transition-colors sm:text-sm',
                        isSelected
                          ? 'bg-muted text-foreground font-semibold'
                          : 'text-muted-foreground hover:text-foreground'
                      )}
                    >
                      {facetDisplayName(subject, i18n.language)}
                    </button>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </nav>
  )
}
