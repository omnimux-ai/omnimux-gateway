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
import { useEffect, useRef, useState } from 'react'

export type MediaPhase = 'loading' | 'ready' | 'waiting' | 'unavailable'

/** Only our stable same-origin artifact entry supports proxy delivery. */
function proxySource(source: string): string {
  if (
    !source.startsWith('/') ||
    source.startsWith('//') ||
    source.includes('\\')
  ) {
    return source
  }
  const parsed = new URL(source, 'https://task-content.invalid')
  if (parsed.origin !== 'https://task-content.invalid') return source
  if (
    !/^\/v1\/tasks\/[^/]+\/artifacts\/[^/]+\/content$/.test(parsed.pathname)
  ) {
    return source
  }
  parsed.searchParams.delete('delivery')
  return parsed.pathname + parsed.search
}

/** A keyed media lifecycle has one bounded budget, independent of generation. */
export function useMediaRecovery(source: string) {
  const [state, setState] = useState({
    attempt: 0,
    cycle: 0,
    phase: 'loading' as MediaPhase,
  })
  const mounted = useRef(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  useEffect(() => {
    if (state.phase !== 'waiting') return
    let timer: ReturnType<typeof setTimeout> | undefined
    const schedule = () => {
      if (timer !== undefined) clearTimeout(timer)
      timer = undefined
      if (document.visibilityState === 'hidden' || !navigator.onLine) return
      timer = setTimeout(
        () => {
          setState((current) => {
            if (
              current.cycle !== state.cycle ||
              current.attempt !== state.attempt ||
              current.phase !== 'waiting'
            ) {
              return current
            }
            return {
              ...current,
              attempt: current.attempt + 1,
              phase: 'loading',
            }
          })
        },
        1000 * (state.attempt + 1)
      )
    }
    schedule()
    document.addEventListener('visibilitychange', schedule)
    window.addEventListener('online', schedule)
    window.addEventListener('offline', schedule)
    return () => {
      if (timer !== undefined) clearTimeout(timer)
      document.removeEventListener('visibilitychange', schedule)
      window.removeEventListener('online', schedule)
      window.removeEventListener('offline', schedule)
    }
  }, [state.phase, state.attempt, state.cycle])

  const update = (phase: 'ready' | 'waiting') => {
    if (!mounted.current) return
    setState((current) => {
      if (
        current.attempt !== state.attempt ||
        current.cycle !== state.cycle ||
        current.phase === 'waiting' ||
        current.phase === 'unavailable'
      ) {
        return current
      }
      return {
        ...current,
        phase:
          phase === 'waiting' && current.attempt >= 2 ? 'unavailable' : phase,
      }
    })
  }
  return {
    phase: state.phase,
    key: `${state.cycle}:${state.attempt}`,
    src: state.attempt === 0 ? source : proxySource(source),
    onLoad: () => update('ready'),
    onError: () => update('waiting'),
    reload: () =>
      setState((current) => ({
        attempt: 0,
        cycle: current.cycle + 1,
        phase: 'loading',
      })),
  }
}
