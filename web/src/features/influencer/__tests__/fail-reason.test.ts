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
import { describe, expect, test } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'

import { describeFailReason } from '../lib/fail-reason'

type TranslationTable = Record<string, unknown>

const LOCALES: Record<string, TranslationTable> = {
  en: en.translation,
  zh: zh.translation,
  'zh-TW': zhTW.translation,
  fr: fr.translation,
  ja: ja.translation,
  ru: ru.translation,
  vi: vi.translation,
}

const CHINESE = new Set(['zh', 'zh-TW'])
const HAN = /[\u3400-\u4dbf\u4e00-\u9fff]/
const KANA = /[\u3040-\u30ff]/

const REFUNDED =
  'Submission was not confirmed in time. Reserved credits were refunded.'
const BILLING_REVIEW =
  'Submission was not confirmed in time. Billing is under manual review.'
const NEEDS_REVIEW =
  'The submission outcome needs review. This take will not be sent again.'
const NOT_ACCEPTED =
  'The generation was not accepted, so the reserved credits were released.'
const NOT_STARTED =
  'This take could not start, so it was not sent. Please submit it again.'
const UNKNOWN = 'Generation failed. Please try again.'

function value(locale: string, key: string): string {
  const found = LOCALES[locale][key]
  return typeof found === 'string' ? found : key
}

function translator(locale: string) {
  return (key: string): string => value(locale, key)
}

interface ReasonCase {
  name: string
  raw: string
  expectedKey: string
  showDetail: boolean
  kind: 'known' | 'unknown' | 'empty'
}

const CASES: ReasonCase[] = [
  {
    name: 'fixed adaptor timeout',
    raw: 'Task timed out',
    expectedKey: 'Task timed out',
    showDetail: false,
    kind: 'known',
  },
  {
    name: 'legacy-system timeout',
    raw: 'Task timed out (a task left by the old system is not refunded; contact the administrator)',
    expectedKey:
      'Task timed out (a task left by the old system is not refunded; contact the administrator)',
    showDetail: false,
    kind: 'known',
  },
  {
    name: 'missing channel information',
    raw: 'Failed to get the channel information. Please contact the administrator',
    expectedKey:
      'Failed to get the channel information. Please contact the administrator',
    showDetail: false,
    kind: 'known',
  },
  {
    name: 'upstream task missing',
    raw: 'Upstream task not found',
    expectedKey: 'Upstream task not found',
    showDetail: false,
    kind: 'known',
  },
  {
    name: 'gateway review marker',
    raw: '提交结果待核对，不会自动重发',
    expectedKey: NEEDS_REVIEW,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'gateway review marker with stage and code',
    raw: '提交结果待核对，不会自动重发（阶段：submit；代码：submission_failed）',
    expectedKey: NEEDS_REVIEW,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'expired with refunded reservation',
    raw: '提交结果超过 60 分钟未确认，已判定失败，预留积分按任务记录退回',
    expectedKey: REFUNDED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'expired with nested original reason',
    raw: '提交结果超过 60 分钟未确认，已判定失败，预留积分按任务记录退回（原记录：生成未被接受，预留已释放）',
    expectedKey: REFUNDED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'expired with billing review',
    raw: '提交结果超过 30 分钟未确认，已判定失败；扣费待人工核对',
    expectedKey: BILLING_REVIEW,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'not accepted, reservation released',
    raw: '生成未被接受，预留已释放',
    expectedKey: NOT_ACCEPTED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'queue preflight suffix',
    raw: '参考素材不可用或已失效，任务未开始',
    expectedKey: NOT_STARTED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'object-swap preflight suffix',
    raw: '对象替换信息不可读取，视频生成未开始',
    expectedKey: NOT_STARTED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'object-swap preflight with trailing advice',
    raw: '参考素材与要求无法形成合理的对象替换，视频生成未开始，不会自动重复理解',
    expectedKey: NOT_STARTED,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'english preflight sentence',
    raw: 'generation could not start; check account, balance and available route',
    expectedKey: NOT_STARTED,
    showDetail: false,
    kind: 'known',
  },
  {
    name: 'legacy english review sentence',
    raw: 'Submission needs review; do not resubmit.',
    expectedKey: NEEDS_REVIEW,
    showDetail: true,
    kind: 'known',
  },
  {
    name: 'unbounded upstream prose (the reported screenshot)',
    raw: '这次是上游资源临时忙不过来,任务没排上(若已扣费会自动退回)。歇口气,过几分钟再提交,通常就排上了。好东西不怕等,我们随时接单!',
    expectedKey: UNKNOWN,
    showDetail: true,
    kind: 'unknown',
  },
  {
    name: 'empty reason',
    raw: '   ',
    expectedKey: 'Generation failed',
    showDetail: false,
    kind: 'empty',
  },
]

describe('describeFailReason', () => {
  test.each(CASES)(
    'classifies $name',
    ({ raw, expectedKey, showDetail, kind }) => {
      const view = describeFailReason(translator('en'), raw)
      expect(view.message).toBe(expectedKey)
      expect(view.showDetail).toBe(showDetail)
      expect(view.kind).toBe(kind)
    }
  )

  test('never renders upstream Chinese text as the primary message', () => {
    const upstream = CASES.find((item) => item.kind === 'unknown')
    if (!upstream) throw new Error('missing unknown-reason fixture')
    for (const locale of Object.keys(LOCALES)) {
      const view = describeFailReason(translator(locale), upstream.raw)
      expect(view.message).toBe(value(locale, UNKNOWN))
    }
  })

  test('localizes every known reason in all seven interface languages', () => {
    for (const locale of Object.keys(LOCALES)) {
      const t = translator(locale)
      for (const item of CASES.filter((entry) => entry.kind !== 'empty')) {
        const view = describeFailReason(t, item.raw)
        const translated = value(locale, item.expectedKey)
        expect(view.message).toBe(translated)
        if (locale !== 'en' && !CHINESE.has(locale) && item.kind === 'known') {
          // No locale may fall back to the stored Chinese gateway sentence.
          expect(view.message).not.toBe(item.raw)
          if (locale === 'ja') {
            // Japanese sentences carry kana; a Han-only string means the
            // Chinese reason leaked into that locale.
            expect(view.message).toMatch(KANA)
          } else {
            expect(view.message).not.toMatch(HAN)
          }
        }
      }
    }
  })

  test('keeps the raw text for support but never as the message', () => {
    const raw =
      '提交结果待核对，不会自动重发（阶段：submit；代码：submission_failed）'
    const view = describeFailReason(translator('ru'), raw)

    expect(view.raw).toBe(raw)
    expect(view.showDetail).toBe(true)
    expect(view.message).not.toContain('提交结果')
  })
})
