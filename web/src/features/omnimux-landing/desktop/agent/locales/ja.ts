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
// fork: 日本語 Agent 入口ページのコピー。用語は既存のランディングページに合わせる
// （動画／画像／ダウンロード）。コマンドは cli/README.md に実在するもののみ。

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: 'CLI をインストール',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: 'omnimux help で確認できます。CLI は既定で omnimux.ai に接続します。',
  },
  {
    title: 'API キーを取得',
    body: 'コンソールの API Keys ページでキーを作成し、一度だけコピーしてください。',
    action: { label: 'API キーを作成', href: '/keys' },
  },
  {
    title: 'API キーを設定',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: 'この行を ~/.zshrc か ~/.bashrc に追加し、シェルを再読み込みしてから Agent を再起動します。',
  },
]

const SKILL_STEP: AgentStep = {
  title: 'Skills をインストール',
  command: 'omnimux skill install --user',
  body: '先に CLI を入れてください。skills は ~/.agents/skills に入り、Agent の再起動後に使えます。',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: 'TikTok 動画を分析',
    body: '任意の TikTok の台本、カット割り、Hook の構成と転換ロジックを分解します。',
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
    title: 'ショート動画を生成',
    body: '台本やクリエイティブブリーフから、TikTok らしいショート動画を作ります。',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: 'AI 画像を生成',
    body: 'マーケティング、SNS、クリエイティブ制作向けの高品質な画像を生成します。',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: 'デジタルアバターを使う',
    body: '写真から一貫性のあるデジタル人物を作り、トーク動画に使えます。',
    mock: { type: 'voice' },
  },
  {
    title: '動画を強化',
    body: '字幕とウォーターマークを消し、画質を高めてそのまま公開できる状態にします。',
    mock: { type: 'chips', chips: ['字幕を消す', '透かしを消す', '高画質化'] },
  },
  {
    title: 'TikTok へ公開',
    body: 'Open API と CLI を通じて、TikTok Shop と通常の動画・画像投稿を Agent から公開します。',
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
      chips: [
        'TikTok Shop コンテンツ',
        '通常の動画と画像',
        'Web・Open API・CLI',
      ],
    },
  },
]

const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'AI 動画分析',
    body: '参考動画の導入、セリフ、カット割りを分解し、TikTok の商品広告を中心に分析して、対話で方向性を調整します。',
    mock: { type: 'chips', chips: ['導入の構成', 'セリフ分解', 'カット再現'] },
  },
  {
    title: 'TikTok カルーセル生成',
    body: '商品画像からカバー、ページごとのコピー、カルーセル一式を生成し、各ページの訴求点と構成を調整します。',
    mock: {
      type: 'chips',
      chips: ['カバー', 'ページコピー', 'カルーセル一式'],
    },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: 'AI Agent で TikTok 動画を分析するには？',
    a: 'OmniMux CLI と skills を入れて、Agent に TikTok のリンクを貼り「この動画を分析して」と伝えます。Agent が動画分析 skill を呼び、Hook の公式、台本の構成、再利用できるテンプレートを返します。',
  },
  {
    q: 'TikTok の Hook とは？なぜ重要ですか？',
    a: 'Hook は冒頭 1〜3 秒で視聴者の離脱を決める部分です。OmniMux は反響のあった動画の導入パターン、テンポ、引き金を特定します。',
  },
  {
    q: 'OmniMux とは？',
    a: 'OmniMux は TikTok 越境セラー向けの AI キャラクター生成と参考動画再現のプラットフォームで、Web コンソール、Open API、CLI と skills から利用できます。',
  },
  {
    q: 'Skills とは？',
    a: 'Skills は Agent が読み込める機能パックです。omnimux skill install --user を実行すると ~/.agents/skills に入り、コードを書かずに背後のツールと API を呼べます。',
  },
  {
    q: '追加料金はかかりますか？',
    a: 'CLI と skills 自体に追加料金はなく、消費はサイトと同じ残高から引かれます。Codex や Claude Code など各 Agent の料金は提供元が定めます。',
  },
  {
    q: '分析にはどのくらいかかりますか？',
    a: '通常 30 秒から 1 分ほどで、文字起こし、映像の理解、構造化された出力まで含みます。',
  },
  {
    q: 'CLI から TikTok 動画を公開できますか？',
    a: 'はい、omnimux social 系のコマンドで公開できます。先にアカウントを接続し、そのアカウントの provider を指定してください。',
  },
  {
    q: 'インストールしても動かない場合は？',
    a: 'Agent を再起動し、omnimux skill status で skills の場所を、omnimux balance でキーの有効性を確認してください。',
  },
]

