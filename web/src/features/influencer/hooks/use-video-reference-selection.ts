import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { handleServerError } from '@/lib/handle-server-error'

import { fetchTaskRecord } from '../api'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import { influencerErrorText } from '../lib/error-copy'
import {
  checkVideoFileBasic,
  validateBrowserMotionMetadata,
  validateMotionReference,
  reusableMotionMetadata,
  motionVideoFailureMessage,
  type VideoValidationResult,
} from '../lib/motion'
import { referenceVideoCandidate } from '../lib/reference-candidates'
import type {
  MotionAspectRatio,
  MotionMode,
  MotionVideoMetadata,
  MotionVideoSource,
  TaskRecord,
} from '../types'

interface Props {
  mode?: MotionMode
  resolution?: string
  outputAspects?: Record<string, MotionAspectRatio[]>
  onCommit: (source: MotionVideoSource) => void
  /** The same complete local result is persisted alongside the uploaded URL. */
  upload: (
    file: File,
    metadata: MotionVideoMetadata,
    signal?: AbortSignal
  ) => Promise<{ url: string; metadata: MotionVideoMetadata }>
}

type SelectableSource = Exclude<MotionVideoSource, { kind: 'preset' }>

/** A pending candidate never replaces the last successfully checked source. */
export function useVideoReferenceSelection(props: Props) {
  const { t } = useTranslation()
  const [busy, setBusy] = useState(false)
  const [phase, setPhase] = useState<
    'reading' | 'checking' | 'uploading' | null
  >(null)
  const [preparingTaskId, setPreparingTaskId] = useState<string>()
  const request = useRef(0)
  const locked = useRef(false)
  const controller = useRef<AbortController | null>(null)
  const callbacks = useRef(props)
  callbacks.current = props

  const cancel = useCallback(() => {
    request.current += 1
    controller.current?.abort()
    controller.current = null
    locked.current = false
    setBusy(false)
    setPhase(null)
    setPreparingTaskId(undefined)
  }, [])

  useEffect(
    () => () => {
      request.current += 1
      controller.current?.abort()
      locked.current = false
    },
    []
  )

  const begin = useCallback((taskId?: string) => {
    if (locked.current) return null
    locked.current = true
    const id = ++request.current
    const abort = new AbortController()
    controller.current = abort
    setBusy(true)
    setPreparingTaskId(taskId)
    return { id, abort }
  }, [])

  const finish = useCallback((id: number) => {
    if (id !== request.current) return
    locked.current = false
    controller.current = null
    setBusy(false)
    setPhase(null)
    setPreparingTaskId(undefined)
  }, [])

  const report = useCallback(
    (failure: unknown) => {
      // The local text goes through the influencer helper so a raw Chinese
      // server reason never reaches a non-Chinese UI. The toast itself stays on
      // handleServerError, which de-dupes a failure that is already reported.
      const message = influencerErrorText(t, failure, 'Could not prepare video')
      handleServerError(failure, message, { title: message })
    },
    [t]
  )

  const pickVideo = useCallback(
    async (file: File): Promise<VideoValidationResult> => {
      const operation = begin()
      if (!operation) return { ok: false, reason: 'corrupt' }
      try {
        const basic = checkVideoFileBasic(file)
        if (!basic.ok) {
          report(
            new Error(
              motionVideoFailureMessage(
                basic.reason,
                callbacks.current.mode ?? 'motion'
              )
            )
          )
          return basic
        }
        const mode = callbacks.current.mode ?? 'motion'
        setPhase('reading')
        const raw = await readBrowserVideoMetadata(file, operation.abort.signal)
        if (
          operation.id !== request.current ||
          operation.abort.signal.aborted
        ) {
          return { ok: false, reason: 'corrupt' }
        }
        const checked = validateBrowserMotionMetadata(raw, mode)
        const ready = checked.ok
          ? validateMotionReference(
              checked,
              mode,
              callbacks.current.resolution ?? '480p',
              callbacks.current.outputAspects
            )
          : checked
        if (!ready.ok) {
          report(new Error(motionVideoFailureMessage(ready.reason, mode)))
          return ready
        }
        setPhase('uploading')
        const prepared = await callbacks.current.upload(
          file,
          ready,
          operation.abort.signal
        )
        if (
          operation.id !== request.current ||
          operation.abort.signal.aborted
        ) {
          return { ok: false, reason: 'corrupt' }
        }
        callbacks.current.onCommit({
          kind: 'upload',
          url: prepared.url,
          fileName: file.name,
          metadata: ready,
        })
        return ready
      } catch (failure) {
        if (
          operation.id === request.current &&
          !operation.abort.signal.aborted
        ) {
          report(failure)
        }
        return { ok: false, reason: 'corrupt' }
      } finally {
        finish(operation.id)
      }
    },
    [begin, finish, report]
  )

  const prepare = useCallback(
    async (
      source: SelectableSource,
      operation: { id: number; abort: AbortController },
      mode?: MotionMode,
      resolution?: string
    ): Promise<boolean> => {
      setPhase('reading')
      const referenceMode = mode ?? callbacks.current.mode ?? 'motion'
      let metadata = reusableMotionMetadata(source.metadata, referenceMode)
      if (!metadata) {
        const raw = await readBrowserVideoMetadata(
          source.url,
          operation.abort.signal
        )
        const checked = validateBrowserMotionMetadata(raw, referenceMode)
        if (!checked.ok) {
          throw new Error(
            motionVideoFailureMessage(checked.reason, referenceMode)
          )
        }
        metadata = checked
      }
      if (operation.id !== request.current || operation.abort.signal.aborted) {
        return false
      }
      const checked = validateMotionReference(
        metadata,
        referenceMode,
        resolution ?? callbacks.current.resolution ?? '480p',
        callbacks.current.outputAspects
      )
      if (!checked.ok) {
        throw new Error(
          motionVideoFailureMessage(checked.reason, referenceMode)
        )
      }
      metadata = checked
      callbacks.current.onCommit({ ...source, metadata })
      return true
    },
    []
  )

  const pickRemote = useCallback(
    async (
      source: SelectableSource,
      mode?: MotionMode,
      resolution?: string
    ): Promise<boolean> => {
      const operation = begin(
        source.kind === 'remote' ? source.taskId : undefined
      )
      if (!operation) return false
      try {
        return await prepare(source, operation, mode, resolution)
      } catch (failure) {
        if (
          operation.id === request.current &&
          !operation.abort.signal.aborted
        ) {
          report(failure)
        }
        return false
      } finally {
        finish(operation.id)
      }
    },
    [begin, finish, prepare, report]
  )

  const pickHistory = useCallback(
    async (task: TaskRecord): Promise<void> => {
      const candidate = referenceVideoCandidate(task)
      if (!candidate) return
      const operation = begin(candidate.taskId)
      if (!operation) return
      try {
        setPhase('reading')
        const refreshed = await fetchTaskRecord(candidate.taskId, {
          action: 'influencer_motion',
          with_artifacts: true,
          signal: operation.abort.signal,
        })
        if (operation.id !== request.current) return
        const current = refreshed ? referenceVideoCandidate(refreshed) : null
        if (
          !current ||
          current.taskId !== candidate.taskId ||
          current.artifactKey !== candidate.artifactKey ||
          !current.referenceUrl
        ) {
          throw new Error(
            'This video is unavailable. Choose another video or upload a file.'
          )
        }
        await prepare(
          {
            kind: 'remote',
            url: current.referenceUrl,
            name: current.title,
            previewUrl: current.previewUrl,
            taskId: current.taskId,
            artifactKey: current.artifactKey,
            ...(current.metadata ? { metadata: current.metadata } : {}),
          },
          operation
        )
      } catch (failure) {
        if (
          operation.id === request.current &&
          !operation.abort.signal.aborted
        ) {
          report(failure)
        }
      } finally {
        finish(operation.id)
      }
    },
    [begin, finish, prepare, report]
  )

  const isPreparing = useCallback(() => locked.current, [])
  return {
    busy,
    phase,
    preparingTaskId,
    cancel,
    pickVideo,
    pickHistory,
    pickRemote,
    pickUploaded: pickRemote,
    isPreparing,
  }
}
