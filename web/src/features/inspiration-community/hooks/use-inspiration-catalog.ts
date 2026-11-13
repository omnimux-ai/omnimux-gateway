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
import { useCallback, useEffect, useReducer, useRef, useState } from 'react'

import {
  type CatalogManifest,
  type CatalogPage,
  type CatalogZone,
  fetchCatalogPage,
  inspirationCatalogBase,
  isAbortError,
  toCatalogError,
} from '../lib/catalog'
import type { PromptItem } from '../types'
import { useInspirationManifest } from './use-inspiration-manifest'

function zoneKey(zoneId: string, quarantined: boolean): string {
  return `${zoneId}:${quarantined ? 'quarantined' : 'displayable'}`
}

/* ── L1: the first page of every zone, loaded together ─────────────────── */

export interface InspirationCatalogPreviewState {
  manifest: CatalogManifest | null
  loading: boolean
  /** The manifest itself failed; no page can be rendered. */
  error: Error | null
  /** Zone id -> the rows of its first page. */
  rowsByZone: Record<string, PromptItem[]>
  /** Zone id -> its first page failed; that section renders its own error. */
  zoneErrors: Record<string, Error>
  reload: () => void
}

/**
 * Data source of L1 (`/inspiration`).
 *
 * L1 is a preview layer: it prints every model's manifest total and a teaser
 * grid per section, so it loads the first page of every zone in the manifest
 * once and holds it. Pages are requested together and aborted together; a zone
 * whose page fails keeps the others on screen and reports its own error.
 */
export function useInspirationCatalogPreview(
  base: string = inspirationCatalogBase()
): InspirationCatalogPreviewState {
  const manifestState = useInspirationManifest(base)
  const reloadManifest = manifestState.reload
  const zones = manifestState.manifest?.zones ?? null

  const [pages, setPages] = useState<{
    loading: boolean
    rowsByZone: Record<string, PromptItem[]>
    zoneErrors: Record<string, Error>
  }>({ loading: false, rowsByZone: {}, zoneErrors: {} })

  useEffect(() => {
    if (!zones || zones.length === 0) return

    const controller = new AbortController()
    setPages({ loading: true, rowsByZone: {}, zoneErrors: {} })

    const requests = zones.map(async (zone) => {
      try {
        const page = await fetchCatalogPage({
          base,
          zone: zone.id,
          page: 0,
          signal: controller.signal,
        })
        return { zoneId: zone.id, items: page.items, error: null }
      } catch (error: unknown) {
        return { zoneId: zone.id, items: [], error: toCatalogError(error) }
      }
    })

    void Promise.all(requests).then((results) => {
      if (controller.signal.aborted) return

      const rowsByZone: Record<string, PromptItem[]> = {}
      const zoneErrors: Record<string, Error> = {}
      for (const result of results) {
        if (!result.error) rowsByZone[result.zoneId] = result.items
        else if (!isAbortError(result.error)) zoneErrors[result.zoneId] = result.error
      }
      setPages({ loading: false, rowsByZone, zoneErrors })
    })

    return () => controller.abort()
  }, [base, zones])

  return {
    manifest: manifestState.manifest,
    loading: manifestState.loading || pages.loading,
    error: manifestState.error,
    rowsByZone: pages.rowsByZone,
    zoneErrors: pages.zoneErrors,
    // Re-reading the manifest yields a fresh object, which re-runs the page
    // effect above — one retry action covers both failures.
    reload: reloadManifest,
  }
}

/* ── L2: one zone, page by page ───────────────────────────────────────── */

interface ZoneRowsState {
  /** `<zone>:<displayable|quarantined>` the loaded rows belong to. */
  key: string
  items: PromptItem[]
  loadedPages: number
  total: number
  totalPages: number
  loading: boolean
  error: Error | null
}

const EMPTY_ZONE_ROWS: ZoneRowsState = {
  key: '',
  items: [],
  loadedPages: 0,
  total: 0,
  totalPages: 0,
  loading: false,
  error: null,
}

type ZoneRowsAction =
  | { type: 'clear' }
  | { type: 'reset'; key: string }
  | { type: 'loading'; key: string }
  | { type: 'append'; key: string; page: CatalogPage }
  | { type: 'fail'; key: string; error: Error }

