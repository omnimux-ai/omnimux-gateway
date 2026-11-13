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
// fork: 简体中文 Agent 入口页文案。命令均取自 cli/README.md。

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: '安装 CLI',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: '装好后执行 omnimux help 验证；CLI 默认连 omnimux.ai。',
  },
  {
    title: '获取 API Key',
    body: '登录控制台，前往 API Keys 页面，点击「生成 API Key」并复制保存。',
    action: { label: '生成 API Key', href: '/keys' },
  },
  {
    title: '配置 API Key',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: '把上面这行加入你的 shell 配置文件（~/.zshrc 或 ~/.bashrc），重新加载 shell，然后重启你的 Agent。',
  },
]

const SKILL_STEP: AgentStep = {
  title: '安装 Skills',
  command: 'omnimux skill install --user',
  body: '先装 CLI，再执行这一行；skills 会装进 ~/.agents/skills，Agent 重启后即可调用。',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: '分析 TikTok 视频',
    body: '拆解任意 TikTok 的脚本、分镜、Hook 结构和转化逻辑。',
    mock: {
      type: 'checklist',
      rows: [
        { label: 'Hook formula', value: 'Problem → Solution' },
        { label: 'Structure', value: 'Hook → Value → CTA' },
        { label: 'Timestamps', value: '0:02 · 0:11 · 0:24' },
        { label: 'Script template', value: 'Ready', accent: true },
      ],
    },
  },
  {
    title: '生成短视频',
    body: '给定脚本或创意简报，自动生成一条 TikTok 风格的短视频。',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: '生成 AI 图片',
    body: '为营销、社交媒体或创意项目生成高质量图片。',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: '使用虚拟形象',
    body: '用照片生成数字形象，用于保持身份一致性的口播视频。',
    mock: { type: 'voice' },
  },
  {
    title: '视频增强',
    body: '一键去字幕、去水印、画质超分，把粗糙素材修复成可直接发布的高清视频。',
    mock: { type: 'chips', chips: ['去字幕', '去水印', '超分辨率'] },
  },
  {
    title: '发布 TikTok 视频',
    body: '通过 OmniMux Open API 与 CLI，在 AI Agent 中自动化发布 TikTok Shop 和普通视频或图文。',
    mock: {
      type: 'publish',
      rows: [
        {
          title: 'Restock drop — glow serum',
          meta: '@shopwave · Today, 6:30 PM',
          status: 'Published',
        },
        {
          title: 'Everyday tote bag reveal',
          meta: '@dailycarry · Tomorrow, 9:00 AM',
          status: 'Scheduled',
          tone: 'busy',
        },
      ],
      chips: ['TikTok Shop 内容', '普通视频与图文', 'Web、Open API 与 CLI'],
    },
  },
]

const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'AI 视频分析',
    body: '拆解参考视频的开场、台词与分镜，重点分析 TikTok 商品广告，再通过对话调整你的创作方向。',
    mock: { type: 'chips', chips: ['开场结构', '台词拆解', '分镜还原'] },
  },
  {
    title: 'TikTok 带货图文生成器',
    body: '从商品图生成封面、逐页文案与整套轮播图片，用 Skill 调整每一页的卖点和叙事顺序。',
    mock: { type: 'chips', chips: ['封面', '逐页文案', '整套轮播'] },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: '如何用 AI Agent 分析 TikTok 视频？',
    a: '装好 OmniMux CLI 与 skills 后，在 Agent 里粘贴 TikTok 链接说「分析这个视频」。Agent 会调用视频分析 skill 输出 Hook 公式、脚本结构与可复用模板。',
  },
  {
    q: '什么是 TikTok Hook？为什么重要？',
    a: 'TikTok Hook 是视频开头 1–3 秒决定观众留存的内容。OmniMux 的 Hook 分析会指出爆款视频的开场模式、节奏与触发点。',
  },
  {
    q: 'OmniMux 是什么？',
    a: 'OmniMux 是面向 TikTok 跨境卖家的 AI 角色生成与参考视频复刻平台，同时提供 Web、Open API 与 CLI/Skills 三种入口。',
  },
  {
    q: '什么是 Skills？',
    a: 'Skills 是 Agent 的能力插件。执行 omnimux skill install --user 后，skills 会装进 ~/.agents/skills，Agent 即可直接调用背后的工具与 API，不需要写代码。',
  },
  {
    q: '需要付费吗？',
    a: 'CLI 与 Skills 本身不额外收费，消耗与网站额度一致。Codex、Claude Code 等 Agent 各自定价由提供方决定。',
  },
  {
    q: '分析一个 TikTok 视频需要多久？',
    a: '通常在 30 秒到 1 分钟内完成，包括字幕转录、画面理解和结构化分析。',
  },
  {
    q: 'OmniMux CLI 可以发布 TikTok 视频吗？',
    a: '可以。通过 omnimux social 系列命令发布，需先连接对应账号并提供 provider。',
  },
  {
    q: '安装后不生效怎么办？',
    a: '先重启 Agent，再执行 omnimux skill status 确认安装位置；用 omnimux balance 验证 Key 是否有效。',
  },
]

