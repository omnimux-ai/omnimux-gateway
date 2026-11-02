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

For commercial licensing, please contact support@quantumnous.com.
*/

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import {
  lockSubset,
  normalizeSession,
  type WorkbenchSession,
} from '../lib/builder-draft'
import { randomizeSelection } from '../lib/randomize'
import {
  filterSelectionForTier,
  reconcileSelection,
  trySelect,
  type ReconcileResult,
  type RemovedPick,
} from '../lib/taxonomy'
import {
  dropLegacyDraft,
  readSession,
  writeSession,
} from '../lib/workbench-session'
import type { InfluencerTaxonomy, LookLocks, Selection } from '../types'
import { useImageReference } from './use-image-reference'

/** What a tier change had to take out, so the caller can report it. */
export interface TierChangeResult {
  /** Options the new tier does not show at all. */
  dropped: string[]
  /** Options the new tier shows but forbids next to the rest of the look. */
  removed: RemovedPick[]
}

/** A whole look written into the panel in one step. */
export interface ApplyLookInput {
  tier?: string
  values: Selection
  /**
   * Locks recorded by the visitor's own gestures, carried over by a draft that
   * captured them (landing page, pre-login). Omitted means a whole-look
   * replacement: the look arrives without claiming any category.
   */
  locks?: LookLocks
  seed?: number
  image_url?: string
  image_mime_type?: string
  model?: string
  group?: string
  vendor_id?: number | null
}

export interface BuilderState {
  tier: string
  selection: Selection
  brief: ''
  seed: string
  imageUrl: string
  imageMode: 'text' | 'reference'
  imageMimeType: string
  model: string
  group: string
  vendorId: number | null
  openGroups: Set<string>
}

