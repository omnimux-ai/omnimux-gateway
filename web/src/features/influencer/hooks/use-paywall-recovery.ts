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
import { useQueryClient } from '@tanstack/react-query'
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { useAuthStore } from '@/stores/auth-store'

import {
  fetchPaymentImageCatalog,
  type MotionSubmitBody,
  type MultiViewSubmitBody,
  type SheetSubmitBody,
} from '../api'
import { submitFailureText, toastInfluencerError } from '../lib/error-copy'
import {
  describeFailReason,
  isInsufficientQuotaFailure,
} from '../lib/fail-reason'
import {
  historicalPaywallTarget,
  quoteImageGenerationRoute,
  type GenerationRequest,
  type HistoryPaywallRequest,
  type PaywallTarget,
} from '../lib/generation-recovery'
import { isSuccessStatus, isTerminalStatus, taskKind } from '../lib/history'
import {
  isInsufficientQuotaError,
  type PaywallOrigin,
} from '../lib/insufficient-credits'
import { INFLUENCER_ANALYTICS_EVENTS, trackInfluencerEvent } from '../lib/track'
import type { MotionAspectRatio, PricingResponse, TaskRecord } from '../types'

export type PaywallRequest = PaywallTarget | HistoryPaywallRequest

export interface PaywallSession {
  id: number
  request: PaywallRequest
}

export interface UsePaywallRecoveryOptions {
  accountId: number | undefined
  isGuest: boolean
  historyItems: readonly TaskRecord[]
  submittedGenerations: readonly TaskRecord[]
  storedBoards: ReadonlyMap<string, TaskRecord[]>
  pricing: PricingResponse | undefined
  groups: Record<string, { ratio: number }> | undefined
  outputCapabilities: Record<string, MotionAspectRatio[]> | undefined
  hasActiveMultiView: (parentTaskId: string) => boolean
  submitSheet: (
    body: SheetSubmitBody,
    onError?: (error: unknown) => void
  ) => Promise<TaskRecord | null>
  submitMotion: (
    body: MotionSubmitBody,
    onError?: (error: unknown) => void
  ) => Promise<TaskRecord | null>
  submitMultiView: (
    body: MultiViewSubmitBody,
    storedChildren?: readonly TaskRecord[]
  ) => Promise<TaskRecord | null>
  confirmMultiViewTerminal: (parentId: string, record: TaskRecord) => void
  onGenerationAccepted?: (origin: PaywallOrigin) => void
  onResumeAccepted?: () => void
}

/**
 * Owns generation submission serialization, quota-failure paywall sessions,
 * FIFO overlapping-failure queuing, async terminal reconciliation, and
 * post-payment resumption with cross-account isolation.
 */
