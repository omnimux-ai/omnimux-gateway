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
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import {
  fetchPaymentImageCatalog,
  INFLUENCER_ACTION,
  INFLUENCER_MOTION_ACTION,
  type MotionSubmitBody,
  type MultiViewSubmitBody,
  type SheetSubmitBody,
} from '../api'
import {
  usePaywallRecovery,
  type UsePaywallRecoveryOptions,
} from '../hooks/use-paywall-recovery'
import type { PricingResponse, TaskRecord } from '../types'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    fetchPaymentImageCatalog: vi.fn(),
  }
})

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}))

const fetchPaymentImageCatalogMock = vi.mocked(fetchPaymentImageCatalog)

const ACCOUNT_1: AuthUser = { id: 1, username: 'alice', role: 1, quota: 0 }
const ACCOUNT_2: AuthUser = { id: 2, username: 'bob', role: 1, quota: 0 }

const PRICING: PricingResponse = {
  success: true,
  data: [
    {
      model_name: 'gpt-image-2.5-sunburst',
      quota_type: 1,
      model_price: 0.04,
      enable_groups: ['gpt-image-2.5-sunburst-image'],
      supported_endpoint_types: ['image-generation'],
      vendor_id: 1,
    },
  ],
  vendors: [{ id: 1, name: 'OpenAI' }],
  group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
}

const GROUPS = {
  'gpt-image-2.5-sunburst-image': { ratio: 1 },
}

const SHEET_BODY: SheetSubmitBody = {
  model: 'gpt-image-2.5-sunburst',
  group: 'gpt-image-2.5-sunburst-image',
  tier: 'total',
  selection: { body: ['curvy'] },
  brief: '',
  size: '1024x576',
  image_mode: 'text',
}

const MOTION_BODY: MotionSubmitBody = {
  mode: 'motion',
  prompt: 'walk forward',
  subject_image_url: 'https://files.example/character.png',
  subject_image_urls: ['https://files.example/character.png'],
  video_url: 'https://files.example/reference.mp4',
  duration_sec: 5,
  aspect_ratio: '9:16',
  resolution: '480p',
}

const MULTIVIEW_BODY: MultiViewSubmitBody = {
  model: 'gpt-image-2.5-sunburst',
  group: 'gpt-image-2.5-sunburst-image',
  parentTaskId: 'parent-1',
  imageUrl: 'https://files.example/v1/tasks/parent-1/artifacts/image/content',
  size: '16:9',
}

const PARENT_TASK: TaskRecord = {
  id: 10,
  task_id: 'parent-1',
  action: INFLUENCER_ACTION,
  status: 'SUCCESS',
  properties: {
    origin_model_name: 'gpt-image-2.5-sunburst',
    input: JSON.stringify({
      tier: 'total',
      selection: { body: ['curvy'] },
      size: '1024x576',
    }),
  },
  artifacts: [
    {
      key: 'image',
      mime_type: 'image/png',
      content_url:
        'https://files.example/v1/tasks/parent-1/artifacts/image/content',
    },
  ],
}

function quota403Error() {
  return {
    response: {
      status: 403,
      data: {
        error: {
          code: 'insufficient_user_quota',
          message: 'Insufficient user quota',
        },
      },
    },
  }
}

function failedSheetTask(
  taskId: string,
  failReason = 'Insufficient user quota'
): TaskRecord {
  return {
    id: 101,
    task_id: taskId,
    action: INFLUENCER_ACTION,
    status: 'FAILURE',
    fail_reason: failReason,
    group: 'gpt-image-2.5-sunburst-image',
    properties: {
      origin_model_name: 'gpt-image-2.5-sunburst',
      input: JSON.stringify({
        tier: 'total',
        selection: { body: ['curvy'] },
        size: '1024x576',
      }),
    },
  }
}