export function useBuilder(taxonomy: InfluencerTaxonomy | null) {
  const [savedSession] = useState(readSession)
  const [tier, setTier] = useState(savedSession?.tier ?? 'total')
  const [selection, setSelection] = useState<Selection>(
    savedSession?.selection ?? {}
  )
  const [locks, setLocks] = useState<LookLocks>(savedSession?.locks ?? {})
  const initializedRef = useRef(false)
  const [initialized, setInitialized] = useState(false)
  const lastTaxonomyRef = useRef<InfluencerTaxonomy | null>(null)

  // A tab upgraded from the release that persisted this session in storage must
  // not stay pinned to the stale layer it wrote.
  useEffect(() => {
    dropLegacyDraft()
  }, [])

  useEffect(() => {
    if (!taxonomy || lastTaxonomyRef.current === taxonomy) return
    lastTaxonomyRef.current = taxonomy
    if (initializedRef.current) {
      const session = normalizeSession(taxonomy, {
        version: 1,
        tier,
        selection,
        locks,
        initialized: true,
      })
      setTier(session.tier)
      setSelection(session.selection)
      setLocks(session.locks)
      return
    }
    initializedRef.current = true
    if (savedSession) {
      const session = normalizeSession(taxonomy, savedSession)
      setTier(session.tier)
      setSelection(session.selection)
      setLocks(session.locks)
    } else {
      setSelection(randomizeSelection(taxonomy, 'total'))
    }
    setInitialized(true)
  }, [taxonomy, savedSession, tier, selection, locks])

  useEffect(() => {
    if (!initialized) return
    const session: WorkbenchSession = {
      version: 1,
      tier,
      selection,
      locks,
      initialized: true,
    }
    writeSession(session)
  }, [initialized, tier, selection, locks])

  const [seed, setSeed] = useState('')
  const [imageUrl, writeImageUrl] = useState('')
  const [imageMimeType, setImageMimeType] = useState('')
  const imageMode = imageUrl.trim() ? 'reference' : 'text'
  const writeReferenceUrl = useCallback((url: string, mimeType = '') => {
    writeImageUrl(url)
    setImageMimeType(mimeType)
  }, [])
  const image = useImageReference(writeReferenceUrl)
  const { invalidateImage } = image
  const setImageUrl = useCallback(
    (url: string, mimeType?: string) => {
      invalidateImage()
      writeReferenceUrl(url, mimeType)
    },
    [invalidateImage, writeReferenceUrl]
  )
  const [model, setModel] = useState('')
  const [group, setGroup] = useState('')
  const [vendorId, setVendorId] = useState<number | null>(null)
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => {
    if (taxonomy) return new Set(taxonomy.categories.map((c) => c.id))
    return new Set()
  })
  const initializedGroupsRef = useRef(false)

  // 默认全展开所有分类
  useEffect(() => {
    if (taxonomy && !initializedGroupsRef.current) {
      setOpenGroups(new Set(taxonomy.categories.map((c) => c.id)))
      initializedGroupsRef.current = true
    }
  }, [taxonomy])

  const pick = useCallback(
    (catId: string, optId: string): string | null => {
      if (!taxonomy) return null
      const res = trySelect(taxonomy.categories, tier, selection, catId, optId)
      if (res.message) return res.message
      if (JSON.stringify(res.selection) === JSON.stringify(selection)) {
        return null
      }
      // A hand pick is the only way to take ownership of a category, and
      // cancelling its last pick hands the category back to the dice.
      const nextLocks = { ...locks }
      const picks = (res.selection[catId] ?? []).filter(
        (id) => id === optId || locks[catId]?.includes(id)
      )
      if (picks.length) nextLocks[catId] = picks
      else delete nextLocks[catId]
      setLocks(nextLocks)
      setSelection(res.selection)
      return null
    },
    [taxonomy, tier, selection, locks]
  )

  const switchTier = useCallback(
    (next: string): TierChangeResult => {
      if (!taxonomy) {
        setTier(next)
        return { dropped: [], removed: [] }
      }
      const { selection: filtered, dropped } = filterSelectionForTier(
        taxonomy.categories,
        next,
        selection
      )
      // 切档会让原本合法的一对变成硬冲突（Average 档的 Muscular + Short legs
      // 切到 Bold/Extreme 就非法），而切档没有逐项拒绝的时机，因此与套用整份
      // 形象共用同一条修正，避免切档后卡在无法生成的状态。
      const { selection: reconciled, removed } = reconcileSelection(
        taxonomy.categories,
        next,
        filtered
      )
      setTier(next)
      setSelection(reconciled)
      setLocks(lockSubset(reconciled, locks))
      return { dropped, removed }
    },
    [taxonomy, selection, locks]
  )

  const randomize = useCallback((): Selection => {
    if (!taxonomy) return {}
    const sel = randomizeSelection(taxonomy, tier, Math.random, locks)
    setSelection(sel)
    return sel
  }, [taxonomy, tier, locks])

  const resetFeatures = useCallback(() => {
    initializedRef.current = true
    setInitialized(true)
    setSelection({})
    setLocks({})
  }, [])

  const reset = useCallback(() => {
    resetFeatures()
    setSeed('')
    setImageUrl('')
  }, [setImageUrl, resetFeatures])

  const toggleGroup = useCallback((catId: string) => {
    setOpenGroups((prev) => {
      const next = new Set(prev)
      if (next.has(catId)) next.delete(catId)
      else next.add(catId)
      return next
    })
  }, [])

  /**
   * Writes a whole look (an inspiration preset, a saved task, a draft carried
   * over from the landing page) into the panel in one step.
   *
   * This is the only way a look enters the panel, so no caller can turn values
   * into locks by accident: a lock exists only where the visitor picked by hand
   * and a draft carried that pick over. Returns the selection that was actually
   * written plus the picks a hard conflict forced out, so the caller can report
   * what changed and submit exactly what the panel now holds.
   */
  const applyLook = useCallback(
    (input: ApplyLookInput): ReconcileResult => {
      const nextTier = input.tier ?? tier
      const reconciled = taxonomy
        ? reconcileSelection(taxonomy.categories, nextTier, input.values)
        : { selection: input.values, removed: [] as RemovedPick[] }

      if (input.tier) setTier(input.tier)
      initializedRef.current = true
      setInitialized(true)
      setSelection(reconciled.selection)
      setLocks(lockSubset(reconciled.selection, input.locks ?? {}))
      setSeed(input.seed != null ? String(input.seed) : '')
      setImageUrl(input.image_url ?? '', input.image_mime_type)
      if (input.model) setModel(input.model)
      if (input.group) setGroup(input.group)
      if (input.vendor_id != null) setVendorId(input.vendor_id)
      return reconciled
    },
    [taxonomy, tier, setImageUrl]
  )

  const state: BuilderState = useMemo(
    () => ({
      tier,
      selection,
      brief: '' as const,
      seed,
      imageUrl,
      imageMode,
      imageMimeType,
      model,
      group,
      vendorId,
      openGroups,
    }),
    [
      tier,
      selection,
      seed,
      imageUrl,
      imageMode,
      imageMimeType,
      model,
      group,
      vendorId,
      openGroups,
    ]
  )

  return {
    ...image,
    state,
    locks,
    pick,
    switchTier,
    randomize,
    reset,
    resetFeatures,
    toggleGroup,
    applyLook,
    setSeed,
    setImageUrl,
    setModel,
    setGroup,
    setVendorId,
  }
}
