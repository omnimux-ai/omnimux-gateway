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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearch } from '@tanstack/react-router'
import { Plus, Compass, History } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import { getSelf } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import './influencer.css'
import {
  deletePublishedPreset,
  fetchInfluencerTasks,
  fetchPricing,
  fetchTaskRecord,
  INFLUENCER_MOTION_ACTION,
  fetchPublishedPresets,
  fetchTaxonomy,
  fetchUserGroups,
  publishInfluencerPreset,
  IMAGE_SIZE,
  SHEET_SIZE,
} from './api'
import { BuilderPanel } from './components/builder-panel'
import { CharacterPickerPanel } from './components/character-picker-panel'
import { ExploreHero } from './components/explore-hero'
import { ExploreMotionGrid } from './components/explore-motion-grid'
import { ExplorePresetGrid } from './components/explore-preset-grid'
import { ExploreSkeleton } from './components/explore-skeleton'
import { ExploreTabs } from './components/explore-tabs'
import { FeedGrid } from './components/feed-grid'
import { GenerateBar } from './components/generate-bar'
import { HistoryPreviewDialog } from './components/history-preview-dialog'
import { LoginGateDialog } from './components/login-gate-dialog'
import { MediaSlot } from './components/media-slot'
import { MotionPanel } from './components/motion-panel'
import { MotionPresetDialog } from './components/motion-preset-dialog'
import { MultiViewDialog } from './components/multi-view-dialog'
import { PresetPreviewDialog } from './components/preset-preview-dialog'
import { PublishPresetDialog } from './components/publish-preset-dialog'
import { CharacterSettingsSummary } from './components/settings-summary'
import { SourceTabs } from './components/source-tabs'
import { SubscribePaywall } from './components/subscribe-paywall'
import { VideoPickerPanel } from './components/video-picker-panel'
import { WorkbenchModeTabs } from './components/workbench-mode-tabs'
import { useBuilder } from './hooks/use-builder'
import { useGenerationQueueNotice } from './hooks/use-generation-queue-notice'
import { useLoginGate } from './hooks/use-login-gate'
import { RECREATE_MAX_IMAGES, useMotion } from './hooks/use-motion'
import { useMultiViewSubmit } from './hooks/use-multiview'
import {
  useInfluencerHistory,
  useMotionSubmit,
  useSheetSubmit,
} from './hooks/use-tasks'
import { useUploadLibrary } from './hooks/use-upload-library'
import { normalizeSelection } from './lib/builder-draft'
import {
  imageModels,
  quoteCredits,
  resolveImageRoute,
  SUNBURST_MODEL,
} from './lib/catalog'
import { submitFailureText, toastInfluencerError } from './lib/error-copy'
import { readExploreTab, writeExploreTab } from './lib/explore-tab'
import { describeFailReason } from './lib/fail-reason'
import { takeGuestDraft } from './lib/guest'
import { hiddenTaskIds, hideTask } from './lib/hidden'
import { rollableCount } from './lib/randomize'
import {
  getTaskTimestamp,
  isTerminalStatus,
  parseTaskMeta,
  taskAbsoluteImageUrl,
  resolveTaskCharacterSource,
  taskImageUrl,
  taskKind,
} from './lib/history'
import {
  creditsFromQuota,
  isInsufficientQuotaError,
  type PaywallOrigin,
} from './lib/insufficient-credits'
import {
  readPresetsCache,
  readTaxonomyCache,
  writePresetsCache,
  writeTaxonomyCache,
} from './lib/library-cache'
import {
  communityTrendPresets,
  motionExplorePresets,
} from './lib/motion-explore'
import {
  canDeriveMultiView,
  MULTIVIEW_SIZE,
  partitionTasks,
  taskKey,
} from './lib/multiview'
import {
  applyPresetSelection,
  publishedPresets,
  usablePresets,
} from './lib/presets'
import {
  classifyReference,
  type PreviewEntry,
  type PreviewMediaKind,
} from './lib/preview-reference'
import { recreateQuoteFor } from './lib/recreate'
import { referenceVideoCandidate } from './lib/reference-candidates'
import { readSource, writeSource } from './lib/source'
import {
  buildMotionSubmitPayload,
  parseStoredMotionPayload,
} from './lib/submit-payload'
import {
  conflictMessage,
  isAnimalTier,
  isCharacterTier,
  totalSelected,
  type RemovedPick,
} from './lib/taxonomy'
import { INFLUENCER_ANALYTICS_EVENTS, trackInfluencerEvent } from './lib/track'
import { quoteVideoCredits } from './lib/video-quote'
import { readWorkbenchMode, writeWorkbenchMode } from './lib/workbench-mode'
import type {
  ExplorePreset,
  InfluencerExploreTab,
  InfluencerSource,
  MotionExplorePreset,
  Selection,
  TaskRecord,
  WorkbenchMode,
} from './types'

/** 一次生成所需的形象参数；显式传入时不再读面板当前状态。 */
interface SubmitSheet {
  tier: string
  selection: Selection
  seed?: string
  image_url?: string
}

/** 只有真正生成成功的形象才可发布；失败或进行中的记录没有成品素材。 */
function isSuccessStatus(record: TaskRecord): boolean {
  const status = (record.status ?? '').toUpperCase()
  return status === 'SUCCESS' || status === 'SUCCEEDED'
}