function zoneRowsReducer(
  state: ZoneRowsState,
  action: ZoneRowsAction
): ZoneRowsState {
  if (action.type === 'clear') return EMPTY_ZONE_ROWS
  // A response for a set the visitor already left must not touch the screen.
  if (action.type !== 'reset' && action.key !== state.key) return state

  switch (action.type) {
    case 'reset':
      return { ...EMPTY_ZONE_ROWS, key: action.key, loading: true }
    case 'loading':
      return { ...state, loading: true, error: null }
    case 'append':
      return {
        key: state.key,
        items: [...state.items, ...action.page.items],
        loadedPages: state.loadedPages + 1,
        total: action.page.total,
        totalPages: action.page.totalPages,
        loading: false,
        error: null,
      }
    case 'fail':
      return { ...state, loading: false, error: action.error }
  }
}

export interface CatalogZoneRowsState {
  items: PromptItem[]
  /** Pages merged into `items` so far. */
  loadedPages: number
  total: number
  totalPages: number
  loading: boolean
  error: Error | null
  hasMore: boolean
  /** Fetches the next page of this zone and appends it. */
  loadMore: () => void
  /** Restarts from page 0; wired to the error state's retry action. */
  reload: () => void
}

export interface UseCatalogZoneRowsOptions {
  /** Hidden-by-default quarantined set; see `ModelPromptsPage`. */
  quarantined?: boolean
  /** Catalog mount point; defaults to `VITE_INSPIRATION_CATALOG_BASE`. */
  base?: string
}

/**
 * Paged row source of L2 (`/inspiration/models/$modelSlug`).
 *
 * `zoneId` is `null` until the manifest has resolved, which keeps a page for an
 * unknown model slug from being requested at all. The first page is fetched by
 * an effect that runs once per zone (not per render); `loadMore` appends the
 * next page of the same zone and is dropped while a page is already in flight,
 * so a double click can never append the same rows twice.
 */
export function useCatalogZoneRows(
  zoneId: string | null,
  options: UseCatalogZoneRowsOptions = {}
): CatalogZoneRowsState {
  const base = options.base ?? inspirationCatalogBase()
  const quarantined = options.quarantined === true

  const [state, dispatch] = useReducer(zoneRowsReducer, EMPTY_ZONE_ROWS)
  const [attempt, setAttempt] = useState(0)

  // Cursor and in-flight handle live in a ref so `loadMore` reads them
  // synchronously instead of depending on the rendered state.
  const cursor = useRef({ nextPage: 0, totalPages: 0, loading: false })
  const inFlight = useRef<AbortController | null>(null)

  const request = useCallback(
    (page: number, replace: boolean) => {
      if (!zoneId || cursor.current.loading) return

      const key = zoneKey(zoneId, quarantined)
      const controller = new AbortController()
      inFlight.current = controller
      cursor.current = { ...cursor.current, loading: true }
      dispatch(replace ? { type: 'reset', key } : { type: 'loading', key })

      fetchCatalogPage({
        base,
        zone: zoneId,
        page,
        quarantined,
        signal: controller.signal,
      })
        .then((payload) => {
          if (controller.signal.aborted) return
          inFlight.current = null
          cursor.current = {
            nextPage: page + 1,
            totalPages: payload.totalPages,
            loading: false,
          }
          dispatch({ type: 'append', key, page: payload })
        })
        .catch((error: unknown) => {
          if (controller.signal.aborted || isAbortError(error)) return
          inFlight.current = null
          cursor.current = { ...cursor.current, loading: false }
          dispatch({ type: 'fail', key, error: toCatalogError(error) })
        })
    },
    [base, quarantined, zoneId]
  )

  useEffect(() => {
    if (!zoneId) {
      cursor.current = { nextPage: 0, totalPages: 0, loading: false }
      dispatch({ type: 'clear' })
      return
    }

    inFlight.current?.abort()
    inFlight.current = null
    cursor.current = { nextPage: 0, totalPages: 0, loading: false }
    request(0, true)

    return () => {
      inFlight.current?.abort()
      inFlight.current = null
      cursor.current = { ...cursor.current, loading: false }
    }
  }, [request, zoneId, attempt])

  const loadMore = useCallback(() => {
    const { nextPage, totalPages, loading } = cursor.current
    if (loading || nextPage >= totalPages) return
    request(nextPage, false)
  }, [request])

  const reload = useCallback(() => setAttempt((current) => current + 1), [])

  return {
    items: state.items,
    loadedPages: state.loadedPages,
    total: state.total,
    totalPages: state.totalPages,
    loading: state.loading,
    error: state.error,
    hasMore: state.totalPages > state.loadedPages,
    loadMore,
    reload,
  }
}

/** Resolve a zone from the manifest by its id, e.g. for the L2 header. */
export function findCatalogZone(
  manifest: CatalogManifest | null,
  zoneId: string | null
): CatalogZone | null {
  if (!manifest || !zoneId) return null
  return manifest.zones.find((zone) => zone.id === zoneId) ?? null
}
