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
// fork: 繁體中文 Agent 入口頁文案。用詞沿用落地頁既有譯法（影片／圖片／下載）。

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: '安裝 CLI',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: '裝好後執行 omnimux help 驗證；CLI 預設連 omnimux.ai。',
  },
  {
    title: '取得 API Key',
    body: '登入控制台，前往 API Keys 頁面，點擊「產生 API Key」並複製保存。',
    action: { label: '產生 API Key', href: '/keys' },
  },
  {
    title: '設定 API Key',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: '把上面這行加入你的 shell 設定檔（~/.zshrc 或 ~/.bashrc），重新載入 shell，然後重啟你的 Agent。',
  },
]

const SKILL_STEP: AgentStep = {
  title: '安裝 Skills',
  command: 'omnimux skill install --user',
  body: '先裝 CLI，再執行這一行；skills 會裝進 ~/.agents/skills，Agent 重啟後即可呼叫。',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: '分析 TikTok 影片',
    body: '拆解任意 TikTok 的腳本、分鏡、Hook 結構和轉化邏輯。',
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
    title: '生成短影音',
    body: '給定腳本或創意簡報，自動生成一支 TikTok 風格的短影音。',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: '生成 AI 圖片',
    body: '為行銷、社群媒體或創意專案生成高品質圖片。',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: '使用虛擬形象',
    body: '用照片生成數位形象，用於保持身分一致性的口播影片。',
    mock: { type: 'voice' },
  },
  {
    title: '影片增強',
    body: '一鍵去字幕、去浮水印、畫質超分，把粗糙素材修復成可直接發布的高清影片。',
    mock: { type: 'chips', chips: ['去字幕', '去浮水印', '超解析度'] },
  },
  {
    title: '發布 TikTok 影片',
    body: '透過 OmniMux Open API 與 CLI，在 AI Agent 中自動化發布 TikTok Shop 和一般影片或圖文。',
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
      chips: ['TikTok Shop 內容', '一般影片與圖文', 'Web、Open API 與 CLI'],
    },
  },
]

const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'AI 影片分析',
    body: '拆解參考影片的開場、台詞與分鏡，重點分析 TikTok 商品廣告，再透過對話調整你的創作方向。',
    mock: { type: 'chips', chips: ['開場結構', '台詞拆解', '分鏡還原'] },
  },
  {
    title: 'TikTok 帶貨圖文生成器',
    body: '從商品圖生成封面、逐頁文案與整套輪播圖片，用 Skill 調整每一頁的賣點和敘事順序。',
    mock: { type: 'chips', chips: ['封面', '逐頁文案', '整套輪播'] },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: '如何用 AI Agent 分析 TikTok 影片？',
    a: '裝好 OmniMux CLI 與 skills 後，在 Agent 裡貼上 TikTok 連結說「分析這個影片」。Agent 會呼叫影片分析 skill 輸出 Hook 公式、腳本結構與可重複使用的範本。',
  },
  {
    q: '什麼是 TikTok Hook？為什麼重要？',
    a: 'TikTok Hook 是影片開頭 1–3 秒決定觀眾留存的內容。OmniMux 的 Hook 分析會指出爆款影片的開場模式、節奏與觸發點。',
  },
  {
    q: 'OmniMux 是什麼？',
    a: 'OmniMux 是面向 TikTok 跨境賣家的 AI 角色生成與參考影片複刻平台，同時提供 Web、Open API 與 CLI/Skills 三種入口。',
  },
  {
    q: '什麼是 Skills？',
    a: 'Skills 是 Agent 的能力外掛。執行 omnimux skill install --user 後，skills 會裝進 ~/.agents/skills，Agent 即可直接呼叫背後的工具與 API，不需要寫程式。',
  },
  {
    q: '需要付費嗎？',
    a: 'CLI 與 Skills 本身不額外收費，消耗與網站額度一致。Codex、Claude Code 等 Agent 各自定價由提供方決定。',
  },
  {
    q: '分析一支 TikTok 影片需要多久？',
    a: '通常在 30 秒到 1 分鐘內完成，包括字幕轉錄、畫面理解和結構化分析。',
  },
  {
    q: 'OmniMux CLI 可以發布 TikTok 影片嗎？',
    a: '可以。透過 omnimux social 系列指令發布，需先連接對應帳號並提供 provider。',
  },
  {
    q: '安裝後不生效怎麼辦？',
    a: '先重啟 Agent，再執行 omnimux skill status 確認安裝位置；用 omnimux balance 驗證 Key 是否有效。',
  },
]

