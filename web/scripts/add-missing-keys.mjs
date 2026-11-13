import fs from 'node:fs/promises'
import path from 'node:path'

const locales = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']
// Optional JSON input: { en: {key: value}, zh: {...}, ... }.
// Run from web/, then bun run i18n:sync to normalize all locales.
const source = process.argv[2]
const newKeys = source
  ? JSON.parse(await fs.readFile(path.resolve(source), 'utf8'))
  : Object.fromEntries(locales.map((locale) => [locale, {}]))
// Existing OIDC labels and regression assertions preserve this technical term.
newKeys.zh['Well-Known URL must start with http:// or https://'] =
  'Well-Known URL 必须以 http:// 或 https:// 开头'
newKeys['zh-TW']['Well-Known URL must start with http:// or https://'] =
  'Well-Known URL 必須以 http:// 或 https:// 開頭'
// Refund copy describes generation credits and defers to the task record.
const landingRefundCopy = {
  en: [
    'Generation credit refunds follow task records',
    'For failed tasks, generation credit refunds follow the task record. This does not mean a cash refund.',
  ],
  zh: [
    '生成积分退回以任务记录为准',
    '任务失败后的生成积分退回以任务记录为准，不代表现金退款。',
  ],
  'zh-TW': [
    '生成積分退回以任務記錄為準',
    '任務失敗後的生成積分退回以任務記錄為準，不代表現金退款。',
  ],
  fr: [
    'Crédits de génération : remboursement selon le suivi des tâches',
    'Pour une tâche échouée, le remboursement des crédits de génération dépend du suivi de la tâche. Il ne s’agit pas d’un remboursement en argent.',
  ],
  ja: [
    '生成クレジットの返還はタスク記録に準じます',
    '失敗したタスクの生成クレジット返還は、タスク記録に準じます。現金の返金を意味するものではありません。',
  ],
  ru: [
    'Возврат кредитов генерации — по записям задач',
    'При сбое задачи возврат кредитов генерации определяется записью задачи. Это не означает возврат денег.',
  ],
  vi: [
    'Hoàn điểm tạo nội dung theo bản ghi tác vụ',
    'Với tác vụ thất bại, việc hoàn điểm tạo nội dung dựa trên bản ghi tác vụ. Điều này không có nghĩa là hoàn tiền.',
  ],
}
for (const locale of locales) {
  const [refund, answer] = landingRefundCopy[locale]
  Object.assign(newKeys[locale], {
    'influencer_landing.hero.perk.refund': refund,
    'influencer_landing.pricing.feature.refund': refund,
    'Failed tasks return generation credits': refund,
    'influencer_landing.faq.a3': answer,
  })
}
const updateKeys = new Set([
  'Well-Known URL must start with http:// or https://',
  // Workbench labels that shipped with the English key as the zh/zh-TW value.
  'Regenerate',
  'No preview available',
  'This task does not carry sheet parameters',
  'Form filled from task',
  // The inspiration preset action: English on every locale until translated.
  'Recreate',
  // The influencer landing hero and its USD line: Intl already renders the
  // currency symbol, so the template must not add a second one.
  'influencer_landing.hero.titleLead',
  'influencer_landing.hero.titleAccent',
  'influencer_landing.hero.titleTail',
  'influencer_landing.hero.subline',
  'influencer_landing.pricing.usd',
  // The pricing section now sells subscription plans, so its heading copy
  // must not keep describing pure pay-per-image pricing.
  'influencer_landing.pricing.title',
  'influencer_landing.pricing.sub',
  'influencer_landing.pricing.note',
  // A plan credit is platform quota, not one generation: say "credits".
  'influencer_landing.pricing.credits',
  // The landing card no longer asks for a style, so the subline must not
  // promise one.
  'influencer_landing.creator.sub',
  // Landing capability copy: exact keys only, never a namespace-wide update.
  'influencer_landing.hero.perk.payPerImage',
  'influencer_landing.faq.a2',
  'influencer_landing.faq.a5',
  'influencer_landing.final.sub',
  'influencer_landing.step1.desc',
  'influencer_landing.step2.title',
  'influencer_landing.step2.desc',
  'influencer_landing.step3.desc',
  'influencer_landing.step4.title',
  'influencer_landing.step4.desc',
  'influencer_landing.steps.sub',
  'influencer_landing.mock.avatarConsistent',
  'influencer_landing.mock.renderReady',
  'influencer_landing.mock.publishBtn',
  'influencer_landing.mock.currentLocked',
  'influencer_landing.creator.photoHint',
  'influencer_landing.creator.submit',
  'influencer_landing.multiview.sub',
  'influencer_landing.multiview.point3',
  'influencer_landing.pricing.feature.character',
  'influencer_landing.creator.tabPhoto',
  'influencer_landing.creator.note',
  'influencer_landing.faq.a4',
  'influencer_landing.mock.tabPhoto',
  'influencer_landing.pricing.feature.noSubscription',
  'influencer_landing.hero.perk.refund',
  'influencer_landing.pricing.feature.refund',
  'Failed tasks return generation credits',
  'influencer_landing.faq.a3',
  'influencer_landing.mock.chip1',
  'influencer_landing.mock.chip3',
])
const applied = []
for (const locale of locales) {
  const file = path.resolve(`src/i18n/locales/${locale}.json`)
  const json = JSON.parse(await fs.readFile(file, 'utf8'))
  const inserted = []
  for (const [key, value] of Object.entries(newKeys[locale])) {
    if (
      !(key in json.translation) ||
      ((source || updateKeys.has(key)) && json.translation[key] !== value)
    ) {
      json.translation[key] = value
      inserted.push(key)
    }
  }
  const translation = Object.fromEntries(
    Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
  )
  // These root duplicates are already preserved by reachable fork history and
  // are never loaded by namespaceOf. Keep the effective namespace unchanged.
  await fs.writeFile(file, `${JSON.stringify({ translation }, null, 2)}\n`)
  applied.push({ locale, inserted, count: inserted.length })
}
console.log(JSON.stringify(applied, null, 2))