function failedMultiViewTask(
  taskId: string,
  parentId = 'parent-1',
  failReason = 'Insufficient user quota'
): TaskRecord {
  return {
    id: 102,
    task_id: taskId,
    action: INFLUENCER_ACTION,
    status: 'FAILURE',
    fail_reason: failReason,
    group: 'gpt-image-2.5-sunburst-image',
    properties: {
      origin_model_name: 'gpt-image-2.5-sunburst',
      input: JSON.stringify({
        kind: 'multiview',
        parent_task_id: parentId,
        image_url:
          'https://files.example/v1/tasks/parent-1/artifacts/image/content',
        size: '16:9',
      }),
    },
  }
}

function failedMotionTask(
  taskId: string,
  failReason = 'Insufficient user quota'
): TaskRecord {
  return {
    id: 103,
    task_id: taskId,
    action: INFLUENCER_MOTION_ACTION,
    status: 'FAILURE',
    fail_reason: failReason,
    properties: {
      input: JSON.stringify(MOTION_BODY),
    },
  }
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function createOptions(
  overrides: Partial<UsePaywallRecoveryOptions> = {}
): UsePaywallRecoveryOptions {
  return {
    accountId: ACCOUNT_1.id,
    isGuest: false,
    historyItems: [],
    submittedGenerations: [],
    storedBoards: new Map(),
    pricing: PRICING,
    groups: GROUPS,
    outputCapabilities: undefined,
    hasActiveMultiView: vi.fn(() => false),
    submitSheet: vi.fn(async () => null),
    submitMotion: vi.fn(async () => null),
    submitMultiView: vi.fn(async () => null),
    confirmMultiViewTerminal: vi.fn(),
    onGenerationAccepted: vi.fn(),
    onResumeAccepted: vi.fn(),
    ...overrides,
  }
}

function setupHook(initialOverrides: Partial<UsePaywallRecoveryOptions> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const initialOptions = createOptions(initialOverrides)
  const rendered = renderHook((props) => usePaywallRecovery(props), {
    initialProps: initialOptions,
    wrapper,
  })
  return { ...rendered, options: initialOptions, queryClient, invalidateSpy }
}

let trackMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  useAuthStore.getState().auth.setUser(ACCOUNT_1)
  trackMock = vi.fn()
  vi.stubGlobal('__OMNIMUX_TRACK__', trackMock)
  fetchPaymentImageCatalogMock.mockResolvedValue([PRICING, GROUPS])
  vi.clearAllMocks()
})

afterEach(() => {
  useAuthStore.getState().auth.reset()
  vi.unstubAllGlobals()
})