const SHELL = {
  stepLabel: '安裝步驟',
  compatTitle: '相容所有主流 Agent',
  compatLead:
    '支援 Codex、Claude Code、Hermes、OpenClaw 等幾乎所有主流 Agent。',
  flowLabels: {
    agents: '你的本機 AI Agent',
    hub: 'OmniMux CLI',
    caps: '擁有 OmniMux 的能力',
  },
  coreTitle: '核心能力',
  coreLead:
    '包含 OmniMux 最核心的能力：TikTok 爆款分析、影片發布、AI 影片生成、圖片創作與虛擬形象製作。',
  copy: '複製',
  copied: '已複製',
  kindAria: 'Agent 入口',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: '一個 Key，解鎖全部能力',
  ctaBody: '產生 Key 設定到你的 Agent，下一個 TikTok 爆款讓 AI 幫你拆。',
  ctaButton: '產生 API Key',
  faqTitle: '常見問題',
}

export const zhTW: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS['zh-TW'].cli,
    title: 'OmniMux CLI — 在任意 AI 中分析 TikTok、發布影片與生成內容',
    description:
      '安裝 OmniMux CLI，即可在 Codex、Claude Code、OpenClaw 等 Agent 中呼叫 TikTok 分析、AI 影片與圖片生成，以及發布能力。',
    badge: 'AI Agent CLI for TikTok',
    h1: '一行指令，在任意 AI 使用 OmniMux 能力',
    subtitle: '分析 TikTok 影片，生成圖片與影片，直接發布到 TikTok。',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: '一行安裝 · 一個 Key',
    flowCaps: [
      { title: '看爆款 · 找靈感', body: '拆解任意 TikTok 的 Hook、結構與腳本' },
      { title: '做內容', body: 'AI 短影音、圖片、數位人與影片增強' },
      { title: '發布 · 營運', body: 'TikTok Shop 與一般影片，定時發布' },
    ],
    cards: CORE_CARDS,
    tryTitle: '試試看',
    tryHeading: '從提示詞到影片素材，一行搞定',
    tryBody:
      '把想法發給你的 AI Agent，它自動呼叫 OmniMux 完成生成——影片、分鏡圖，一氣呵成。',
    tryPrompts: [
      {
        label: '分析影片結構',
        text: '幫我分析這支 TikTok 影片的結構：https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: '生成分鏡圖',
        text: '幫我生成一支美容儀 TikTok 帶貨影片的分鏡，30 秒，強 hook 開場',
      },
      { label: '生成影片', text: '好，基於這套分鏡生成影片' },
    ],
    tryChat: {
      prompt:
        '幫我分析這支 TikTok 影片的結構：https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: '問題 → 解決方案' },
        { label: '結構', value: 'Hook → 價值 → CTA' },
        { label: '時間軸', value: '0:02 · 0:11 · 0:24' },
        { label: '腳本範本', value: '可直接重複使用' },
      ],
      follow: '幫我生成一支美容儀 TikTok 帶貨影片的分鏡，30 秒，強 hook 開場',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS['zh-TW'].skills,
    title: 'OmniMux Skills — 給任意 AI 裝上 TikTok 爆款分析與短影音生成能力',
    description:
      '給 Codex、OpenClaw、Claude Code 等裝上 OmniMux Skills，自動分析 TikTok 爆款、生成短影音。',
    badge: 'AI Agent Skills for TikTok',
    h1: '給任意 AI 裝上 OmniMux Skills',
    subtitle:
      '給 Codex、OpenClaw、Claude Code 等裝上 OmniMux Skills，自動分析 TikTok 爆款、生成短影音。',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: '一次安裝 · 一個 Key',
    flowCaps: [
      { title: '看爆款 · 找靈感', body: '拆解任意 TikTok 的 Hook、結構與腳本' },
      { title: '仿製爆款', body: '以競品或參考影片為範本，為你的產品重寫一版' },
      { title: '做內容', body: 'AI 短影音、圖片、數位人與影片增強' },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: '試試看',
    tryHeading: '一條連結，拆解 TikTok 爆款',
    tryBody:
      '複製下面的指令發送給你的 AI Agent，系統自動完成轉錄 → 畫面理解 → 輸出腳本與 Hook 分析。',
    tryPrompts: [
      {
        label: '分析這個影片',
        text: '分析這個影片：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: '複刻同款結構',
        text: '按這個結構，幫我的精華液寫一版 30 秒腳本',
      },
    ],
    tryChat: {
      prompt:
        '分析這個影片：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: '按這個結構，幫我的精華液寫一版 30 秒腳本',
    },
    faq: FAQ,
  },
}
