# OmniMux 落地页文案与实现

现行产品边界以 [品牌规范](README.md) 为准。本文描述已合入的角色生成与视频复刻落地页；源码存在不等于该版本已部署，线上状态必须另行核验。

## 英文教程与搜索入口

首页角色步骤与第四条 FAQ 链接 `/guides/create-ai-character`，参考视频步骤与第五条 FAQ 链接 `/guides/recreate-video-with-your-character`，页脚增加 `/guides`。静态教程与应用之间使用普通整页链接。应用入口翻译七语言，文章正文与静态导航固定英文，并明确标注 English。

正文、摘要、审核日期、媒体引用和相关文章由 `web/src/features/guides/registry.ts` 维护，静态文档由同功能目录构建。服务端 `search-pages.json` 保留完整正文，浏览器元数据仅包含 head。网站地图从相同公开页清单生成；教程 CSS 经现有 Rsbuild 编译链生成。缺失正文时构建失败，运行时返回 `503 + noindex`。每篇带 `Step N` 标记的教程在 `Article` 旁输出 `HowTo` JSON-LD，封面图作为 `og:image`/`twitter:image`。

价格页继续是高级模型目录，且对搜索引擎公开（`index,follow`）：搜索标题为 `OmniMux Model Pricing | Generation Rates and Credits`，noscript 摘要说明可选订阅与按次付费并引用收费与失败退款 FAQ；目录本身仍由前端渲染，摘要不写死具体价格。桌面站该路径仍是 `noindex`。

教程浏览使用现有 Umami pageview；工作台按钮通过 `guide_studio_click` 记录文章 slug 与按钮位置，`/influencer` pageview 单独表示实际抵达。统计不可用时仍使用原生链接导航。发布验收须分别记录脚本加载、事件请求、后台入库、报表可见；代码通过不代替生产与后台验收。

## 当前首屏文案

| 位置 | 中文 | English |
| --- | --- | --- |
| 标题 | 让你的角色成为热门视频的主角 | Put your character in any trend |
| 描述 | 创建角色 挑选热门玩法 生成专属视频 玩法不断换 角色始终如一 | Create a character Pick a trend Generate the video Same face every video |
| 主动作 | 开始设计我的达人 | Design my influencer |
| 次动作 | 看看作品 | See the work |
| 演示关系 | 角色 + 任意视频 = 你的视频 | Character + Any video = Your video |

逐字真源为 [中文词条](../../web/src/i18n/locales/zh.json) 和 [英文词条](../../web/src/i18n/locales/en.json) 的 `translation` 命名空间，首屏使用 `influencer_landing.hero.*`。其余语言同目录维护，不能用已废弃词条、顶层重复键或旧设计稿恢复主标题。

## 页面结构与真实入口

| 顺序 | 区域 | 内容与边界 |
| --- | --- | --- |
| 顶栏 | 品牌、定价、灵感、登录或开始 | 定价到 `#pricing`，灵感到 `#works`；登录态开始到 `/influencer`，访客开始到 `/sign-up`；窄屏菜单保留对应目的地 |
| 首屏 | 角色与视频对照 | 主动作到 `/influencer`，次动作到 `#works`；两段视频同步静音循环，减少动态效果时显示海报 |
| 角色展示带 | 工作台角色样例 | 展示素材，不是实时生成或客户评价 |
| 使用说明 | 建立角色、多角度、参考动作、生成视频 | 说明创作路径，不证明社交平台发布已经接通 |
| 作品 | 特征造型与参考入口 | 选择造型后携参数进入工作台，更多作品到 `/influencer` |
| 多角度 | 设定图前后对照 | 展示角色参考图，不保证每次输出都完全一致 |
| 创建入口 | 文字描述、照片使用意图 | 保存意图后带入工作台；性别、风格与夸张度在工作台选择，照片在工作台上传；首页不生成也不上传 |
| 定价 | 体验包、入门版、创作版、月付年付与按次入口 | 月付和年付是可选套餐，按次付费仍存在；金额、额度与折扣须核验钱包和当前配置 |
| 常见问题 | 时长、收费、失败、访客、形象复用 | 时长是估计；失败退款指积分，不是自动退订阅款 |
| 页尾行动与页脚 | 开始创作、帮助与品牌 | 主动作到 `/influencer`；页脚不再提供 `/pricing` 入口（桌面端站点独占），顶部套餐锚点保留；页脚可切换语言；无原桌面下载区、安装卡或旧七段电商页面 |

