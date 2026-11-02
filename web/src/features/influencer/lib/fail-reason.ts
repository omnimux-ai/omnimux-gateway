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

/**
 * The gateway stores `task.fail_reason` as free text: a few fixed English
 * adaptor strings, a set of fixed/parameterized Chinese gateway strings, and
 * whatever an upstream provider sent. Rendering it verbatim leaks Chinese or
 * raw provider prose into every interface language, so the failure surfaces
 * resolve it through this table first and keep the original only as a support
 * detail.
 */

type Translator = (key: string, options?: Record<string, unknown>) => string

export interface FailReasonView {
  /** Localized sentence that may be shown as the primary failure copy. */
  message: string
  /** Original server text, kept for the optional detail disclosure. */
  raw: string
  /** Whether the original text carries information beyond the message. */
  showDetail: boolean
  /** How the text was classified, so callers can prefer a keyed fallback. */
  kind: 'known' | 'unknown' | 'empty'
}

const EMPTY_KEY = 'Generation failed'

const UNKNOWN_KEY = 'Generation failed. Please try again.'

const REFUNDED_KEY =
  'Submission was not confirmed in time. Reserved credits were refunded.'

const BILLING_REVIEW_KEY =
  'Submission was not confirmed in time. Billing is under manual review.'

const NEEDS_REVIEW_KEY =
  'The submission outcome needs review. This take will not be sent again.'

const NOT_ACCEPTED_KEY =
  'The generation was not accepted, so the reserved credits were released.'

const NOT_STARTED_KEY =
  'This take could not start, so it was not sent. Please submit it again.'

/** Fixed gateway/adaptor reasons. Values are the locale keys themselves. */
const EXACT_REASONS = new Map<string, string>([
  ['Task timed out', 'Task timed out'],
  [
    'Task timed out (a task left by the old system is not refunded; contact the administrator)',
    'Task timed out (a task left by the old system is not refunded; contact the administrator)',
  ],
  [
    'Failed to get the channel information. Please contact the administrator',
    'Failed to get the channel information. Please contact the administrator',
  ],
  ['Upstream task not found', 'Upstream task not found'],
  [
    'Failed to query the upstream task status',
    'Failed to query the upstream task status',
  ],
  [
    'Upstream task timed out (over 1 hour)',
    'Upstream task timed out (over 1 hour)',
  ],
  // Preflight failures: the generation never started, nothing was sent.
  ['generation account or request is unavailable', NOT_STARTED_KEY],
  [
    'generation could not start; check account, balance and available route',
    NOT_STARTED_KEY,
  ],
  // Error sentences this feature throws. They are locale keys, so a server or
  // client message carrying one must resolve like any other known reason.
  [
    'Could not submit the task. Please try again.',
    'Could not submit the task. Please try again.',
  ],
  [
    'Could not submit the multi-view task. Please try again.',
    'Could not submit the multi-view task. Please try again.',
  ],
  [
    'Could not submit the video task. Please try again.',
    'Could not submit the video task. Please try again.',
  ],
  [
    'Could not analyze the reference video. Please try again.',
    'Could not analyze the reference video. Please try again.',
  ],
  [
    'File upload failed. Please try again.',
    'File upload failed. Please try again.',
  ],
  [
    'The upload session is invalid. Please refresh and try again.',
    'The upload session is invalid. Please refresh and try again.',
  ],
  [
    'The upload returned no response. Please try again.',
    'The upload returned no response. Please try again.',
  ],
  [
    'The upload returned no file URL. Please try again.',
    'The upload returned no file URL. Please try again.',
  ],
  ['Unknown category.', 'Unknown category.'],
  ['Unknown option.', 'Unknown option.'],
  ['Could not prepare video', 'Could not prepare video'],
  ['Request failed', 'Request failed'],
  ['Upload failed. Please try again.', 'Upload failed. Please try again.'],
  [
    'Image upload failed. Please try again.',
    'Image upload failed. Please try again.',
  ],
  ['Payment request failed', 'Payment request failed'],
  [
    'This video is unavailable. Choose another video or upload a file.',
    'This video is unavailable. Choose another video or upload a file.',
  ],
  [
    'The character image is no longer available.',
    'The character image is no longer available.',
  ],
  [
    'Prompt must be 4000 characters or fewer.',
    'Prompt must be 4000 characters or fewer.',
  ],
  ['Something went wrong!', 'Something went wrong!'],
])

/**
 * Chinese gateway families. Each pattern is anchored on purpose: an upstream
 * message that merely quotes part of a gateway sentence must not be
 * misclassified as a refund/review claim.
 */
const REASON_FAMILIES: Array<[RegExp, string]> = [
  [
    /^提交结果超过\s*\d+\s*分钟未确认，已判定失败，预留积分按任务记录退回$/,
    REFUNDED_KEY,
  ],
  [
    /^提交结果超过\s*\d+\s*分钟未确认，已判定失败；扣费待人工核对$/,
    BILLING_REVIEW_KEY,
  ],
  [
    /^(提交结果待核对|提交或资金状态待核对|提交或资金释放待核对)，不会自动重发$/,
    NEEDS_REVIEW_KEY,
  ],
  [/^生成未被接受，预留已释放$/, NOT_ACCEPTED_KEY],
  // 18 preflight literals (queue + object swap) all end with this suffix.
  [/[，,](任务|视频生成)未开始(，不会自动重复理解)?$/, NOT_STARTED_KEY],
  // Older deployed rows carry the previous English review sentence.
  [/^Submission (outcome )?needs review\b/i, NEEDS_REVIEW_KEY],
]

const NESTED_RECORD = /（原记录：([\s\S]*)）$/
const STAGE_CODE = /（阶段：([\s\S]*?)；代码：([\s\S]*?)）$/

/** Strip the wrappers the gateway appends, keeping them as support detail. */
function stripWrappers(text: string): { base: string; detail: string } {
  let base = text
  const details: string[] = []
  let match = base.match(NESTED_RECORD)
  if (match) {
    details.push(match[1].trim())
    base = base.slice(0, match.index).trim()
  }
  match = base.match(STAGE_CODE)
  if (match) {
    details.push(match[1].trim(), match[2].trim())
    base = base.slice(0, match.index).trim()
  }
  return { base, detail: details.filter(Boolean).join(' · ') }
}

/**
 * Resolve a stored failure reason into a localized sentence.
 *
 * Never returns upstream text as `message`: unknown prose falls back to the
 * generic failure copy with `showDetail` set, so the original stays available
 * without becoming the primary message in a language it was not written in.
 */
export function describeFailReason(
  t: Translator,
  raw?: string | null
): FailReasonView {
  const text = (raw ?? '').trim()
  if (!text) {
    return {
      message: t(EMPTY_KEY),
      raw: '',
      showDetail: false,
      kind: 'empty',
    }
  }

  const { base, detail } = stripWrappers(text)
  const exact = EXACT_REASONS.get(base)
  if (exact) {
    return {
      message: t(exact),
      raw: text,
      showDetail: detail.length > 0,
      kind: 'known',
    }
  }

  for (const [pattern, key] of REASON_FAMILIES) {
    if (pattern.test(base)) {
      return {
        message: t(key),
        raw: text,
        // These families carry facts the localized sentence drops (the
        // confirmation window, the nested original record), so the original
        // text stays reachable.
        showDetail: true,
        kind: 'known',
      }
    }
  }

  return {
    message: t(UNKNOWN_KEY),
    raw: text,
    showDetail: true,
    kind: 'unknown',
  }
}