describe('usePaywallRecovery', () => {
  it.each(['image', 'video', 'multiview'] as const)(
    'opens paywall on sync 403 quota rejection for %s and emits PAYWALL_VIEW once without error toast',
    async (origin) => {
      const submitSheet = vi.fn(
        async (_body: SheetSubmitBody, onError?: (err: unknown) => void) => {
          onError?.(quota403Error())
          return null
        }
      )
      const submitMotion = vi.fn(
        async (_body: MotionSubmitBody, onError?: (err: unknown) => void) => {
          onError?.(quota403Error())
          return null
        }
      )
      const submitMultiView = vi.fn(async () => {
        throw quota403Error()
      })
      const { result, rerender, options } = setupHook({
        submitSheet,
        submitMotion,
        submitMultiView,
      })

      const requests = {
        image: { origin: 'image', body: SHEET_BODY },
        video: { origin: 'video', body: MOTION_BODY },
        multiview: { origin: 'multiview', body: MULTIVIEW_BODY },
      } as const
      const request = requests[origin]

      let accepted = true
      await act(async () => {
        accepted = await result.current.submitGeneration(request, 12)
      })

      expect(accepted).toBe(false)
      expect(result.current.paywall).not.toBeNull()
      expect(result.current.paywallTarget).toEqual({
        origin,
        required: 12,
        generation: request,
      })
      expect(toast.error).not.toHaveBeenCalled()
      expect(options.onGenerationAccepted).not.toHaveBeenCalled()

      await waitFor(() =>
        expect(
          trackMock.mock.calls.filter(
            ([event]) => event === 'influencer_paywall_view'
          )
        ).toEqual([
          ['influencer_paywall_view', { origin, required_credits: 12 }],
        ])
      )

      // Rerendering the same paywall session must not duplicate PAYWALL_VIEW.
      rerender({ ...options, historyItems: [PARENT_TASK] })
      expect(
        trackMock.mock.calls.filter(
          ([event]) => event === 'influencer_paywall_view'
        )
      ).toHaveLength(1)
    }
  )

  it('queues overlapping async quota failures in strict FIFO order and tracks each once', async () => {
    const mvQueued: TaskRecord = {
      ...failedMultiViewTask('mv-1'),
      status: 'IN_PROGRESS',
      fail_reason: undefined,
    }
    const imgQueued: TaskRecord = {
      ...failedSheetTask('img-2'),
      status: 'QUEUED',
      fail_reason: undefined,
    }
    const vidQueued: TaskRecord = {
      ...failedMotionTask('vid-3'),
      status: 'IN_PROGRESS',
      fail_reason: undefined,
    }

    const { result, rerender, options } = setupHook({
      historyItems: [PARENT_TASK, mvQueued, imgQueued, vidQueued],
    })
    expect(result.current.paywall).toBeNull()

    // First failure opens immediately.
    rerender({
      ...options,
      historyItems: [
        PARENT_TASK,
        failedMultiViewTask('mv-1'),
        imgQueued,
        vidQueued,
      ],
    })
    await waitFor(() =>
      expect(result.current.paywallTarget?.origin).toBe('multiview')
    )

    // While the first paywall is open, two more failures arrive in order: image, then video.
    rerender({
      ...options,
      historyItems: [
        PARENT_TASK,
        failedMultiViewTask('mv-1'),
        failedSheetTask('img-2'),
        vidQueued,
      ],
    })
    rerender({
      ...options,
      historyItems: [
        PARENT_TASK,
        failedMultiViewTask('mv-1'),
        failedSheetTask('img-2'),
        failedMotionTask('vid-3'),
      ],
    })

    // First paywall stays active and only its view event has been emitted.
    expect(result.current.paywallTarget?.origin).toBe('multiview')
    const paywallViews = () =>
      trackMock.mock.calls
        .filter(([event]) => event === 'influencer_paywall_view')
        .map(([, payload]) => payload.origin)
    expect(paywallViews()).toEqual(['multiview'])

    // Dismissing the first opens the second (image).
    act(() => {
      result.current.closePaywall()
    })
    await waitFor(() =>
      expect(result.current.paywallTarget?.origin).toBe('image')
    )
    expect(paywallViews()).toEqual(['multiview', 'image'])

    // Dismissing the second opens the third (video).
    act(() => {
      result.current.closePaywall()
    })
    await waitFor(() =>
      expect(result.current.paywallTarget?.origin).toBe('video')
    )
    expect(paywallViews()).toEqual(['multiview', 'image', 'video'])

    // Dismissing the third drains the queue completely without re-opening.
    act(() => {
      result.current.closePaywall()
    })
    expect(result.current.paywall).toBeNull()
    rerender({
      ...options,
      historyItems: [
        PARENT_TASK,
        failedMultiViewTask('mv-1'),
        failedSheetTask('img-2'),
        failedMotionTask('vid-3'),
      ],
    })
    expect(result.current.paywall).toBeNull()
    expect(paywallViews()).toEqual(['multiview', 'image', 'video'])
  })

  it('enforces the sync submitInFlight re-entry lock and resumes after payment once', async () => {
    const firstSubmit = createDeferred<TaskRecord | null>()
    const retrySubmit = createDeferred<TaskRecord | null>()
    const submitSheet = vi
      .fn()
      .mockImplementationOnce(() => firstSubmit.promise)
      .mockImplementationOnce(
        async (_body: SheetSubmitBody, onError?: (err: unknown) => void) => {
          onError?.(quota403Error())
          return null
        }
      )
      .mockImplementationOnce(() => retrySubmit.promise)

    const { result, options, invalidateSpy } = setupHook({ submitSheet })
    expect(result.current.isSubmitInFlight()).toBe(false)

    // Launch first submission and confirm sync lock blocks concurrent re-entry.
    let firstPromise!: Promise<boolean>
    act(() => {
      firstPromise = result.current.submitGeneration(
        { origin: 'image', body: SHEET_BODY },
        4
      )
    })
    expect(result.current.isSubmitInFlight()).toBe(true)

    let blocked = true
    await act(async () => {
      blocked = await result.current.submitGeneration(
        { origin: 'image', body: SHEET_BODY },
        4
      )
    })
    expect(blocked).toBe(false)
    expect(submitSheet).toHaveBeenCalledTimes(1)

    // Complete first submission and release the lock.
    await act(async () => {
      firstSubmit.resolve({
        id: 1,
        task_id: 'sheet-accepted-1',
        action: INFLUENCER_ACTION,
        status: 'QUEUED',
      })
      expect(await firstPromise).toBe(true)
    })
    expect(result.current.isSubmitInFlight()).toBe(false)
    expect(options.onGenerationAccepted).toHaveBeenCalledExactlyOnceWith(
      'image'
    )
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ['influencer', 'history'],
    })

    // Trigger 403 paywall, verify checkResumeAfterPayment, then resume via handleSubscribed.
    await act(async () => {
      expect(
        await result.current.submitGeneration(
          { origin: 'image', body: SHEET_BODY },
          4
        )
      ).toBe(false)
    })
    expect(result.current.paywall).not.toBeNull()
    expect(await result.current.checkResumeAfterPayment()).toBe(true)

    let resumePromise!: Promise<void>
    act(() => {
      resumePromise = result.current.handleSubscribed(true)
    })
    expect(result.current.paywall).toBeNull()
    expect(result.current.isSubmitInFlight()).toBe(true)

    await act(async () => {
      retrySubmit.resolve({
        id: 2,
        task_id: 'sheet-resumed-2',
        action: INFLUENCER_ACTION,
        status: 'QUEUED',
      })
      await resumePromise
    })

    expect(result.current.isSubmitInFlight()).toBe(false)
    expect(submitSheet).toHaveBeenCalledTimes(3)
    expect(options.onResumeAccepted).toHaveBeenCalledOnce()
    expect(
      trackMock.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toEqual([['influencer_paywall_subscribed', { origin: 'image' }]])
  })

  it('intercepts in-flight submissions and post-payment resume on account switch without cross-account leakage', async () => {
    const account1Submit = createDeferred<TaskRecord | null>()
    const account2Submit = createDeferred<TaskRecord | null>()
    const submitSheet = vi
      .fn()
      .mockImplementationOnce(() => account1Submit.promise)
      .mockImplementationOnce(() => account2Submit.promise)

    const { result, rerender, options, invalidateSpy } = setupHook({
      submitSheet,
    })

    // Account 1 starts a submission.
    let account1Promise!: Promise<boolean>
    act(() => {
      account1Promise = result.current.submitGeneration(
        { origin: 'image', body: SHEET_BODY },
        4
      )
    })
    expect(result.current.isSubmitInFlight()).toBe(true)

    // Switch to Account 2 while Account 1's submission is still in flight.
    act(() => {
      useAuthStore.getState().auth.setUser(ACCOUNT_2)
    })
    rerender({ ...options, accountId: ACCOUNT_2.id })
    expect(result.current.isSubmitInFlight()).toBe(false)

    // Account 2 starts its own submission.
    let account2Promise!: Promise<boolean>
    act(() => {
      account2Promise = result.current.submitGeneration(
        { origin: 'image', body: SHEET_BODY },
        4
      )
    })
    expect(result.current.isSubmitInFlight()).toBe(true)

    // Account 1's stale request resolves late: must be discarded and must not clear Account 2's lock.
    await act(async () => {
      account1Submit.resolve({
        id: 99,
        task_id: 'stale-account-1-task',
        action: INFLUENCER_ACTION,
        status: 'QUEUED',
      })
      expect(await account1Promise).toBe(false)
    })
    expect(result.current.isSubmitInFlight()).toBe(true)
    expect(options.onGenerationAccepted).not.toHaveBeenCalled()
    expect(invalidateSpy).not.toHaveBeenCalled()

    // Account 2's submission completes normally.
    await act(async () => {
      account2Submit.resolve({
        id: 100,
        task_id: 'account-2-task',
        action: INFLUENCER_ACTION,
        status: 'QUEUED',
      })
      expect(await account2Promise).toBe(true)
    })
    expect(result.current.isSubmitInFlight()).toBe(false)
    expect(options.onGenerationAccepted).toHaveBeenCalledExactlyOnceWith(
      'image'
    )

    // Open a paywall on Account 2, then switch auth store synchronously before handleSubscribed runs.
    act(() => {
      result.current.onTopUpTask(failedSheetTask('img-acc2'))
    })
    expect(result.current.paywall).not.toBeNull()

    act(() => {
      useAuthStore.getState().auth.setUser(ACCOUNT_1)
    })
    expect(await result.current.checkResumeAfterPayment()).toBe(false)

    await act(async () => {
      await result.current.handleSubscribed(true)
    })
    expect(submitSheet).toHaveBeenCalledTimes(2)
    expect(options.onResumeAccepted).not.toHaveBeenCalled()
    expect(
      trackMock.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toHaveLength(0)

    // Completing the React account switch clears paywall state for Account 1.
    rerender({ ...options, accountId: ACCOUNT_1.id })
    expect(result.current.paywall).toBeNull()
  })

  it('calls confirmMultiViewTerminal when history confirms a multiview quota failure before polling catches up', async () => {
    const acceptedBoard: TaskRecord = {
      id: 0,
      task_id: 'board-1',
      action: INFLUENCER_ACTION,
      status: 'QUEUED',
      properties: {
        input: JSON.stringify({
          kind: 'multiview',
          parent_task_id: 'parent-1',
        }),
      },
    }
    const submitMultiView = vi.fn(async () => acceptedBoard)
    const confirmMultiViewTerminal = vi.fn()
    const { result, rerender, options } = setupHook({
      historyItems: [PARENT_TASK],
      submitMultiView,
      confirmMultiViewTerminal,
    })

    await act(async () => {
      expect(
        await result.current.submitGeneration(
          { origin: 'multiview', body: MULTIVIEW_BODY },
          4
        )
      ).toBe(true)
    })

    // Local submittedGenerations still sees QUEUED while history already reports FAILURE.
    const terminalBoard = failedMultiViewTask('board-1', 'parent-1')
    rerender({
      ...options,
      submittedGenerations: [acceptedBoard],
      historyItems: [PARENT_TASK, terminalBoard],
    })

    expect(confirmMultiViewTerminal).toHaveBeenCalledExactlyOnceWith(
      'parent-1',
      terminalBoard
    )
    await waitFor(() =>
      expect(result.current.paywallTarget).toEqual({
        origin: 'multiview',
        required: 0.4,
        generation: { origin: 'multiview', body: MULTIVIEW_BODY },
      })
    )
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('holds the next queued failure until completingPaywall finishes and supports top-up-only when route recheck fails', async () => {
    const resumeDeferred = createDeferred<TaskRecord | null>()
    const submitSheet = vi.fn(() => resumeDeferred.promise)

    const img1Queued: TaskRecord = {
      ...failedSheetTask('img-1'),
      status: 'IN_PROGRESS',
      fail_reason: undefined,
    }
    const vid2Queued: TaskRecord = {
      ...failedMotionTask('vid-2'),
      status: 'QUEUED',
      fail_reason: undefined,
    }

    const { result, rerender, options } = setupHook({
      historyItems: [img1Queued, vid2Queued],
      submitSheet,
    })

    rerender({
      ...options,
      historyItems: [failedSheetTask('img-1'), failedMotionTask('vid-2')],
    })
    await waitFor(() =>
      expect(result.current.paywallTarget?.origin).toBe('image')
    )

    // Route recheck returns false when catalog fetch fails.
    fetchPaymentImageCatalogMock.mockRejectedValueOnce(new Error('network'))
    expect(await result.current.checkResumeAfterPayment()).toBe(false)

    // Route recheck succeeds when pricing + groups confirm the original route.
    fetchPaymentImageCatalogMock.mockResolvedValueOnce([PRICING, GROUPS])
    expect(await result.current.checkResumeAfterPayment()).toBe(true)

    // While img-1 is resuming, vid-2 must NOT pop open prematurely.
    let subscribedPromise!: Promise<void>
    act(() => {
      subscribedPromise = result.current.handleSubscribed(true)
    })
    expect(result.current.paywall).toBeNull()

    await act(async () => {
      resumeDeferred.resolve({
        id: 201,
        task_id: 'img-1-retried',
        action: INFLUENCER_ACTION,
        status: 'QUEUED',
      })
      await subscribedPromise
    })

    // Once img-1 finishes resuming, vid-2 pops open next in FIFO order.
    await waitFor(() =>
      expect(result.current.paywallTarget?.origin).toBe('video')
    )
    expect(
      trackMock.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toEqual([['influencer_paywall_subscribed', { origin: 'image' }]])
  })

  it('degrades multiview paywallTarget to top-up-only when another board for the same parent is active', async () => {
    const { result } = setupHook({
      historyItems: [PARENT_TASK, failedMultiViewTask('mv-blocked')],
      hasActiveMultiView: vi.fn((parentId) => parentId === 'parent-1'),
    })

    act(() => {
      result.current.onTopUpTask(failedMultiViewTask('mv-blocked'), PARENT_TASK)
    })

    expect(result.current.paywallTarget).toEqual({
      origin: 'multiview',
      required: null,
    })
    expect(await result.current.checkResumeAfterPayment()).toBe(false)
  })

  it('toasts non-quota async sheet failures without opening paywall and skips resubmit when canResume is false', async () => {
    const acceptedSheet: TaskRecord = {
      id: 0,
      task_id: 'sheet-timeout',
      action: INFLUENCER_ACTION,
      status: 'QUEUED',
    }
    const submitSheet = vi.fn(async () => acceptedSheet)
    const { result, rerender, options } = setupHook({ submitSheet })

    await act(async () => {
      expect(
        await result.current.submitGeneration(
          { origin: 'image', body: SHEET_BODY },
          4
        )
      ).toBe(true)
    })

    rerender({
      ...options,
      submittedGenerations: [
        { ...acceptedSheet, status: 'FAILURE', fail_reason: 'Task timed out' },
      ],
    })
    expect(result.current.paywall).toBeNull()
    expect(toast.error).toHaveBeenCalledWith('Task timed out')

    // Opening top-up and finishing with canResume=false tracks PAYWALL_SUBSCRIBED without resubmitting.
    act(() => {
      result.current.onTopUpTask(failedSheetTask('sheet-topup-only'))
    })
    expect(result.current.paywall).not.toBeNull()
    await act(async () => {
      await result.current.handleSubscribed(false)
    })
    expect(submitSheet).toHaveBeenCalledTimes(1)
    expect(options.onResumeAccepted).not.toHaveBeenCalled()
    expect(
      trackMock.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toEqual([['influencer_paywall_subscribed', { origin: 'image' }]])
  })
})
