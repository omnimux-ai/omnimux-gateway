# OmniMux 品牌与定位

本目录是本仓库品牌、文案与信息架构的现行依据。OmniMux 定位于 **「属于你的内容电商团队」（Your Content Commerce Team）**——面向跨境与内容电商卖家及创作者的 AI 内容电商系统：把商品和创意交给桌面端 Agent，完成对标拆解、商品图与带货视频生成、角色视频复刻与排期发布。官网首页展示桌面端创作工作台，角色生成与参考视频复刻是其中的核心能力主线之一。以已合入的落地页、桌面插件、角色工作台和实际可用能力为依据，不以历史研究、计划或记忆代替当前事实。

## 核心定位

**属于你的内容电商团队**：一套内容电商系统，让商品卖点贯穿每一次出片。用你熟悉的 Agent，在桌面端跑通「对标 → 拆解 → 制作 → 发布 → 复盘」闭环，搞定广告、电商与种草视频。**首页是桌面端落地页**（2026-10-27 起），角色生成与参考视频复刻在桌面工作台与 `/ai-influencer-studio` 角色工作台中继续提供。

| 位置 | 中文 | English |
| --- | --- | --- |
| 品牌核心主张 | 属于你的内容电商团队 | Your Content Commerce Team |
| 首页首屏标题 | 发现、复刻和裂变电商爆款视频 | Find, recreate & scale viral shoppable videos |
| 首页首屏描述 | 一套内容电商系统，让商品卖点贯穿每一次出片。用你熟悉的 Agent，一站式搞定广告、电商与种草视频。 | One content commerce system that carries your product's selling points through every video. Use the Agents you already know to create ads, shoppable videos, and product reviews—all in one place. |
| 角色落地页首屏（`/ai-influencer-studio`） | 让你的角色成为热门视频的主角 | Put your character in any trend |
| 可复用简介 | OmniMux 是属于你的内容电商团队：一套内容电商系统，帮你管理商品卖点、拆解对标视频、创建 AI 角色与多角度设定图、生成与复刻带货视频，并在桌面端衔接从灵感对标到排期发布的创作链路。 | OmniMux is Your Content Commerce Team: a content-commerce system that helps you manage product selling points, break down reference videos, create AI characters and multi-view reference sheets, generate and recreate shoppable videos, and connect the workflow from inspiration to scheduled publishing on desktop. |
| 收费口径 | 按次消耗积分（支持预充值阶梯折扣加量包；可选套餐以价格区与钱包当前实际启用配置为准） | Pay-per-generation credits with volume top-up discounts (optional plans subject to active pricing and wallet configuration). |

首屏逐字文案及所在源码见 [落地页文案与实现](landing-page-copy.md)。首屏标题里的“爆款”“裂变”描述的是对标与复刻的工作方式，**不承诺**作品一定获得流量或销售额；桌面端能力边界见下文“已交付与未交付”。

## 品牌与网址绑定

对外资料统一使用品牌名 **OmniMux**、官网 **https://omnimux.ai/** 和上表的“可复用简介”。中文与英文各用同一份对应文本，不将首屏创意标题代替产品介绍。官网关于页、公开文档品牌页与官方社媒资料页相互链接；只有已核实归属的账号可列为官方入口。

准备公开收录的官网页面为首页、价格、关于、隐私政策（`/privacy`）、服务条款（`/terms`），以及英文教程目录（`/guides`）、角色创建教程（`/guides/create-ai-character`）和参考视频教程（`/guides/recreate-video-with-your-character`），各用本页正式地址、独立标题和对应正文。教程返回完整静态 HTML，不生成其他语言的英文复制地址；应用入口文案维护七语言。创作工作区仍不参与搜索收录；登录、账户及其他应用页面也排除收录。旧的自定义法律页面（`/privacy-policy`、`/user-agreement`）保留访问和自定义内容，不列入网站地图，也不合并到正文可能不同的新法律页面。

`/pricing` 是高级模型价目目录，搜索标题为 `OmniMux Model Pricing | Generation Rates and Credits`。可选套餐在首页价格区，账户实际权益在钱包；教程不复制静态价格或余额。教程素材是界面截图合成的操作示意与独立真实生成结果，不能描述为连续操作录屏。首篇角色结果只说明脸部与全身参考，不代指多角度设定图。

产品教程说明实际可用的操作；生成案例须能追溯输入、参考素材使用权和原始成品。现有产品演示不能自动升级为客户案例、现场生成结果或流量成效。搜索纠错原因须保留未知，品牌与网址统一不构成立即消除纠错的保证。谷歌收录、所选正式地址及最近抓取以站长后台核查为准；上线后再提交网站地图与关键页面抓取申请，提交回执不等于已经重新收录。

## 产品主线与边界

