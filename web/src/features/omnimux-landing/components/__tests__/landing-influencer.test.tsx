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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { describe, test } from 'vitest'
import { fileURLToPath } from 'node:url'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

type TranslationMap = Record<string, string | string[]>
const enMap = en.translation as unknown as TranslationMap
const zhMap = zh.translation as unknown as TranslationMap

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const featureDir = path.resolve(__dirname, '..', '..')

const readSrc = (rel: string) =>
  readFileSync(path.resolve(featureDir, rel), 'utf-8')

const COPY_KEYS = Object.keys(en.translation).filter((k) =>
  k.startsWith('influencer_landing.')
)

describe('Influencer landing copy contracts', () => {
  test('every influencer_landing.* key exists in en and zh, and zh differs from en where it should', () => {
    assert.ok(COPY_KEYS.length > 80, 'expected ~126 influencer_landing keys')
    
    for (const key of COPY_KEYS) {
      const enValue = enMap[key]
      const zhValue = zhMap[key]
      assert.ok(zhValue, `zh missing ${key}`)
      if (!/titleTail|aesthetic\.y2k/.test(key)) {
        assert.notEqual(
          zhValue,
          enValue,
          `zh ${key} still untranslated English`
        )
      }
    }
  })

  test('billing copy sells top-ups only, with no residual subscription claim', () => {
    assert.equal(enMap['influencer_landing.hero.perk.payPerImage'], 'Pay per use, volume discounts')
    assert.equal(zhMap['influencer_landing.hero.perk.payPerImage'], '按次使用，充得多更便宜')
    for (const map of [enMap, zhMap]) {
      const answer = String(map['influencer_landing.faq.a2'])
      assert.match(answer, /top up|充值/i)
      assert.match(answer, /wallet|钱包/)
      // 套餐已下线：不得再出现订阅/套餐分支。
      assert.doesNotMatch(answer, /subscription|订阅|套餐|plan/i)
      assert.doesNotMatch(answer, /no subscription|shown before|生成前.*价格/i)
    }
  })

  test('video workflow describes references and download, not permanent editing or publishing', () => {
    assert.equal(enMap['influencer_landing.step4.title'], 'Generate & download')
    assert.equal(zhMap['influencer_landing.step4.title'], '生成并下载视频')
    assert.equal(enMap['influencer_landing.mock.renderReady'], 'Video result preview')
    assert.equal(enMap['influencer_landing.creator.submit'], 'Open studio')
    for (const suffix of ['step1.desc', 'step2.desc', 'step3.desc', 'step4.desc', 'final.sub', 'multiview.sub', 'multiview.point3', 'pricing.feature.character', 'faq.a5']) {
      const value = String(enMap[`influencer_landing.${suffix}`])
      assert.doesNotMatch(value, /every video|rollback|edit your|any (clip|result|finished)|exact moves|post instantly|UGC team|permanent/i)
    }
    assert.match(String(enMap['influencer_landing.creator.photoHint']), /studio/)
    assert.match(String(enMap['influencer_landing.step3.desc']), /rights/)
    assert.match(String(enMap['influencer_landing.multiview.sub']), /your own/)
  })

  test('all seven locales provide effective nonempty translated landing text', () => {
    const localeDir = path.resolve(featureDir, '..', '..', 'i18n', 'locales')
    for (const locale of ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']) {
      const map = JSON.parse(readFileSync(path.join(localeDir, `${locale}.json`), 'utf-8')).translation as TranslationMap
      const pricingKeys = [
        'Check prices and wallet allowances',
        'Review generation history',
        'Failed tasks return generation credits',
        'Top up and pay per generation. This catalog lists {{count}} available models; prices and allowances follow your wallet settings.',
        'Top up and pay per generation. Check current prices and allowances in your wallet.',
      ]
      const guideKeys = [
        'Guides',
        'Read the character guide',
        'Read the reference video guide',
        'App links follow your interface language. Guide articles are in English.',
      ]
      for (const key of [...COPY_KEYS, ...pricingKeys, ...guideKeys]) {
        const value = map[key]
        assert.equal(typeof value, 'string', `${locale} missing effective text: ${key}`)
        if (!key.endsWith('hero.titleTail')) assert.ok(String(value).trim(), `${locale} empty ${key}`)
        if (
          (locale !== 'en' || key.startsWith('influencer_landing.')) &&
          !(locale === 'fr' && key === 'Guides')
        ) {
          assert.notEqual(value, key, `${locale} unresolved ${key}`)
        }
        if (key.includes('{{count}}')) assert.ok(String(value).includes('{{count}}'))
      }
      if (locale !== 'en') {
        for (const suffix of ['step4.title', 'creator.submit', 'faq.a2']) {
          const key = `influencer_landing.${suffix}`
          assert.notEqual(map[key], enMap[key], `${locale} untranslated ${key}`)
        }
      }
    }
  })

  test('title fragments form a complete sentence in both locales', () => {
    const enText =
      String(enMap['influencer_landing.hero.titleLead']) +
      String(enMap['influencer_landing.hero.titleAccent']) +
      String(enMap['influencer_landing.hero.titleTail'])
    const zhText =
      String(zhMap['influencer_landing.hero.titleLead']) +
      String(zhMap['influencer_landing.hero.titleAccent']) +
      String(zhMap['influencer_landing.hero.titleTail'])
    assert.equal(enText, 'Put your character in any trend')
    assert.equal(zhText, '让你的角色成为热门视频的主角')
    assert.equal(enMap['influencer_landing.hero.subline'], 'Create a character Pick a trend Generate the video Same face every video')
    assert.equal(zhMap['influencer_landing.hero.subline'], '创建角色 挑选热门玩法 生成专属视频 玩法不断换 角色始终如一')
    assert.doesNotMatch(enText + zhText, /[.,。，、]/)
    assert.ok(zhText.includes('角色'))
  })

  test('the USD line never adds a second currency symbol', () => {
    // Intl renders the currency symbol itself, so a literal one in the
    // template would print "$$0.00147" on every locale.
    for (const map of [enMap, zhMap]) {
      const usd = String(map['influencer_landing.pricing.usd'])
      assert.equal(
        (usd.match(/\$/g) || []).length,
        0,
        `pricing.usd must not hardcode a currency symbol: ${usd}`
      )
      assert.ok(usd.includes('{{usd}}'), 'pricing.usd must interpolate the amount')
    }
  })

  test('every CTA the page renders points at /influencer or a real anchor, never a dead one', () => {
    const files = [
      'components/influencer/hero.tsx',
      'components/influencer/works.tsx',
      'components/influencer/pricing-faq-final.tsx',
      'components/influencer/creator.tsx',
    ]
    for (const rel of files) {
      const src = readSrc(rel)
      assert.ok(!src.includes('pluginMarket'), `${rel} still links a dead anchor`)
      assert.ok(src.includes('/influencer') || src.includes('useNavigate'), `${rel} has no way forward`)
    }
  })

  test('hero equation shows three real media tiles and falls back to posters without motion', () => {
    const src = readSrc('components/influencer/hero.tsx')
    for (const key of ['card.character', 'card.anyVideo', 'card.yourVideo']) {
      assert.ok(src.includes(key), `hero must label the ${key} tile`)
    }
    assert.ok(
      src.includes('prefersReducedMotion'),
      'autoplaying videos must degrade to a poster still'
    )
    assert.ok(
      src.includes('playsInline') && src.includes('muted'),
      'hero autoplay must stay silent and inline'
    )
    assert.ok(
      src.includes('useVideoSync') && src.includes('playbackRate'),
      'the two hero clips must share one timeline instead of looping apart'
    )
    const looks = readSrc('lib/influencer-looks.ts')
    for (const asset of [
      'hero-character.webp',
      'hero-any-video.mp4',
      'hero-any-video-poster.webp',
      'hero-your-video.mp4',
      'hero-your-video-poster.webp',
    ]) {
      assert.ok(looks.includes(asset), `hero media should reference ${asset}`)
    }
  })

  test('creator writes a guest draft only when there is something to carry over', () => {
    const src = readSrc('components/influencer/creator.tsx')
    assert.ok(src.includes('writeGuestDraft'), 'creator must hand picks to the workbench draft')
    assert.ok(src.includes('!canSubmit'), 'empty composer must stay disabled')
    assert.ok(!src.includes('localStorage'), 'draft must use sessionStorage, never localStorage')
  })

  test('works wall carries each look real traits into the same draft mechanism', () => {
    const src = readSrc('components/influencer/works.tsx')
    assert.ok(src.includes('draftFromLook'), 'work cards must reuse the same draft shape')
    const looks = readSrc('lib/influencer-looks.ts')
    for (const id of ['goth', 'sporty', 'y2k', 'retro']) {
      assert.ok(looks.includes(id), `looks data should keep the ${id} category`)
    }
  })

  test('no fake social proof, no unshipped promises leaked into copy', () => {
    const forbidden = [
      /videos made/i,
      /users? (love|say|posted)/i,
      /trend video/i,
      /whole squad/i,
      /group trends/i,
    ]
    for (const key of COPY_KEYS) {
      const enValue = enMap[key]
      for (const re of forbidden) {
        assert.ok(!re.test(String(enValue)), `${key} smuggles marketing fluff: ${enValue}`)
      }
    }
  })

  test('pricing CTA routes to the wallet top-up surface by auth state', () => {
    const src = readSrc('components/influencer/pricing-faq-final.tsx')
    assert.ok(
      src.includes('useAuthStore'),
      'pricing section must observe auth state to determine sign-in vs wallet routing'
    )
    assert.ok(
      src.includes('/wallet#wallet-add-funds'),
      'signed-in visitors must land on the wallet top-up form'
    )
    assert.ok(
      src.includes('/sign-in?redirect='),
      'unauthenticated visitors must receive a return redirect to the wallet'
    )
    assert.ok(
      !src.includes('/wallet?plan='),
      'subscription checkout routing must be gone now that plans are disabled'
    )
  })
})
