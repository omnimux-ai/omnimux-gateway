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
import { useCallback, useEffect, useState } from 'react'

import {
  type CatalogManifest,
  fetchCatalogManifest,
  inspirationCatalogBase,
  isAbortError,
  toCatalogError,
} from '../lib/catalog'

export interface InspirationManifestState {
  manifest: CatalogManifest | null
  loading: boolean
  error: Error | null
  /** Re-requests the manifest; wired to the error state's retry action. */
  reload: () => void
}

/**
 * Loads `manifest.json` once per mounted page and keeps it for its lifetime.
 *
 * Every count, model label and filter-chip vocabulary the prompt pages print
 * comes from the manifest, so nothing in the feature hard-codes today's
 * numbers. The request is aborted when the page unmounts, so a late response
 * can never set state on a dead component.
 */
export function useInspirationManifest(
  base: string = inspirationCatalogBase()
): InspirationManifestState {
  const [state, setState] = useState<{
    manifest: CatalogManifest | null
    loading: boolean
    error: Error | null
  }>({ manifest: null, loading: true, error: null })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setState({ manifest: null, loading: true, error: null })

    fetchCatalogManifest(base, controller.signal)
      .then((manifest) => {
        if (controller.signal.aborted) return
        setState({ manifest, loading: false, error: null })
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || isAbortError(error)) return
        setState({ manifest: null, loading: false, error: toCatalogError(error) })
      })

    return () => controller.abort()
  }, [base, attempt])

  const reload = useCallback(() => setAttempt((current) => current + 1), [])

  return { manifest: state.manifest, loading: state.loading, error: state.error, reload }
}