| 主线 | 用户任务 | 边界 |
| --- | --- | --- |
| 商品与卖点管理 | 在桌面端商品库（`omnimux-products`）维护商品图、核心卖点与素材，供 Agent 拆解与出片调用 | 不承诺自动完成选品或外部店铺同步 |
| 对标视频拆解与复刻 | 拆解参考视频的开场钩子、卖点与购买引导，结合商品素材或角色素材复刻带货视频 | 受参考素材规格和生成契约约束；须拥有素材、肖像与声音的使用权 |
| 角色生成与多角度设定图 | 用描述或特征搭配生成 AI 角色，从本人成功的角色主图生成多角度参考，供后续带货与剧情创作复用 | 图片输入数量与格式以工作台校验为准；页面样例和生成提示不保证每次完全一致 |
| 画布编排与视频剪辑 | 在桌面端无限画布（`omnimux-workflow`）编排节点流程，并用剪辑工具（`omnimux-clip`）处理成片 | 不承诺无人值守全自动策划或独立封面生成 |
| 成品管理与排期发布 | 查看历史结果、下载成品，或在桌面发布中心（`omnimux-publish`）通过 Zernio 调度链路向海外平台（TikTok / Instagram / Facebook / YouTube）定时排期发布 | 导出不等于已发布；TikTok 官方直连默认关闭且默认 `SELF_ONLY` 私密；不支持大陆平台发布与发布前自动预检 |

不把选品、全自动账号托管、大陆平台发布或实时竞品趋势抓取写成当前产品自动完成的整套服务。相关历史接口或命令存在，不构成落地页的默认产品承诺，也不授权重新开放未交付业务。

## 桌面端与首页（omnimux.ai）

2026-10-27 起，**桌面端落地页就是 omnimux.ai 的首页**（`web/src/features/omnimux-landing/desktop/`）。原角色落地页迁到 `/ai-influencer-studio`，两套文案各自独立、不互相复用主张。`d.omnimux.ai` 不再是独立对外表面：它 301 跳转到 `omnimux.ai`，过渡期后删除解析；代码里按 Host 分流的桌面表面只保留给 `?surface=desktop` 预览。

| 项 | 首页（omnimux.ai） | 角色落地页（`/ai-influencer-studio`） |
| --- | --- | --- |
| 首页 | 桌面落地页：发现 → 拆解 → 制作 → 发布 → 复盘 | 角色生成与参考视频复刻落地页 |
| 顶栏入口 | 文档、能力、**定价**、**灵感社区** | 定价与灵感为同页锚点，无独立入口 |
| 模型价目（`/pricing`） | 显式入口，默认 `desktop-720p` 分组 | 页面可直达但无入口、不进 sitemap、`noindex` |
| 灵感社区（`/inspiration`） | 显式入口 | 页面可直达但无入口、`noindex` |
| 计费 | `desktop-480p` / `desktop-720p` 模型专属分组 | `seedance-2-5-480p` / `seedance-2-5-720p` |
| 收录 | 首页 `index,follow` + canonical，进 sitemap | 保持现有公开页与 sitemap |

下载主 CTA 只能指向 `dl.omnimux.ai`。

### 已交付与未交付

能力有两个可能的落点，只查一处等于没查：网关行为看 `router/` + `controller/` + `service/`；桌面端行为看 `omnimux-desktop-fork` 的 `dsh-plugin-desktop/preset/skeleton/package.json` 里 `dsh.profile.bundles` 的清单，再看 `dsh-plugin/product/omnimux-dsh/plugins/<name>/` 的源码。2026-10-27 曾因只搜 Go 仓，把商品库、画布、发布中心误判为「未实现」——三者实际都已发布。

| 页面表述 | 实际状态 | 对外口径 |
| --- | --- | --- |
| 视频拆解（开场钩子 / 卖点 / 购买引导） | 已交付：`/pg/influencer/recreate/analyze` | 可写 |
| 视频复刻 | 已交付：复用 motion 生成线 | 可写 |
| 角色生成、多角度设定图、灵感社区 | 已交付 | 可写，属工作台能力 |
| 商品库 | 已交付：`omnimux-products` 插件，经已发布的 `omnimux-assets` 作为依赖装入 | 可写 |
| 视频剪辑 | 已交付：`omnimux-clip` 插件，在已发布插件集内 | 可写 |
| 发布中心 | 已交付：`omnimux-publish` 插件，在已发布 bundles 内 | 可写 |
| 画布 | 已交付：`omnimux-workflow` 插件（无限画布 / 节点编排），在已发布 bundles 内 | 可写 |
| TikTok 官方直连发布 | 已交付代码，但 `TIKTOK_DIRECT_ENABLED` 默认关闭，默认 `SELF_ONLY` 私密 | 须写明默认私密，不得说成公开直发 |
| 抖音 / 小红书 / 视频号 / 快手 / B 站发布 | **未实现**，网关与插件树均无对应实现 | 不得写成已支持 |
| 爆款趋势、竞品账号分析、品类趋势 | **未实现**，页面为静态示意数据 | 须标注为流程示意，不得说成实时数据 |
| 封面生成 | **未实现**，网关与插件树均无此能力 | 不得列为已交付能力 |
| 发布前自动检查 | 无独立预检实现；TikTok 直连路径未发送定时参数 | 不得作为已交付能力描述 |
| 定时发布（Zernio 排程） | 已交付：`service/social` Zernio 调度链路（海外 4 平台 TikTok / Instagram / Facebook / YouTube） | 可写为「已排期 / Scheduled」；不得扩展成「发布前已检查」 |