export function InfluencerWorkbench() {
  const { t } = useTranslation()
  const search = useSearch({ strict: false }) as { recreate?: string }
  const queryClient = useQueryClient()
  // 灵感库的发布/下架是管理员特权；未登录或资料未就绪时按普通用户处理。
  const role = useAuthStore((s) => s.auth.user?.role ?? 0)
  const isAdmin = role >= ROLE.ADMIN
  const loginGate = useLoginGate()
  const { isGuest, requireLogin } = loginGate

  // 工作台的两个面：角色设计与动态视频。各自的历史记录分开列出，
  // 视频面的生成键提交视频任务。
  const [workbenchMode, setWorkbenchMode] = useState<WorkbenchMode>(() =>
    readWorkbenchMode()
  )
  const motion = useMotion()
  const accountId = useAuthStore((state) => state.auth.user?.id)
  const previousAccount = useRef(accountId)
  const resetMotion = motion.reset
  useEffect(() => {
    if (previousAccount.current === accountId) return
    previousAccount.current = accountId
    resetMotion()
  }, [accountId, resetMotion])

  const closeVideoPicker = motion.closeVideoPicker
  const closeCharacterPicker = motion.closeCharacterPicker
  const onWorkbenchModeChange = useCallback(
    (next: WorkbenchMode) => {
      closeVideoPicker()
      closeCharacterPicker()
      setWorkbenchMode(next)
      writeWorkbenchMode(next)
    },
    [closeVideoPicker, closeCharacterPicker]
  )

  const cachedTaxonomy = useMemo(() => readTaxonomyCache(), [])
  const cachedPresets = useMemo(() => readPresetsCache(), [])

  const taxonomyQ = useQuery({
    queryKey: ['influencer', 'taxonomy'],
    queryFn: fetchTaxonomy,
    initialData: cachedTaxonomy ?? undefined,
    initialDataUpdatedAt: 0,
  })
  const pricingQ = useQuery({
    queryKey: ['influencer', 'pricing', accountId],
    queryFn: fetchPricing,
  })
  const groupsQ = useQuery({
    queryKey: ['influencer', 'groups', accountId],
    queryFn: fetchUserGroups,
    enabled: !isGuest,
  })
  const presetsQ = useQuery({
    queryKey: ['influencer', 'presets'],
    queryFn: fetchPublishedPresets,
    initialData: cachedPresets ?? undefined,
    initialDataUpdatedAt: 0,
    staleTime: 5 * 60 * 1000,
  })
  // 历史记录聚合显示：图片与视频任务不分页签，统一按时间排序混排。
  const historyQ = useInfluencerHistory(!isGuest)
  const sheetHistoryQ = historyQ
  // Both pickers read this one merged page set. A motion-only query here used
  // to stay cold until the video panel opened, so the first open waited for a
  // list request; loading more may also fetch an image page, which is cheap
  // compared with a second polling query.

  const taxonomyData = taxonomyQ.data
  const fetchedTaxonomyAfterMount = taxonomyQ.isFetchedAfterMount
  useEffect(() => {
    if (!fetchedTaxonomyAfterMount || !taxonomyData) return
    writeTaxonomyCache(taxonomyData)
  }, [fetchedTaxonomyAfterMount, taxonomyData])

  const presetsData = presetsQ.data
  const fetchedPresetsAfterMount = presetsQ.isFetchedAfterMount
  useEffect(() => {
    if (!fetchedPresetsAfterMount || !presetsData) return
    writePresetsCache(presetsData)
  }, [fetchedPresetsAfterMount, presetsData])

  const taxonomy = taxonomyQ.data?.taxonomy ?? null
  const builder = useBuilder(taxonomy)
  const resetBuilder = builder.reset
  const setImageModel = builder.setModel
  const currentImageModel = builder.state.model
  const previousBuilderAccount = useRef(accountId)
  useEffect(() => {
    if (previousBuilderAccount.current === accountId) return
    previousBuilderAccount.current = accountId
    resetBuilder()
  }, [accountId, resetBuilder])
  // Restore the look a visitor assembled on the landing page or before
  // signing in — consumed once, for guests and signed-in users alike.
  const draftRestored = useRef(false)
  useEffect(() => {
    if (draftRestored.current || !taxonomy) return
    draftRestored.current = true
    const draft = takeGuestDraft()
    if (!draft) return
    const draftTier = draft.tier
    builder.applyLook({
      tier: draftTier,
      values: normalizeSelection(taxonomy, draftTier, draft.selection),
      locks: draft.locks,
      image_url: draft.imageUrl,
      image_mime_type: draft.imageMimeType,
      model: draft.model || undefined,
      group: draft.group || undefined,
    })
    if (draft.vendorId != null) builder.setVendorId(draft.vendorId)
    // A visitor who arrived through the landing's photo entry was promised the
    // photo slot, which lives on the video face.
    if (draft.mode && draft.mode !== workbenchMode) {
      onWorkbenchModeChange(draft.mode)
    }
    builder.setSeed(draft.seed)
  }, [isGuest, taxonomy, builder, workbenchMode, onWorkbenchModeChange])

  // Funnel start: separate guest landings from signed-in visits.
  const pageViewed = useRef(false)
  useEffect(() => {
    if (pageViewed.current) return
    pageViewed.current = true
    trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PAGE_VIEW, {
      guest: isGuest,
    })
  }, [isGuest])
  const sheetSubmit = useSheetSubmit()
  const { submitting, submit } = sheetSubmit
  const motionSubmit = useMotionSubmit()
  const multiView = useMultiViewSubmit()
  const submissionBusy =
    submitting || motionSubmit.submitting || multiView.submittingParent !== null
  const generationBusy =
    submissionBusy ||
    motion.characterUploading ||
    motion.videoMetadataLoading ||
    motion.videoPreparing
  const submitInFlight = useRef(false)
  const pending = useMemo(
    () => [...sheetSubmit.tasks, ...motionSubmit.tasks],
    [sheetSubmit.tasks, motionSubmit.tasks]
  )
  const submittedGenerations = useMemo(
    () => [...pending, ...Object.values(multiView.childByParent)],
    [pending, multiView.childByParent]
  )
  useGenerationQueueNotice(submittedGenerations, historyQ.items)
  const { submit: submitMultiViewTask, forget: forgetMultiView } = multiView
  const uploadLibrary = useUploadLibrary()
  // The history preview is a stack: references open above the record that cites them.
  const [previewStack, setPreviewStack] = useState<PreviewEntry[]>([])
  const previewRequest = useRef(0)
  /** Opens one record on its own, or closes the preview when given null. */
  const setViewTask = useCallback((task: TaskRecord | null) => {
    previewRequest.current += 1
    setPreviewStack(task ? [{ kind: 'task', task }] : [])
  }, [])
  // Detail follows the same current record as the gallery, not the opening snapshot.
  const selectedAccount = useRef(accountId)
  useEffect(() => {
    previewRequest.current += 1
    setPreviewStack([])
    return () => {
      previewRequest.current += 1
    }
  }, [accountId])
  const viewStack = useMemo<PreviewEntry[]>(() => {
    if (selectedAccount.current !== accountId) return []
    return previewStack.map((entry) => {
      if (entry.kind !== 'task') return entry
      const current = historyQ.items.find(
        (item) => item.task_id === entry.task.task_id
      )
      return current ? { ...entry, task: current } : entry
    })
  }, [previewStack, historyQ.items, accountId])
  const [mobileCreating, setMobileCreating] = useState(() => !search.recreate)
  const [source, setSource] = useState<InfluencerSource>(() => readSource())
  const [exploreTab, setExploreTab] = useState<InfluencerExploreTab>(() =>
    readExploreTab()
  )
  const [previewPreset, setPreviewPreset] = useState<ExplorePreset | null>(null)
  const [previewMotion, setPreviewMotion] =
    useState<MotionExplorePreset | null>(null)
  const [multiViewParent, setMultiViewParent] = useState<TaskRecord | null>(
    null
  )
  const [publishTask, setPublishTask] = useState<TaskRecord | null>(null)
  const [publishing, setPublishing] = useState(false)
  const recreateHandled = useRef(false)
  /** 窄屏下左侧面板在 Explore 之上，套用预设后需要把它滚回视野。 */
  const builderRef = useRef<HTMLElement | null>(null)
  const assetsRef = useRef<HTMLElement>(null)

  const onSourceChange = useCallback((next: InfluencerSource) => {
    setSource(next)
    writeSource(next)
  }, [])

  const onExploreTabChange = useCallback((next: InfluencerExploreTab) => {
    setExploreTab(next)
    writeExploreTab(next)
  }, [])

  const motionPresets = useMemo(() => motionExplorePresets(), [])
  const trendPresets = useMemo(() => communityTrendPresets(), [])

  const presets = useMemo(() => usablePresets(taxonomy), [taxonomy])
  // 自制条目排在静态竞品预设之前：管理员刚发布的形象应当第一眼可见。
  const published = useMemo(
    () => publishedPresets(presetsQ.data, taxonomy),
    [presetsQ.data, taxonomy]
  )
  const explorePresets = useMemo(
    () => [...published, ...presets],
    [published, presets]
  )

  const isExploreLoading =
    (taxonomyQ.isPending && !taxonomy) ||
    (presetsQ.isPending && !presetsQ.data && !presetsQ.isError)

  const models = useMemo(() => imageModels(pricingQ.data), [pricingQ.data])
  // The gateway is the source of truth: prefer the verified Sunburst route and
  // fall back to the audited economy route when it is not sellable right now.
  const imageRoute = useMemo(
    () => resolveImageRoute(models, groupsQ.data, pricingQ.data?.group_ratio),
    [models, groupsQ.data, pricingQ.data?.group_ratio]
  )
  const modelRow = useMemo(
    () =>
      models.find(
        (m) => m.model_name === (imageRoute?.model ?? builder.state.model)
      ),
    [models, imageRoute, builder.state.model]
  )

  // A stale draft cannot select another ID than the one the gateway sells.
  useEffect(() => {
    if (imageRoute && currentImageModel !== imageRoute.model) {
      setImageModel(imageRoute.model)
    }
  }, [imageRoute, currentImageModel, setImageModel])

  const imageGroup = imageRoute?.group ?? null
  const multiViewGroup = imageGroup
  const multiViewQuote = quoteCredits(
    modelRow,
    multiViewGroup ?? undefined,
    pricingQ.data?.group_ratio
  )
  let multiViewDisabledReason = ''
  if (builder.imageUploading || generationBusy) {
    multiViewDisabledReason = t(
      'Wait for the current upload or submission to finish'
    )
  } else if (!isGuest && (!multiViewGroup || !multiViewQuote)) {
    multiViewDisabledReason = t(
      'Style reference is not available for this model'
    )
  }
  const quote = useMemo(
    () =>
      imageGroup
        ? quoteCredits(modelRow, imageGroup, pricingQ.data?.group_ratio)
        : null,
    [modelRow, imageGroup, pricingQ.data?.group_ratio]
  )
  // The video surface bills per second from the reference clip's rounded
  // duration and the resolution tier group. Its ratio comes from the account's
  // own groups payload so subscription discounts are the charged ones, not a
  // second multiplier on top of a displayed full price.
  const motionQuote = useMemo(
    () =>
      quoteVideoCredits(
        pricingQ.data,
        groupsQ.data,
        motion.resolution,
        motion.durationSec
      ),
    [pricingQ.data, groupsQ.data, motion.resolution, motion.durationSec]
  )
  // All three modes share one model line, so the motion per-second quote is
  // also the recreation's pre-analysis amount. The analysis price replaces it
  // once the server has derived the final duration.
  const videoQuote =
    motion.mode === 'recreate'
      ? (motion.recreateQuote ?? motionQuote)
      : motionQuote

  // --- Subscription paywall: an empty wallet must never be a dead end -------
  const [paywall, setPaywall] = useState<{
    origin: PaywallOrigin
    required: number | null
  } | null>(null)
  const resumingRef = useRef(false)
  const handledSubmitErrors = useRef(new WeakSet<object>())
  const handledFailures = useRef(new Set<string>())
  const setAuthUser = useAuthStore((state) => state.auth.setUser)
  /** `undefined` means the session has no quota field yet; never gate on that. */
  const rawQuota = useAuthStore((state) => state.auth.user?.quota)
  const quotaRef = useRef<number | undefined>(rawQuota)
  useEffect(() => {
    quotaRef.current = rawQuota
  }, [rawQuota])

  const refreshQuota = useCallback(async (): Promise<number | null> => {
    try {
      const res = (await getSelf()) as { success?: boolean; data?: AuthUser }
      if (res?.success && res.data) {
        setAuthUser(res.data)
        quotaRef.current = res.data.quota
        return res.data.quota ?? 0
      }
    } catch {
      // The caller resumes regardless; the paywall already refreshed while open.
    }
    return null
  }, [setAuthUser])

  const openPaywall = useCallback(
    (origin: PaywallOrigin, required: number | null) => {
      resumingRef.current = false
      trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PAYWALL_VIEW, {
        origin,
        required_credits: required ?? -1,
      })
      setPaywall({ origin, required })
    },
    []
  )

  /**
   * A click must not become a doomed task: the generation queue accepts first
   * and bills at dispatch, so an unfunded submit only shows up later as a
   * generic failed task. Gate the click on the displayed balance; an unknown
   * balance is never treated as empty.
   */
  const ensureFunded = useCallback(
    (origin: PaywallOrigin, required: number | null): boolean => {
      const current = quotaRef.current
      if (typeof current !== 'number') return true
      const balance = creditsFromQuota(current)
      if (required === null) {
        if (balance > 0) return true
        openPaywall(origin, null)
        return false
      }
      if (balance + 1e-9 >= required) return true
      openPaywall(origin, required)
      return false
    },
    [openPaywall]
  )

  /** Returns true when the failure was an unfunded wallet and the paywall opened. */
  const handleSubmissionFailure = useCallback(
    (
      error: unknown,
      origin: PaywallOrigin,
      required: number | null
    ): boolean => {
      if (!isInsufficientQuotaError(error)) return false
      void refreshQuota().then(() => openPaywall(origin, required))
      return true
    },
    [openPaywall, refreshQuota]
  )

  const canGenerate = useMemo(() => {
    if (!taxonomy) return false
    // Guests click "Log in" — the button is a sign-in entry, not a form
    // submit, so it must never be disabled for an empty selection.
    if (isGuest) return true
    if (!quote) return false
    const tot = totalSelected(builder.state.selection)
    if (!isCharacterTier(builder.state.tier)) return false
    if (
      !isAnimalTier(builder.state.tier) &&
      tot === 0 &&
      !builder.state.imageUrl.trim()
    ) {
      return false
    }
    if (!builder.state.model || !imageGroup) {
      return false
    }
    return true
  }, [
    isGuest,
    taxonomy,
    builder.state.selection,
    builder.state.tier,
    builder.state.imageUrl,
    builder.state.model,
    quote,
    imageGroup,
  ])

  /** The look a guest would lose by leaving for sign-in — snapshot every time. */
  const currentDraft = useCallback(
    () => ({
      tier: builder.state.tier,
      selection: builder.state.selection,
      locks: builder.locks,
      brief: '',
      seed: builder.state.seed,
      imageUrl: builder.state.imageUrl,
      imageMimeType: builder.state.imageMimeType,
      model: builder.state.model,
      group: imageGroup ?? '',
      vendorId: builder.state.vendorId,
    }),
    [builder.state, builder.locks, imageGroup]
  )

  /**
   * Submits the panel as it stands, or an explicit sheet when the caller has
   * just written one (Regenerate applies a task and submits it in one step, so
   * reading the panel here would send the previous look).
   */
  const doSubmit = useCallback(
    async (sheet?: SubmitSheet): Promise<boolean> => {
      if (
        !taxonomy ||
        builder.isImageUploading() ||
        generationBusy ||
        submitInFlight.current
      ) {
        return false
      }
      const next: SubmitSheet = sheet ?? {
        tier: builder.state.tier,
        selection: builder.state.selection,
        seed: builder.state.seed,
        image_url: builder.state.imageUrl,
      }
      if (!isCharacterTier(next.tier)) {
        toast.error(t('This character type cannot be reused'))
        return false
      }
      const conflict = conflictMessage(next.tier, next.selection)
      if (conflict) {
        toast.error(t(conflict))
        return false
      }
      const imageUrl = next.image_url?.trim() || undefined
      const mode = imageUrl ? 'reference' : 'text'
      if (
        mode === 'text' &&
        !isAnimalTier(next.tier) &&
        totalSelected(next.selection) === 0
      ) {
        return false
      }
      if (!imageRoute || !imageGroup || !quote) {
        toast.error(
          imageUrl
            ? t('Style reference is not available for this model')
            : t('No available models')
        )
        return false
      }
      if (!ensureFunded('image', quote.credits)) return false
      submitInFlight.current = true
      const sunburst = imageRoute.model === SUNBURST_MODEL
      const res = await submit({
        tier: next.tier,
        selection: next.selection,
        brief: '',
        size: sunburst ? IMAGE_SIZE : SHEET_SIZE,
        image_url: imageUrl,
        image_mode: sunburst ? mode : undefined,
        model: imageRoute.model,
        group: imageGroup,
      })
      submitInFlight.current = false
      if (res) {
        void queryClient.invalidateQueries({
          queryKey: ['influencer', 'history'],
        })
        // 提交成功即把用户带到刚生成的那张图：与竞品生成后自动落 History 一致。
        setSource('history')
        setMobileCreating(false)
        writeSource('history')
      }
      return Boolean(res)
    },
    [
      taxonomy,
      builder,
      generationBusy,
      imageRoute,
      imageGroup,
      quote,
      submit,
      queryClient,
      ensureFunded,
      t,
    ]
  )

  const { submit: submitMotionTask } = motionSubmit
  const doSubmitMotion = useCallback(async () => {
    if (
      motion.isVideoPreparing() ||
      motion.isCharacterUploading() ||
      builder.isImageUploading() ||
      generationBusy ||
      submitInFlight.current
    ) {
      return
    }
    if (motion.video?.kind === 'preset') {
      toast.error(
        t('Built-in templates are not available yet. Upload your own clip.')
      )
      return
    }
    submitInFlight.current = true
    try {
      // A recreation has one action: the decomposition is an internal step of
      // generation, so it runs here and its compiled prompt is submitted
      // immediately. A kept decomposition (retry after a failed submit) is
      // reused instead of paying for another analysis.
      let recreate: { prompt: string; durationSec: number } | undefined
      let decomposition = motion.recreateResult
      let motionQuote = videoQuote
      if (motion.mode === 'recreate') {
        if (!decomposition) {
          // The pre-analysis quote is the real catalog price, so an empty
          // wallet fails immediately instead of after the long decomposition.
          if (!ensureFunded('video', videoQuote?.credits ?? null)) return
          decomposition = await motion.analyzeRecreate()
          if (!decomposition) return
        }
        recreate = {
          prompt: decomposition.prompt,
          durationSec: Math.round(decomposition.durationSec),
        }
        motionQuote = recreateQuoteFor(decomposition.meta, motion.resolution)
      }
      let body: ReturnType<typeof buildMotionSubmitPayload>
      try {
        body = buildMotionSubmitPayload({
          mode: motion.mode,
          prompt: motion.effectivePrompt,
          character: motion.character,
          characters:
            motion.mode === 'recreate'
              ? motion.recreateImages
              : motion.characters,
          video: motion.video,
          resolution: motion.resolution,
          outputCapabilities: motion.outputCapabilities,
          recreate,
        })
      } catch (error) {
        toast.error(
          submitFailureText(t, error, {
            prefixKey: 'Video generation failed to start: {{msg}}',
            fallbackKey: 'Video generation failed to start. Please try again.',
          })
        )
        return
      }
      if (!body) return
      if (!ensureFunded('video', motionQuote?.credits ?? null)) return
      const res = await submitMotionTask(body)
      if (!res) return
      if (motion.mode === 'recreate' && decomposition) {
      }
      void queryClient.invalidateQueries({
        queryKey: ['influencer', 'history'],
      })
      setSource('history')
      setMobileCreating(false)
      writeSource('history')
    } finally {
      submitInFlight.current = false
    }
  }, [
    builder,
    generationBusy,
    motion,
    submitMotionTask,
    queryClient,
    videoQuote,
    ensureFunded,
    t,
  ])

  // A rejected submission stays on the form; the gateway's own reason is shown
  // so the user knows whether to fix the inputs or simply retry.
  const motionError = motionSubmit.error
  useEffect(() => {
    if (!motionError) return
    if (handleSubmissionFailure(motionError, 'video', null)) return
    toast.error(
      submitFailureText(t, motionError, {
        prefixKey: 'Video generation failed to start: {{msg}}',
        fallbackKey: 'Video generation failed to start. Please try again.',
      })
    )
  }, [motionError, handleSubmissionFailure, t])

  /**
   * The image form used to drop submit failures entirely (no toast, no exit).
   * Every failure now surfaces: insufficient quota opens the paywall, anything
   * else keeps the gateway's own reason.
   */
  const sheetError = sheetSubmit.error
  useEffect(() => {
    if (!sheetError) return
    if (typeof sheetError === 'object') {
      if (handledSubmitErrors.current.has(sheetError)) return
      handledSubmitErrors.current.add(sheetError)
    }
    if (handleSubmissionFailure(sheetError, 'image', quote?.credits ?? null)) {
      return
    }
    toastInfluencerError(t, sheetError, 'Something went wrong!')
  }, [sheetError, handleSubmissionFailure, quote?.credits, t])

  /**
   * The queue can still reject a submission after acceptance (for example a
   * stale displayed balance). Classify that failure against a fresh wallet:
   * an unfunded account gets the paywall, anything else gets the real reason.
   */
  useEffect(() => {
    const failed = [...sheetSubmit.tasks, ...motionSubmit.tasks].filter(
      (task) => {
        const status = (task.status ?? '').toUpperCase()
        if (status !== 'FAILURE' && status !== 'FAILED') return false
        if (!task.task_id || handledFailures.current.has(task.task_id)) {
          return false
        }
        return true
      }
    )
    if (failed.length === 0) return
    for (const task of failed) handledFailures.current.add(task.task_id)
    const failedVideo = failed.some(
      (task) => task.action === INFLUENCER_MOTION_ACTION
    )
    if (failedVideo) {
      void refreshQuota().then((fresh) => {
        if (fresh === null) return
        const required = videoQuote?.credits ?? null
        const insufficient =
          required === null
            ? creditsFromQuota(fresh) <= 0
            : creditsFromQuota(fresh) + 1e-9 < required
        if (insufficient && paywall === null) openPaywall('video', required)
      })
      return
    }
    void refreshQuota().then((fresh) => {
      const required = quote?.credits ?? null
      const insufficient =
        fresh !== null &&
        (required === null
          ? creditsFromQuota(fresh) <= 0
          : creditsFromQuota(fresh) + 1e-9 < required)
      if (insufficient) {
        if (paywall === null) openPaywall('image', required)
        return
      }
      toast.error(describeFailReason(t, failed[0].fail_reason).message)
    })
  }, [
    sheetSubmit.tasks,
    motionSubmit.tasks,
    handleSubmissionFailure,
    refreshQuota,
    openPaywall,
    paywall,
    quote?.credits,
    videoQuote?.credits,
    t,
  ])

  /** 被硬冲突修正掉的选项必须说出来，否则用户只会看到自己没点过的一项消失。 */
  const notifyRemovedPicks = useCallback(
    (removed: RemovedPick[]) => {
      if (!removed.length) return
      toast.warning(
        t(
          '{{options}} removed: not compatible with the rest of this look in this mode.',
          {
            options: removed.map((r) => t(r.label)).join(', '),
          }
        )
      )
    },
    [t]
  )

  /**
   * Applies an inspiration preset to the sidebar. Mirrors the upstream
   * behaviour: the preset is a whole look, so the tier and every pick are
   * replaced together, and no generation is triggered here — the user still
   * reviews the panel and presses Generate.
   */
  const onApplyPreset = useCallback(
    (preset: ExplorePreset) => {
      const applied = taxonomy ? applyPresetSelection(preset, taxonomy) : null
      if (!applied) {
        toast.error(
          t('Preset settings are being updated. Please try again later.')
        )
        return
      }
      const { removed } = builder.applyLook({
        tier: preset.tier,
        values: applied.selection,
        seed: preset.seed,
      })
      if (isGuest) {
        trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PRESET_APPLY_GUEST, {
          preset: preset.id,
        })
      }
      setPreviewPreset(null)
      setMobileCreating(true)
      onWorkbenchModeChange('image')
      toast.success(t('Character settings applied to the sidebar'))
      notifyRemovedPicks(removed)
      builderRef.current?.scrollIntoView({ block: 'start', behavior: 'smooth' })
    },
    [builder, taxonomy, notifyRemovedPicks, isGuest, t, onWorkbenchModeChange]
  )

  /**
   * Applies a motion preset or community trend to the video form: the clip's
   * public URL fills the driving-video slot, the preset's own mode switches
   * the form, and the user still reviews the panel and presses Generate —
   * matching how character presets apply without submitting.
   */
  const onApplyMotion = useCallback(
    async (preset: MotionExplorePreset) => {
      motion.setMode(preset.mode, false)
      const accepted = await motion.setRemoteVideo(
        preset.referenceVideoUrl ?? preset.videoUrl,
        preset.title,
        preset.videoUrl,
        preset.mode
      )
      if (!accepted) return
      motion.setPrompt(preset.description ?? preset.title)
      setPreviewMotion(null)
      setMobileCreating(true)
      if (workbenchMode !== 'video') {
        setWorkbenchMode('video')
        writeWorkbenchMode('video')
      }
      toast.success(t('Motion preset applied to the sidebar'))
      builderRef.current?.scrollIntoView({ block: 'start', behavior: 'smooth' })
    },
    [motion, workbenchMode, t]
  )

  const turnToMotionDisabledReason = (task: TaskRecord): string | undefined => {
    const url = taskAbsoluteImageUrl(task)
    if (!url || !url.startsWith('https://')) return t('No preview available')
    if (
      motion.characters.length >= 30 &&
      !motion.characters.some((item) => item.url === url)
    ) {
      return t('Up to 30 reference images')
    }
    return undefined
  }

  const onTurnToMotion = (task: TaskRecord): void => {
    if (
      taskKind(task) === 'motion' ||
      !isSuccessStatus(task) ||
      turnToMotionDisabledReason(task)
    ) {
      return
    }
    const url = taskAbsoluteImageUrl(task)
    if (!url) return
    void resolveTaskCharacterSource(task, url, t('Character'), historyQ.items)
      .then((source) => {
        motion.addCharacter(source)
        onWorkbenchModeChange('video')
        setMobileCreating(true)
        setViewTask(null)
      })
      .catch((error) => {
        toastInfluencerError(t, error, 'Something went wrong!')
      })
  }

  let exploreContent
  if (isExploreLoading) {
    exploreContent = <ExploreSkeleton />
  } else if (exploreTab === 'influencers') {
    exploreContent = (
      <ExplorePresetGrid
        presets={explorePresets}
        onPreview={setPreviewPreset}
        onApply={onApplyPreset}
      />
    )
  } else {
    exploreContent = (
      <ExploreMotionGrid
        presets={exploreTab === 'presets' ? motionPresets : trendPresets}
        onPreview={setPreviewMotion}
        onApply={onApplyMotion}
      />
    )
  }

  /**
   * Fills the panel from a saved task and returns the sheet that was actually
   * written, so a caller that submits right away submits this task rather than
   * whatever the panel happened to hold before.
   */
  const onRecreate = useCallback(
    (task: TaskRecord): SubmitSheet | null => {
      const meta = parseTaskMeta(task)
      if (!meta) {
        toast.error(t('This task does not carry sheet parameters'))
        return null
      }
      const applied = builder.applyLook({
        tier: meta.tier,
        values: meta.selection,
        seed: meta.seed,
        image_url: meta.image_url,
      })
      toast.success(t('Form filled from task'))
      notifyRemovedPicks(applied.removed)
      return {
        tier: meta.tier,
        selection: applied.selection,
        seed: meta.seed ? String(meta.seed) : '',
        image_url: meta.image_url,
      }
    },
    [builder, notifyRemovedPicks, t]
  )

  const onRegenerate = useCallback(
    (task: TaskRecord) => {
      requireLogin('regenerate', currentDraft(), () => {
        if (builder.isImageUploading() || motion.isCharacterUploading()) return
        if (taskKind(task) === 'motion') {
          const body = parseStoredMotionPayload(task.properties?.input)
          if (!body) return
          void motion.restoreTask(body).then((accepted) => {
            if (!accepted) return
            onWorkbenchModeChange('video')
            toast.success(t('Form filled from task'))
          })
        } else {
          if (!onRecreate(task)) return
          onWorkbenchModeChange('image')
        }
        setViewTask(null)
        setMobileCreating(true)
      })
    },
    [
      builder,
      motion,
      requireLogin,
      currentDraft,
      onRecreate,
      onWorkbenchModeChange,
      setViewTask,
      t,
    ]
  )

  // 基于当前任务主图，生成多视角分镜设定板。
  // 派生任务挂在主图卡片上（左下角 1:1 缩略格），因此这里不新增画廊卡片；
  // 提交成功后由调用方关闭自己的弹窗，用户才能看见主图上的缩略格进度。
  // 返回是否受理成功：失败时弹窗保持打开，用户可原地重试。
  const onGenerateMultiView = useCallback(
    async (task: TaskRecord): Promise<boolean> => {
      if (isGuest) {
        requireLogin('multiview', currentDraft(), () => {})
        return false
      }
      if (
        builder.isImageUploading() ||
        generationBusy ||
        submitInFlight.current
      ) {
        return false
      }
      if (!canDeriveMultiView(task)) return false
      if (!imageRoute || !multiViewGroup || !multiViewQuote) {
        toast.error(t('Style reference is not available for this model'))
        return false
      }
      if (!ensureFunded('multiview', multiViewQuote.credits)) return false
      const parentId = taskKey(task)
      const existingChild =
        multiView.childByParent[parentId] ??
        partitionTasks(historyQ.items).childByParent.get(parentId)
      if (existingChild && !isTerminalStatus(existingChild.status)) return false
      const imageUrl = taskAbsoluteImageUrl(task)
      if (!imageUrl) {
        toast.error(t('No image available for multi-view generation'))
        return false
      }
      toast.info(t('Submitting multi-view storyboard…'))
      submitInFlight.current = true
      try {
        const record = await submitMultiViewTask({
          model: imageRoute.model,
          group: multiViewGroup,
          parentTaskId: parentId,
          imageUrl,
          size: MULTIVIEW_SIZE,
        })
        if (record) {
          void queryClient.invalidateQueries({
            queryKey: ['influencer', 'history'],
          })
        }
        return true
      } catch (e) {
        if (
          handleSubmissionFailure(
            e,
            'multiview',
            multiViewQuote?.credits ?? null
          )
        ) {
          return false
        }
        toast.error(
          submitFailureText(t, e, {
            prefixKey: 'Multi-view generation failed: {{msg}}',
            fallbackKey: 'Multi-view generation failed',
          })
        )
        return false
      } finally {
        submitInFlight.current = false
      }
    },
    [
      submitMultiViewTask,
      multiView.childByParent,
      historyQ.items,
      builder,
      generationBusy,
      imageRoute,
      multiViewGroup,
      multiViewQuote,
      queryClient,
      isGuest,
      requireLogin,
      currentDraft,
      handleSubmissionFailure,
      ensureFunded,
      t,
    ]
  )

  /** Payment confirmed: refresh the wallet, close the takeover and resume. */
  const handleSubscribed = useCallback(async () => {
    if (resumingRef.current) return
    resumingRef.current = true
    const origin = paywall?.origin ?? null
    const parent = multiViewParent
    try {
      await refreshQuota()
    } catch {
      // The component already refreshed; resuming must not be blocked by it.
    }
    setPaywall(null)
    trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.PAYWALL_SUBSCRIBED, {
      origin: origin ?? 'image',
    })
    if (origin === 'multiview' && parent) {
      void onGenerateMultiView(parent)
      return
    }
    if (origin === 'video') {
      void doSubmitMotion()
      return
    }
    void doSubmit()
  }, [
    paywall,
    multiViewParent,
    refreshQuota,
    doSubmit,
    doSubmitMotion,
    onGenerateMultiView,
  ])

  // A `?recreate=` deep link names a history record, so surface the gallery it
  // belongs to instead of leaving the user on the inspiration library.
  useEffect(() => {
    if (!search.recreate) return
    setSource('history')
    setMobileCreating(false)
    writeSource('history')
  }, [search.recreate])

  // Recreate deep-link support: ?recreate=<task-id>
  // The target may sit beyond the pages loaded so far, so a miss reads that one
  // record directly instead of paging until it shows up.
  useEffect(() => {
    if (recreateHandled.current) return
    const target = search.recreate
    if (!target || !taxonomy || !sheetHistoryQ.items.length) return
    const local = sheetHistoryQ.items.find(
      (x) => x.task_id === target || String(x.id) === target
    )
    if (local) {
      recreateHandled.current = true
      onRecreate(local)
      return
    }
    let cancelled = false
    void (async () => {
      try {
        const { items } = await fetchInfluencerTasks({
          task_id: target,
          page_size: 1,
        })
        if (cancelled || recreateHandled.current) return
        const record =
          items.find((x) => x.task_id === target || String(x.id) === target) ??
          items[0]
        if (!record) return
        recreateHandled.current = true
        onRecreate(record)
      } catch {
        // Best-effort: the gallery itself already reports load failures.
      }
    })()
    return () => {
      cancelled = true
    }
  }, [search.recreate, taxonomy, sheetHistoryQ.items, onRecreate])

  const [hiddenIds, setHiddenIds] = useState<Set<string>>(() => hiddenTaskIds())
  const visibleTasks = useMemo(
    () =>
      historyQ.items.filter((x) => !hiddenIds.has(String(x.task_id || x.id))),
    [historyQ.items, hiddenIds]
  )

  const visiblePending = useMemo(
    () => pending.filter((task) => !hiddenIds.has(taskKey(task))),
    [pending, hiddenIds]
  )

  // 画廊只渲染角色主图；多视角任务按父任务编号挂到对应卡片上，
  // 刚从本页提交、尚未出现在历史里的那条以本地状态为准（更新更及时）。
  const partitioned = useMemo(
    () => partitionTasks(visibleTasks),
    [visibleTasks]
  )
  const childByParent = useMemo(() => {
    const merged = new Map(partitioned.childByParent)
    for (const [parent, child] of Object.entries(multiView.childByParent)) {
      const stored = merged.get(parent)
      if (stored && taskKey(stored) === taskKey(child)) {
        if (
          isTerminalStatus(stored.status) &&
          (!isTerminalStatus(child.status) ||
            (!taskImageUrl(child) && taskImageUrl(stored)))
        ) {
          continue
        }
      } else if (stored) {
        const storedIsNewer =
          child.id > 0
            ? stored.id > child.id
            : getTaskTimestamp(stored) > getTaskTimestamp(child)
        if (storedIsNewer) continue
      }
      merged.set(parent, child)
    }
    return merged
  }, [partitioned.childByParent, multiView.childByParent])

  const multiViewChild = multiViewParent
    ? (childByParent.get(taskKey(multiViewParent)) ?? null)
    : null

  // Hiding a parent drops its derived row from view, so an unfinished
  // multi-view would keep its slot while no longer visible anywhere.
  const hasActiveMultiView = useCallback(
    (task: TaskRecord) => {
      const child = childByParent.get(taskKey(task))
      return Boolean(child) && !isTerminalStatus(child?.status)
    },
    [childByParent]
  )

  const onDelete = useCallback(
    (task: TaskRecord) => {
      if (hasActiveMultiView(task)) return
      requireLogin('delete', currentDraft(), () => {
        setHiddenIds(hideTask(String(task.task_id || task.id)))
        setViewTask(null)
        toast.success(t('Removed from gallery'))
      })
    },
    [hasActiveMultiView, requireLogin, currentDraft, setViewTask, t]
  )

  /** Read-only when the record is a derived board or was removed from the gallery. */
  const previewTaskEntry = useCallback(
    (task: TaskRecord): PreviewEntry => ({
      kind: 'task',
      task,
      readOnly:
        taskKind(task) === 'multiview' ||
        hiddenIds.has(String(task.task_id || task.id)),
    }),
    [hiddenIds]
  )
  const onOpenReference = useCallback(
    (url: string, media: PreviewMediaKind) => {
      const request = ++previewRequest.current
      const push = (entry: PreviewEntry) => {
        if (request !== previewRequest.current) return
        // A reference can also open the shared playback window on its own:
        // the workbench slots and the video picker reuse this same entry point.
        setPreviewStack((stack) => (stack.length ? [...stack, entry] : [entry]))
      }
      const found = classifyReference(url, media, historyQ.items, [
        ...uploadLibrary.images,
        ...uploadLibrary.videos,
      ])
      if ('entry' in found) {
        push(
          found.entry.kind === 'task'
            ? previewTaskEntry(found.entry.task)
            : found.entry
        )
        return
      }
      // Not on a loaded history page: read that one owned record.
      void fetchTaskRecord(found.taskId, {
        action: media === 'video' ? INFLUENCER_MOTION_ACTION : undefined,
        with_artifacts: true,
      })
        .then((task) => {
          push(
            task && task.task_id === found.taskId
              ? previewTaskEntry(task)
              : { kind: 'unknown', url, media }
          )
        })
        .catch(() => push({ kind: 'unknown', url, media }))
    },
    [
      historyQ.items,
      uploadLibrary.images,
      uploadLibrary.videos,
      previewTaskEntry,
    ]
  )
  /** Reads a reference into the one shared playback window; no second preview layer. */
  const onPreviewVideo = useCallback(
    (url: string) => onOpenReference(url, 'video'),
    [onOpenReference]
  )
  /** Images share the same preview window; only the classified media differs. */
  const onPreviewImage = useCallback(
    (url: string) => onOpenReference(url, 'image'),
    [onOpenReference]
  )
  const onPreviewBack = useCallback(() => {
    previewRequest.current += 1
    setPreviewStack((stack) => stack.slice(0, -1))
  }, [])
  const closePreview = useCallback(() => {
    previewRequest.current += 1
    setPreviewStack([])
  }, [])
  /** Fills the video form with an existing reference, as Turn to motion does; never submits. */
  const showVideoForm = useCallback(() => {
    onWorkbenchModeChange('video')
    setMobileCreating(true)
    closePreview()
  }, [onWorkbenchModeChange, closePreview])
  const referenceImageDisabledReason = (url: string): string | undefined => {
    if (!url.startsWith('https://')) return t('No preview available')
    if (
      motion.characters.length >= 30 &&
      !motion.characters.some((item) => item.url === url)
    ) {
      return t('Up to 30 reference images')
    }
    return undefined
  }

  const onDeleteMultiView = useCallback(() => {
    if (
      !multiViewParent ||
      (multiViewChild && !isTerminalStatus(multiViewChild.status))
    ) {
      return
    }
    const parentId = taskKey(multiViewParent)
    if (multiViewChild) {
      setHiddenIds(
        hideTask(String(multiViewChild.task_id || multiViewChild.id))
      )
    }
    forgetMultiView(parentId)
    setMultiViewParent(null)
    toast.success(t('Removed from gallery'))
  }, [multiViewParent, multiViewChild, forgetMultiView, t])

  const onRegenerateMultiView = useCallback(() => {
    if (!multiViewParent) return
    void onGenerateMultiView(multiViewParent).then((accepted) => {
      if (accepted) setMultiViewParent(null)
    })
  }, [multiViewParent, onGenerateMultiView])

  // 发布弹窗要同时展示角色主图与该形象的多视角设定板（存在时）。
  const publishChild = publishTask
    ? (childByParent.get(taskKey(publishTask)) ?? null)
    : null

  /**
   * 发布：素材字节由这里统一抓取，服务端只接受图片字节与名称，
   * 档位与选项一律以数据库中的任务记录为准。
   * 任何一步失败都保持弹窗打开，绝不留下“半发布”的状态。
   */
  const onPublishSubmit = useCallback(
    async (input: { name: string }) => {
      const task = publishTask
      if (!task || isGuest) return
      setPublishing(true)
      try {
        const imageUrl = taskImageUrl(task)
        if (!imageUrl) {
          throw new Error(t('The character image is no longer available.'))
        }
        const previewRes = await fetch(imageUrl)
        if (!previewRes.ok) {
          throw new Error(
            t(
              'The character image could not be read. The material may have expired.'
            )
          )
        }
        const preview = await previewRes.blob()

        // 设定板缺失（或已过期）不阻塞发布：服务端会退回用主图充当设定图。
        let sheet: Blob | undefined
        const boardUrl = publishChild ? taskImageUrl(publishChild) : null
        if (boardUrl) {
          const boardRes = await fetch(boardUrl)
          if (boardRes.ok) sheet = await boardRes.blob()
        }

        await publishInfluencerPreset({
          taskId: taskKey(task),
          name: input.name,
          preview,
          sheet,
        })
        toast.success(t('Published to the inspiration library'))
        void queryClient.invalidateQueries({
          queryKey: ['influencer', 'presets'],
        })
        setPublishTask(null)
      } catch (e) {
        toast.error(
          submitFailureText(t, e, {
            prefixKey: 'Failed to publish: {{msg}}',
            fallbackKey: 'Request failed',
          })
        )
        // 让弹窗就地显示同一条原因，用户可直接改名重试。
        throw e
      } finally {
        setPublishing(false)
      }
    },
    [publishTask, publishChild, queryClient, isGuest, t]
  )

  /** 下架：先删行与素材，再让灵感库重新拉取。 */
  const onRemovePreset = useCallback(
    async (preset: ExplorePreset) => {
      try {
        await deletePublishedPreset(preset.id)
        void queryClient.invalidateQueries({
          queryKey: ['influencer', 'presets'],
        })
        setPreviewPreset(null)
        toast.success(t('Removed from the inspiration library'))
      } catch (e) {
        toast.error(
          submitFailureText(t, e, {
            prefixKey: 'Failed to remove: {{msg}}',
            fallbackKey: 'Request failed',
          })
        )
      }
    },
    [queryClient, t]
  )

  const referenceBusy = builder.imageUploading || submissionBusy
  const beforeReferencePick = () => {
    if (referenceBusy || builder.isImageUploading()) return false
    if (!isGuest) return true
    requireLogin('upload', currentDraft(), () => {})
    return false
  }

  // 左栏的两个面各自持有一个底部生成栏，切换 tab 时整块替换。用变量而不是在
  // JSX 里堆叠条件表达式，既保持可读，也避免嵌套三元。
  const imageGenerationUnavailable =
    !isGuest && !pricingQ.isPending && !groupsQ.isPending && !quote
  const imageInputs = (
    <MediaSlot
      kind='image'
      variant='reference'
      label={t('Style reference')}
      hint=''
      accept='image/jpeg,image/png,image/webp'
      previewUrl={builder.state.imageUrl || undefined}
      busy={referenceBusy}
      uploading={builder.imageUploading}
      onPreviewImage={onPreviewImage}
      onBeforePick={beforeReferencePick}
      onPick={(file) => {
        if (!referenceBusy) void builder.pickImage(file)
      }}
      onClear={() => {
        if (!referenceBusy) builder.setImageUrl('')
      }}
    />
  )
  let builderControls = <div className='flex-1' />
  if (taxonomy) {
    builderControls = (
      <BuilderPanel
        taxonomy={taxonomy}
        referenceSlot={imageInputs}
        tier={builder.state.tier}
        selection={builder.state.selection}
        locks={builder.locks}
        openGroups={builder.state.openGroups}
        onTier={(x) => {
          const { dropped, removed } = builder.switchTier(x)
          if (dropped.length) {
            toast.warning(
              t('{{n}} picks not visible in this tier were removed', {
                n: dropped.length,
              })
            )
          }
          notifyRemovedPicks(removed)
        }}
        onPick={(c, o) => {
          const msg = builder.pick(c, o)
          if (msg) toast.warning(t(msg))
        }}
        onToggleGroup={builder.toggleGroup}
      />
    )
  }

  // The dice is a no-op only when the visitor owns every visible category; the
  // disabled state says why instead of leaving a silent click.
  const rolledNothing = taxonomy
    ? rollableCount(taxonomy, builder.state.tier, builder.locks) === 0
    : true

  const builderFooter = (
    <GenerateBar
      summary={
        taxonomy ? (
          <CharacterSettingsSummary
            taxonomy={taxonomy}
            tier={builder.state.tier}
            selection={builder.state.selection}
            disabled={referenceBusy}
            onReset={() => {
              if (!referenceBusy) builder.resetFeatures()
            }}
          />
        ) : null
      }
      quote={isGuest ? null : quote}
      canGenerate={canGenerate && !referenceBusy && !generationBusy}
      submitting={submitting}
      isGuest={isGuest}
      ctaLabel={
        imageGenerationUnavailable ? t('Temporarily unavailable') : undefined
      }
      ctaTitle={
        imageGenerationUnavailable
          ? t('Image generation is temporarily unavailable')
          : undefined
      }
      onGenerate={() => {
        requireLogin('generate', currentDraft(), () => {
          void doSubmit()
        })
      }}
      onRandomize={() => builder.randomize()}
      randomizeDisabled={rolledNothing}
      randomizeHint={t('All traits are locked')}
    />
  )

  const videoFooter = (
    <GenerateBar
      quote={isGuest ? null : videoQuote}
      canGenerate={
        (isGuest || motion.canSubmit) &&
        !referenceBusy &&
        !motion.videoPreparing
      }
      submitting={motionSubmit.submitting || motion.recreateAnalyzing}
      isGuest={isGuest}
      ctaLabel='Generate video'
      busyLabel={
        motion.mode === 'recreate' && motion.recreateAnalyzing
          ? 'Analyzing reference video'
          : undefined
      }
      onGenerate={() => {
        requireLogin('generate', currentDraft(), () => {
          void doSubmitMotion()
        })
      }}
    />
  )

  // 右栏内容：探索区是 hero + 三个 tab 的灵感货架；历史区签名前多一层游客态。
  // 抽成变量避免在 JSX 里堆嵌套三元。
  let rightContent
  if (source === 'explore') {
    rightContent = (
      <>
        <ExploreHero />
        <div className='flex justify-center pt-2'>
          <ExploreTabs value={exploreTab} onChange={onExploreTabChange} />
        </div>
        {exploreContent}
      </>
    )
  } else if (isGuest) {
    // Guests get a conversion touchpoint instead of the signed-in gallery —
    // nothing to fetch and nothing to pretend exists.
    rightContent = (
      <EmptyState
        title={t('No characters yet')}
        description={t('Sign in to see the characters you generate here.')}
      />
    )
  } else {
    rightContent = (
      <FeedGrid
        tasks={partitioned.roots}
        pending={visiblePending}
        childByParent={childByParent}
        isInitialLoading={historyQ.isInitialLoading}
        hasMore={historyQ.hasMore}
        isFetchingMore={historyQ.isFetchingMore}
        loadMoreFailed={historyQ.loadMoreFailed}
        onLoadMore={historyQ.loadMore}
        onView={(task) => {
          selectedAccount.current = accountId
          setViewTask(task)
        }}
        onReloadPreview={() => {
          void historyQ.retry?.()
        }}
        onTurnToMotion={onTurnToMotion}
        turnToMotionDisabledReason={turnToMotionDisabledReason}
        onOpenMultiView={(parent) => setMultiViewParent(parent)}
      />
    )
  }

  return (
    <div className='influencer-page flex h-full min-h-0 flex-col bg-[var(--bg-app)] text-[var(--inf-txt)]'>
      {/* 卡片式双容器：左侧选项面板 + 右侧画廊，各自独立圆角浮层，用间隙分隔而非分割线。
          左栏 320px 与竞品实测一致（内容区 304 = 320 − 8×2）。 */}
      <div
        className={cn(
          'relative grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,1fr)] gap-2 p-2 md:grid-cols-[320px_1fr] md:p-3',
          source === 'history' && 'inf-history-workspace'
        )}
      >
        {/* 左侧侧边栏：顶部 tab 切换工作台的两个面，下方是该面的表单与生成栏 */}
        <aside
          ref={builderRef}
          className={cn(
            'inf-builder min-h-0 flex-col overflow-hidden rounded-[var(--inf-radius-panel)] bg-[var(--inf-surface)] md:flex',
            mobileCreating ? 'flex' : 'hidden'
          )}
          aria-label={t('Create')}
        >
          <div className='inf-builder-scroll'>
            <header className='inf-builder-heading'>
              <p>{t('Create your own character')}</p>
              <h1>{t('Character studio')}</h1>
            </header>
            <WorkbenchModeTabs
              value={workbenchMode}
              onChange={onWorkbenchModeChange}
            />
            {workbenchMode !== 'video' ? builderControls : null}
            <div hidden={workbenchMode !== 'video'}>
              <MotionPanel
                assetsRef={assetsRef}
                motion={motion}
                submitting={motionSubmit.submitting}
                onPreviewVideo={onPreviewVideo}
                onPreviewImage={onPreviewImage}
                onBeforeVideoPick={() => {
                  if (!isGuest) return true
                  requireLogin('generate', currentDraft(), () => {})
                  return false
                }}
              />
            </div>
          </div>
          {workbenchMode !== 'video' ? builderFooter : videoFooter}
        </aside>

        {/* 右侧卡片：顶部固定模型配置作为卡片页眉，下方画廊独立滚动 */}
        <section
          aria-label={t(source === 'history' ? 'History' : 'Explore')}
          className={cn(
            'inf-browser min-h-0 flex-col overflow-hidden rounded-[var(--inf-radius-panel)] border border-[var(--inf-line)] bg-[var(--inf-surface)] md:flex',
            mobileCreating ? 'hidden' : 'flex'
          )}
        >
          {/* 模型配置：卡片页眉，始终固定在容器顶部，收成一颗按钮，点击才展开配置面板 */}
          <div className='flex flex-wrap items-center justify-between gap-2 px-4 py-3'>
            <div className='hidden min-w-0 flex-wrap items-center gap-2 md:flex'>
              {/* 灵感库 / 历史记录：默认落在灵感库，首次访问即可一键套用现成形象 */}
              <SourceTabs value={source} onChange={onSourceChange} />
            </div>

            <h2 className='text-base font-semibold md:hidden'>
              {t(source === 'history' ? 'History' : 'Explore')}
            </h2>
          </div>

          {/* 滚动区：仅此区域滚动，页眉保持不动 */}
          <div
            className={cn(
              'min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 pb-4',
              source === 'history' && 'inf-history-viewport'
            )}
          >
            {rightContent}
          </div>
        </section>

        {/* 角色素材选择面板（位于上传器外侧靠右） */}
        {workbenchMode === 'video' && (
          <CharacterPickerPanel
            anchorRef={assetsRef}
            open={motion.characterPickerOpen}
            onClose={motion.closeCharacterPicker}
            characters={motion.characters}
            max={motion.mode === 'recreate' ? RECREATE_MAX_IMAGES : undefined}
            uploadedCharacters={motion.uploadedCharacters}
            uploads={motion.characterUploads}
            historyItems={historyQ.items}
            busy={submissionBusy}
            hasMore={historyQ.hasMore}
            isFetchingMore={historyQ.isFetchingMore}
            loadMoreFailed={historyQ.loadMoreFailed}
            onLoadMore={historyQ.loadMore}
            onUploadFiles={(files) => void motion.pickCharacters(files)}
            onRetryUpload={(id) => void motion.retryCharacterUpload(id)}
            onDismissUpload={motion.dismissCharacterUpload}
            onToggleUrl={motion.toggleCharacter}
            onPreviewImage={onPreviewImage}
          />
        )}
        {workbenchMode === 'video' && (
          <VideoPickerPanel
            anchorRef={assetsRef}
            open={motion.videoPickerOpen}
            onClose={motion.closeVideoPicker}
            historyItems={historyQ.items.filter(
              (task) =>
                !hiddenIds.has(task.task_id) &&
                referenceVideoCandidate(task) !== null
            )}
            selectedTaskId={
              motion.video?.kind === 'remote' ? motion.video.taskId : undefined
            }
            selectedVideoUrl={
              motion.video?.kind === 'remote'
                ? motion.video.previewUrl
                : undefined
            }
            selectedArtifactKey={
              motion.video?.kind === 'remote'
                ? motion.video.artifactKey
                : undefined
            }
            uploadedVideos={motion.uploadedVideos}
            selectedUploadUrl={
              motion.video?.kind === 'upload' ? motion.video.url : undefined
            }
            onSelectUpload={(source) => {
              if (
                motion.video?.kind === 'upload' &&
                motion.video.url === source.url
              ) {
                motion.clearVideo()
              } else {
                motion.selectUploadedVideo(source)
              }
            }}
            preparationPhase={motion.videoPreparationPhase}
            uploadProgress={motion.videoUploadProgress}
            preparingTaskId={motion.preparingTaskId}
            busy={motion.videoPreparing || motionSubmit.submitting}
            isInitialLoading={historyQ.isInitialLoading}
            error={historyQ.error}
            hasMore={historyQ.hasMore}
            isFetchingMore={historyQ.isFetchingMore}
            loadMoreFailed={historyQ.loadMoreFailed}
            onLoadMore={historyQ.loadMore}
            onRetry={() => void historyQ.retry?.()}
            onPreviewVideo={onPreviewVideo}
            onUploadFile={async (file) => {
              await motion.pickVideo(file)
            }}
            onSelectTask={(task) => {
              if (
                motion.video?.kind === 'remote' &&
                motion.video.taskId === task.task_id &&
                motion.video.artifactKey ===
                  referenceVideoCandidate(task)?.artifactKey
              ) {
                motion.clearVideo()
              } else {
                void motion.pickHistoryVideo(task)
              }
            }}
          />
        )}
      </div>

      <nav
        aria-label={t('Workspace')}
        className='inf-mobile-nav grid shrink-0 grid-cols-3 gap-2 border-t border-[var(--inf-line)] bg-[var(--inf-surface)] px-3 pt-2 md:hidden'
      >
        {[
          {
            id: 'create',
            label: t('Create'),
            icon: Plus,
            active: mobileCreating,
          },
          {
            id: 'explore',
            label: t('Explore'),
            icon: Compass,
            active: !mobileCreating && source === 'explore',
          },
          {
            id: 'history',
            label: t('History'),
            icon: History,
            active: !mobileCreating && source === 'history',
          },
        ].map((item) => (
          <Button
            key={item.id}
            variant='ghost'
            aria-pressed={item.active}
            className={cn(
              'h-12 flex-col gap-1 rounded-lg text-xs',
              item.active
                ? 'bg-[var(--inf-surface-2)] text-[var(--inf-txt)]'
                : 'text-[var(--inf-txt-3)]'
            )}
            onClick={() => {
              if (item.id !== 'create') {
                motion.closeVideoPicker()
                motion.closeCharacterPicker()
              }
              setMobileCreating(item.id === 'create')
              if (item.id === 'explore' || item.id === 'history') {
                onSourceChange(item.id)
              }
            }}
          >
            <item.icon className='size-4' aria-hidden='true' />
            {item.label}
          </Button>
        ))}
      </nav>

      {/* 灵感预设预览：大图 + 参数摘要 + 一键套用到左侧面板 */}
      <PresetPreviewDialog
        preset={previewPreset}
        taxonomy={taxonomy}
        onClose={() => setPreviewPreset(null)}
        onApply={onApplyPreset}
        isAdmin={isAdmin}
        onRemove={(preset) => {
          void onRemovePreset(preset)
        }}
      />

      {/* 动作预设预览：视频自动播放 + 一键套用到视频表单 */}
      <MotionPresetDialog
        preset={previewMotion}
        onClose={() => setPreviewMotion(null)}
        onApply={onApplyMotion}
      />

      {viewStack.length > 0 && (
        <HistoryPreviewDialog
          stack={viewStack}
          taxonomy={taxonomy}
          onClose={closePreview}
          onBack={onPreviewBack}
          onOpenReference={onOpenReference}
          taskActions={(viewTask) => ({
            onReloadPreview: () => {
              void historyQ.retry?.()
            },
            onRegenerate,
            onTurnToMotion,
            turnToMotionDisabledReason: turnToMotionDisabledReason(viewTask),
            onDelete,
            deleteDisabledReason: hasActiveMultiView(viewTask)
              ? t('Remove after the multi-view generation finishes')
              : undefined,
            multiViewDisabledReason: hasActiveMultiView(viewTask)
              ? t(
                  'A multi-view generation is already queued for this character'
                )
              : multiViewDisabledReason,
            onGenerateMultiView: (task) => {
              void onGenerateMultiView(task).then((accepted) => {
                if (accepted) setViewTask(null)
              })
            },
            onPublish:
              isAdmin &&
              taskKind(viewTask) === 'sheet' &&
              isSuccessStatus(viewTask)
                ? setPublishTask
                : undefined,
          })}
          referenceActions={{
            onUseImage: (url, fileName) => {
              if (referenceImageDisabledReason(url)) return
              motion.addCharacter({ url, fileName })
              showVideoForm()
            },
            useImageDisabledReason: referenceImageDisabledReason,
            onUseVideo: (entry) => {
              if (motion.videoPreparing || motionSubmit.submitting) return
              motion.selectUploadedVideo({
                kind: 'upload',
                url: entry.record.url,
                fileName: entry.record.fileName,
                metadata: entry.record.metadata,
              })
              showVideoForm()
            },
            useVideoDisabledReason:
              motion.videoPreparing || motionSubmit.submitting
                ? t('Wait for the current upload or submission to finish')
                : undefined,
          }}
        />
      )}

      {/* 发布到灵感库：把角色主图（与多视角设定板）连同任务参数一并入库 */}
      <PublishPresetDialog
        open={publishTask !== null}
        onOpenChange={(open) => {
          if (!open) setPublishTask(null)
        }}
        task={publishTask}
        child={publishChild}
        onSubmit={onPublishSubmit}
        submitting={publishing}
      />
      {/* 多视角放大浏览：下载 / 删除 / 重新生成 */}
      <MultiViewDialog
        open={multiViewParent !== null}
        onOpenChange={(open) => {
          if (!open) setMultiViewParent(null)
        }}
        task={multiViewChild}
        regenerating={
          generationBusy ||
          Boolean(multiViewChild && !isTerminalStatus(multiViewChild.status))
        }
        disabledReason={multiViewDisabledReason}
        onRegenerate={onRegenerateMultiView}
        onDelete={onDeleteMultiView}
      />

      {/* 余额不足：全屏订阅接管。草稿留在内存，支付成功后自动继续这次生成 */}
      <SubscribePaywall
        open={paywall !== null}
        requiredCredits={paywall?.required ?? null}
        origin={paywall?.origin ?? 'image'}
        modelName={
          paywall && paywall.origin !== 'video'
            ? (imageRoute?.model ?? null)
            : null
        }
        onClose={() => setPaywall(null)}
        onSubscribed={handleSubscribed}
      />

      {/* 游客写操作统一收口：快照已选参数后引导登录，回跳恢复原样 */}
      <LoginGateDialog
        open={loginGate.gateOpen}
        onOpenChange={loginGate.onGateOpenChange}
        hasDraft={
          totalSelected(builder.state.selection) > 0 || !!builder.state.imageUrl
        }
      />
    </div>
  )
}
