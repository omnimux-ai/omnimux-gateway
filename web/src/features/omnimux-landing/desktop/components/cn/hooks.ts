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
import { useCallback, useEffect, useRef, useState } from 'react'

const REDUCED_QUERY = '(prefers-reduced-motion: reduce)'

export function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia(REDUCED_QUERY).matches
  )
}

export function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(prefersReducedMotion)
  useEffect(() => {
    const media = window.matchMedia(REDUCED_QUERY)
    const onChange = () => setReduced(media.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])
  return reduced
}

/**
 * Scripted product demo clock. Runs once when the element scrolls into view
 * and can be replayed. With reduced motion the clock jumps to the end state.
 */
export function useDemoClock(totalMs: number, threshold = 0.35) {
  const ref = useRef<HTMLDivElement>(null)
  const [elapsed, setElapsed] = useState(0)
  const frame = useRef<number | null>(null)
  const started = useRef(false)
  const reduced = usePrefersReducedMotion()

  const stop = useCallback(() => {
    if (frame.current !== null) {
      cancelAnimationFrame(frame.current)
      frame.current = null
    }
  }, [])

  const run = useCallback(() => {
    stop()
    if (reduced) {
      setElapsed(Number.POSITIVE_INFINITY)
      return
    }
    setElapsed(0)
    const start = performance.now()
    const tick = (now: number) => {
      const value = now - start
      setElapsed(value)
      if (value < totalMs) frame.current = requestAnimationFrame(tick)
    }
    frame.current = requestAnimationFrame(tick)
  }, [reduced, stop, totalMs])

  useEffect(() => {
    if (reduced) {
      setElapsed(Number.POSITIVE_INFINITY)
      return
    }
    const node = ref.current
    if (!node) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting) && !started.current) {
          started.current = true
          run()
          observer.disconnect()
        }
      },
      { threshold }
    )
    observer.observe(node)
    return () => {
      observer.disconnect()
      stop()
    }
  }, [reduced, run, stop, threshold])

  return { ref, elapsed, run, reduced }
}

export function useInView<T extends Element>(
  options: IntersectionObserverInit = { threshold: 0.25 }
) {
  const ref = useRef<T>(null)
  const [inView, setInView] = useState(false)
  useEffect(() => {
    const node = ref.current
    if (!node) return
    const observer = new IntersectionObserver((entries) => {
      setInView(entries.some((entry) => entry.isIntersecting))
    }, options)
    observer.observe(node)
    return () => observer.disconnect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  return { ref, inView }
}