## 源码与维护

- 页面入口：[index.tsx](../../web/src/features/omnimux-landing/index.tsx)。当前只装配 `components/influencer/` 与共享页头、页脚，不能按已删除的 `commerce-sections.tsx` 或 `components/commerce/` 维护。
- 首屏：[hero.tsx](../../web/src/features/omnimux-landing/components/influencer/hero.tsx)。样例素材：[influencer-looks.ts](../../web/src/features/omnimux-landing/lib/influencer-looks.ts) 与 `assets/influencer/`。
- 定价和问题：[pricing-faq-final.tsx](../../web/src/features/omnimux-landing/components/influencer/pricing-faq-final.tsx)。套餐名复用共享套餐模块，单张最低价读取实际价目；静态套餐金额不证明后台永远相同。
- 样式：[landing.css](../../web/src/features/omnimux-landing/landing.css) 提供深色中性变量与基础按钮，[influencer-landing.css](../../web/src/features/omnimux-landing/influencer-landing.css) 提供本页布局。视觉遵循 [DESIGN.md](../../DESIGN.md)，不恢复暖白旧首页。
- 搜索与爬虫面：[index.html](../../web/index.html)、[rsbuild.config.ts](../../web/rsbuild.config.ts)、[llms.txt](../../web/public/llms.txt)、[manifest.webmanifest](../../web/public/manifest.webmanifest) 是独立维护面；页面换了主张不代表这些静态描述已经一致。

## 文案收敛与剩余能力边界

角色页面文案已在修订源码中收敛；工作树预览通过不等于主干已合入或生产已发布。以下区分已清理文字与仍然存在的能力边界：

| 位置 | 源码状态或剩余边界 | 文档采用的口径 |
| --- | --- | --- |
| `influencer_landing.faq.a2` 与付费提示 | 七语言源码已清理无订阅承诺，未合入生产 | 可选订阅，同时支持不订阅按次付费 |
| 页尾与角色入口 | 七语言源码已清理 AI UGC 团队、照片直传和一键生成承诺，未合入生产 | 角色生成与参考视频复刻；首页保存创作意图，在工作台上传和生成 |
| 使用说明最后一步 | 七语言源码已改生成与下载，未合入生产 | 发布须独立授权和能力验收，不承诺全平台直发 |
| 积分退回提示与常见问题 | 七语言源码已明确生成积分及任务记录限制，未合入生产 | 生成积分退回以任务记录为准，不代表现金退款或所有失败无条件退回 |
| 搜索标题与静态爬虫介绍 | 源码已清理团队、网关、假版号与未经验证发布承诺，未合入生产 | 静态壳不等于可见首屏或部署证明；既有 JSON-LD 结构另任务处理 |
| 视频入口 | 未显示逐次预估报价，只有生图栏有报价 | 不能宣称所有生成前都能看到精确费用 |
| 四步模拟画面 | 源码已清理假分辨率、永久一致与自由回退承诺，演示标签为静态示例；未合入生产 | 工作台视频为 480p/720p，素材受限制，示例不证明可操作编辑能力 |
| 社区趋势 | 静态素材快照，不是实时抓取 | 不以热门措辞承诺实时更新或自有原创来源 |
| 独立 `/pricing` 价目页 | 计费宣传源码已清理，仍展示和复制技术模型编号；未合入生产 | 不把角色工作台自动选型写成全站已经隐藏模型编号，不从旧价目文案恢复消费手选 |

## 首页（omnimux.ai）与角色落地页（`/ai-influencer-studio`）

2026-10-27 起，桌面落地页（`web/src/features/omnimux-landing/desktop/`）成为 `omnimux.ai` 的首页，原角色落地页迁到 `/ai-influencer-studio` 并保持自己的文案与素材。首页顶栏显式提供 `/pricing` 与 `/inspiration`；`/pricing` 默认 `desktop-720p` 分组。首页恢复可收录（`index,follow` + canonical + sitemap），`d.omnimux.ai` 不再有独立收录面，它 301 到 `omnimux.ai` 后在过渡期结束时删除解析。下载主 CTA 固定 `https://dl.omnimux.ai/releases/latest/`。

