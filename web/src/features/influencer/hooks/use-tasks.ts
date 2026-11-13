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
  CancelledError,
  useInfiniteQuery,
  useQueries,
  useQueryClient,
  type InfiniteData,
} from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import {
  fetchInfluencerTasks,
  loadTaskRecord,
  INFLUENCER_ACTION,
  INFLUENCER_MOTION_ACTION,
  submitMotion,
  submitSheet,
  type MotionSubmitBody,
  type SheetSubmitBody,
  type TaskPage,
} from '../api'
import {
  isTerminalStatus,
  sortTasksByTime,
  taskImageUrl,
  taskVideoUrl,
} from '../lib/history'
import { readHistoryCache, writeHistoryCache } from '../lib/history-cache'
import type { TaskRecord } from '../types'

/** Records per request. Small enough that the first page paints quickly, while
 * `with_artifacts` still runs the plugin projection for each successful task. */
const PAGE_SIZE = 12
const POLL_MS = 2_000
/** A queued task can sit for minutes, so polling every two seconds forever only
 * spends requests: the interval widens once the task has had time to start. */
const POLL_SLOW_MS = 5_000
const POLL_BACKOFF_AFTER_MS = 30_000
const ENRICHMENT_DELAY_MS = 4_000
const ENRICHMENT_LIMIT = 3
// Only counters live outside Query data, scoped to the owning history Query's
// cache lifetime. Observers, remounts and cancelled delays cannot reset them.
const enrichmentAttempts = new WeakMap<object, Map<string, number>>()

function needsHistoryMedia(item: TaskRecord, action: string): boolean {
  if ((item.status ?? '').toUpperCase() !== 'SUCCESS') return false
  if (item.action && item.action !== action) return false
  if (action === INFLUENCER_MOTION_ACTION) {
    return !taskVideoUrl(item) || !item.poster_url
  }
  return !taskImageUrl(item)
}

/** Abort owns the delay/subscription only: GET deduplication ignores signals,
 * so cancelling its transport could also cancel an unrelated shared reader. */
function waitForEnrichment(signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const clear = () => {
      clearTimeout(timer)
      timer = undefined
    }
    const cleanup = () => {
      clear()
      document.removeEventListener('visibilitychange', schedule)
      window.removeEventListener('online', schedule)
      window.removeEventListener('offline', schedule)
      signal.removeEventListener('abort', cancel)
    }
    const cancel = () => {
      cleanup()
      reject(new CancelledError())
    }
    const schedule = () => {
      clear()
      if (document.visibilityState === 'hidden' || !navigator.onLine) return
      timer = setTimeout(() => {
        cleanup()
        resolve()
      }, ENRICHMENT_DELAY_MS)
    }
    if (signal.aborted) {
      cancel()
      return
    }
    document.addEventListener('visibilitychange', schedule)
    window.addEventListener('online', schedule)
    window.addEventListener('offline', schedule)
    signal.addEventListener('abort', cancel, { once: true })
    schedule()
  })
}

export interface InfluencerHistory {
  items: TaskRecord[]
  total: number
  hasMore: boolean
  /** True only while the first page is loading with nothing to paint yet. */
  isInitialLoading: boolean
  isFetchingMore: boolean
  loadMoreFailed: boolean
  loadMore: () => void
  error: unknown
  retry?: () => unknown
}

/**
 * One action's slice of the server-side history, paged in on demand and — for
 * the character sheet action — seeded from the local cache so a revisit paints
 * before the network answers.
 */
