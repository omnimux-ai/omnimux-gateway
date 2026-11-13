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
import { acceptableUseTranslations } from './translations-acceptable-use'
import { legalRussian } from './translations-ru'

export const acceptableUseContent: LegalDocumentContent = {
  title: {
    en: 'Acceptable Use Policy',
    zh: '可接受使用政策',
    fr: acceptableUseTranslations.fr.title,
    ja: acceptableUseTranslations.ja.title,
    vi: acceptableUseTranslations.vi.title,
    zhTW: acceptableUseTranslations.zhTW.title,
    ru: legalRussian['acceptable-use'].title,
  },
  updated: '2026-10-08',
  body: {
    fr: acceptableUseTranslations.fr.body,
    ja: acceptableUseTranslations.ja.body,
    vi: acceptableUseTranslations.vi.body,
    zhTW: acceptableUseTranslations.zhTW.body,
    ru: legalRussian['acceptable-use'].body,
    en: `OmniMux is a creative tool for making AI UGC videos: you create a character from a description or an image, generate videos with that character, and publish the results. It is not a tool for impersonating, deceiving or harming real people. This policy applies to everything you upload, describe or generate with OmniMux, and it forms part of your agreement with us under the [Terms of Service](/terms).

## 1. Real people and consent

A character can be built from a written description or from an image. Where a real person is involved, that person's consent decides whether the use is allowed.

- You may use a photo of a real person as a character only if it is a photo of you, or if you have that person's explicit permission.
- You may upload a video of a real person only if you appear in it yourself, or if you have the permission of the people who appear in it and you hold the rights to that video.
- You may not use a public figure - a celebrity, a politician, an influencer, an athlete or any other well-known person - even as a parody or a joke, unless you are that person.
- You may not create or publish content that could reasonably be mistaken for a real event or for a real statement by a real person. This covers deepfakes, fake news, and fake endorsements or testimonials attributed to someone who never gave them.
- Permission has to be real and specific to the use you are making of it. A photo or video you found online, an assumption that someone would agree, or permission given for a different purpose is not enough.
- If someone withdraws their permission, stop using their image and remove the content.

## 2. Strictly prohibited content

The following is never allowed on OmniMux, whether it appears in an input, a description, a character or a generated video.

- Anything involving minors, or characters that look like minors, in any sexual or suggestive context.
- Nudity, and sexual or pornographic content of any kind.
- Harassment, bullying, humiliation, threats, or content that targets a specific person or group.
- Hate speech, or content that promotes discrimination or violence.
- Graphic violence, content that promotes or depicts self-harm, or content that promotes illegal activity.
- Political or electoral manipulation, misinformation, scams and fraud.
- Content that infringes copyright, trademarks or any other rights of any person, including videos you have no right to use.

## 3. Children

- OmniMux is a service for adults, and you must be at least 18 years old to use it, as set out in our [Terms of Service](/terms).
- Do not create a character from a photo of a child, and do not upload videos that feature children.
- Accounts and content that involve children are removed when we find them.

## 4. Label AI content

- When you publish a video made with OmniMux, follow the rules of the platform you publish it on.
- TikTok, Instagram and YouTube require realistic AI-generated content to be labelled as AI-generated. Use the AI label those platforms provide.
- Labelling is your responsibility as the person who publishes the content.

## 5. How we enforce this policy

- Inputs and outputs may be checked by automated safety filters. A generation that fails those filters is blocked.
- We may review content that is reported to us, or that our systems flag.
- If you break this policy, we may remove the content, refuse to generate it, and suspend or close your account without a refund, as described in our [Refund Policy](/refunds).
- Serious violations may be reported to the competent authorities.
- These checks are not exhaustive. We do not review every piece of content, and a generation passing a filter does not make it allowed. You remain responsible for what you create and publish.
- If you believe we got an enforcement decision wrong, contact hello@omnimux.ai and tell us why.

## 6. Report abuse

- If you believe content made with OmniMux uses your image or likeness without your consent, or breaks this policy in any other way, email hello@omnimux.ai.
- Include a link to the content or a description that helps us find it, and explain what the problem is. If you are reporting for someone else, say so.
- We review reports and take action where this policy has been broken.
- See our [Privacy Policy](/privacy) for how we handle the information you send us.
`,
    zh: `OmniMux 是一款创作 AI UGC 视频的工具：您用一段描述或一张图片创建角色，用该角色生成视频，并发布成片。它不是用来冒充、欺骗或伤害真实人物的工具。本政策适用于您通过 OmniMux 上传、描述或生成的一切内容，并依据[服务条款](/terms)构成您与我们之间协议的一部分。

## 1. 真实人物与授权

角色可以用一段文字描述创建，也可以用一张图片创建。只要涉及真实人物，对方是否同意就决定了这种使用是否被允许。

- 只有当照片是您本人，或您已取得照片中人物明确授权时，才能用真实人物的照片创建角色。
- 只有当视频中有您本人出镜，或您已取得片中出镜人物的授权、并且拥有该视频的权利时，才能上传真实人物的视频。
- 不得使用公众人物——明星、政客、网红、运动员或其他任何知名人物——即使是恶搞或玩笑，除非您就是本人。
- 不得创建或发布可能被合理误认为真实事件、或真实人物真实言论的内容。这包括深度伪造、虚假新闻，以及把从未作出该表述的人包装成代言或证言的内容。
- 授权必须是真实的，且明确针对您实际进行的使用。网上找到的照片或视频、您以为对方会同意的猜测，或为其他用途取得的授权，都不算数。
- 如果对方撤回授权，请停止使用其形象并删除相关内容。

## 2. 严格禁止的内容

以下内容在 OmniMux 上绝对不允许，无论它出现在输入、描述、角色还是生成的视频中。

- 涉及未成年人、或外观像未成年人的角色，出现在任何性相关或性暗示场景中。
- 任何形式的裸露，以及任何色情或性内容。
- 骚扰、霸凌、羞辱、威胁，或针对特定个人或群体的内容。
- 仇恨言论，或宣扬歧视与暴力的内容。
- 血腥暴力、宣扬或描绘自残的内容，或宣扬违法行为的内容。
- 政治或选举操纵、虚假信息、诈骗与欺诈。
- 侵犯著作权、商标权或任何人其他权利的内容，包括您无权使用的视频。

## 3. 儿童

- OmniMux 是面向成年人的服务，您必须年满 18 周岁才能使用，详见[服务条款](/terms)。
- 不得用儿童的照片创建角色，也不得上传播放儿童的视频。
- 涉及儿童的账号与内容，一经发现即予移除。

## 4. 标注 AI 内容

- 发布用 OmniMux 制作的视频时，请遵守您所发布平台的规则。
- TikTok、Instagram 与 YouTube 均要求逼真的 AI 生成内容标注为 AI 生成。请使用这些平台提供的 AI 标注功能。
- 作为发布者，标注是您的责任。

## 5. 我们如何执行本政策

- 输入与输出可能经过自动化安全过滤。未通过过滤的生成会被拦截。
- 我们可能审核被举报的内容，或被我们系统标记的内容。
- 如果您违反本政策，我们可能删除相关内容、拒绝生成，并在不退款的情况下暂停或关闭您的账号，具体见[退款政策](/refunds)。
- 严重违规可能被报告给主管机关。
- 这些检查并非穷尽。我们不会审核每一条内容，某次生成通过了过滤也不代表它就被允许。您仍需对自己创建和发布的内容负责。
- 如果您认为我们的处理决定有误，请写信至 hello@omnimux.ai 说明理由。

## 6. 举报滥用

- 如果您认为用 OmniMux 制作的内容未经同意使用了您的形象或肖像，或以其他方式违反本政策，请写信至 hello@omnimux.ai。
- 请附上内容链接，或能帮助我们定位该内容的说明，并解释问题所在。如果您是代他人举报，请一并说明。
- 我们会审核举报，并在确认违反本政策时采取处理措施。
- 关于我们如何处理您提供的信息，请见[隐私政策](/privacy)。
`,
  },
}
