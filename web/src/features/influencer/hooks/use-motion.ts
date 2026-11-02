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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'
import { useMotionPreferencesStore } from '@/stores/motion-preferences-store'

import {
  fetchRecreateAnalysis,
  startRecreateAnalysis,
  type MotionSubmitBody,
} from '../api'
import { toastInfluencerError } from '../lib/error-copy'
import {
  canSubmitMotion,
  validateBrowserMotionMetadata,
  validateMotionReference,
  motionVideoFailureMessage,
  motionModeMeta,
} from '../lib/motion'
import { readMotionMode, writeMotionMode } from '../lib/motion-mode'
import { combineStylePrompt } from '../lib/motion-style'
import { recreateQuoteFor } from '../lib/recreate'
import { pollRecreateAnalysis } from '../lib/recreate-poll'
import type {
  MotionAspectRatio,
  MotionCharacterSource,
  MotionMode,
  MotionStyleChoice,
  MotionVideoSource,
  RecreateResult,
  TaskRecord,
} from '../types'
import { useCharacterUploads } from './use-character-uploads'
import { useUploadLibrary } from './use-upload-library'
import { useVideoReferenceSelection } from './use-video-reference-selection'

/** Free-tier callers are limited to the lower rung; the switch still shows both. */
export const MOTION_RESOLUTIONS = ['480p', '720p']

/** The planner accepts at most this many reference images, in upload order. */
export const RECREATE_MAX_IMAGES = 30

/** Mirrors the public prompt ceiling enforced by the gateway. */
export const RECREATE_PROMPT_MAX_RUNES = 4000

/** The planner treats the caller's intent as context, not a provider parameter. */
export const RECREATE_INTENT_MAX_RUNES = 1200

export { DEFAULT_MOTION_RESOLUTION } from '@/stores/motion-preferences-store'

/** Each mode keeps its own video, reference images and prompt draft; the
 * upload/history libraries stay shared and only the current selection is local. */
interface ModeAssetSnapshot {
  video: MotionVideoSource | null
  characters: MotionCharacterSource[]
  prompt: string
  promptEnabled: boolean
}

function emptyModeAssets(): Record<MotionMode, ModeAssetSnapshot> {
  return {
    motion: { video: null, characters: [], prompt: '', promptEnabled: false },
    replace: { video: null, characters: [], prompt: '', promptEnabled: false },
    recreate: { video: null, characters: [], prompt: '', promptEnabled: false },
  }
}

/**
 * The motion form's state.
 *
 * Uploads go through the page's upload library so the form holds a public URL
 * rather than a local file, and every finished upload stays listed for the
 * account after a refresh. Nothing here submits a generation — the second phase
 * wires that up.
 */
