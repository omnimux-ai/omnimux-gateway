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

import type { MotionSubmitBody } from '../api'
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
import type {
  MotionAspectRatio,
  MotionCharacterSource,
  MotionMode,
  MotionStyleChoice,
  MotionVideoSource,
  TaskRecord,
} from '../types'
import { useCharacterUploads } from './use-character-uploads'
import { useUploadLibrary } from './use-upload-library'
import { useVideoReferenceSelection } from './use-video-reference-selection'

/** Free-tier callers are limited to the lower rung; the switch still shows both. */
export const MOTION_RESOLUTIONS = ['480p', '720p']

/** The planner accepts at most this many reference images, in upload order. */
export const RECREATE_MAX_IMAGES = 30

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
  // The decomposition intent keeps its own draft and switch so switching modes
  // never leaks a generation prompt into the recreation form.
  const [recreateIntentPrompt, setRecreateIntentPromptState] = useState('')
  const [recreateIntentPromptEnabled, setRecreateIntentPromptEnabledState] =
    useState(false)
  const [styles, setStyles] = useState<
    Record<MotionMode, MotionStyleChoice | null>
  >({
    motion: null,
    replace: null,
    recreate: null,
  })
  const style = styles[mode]
  const [promptEnabled, setPromptEnabled] = useState(false)
  const modeAssets =
    useRef<Record<MotionMode, ModeAssetSnapshot>>(emptyModeAssets())
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
    [rememberVideoMetadata]
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
    [cancelVideoSelection, video, characters, prompt, promptEnabled]
  )

  /** All three modes now share the 30-reference ceiling. */
  const imageLimit = useCallback(
    () => (currentMode.current === 'recreate' ? RECREATE_MAX_IMAGES : 30),
    []
  )

  const addCharacter = useCallback(
    (source: MotionCharacterSource) => {
      const limit = imageLimit()
      setCharacters((prev) => {
        if (prev.length >= limit) return prev
        if (prev.some((c) => c.url === source.url)) return prev
        return [...prev, source]
      })
    },
    [imageLimit]
  )

  /**
   * Multi-select uploads run through the shared batch queue: three at a time,
   * capped by the batch limit and the current mode's reference ceiling. Each
   * finished file appends to the ordered reference list exactly like before.
   */
  const addUploadedCharacter = useCallback((source: MotionCharacterSource) => {
    const target = characterTargetMode.current
    const limit = target === 'recreate' ? RECREATE_MAX_IMAGES : 30
    const append = (prev: MotionCharacterSource[]) => {
      if (prev.length >= limit) return prev
      if (prev.some((c) => c.url === source.url)) return prev
      return [...prev, source]
    }
    if (target === currentMode.current) {
      setCharacters(append)
      return
    }
    modeAssets.current[target] = {
      ...modeAssets.current[target],
      characters: append(modeAssets.current[target].characters),
    }
  }, [])
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

  const removeCharacter = useCallback((index: number) => {
    setCharacters((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const setAllCharacters = useCallback(
    (list: MotionCharacterSource[]) => {
      setCharacters(list.slice(0, imageLimit()))
    },
    [imageLimit]
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
    },
    [imageLimit]
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
  }, [])

  /** A visual style survives removal of its motion reference. */
  const clearVideo = useCallback(() => {
    cancelVideoSelection()
    if (mode === 'replace') {
      setStyles((current) => ({ ...current, replace: null }))
    }
    setVideo(null)
  }, [mode, cancelVideoSelection])

  const reset = useCallback(() => {
    cancelVideoSelection()
    modeAssets.current = emptyModeAssets()
    setVideoPickerOpen(false)
    setCharacterPickerOpen(false)
    setCharacters([])
    cancelCharacterUploads()
    setVideo(null)
    setStyles({ motion: null, replace: null, recreate: null })
    setPrompt('')
    setPromptEnabled(false)
    setRecreateIntentPromptState('')
    setRecreateIntentPromptEnabledState(false)
  }, [cancelVideoSelection, cancelCharacterUploads])

  /** Restore a historical form atomically without uploading or submitting. */
  const restoreTask = useCallback(
    async (body: MotionSubmitBody): Promise<boolean> => {
      cancelVideoSelection()
      modeAssets.current[currentMode.current] = {
        video,
        characters,
        prompt,
        promptEnabled,
      }
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
      if (body.mode === 'recreate') {
        setRecreateIntentPromptState(body.prompt)
        setRecreateIntentPromptEnabledState(Boolean(body.prompt))
      } else {
        setPrompt(body.prompt)
        setPromptEnabled(Boolean(body.prompt))
      }
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
      cancelCharacterUploads,
      pickRemoteVideo,
      setStoredResolution,
      video,
      characters,
      prompt,
      promptEnabled,
    ]
  )

  const setRecreateIntentPrompt = useCallback((value: string) => {
    setRecreateIntentPromptState(value)
  }, [])

  const setRecreateIntentPromptEnabled = useCallback((value: boolean) => {
    setRecreateIntentPromptEnabledState(value)
  }, [])

  /**
   * What actually reaches the gateway: the style's instruction joined with the
   * caller's own words. The style is an explicit pick and always travels; the
   * prompt box is opt-in, so its draft only joins once it is switched on. The
   * gateway then prepends its mode framing, giving framing → style → caller.
   * Recreation sends only the caller's raw intent; the worker compiles it.
   */
  const effectivePrompt = useMemo(() => {
    if (mode === 'recreate') {
      return recreateIntentPromptEnabled ? recreateIntentPrompt.trim() : ''
    }
    return combineStylePrompt(style?.prompt, promptEnabled ? prompt : '', mode)
  }, [
    style,
    prompt,
    promptEnabled,
    mode,
    recreateIntentPrompt,
    recreateIntentPromptEnabled,
  ])

  /** Recreate uses the first thirty references, in upload order. */
  const recreateImages = useMemo(
    () => characters.slice(0, RECREATE_MAX_IMAGES),
    [characters]
  )
  const videoRawDurationSec = videoMetadata?.rawDurationSec ?? 0

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
    const recreateReady =
      recreateImages.length > 0 &&
      (!recreateIntentPromptEnabled ||
        [...recreateIntentPrompt.trim()].length <= RECREATE_INTENT_MAX_RUNES)
    return (
      !characterUploading &&
      !videoSelection.busy &&
      (mode !== 'recreate' || recreateReady) &&
      canSubmitMotion({
        mode,
        hasCharacter: recreateImages.length > 0,
        hasVideo: videoUsable,
      })
    )
  }, [
    mode,
    recreateImages,
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
    recreateIntentPrompt,
    setRecreateIntentPrompt,
    recreateIntentPromptEnabled,
    setRecreateIntentPromptEnabled,
    videoRawDurationSec,
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
