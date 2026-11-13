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
import { privacyTranslations } from './translations-privacy'
import { legalRussian } from './translations-ru'

export const privacyContent: LegalDocumentContent = {
  title: {
    en: 'Privacy Policy',
    zh: '隐私政策',
    fr: privacyTranslations.fr.title,
    ja: privacyTranslations.ja.title,
    vi: privacyTranslations.vi.title,
    zhTW: privacyTranslations.zhTW.title,
    ru: legalRussian['privacy'].title,
  },
  updated: '2026-11-02',
  body: {
    fr: privacyTranslations.fr.body,
    ja: privacyTranslations.ja.body,
    vi: privacyTranslations.vi.body,
    zhTW: privacyTranslations.zhTW.body,
    ru: legalRussian['privacy'].body,
    en: `This policy explains what personal data OmniMux collects when you use the service, why we use it, who we share it with, how long we keep it, and the rights you have over it. It covers the OmniMux workspace at omnimux.ai, where you describe or upload a character, generate videos with that character, and publish the results.

## 1. Who is responsible

OmniMux is the data controller for the personal data described in this policy. That means we decide why and how that data is used, and we are the party you contact about it. The service is operated by **Twinspace Labs LLC**, a limited liability company registered in Wyoming, United States (file number 2025-001824581), 447 Broadway, 2nd Floor Suite 3272, New York, NY 10013, USA.

- Privacy questions, requests and complaints: hello@omnimux.ai
- We answer requests about your personal data at that address. To verify who is asking, we normally ask you to write from the email address registered to your account.

The other rules that apply to your account are in the [Terms of Service](/terms) and the [Acceptable Use Policy](/acceptable-use).

## 2. What we collect

We collect the following categories of personal data:

- **Account data**: your email address and the date you signed up.
- **Character data**: the character descriptions you write and the images you import to build a character.
- **Content you upload**: videos and other material you upload to the service.
- **Generated content**: the images and videos the service produces for you.
- **Plan and credits**: your plan, your credit balance and your credit history.
- **Payment records**: what you bought and when. Card details are handled by our payment provider and are never seen by OmniMux.
- **Technical data**: the logs we need to run and secure the service, such as IP address, browser and device information, error reports and timestamps.
- **Connected TikTok account data (optional)**: when you link your TikTok account through TikTok Login Kit, we receive your TikTok account identifier (open_id), avatar image, display name, creator publishing settings, and the OAuth access and refresh tokens needed to act on your instructions. We never receive your TikTok password.

We do not sell your personal data, and we do not build advertising profiles from it.

## 3. Photos and videos of people

You can upload photos and videos that show real people, and some of what you generate will show faces.

- Uploaded material is used **only to generate the content you asked for**. We do not use it for anything else.
- We do not create face templates or biometric identifiers from your uploads, and we do not use them to recognise or identify anyone.
- We do not train AI models on your content.
- You may only upload images or videos of yourself, or of people who have given you their consent to be used in this way. Using someone else's face without permission breaks our [Acceptable Use Policy](/acceptable-use).
- Raw uploads are removed from active processing once the generation that needed them has finished. Generated results stay in your library until you delete them.

## 4. Why we use your data (legal bases)

We use your personal data for the purposes below. Each purpose rests on a legal basis.

- **To provide the service (performance of a contract)**: creating and securing your account, storing your characters and uploads, running generations on our AI model providers, keeping your credit balance, and delivering the media you asked for. We cannot provide the service without this data.
- **To connect and publish to your TikTok account (performance of a contract and your explicit authorization)**: displaying your linked TikTok account in your workspace, checking your publishing eligibility (creator_info), and uploading or publishing the videos and photo posts you select to your TikTok profile via the TikTok Content Posting API, plus polling delivery status. We never post or interact on TikTok without your explicit action.
- **To keep the service safe and prevent abuse (legitimate interests)**: detecting fraud, misuse and attempts to generate prohibited content, enforcing our [Acceptable Use Policy](/acceptable-use), and protecting the service and other users. We weigh these interests against your rights and expectations.
- **To meet legal obligations (legal obligation)**: keeping invoicing, tax and accounting records, and responding to lawful requests from authorities.
- **To send you product and marketing email (consent)**: only where you have opted in. You can withdraw that consent at any time using the link in the email or by writing to hello@omnimux.ai, and we will stop sending marketing email. We may still send service messages that your account needs, such as security, billing and policy notices.

## 5. Who we share it with

We do not sell your personal data and we do not share it for advertising. We share it only in these situations.

- **Providers acting on our instructions**: companies that process data on our behalf and only for the purposes we set. They fall into these categories:
  - our hosting and analytics provider, which runs the service and gives us privacy-friendly aggregate usage statistics;
  - our database and file-storage provider, which stores your account, your characters and your media;
  - our AI model providers, which generate the images and videos you request;
  - our email provider, which sends account and service email;
  - our payment provider, which processes payments and handles card data directly.
- **Third-party platforms you connect (TikTok)**: when you link your TikTok account and submit a post, the media and post settings you choose are transmitted to TikTok (and, where applicable, our social-publishing infrastructure subprocessor) to complete your request under TikTok's own terms and privacy policy. Your TikTok OAuth tokens are stored encrypted at rest using AES-256-GCM.
- **Legal requirements**: where the law, a regulation or a valid legal order requires us to disclose data, or where disclosure is necessary to protect the rights, safety or property of OmniMux, our users or the public.
- **Business transfers**: if the service or its assets move to another operator, data may transfer as part of that deal. We would tell you before your data became subject to a different privacy policy.

Where data leaves your region, we rely on appropriate safeguards for that transfer.

## 6. How long we keep it

We keep personal data only as long as we need it for the purposes above.

- **Account, character and content data**: for as long as your account is open. You can delete a character, a video or your whole account yourself; account deletion removes the account and its content.
- **TikTok OAuth tokens and linked-account records**: kept only while your TikTok account remains connected. When you click Disconnect in OmniMux, we immediately revoke the token with TikTok and erase the stored credentials; you can also revoke OmniMux at any time in the TikTok app under Settings and privacy → Security and permissions → Apps and services permissions, or request deletion via our privacy contact email.
- **Payment and invoicing records**: for as long as tax and accounting law requires, which may be after your account is closed.
- **Technical logs**: for a short period, long enough to investigate incidents and keep the service secure, then deleted or reduced to aggregate statistics.
- **Backups**: deleted data can persist briefly in backups until those backups rotate out.

## 7. Cookies

We use cookies and similar browser storage only for what the service needs.

- **Strictly necessary**: keeping you signed in, remembering your language and theme, and protecting your session against cross-site request forgery. The service cannot work without these.
- **Aggregate analytics**: privacy-friendly aggregate statistics about which pages and features are used. They do not identify you individually, and they are not used to follow you across other sites.
- **No advertising cookies**: we do not use advertising or cross-site tracking cookies, and we do not sell data derived from cookies.

You can block or delete cookies in your browser settings. Blocking the strictly necessary ones will stop sign-in and other core parts of the service from working.

## 8. Your rights

Depending on where you live, you have some or all of the following rights over your personal data.

- **Access**: ask for a copy of the personal data we hold about you.
- **Correction**: ask us to fix data that is wrong or incomplete.
- **Export**: ask for your data in a portable, machine-readable format.
- **Deletion**: delete your characters, your videos and your whole account yourself in the app, or ask us to delete your data.
- **Restriction**: ask us to limit how we use your data while a question about it is resolved.
- **Objection**: object to processing that rests on our legitimate interests, including the checks we run for safety and abuse prevention.
- **Withdraw consent**: withdraw your consent to marketing email at any time. This does not affect what we lawfully did before you withdrew it.
- **Complain**: lodge a complaint with your local data protection authority. We would rather hear from you first at hello@omnimux.ai.

To exercise a right, write to hello@omnimux.ai from your account email. We answer within the period the applicable law sets. Exercising your rights is free, and we will not treat you differently for doing so.

**If you are in the United States, and in particular in California**, you also have the right to know the categories of personal data we collect and how we use and share them, the right to request deletion and correction, the right to opt out of the sale or sharing of personal data for targeted advertising (we do neither), the right to limit the use of sensitive personal data, and the right not to be discriminated against for exercising any of these rights. You or an authorised agent may make a request at hello@omnimux.ai; we verify it against your account before we act.

## 9. Children

OmniMux is a service for adults. You must be at least 18 years old, or the age of majority where you live, to hold an account.

- We do not knowingly collect personal data from children.
- If we learn that an account belongs to someone below that age, we remove the account and its content.
- If you believe a child has given us personal data, write to hello@omnimux.ai and we will act on it.

## 10. Security

We protect your data with technical and organisational measures, including:

- encryption of data in transit between your browser and our service;
- restricted, logged access to our production systems and to stored media;
- keeping card data out of our systems entirely, because payments are handled by our payment provider.

No method of transmission or storage is completely secure, so we cannot guarantee absolute security. If a breach affects your personal data, we notify you and the relevant authority as the applicable law requires.

## 11. Changes

We may update this policy as the service and the law change.

- The current version is always published on this page, with the date it was last updated shown at the top.
- If a change significantly affects how we use your personal data, we will notify you before it takes effect, for example by email or by a notice in the app.
- Continuing to use OmniMux after an update takes effect means you accept the updated policy. If you do not agree with it, you can stop using the service and delete your account.

Questions about this policy, or about how we handle your data, go to hello@omnimux.ai.
`,
    zh: `本政策说明您在使用 OmniMux 时我们会收集哪些个人数据、为什么使用、与谁共享、保留多久，以及您对这些数据拥有哪些权利。本政策适用于 omnimux.ai 上的 OmniMux 工作空间：您在这里描述或上传角色，用该角色生成视频，并发布生成结果。

## 1. 谁负责

OmniMux 是本政策所述个人数据的控制者。这意味着由我们决定这些数据为何被使用、如何被使用，您也就相关问题与我们联系。本服务由 **Twinspace Labs LLC** 运营，该公司为在美国怀俄明州注册的有限责任公司（备案编号 2025-001824581），办公地址为 447 Broadway, 2nd Floor Suite 3272, New York, NY 10013, USA。

- 隐私相关问题、请求与投诉：hello@omnimux.ai
- 我们通过该邮箱答复与您个人数据有关的请求。为核实申请人身份，我们通常会要求您使用账号注册邮箱来信。

适用于您账号的其他规则，见[服务条款](/terms)与[可接受使用政策](/acceptable-use)。

## 2. 我们收集什么

我们收集以下类别的个人数据：

- **账号数据**：您的邮箱地址与注册日期。
- **角色数据**：您撰写的角色描述，以及您为创建角色而导入的图片。
- **您上传的内容**：您上传到服务的视频及其他素材。
- **生成内容**：服务为您生成的图片与视频。
- **套餐与积分**：您的套餐、积分余额与积分流水。
- **支付记录**：您购买了什么、何时购买。卡号信息由我们的支付服务商处理，OmniMux 从不会看到。
- **技术数据**：运行与保障服务所需的日志，例如 IP 地址、浏览器与设备信息、错误报告与时间戳。
- **连接的 TikTok 账号数据（可选）**：当您通过 TikTok Login Kit 授权连接自己的 TikTok 账号时，我们会接收您的 TikTok 唯一标识（open_id）、头像、显示名称、创作者发布权限状态，以及用于代表您执行操作的 OAuth 访问令牌与刷新令牌。我们不会收集您的 TikTok 登录密码。

我们不会出售您的个人数据，也不会用它建立广告画像。

## 3. 含人物的照片与视频

您可以上传包含真实人物的照片与视频，您生成的部分内容也会包含人脸。

- 上传的素材**仅用于生成您所要求的内容**，我们不会用于其他任何用途。
- 我们不会从您的上传素材中建立人脸模板或生物识别标识，也不会用它们识别或辨认任何人。
- 我们不会用您的内容训练 AI 模型。
- 您只能上传您本人的图片或视频，或已就此取得对方同意的图片或视频。未经许可使用他人的脸，违反我们的[可接受使用政策](/acceptable-use)。
- 原始上传素材在所需的生成任务完成后，即从在用处理环节中移除。生成结果会保留在您的素材库中，直到您删除。

## 4. 我们为何使用您的数据（法律依据）

我们基于下述目的使用您的个人数据，每个目的都有相应的法律依据。

- **提供服务（履行合同）**：创建并保护您的账号、存储您的角色与上传素材、在我们的 AI 模型服务商上执行生成、维护您的积分余额，以及交付您所要求的媒体内容。没有这些数据，我们无法提供服务。
- **连接与发布至您的 TikTok 账号（履行合同与您的明确授权）**：仅在您主动点击绑定或发起发布指令时，使用您的 TikTok 授权令牌在应用内展示已绑定账号、查询创作者发布能力（creator_info），并通过 TikTok Content Posting API 将您指定的原创视频或图文上传或发布至您本人的 TikTok 账号及查询发布状态。未经您的操作指令，我们绝不会自动发帖或进行任何互动操作。
- **保障服务安全、防止滥用（正当利益）**：识别欺诈、滥用与生成违禁内容的尝试，执行我们的[可接受使用政策](/acceptable-use)，以及保护本服务与其他用户。我们会将这些利益与您的权利和合理预期相权衡。
- **履行法律义务（法律义务）**：保存开票、税务与会计记录，并回应主管部门的合法要求。
- **向您发送产品与营销邮件（同意）**：仅在您主动选择接收时。您可以随时通过邮件中的链接或写信至 hello@omnimux.ai 撤回该同意，我们将停止发送营销邮件。我们仍可能发送账号所需的服务通知，例如安全、账单与政策通知。

## 5. 我们与谁共享

我们不会出售您的个人数据，也不会为广告目的共享。仅在以下情形共享：

- **按我们指示行事的服务商**：代表我们处理数据、且仅用于我们设定目的的公司。它们分为以下几类：
  - 我们的托管与分析服务商：运行本服务，并为我们提供不针对个人的汇总使用统计；
  - 我们的数据库与文件存储服务商：存储您的账号、角色与媒体素材；
  - 我们的 AI 模型服务商：生成您所请求的图片与视频；
  - 我们的邮件服务商：发送账号与服务邮件；
  - 我们的支付服务商：处理支付，并直接处理卡号数据。
- **您主动授权的第三方社交平台（TikTok）**：当您使用 TikTok 账号绑定与内容发布功能时，您的发布指令及所选视频或图文素材会直接或经由我们的社媒基础设施处理商传输至 TikTok（受 TikTok 自身隐私政策约束）。您的 TikTok OAuth 令牌在我们的数据库中始终采用 AES-256-GCM 加密存储，绝不向其他无关第三方披露。
- **法律要求**：法律、法规或有效法律命令要求我们披露数据，或为保护 OmniMux、我们的用户或公众的权利、安全或财产而必须披露时。
- **业务转让**：如果本服务或其资产转由其他运营方经营，数据可能随该交易一并转移。在您的数据受另一份隐私政策约束之前，我们会通知您。

当数据离开您所在地区时，我们会为该传输依赖适当的保障措施。

## 6. 我们保留多久

我们仅在实现上述目的所需的期限内保留个人数据。

- **账号、角色与内容数据**：只要您的账号处于开通状态。您可以自行删除某个角色、某个视频乃至整个账号；删除账号会移除该账号及其内容。
- **TikTok 授权令牌与绑定信息**：仅在您维持账号连接期间保留（访问令牌约 24 小时有效并自动刷新，刷新令牌最长 365 天有效）。您可随时在 OmniMux 账号管理页点击「解绑（Disconnect）」——我们会立即调用 TikTok 接口吊销令牌并清除本地加密令牌；您也可随时在 TikTok App「设置与隐私 → 安全与权限 → 应用与服务权限」中移除 OmniMux，或发信至法务联系邮箱要求彻底删除。
- **支付与开票记录**：在税务与会计法律要求的期限内保留，可能延续至您的账号关闭之后。
- **技术日志**：短期保留，足够用于排查事件与维护服务安全，之后删除或汇总为统计数据。
- **备份**：被删除的数据可能在备份中短暂留存，直至相关备份轮换淘汰。

## 7. Cookie

我们仅为本服务的必要用途使用 Cookie 及类似的浏览器存储。

- **严格必要**：维持您的登录状态、记住您的语言与主题，以及保护您的会话免受跨站请求伪造攻击。没有这些，服务无法运行。
- **汇总分析**：关于哪些页面与功能被使用的、不针对个人的汇总统计。它不会单独识别您的身份，也不会用于跨其他网站追踪您。
- **无广告 Cookie**：我们不使用广告或跨站追踪 Cookie，也不出售由 Cookie 得出的数据。

您可以在浏览器设置中阻止或删除 Cookie。若阻止严格必要的 Cookie，登录及服务的其他核心功能将无法使用。

## 8. 您的权利

视您所在地区而定，您对个人数据享有下列部分或全部权利。

- **访问**：索取我们持有的您的个人数据副本。
- **更正**：要求我们修正错误或不完整的数据。
- **导出**：以可迁移、机器可读的格式索取您的数据。
- **删除**：在应用内自行删除您的角色、视频乃至整个账号，或要求我们删除您的数据。
- **限制处理**：在有关数据的问题得到解决之前，要求我们限制对数据的使用。
- **反对**：反对基于我们正当利益的处理，包括我们为安全与防滥用所做的检查。
- **撤回同意**：随时撤回对营销邮件的同意。这不影响您撤回之前我们已合法进行的处理。
- **投诉**：向您当地的数据保护主管机构投诉。我们也希望您先写信至 hello@omnimux.ai 告诉我们。

如需行使权利，请用您的账号邮箱写信至 hello@omnimux.ai。我们会在适用法律规定的期限内答复。行使权利是免费的，我们也不会因此对您区别对待。

**如果您位于美国，尤其是加利福尼亚州**，您还有权了解我们收集的个人数据类别以及我们如何使用和共享这些数据，有权要求删除与更正，有权选择退出个人数据的出售或为定向广告而进行的共享（我们两者都不做），有权限制敏感个人数据的使用，并且不会因行使上述任何权利而受到歧视。您本人或经授权的代理人可通过 hello@omnimux.ai 提出请求；我们会在采取行动前对照您的账号进行核实。

## 9. 儿童

OmniMux 是面向成年人的服务。您必须年满 18 周岁，或达到您所在地区的法定成年年龄，才能持有账号。

- 我们不会在知情的情况下收集儿童的个人数据。
- 如果我们发现某账号属于未达上述年龄的人，我们会移除该账号及其内容。
- 如果您认为有儿童向我们提供了个人数据，请写信至 hello@omnimux.ai，我们会据此处理。

## 10. 安全

我们通过技术与组织措施保护您的数据，包括：

- 对您的浏览器与我们的服务之间传输的数据进行加密；
- 对生产系统与所存储的媒体素材实行受限且留痕的访问；
- 由于支付由我们的支付服务商处理，卡号数据完全不进入我们的系统。

任何传输或存储方式都无法做到绝对安全，因此我们无法保证绝对的安全性。若发生影响您个人数据的数据泄露，我们会按适用法律的要求通知您及相关主管机构。

## 11. 变更

随着服务与法律的变化，我们可能更新本政策。

- 当前版本始终发布在本页面，并在页面顶部标明最近更新日期。
- 如果某项变更会显著影响我们使用您个人数据的方式，我们会在其生效前通知您，例如通过邮件或应用内通知。
- 更新生效后您继续使用 OmniMux，即表示您接受更新后的政策。如果您不同意，可以停止使用本服务并删除您的账号。

有关本政策或我们如何处理您数据的问题，请联系 hello@omnimux.ai。
`,
  },
}