const SHELL = {
  stepLabel: 'セットアップ',
  compatTitle: '主要な Agent すべてに対応',
  compatLead:
    'Codex、Claude Code、Hermes、OpenClaw など、ほとんどの Agent で使えます。',
  flowLabels: {
    agents: 'ローカルの AI Agent',
    hub: 'OmniMux CLI',
    caps: 'OmniMux の機能',
  },
  coreTitle: '主要な機能',
  coreLead:
    'TikTok 分析、公開、AI 動画、画像生成、デジタルアバターまで、OmniMux の中核機能をまとめています。',
  copy: 'コピー',
  copied: 'コピーしました',
  kindAria: 'Agent の入口',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: '1 つのキーで、すべての機能を',
  ctaBody:
    'キーを作成して Agent に設定すれば、次の TikTok の当たり作を AI が分解します。',
  ctaButton: 'API キーを作成',
  faqTitle: 'よくある質問',
}

export const ja: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS.ja.cli,
    title: 'OmniMux CLI — Agent から TikTok 分析・動画公開・AI 生成まで',
    description:
      'OmniMux CLI をインストールすると、Codex、Claude Code、OpenClaw などの Agent から TikTok 分析、AI 動画・画像生成、公開を呼び出せます。',
    badge: 'AI Agent CLI for TikTok',
    h1: '1 行のコマンドで、どの AI でも OmniMux の機能を',
    subtitle:
      'TikTok 動画を分析し、画像と動画を生成し、そのまま TikTok へ公開します。',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: 'インストール 1 回 · キー 1 つ',
    flowCaps: [
      {
        title: '当たり作を見つける',
        body: '任意の TikTok の Hook、構成、台本を分解',
      },
      {
        title: 'コンテンツを作る',
        body: 'AI 動画、画像、デジタルアバター、動画強化',
      },
      { title: '公開して運用する', body: 'TikTok Shop と通常動画を予約投稿' },
    ],
    cards: CORE_CARDS,
    tryTitle: '試してみる',
    tryHeading: 'プロンプトから映像素材まで 1 行で',
    tryBody:
      'アイデアを Agent に送るだけ。OmniMux を呼び出し、動画とカット画像をまとめて返します。',
    tryPrompts: [
      {
        label: '構成を分析',
        text: 'この TikTok の構成を分析して：https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: 'カット画像を生成',
        text: '美容デバイスの TikTok 販売動画の絵コンテを 30 秒で、強い Hook の入りで作って',
      },
      {
        label: '動画を生成',
        text: 'いいね、この絵コンテから動画を生成して',
      },
    ],
    tryChat: {
      prompt:
        'この TikTok の構成を分析して：https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: '課題 → 解決' },
        { label: '構成', value: 'Hook → 価値 → CTA' },
        { label: 'タイムライン', value: '0:02 · 0:11 · 0:24' },
        { label: 'テンプレート', value: '再利用可' },
      ],
      follow:
        '美容デバイスの TikTok 販売動画の絵コンテを 30 秒で、強い Hook の入りで作って',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS.ja.skills,
    title: 'OmniMux Skills — どの AI にも TikTok 分析とショート動画生成を',
    description:
      'Codex、OpenClaw、Claude Code などに OmniMux Skills を入れて、TikTok の当たり作分析とショート動画生成を自動化します。',
    badge: 'AI Agent Skills for TikTok',
    h1: 'どの AI にも OmniMux Skills を',
    subtitle:
      'Codex、OpenClaw、Claude Code などに OmniMux Skills を入れ、TikTok の当たり作を分析してショート動画を生成します。',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: 'インストール 1 回 · キー 1 つ',
    flowCaps: [
      {
        title: '当たり作を見つける',
        body: '任意の TikTok の Hook、構成、台本を分解',
      },
      {
        title: '当たり作を再現',
        body: '競合や参考動画をテンプレートに、自分の商品向けに作り直す',
      },
      {
        title: 'コンテンツを作る',
        body: 'AI 動画、画像、デジタルアバター、動画強化',
      },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: '試してみる',
    tryHeading: 'リンク 1 本で、まるごと分解',
    tryBody:
      '下のプロンプトを Agent に送ると、文字起こし、映像の読み取り、台本と Hook の分析まで返します。',
    tryPrompts: [
      {
        label: 'この動画を分析',
        text: 'この動画を分析して：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: '構成を再現',
        text: 'この構成で、私の美容液の 30 秒台本を書いて',
      },
    ],
    tryChat: {
      prompt:
        'この動画を分析して：https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: 'この構成で、私の美容液の 30 秒台本を書いて',
    },
    faq: FAQ,
  },
}