function useActionHistory(
  enabled: boolean,
  action: string,
  seedCache: boolean
) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  // A guest never reads the per-user paint cache: it belongs to the account
  // last signed in on this browser, not to the visitor.
  const cached = useMemo(
    () => (enabled && seedCache ? readHistoryCache(userId) : null),
    [enabled, seedCache, userId]
  )

  const query = useInfiniteQuery({
    queryKey: ['influencer', 'history', 'pages', action, userId],
    enabled,
    queryFn: ({ pageParam }) =>
      fetchInfluencerTasks({
        action,
        page: pageParam,
        page_size: PAGE_SIZE,
        with_artifacts: true,
      }),
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((sum, page) => sum + page.items.length, 0)
      // An empty page means the offset ran past the data even if `total`
      // disagrees, so it stops here instead of asking again forever.
      if (!lastPage.items.length) return undefined
      if (loaded >= lastPage.total) return undefined
      return allPages.length + 1
    },
    initialData: cached
      ? {
          pages: [{ items: cached.items, total: cached.total }],
          pageParams: [1],
        }
      : undefined,
    // Seeded data is a paint-first cache, not proof the page is current.
    initialDataUpdatedAt: 0,
    staleTime: 30_000,
    refetchOnMount: 'always',
    refetchInterval: (current) =>
      current.state.data?.pages.some((page) =>
        page.items.some((item) => !isTerminalStatus(item.status))
      )
        ? POLL_MS
        : false,
  })

  const items = useMemo(
    () => query.data?.pages.flatMap((page) => page.items) ?? [],
    [query.data]
  )
  const total = query.data?.pages[0]?.total ?? 0
  const fetchedAfterMount = query.isFetchedAfterMount

  useEffect(() => {
    // Only what the network returned this session is persisted, so a stale seed
    // cannot keep refreshing its own timestamp.
    if (!fetchedAfterMount || !seedCache) return
    writeHistoryCache(userId, items, total)
  }, [fetchedAfterMount, seedCache, items, total, userId])

  const queryClient = useQueryClient()
  const historyKey = ['influencer', 'history', 'pages', action, userId]
  const owner = queryClient.getQueryCache().find({ queryKey: historyKey })
  let attempts = owner ? enrichmentAttempts.get(owner) : undefined
  if (!attempts) {
    attempts = new Map<string, number>()
    if (owner) enrichmentAttempts.set(owner, attempts)
  }
  const budget = attempts
  const awaitingMedia = enabled
    ? items.filter((item) => item.task_id && needsHistoryMedia(item, action))
    : []
  // Query owns one delayed read loop per account/action/task, even when the
  // gallery and picker observe the same history. Only executed reads cost budget.
  useQueries({
    queries: awaitingMedia.map((item) => ({
      queryKey: [
        'influencer',
        'history',
        'enrichment',
        action,
        userId,
        item.task_id,
      ],
      enabled,
      staleTime: Infinity,
      gcTime: 0,
      retry: false,
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
      meta: { errorToast: false },
      queryFn: async ({ signal }: { signal: AbortSignal }) => {
        const currentRecord = () =>
          queryClient
            .getQueryData<InfiniteData<TaskPage>>(historyKey)
            ?.pages.flatMap((page) => page.items)
            .find((record) => record.task_id === item.task_id)
        while ((budget.get(item.task_id) ?? 0) < ENRICHMENT_LIMIT) {
          await waitForEnrichment(signal)
          const current = currentRecord()
          if (
            signal.aborted ||
            useAuthStore.getState().auth.user?.id !== userId ||
            !current ||
            !needsHistoryMedia(current, action)
          ) {
            break
          }
          budget.set(item.task_id, (budget.get(item.task_id) ?? 0) + 1)
          try {
            const page = await fetchInfluencerTasks({
              action,
              task_id: item.task_id,
              page_size: 1,
              with_artifacts: true,
            })
            if (
              signal.aborted ||
              useAuthStore.getState().auth.user?.id !== userId
            ) {
              break
            }
            const next = page.items.find(
              (record) => record.task_id === item.task_id
            )
            if (
              !next ||
              (next.action && next.action !== action) ||
              (next.status ?? '').toUpperCase() !== 'SUCCESS'
            ) {
              continue
            }
            queryClient.setQueryData<InfiniteData<TaskPage>>(
              historyKey,
              (data) => {
                if (!data) return data
                return {
                  ...data,
                  pages: data.pages.map((existingPage) => ({
                    ...existingPage,
                    items: existingPage.items.map((record) => {
                      if (
                        record.task_id !== item.task_id ||
                        !needsHistoryMedia(record, action)
                      ) {
                        return record
                      }
                      // Enrichment cannot regress an already confirmed source or
                      // replace current task metadata with a delayed snapshot.
                      const hasMain =
                        action === INFLUENCER_MOTION_ACTION
                          ? Boolean(taskVideoUrl(record))
                          : Boolean(taskImageUrl(record))
                      return {
                        ...record,
                        artifacts:
                          !hasMain && next.artifacts?.length
                            ? next.artifacts
                            : record.artifacts,
                        result_url: hasMain
                          ? record.result_url
                          : next.result_url,
                        legacy_content_url: hasMain
                          ? record.legacy_content_url
                          : next.legacy_content_url,
                        poster_url: record.poster_url || next.poster_url,
                      }
                    }),
                  })),
                }
              }
            )
          } catch {
            // An artifact read failure leaves task acceptance/status untouched.
          }
          const latest = currentRecord()
          if (!latest || !needsHistoryMedia(latest, action)) break
        }
        return null
      },
    })),
  })

  const hasNextPage = query.hasNextPage ?? false
  const isFetchingNextPage = query.isFetchingNextPage
  const fetchNextPage = query.fetchNextPage

  const loadMore = useCallback(() => {
    if (!hasNextPage || isFetchingNextPage) return
    void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  return {
    items,
    total,
    hasMore: hasNextPage,
    isInitialLoading: items.length === 0 && query.isFetching,
    isFetchingMore: isFetchingNextPage,
    loadMoreFailed: query.isFetchNextPageError,
    loadMore,
    error: query.error,
    retry: query.refetch,
  }
}

/**
 * Server-side history, paged in on demand. Without an explicit `action` the
 * feed is the union of the image and video actions, merged newest-first — the
 * gallery does not split records by what generated them.
 */
export function useInfluencerHistory(
  enabled = true,
  action?: string
): InfluencerHistory {
  const sheet = useActionHistory(
    enabled && (!action || action === INFLUENCER_ACTION),
    INFLUENCER_ACTION,
    // Only the character gallery has a paint cache; the video list is small
    // and always read fresh.
    !action || action === INFLUENCER_ACTION
  )
  const video = useActionHistory(
    enabled && (!action || action === INFLUENCER_MOTION_ACTION),
    INFLUENCER_MOTION_ACTION,
    false
  )

  const items = useMemo(() => {
    const seen = new Set<string | number>()
    const merged: TaskRecord[] = []
    for (const task of [...sheet.items, ...video.items]) {
      const id = task.task_id || task.id
      if (id && seen.has(id)) continue
      if (id) seen.add(id)
      merged.push(task)
    }
    return sortTasksByTime(merged)
  }, [sheet.items, video.items])

  const loadSheetMore = sheet.loadMore
  const loadVideoMore = video.loadMore
  const loadMore = useCallback(() => {
    loadSheetMore()
    loadVideoMore()
  }, [loadSheetMore, loadVideoMore])

  if (action === INFLUENCER_ACTION) return sheet
  if (action === INFLUENCER_MOTION_ACTION) return video

  return {
    items,
    total: sheet.total + video.total,
    hasMore: sheet.hasMore || video.hasMore,
    isInitialLoading:
      items.length === 0 && (sheet.isInitialLoading || video.isInitialLoading),
    isFetchingMore: sheet.isFetchingMore || video.isFetchingMore,
    loadMoreFailed: sheet.loadMoreFailed || video.loadMoreFailed,
    loadMore,
    error: sheet.error ?? video.error,
    retry: () => Promise.all([sheet.retry(), video.retry()]),
  }
}

/**
 * Submit a sheet or motion task and follow the accepted ID to a confirmed terminal state.
 * Before its record is readable, only the accepted ID and submitted state are known.
 */
function useTaskSubmit<Body>(
  send: (body: Body) => Promise<string>,
  action: string
) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const [tasks, setTasks] = useState<TaskRecord[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const timersRef = useRef(new Map<string, ReturnType<typeof setTimeout>>())
  const ticksRef = useRef(new Map<string, () => void>())
  const mountedRef = useRef(true)
  const submittingRef = useRef(false)
  const epochRef = useRef(0)

  const writeRecord = useCallback((record: TaskRecord) => {
    setTasks((current) => {
      const index = current.findIndex((item) => item.task_id === record.task_id)
      if (index < 0) return [...current, record]
      return current.map((item, i) => (i === index ? record : item))
    })
  }, [])

  const clearTimer = useCallback((taskId: string) => {
    const timer = timersRef.current.get(taskId)
    if (timer !== undefined) clearTimeout(timer)
    timersRef.current.delete(taskId)
    ticksRef.current.delete(taskId)
  }, [])

  const reset = useCallback(() => {
    epochRef.current += 1
    for (const timer of timersRef.current.values()) clearTimeout(timer)
    timersRef.current.clear()
    ticksRef.current.clear()
    submittingRef.current = false
    setSubmitting(false)
    setTasks([])
    setError(null)
  }, [])

  const loadRecord = useCallback(
    async (taskId: string): Promise<TaskRecord | null> => {
      return loadTaskRecord(taskId, action)
    },
    [action]
  )

  useEffect(() => {
    mountedRef.current = true
    reset()
    const timers = timersRef.current
    const ticks = ticksRef.current
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        for (const tick of ticksRef.current.values()) tick()
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
    return () => {
      mountedRef.current = false
      epochRef.current += 1
      for (const timer of timers.values()) clearTimeout(timer)
      timers.clear()
      ticks.clear()
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [userId, reset])

  const startPolling = useCallback(
    (taskId: string, epoch: number, immediate = false) => {
      clearTimer(taskId)
      const startedAt = Date.now()
      let inFlight = false
      const isCurrent = () =>
        mountedRef.current &&
        epochRef.current === epoch &&
        ticksRef.current.has(taskId)
      const schedule = () => {
        if (!isCurrent()) return
        const previous = timersRef.current.get(taskId)
        if (previous !== undefined) clearTimeout(previous)
        const delay =
          Date.now() - startedAt > POLL_BACKOFF_AFTER_MS
            ? POLL_SLOW_MS
            : POLL_MS
        timersRef.current.set(
          taskId,
          setTimeout(() => void tick(), delay)
        )
      }
      const tick = async () => {
        if (!isCurrent() || inFlight) return
        if (document.visibilityState === 'hidden') {
          schedule()
          return
        }
        inFlight = true
        try {
          const next = await loadRecord(taskId)
          if (!isCurrent()) return
          if (next) writeRecord(next)
          if (next && isTerminalStatus(next.status)) {
            clearTimer(taskId)
            return
          }
        } catch {
          // A failed status read does not undo acceptance or replay the POST.
        } finally {
          inFlight = false
        }
        schedule()
      }
      ticksRef.current.set(taskId, () => void tick())
      if (immediate) void tick()
      else schedule()
    },
    [clearTimer, loadRecord, writeRecord]
  )

  const submit = useCallback(
    async (body: Body): Promise<TaskRecord | null> => {
      if (submittingRef.current) return null
      const epoch = epochRef.current
      const isCurrent = () => mountedRef.current && epochRef.current === epoch
      submittingRef.current = true
      setSubmitting(true)
      setError(null)
      try {
        const taskId = await send(body)
        if (!isCurrent()) return null
        const accepted: TaskRecord = {
          id: 0,
          task_id: taskId,
          action,
          status: 'QUEUED',
          submit_time: Math.floor(Date.now() / 1000),
          properties: { input: JSON.stringify(body) },
        }
        writeRecord(accepted)
        // The accepted ID is sufficient to resume editing and submitting.
        // Its first status read must not prolong the write lock.
        submittingRef.current = false
        setSubmitting(false)
        startPolling(taskId, epoch, true)
        return accepted
      } catch (e) {
        if (isCurrent()) setError(e)
        return null
      } finally {
        if (isCurrent() && submittingRef.current) {
          submittingRef.current = false
          setSubmitting(false)
        }
      }
    },
    [action, startPolling, send, writeRecord]
  )

  const pending = tasks.at(-1) ?? null
  return { tasks, pending, submitting, error, submit, reset }
}

/** Submit a character sheet and follow it to a terminal state. */
export function useSheetSubmit() {
  return useTaskSubmit<SheetSubmitBody>(submitSheet, INFLUENCER_ACTION)
}

/** Submit a character video and follow it to a terminal state. */
export function useMotionSubmit() {
  return useTaskSubmit<MotionSubmitBody>(submitMotion, INFLUENCER_MOTION_ACTION)
}
