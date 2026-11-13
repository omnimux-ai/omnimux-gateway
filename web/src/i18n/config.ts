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
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import {
  INTERFACE_LANGUAGE_OPTIONS,
  convertDetectedLanguage,
  toIntlLocale,
  type InterfaceLanguageCode,
} from './languages'
import en from './locales/en.json'

/**
 * A locale file holds several hundred kilobytes of keys and a visitor reads
 * exactly one of them, so only the fallback language ships inside the entry
 * chunk. Every other language is split into its own async chunk and fetched
 * when that language is actually in use, instead of every visitor downloading
 * all seven.
 */
const BUNDLED_LANGUAGE: InterfaceLanguageCode = 'en'

/** One namespace of keys, i.e. the content of a locale file's `translation`. */
type TranslationNamespace = Record<string, unknown>

type LocaleFile = { translation: TranslationNamespace }

const namespaceOf = (file: LocaleFile): TranslationNamespace => file.translation

const localeLoaders: Record<
  InterfaceLanguageCode,
  () => Promise<TranslationNamespace>
> = {
  en: () => Promise.resolve(namespaceOf(en)),
  zhCN: () => import('./locales/zh.json').then((mod) => namespaceOf(mod.default)),
  fr: () => import('./locales/fr.json').then((mod) => namespaceOf(mod.default)),
  ru: () => import('./locales/ru.json').then((mod) => namespaceOf(mod.default)),
  ja: () => import('./locales/ja.json').then((mod) => namespaceOf(mod.default)),
  vi: () => import('./locales/vi.json').then((mod) => namespaceOf(mod.default)),
  zhTW: () =>
    import('./locales/zh-TW.json').then((mod) => namespaceOf(mod.default)),
}

/**
 * Resolve a detected language onto one of our interface codes. Browsers (and
 * i18next's detection) report regional tags such as `en-US` or `zh-CN`, so match
 * the exact code first and then the base subtag.
 */
function resolveInterfaceLanguage(
  language?: string | null
): InterfaceLanguageCode | undefined {
  if (!language) return undefined

  // Browsers report `zh-CN` / `zh-TW`; map them onto our `zhCN` / `zhTW` codes
  // first, since the base-subtag fallback below would read both as Chinese.
  const normalized = convertDetectedLanguage(language)
  const exact = INTERFACE_LANGUAGE_OPTIONS.find(
    (option) => option.code === normalized
  )
  if (exact) return exact.code

  const base = normalized.split('-')[0].toLowerCase()
  return INTERFACE_LANGUAGE_OPTIONS.find((option) =>
    option.code.toLowerCase().startsWith(base)
  )?.code
}

/** Where the language detector stores a manual choice. */
const LANGUAGE_STORAGE_KEY = 'i18nextLng'

/**
 * The language to start in, resolved before i18next initializes. Startup waits
 * for this language's chunk, so it must not depend on detector/init ordering:
 * a visitor whose preference is stored would otherwise render the fallback.
 */
function resolveInitialLanguage(): InterfaceLanguageCode {
  const stored =
    typeof window === 'undefined'
      ? undefined
      : (() => {
          try {
            return window.localStorage.getItem(LANGUAGE_STORAGE_KEY)
          } catch {
            return undefined
          }
        })()
  const browser = typeof navigator === 'undefined' ? undefined : navigator.language

  return (
    resolveInterfaceLanguage(stored) ??
    resolveInterfaceLanguage(browser) ??
    BUNDLED_LANGUAGE
  )
}

/**
 * The language this session starts in. `i18n.resolvedLanguage` cannot be used
 * for this: while the language's chunk is still missing i18next reports the
 * fallback it resolved to, which would make startup load the wrong chunk and
 * mislabel the document.
 */
const INITIAL_LANGUAGE = resolveInitialLanguage()

/**
 * Fetches the chunk for a non-bundled interface language and hands it to
 * i18next. The bundled fallback needs no work; anything else resolves to it
 * when the chunk cannot be loaded, so the app never renders without a language.
 */
export async function ensureLocaleLoaded(language?: string | null) {
  const code = resolveInterfaceLanguage(language)
  if (!code || code === BUNDLED_LANGUAGE) return
  if (i18n.hasResourceBundle(code, 'translation')) return

  try {
    i18n.addResourceBundle(code, 'translation', await localeLoaders[code]())
  } catch (error: unknown) {
    console.error(`[i18n] failed to load the "${code}" locale chunk`, error)
  }
}

function applyDocumentLang(lng?: string | null) {
  if (typeof document === 'undefined') return
  const htmlLang = toIntlLocale(lng) || 'en'
  document.documentElement.lang = htmlLang
}

/**
 * Resolves once the active interface language is available. Await it before the
 * first render: a non-English visitor must not see a flash of the fallback
 * language while the locale chunk is still in flight. A locale chunk that fails
 * to load resolves anyway, so the app always renders.
 */
export const i18nReady = i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { [BUNDLED_LANGUAGE]: en },
    lng: INITIAL_LANGUAGE,
    fallbackLng: BUNDLED_LANGUAGE,
    supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
    nonExplicitSupportedLngs: true,
    load: 'currentOnly',
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    // Locale chunks are added to the store after the language has already
    // changed, and react-i18next only re-renders on store events when asked to.
    // Without this a language switch keeps rendering the old language until an
    // unrelated re-render happens.
    react: { bindI18nStore: 'added' },
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      // First visit: follow system/browser language. After a manual switch,
      // localStorage wins so the choice sticks across landing + console.
      order: ['localStorage', 'navigator', 'htmlTag'],
      caches: ['localStorage'],
      // Browsers report `zh-CN`/`zh-TW`/`zh`; map them onto our `zhCN`/`zhTW`
      // codes (non-Chinese codes pass through for normal supportedLngs matching).
      convertDetectedLanguage,
    },
  })
  .then(async () => {
    await ensureLocaleLoaded(INITIAL_LANGUAGE)
    applyDocumentLang(INITIAL_LANGUAGE)
  })
  .catch((error: unknown) => {
    // The app still renders its fallback language when a locale chunk fails.
    console.error('[i18n] failed to load the interface language', error)
  })

i18n.on('languageChanged', (lng) => {
  applyDocumentLang(lng)
  // Switching language loads that locale's chunk; react-i18next re-renders when
  // the bundle lands, so the UI settles on the chosen language either way.
  void ensureLocaleLoaded(lng)
})

export default i18n