export function usePaywallRecovery({
  accountId,
  isGuest,
  historyItems,
  submittedGenerations,
  storedBoards,
  pricing,
  groups,
  outputCapabilities,
  hasActiveMultiView,
  submitSheet,
  submitMotion,
  submitMultiView,
  confirmMultiViewTerminal,
  onGenerationAccepted,
  onResumeAccepted,
}: UsePaywallRecoveryOptions) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const currentStoredBoards = useRef(storedBoards)
  useLayoutEffect(() => {
    currentStoredBoards.current = storedBoards
  }, [storedBoards])

  const [paywall, setPaywall] = useState<PaywallSession | null>(null)
  const activePaywall = useRef<PaywallSession | null>(null)
  const completingPaywall = useRef<PaywallSession | null>(null)
  const paywallSequence = useRef(0)
  const queuedFailures = useRef<HistoryPaywallRequest[]>([])
  const currentAccount = useRef(accountId)
  const acceptedGenerations = useRef(new Map<string, PaywallTarget>())
  const handledFailures = useRef(new Set<string>())
  const activeHistory = useRef(new Set<string>())
  const submitInFlight = useRef<Promise<TaskRecord | null> | null>(null)

  const isSubmitInFlight = useCallback(
    () => submitInFlight.current !== null,
    []
  )

  const clearPaywall = useCallback(() => {
    activePaywall.current = null
    setPaywall(null)
  }, [])

  const openPaywall = useCallback((request: PaywallRequest) => {
    if (activePaywall.current) return
    if ('task' in request) {
      queuedFailures.current = queuedFailures.current.filter(
        (entry) => entry.task.task_id !== request.task.task_id
      )
    }
    const next = { id: ++paywallSequence.current, request }
    activePaywall.current = next
    setPaywall(next)
  }, [])

  const openNextPaywall = useCallback(() => {
    if (activePaywall.current || completingPaywall.current) return
    const next = queuedFailures.current.shift()
    if (next) openPaywall(next)
  }, [openPaywall])

  const closePaywall = useCallback(() => {
    if (!paywall || activePaywall.current !== paywall) return
    clearPaywall()
    openNextPaywall()
  }, [clearPaywall, openNextPaywall, paywall])

  useEffect(() => {
    if (currentAccount.current === accountId) return
    currentAccount.current = accountId
    acceptedGenerations.current.clear()
    handledFailures.current.clear()
    activeHistory.current.clear()
    queuedFailures.current = []
    completingPaywall.current = null
    submitInFlight.current = null
    clearPaywall()
  }, [accountId, clearPaywall])

  const submitGeneration = useCallback(
    async (
      generation: GenerationRequest,
      required: number | null,
      retryRequest?: PaywallRequest
    ) => {
      if (submitInFlight.current || currentAccount.current !== accountId) {
        return false
      }
      const { origin } = generation
      const target = { origin, required, generation }
      const onError = (error: unknown) => {
        if (currentAccount.current !== accountId) return
        if (isInsufficientQuotaError(error)) {
          openPaywall(retryRequest ?? target)
          return
        }
        if (origin === 'image') {
          toastInfluencerError(t, error, 'Something went wrong!')
        } else if (origin === 'video') {
          toast.error(
            submitFailureText(t, error, {
              prefixKey: 'Video generation failed to start: {{msg}}',
              fallbackKey:
                'Video generation failed to start. Please try again.',
            })
          )
        } else {
          toast.error(
            submitFailureText(t, error, {
              prefixKey: 'Multi-view generation failed: {{msg}}',
              fallbackKey: 'Multi-view generation failed',
            })
          )
        }
      }
      let request: Promise<TaskRecord | null>
      switch (generation.origin) {
        case 'image':
          request = submitSheet(generation.body, onError)
          break
        case 'video':
          request = submitMotion(generation.body, onError)
          break
        case 'multiview':
          request = submitMultiView(
            generation.body,
            currentStoredBoards.current.get(generation.body.parentTaskId)
          )
          break
      }
      submitInFlight.current = request
      try {
        const record = await request
        if (!record || currentAccount.current !== accountId) return false
        acceptedGenerations.current.set(record.task_id, target)
        void queryClient.invalidateQueries({
          queryKey: ['influencer', 'history'],
        })
        onGenerationAccepted?.(origin)
        return true
      } catch (error) {
        onError(error)
        return false
      } finally {
        if (submitInFlight.current === request) submitInFlight.current = null
      }
    },
    [
      accountId,
      onGenerationAccepted,
      openPaywall,
      queryClient,
      submitMotion,
      submitMultiView,
      submitSheet,
      t,
    ]
  )

  const onTopUpTask = useCallback(
    (task: TaskRecord, parent?: TaskRecord) => {
      openPaywall({
        task,
        parent,
        accepted: acceptedGenerations.current.get(task.task_id),
      })
    },
    [openPaywall]
  )

  const paywallTarget = useMemo(() => {
    if (!paywall) return null
    const request = paywall.request
    const target =
      'task' in request
        ? historicalPaywallTarget(
            request,
            historyItems,
            pricing,
            groups,
            outputCapabilities
          )
        : request
    if (target.generation?.origin !== 'multiview') return target
    if (hasActiveMultiView(target.generation.body.parentTaskId)) {
      return { origin: target.origin, required: null }
    }
    return target
  }, [
    groups,
    hasActiveMultiView,
    historyItems,
    outputCapabilities,
    paywall,
    pricing,
  ])

  const currentPaywallTarget = useRef(paywallTarget)
  useLayoutEffect(() => {
    currentPaywallTarget.current = paywallTarget
  }, [paywallTarget])

  const shownPaywall = useRef(0)
  useEffect(() => {
    if (
      !paywall ||
      !paywallTarget ||
      shownPaywall.current === paywall.id ||
      currentAccount.current !== accountId ||
      activePaywall.current !== paywall
    ) {
      return
    }
    shownPaywall.current = paywall.id
    trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PAYWALL_VIEW, {
      origin: paywallTarget.origin,
      required_credits: paywallTarget.required ?? -1,
    })
  }, [accountId, paywall, paywallTarget])

  useEffect(() => {
    if (isGuest) return
    const candidates = new Map<string, TaskRecord>()
    for (const task of submittedGenerations) {
      if (acceptedGenerations.current.has(task.task_id)) {
        candidates.set(task.task_id, task)
      }
    }
    for (const task of historyItems) {
      if (!task.task_id) continue
      if (!isTerminalStatus(task.status)) {
        activeHistory.current.add(task.task_id)
        continue
      }
      const wasActive = activeHistory.current.delete(task.task_id)
      const submitted = candidates.get(task.task_id)
      if (submitted && !isTerminalStatus(submitted.status)) {
        candidates.set(task.task_id, task)
        const generation = acceptedGenerations.current.get(
          task.task_id
        )?.generation
        if (
          generation?.origin === 'multiview' &&
          isInsufficientQuotaFailure(task.fail_reason)
        ) {
          confirmMultiViewTerminal(generation.body.parentTaskId, task)
        }
      } else if (wasActive && isInsufficientQuotaFailure(task.fail_reason)) {
        candidates.set(task.task_id, task)
      }
    }
    for (const [id, task] of candidates) {
      if (!id || !isTerminalStatus(task.status)) continue
      if (isSuccessStatus(task)) {
        acceptedGenerations.current.delete(id)
        continue
      }
      if (handledFailures.current.has(id)) continue
      handledFailures.current.add(id)
      if (isInsufficientQuotaFailure(task.fail_reason)) {
        const current = activePaywall.current?.request
        if (!current || !('task' in current) || current.task.task_id !== id) {
          queuedFailures.current.push({
            task,
            accepted: acceptedGenerations.current.get(id),
          })
        }
        continue
      }
      acceptedGenerations.current.delete(id)
      if (taskKind(task) === 'sheet') {
        toast.error(describeFailReason(t, task.fail_reason).message)
      }
    }
    openNextPaywall()
  }, [
    submittedGenerations,
    historyItems,
    confirmMultiViewTerminal,
    isGuest,
    openNextPaywall,
    t,
  ])

  const checkResumeAfterPayment = useCallback(async () => {
    if (
      !paywall ||
      activePaywall.current !== paywall ||
      useAuthStore.getState().auth.user?.id !== accountId
    ) {
      return false
    }
    if (!('task' in paywall.request)) return true
    const generation = currentPaywallTarget.current?.generation
    if (!generation) return false
    if (generation.origin === 'video') return true
    try {
      const [latestPricing, latestGroups] = await fetchPaymentImageCatalog()
      return Boolean(
        quoteImageGenerationRoute(generation, latestPricing, latestGroups)
      )
    } catch {
      // Keep the successful top-up when a route cannot be confirmed.
      return false
    }
  }, [accountId, paywall])

  const handleSubscribed = useCallback(
    async (canResume: boolean) => {
      if (!paywall || activePaywall.current !== paywall) return
      if (submitInFlight.current) await submitInFlight.current.catch(() => null)
      if (
        activePaywall.current !== paywall ||
        currentAccount.current !== accountId ||
        useAuthStore.getState().auth.user?.id !== accountId
      ) {
        return
      }
      const target = currentPaywallTarget.current
      if (!target) return
      completingPaywall.current = paywall
      clearPaywall()
      trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PAYWALL_SUBSCRIBED, {
        origin: target.origin,
      })
      try {
        if (!canResume || !target.generation) return
        const accepted = await submitGeneration(
          target.generation,
          target.required,
          paywall.request
        )
        if (accepted && currentAccount.current === accountId) {
          onResumeAccepted?.()
        }
      } finally {
        if (completingPaywall.current === paywall) {
          completingPaywall.current = null
          openNextPaywall()
        }
      }
    },
    [
      accountId,
      clearPaywall,
      onResumeAccepted,
      openNextPaywall,
      paywall,
      submitGeneration,
    ]
  )

  return {
    paywall,
    paywallTarget,
    isSubmitInFlight,
    submitGeneration,
    onTopUpTask,
    closePaywall,
    checkResumeAfterPayment,
    handleSubscribed,
  }
}
