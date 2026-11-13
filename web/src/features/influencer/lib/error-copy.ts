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
import i18next from 'i18next'
import { toast } from 'sonner'

import {
  getServerErrorSources,
  getServerErrorMessage,
  translateServerText,
} from '@/lib/server-error-message'

import { describeFailReason } from './fail-reason'

type Translator = (key: string, options?: Record<string, unknown>) => string

/** Han characters carry the gateway's untranslated Chinese reasons. */
const HAN_TEXT = /[\u3400-\u4dbf\u4e00-\u9fff]/
/** Kana/Hangul mean the text is already a Japanese/Korean translation. */
const KANA_OR_HANGUL = /[\u3040-\u30ff\uac00-\ud7af]/

function interfaceIsChinese(): boolean {
  const language = i18next.resolvedLanguage ?? i18next.language ?? 'en'
  return language.toLowerCase().startsWith('zh')
}

/**
 * The backend localizes web-console messages itself when the payload carries a
 * `message_key`; such text is already in the interface language and must not be
 * mistaken for an untranslated Chinese reason.
 */
function carriesMessageKey(error: unknown): boolean {
  return getServerErrorSources(error).some(
    (source) => typeof source.message_key === 'string' && source.message_key
  )
}

/** The message a request failure carries, before any key translation. */
function rawErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  return getServerErrorMessage(error)
}

/**
 * Localize whatever a failed request carried.
 *
 * Known gateway reasons get their reason-group sentence; anything else that is
 * still Chinese is replaced by the caller's localized fallback, because a
 * consumer should never be handed a Chinese server string. English technical
 * text is kept as-is: it is the only hint support has, and the caller usually
 * shows a localized prefix or fallback around it.
 */
export function influencerErrorText(
  t: Translator,
  error: unknown,
  fallbackKey: string
): string {
  const raw = rawErrorMessage(error)
  if (!raw) return t(fallbackKey)

  const view = describeFailReason(t, raw)
  if (view.kind === 'known') return view.message
  if (view.kind === 'empty') return t(fallbackKey)

  const translated = translateServerText(t, raw)
  if (translated !== raw) return translated

  if (
    !interfaceIsChinese() &&
    !carriesMessageKey(error) &&
    HAN_TEXT.test(raw) &&
    !KANA_OR_HANGUL.test(raw)
  ) {
    return t(fallbackKey)
  }
  return raw
}

/** Toast a failed influencer request without leaking untranslated Chinese. */
export function toastInfluencerError(
  t: Translator,
  error: unknown,
  fallbackKey: string
): void {
  toast.error(influencerErrorText(t, error, fallbackKey))
}

interface SubmitFailureCopy {
  /** Localized template when the payload is not a translatable sentence. */
  prefixKey: string
  /** Localized sentence when the failure carries no usable text at all. */
  fallbackKey: string
}

/**
 * Copy for "the operation failed: <reason>" toasts.
 *
 * A recognized reason replaces the template with one complete sentence, so a
 * user never reads a localized prefix followed by untranslated text. Only an
 * unknown non-Chinese payload keeps the template around the original text.
 */
export function submitFailureText(
  t: Translator,
  error: unknown,
  copy: SubmitFailureCopy
): string {
  const raw = rawErrorMessage(error)
  if (!raw) return t(copy.fallbackKey)

  const view = describeFailReason(t, raw)
  if (view.kind === 'known') return view.message

  const translated = translateServerText(t, raw)
  if (translated !== raw) return translated

  if (
    !interfaceIsChinese() &&
    !carriesMessageKey(error) &&
    HAN_TEXT.test(raw) &&
    !KANA_OR_HANGUL.test(raw)
  ) {
    return t(copy.fallbackKey)
  }
  return t(copy.prefixKey, { msg: raw })
}
