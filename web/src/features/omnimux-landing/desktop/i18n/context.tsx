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
/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'

import { LANDING_LANG_STORAGE_KEY } from '@/lib/landing-storage'

import type { LandingCopy, LandingLang, LandingLangOption } from './types'

export { LANDING_LANG_STORAGE_KEY } from '@/lib/landing-storage'

export const LANDING_LANGS: readonly LandingLangOption[] = [
  { code: 'en', label: 'English', short: 'EN', htmlLang: 'en' },
  { code: 'id', label: 'Bahasa Indonesia', short: 'ID', htmlLang: 'id' },
  { code: 'es', label: 'Español', short: 'ES', htmlLang: 'es' },
  { code: 'ja', label: '日本語', short: '日', htmlLang: 'ja' },
  { code: 'zh-TW', label: '繁體中文', short: '繁', htmlLang: 'zh-Hant' },
  { code: 'zh-CN', label: '简体中文', short: '中', htmlLang: 'zh-CN' },
]

const copyLoaders: Record<LandingLang, () => Promise<LandingCopy>> = {
  en: () => import('./locales/en').then((mod) => mod.en),
  id: () => import('./locales/id').then((mod) => mod.id),
  es: () => import('./locales/es').then((mod) => mod.es),
  ja: () => import('./locales/ja').then((mod) => mod.ja),
  'zh-TW': () => import('./locales/zh-TW').then((mod) => mod.zhTW),
  'zh-CN': () => import('./locales/zh-CN').then((mod) => mod.zhCN),
}

export function loadLandingCopy(lang: LandingLang): Promise<LandingCopy> {
  return copyLoaders[lang]()
}

function normalizeLang(value: string | null | undefined): LandingLang | null {
  if (!value) return null
  const v = value.trim().toLowerCase()
  if (!v) return null
  if (v === 'en' || v.startsWith('en-')) return 'en'
  if (v === 'id' || v === 'in' || v.startsWith('id-') || v.startsWith('in-')) {
    return 'id'
  }
  if (v === 'es' || v.startsWith('es-')) {
    return 'es'
  }
  if (v === 'ja' || v === 'jp' || v.startsWith('ja-') || v.startsWith('jp-')) {
    return 'ja'
  }
  if (
    v === 'zh' ||
    v.startsWith('zh-') ||
    v === 'cn' ||
    v === 'tw' ||
    v === 'hant' ||
    v === 'hans'
  ) {
    if (
      v.includes('hant') ||
      v.includes('tw') ||
      v.includes('hk') ||
      v.includes('mo')
    ) {
      return 'zh-TW'
    }
    return 'zh-CN'
  }
  return null
}

export function landingLangOption(lang: LandingLang): LandingLangOption {
  return (
    LANDING_LANGS.find((option) => option.code === lang) ?? LANDING_LANGS[5]
  )
}

/**
 * Resolution order: ?lang= → localStorage → navigator.languages → zh-CN.
 */
export function resolveLandingLang(): LandingLang {
  if (typeof window === 'undefined') return 'zh-CN'
  const param = normalizeLang(
    new URLSearchParams(window.location.search).get('lang')
  )
  if (param) return param
  try {
    const stored = normalizeLang(
      window.localStorage.getItem(LANDING_LANG_STORAGE_KEY)
    )
    if (stored) return stored
  } catch {
    /* storage unavailable */
  }
  const candidates = navigator.languages?.length
    ? navigator.languages
    : [navigator.language]
  for (const tag of candidates) {
    const resolved = normalizeLang(tag)
    if (resolved) return resolved
  }
  return 'zh-CN'
}

/** Homepage URL of a language; the homepage reads `?lang=` on load. */
export function landingHomeHref(lang: LandingLang): string {
  return `/?lang=${lang}`
}

type LandingLangValue = {
  lang: LandingLang
  setLang: (lang: LandingLang) => void
  copy: LandingCopy
  /** Languages the switcher offers on this page. */
  langOptions: readonly LandingLangOption[]
  /** Real URL of each offered language, for new tabs and crawlers. */
  langHref: (lang: LandingLang) => string
}

const LandingLangContext = createContext<LandingLangValue | null>(null)

export type LandingLangProviderProps = {
  children: ReactNode
  /**
   * Pins the page language. Each tool page URL is a distinct SEO page in one
   * language, so `?lang=`, storage and the browser language are ignored.
   */
  lang?: LandingLang
  /**
   * Replaces the default switch (copy + storage + `?lang=`). Pinned pages use
   * it to navigate to the URL of the chosen language instead.
   */
  onLangChange?: (lang: LandingLang) => void
  /**
   * Limits the switcher to languages this page really publishes. Defaults to
   * every landing language.
   */
  langs?: readonly LandingLang[]
  /** URL of the chosen language. Defaults to the homepage `?lang=` URL. */
  langHref?: (lang: LandingLang) => string
  /** Preloaded copy for the initial language, used by the landing entry. */
  initialCopy?: LandingCopy
  /** Initial language paired with `initialCopy`; not a pinned language. */
  initialLang?: LandingLang
}

export function LandingLangProviderBase(props: LandingLangProviderProps) {
  const [resolvedLang, setLangState] = useState<LandingLang>(
    () => props.lang ?? props.initialLang ?? resolveLandingLang()
  )
  const lang = props.lang ?? resolvedLang
  const onLangChange = props.onLangChange
  const langs = props.langs
  const langHref = props.langHref ?? landingHomeHref
  const [copyState, setCopyState] = useState<{
    lang: LandingLang
    copy: LandingCopy
  } | null>(() =>
    props.initialCopy ? { lang, copy: props.initialCopy } : null
  )
  const langOptions = useMemo(
    () =>
      langs
        ? LANDING_LANGS.filter((option) => langs.includes(option.code))
        : LANDING_LANGS,
    [langs]
  )

  useEffect(() => {
    if (copyState?.lang === lang) return
    let cancelled = false
    void loadLandingCopy(lang).then((copy) => {
      if (!cancelled) setCopyState({ lang, copy })
    })
    return () => {
      cancelled = true
    }
  }, [copyState?.lang, lang])

  const setLang = useCallback(
    (next: LandingLang) => {
      if (onLangChange) {
        onLangChange(next)
        return
      }
      setLangState(next)
      try {
        window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, next)
      } catch {
        /* storage unavailable */
      }
      try {
        const url = new URL(window.location.href)
        url.searchParams.set('lang', next)
        window.history.replaceState(null, '', url)
      } catch {
        /* non-critical */
      }
    },
    [onLangChange]
  )

  const value = useMemo<LandingLangValue | null>(
    () =>
      copyState
        ? {
            lang,
            setLang,
            copy: copyState.copy,
            langOptions,
            langHref,
          }
        : null,
    [copyState, lang, setLang, langOptions, langHref]
  )

  if (!value) return null
  return (
    <LandingLangContext.Provider value={value}>
      {props.children}
    </LandingLangContext.Provider>
  )
}

export function useLanding(): LandingLangValue {
  const value = useContext(LandingLangContext)
  if (!value) {
    throw new Error('useLanding must be used within LandingLangProvider')
  }
  return value
}

export function useCopy(): LandingCopy {
  return useLanding().copy
}