迁移口径是**原样迁移**：页面结构、文案与六种语言（en / id / es / ja / zh-TW / zh-CN）不重写。首页的媒体沿用现有 `LazyVideo`（`preload='none'` + 进视口才加载），首屏以下的图片补 `loading='lazy'`。

### 首页能力口径（2026-10-27 已收敛并上线）

核对必须覆盖两个落点：网关（`router/` + `controller/` + `service/`）与桌面端插件（`omnimux-ai/omnimux-client` 的 `app/dsh-plugin-desktop/preset/skeleton/package.json` 中 `dsh.profile.bundles`，以及 `plugins/plugins/<name>/`）。

| 页面表述 | 事实 | 处理 |
| --- | --- | --- |
| 发布到抖音 / 小红书 / 视频号 / 快手 / B 站 | 无实现；`service/social` 只有 TikTok 官方直连与 Zernio 海外平台 | 已改为 TikTok / Instagram / Facebook / YouTube（#694） |
| 爆款趋势、竞品账号数据 | 静态示意数据，无接口 | 按钮改为「拆解一条参考视频」，面板标注流程示意（#700） |
| 封面生成 | 网关与插件树均无此能力 | 已从工具清单移除（#700） |
| 240+ 技能与专家、10 个内置成片应用 | `cli/skill/manifest.json` 17 条；`hero.reel` 样片 7 条 | 已改为 17 个内置技能、7 套成片模板（#700） |
| 平台数量标签 | 与徽标列表长度一致 | 已改为 4 个发布平台（#698） |
| 拆解面板竞品账号 / 品类趋势标签、`catMeta` 汇总 | 静态示意数据，无接口 | 标签补「· 示意 / Sample / Ejemplo / Contoh / サンプル」、`catMeta` 补「· 示例数据 / Sample data / サンプルデータ」，六语言同步 |
| 拆解卡 Douyin 链接与发布步骤平台圆点 | 抖音等平台未实现，非 TikTok 定位 | 链接改为 `tiktok.com/@pet.diary/video/73421`；`data.ts` 平台键由 `dy/xhs/sph/ks/bili` 改为 `tiktok/instagram/youtube`，`p-*` 圆点配色与 `agent-demo.tsx` 同步 |
| 发布 `feats`「发布前自动检查」卡、`schedOk`「已通过预检」、FAQ「几分钟内出片」与「检查封面/原创声明」 | 无独立预检实现；Zernio 排程为已交付能力 | `feats` 删卡（六语言各留 3 项）、`schedOk` 改「已排期 / Scheduled / Programado / Terjadwal / スケジュール済み」、FAQ 去掉时间承诺并改为「可添加 AI 生成内容声明」 |
| 商品库 / 画布 / 发布中心 | **已交付插件**：`omnimux-products`（经 `omnimux-assets` 装入）、`omnimux-workflow`、`omnimux-publish` | 保留，不得移除 |
| 视频拆解、视频复刻、角色生成、灵感社区 | 已交付 | 保留 |

本页此前的表格把商品库、画布、发布中心列为「无后端接口」，那是只搜 Go 仓得出的错误结论；2026-10-27 已按插件清单核实并更正。

## 维护与验证约定

对本页的后续改动必须同步受影响的品牌文档、有效翻译与搜索面；发现文档和真实页面不符时先记录差异，不依据历史要求改回页面。

仅文档改动检查差异、链接、路径和任务场景。页面行为改动按根规范执行前端验证及相关用例，落地页专项测试入口为：

```bash
cd web
bun run test --config src/features/omnimux-landing/components/__tests__/vitest.config.ts
```

浏览器验收覆盖首屏文案、桌面与窄屏菜单、造型带入、多角度展示、创作意图带入、套餐周期切换与登录后的目标页。不得提交真实支付，也不得用静态样例代替真实生成验收。

首页（桌面落地页）专项测试入口：

```bash
cd web
bun run test --config src/features/omnimux-landing/desktop/components/__tests__/vitest.config.ts
```

首页验收还须覆盖：`/` 返回桌面落地页、`/ai-influencer-studio` 返回角色落地页、首页 `index,follow` 与 canonical、`/pricing` 默认 `desktop-720p`、未登录访问 `/` 与登录后落回 `/`。
