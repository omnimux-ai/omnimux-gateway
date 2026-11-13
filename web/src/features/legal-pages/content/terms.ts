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
import { legalRussian } from './translations-ru'
import { termTranslations } from './translations-terms'

export const termsContent: LegalDocumentContent = {
  title: {
    en: 'Terms of Service',
    zh: '服务条款',
    fr: termTranslations.fr.title,
    ja: termTranslations.ja.title,
    vi: termTranslations.vi.title,
    zhTW: termTranslations.zhTW.title,
    ru: legalRussian['terms'].title,
  },
  updated: '2026-10-08',
  body: {
    fr: termTranslations.fr.body,
    ja: termTranslations.ja.body,
    vi: termTranslations.vi.body,
    zhTW: termTranslations.zhTW.body,
    ru: legalRussian['terms'].body,
    en: `These terms are a contract between you and OmniMux. By creating an account or using OmniMux in any way, you accept them. If you do not accept them, do not use the service.

## 1. Who we are

OmniMux is an AI UGC video workspace operated by **Twinspace Labs LLC**, a limited liability company registered in Wyoming, United States (file number 2025-001824581), with its principal office at 447 Broadway, 2nd Floor Suite 3272, New York, NY 10013, USA. In these terms, "we", "us" and "OmniMux" mean Twinspace Labs LLC, and "you" means the person who holds the account.

You can reach us at hello@omnimux.ai for any question about these terms.

## 2. The service

- OmniMux lets you describe or upload a character, generate videos featuring that character, and publish the results.
- You can reach the service through the web console, a REST API, a CLI, a Skill and an MCP server. Use through the API, CLI, Skill and MCP server is billed per call.
- Video and image output is produced by third-party AI models. Generation is probabilistic: results may contain errors, artifacts, distortions or unintended content, and may differ from your description or from an earlier run of the same prompt.
- We do not guarantee any particular output, any level of quality, or that a given generation will succeed.

## 3. Your account

- You must be at least 18 years old to create an account and use OmniMux.
- You sign in with an email address and a password. Keep your password secret and keep your email inbox secure: confirmation links and password reset codes are sent there. Anyone who can read that inbox can take over your account.
- One account is for one person. Do not share your account or your credentials, and do not create accounts on behalf of others.
- You are responsible for everything that happens through your account, including generations made and credits spent, whether or not you authorised it.
- Tell us promptly at hello@omnimux.ai if you believe your account has been accessed without your permission.
- The information you give us when you register must be accurate, and you must keep it up to date.

## 4. Credits, plans and payments

- Generations are paid for with credits. The price of a generation and the credits it costs are shown on the pricing page before you confirm and pay. Prices and credit costs may differ between models and between settings.
- Credits are a prepaid balance for using the service. They have no cash value, cannot be transferred or sold, and cannot be exchanged for money except where the [Refund Policy](/refunds) says otherwise.
- Subscription plans exist. A plan renews automatically at the end of each billing period and keeps renewing until you cancel it.
- You can cancel a subscription at any time from your account. Cancellation stops future renewals; the plan stays active until the end of the period you have already paid for. Cancelling does not by itself refund the current period.
- We may change prices and credit costs. If we change the price of a subscription plan, we will give you notice before the change takes effect on your next renewal, and you may cancel before then. The price of an individual generation is shown before each purchase.
- Payments are handled by an external payment provider. Your card details are entered with that provider and never reach our servers. Your use of the payment provider may also be governed by its own terms.
- If a payment fails, is reversed or is charged back, we may suspend the plan or the account until the amount is settled.
- Failed generations are refunded automatically. See the [Refund Policy](/refunds) for what counts as a failure and how refunds work.
- You are responsible for any taxes that apply to your purchase, other than taxes on our own income.

## 5. Your content

- "Your content" means the material you upload, submit or describe to OmniMux — for example photos, videos, prompts and character descriptions — and the output you generate from it.
- You keep ownership of your content. We do not claim ownership of your uploads or of the videos you generate, and you may use your output for commercial purposes, including advertising and client work.
- You grant us a limited, worldwide, non-exclusive licence to host, store, copy, transmit, process and display your content, and to share it with the third-party providers described in section 7, solely to operate, provide and support the service and to comply with the law. This licence ends when you delete the content or your account, except for copies we must keep for legal, security or backup purposes, and except for what a third party has already received as part of a generation.
- You confirm that you hold all the rights needed to upload your content and to generate from it, and that your content does not infringe anyone's copyright, trademark, privacy or other rights.
- If your content shows or describes a real person, you must have that person's consent and must be able to show it if we ask. The [Acceptable Use Policy](/acceptable-use) sets out the rules in full, including the ban on public figures.
- We do not train AI models on your content. We do not use your uploads or your generated output to train our own models or third-party models, and we do not sell your content.
- You are responsible for your content and for how you use the output. AI-generated video can be mistaken for real footage: label it as AI-generated where the platform you publish on requires it, and do not present it as real footage of a real person or a real event.
- We may remove content or refuse a generation where the law requires it, or where the content breaks these terms or the [Acceptable Use Policy](/acceptable-use).

## 6. Acceptable use

Your use of OmniMux is governed by the [Acceptable Use Policy](/acceptable-use), which forms part of these terms. It covers, among other things, consent for real people, prohibited content, the labelling of AI-generated video, and how we enforce the rules.

If you break the Acceptable Use Policy, we may remove content, refuse or cancel generations, and suspend or close your account. Where the law requires it, we may also report content to the authorities.

## 7. Third-party services

- OmniMux depends on services we do not control, including our hosting, database, AI model, email and payment providers.
- Your generations are produced by those third-party AI models. Their output may be subject to their own terms and policies, and their availability and behaviour can change without notice to us.
- We are not responsible for the acts, omissions, outages or output of third-party services, and we do not endorse or guarantee anything they provide.
- If a third-party provider stops offering a capability we rely on, we may have to change or withdraw the affected feature. Section 8 explains how that works.

## 8. Availability and changes

- We work to keep OmniMux available, but we do not promise any level of availability, uptime or response time. The service may be unavailable because of maintenance, capacity limits, provider outages, or factors outside our control.
- Generations depend on third-party models and may fail, time out, queue for a long time, or return a broken result. Failed generations are refunded under the [Refund Policy](/refunds).
- We may add, change, limit or remove features, models, settings or access channels (the web console, API, CLI, Skill and MCP server) at any time. Where a change materially reduces what you have paid for, we will give you reasonable notice and you may cancel the affected plan.
- We may set limits on usage, such as rate limits, queueing, storage, or the size and number of generations, to protect the service and other users.

## 9. Liability

- OmniMux is provided "as is" and "as available". To the extent the law allows, we exclude all warranties, conditions and representations that are not stated in these terms, including any implied warranty of merchantability or fitness for a particular purpose, and any warranty that output will be accurate, original, lawful in your jurisdiction, or fit for the use you intend.
- We are not liable for indirect, incidental, special, punitive or consequential losses, or for lost profits, lost revenue, lost goodwill, lost data or business interruption, however they arise and even if we were warned they were possible.
- We are not liable for what you do with the output, for how third parties react to it, or for claims that your content or your use of the output infringes someone else's rights.
- To the extent the law allows, our total liability for all claims relating to OmniMux or these terms is limited to the total amount you actually paid us for the service in the twelve months immediately before the event that gave rise to the claim.
- Nothing in these terms limits liability that cannot lawfully be limited, and nothing removes or reduces the mandatory rights you have as a consumer under the law that applies to you, including your right to bring proceedings in your local courts (see section 13).

## 10. Indemnity

You agree to indemnify and hold harmless the OmniMux team against claims, demands, losses, liabilities, costs and reasonable legal fees that arise from: your content; your use of the output; your breach of these terms, of the [Acceptable Use Policy](/acceptable-use) or of the law; or your infringement of another person's rights.

We will tell you promptly about a claim we intend to cover under this section, we will not settle it in a way that admits your fault without your agreement, and you may take part in the defence with your own lawyer. This section does not apply to the extent a claim results from something we did wrong.

## 11. Ending your account

- You may delete your account at any time from your account settings.
- Deleting your account takes effect immediately. It cancels any subscription immediately, with no refund for the unused part of the current period, and it removes your characters, your videos and any credits remaining in your balance. Deleted content and deleted credits cannot be restored.
- Before deleting, export or download anything you want to keep.
- We may suspend or close your account if you materially or repeatedly breach these terms or the [Acceptable Use Policy](/acceptable-use), if the law requires it, if we must do so to protect other users or the service, or if your account is used for fraud or abuse. Where it is reasonable to do so, we will tell you why and give you a chance to respond.
- If we close your account because you breached these terms, we may do so without refund. If we close it for our own convenience and without cause, we will refund credits you have paid for and not used.
- We may keep some records after your account closes where the law requires it or where we need them to resolve a dispute. The [Privacy Policy](/privacy) explains what we keep and for how long.

## 12. Changes to these terms

- We may update these terms as the service, the law or our practices change. The date at the top of this page shows when they were last updated.
- For significant changes — for example changes that affect your rights, your content, or what you pay — we will give you notice in advance, by email or in the product, before the change takes effect.
- If you do not agree with a change, you may stop using OmniMux and cancel your plan before the change takes effect. Continuing to use OmniMux after a change takes effect means you accept the updated terms.
- The English version of these terms is the authoritative one. Translations are provided for convenience.

## 13. Law and disputes

- These terms are governed by the laws of the State of Wyoming, United States, without regard to conflict-of-law rules, and disputes are handled by the courts of Wyoming, except where the law that protects you as a consumer gives you the right to rely on the law and the courts of the country where you live. Nothing in this section takes that right away.
- Before starting formal proceedings, please write to hello@omnimux.ai and describe the problem. Most issues are resolved faster that way, and we will try to answer within a reasonable time.
- If a dispute cannot be settled informally, either of us may bring proceedings before the courts that are competent under the paragraph above.

## 14. Contact

Questions about these terms, your account, payments or content can be sent to hello@omnimux.ai. We aim to answer within a reasonable time.

See also the [Privacy Policy](/privacy), the [Refund Policy](/refunds) and the [Acceptable Use Policy](/acceptable-use).
`,
    zh: `本条款是您与 OmniMux 之间的协议。您注册账号或以任何方式使用 OmniMux，即表示接受本条款。如不接受，请勿使用本服务。

## 1. 我们是谁

OmniMux 是一个 AI UGC 视频工作台，由 **Twinspace Labs LLC** 运营。该公司为在美国怀俄明州注册的有限责任公司（备案编号 2025-001824581），主要办公地址为 447 Broadway, 2nd Floor Suite 3272, New York, NY 10013, USA。在本条款中，“我们”“我方”与“OmniMux”均指 Twinspace Labs LLC，“您”指账号持有人。

如对本条款有任何疑问，可通过 hello@omnimux.ai 联系我们。

## 2. 服务内容

- OmniMux 让您可以描述或上传一个角色，生成以该角色为主角的视频，并发布成片。
- 您可以通过网页控制台、REST API、命令行工具（CLI）、Skill 与 MCP 服务器访问本服务。通过 API、CLI、Skill 与 MCP 服务器使用，按调用次数计费。
- 视频与图片结果由第三方 AI 模型生成。生成具有概率性：结果可能包含错误、瑕疵、扭曲或非预期内容，也可能与您的描述、或与同一提示词此前的生成结果不同。
- 我们不保证任何特定结果、任何质量水平，也不保证某次生成一定成功。

## 3. 您的账号

- 您必须年满 18 周岁才能注册账号并使用 OmniMux。
- 您使用邮箱地址与密码登录。请妥善保管密码，并确保邮箱安全：确认链接与密码重置验证码都会发送到该邮箱。任何能读取该邮箱的人都可以接管您的账号。
- 一个账号仅限一人使用。请勿共享账号或登录凭据，也请勿代他人注册账号。
- 账号下发生的一切由您负责，包括产生的生成与消耗的积分，无论是否经过您的授权。
- 如果您认为账号在未经您许可的情况下被访问，请尽快通过 hello@omnimux.ai 告知我们。
- 您注册时提供的信息必须真实准确，并在发生变化后及时更新。

## 4. 积分、套餐与支付

- 生成使用积分支付。每次生成的价格与所需积分会在您确认付款前于定价页面展示。不同模型、不同设置的定价与积分消耗可能不同。
- 积分是用于使用本服务的预付余额。积分不具备现金价值，不可转让或出售；除[退款政策](/refunds)另有规定外，不可兑换为现金。
- 我们提供订阅套餐。套餐在每个计费周期结束时自动续费，直至您取消。
- 您可以随时在账号中取消订阅。取消将停止后续续费；套餐在您已付费的周期结束前保持有效。取消本身不退还当前周期的费用。
- 我们可能调整价格与积分消耗。如调整订阅套餐的价格，我们会在变更对您的下一次续费生效前通知您，您可以在生效前取消。单次生成的价格会在每次购买前展示。
- 支付由外部支付服务商处理。您的卡号信息在该服务商处输入，不会到达我们的服务器。您对该支付服务商的使用还可能受其自身条款约束。
- 如果支付失败、被撤销或被拒付，我们可能暂停该套餐或账号，直至款项结清。
- 生成失败会自动退回积分。何种情况属于失败、退款如何运作，请见[退款政策](/refunds)。
- 除我们自身收入应缴的税款外，您需承担本次购买产生的相关税费。

## 5. 您的内容

- “您的内容”指您上传、提交或向 OmniMux 描述的材料（例如照片、视频、提示词与角色描述），以及您由此生成的结果。
- 您保留对您内容的所有权。我们不主张对您上传的材料或您生成的视频享有所有权，您可以将生成结果用于商业用途，包括广告投放与客户项目。
- 您授予我们一项有限的、全球范围的、非独占的许可，允许我们托管、存储、复制、传输、处理与展示您的内容，并将其提供给第 7 条所述的第三方服务商，且仅限于运营、提供与支持本服务以及遵守法律之目的。当您删除该内容或注销账号时，该许可终止；但我们为法律、安全或备份目的必须保留的副本，以及第三方在生成过程中已经获取的部分除外。
- 您确认您拥有上传您的内容并据此生成所需的全部权利，且您的内容不侵犯任何人的著作权、商标权、隐私权或其他权利。
- 如果您的内容中出现或描述真实人物，您必须取得该人物的同意，并在我们要求时能够提供证明。[可接受使用政策](/acceptable-use)完整规定了相关规则，包括禁止使用公众人物。
- 我们不会用您的内容训练 AI 模型。我们不会使用您上传的材料或生成的成果来训练我们自己的模型或第三方模型，也不会出售您的内容。
- 您对您的内容以及生成结果的使用方式负责。AI 生成的视频可能被误认为真实影像：在发布平台要求时，请标注为 AI 生成，并且不要把它当作真实人物或真实事件的真实影像来呈现。
- 在法律要求，或内容违反本条款或[可接受使用政策](/acceptable-use)时，我们可能删除内容或拒绝生成。

## 6. 可接受使用

您对 OmniMux 的使用受[可接受使用政策](/acceptable-use)约束，该政策构成本条款的一部分。其内容涵盖真实人物授权、禁止内容、AI 生成视频的标注，以及我们如何执行相关规则等。

若您违反可接受使用政策，我们可能删除内容、拒绝或取消生成，并暂停或关闭您的账号。在法律要求时，我们还可能向有关部门举报相关内容。

## 7. 第三方服务

- OmniMux 依赖我们无法控制的服务，包括我们的托管、数据库、AI 模型、邮件与支付服务商。
- 您的生成结果由上述第三方 AI 模型产出。其产出可能受其自身条款与政策约束，其可用性与表现也可能在我们未获通知的情况下发生变化。
- 我们对第三方服务的行为、不作为、中断或产出不承担责任，也不为其提供的任何内容背书或作出保证。
- 如果第三方服务商停止提供我们所依赖的能力，我们可能不得不变更或下线受影响的功能。第 8 条说明了相关处理方式。

## 8. 可用性与变更

- 我们努力保持 OmniMux 可用，但不承诺任何可用性、正常运行时间或响应时间水平。服务可能因维护、容量限制、服务商中断或我们无法控制的因素而不可用。
- 生成依赖第三方模型，可能失败、超时、长时间排队，或返回损坏的结果。生成失败按[退款政策](/refunds)退款。
- 我们可能随时新增、变更、限制或移除功能、模型、设置或访问渠道（网页控制台、API、CLI、Skill 与 MCP 服务器）。如果某项变更实质减少了您已付费获得的服务，我们会给予合理通知，您可以取消受影响的套餐。
- 为保护本服务与其他用户，我们可能设置使用限制，例如速率限制、排队、存储空间，以及单次生成的大小与数量。

## 9. 责任

- OmniMux 按“现状”与“现有可用”提供。在法律允许的范围内，我们排除本条款未明确陈述的一切保证、条件与陈述，包括对适销性、特定用途适用性的默示保证，以及结果准确、原创、在您所在司法辖区合法或适合您预期用途的任何保证。
- 我们不对间接、附带、特殊、惩罚性或后果性损失负责，也不对利润损失、收入损失、商誉损失、数据丢失或业务中断负责，无论其如何产生，即使我们已被告知可能发生。
- 我们不对您如何使用生成结果、第三方对生成结果的反应，以及因您的内容或您对生成结果的使用而被主张侵犯他人权利承担责任。
- 在法律允许的范围内，我们对与 OmniMux 或本条款相关的一切索赔的累计责任，以引发索赔的事件发生前十二个月内您就本服务实际支付给我们的总金额为上限。
- 本条款不限制依法不可限制的责任，也不排除或削减您依所适用法律作为消费者享有的强制性权利，包括在您当地法院提起诉讼的权利（见第 13 条）。

## 10. 赔偿

对于因以下原因引起的索赔、要求、损失、责任、费用及合理的律师费，您同意向 OmniMux 团队作出赔偿并使其免受损害：您的内容；您对生成结果的使用；您违反本条款、[可接受使用政策](/acceptable-use)或法律；或您侵犯他人权利。

我们会及时告知您我们拟依本条主张的索赔；未经您同意，我们不会以承认您过错的方式和解；您也可以委托自己的律师参与抗辩。若某项索赔系因我们的过错造成，本条不适用。

## 11. 账号终止

- 您可以随时在账号设置中注销账号。
- 注销立即生效。它将立即取消任何订阅，不退还当前周期未使用部分的费用，并删除您的角色、视频以及余额中剩余的积分。已删除的内容与积分无法恢复。
- 注销前，请导出或下载您想保留的内容。
- 若您严重或反复违反本条款或[可接受使用政策](/acceptable-use)，法律要求我们这样做，我们必须这样做以保护其他用户或本服务，或您的账号被用于欺诈或滥用，我们可能暂停或关闭您的账号。在合理可行时，我们会说明原因并给予您回应的机会。
- 若因您违反本条款而关闭账号，我们可能不予退款。若我们为自身便利且无特定原因关闭账号，我们会退还您已支付但未使用的积分。
- 账号关闭后，在法律要求或为解决争议所需的情况下，我们可能保留部分记录。[隐私政策](/privacy)说明了我们保留哪些内容以及保留多久。

## 12. 条款变更

- 随着服务、法律或我们的实践发生变化，我们可能更新本条款。本页顶部的日期表示最近一次更新时间。
- 对于重大变更——例如影响您的权利、您的内容或您付费内容的变更——我们会在变更生效前，通过邮件或产品内通知提前告知您。
- 如果您不同意某项变更，可以在变更生效前停止使用 OmniMux 并取消套餐。变更生效后继续使用 OmniMux，即表示您接受更新后的条款。
- 本条款以英文版本为准，其他语言的翻译仅供参考。

## 13. 法律与争议

- 本条款适用美国怀俄明州法律（不考虑法律冲突规则），争议由怀俄明州法院处理；但如果保护您作为消费者的法律赋予您适用您居住国法律与法院的权利，则该权利不受影响。本条不剥夺该项权利。
- 在启动正式程序之前，请写信至 hello@omnimux.ai 说明问题。多数问题通过这种方式解决得更快，我们会尽量在合理时间内答复。
- 若争议无法通过非正式方式解决，任何一方均可向依上一款有管辖权的法院提起诉讼。

## 14. 联系方式

关于本条款、您的账号、支付或内容的疑问，可发送至 hello@omnimux.ai。我们会尽量在合理时间内答复。

另请参阅[隐私政策](/privacy)、[退款政策](/refunds)与[可接受使用政策](/acceptable-use)。
`,
  },
}