export function useMotion() {
  const { t } = useTranslation()
  const [mode, setModeState] = useState<MotionMode>(() => readMotionMode())
  const currentMode = useRef(mode)
  currentMode.current = mode
  const [characters, setCharacters] = useState<MotionCharacterSource[]>([])
  const charactersRef = useRef<MotionCharacterSource[]>([])
  useEffect(() => {
    charactersRef.current = characters
  }, [characters])
  const character = characters[0] ?? null
  const [characterPickerOpen, setCharacterPickerOpen] = useState(false)
  const library = useUploadLibrary()
  const uploadFile = library.upload
  const prepareVideoUpload = library.uploadVideo
  const uploadedCharacters = useMemo<MotionCharacterSource[]>(
    () =>
      library.images.map((record) => ({
        url: record.url,
        fileName: record.fileName,
      })),
    [library.images]
  )
  const uploadedVideos = useMemo<
    Extract<MotionVideoSource, { kind: 'upload' }>[]
  >(
    () =>
      library.videos.map((record) => ({
        kind: 'upload',
        url: record.url,
        fileName: record.fileName,
        metadata: record.metadata,
      })),
    [library.videos]
  )
  const [videoUploadProgress, setVideoUploadProgress] = useState<
    number | undefined
  >()
  const [video, setVideo] = useState<MotionVideoSource | null>(null)
  const modeWarning = useRef<MotionVideoSource | null>(null)
  const [videoPickerOpen, setVideoPickerOpen] = useState(false)
  const [prompt, setPrompt] = useState('')
  const [recreateResult, setRecreateResult] = useState<RecreateResult | null>(
    null
  )
  const [recreatePrompt, setRecreatePromptState] = useState('')
  // The decomposition intent keeps its own draft and switch so switching modes
  // never leaks a generation prompt into the recreation form.
  const [recreateIntentPrompt, setRecreateIntentPromptState] = useState('')
  const [recreateIntentPromptEnabled, setRecreateIntentPromptEnabledState] =
    useState(false)
  const [recreateAnalyzing, setRecreateAnalyzing] = useState(false)
  const recreateAnalyzingRef = useRef(false)
  const recreatePollRef = useRef<{
    token: number
    abort: AbortController
  } | null>(null)
  const recreateTokenRef = useRef(0)
  const [styles, setStyles] = useState<
    Record<MotionMode, MotionStyleChoice | null>
  >({
    motion: null,
    replace: null,
    recreate: null,
  })
  /** Any changed input invalidates the accepted decomposition; the clip, the
   * image set and the intent all feed the compiled prompt. */
  const dropRecreateResult = useCallback(() => {
    setRecreateResult(null)
    setRecreatePromptState('')
  }, [])
  /** Stops the poll in flight and makes its late responses inert. */
  const cancelRecreateAnalysis = useCallback(() => {
    recreateTokenRef.current += 1
    recreatePollRef.current?.abort.abort()
    recreatePollRef.current = null
    recreateAnalyzingRef.current = false
    setRecreateAnalyzing(false)
  }, [])
  const style = styles[mode]
  const [promptEnabled, setPromptEnabled] = useState(false)
  const modeAssets = useRef<Record<MotionMode, ModeAssetSnapshot>>(
    emptyModeAssets()
  )
  /** The mode an in-flight picker/upload will write to, not whichever is live. */
  const videoTargetMode = useRef<MotionMode>(mode)
  const characterTargetMode = useRef<MotionMode>(mode)
  const resolution = useMotionPreferencesStore((state) => state.resolution)
  const setStoredResolution = useMotionPreferencesStore(
    (state) => state.setResolution
  )
  const [outputCapabilities, setOutputCapabilities] = useState<
    Record<string, MotionAspectRatio[]> | undefined
  >()
  const rememberVideoMetadata = library.rememberVideoMetadata
  const onCommitVideo = useCallback(
    (source: MotionVideoSource) => {
      modeWarning.current = null
      const target = videoTargetMode.current
      if (target === currentMode.current) {
        if (target === 'recreate') dropRecreateResult()
        setVideo(source)
      } else {
        modeAssets.current[target] = {
          ...modeAssets.current[target],
          video: source,
        }
      }
      if (source.kind === 'upload' && source.metadata) {
        rememberVideoMetadata(source.url, source.metadata)
      }
    },
    [dropRecreateResult, rememberVideoMetadata]
  )
  const uploadVideo = useCallback(
    (
      file: File,
      metadata: import('../types').MotionVideoMetadata,
      signal?: AbortSignal
    ) => {
      setVideoUploadProgress(0)
      return prepareVideoUpload(file, metadata, signal, (loaded, total) => {
        setVideoUploadProgress(total > 0 ? (loaded / total) * 100 : 0)
      }).finally(() => setVideoUploadProgress(undefined))
    },
    [prepareVideoUpload]
  )
  const videoSelection = useVideoReferenceSelection({
    mode,
    resolution,
    outputAspects: outputCapabilities,
    onCommit: onCommitVideo,
    upload: uploadVideo,
  })
  const cancelVideoSelection = videoSelection.cancel
  const pickRemoteSelection = videoSelection.pickRemote
  const pickRemoteVideo = useCallback(
    (...args: Parameters<typeof pickRemoteSelection>) => {
      videoTargetMode.current = args[1] ?? currentMode.current
      return pickRemoteSelection(...args)
    },
    [pickRemoteSelection]
  )
  const videoMetadata = video?.kind === 'preset' ? undefined : video?.metadata
  const videoValidation = videoMetadata
    ? { ok: true as const, ...videoMetadata }
    : null
  const videoMetadataLoading = videoSelection.busy

  useEffect(
    () => () => {
      // Never leave a poll or its fetch running after the form unmounts.
      recreateTokenRef.current += 1
      recreatePollRef.current?.abort.abort()
      recreatePollRef.current = null
    },
    []
  )

  const durationSec = videoMetadata?.durationSec ?? 0
  const outputAspect = videoMetadata?.aspectRatio ?? null
  const setResolution = useCallback(
    (next: string) => {
      if (videoMetadata) {
        const checked = validateMotionReference(
          videoMetadata,
          mode,
          next,
          outputCapabilities
        )
        if (!checked.ok) {
          handleServerError(
            new Error(t(motionVideoFailureMessage(checked.reason, mode)))
          )
          return
        }
      }
      if (next !== resolution) cancelVideoSelection()
      setStoredResolution(next)
    },
    [
      videoMetadata,
      mode,
      resolution,
      outputCapabilities,
      cancelVideoSelection,
      setStoredResolution,
      t,
    ]
  )

  useEffect(() => {
    if (!videoMetadata || modeWarning.current === video) return
    const checked = validateMotionReference(
      videoMetadata,
      mode,
      resolution,
      outputCapabilities
    )
    if (!checked.ok) {
      modeWarning.current = video
      handleServerError(
        new Error(t(motionVideoFailureMessage(checked.reason, mode)))
      )
    }
  }, [video, videoMetadata, mode, resolution, outputCapabilities, t])

  const openCharacterPicker = useCallback(() => {
    setVideoPickerOpen(false)
    setCharacterPickerOpen(true)
  }, [])
  const openVideoPicker = useCallback(() => {
    setCharacterPickerOpen(false)
    setVideoPickerOpen(true)
  }, [])
  // Panel visibility does not own the preparation task or its result.
  const closeVideoPicker = useCallback(() => {
    setVideoPickerOpen(false)
  }, [])
  const closeCharacterPicker = useCallback(
    () => setCharacterPickerOpen(false),
    []
  )

  const setMode = useCallback(
    (next: MotionMode, _notifyIncompatibleReference = true) => {
      if (next === currentMode.current) return
      cancelRecreateAnalysis()
      cancelVideoSelection()
      setVideoPickerOpen(false)
      setCharacterPickerOpen(false)
      // Stash the mode being left, then restore the target's own selection.
      modeAssets.current[currentMode.current] = {
        video,
        characters,
        prompt,
        promptEnabled,
      }
      currentMode.current = next
      setModeState(next)
      writeMotionMode(next)
      const snapshot = modeAssets.current[next]
      setVideo(snapshot.video)
      setCharacters(snapshot.characters)
      setPrompt(snapshot.prompt)
      setPromptEnabled(snapshot.promptEnabled)
    },
    [
      cancelVideoSelection,
      cancelRecreateAnalysis,
      video,
      characters,
      prompt,
      promptEnabled,
    ]
  )

  /** All three modes now share the 30-reference ceiling. */
  const imageLimit = useCallback(
    () => (currentMode.current === 'recreate' ? RECREATE_MAX_IMAGES : 30),
    []
  )
  const invalidateRecreateIfActive = useCallback(() => {
    if (currentMode.current === 'recreate') dropRecreateResult()
  }, [dropRecreateResult])

  const addCharacter = useCallback(
    (source: MotionCharacterSource) => {
      const limit = imageLimit()
      setCharacters((prev) => {
        if (prev.length >= limit) return prev
        if (prev.some((c) => c.url === source.url)) return prev
        return [...prev, source]
      })
      invalidateRecreateIfActive()
    },
    [imageLimit, invalidateRecreateIfActive]
  )

  /**
   * Multi-select uploads run through the shared batch queue: three at a time,
   * capped by the batch limit and the current mode's reference ceiling. Each
   * finished file appends to the ordered reference list exactly like before.
   */
  const addUploadedCharacter = useCallback(
    (source: MotionCharacterSource) => {
      const target = characterTargetMode.current
      const limit = target === 'recreate' ? RECREATE_MAX_IMAGES : 30
      const append = (prev: MotionCharacterSource[]) => {
        if (prev.length >= limit) return prev
        if (prev.some((c) => c.url === source.url)) return prev
        return [...prev, source]
      }
      if (target === currentMode.current) {
        setCharacters(append)
        if (target === 'recreate') dropRecreateResult()
        return
      }
      modeAssets.current[target] = {
        ...modeAssets.current[target],
        characters: append(modeAssets.current[target].characters),
      }
    },
    [dropRecreateResult]
  )
  const characterUploader = useCharacterUploads({
    uploadImage: (file, onProgress, signal) =>
      uploadFile(file, 'image', undefined, onProgress, signal),
    addCharacter: addUploadedCharacter,
    imageLimit,
    selectedCount: () => charactersRef.current.length,
    onLimit: (allowed) =>
      toast.warning(
        t('Select up to {{count}} images at once', { count: allowed })
      ),
  })
  const cancelCharacterUploads = characterUploader.cancel
  const pickCharacterFilesImpl = characterUploader.pickFiles
  const retryCharacterFileImpl = characterUploader.retry
  const pickCharacterFiles = useCallback(
    (...args: Parameters<typeof pickCharacterFilesImpl>) => {
      characterTargetMode.current = currentMode.current
      return pickCharacterFilesImpl(...args)
    },
    [pickCharacterFilesImpl]
  )
  const retryCharacterFile = useCallback(
    (...args: Parameters<typeof retryCharacterFileImpl>) => {
      characterTargetMode.current = currentMode.current
      return retryCharacterFileImpl(...args)
    },
    [retryCharacterFileImpl]
  )
  const characterUploading = characterUploader.uploading
  const characterUploadProgress = characterUploader.progress

  /**
   * Single-file entry for older callers: it resolves once that one file
   * settles and still throws on failure, so the caller keeps owning the
   * user-facing message.
   */
  const pickCharacter = useCallback(
    async (file: File) => {
      const result = await pickCharacterFiles([file])
      if (result.accepted > 0 && result.failed > 0) {
        throw result.error ?? new Error(t('Upload failed. Please try again.'))
      }
    },
    [pickCharacterFiles, t]
  )

  /**
   * Pick entry for the picker and the character slot. Over-limit picks are
   * refused inside the queue (with the limit toast); transport failures keep
   * the server reason.
   */
  const pickCharacters = useCallback(
    async (files: File[]) => {
      const result = await pickCharacterFiles(files)
      if (result.failed > 0) {
        toastInfluencerError(
          t,
          result.error,
          'Upload failed. Please try again.'
        )
      }
      return result
    },
    [pickCharacterFiles, t]
  )

  /** Re-queues one failed file; the same ceiling and error copy apply. */
  const retryCharacterUpload = useCallback(
    async (id: string) => {
      const result = await retryCharacterFile(id)
      if (result.failed > 0) {
        toastInfluencerError(
          t,
          result.error,
          'Upload failed. Please try again.'
        )
      }
      return result
    },
    [retryCharacterFile, t]
  )

  const removeCharacter = useCallback(
    (index: number) => {
      setCharacters((prev) => prev.filter((_, i) => i !== index))
      invalidateRecreateIfActive()
    },
    [invalidateRecreateIfActive]
  )

  const setAllCharacters = useCallback(
    (list: MotionCharacterSource[]) => {
      setCharacters(list.slice(0, imageLimit()))
      invalidateRecreateIfActive()
    },
    [imageLimit, invalidateRecreateIfActive]
  )

  const toggleCharacter = useCallback(
    (source: MotionCharacterSource) => {
      const limit = imageLimit()
      setCharacters((prev) => {
        const exists = prev.some((c) => c.url === source.url)
        if (exists) {
          return prev.filter((c) => c.url !== source.url)
        }
        if (prev.length >= limit) return prev
        return [...prev, source]
      })
      invalidateRecreateIfActive()
    },
    [imageLimit, invalidateRecreateIfActive]
  )

  const pickVideoSelection = videoSelection.pickVideo
  const pickVideo = useCallback(
    (...args: Parameters<typeof pickVideoSelection>) => {
      videoTargetMode.current = currentMode.current
      return pickVideoSelection(...args)
    },
    [pickVideoSelection]
  )

  /** Visual styles are text; replacement templates must pass the same input check. */
  const applyStyle = useCallback(
    (preset: MotionStyleChoice | null) => {
      if (preset && preset.mode !== mode) return
      if (!preset || preset.mode === 'motion') {
        setStyles((current) => ({ ...current, [mode]: preset }))
        return
      }
      if (!preset.videoUrl) {
        setStyles((current) => ({ ...current, replace: preset }))
        return
      }
      cancelVideoSelection()
      return pickRemoteVideo(
        {
          kind: 'remote',
          url: preset.referenceVideoUrl ?? preset.videoUrl,
          previewUrl: preset.videoUrl,
          name: preset.nameKey,
        },
        preset.mode
      ).then((accepted) => {
        if (accepted) setStyles((current) => ({ ...current, replace: preset }))
      })
    },
    [mode, cancelVideoSelection, pickRemoteVideo]
  )

  /** Commit a preset only after its full-quality original passes the browser checks. */
  const setRemoteVideo = useCallback(
    async (
      url: string,
      name: string,
      previewUrl?: string,
      modeOverride?: MotionMode
    ) => {
      cancelVideoSelection()
      return pickRemoteVideo(
        { kind: 'remote', url, name, previewUrl },
        modeOverride
      )
    },
    [cancelVideoSelection, pickRemoteVideo]
  )

  const clearCharacter = useCallback(() => {
    setCharacters([])
    dropRecreateResult()
  }, [dropRecreateResult])

  /** A visual style survives removal of its motion reference. */
  const clearVideo = useCallback(() => {
    cancelVideoSelection()
    cancelRecreateAnalysis()
    if (mode === 'replace') {
      setStyles((current) => ({ ...current, replace: null }))
    }
    dropRecreateResult()
    setVideo(null)
  }, [mode, cancelVideoSelection, cancelRecreateAnalysis, dropRecreateResult])

  const reset = useCallback(() => {
    cancelVideoSelection()
    cancelRecreateAnalysis()
    modeAssets.current = emptyModeAssets()
    setVideoPickerOpen(false)
    setCharacterPickerOpen(false)
    setCharacters([])
    cancelCharacterUploads()
    setVideo(null)
    setStyles({ motion: null, replace: null, recreate: null })
    setPrompt('')
    setPromptEnabled(false)
    setRecreateResult(null)
    setRecreatePromptState('')
    setRecreateIntentPromptState('')
    setRecreateIntentPromptEnabledState(false)
    recreateAnalyzingRef.current = false
    setRecreateAnalyzing(false)
  }, [cancelVideoSelection, cancelRecreateAnalysis, cancelCharacterUploads])

  /** Restore a historical form atomically without uploading or submitting. */
  const restoreTask = useCallback(
    async (body: MotionSubmitBody): Promise<boolean> => {
      cancelVideoSelection()
      cancelRecreateAnalysis()
      modeAssets.current[currentMode.current] = {
        video,
        characters,
        prompt,
        promptEnabled,
      }
      // A recreation row has no clip in its input: it cannot rebuild the
      // form, so it stays a history-only record.
      if (!body.video_url) return false
      const accepted = await pickRemoteVideo(
        {
          kind: 'remote',
          url: body.video_url,
          name: 'video',
          metadata: body.video_metadata
            ? (() => {
                const checked = validateBrowserMotionMetadata(
                  body.video_metadata,
                  body.mode
                )
                return checked.ok ? checked : undefined
              })()
            : undefined,
        },
        body.mode,
        body.resolution
      )
      if (!accepted) return false
      cancelCharacterUploads()
      setCharacterPickerOpen(false)
      setVideoPickerOpen(false)
      if (currentMode.current !== body.mode) {
        currentMode.current = body.mode
        setModeState(body.mode)
        writeMotionMode(body.mode)
        // The committed clip was routed to this mode's snapshot by onCommitVideo.
        setVideo(modeAssets.current[body.mode].video)
      }
      setStyles({ motion: null, replace: null, recreate: null })
      const urls = body.subject_image_urls?.length
        ? body.subject_image_urls
        : [body.subject_image_url]
      const restoredCharacters = urls.slice(0, 30).map((url, index) => ({
        url,
        fileName: url.split('/').pop()?.split('?')[0] || 'image',
        characterType: index === 0 ? body.subject_character_type : undefined,
        sourceTaskId: index === 0 ? body.source_task_id : undefined,
      }))
      setCharacters(restoredCharacters)
      setPrompt(body.prompt)
      setPromptEnabled(Boolean(body.prompt))
      modeAssets.current[body.mode] = {
        ...modeAssets.current[body.mode],
        characters: restoredCharacters,
        prompt: body.prompt,
        promptEnabled: Boolean(body.prompt),
      }
      setStoredResolution(body.resolution)
      return true
    },
    [
      cancelVideoSelection,
      cancelRecreateAnalysis,
      cancelCharacterUploads,
      pickRemoteVideo,
      setStoredResolution,
      video,
      characters,
      prompt,
      promptEnabled,
    ]
  )

  const setRecreatePrompt = useCallback((value: string) => {
    setRecreatePromptState(value)
  }, [])

  /** The decomposition intent is part of the compiled prompt, so editing it
   * invalidates the accepted decomposition exactly like changing the clip. */
  const setRecreateIntentPrompt = useCallback(
    (value: string) => {
      setRecreateIntentPromptState(value)
      dropRecreateResult()
    },
    [dropRecreateResult]
  )

  /** Switching the intent on/off changes the request the planner sees. */
  const setRecreateIntentPromptEnabled = useCallback(
    (value: boolean) => {
      setRecreateIntentPromptEnabledState(value)
      dropRecreateResult()
    },
    [dropRecreateResult]
  )

  /**
   * Decomposes the reference clip into a compiled prompt plus a readable
   * breakdown. The clip never enters the generation request, so a failure
   * here costs no video credits; the caller keeps every input for a retry.
   */
  const analyzeRecreate = useCallback(async (): Promise<RecreateResult | null> => {
    if (currentMode.current !== 'recreate') return null
    // A repeat click restarts the job; it must never leave two polls running.
    cancelRecreateAnalysis()
    const source = video
    if (!source || source.kind === 'preset' || !source.metadata) {
      toast.error(t('Add a reference video first.'))
      return null
    }
    // Decomposition never depends on the output quality.
    const checked = validateMotionReference(
      source.metadata,
      'recreate',
      '480p',
      outputCapabilities
    )
    if (!checked.ok) {
      handleServerError(
        new Error(t(motionVideoFailureMessage(checked.reason, 'recreate')))
      )
      return null
    }
    const images = characters.slice(0, RECREATE_MAX_IMAGES)
    if (images.length === 0) {
      toast.error(t('Add at least one reference image first.'))
      return null
    }
    const intent = recreateIntentPromptEnabled
      ? recreateIntentPrompt.trim()
      : ''
    if ([...intent].length > RECREATE_INTENT_MAX_RUNES) {
      toast.error(
        t('Intent must be {{count}} characters or fewer.', {
          count: RECREATE_INTENT_MAX_RUNES,
        })
      )
      return null
    }
    const token = recreateTokenRef.current + 1
    recreateTokenRef.current = token
    const abort = new AbortController()
    recreatePollRef.current = { token, abort }
    recreateAnalyzingRef.current = true
    setRecreateAnalyzing(true)
    const isCurrent = () =>
      recreateTokenRef.current === token && !abort.signal.aborted
    const settle = () => {
      if (recreateTokenRef.current !== token) return
      recreateAnalyzingRef.current = false
      setRecreateAnalyzing(false)
      recreatePollRef.current = null
    }
    try {
      const outcome = await pollRecreateAnalysis({
        // A single POST enqueues the job; the helper only GETs afterwards.
        start: () =>
          startRecreateAnalysis({
            video_url: source.url,
            image_urls: images.map((image) => image.url),
            image_tags: [],
            prompt: intent,
            // Analysis never depends on the output quality; the tier only
            // prices the later generation request.
            resolution: '480p',
          }),
        fetchTask: fetchRecreateAnalysis,
        signal: abort.signal,
      })
      if (!isCurrent()) return null
      if (outcome.status === 'cancelled') return null
      if (outcome.status === 'timeout') {
        toast.error(t('Analysis took too long. Please try again.'))
        return null
      }
      if (outcome.status === 'failed') {
        toastInfluencerError(
          t,
          new Error(
            outcome.message ??
              t('Could not analyze the reference video. Please try again.')
          ),
          'Could not analyze the reference video. Please try again.'
        )
        return null
      }
      const prompt = outcome.task.prompt?.trim()
      if (!prompt) {
        handleServerError(
          new Error(
            t('Could not analyze the reference video. Please try again.')
          )
        )
        return null
      }
      // The clip decides the output length; the server reports the derived
      // whole seconds and the raw duration it read.
      const serverDuration = outcome.task.meta?.duration_sec
      const durationSec =
        typeof serverDuration === 'number' && Number.isFinite(serverDuration)
          ? Math.round(serverDuration)
          : source.metadata.durationSec
      const result: RecreateResult = {
        prompt,
        analysis: outcome.task.analysis ?? null,
        imageAnalysis: outcome.task.image_analysis ?? [],
        meta: outcome.task.meta ?? null,
        durationSec,
        referenceDurationSec: source.metadata.rawDurationSec,
      }
      setRecreateResult(result)
      setRecreatePromptState(prompt)
      return result
    } catch (error) {
      if (isCurrent()) {
        toastInfluencerError(
          t,
          error,
          'Could not analyze the reference video. Please try again.'
        )
      }
      return null
    } finally {
      settle()
    }
  }, [
    video,
    characters,
    recreateIntentPrompt,
    recreateIntentPromptEnabled,
    outputCapabilities,
    cancelRecreateAnalysis,
    t,
  ])

  /**
   * What actually reaches the gateway: the style's instruction joined with the
   * caller's own words. The style is an explicit pick and always travels; the
   * prompt box is opt-in, so its draft only joins once it is switched on. The
   * gateway then prepends its mode framing, giving framing → style → caller.
   */
  const effectivePrompt = useMemo(() => {
    // A recreation prompt is already the server's full compiled text: no
    // local mode framing or visual-style instruction is joined onto it.
    if (mode === 'recreate') return recreatePrompt.trim()
    return combineStylePrompt(style?.prompt, promptEnabled ? prompt : '', mode)
  }, [style, prompt, promptEnabled, mode, recreatePrompt])

  /** Recreate uses the first nine references, in upload order. */
  const recreateImages = useMemo(
    () => characters.slice(0, RECREATE_MAX_IMAGES),
    [characters]
  )
  const recreatePromptRunes = useMemo(
    () => [...recreatePrompt.trim()].length,
    [recreatePrompt]
  )
  const videoRawDurationSec = videoMetadata?.rawDurationSec ?? 0
  /** Exactly what the generation request will carry: the server's derived
   * duration when available, otherwise the local rounded clip duration. */
  const recreateSubmitDuration = useMemo(() => {
    const fromMeta = recreateResult?.meta?.duration_sec
    if (typeof fromMeta === 'number' && Number.isFinite(fromMeta)) {
      return Math.round(fromMeta)
    }
    if (recreateResult) return Math.round(recreateResult.durationSec)
    return videoMetadata?.durationSec ?? 0
  }, [recreateResult, videoMetadata])
  /** Generation credits for the active quality, straight from the server. */
  const recreateQuote = useMemo(
    () =>
      mode === 'recreate' && !recreateAnalyzing
        ? recreateQuoteFor(recreateResult?.meta, resolution)
        : null,
    [mode, recreateAnalyzing, recreateResult, resolution]
  )

  const canSubmit = useMemo(() => {
    const videoUsable =
      video !== null &&
      video.kind !== 'preset' &&
      video.metadata !== undefined &&
      validateMotionReference(
        video.metadata,
        mode,
        resolution,
        outputCapabilities
      ).ok
    // One click starts analysis and submission together, so the CTA is ready
    // as soon as the inputs are; the compiled prompt does not exist yet.
    let recreateReady = true
    if (mode === 'recreate') {
      recreateReady =
        !recreateAnalyzing &&
        recreateImages.length > 0 &&
        (!recreateIntentPromptEnabled ||
          [...recreateIntentPrompt.trim()].length <=
            RECREATE_INTENT_MAX_RUNES)
    }
    return (
      !characterUploading &&
      !videoSelection.busy &&
      recreateReady &&
      canSubmitMotion({
        mode,
        hasCharacter: recreateImages.length > 0,
        hasVideo: videoUsable,
      })
    )
  }, [
    mode,
    recreateImages,
    recreateAnalyzing,
    recreateIntentPrompt,
    recreateIntentPromptEnabled,
    video,
    resolution,
    outputCapabilities,
    videoSelection.busy,
    characterUploading,
  ])

  return {
    mode,
    setMode,
    meta: motionModeMeta(mode),
    character,
    characters,
    uploadedCharacters,
    uploadedVideos,
    characterUploading,
    characterUploadProgress,
    characterUploads: characterUploader.uploads,
    videoUploadProgress,
    selectUploadedVideo: videoSelection.pickUploaded,
    setCharacters: setAllCharacters,
    addCharacter,
    removeCharacter,
    toggleCharacter,
    video,
    prompt,
    setPrompt,
    recreateImages,
    recreateResult,
    recreatePrompt,
    setRecreatePrompt,
    recreatePromptRunes,
    recreateIntentPrompt,
    setRecreateIntentPrompt,
    recreateIntentPromptEnabled,
    setRecreateIntentPromptEnabled,
    recreateSubmitDuration,
    videoRawDurationSec,
    recreateQuote,
    recreateAnalyzing,
    analyzeRecreate,
    resetRecreateAnalysis: dropRecreateResult,
    style,
    applyStyle,
    effectivePrompt,
    promptEnabled,
    setPromptEnabled,
    durationSec,
    videoMetadataLoading,
    videoValidation,
    resolution,
    setResolution,
    outputAspect,
    outputCapabilities,
    setOutputCapabilities,
    pickCharacter,
    pickCharacters,
    retryCharacterUpload,
    dismissCharacterUpload: characterUploader.dismiss,
    isCharacterUploading: () => characterUploading,
    pickVideo,
    setRemoteVideo,
    clearCharacter,
    clearVideo,
    reset,
    restoreTask,
    canSubmit,
    characterPickerOpen,
    openCharacterPicker,
    closeCharacterPicker,
    videoPickerOpen,
    openVideoPicker,
    closeVideoPicker,
    pickHistoryVideo: (task: TaskRecord) => {
      videoTargetMode.current = currentMode.current
      return videoSelection.pickHistory(task)
    },
    videoPreparing: videoSelection.busy,
    videoPreparationPhase: videoSelection.phase,
    preparingTaskId: videoSelection.preparingTaskId,
    cancelVideoSelection,
    isVideoPreparing: videoSelection.isPreparing,
  }
}

/** The shape the motion form renders from; named so the panel can take it whole. */
export type MotionForm = ReturnType<typeof useMotion>