## 目标用户与表达

- 做跨境电商与内容电商、需要围绕商品卖点持续产出带货短视频与种草内容的卖家、创作者与团队。
- 需要固定 AI 角色风格内容的小团队或品牌；商品卖点与角色生成/视频复刻共同构成内容电商出片手段。
- 先讲“对标 → 拆解 → 制作 → 发布 → 复盘”这套桌面端内容电商工作台链路，再讲生成条件与费用，不用模型数量、供应商或技术接入方式定义产品。

消费界面不要求用户选择模型编号或渠道分组。模型、分组和线路是内部执行信息，技术接入文档仍须如实保留请求契约、权限和限制。业务收敛目标不等于所有旧技术入口已经删除，不得据此改写接口示例或擅自变更生产配置。

## 收费与事实核验

现行计费为 **按次消耗积分（预充值阶梯折扣加量包）**；生产环境中旧订阅套餐已停用（`fork: plans-disabled` / `fork: payg-topup-ladder`），代码保留可选订阅能力但对外主推按需充值、按次消耗积分。实际充值阶梯折扣与账户权益以价格区、钱包及当前实际启用配置共同核验；不在品牌文档硬编码旧套餐名、固定金额或模型清单。

角色生图有登录后报价，视频生成入口尚未显示逐次预估价，不能承诺所有生成前都有精确报价。失败任务的积分退回以实际任务状态和退款记录为准，不把积分退款说成订阅款或真实支付退款。免登录浏览、搭配与进入入口不等于免登录提交付费生成。

## 对外禁用说法

| 禁用 | 原因 |
| --- | --- |
| 从策划到发布复盘一切自动化 | 未交付的能力不得写成已自动完成；链路可讲，自动化的每一环须按上表核实 |
| 额度永不过期、只为成功成品付费、虚构已停用订阅套餐权益 | 不可替代实际充值阶梯、积分与扣费规则 |
| 把尚未放入公开仓的组件写成「已开源」；「整个服务开源」「可完整自建 omnimux.ai」 | 公开源码现按三端独立公开仓（AGPL-3.0）发布：网关 `omnimux-ai/omnimux-gateway`、桌面端 `omnimux-ai/omnimux-desktop`、插件 `omnimux-ai/omnimux-dsh`（历史汇总快照仓 `omnimux-ai/omnimux-ai` 已由三端独立公开仓接替）。只有源码已放入公开仓的组件可以称开源，放入前只能写「将以 AGPL-3.0 开源」；桌面端、插件与网关（连同内嵌落地页）源码已发布，可称开源；尚未放入的组件仍不得写成已开源。托管服务的渠道凭据、定价账本、运维与用户数据不开源。须保留 new-api 与 QuantumNous 的真实归属 |
| 真实顾客或真实用户评价（指 AI 出镜人） | 不得伪造证言或冒充真人 |
| 看不出是 AI、免标注 | 不得规避生成内容标注要求 |
| 保证爆款、保证流量、保证销售额 | 无法兑现 |
| 官方合作、官方认证 | 必须有已核验资质 |
| 抖音 / 小红书 / 视频号 / 快手 / B 站已支持发布 | 未实现；只有 TikTok 官方直连（默认关闭、默认私密）与 Zernio 代理的海外平台 |
| 多账号矩阵、防关联 | 不属于当前产品承诺 |
| 把静态示意数据写成实时趋势或客户成果 | 页面样例是流程示意，不是实时抓取，也不是客户案例 |

## 文件与权威关系

| 文件 | 用途 |
| --- | --- |
| [omnimux-brand-strategy.md](omnimux-brand-strategy.md) | 受众、任务、信息架构与转化路径 |
| [landing-page-copy.md](landing-page-copy.md) | 当前页面结构、文案真源、已知冲突与验证路径 |
| [platform-introduction.md](platform-introduction.md) | 可复用的中英文产品介绍 |
| [marketing-agent-context.md](marketing-agent-context.md) | 营销智能体按需加载的上下文与演示边界 |
| [positioning-migration-2026-09-08.md](positioning-migration-2026-09-08.md) | 历史迁移记录，不是现行要求 |

先读取本入口，再按任务读取具体文档和实际实现。历史研究、验收快照、旧方案与旧宣传片只能证明当时状态，不得用来恢复旧定位或扩大能力承诺；仍然有效的输入契约、计费安全与授权要求继续遵守。

## 发布与工作区边界

本仓维护落地页及文案源。公开文档是独立仓库，社区置顶是外部消息；本仓修改不代表它们已经更新。跨工作区写入与外部发布须有对应授权，生产发布须单独授权，合入文档不等于产品上线。