const SHELL = {
  stepLabel: '安装步骤',
  compatTitle: '兼容所有主流 Agent',
  compatLead:
    '支持 Codex、Claude Code、Hermes、OpenClaw 等几乎所有主流 Agent。',
  flowLabels: {
    agents: '你的本地 AI Agent',
    hub: 'OmniMux CLI',
    caps: '拥有 OmniMux 的能力',
  },
  coreTitle: '核心能力',
  coreLead:
    '包含 OmniMux 最核心的能力：TikTok 爆款分析、视频发布、AI 视频生成、图片创作与虚拟形象制作。',
  copy: '复制',
  copied: '已复制',
  kindAria: 'Agent 入口',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: '一个 Key，解锁全部能力',
  ctaBody: '生成 Key 配置到你的 Agent，下一个 TikTok 爆款让 AI 帮你拆。',
  ctaButton: '生成 API Key',
  faqTitle: '常见问题',
}

export const zhCN: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS['zh-CN'].cli,
    title: 'OmniMux CLI — 在任意 AI 中分析 TikTok、发布视频与生成内容',
    description:
      '安装 OmniMux CLI，即可在 Codex、Claude Code、OpenClaw 等 Agent 中调用 TikTok 分析、AI 视频与图片生成，以及发布能力。',
    badge: 'AI Agent CLI for TikTok',
    h1: '一行指令，在任意 AI 使用 OmniMux 能力',
    subtitle: '分析 TikTok 视频，生成图片与视频，直接发布到 TikTok。',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: '一行安装 · 一个 Key',
    flowCaps: [
      { title: '看爆款 · 找灵感', body: '拆解任意 TikTok 的 Hook、结构与脚本' },
      { title: '做内容', body: 'AI 短视频、图片、数字人与视频增强' },
      { title: '发布 · 运营', body: 'TikTok Shop 与普通视频，定时发布' },
    ],
    cards: CORE_CARDS,
    tryTitle: '试试看',
    tryHeading: '从提示词到视频素材，一行搞定',
    tryBody:
      '把想法发给你的 AI Agent，它自动调用 OmniMux 完成生成——视频、分镜图，一气呵成。',
    tryPrompts: [
      {
        label: '分析视频结构',
        text: '帮我分析这条 TikTok 视频的结构：https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: '生成分镜图',
        text: '帮我生成一条美容仪 TikTok 带货视频的分镜，30 秒，强 hook 开场',
      },
      { label: '生成视频', text: '好，基于这套分镜生成视频' },
    ],
    tryChat: {
      prompt:
        '帮我分析这条 TikTok 视频的结构：https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: '问题 → 解决方案' },
        { label: '结构', value: 'Hook → 价值 → CTA' },
        { label: '时间轴', value: '0:02 · 0:11 · 0:24' },
        { label: '脚本模板', value: '可直接复用' },
      ],
      follow: '帮我生成一条美容仪 TikTok 带货视频的分镜，30 秒，强 hook 开场',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS['zh-CN'].skills,
    title: 'OmniMux Skills — 给任意 AI 装上 TikTok 爆款分析与短视频生成能力',
    description:
      '给 Codex、OpenClaw、Claude Code 等装上 OmniMux Skills，自动分析 TikTok 爆款、生成短视频。',
    badge: 'AI Agent Skills for TikTok',
    h1: '给任意 AI 装上 OmniMux Skills',
    subtitle:
      '给 Codex、OpenClaw、Claude Code 等装上 OmniMux Skills，自动分析 TikTok 爆款、生成短视频。',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: '一次安装 · 一个 Key',
    flowCaps: [
      { title: '看爆款 · 找灵感', body: '拆解任意 TikTok 的 Hook、结构与脚本' },
      { title: '仿制爆款', body: '以竞品或参考视频为模板，为你的产品重写一版' },
      { title: '做内容', body: 'AI 短视频、图片、数字人与视频增强' },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: '试试看',
    tryHeading: '一条链接，拆解 TikTok 爆款',
    tryBody:
      '复制下面的指令发送给你的 AI Agent，系统自动完成转录 → 画面理解 → 输出脚本与 Hook 分析。',
    tryPrompts: [
      {
        label: '分析这个视频',
        text: '分析这个视频：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: '复刻同款结构',
        text: '按这个结构，给我的精华液写一版 30 秒脚本',
      },
    ],
    tryChat: {
      prompt:
        '分析这个视频：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: '按这个结构，给我的精华液写一版 30 秒脚本',
    },
    faq: FAQ,
  },
}
