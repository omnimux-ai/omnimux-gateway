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
import type { LegalDocumentContent } from '../types'
import { refundTranslations } from './translations-refunds'
import { legalRussian } from './translations-ru'

export const refundsContent: LegalDocumentContent = {
  title: {
    en: 'Refund Policy',
    zh: '退款政策',
    fr: refundTranslations.fr.title,
    ja: refundTranslations.ja.title,
    vi: refundTranslations.vi.title,
    zhTW: refundTranslations.zhTW.title,
    ru: legalRussian['refunds'].title,
  },
  updated: '2026-10-08',
  body: {
    fr: refundTranslations.fr.body,
    ja: refundTranslations.ja.body,
    vi: refundTranslations.vi.body,
    zhTW: refundTranslations.zhTW.body,
    ru: legalRussian['refunds'].body,
    en: `Every generation on OmniMux runs on real GPUs and costs real money. That is why there is no free plan, and why this policy sets out exactly when credits come back to you.

## 1. Credits

- Generations are paid for with credits. The price of every model and every setting is shown on the pricing page before you generate, so you always know the cost before you commit.
- Credits have no cash value. They cannot be transferred, sold, exchanged for money, or moved to another account.
- Once credits have been spent on a generation they are not refundable, except in the cases described in this policy.
- Credits added to your account by a subscription stay available for you to use, as described in section 4.

## 2. Automatic refunds for failed generations

- When a generation fails, times out, or is blocked by our systems for a technical reason, the credits for that generation are refunded to your balance automatically.
- You do not need to contact us, open a ticket, or do anything else to receive this refund.
- These refunds cover technical failures: the request did not complete, the model did not return a usable result, or our systems stopped the request.
- They do not cover a result that was produced successfully and simply did not match your expectations. That case is described in section 3.

## 3. Broken results

If a generation finishes but the result is clearly broken — for example a character that is unrecognisable, or a video that is distorted, corrupt or unusable — write to hello@omnimux.ai with a link to the result and a short description of what is wrong. We review every report, and when the result is indeed broken the credits for that generation are refunded.

A result that opens and plays but that you simply do not like is not broken. A different framing, a different pose, a different style, or an output that does not match what you imagined is a normal part of generative work, not a technical fault, and it is not refundable.

Please report a broken result as soon as you notice it, while the generation is still easy for us to look up.

## 4. Subscriptions

- Subscription plans add credits to your balance at the start of every billing period.
- Plans renew automatically until you cancel them.
- You can cancel at any time from your account. Cancelling stops future renewals.
- After you cancel, the plan stays active until the end of the billing period you have already paid for, and you keep the access and the credits for that period.
- We do not refund partial periods. If you cancel in the middle of a period, the current period is not refunded, in whole or in part.
- Credits that were already added to your balance stay usable after you cancel, for as long as your account remains open.

## 5. Right of withdrawal (EU/UK customers)

If you are a consumer in the European Union or the United Kingdom, you have a statutory right to withdraw from a distance contract within 14 days of the purchase, without giving a reason. This section describes that statutory framework; it does not reduce any right you have under the law that applies to you.

- If you have bought a subscription or a credit purchase and have not yet used any of the credits, you can withdraw within 14 days and receive a refund of the amount you paid.
- That right is lost once performance begins with your consent. When you start a generation, you ask us to begin supplying digital content immediately, and you acknowledge that the right of withdrawal no longer applies to the credits you have used.
- In practice this means that credits already spent on generations are not refundable under this right, while a purchase you have not used can still be refunded inside the 14-day window.
- To exercise this right, write to hello@omnimux.ai from the address on your account within the 14-day window.

## 6. Other cases

- If you were charged twice for the same purchase, if a payment did not go through correctly, or if you believe your balance is wrong, write to hello@omnimux.ai with the details and we will look into it.
- If something is not covered by this policy and you think it should be, write to us anyway. We reply as quickly as we can.
- This policy works together with our [Terms of Service](/terms), our [Privacy Policy](/privacy) and our [Acceptable Use Policy](/acceptable-use). Nothing in this policy limits any right you have under mandatory consumer law.
`,
    zh: `OmniMux 上的每一次生成都跑在真实 GPU 上，都会产生真实成本。这也是我们没有免费套餐的原因，本政策将说明积分在什么情况下会退回给您。

## 1. 积分

- 生成使用积分付费。每个模型、每项设置的价格都会在您生成之前显示在定价页面上，因此您在付费前始终清楚费用。
- 积分不具备现金价值，不可转让、出售、兑换现金，也不可转移到其他账号。
- 积分一旦用于生成即不退款，本政策另有说明的情形除外。
- 由订阅套餐发放到您账号中的积分可继续使用，具体见第 4 节。

## 2. 生成失败自动退款

- 当生成失败、超时，或因技术原因被我们的系统拦截时，该次生成所消耗的积分会自动退回您的余额。
- 您无需联系我们、提交工单或做任何其他操作即可获得这笔退款。
- 此类退款针对技术故障：请求未能完成、模型未返回可用结果，或我们的系统中断了该请求。
- 如果结果已成功产出，只是不符合您的预期，则不属于此类情形。该情形见第 3 节。

## 3. 结果明显损坏

如果生成已完成，但结果明显损坏——例如角色无法辨认，或视频扭曲、损坏、不可用——请写信至 hello@omnimux.ai，附上该结果的链接并简要说明问题所在。我们会核查每一份反馈；确认结果确实损坏后，该次生成所消耗的积分将退回。

文件能打开、能播放，只是您不喜欢，不属于损坏。构图不同、姿势不同、风格不同，或产出与您的想象不符，都是生成式创作的正常现象，而非技术故障，不予退款。

请在发现结果损坏后尽快反馈，以便我们更容易查到该次生成记录。

## 4. 订阅套餐

- 订阅套餐在每个计费周期开始时向您的余额发放积分。
- 套餐会自动续费，直至您取消。
- 您可随时在账号中取消。取消后不再续费。
- 取消后，套餐在您已付费的当前计费周期结束前保持有效，该周期的使用权限与积分仍归您所有。
- 我们不按比例退还周期费用。若您在周期中途取消，当前周期的费用不予退还，无论全部还是部分。
- 只要账号仍然开放，取消后余额中已发放的积分仍可继续使用。

## 5. 撤回权（欧盟/英国客户）

如果您是欧盟或英国的消费者，您依法享有在购买后 14 天内无需说明理由即可撤回远程合同的权利。本节说明的是该法定框架，不削减适用法律赋予您的任何权利。

- 如果您购买了订阅或积分，且尚未使用任何积分，可在 14 天内撤回，并获得您已支付金额的退款。
- 一旦经您同意开始履行，该权利即告丧失。当您发起生成时，即表示您要求我们立即开始提供数字内容，并知悉已使用的积分不再适用撤回权。
- 实际效果是：已用于生成的积分不因该权利而退款，而未使用的购买仍可在 14 天窗口内退款。
- 如需行使该权利，请在 14 天窗口内使用您账号绑定的邮箱写信至 hello@omnimux.ai。

## 6. 其他情形

- 如果同一笔购买被重复扣款、支付未能正常完成，或您认为余额有误，请写信至 hello@omnimux.ai 说明详情，我们会核查处理。
- 如果某种情形不在本政策覆盖范围内而您认为应当覆盖，也欢迎写信告诉我们。我们会尽快回复。
- 本政策与我们的[服务条款](/terms)、[隐私政策](/privacy)及[可接受使用政策](/acceptable-use)共同适用。本政策不限制强制性消费者法律赋予您的任何权利。
`,
  },
}
