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
  useMemo,
  useState,
  type ReactNode,
} from 'react'

import { en } from './locales/en'
import { es } from './locales/es'
import { id } from './locales/id'
import { ja } from './locales/ja'
import { zhCN } from './locales/zh-CN'
import { zhTW } from './locales/zh-TW'
import type {
  LandingCopy,
  LandingLang,
  LandingLangOption,
} from './types'

export const LANDING_LANGS: readonly LandingLangOption[] = [
  { code: 'en', label: 'English', short: 'EN', htmlLang: 'en' },
  { code: 'id', label: 'Bahasa Indonesia', short: 'ID', htmlLang: 'id' },
  { code: 'es', label: 'Español', short: 'ES', htmlLang: 'es' },
  { code: 'ja', label: '日本語', short: '日', htmlLang: 'ja' },
  { code: 'zh-TW', label: '繁體中文', short: '繁', htmlLang: 'zh-Hant' },
  { code: 'zh-CN', label: '简体中文', short: '中', htmlLang: 'zh-CN' },
]

export const LANDING_COPIES: Record<LandingLang, LandingCopy> = {
  en,
  id,
  es,
  ja,
  'zh-TW': zhTW,
  'zh-CN': zhCN,
}

export const LANDING_LANG_STORAGE_KEY = 'omnimux.d.lang'

function normalizeLang(value: string | null | undefined): LandingLang | null {
  if (!value) return null
  const v = value.trim().toLowerCase()
  if (!v) return null
  if (v === 'en' || v.startsWith('en-')) return 'en'
  if (v === 'id' || v === 'in' || v.startsWith('id-') || v.startsWith('in-'))
    {return 'id'}
  if (v === 'es' || v.startsWith('es-')) return 'es'
  if (v === 'ja' || v === 'jp' || v.startsWith('ja-') || v.startsWith('jp-'))
    {return 'ja'}
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
    )
      {return 'zh-TW'}
    return 'zh-CN'
  }
  return null
}

export function landingLangOption(
  lang: LandingLang
): LandingLangOption {
  return LANDING_LANGS.find((option) => option.code === lang) ?? LANDING_LANGS[5]
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

type LandingLangValue = {
  lang: LandingLang
  setLang: (lang: LandingLang) => void
  copy: LandingCopy
}

const LandingLangContext = createContext<LandingLangValue | null>(null)

export function LandingLangProvider(props: {
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
}) {
  const [resolvedLang, setLangState] = useState<LandingLang>(
    () => props.lang ?? resolveLandingLang()
  )
  const lang = props.lang ?? resolvedLang
  const onLangChange = props.onLangChange

  const setLang = useCallback((next: LandingLang) => {
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
  }, [onLangChange])

  const value = useMemo<LandingLangValue>(
    () => ({ lang, setLang, copy: LANDING_COPIES[lang] }),
    [lang, setLang]
  )

  return (
    <LandingLangContext.Provider value={value}>
      {props.children}
    </LandingLangContext.Provider>
  )
}

const FALLBACK: LandingLangValue = {
  lang: 'zh-CN',
  setLang: () => {},
  copy: zhCN,
}

export function useLanding(): LandingLangValue {
  return useContext(LandingLangContext) ?? FALLBACK
}

export function useCopy(): LandingCopy {
  return useLanding().copy
}
