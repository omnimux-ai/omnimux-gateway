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
import { docsQuickstartUrl } from '../../../lib/landing-urls'
import type { LandingCopy } from '../types'

export const ja: LandingCopy = {
  meta: { title: 'OmniMux デスクトップ — バズ動画を発見・再現・拡散' },
  skip: '本文へスキップ',

  nav: {
    capabilities: '機能',
    workflow: 'ワークフロー',
    inspiration: 'インスピレーション',
    downloadDesktop: 'デスクトップ版をダウンロード',
    menuClose: 'メニューを閉じる',
    brand: 'デスクトップ版',
    home: 'ホーム',
    models: 'モデル',
    tools: 'ツール',
    ext: '拡張機能',
    pricing: '料金',
    resources: 'リソース',
    download: '無料で使う',
    login: 'ログイン',
    menuOpen: 'メニューを開く',
    langTitle: 'ページの言語',
    brandAria: 'OmniMux デスクトップ版ホーム',
    navAria: 'メインナビゲーション',
    toolsMenu: {
      aria: 'ツールメニュー',
      group: 'クリエイティブ',
      video: 'TikTok 動画ダウンロード',
      photo: 'TikTok 画像ダウンロード',
      agentGroup: 'エージェント',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: 'リソースメニュー',
      inspiration: 'インスピレーションコミュニティ',
      docs: '開発者ドキュメント',
    },
    account: {
      aria: 'アカウントメニュー',
      dashboard: 'コンソール',
      profile: 'プロフィール',
      security: 'セキュリティとアクセス',
      wallet: 'ウォレット',
      systemSettings: 'システム設定',
      signOut: 'ログアウト',
    },
  },

  signOut: {
    title: 'ログアウト',
    body: 'ログアウトすると、アカウントに再度アクセスするには再ログインが必要です。',
    cancel: 'キャンセル',
    confirm: 'ログアウト',
    busy: 'ログアウト中…',
    failed: 'ログアウトに失敗しました',
    done: 'ログアウトしました',
    close: '閉じる',
  },

  hero: {
    eyebrow: '越境 EC とコンテンツ EC の販売者に',
    secondaryCta: '動画制作の流れを見る',
    mobileCopyLabel: 'ダウンロードリンクをコピー',
    mobileCopiedLabel: 'コピー済み · パソコンで開く',
    mobileHint:
      'デスクトップ版は macOS または Windows に対応しています。リンクをコピーし、パソコンのブラウザーで開いてください。',
    microTrust:
      'macOS 12+ / Windows 10 / 11 · ダウンロード無料 · 使った分だけの支払い、サブスクリプション不要',
    trustAria: '製品と対応プラットフォーム',
    trustStrip: ['オープンソース · AGPL-3.0', 'macOS + Windows'],
    mockAria: 'OmniMux デスクトップ版の画面プレビュー',
    mockBadge: '画面プレビュー',
    mockTitle: 'OmniMux デスクトップ版 · 新規チャット',
    mockSide: [
      '新規チャット',
      'アプリ',
      '商品ライブラリ',
      '素材ライブラリ',
      'インスピレーション',
      '投稿',
      'キャンバス',
      '編集',
    ],
    mockSidebarGroup: 'ワークスペース',
    mockUserAvatar: '林',
    mockPrompt:
      'この話題の動画を参考に、@美容液 の 15 秒動画を 3 本作って。冒頭で肌に塗ったときの仕上がりを見せて。',
    mockLinkCard:
      'tiktok.com/@pet.diary/video/73421 · 猫との日常、キャットフードの開封',
    mockSteps: [
      '参考動画の冒頭のフック、訴求点、購入への導線を分析',
      '商品の訴求点を確認：なじみが速く、モロモロが出にくい、乾燥肌向け',
      '3 本の台本と絵コンテを作成',
    ],
    mockTags: [
      'フック · 冒頭 2 秒で使用前後を比較',
      '訴求点 · 肌に塗る様子をアップで',
      '購入導線 · 最後に商品リンク',
    ],
    mockTileBadge: '生成済み',
    mockNote:
      '画面プレビュー · リアルタイム生成なし · ワークスペース内のサンプル素材',
    titleA: '発見・再現・拡散する',
    titleB: 'ECバズ動画',
    subPre: 'すべての動画に商品の魅力を反映できるコンテンツ EC システム。',
    subHighlight: '使い慣れた Agent',
    subPost:
      'で、参考動画の分析から予約投稿まで、広告・販売・口コミ動画を一か所で制作できます。',
    tabsAria: '制作モード',
    tabs: { agent: 'Agent', video: '動画', image: '画像' },
    inputLabel: '作りたい内容を入力',
    attachAria: '参考動画または画像を追加',
    sendAria: 'デスクトップで開始',
    dlMac: '無料で使う',
    dlWin: '無料で使う',
    dlMenuAria: 'ダウンロード版を選択',
    ghStar: 'GitHub で Star を付ける',
    dlMacArm: 'macOS（Apple シリコン）',
    dlMacIntel: 'macOS（Intel）',
    dlWinX64: 'Windows（64 ビット）',
    dlUnavailable: '安定版の公開中のため、現在ダウンロードできません',
    skillsLabel: 'スキル',
    skillsAria: 'スキル',
    skills: [
      {
        icon: 'scan',
        label: '動画・画像投稿を分析',
        fill: 'この動画がなぜ売れたのかを分析し、冒頭のフック・訴求ポイント・購入導線を整理 https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: '動画を再現',
        fill: '@美容液 でこのバズ動画を再現。テンポとカット構成を保ち、15 秒',
      },
      {
        icon: 'clap',
        label: '販売動画を制作',
        fill: '@美容液 の 15 秒販売動画を制作。冒頭で実際の使用感を見せる',
      },
      {
        icon: 'img',
        label: '販売画像投稿を再現',
        fill: 'この Instagram の画像投稿を参考に、@美容液 の 6 枚のおすすめ画像セットを作成',
      },
      {
        icon: 'wand',
        label: '動画をプロンプト化',
        fill: 'この動画を再利用できるプロンプトに分解し、カットごとに列挙',
      },
    ],
    modes: {
      agent: {
        prompts: [
          'この動画がなぜ売れたのかを分析 https://www.tiktok.com/@pet.diary/video/73421',
          'このバズ動画を @美容液 で 3 本作り、そのまま投稿できる形で 15 秒',
          '@小鹿のスキンケア日記 の直近 30 日の戦略を分析',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          '@美容液 の 15 秒販売動画を制作。冒頭で実際の使用感を見せる',
          'Bluetooth イヤホンの高所落下テスト。最後にタイムセールを告知',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          '@美容液 の 8 枚の商品画像セットを作成：メイン・訴求・ディテール・仕様・使用シーン',
          'この商品画像を大理石ドレッサーのシーンに置き換え',
        ],
      },
    },
    reelAria: '内蔵の完成動画テンプレート',
    reel: [
      { title: '3D パーティクル演出', tag: 'ペットフード' },
      { title: 'ランウェイ衣装チェンジ', tag: 'サロペット' },
      { title: '落下耐久テスト', tag: 'イヤホン' },
      { title: 'クリエイター自撮り紹介', tag: 'プロバイオティクス' },
      { title: '巨大商品チェイス', tag: 'ショッピングバッグ' },
      { title: 'モバイル Web デモ', tag: '旅行アプリ' },
      { title: '15 秒セール広告', tag: 'スナック' },
    ],
  },

  agentDemo: {
    h2: '1 回のチャットで、売れた動画を再現',
    lead: '売っている商品を伝えるだけ。参考動画の分析、台本づくり、コンテンツの生成、投稿予約までを行います。',
    windowAria: 'OmniMux デスクトップ Agent デモ',
    windowTitle: '私の 3 つのヒット商品でこのバズ動画を再現',
    windowStatus: '3 本の動画を生成しました',
    replay: 'デモを再生',
    replayAria: 'デモを再生',
    side: [
      '新規チャット',
      'アプリ',
      'キャンバス',
      'アセットライブラリ',
      'インスピレーション',
      '商品ライブラリ',
      '投稿',
      'データ',
    ],
    me: '林',
    msgPre: 'このバズ動画を使って',
    msgProducts: ['@ペットフード', '@サロペット', '@イヤホン'],
    msgPost: 'に 1 本ずつ作成し、そのまま投稿できる形で 15 秒。',
    linkCard:
      'tiktok.com/@pet.diary/video/73421 · 猫との日常、キャットフード開封',
    doneBreakdown: '参考動画が売れた理由を分析しました',
    kv: [
      { k: '冒頭', v: '最初の 2 秒でビフォーアフター' },
      { k: '証拠', v: '使用中のクローズアップ' },
      { k: '結び', v: '商品リンクへの誘導' },
    ],
    doneProducts: '3 つの商品と訴求ポイントを分析しました',
    doneScripts: '全商品の台本を作成しました',
    confirmTitle: '3 本の動画を生成しますか？',
    confirmNeed: '確認が必要です',
    confirmCost: '想定クレジット消費。確認後に引き落とされます',
    confirmGo: '確認して生成',
    confirmEdit: '修正',
    inputPh: 'OmniMux にメッセージ。@ で商品や素材を参照',
    resultsTitle: '完成動画',
    resultsMeta: '3 本 · 9:16',
    generating: '生成中…',
    vidNames: ['ペットフード', 'サロペット', 'イヤホン'],
    vidHooks: [
      '冒頭 · 概念対比',
      '冒頭 · 衣装チェンジ',
      '冒頭 · クローズアップ',
    ],
    schedTitle: '予約投稿',
    schedOk: 'スケジュール済み',
    schedPlatform: 'TikTok',
    schedTimes: ['火曜 19:00', '水曜 12:30', '木曜 19:00'],
    schedAll: 'すべて予約',
  },

  flow: {
    kicker: 'ワークフロー',
    lead: '参考探し、分析、制作、投稿、振り返りを一つのワークスペースで進めます。各工程の入力、結果、次の操作を確認できます。',
    h2: '商品アイデアから販売動画までの 5 ステップ',
    lineAria: '発見 · 分析 · 制作 · 投稿 · 振り返り',
    steps: [
      {
        kind: '発見',
        title: '同じカテゴリの参考動画を探す',
        desc: '見つけた動画をまとめ、同じカテゴリで注目を集める動画の構成を探します。',
        chips: ['バズ販売動画と画像投稿'],
        primary: '参考動画を分解する',
        secondary: 'ブラウザ拡張機能でインスピレーションを収集',
      },
      {
        kind: '分析',
        title: '再利用できる台本の構成を作る',
        desc: '冒頭のフック、商品の訴求点、購入への導線を分解し、再利用できる台本の構成にまとめます。',
        chips: ['台本の分析', '競合アカウント分析', 'カテゴリのトレンド'],
        primary: 'バズ動画を分析',
      },
      {
        kind: '制作',
        title: '商品やキャラクターの素材から制作する',
        desc: '商品画像やキャラクター素材で販売動画を制作・再現し、キャンバスとエディターで仕上げます。',
        primary: '動画を生成',
        secondary: 'バズ動画を再現',
      },
      {
        kind: '投稿',
        title: 'アカウントを連携して予約投稿する',
        desc: 'アカウントを連携し、TikTok、Instagram、Facebook、YouTube への投稿を予約します。',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: '動画を投稿',
        secondary: '動画投稿 · 画像投稿',
      },
      {
        kind: '振り返り',
        title: '投稿データを次の制作に生かす',
        desc: '投稿データをワークスペースに戻し、次の動画の分析と改善に使います。',
        primary: 'アカウントを分析',
      },
    ],

    discover: {
      title: '参考動画の例 · フロー示意',
      meta: '過去 7 日間',
      savesPrefix: '保存',
      items: [
        {
          who: '小桃のコーデ帳',
          views: '4.6万',
          length: '画像投稿 · 6 枚',
          title: 'ドットワンピ 1 枚で 3 コーデ、通勤もデートも #コーデ',
          likes: '4.6万',
          saves: '3.1万',
        },
        {
          who: '通勤10分メイク',
          views: '270万',
          length: '00:28',
          title: '車で片手メイク直しの神アイテム #通勤メイク',
          likes: '8.9万',
          saves: '2.4万',
        },
        {
          who: '乾燥肌ラボ',
          views: '160万',
          length: '00:34',
          title: '季節の変わり目の赤みに。スポイト美容液を実肌テスト #敏感肌',
          likes: '5.2万',
          saves: '1.8万',
        },
      ],
    },

    breakdown: {
      tabsAria: '分析対象',
      tabs: {
        video: '動画',
        account: '競合アカウント · サンプル',
        category: 'カテゴリのトレンド · サンプル',
      },
      statsViews: '1500万+ 再生',
      statsLikes: '32万+ いいね',
      why: '売れる理由',
      rows: [
        {
          label: '冒頭のフック',
          time: '0–2s',
          desc: '結果を先出し。商品の前に効果を見せる',
        },
        {
          label: '訴求ポイント',
          time: '3–9s',
          desc: '実用クローズアップで 3 つの訴求をナレーション',
        },
        {
          label: '購入導線',
          time: '10–14s',
          desc: 'タイムセールで商品リンクへ誘導',
        },
      ],
      reuse: '再利用できる構成',
      chain: ['結果先出し', '実用の証拠', '特典 + 商品リンク'],
      accountName: '@小鹿のスキンケア日記',
      accountMeta: 'ビューティー・パーソナルケア',
      thumbs: ['210万', '140万', '86万', '51万'],
      metrics: [
        { label: '投稿動画 · 過去 30 日', value: '42' },
        { label: '平均再生', value: '18.6万' },
        { label: '販売動画', value: '71%' },
      ],
      insights: [
        {
          k: '戦略',
          v: '再生トップ 10 のうち 7 本が「結果先出し」で始まり実用クローズアップへ切り替わる。このアカウントの主力パターンです。',
        },
        {
          k: 'タイミング',
          v: '19–21 時の投稿は他の時間帯より再生が約 40% 高い傾向。',
        },
        {
          k: '機会',
          v: '同カテゴリで「ビフォーアフター」×「価格アンカー」の組み合わせは未開拓。先に試す価値あり。',
        },
      ],
      accountCta: 'この戦略で @美容液 の動画を 3 本作る',
      catTitle: 'ビューティー・パーソナルケア',
      catMeta: '過去 7 日間 · 人気 200 本 · サンプルデータ',
      bars: [
        { label: 'ビフォーアフター', value: '38%' },
        { label: '開封・お試し', value: '24%' },
        { label: '実用クローズアップ', value: '19%' },
        { label: 'ナレーション紹介', value: '12%' },
        { label: 'ストーリー反転', value: '7%' },
      ],
      catInsights: [
        {
          k: '冒頭',
          v: '「ビフォーアフター」の冒頭は人気動画の 38% を占め、平均再生も最高。',
        },
        { k: '尺', v: '15–25 秒の動画が最多。40 秒超は大きく減少。' },
        {
          k: '機会',
          v: '「実用クローズアップ」×「タイムセール」の組み合わせはまだ少なく、新商品の切り口に最適。',
        },
      ],
      catCta: 'これをもとにベンチマーク案を作成',
    },

    make: {
      tabsAria: '制作方法',
      tabs: { generate: '直接生成', replicate: 'バズ動画を再現' },
      capProduct: '商品',
      capResult: '完成動画',
      capRef: '参考バズ動画',
      capMine: 'あなたの商品',
      badgeGenerated: '生成済み',
      badgeRef: '▶ TikTok · 210万',
      arrowReplicateA: '構成を再現',
      arrowReplicateB: '商品を差し替え',
      promptLabel: 'プロンプト',
      promptGenerate:
        'Bluetooth イヤホン：高所落下テスト、着地瞬間を追うカメラ、無傷、最後にタイムセールを告知。実写ナレーション風。',
      intentLabel: 'リライトの意図',
      promptReplicate:
        '参考動画の自撮りナレーションのテンポを維持：冒頭で悩み、中盤で手持ち実演、最後に本音の感想。再現するのは考え方であり、元動画ではありません。',
      altProduct: 'Bluetooth イヤホンの商品画像',
      altRef: '参考バズ動画のカバー',
      altCan: 'プロバイオティクスの商品画像',
    },

    publish: {
      title: '投稿計画',
      meta: '今週',
      posts: [
        {
          title: '止まらないポテチ、ドラマ鑑賞のお供',
          platformLabel: 'TikTok',
          account: '@スナック研究所',
          time: '今日 18:30',
          state: '投稿済み',
        },
        {
          title: '食いつきの悪い犬も奪い合うフード',
          platformLabel: 'TikTok',
          account: '@わんこのごはん',
          time: '今日 20:00',
          state: '予約済み',
        },
        {
          title: 'サロペット 1 枚で 1 週間コーデ',
          platformLabel: 'Instagram',
          account: '@コーデ日記',
          time: '明日 12:00',
          state: '準備完了',
        },
        {
          title: 'プロバイオティクス 1 か月の本音レビュー',
          platformLabel: 'YouTube',
          account: '@良いもの紹介所',
          time: '明日 19:00',
          state: '処理中',
        },
      ],
      feats: [
        {
          title: '複数プラットフォームへ一括投稿',
          desc: 'TikTok・Instagram・Facebook・YouTube。アカウントを選んで一度に投稿し、それぞれの状態を個別に追跡。',
        },
        {
          title: 'アカウントを一元管理',
          desc: 'デスクトップで各プラットフォームのアカウントを連携。投稿時に選ぶだけで、行き来は不要。',
        },
        {
          title: 'ステータス記録と再試行',
          desc: '各タスクのステータスを記録し、失敗したタスクは再試行できます。',
        },
      ],
    },

    review: {
      title: 'アカウントの成果',
      synced: '同期済み · たった今',
      meta: '過去 7 日間',
      kpis: [
        { label: '再生数', value: '12.8万', delta: '+18%' },
        { label: 'いいね', value: '8,640', delta: '+24%' },
        { label: 'エンゲージメント率', value: '5.2%', delta: '+0.8pt' },
      ],
      days: ['月', '火', '水', '木', '金', '土', '日'],
      splitTitle: '再生ソース · 冒頭タイプ別',
      splitLegs: ['ビフォーアフター冒頭 63%', 'ナレーション冒頭 37%'],
      note: '「ビフォーアフター」冒頭の動画が再生の 63% を稼ぎ、19–21 時の投稿が好調。',
      cta: '次のラウンド：このフックでさらに 3 本',
    },
  },

  create: {
    tag: '制作 Agent',
    h2: 'Agent に任せるか、自分でワークフローを組むか',
    lead: 'OmniMux Agent とのチャットでコンテンツを制作するか、Canvas で画像・動画・音声ツールをつなぎ、繰り返し使えるワークフローを構築できます。',
    msgProductCard: 'エナジードリンク · メイン画像',
    msgAsk: 'このエナジードリンクの商品画像セットを作って。',
    doneAnalyze: '商品を分析：形状・ラベル・配色',
    donePlan: '8 枚のセットを計画：メイン・訴求・ディテール・仕様・使用シーン',
    doneConsistent: '全画像で商品の見た目を統一',
    confirmTitle: '8 枚の画像を生成しますか？',
    confirmNeed: '確認が必要です',
    paramCount: '8 枚',
    confirmGo: '確認して生成',
    confirmEdit: '修正',
    doneResult: 'セット完成。8 枚をキャンバスに配置しました',
    tabsAria: 'ワークスペース',
    tabs: {
      canvas: 'キャンバス',
      assets: 'アセットライブラリ',
      inspiration: 'インスピレーション',
      products: '商品ライブラリ',
    },
    groupLabel: '商品画像セット · 8',
    setCaptions: [
      'メイン',
      '訴求',
      'ディテール',
      '仕様',
      'シーン',
      'シーン',
      'ムード',
      'ポスター',
    ],
    libAssets: [
      { label: 'キャラクター', sub: '小鹿 · 3 アングル' },
      { label: 'シーン', sub: 'オープンキッチン' },
      { label: 'スタイル', sub: 'ネオンサイバー' },
      { label: '小道具', sub: 'サロペット' },
    ],
    libInspiration: [
      { label: 'リップグロス色見本', sub: 'TikTok · 300万再生' },
      { label: '車内メイク直し', sub: 'TikTok · 270万再生' },
      { label: '美容液の実測', sub: 'Instagram · 1.8万保存' },
      { label: 'チェイス衝突', sub: '分析済み' },
    ],
    libProducts: [
      {
        name: 'エナジードリンク',
        sub: '0 糖 · 覚醒 · 冷涼感',
        count: '8 枚セット',
      },
      {
        name: 'イヤホン',
        sub: 'ノイキャン · 30 時間駆動 · 耐衝撃',
        count: '2 本の動画',
      },
      {
        name: 'プロバイオティクス',
        sub: '1 日 1 杯 · 個包装 · 携帯性',
        count: '1 本の動画',
      },
    ],
    capAgentTitle: '伝えるだけで、制作一式を Agent が完成',
    capAgentDesc:
      '商品画像から動画、台本まで。Agent がすべての工程を設計し、制作物全体で商品の見た目を統一します。',
    capAgentCta: '制作を開始',
    capCanvasTitle: 'キャンバス：視覚的に組める制作ワークフロー',
    capCanvasDesc:
      '画像・動画・ナレーションのノードをつないでワークフローを構築。各ステップを個別に調整して再実行できます。',
    capCanvasCta: 'キャンバスを開く',
    canvasAria: 'キャンバスワークフローの例',
    nodeImage: '画像',
    nodeText: 'テキスト',
    nodeTextBody: 'ネオン照明、暗い背景、缶のクローズアップ、氷のミストと水滴',
    nodeAIImage: 'AI 画像',
    nodeAIVideo: 'AI 動画',
    nodeCut: '編集・書き出し',
    nodeDone: '完了',
    nodeCutBodyA: '字幕・音楽を追加',
    nodeCutBodyB: '9:16 縦型で書き出し',
  },

  surfaces: {
    h2: 'いつもの場所で、そのまま使う',
    lead: '同じ Agent がデスクトップ、Codex、Claude Code、ブラウザで使えます。',
    desktop: {
      title: 'OmniMux デスクトップ',
      desc: '完全なワークスペース：キャンバス、素材、商品、すべてのツール。macOS と Windows に対応。',
      input: '3 種類の美容液それぞれに動画を 1 本…',
      cta: 'デスクトップ版をダウンロード',
    },
    cli: {
      title: 'Codex・Claude Code などの Agent',
      desc: 'OmniMux CLI をインストールすると、普段使っている Agent から動画・画像・分析ツールを利用できます。',
      cta: 'CLI をインストール',
    },
    ext: {
      title: 'ブラウザ拡張機能',
      desc: 'Web で見つけたバズ動画をワンクリックでインスピレーションに保存。デスクトップに戻ってすぐ分析できます。',
      capSave: 'インスピレーションに保存',
      capScan: '分析',
      toast: 'インスピレーションに保存しました',
      cta: 'アプリ内で拡張機能をインストール',
    },
  },

  tools: {
    h2: 'AI で販売動画・画像・ナレーション素材を生成',
    lead: '商品素材の制作、マルチプラットフォーム向けコンテンツ、既存動画の再活用を、1 つのワークスペースで。',
    centerLabel: 'コンテンツ制作',
    groups: [
      {
        title: '商品リスティング',
        lead: '商品画像を出品レベルに',
        items: [
          { icon: 'img', label: '白背景の商品画像' },
          { icon: 'layers', label: 'マルチアングル商品画像' },
          { icon: 'grid', label: '商品画像セット' },
          { icon: 'box', label: '商品詳細画像' },
          { icon: 'spark', label: 'シーン画像' },
          { icon: 'wand', label: '参考画像からアレンジ' },
        ],
      },
      {
        title: '販売動画',
        lead: 'ゼロから生成、またはヒット動画を参考に',
        items: [
          { icon: 'link', label: '商品から動画' },
          { icon: 'video', label: '動画生成' },
          { icon: 'flame', label: 'バズ動画の再現' },
          { icon: 'wand', label: '動画をプロンプト化' },
          { icon: 'grid', label: '絵コンテ台本' },
          { icon: 'img', label: 'ファーストフレームから動画' },
        ],
      },
      {
        title: '画像投稿で販売',
        lead: '画像セットでも売れます',
        items: [
          { icon: 'layers', label: '画像カルーセル' },
          { icon: 'scan', label: '参考画像のリメイク' },
          { icon: 'img', label: '画像生成' },
          { icon: 'shirt', label: 'バーチャルトライオン' },
        ],
      },
      {
        title: 'ナレーション・出演',
        lead: '声、キャラクター、出演者',
        items: [
          { icon: 'user', label: 'デジタルヒューマン' },
          { icon: 'users', label: 'マルチアングルキャラクター' },
          { icon: 'mic', label: 'ナレーション・効果音' },
          { icon: 'music', label: 'AI 音楽' },
        ],
      },
      {
        title: 'マルチプラットフォーム対応',
        lead: '1 セットの素材を各プラットフォームに最適化',
        items: [
          { icon: 'crop', label: 'アスペクト比調整' },
          { icon: 'hash', label: 'タイトル・ハッシュタグ' },
        ],
      },
      {
        title: '素材の二次加工',
        lead: '既存の動画素材を編集・分割・字幕焼き付け。',
        items: [
          { icon: 'cut', label: '動画編集' },
          { icon: 'scan', label: 'シーン分割' },
          { icon: 'subs', label: '字幕焼き付け' },
        ],
      },
    ],
    modelsTitle: '売ることに最適化されたトップ AI モデル',
    modelsAria: '利用可能なモデル',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: '動画' },
      { mark: 'S', name: 'Seedance 2.0', kind: '動画' },
      { mark: 'G', name: 'GPT Image 2.5', kind: '画像' },
      { mark: 'N', name: 'Nano Banana', kind: '画像' },
      { mark: '♪', name: 'Suno', kind: '音楽' },
      { mark: 'G', name: 'Gemini', kind: '理解と台本' },
    ],
    chipsAria: 'モデルの機能',
    capabilityRows: [
      [
        { icon: 'monitor', label: '480p / 720p' },
        { icon: 'crop', label: '9:16' },
        { icon: 'crop', label: '16:9' },
        { icon: 'crop', label: '1:1' },
        { icon: 'crop', label: '3:4' },
        { icon: 'crop', label: '4:3' },
        { icon: 'clock', label: '4–30 秒の出力' },
      ],
      [
        { icon: 'layers', label: 'マルチモーダル参照' },
        { icon: 'user', label: 'キャラクターの一貫性' },
        { icon: 'img', label: 'ファーストフレーム制御' },
        { icon: 'music', label: 'ネイティブ音声' },
        { icon: 'spark', label: '商品の一貫性' },
      ],
      [
        { icon: 'cut', label: 'ローカル編集' },
        { icon: 'scan', label: 'シーン分割' },
        { icon: 'subs', label: '字幕焼き付け' },
        { icon: 'mic', label: '音声抽出' },
        { icon: 'layers', label: '画像投稿を動画化' },
      ],
    ],
  },

  workspace: {
    h2: '1 つのワークスペースに、素材を全部',
    lead: '商品・素材・インスピレーション・投稿履歴が 1 つのデスクトップに。いつでも使えて、探し回る必要はありません。',
    list: [
      { name: '商品ライブラリ', desc: '訴求ポイント・ターゲット・商品画像' },
      {
        name: 'アセットライブラリ',
        desc: 'キャラクター・シーン・スタイル・プロンプト',
      },
      { name: 'インスピレーション', desc: '保存したバズ動画と分析結果' },
      { name: '投稿センター', desc: '下書き・予約・投稿履歴' },
    ],
    note: 'macOS と Windows に対応。',
    tableTitle: '商品ライブラリ',
    addProduct: '商品を追加',
    cols: ['商品', '訴求ポイント', '今月のコンテンツ'],
    rows: [
      { name: 'エナジードリンク', category: '食品・飲料', points: '3' },
      { name: 'イヤホン', category: 'デジタル', points: '4' },
      { name: 'サロペット', category: 'アパレル', points: '3' },
      { name: 'プロバイオティクス', category: '健康食品', points: '2' },
    ],
  },

  capabilities: {
    kicker: '機能',
    h2: 'すべての動画に商品の魅力を反映',
    lead: '商品ライブラリから投稿ハブまで、素材、キャラクター、ツールを一つの制作フローで扱えます。',
    cards: [
      {
        title: '商品ライブラリ',
        body: '商品画像、訴求点、対象顧客、素材を一か所で管理します。Agent は制作のたびにその情報を参照できます。',
      },
      {
        title: '参考動画の分析と再現',
        body: '冒頭のフック、訴求点、購入への導線を分析し、独自の商品やキャラクターで構成を再現します。',
      },
      {
        title: 'キャラクターの一貫性と多視点',
        body: '同じ AI キャラクターと多視点の設定画像を複数の動画で使用できます。見た目の一貫性は入力画像と生成条件によって変わります。',
      },
    ],
  },

  proof: {
    kicker: '越境 EC の販売者に',
    source:
      'オープンソース · AGPL-3.0 · デスクトップ版とプラグイン、ゲートウェイは GitHub で個別に公開',
    h2: '参考探しから分析、制作、投稿まで一つのデスクトップアプリで',
    lead: '内蔵スキルと動画テンプレートでコンテンツを用意し、投稿ハブで投稿を予約できます。',
    facts: [
      { value: '17', label: '内蔵スキル' },
      { value: '7', label: '完成動画テンプレート' },
      { value: '4', label: '投稿プラットフォーム' },
    ],
    platformsAria: '対応する投稿プラットフォーム',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: [
      '生成前にパラメーターとクレジットを確認',
      '失敗したタスクのクレジット返還はタスク履歴に準拠',
    ],
  },

  pricingTeaser: {
    kicker: '料金',
    h2: '使った分だけ支払い、サブスクリプション不要',
    lead: 'クレジットをチャージし、生成するたびに消費します。最初の動画は少額から始め、必要に応じて追加できます。実際の金額、付与量、割引は料金ページとウォレットでご確認ください。',
    ctaPrimary: '料金の詳細を見る',
    ctaSecondary: 'ダウンロードしてからチャージ',
    cardSubtitle: '生成ごとに課金 · クレジットは全モデルで利用可能',
    cardPriceTitle: '{credits} クレジット',
    cardPriceSub: '定額チャージ {amount} から',
    tierCredits: '{credits} クレジット',
    tierSave: '{percent}% 割引',
    cardCta: 'チャージ金額の一覧を見る',
    cardFootnote: '1 クレジット = {price} · 割引は所定のチャージ金額のみ',
  },

  faq: {
    kicker: 'よくある質問',
    h2: 'EC 販売者からよく寄せられる質問',
    lead: 'ソースの公開範囲、投稿先、生成料金について。',
    items: [
      {
        q: 'OmniMux はオープンソースですか？',
        a: 'はい。デスクトップクライアントとプラグインは github.com/omnimux-ai/omnimux-client、ゲートウェイのソースは github.com/omnimux-ai/omnimux-gateway で AGPL-3.0 のもと公開しています。ホスト版のチャネル認証情報、価格台帳、運用情報、ユーザーデータは公開範囲外です。',
      },
      {
        q: 'ダウンロード後すぐに生成できますか？',
        a: 'ダウンロードしてインストールし、ログインするとワークスペースを利用できます。生成に使うクレジットはウォレットとタスク履歴で確認できます。生成前にパラメーターとクレジットの消費量を確認します。',
      },
      {
        q: 'どのプラットフォームに投稿できますか？',
        a: '投稿ハブから TikTok、Instagram、Facebook、YouTube に予約投稿できます。TikTok 公式の直接連携は初期設定では無効で、公開範囲の初期値は SELF_ONLY です。抖音、小紅書、微信動画号、快手、Bilibili には現在対応していません。',
      },
      {
        q: '動画を生成する前に正確な料金が分かりますか？',
        a: '画像生成画面には参考料金が表示されます。動画の料金はタスク履歴と実際の請求額をご確認ください。生成前にパラメーターとクレジットを確認します。具体的な料金はウォレットの設定に従います。',
      },
      {
        q: '動画がバズることは保証されていますか？',
        a: 'OmniMux では参考動画の分析、再現、制作に必要なツールを提供しています。再生回数、売上、拡散は保証の対象外です。アカウントの安全性は投稿する内容やアカウントの使い方にも左右されます。',
      },
    ],
  },

  finalCta: {
    h2: '次のバズ動画は、OmniMux にお任せ',
    lead: 'ヒットの分析・動画生成・投稿・振り返りまで、1 つの Agent がフルサイクルで回します。',
    dlMac: 'macOS 版をダウンロード',
    dlWin: 'Windows 版をダウンロード',
    note: 'macOS 12+（Apple Silicon・Intel 共通）· Windows 10 / 11 x64',
  },

  download: {
    title: 'OmniMux デスクトップで続ける',
    desc: 'ダウンロード・インストールしてログインすれば、すぐ制作を始められます。',
    version: '安定版 {version}',
    close: '閉じる',
    macArmTitle: 'macOS 版をダウンロード（Apple Silicon）',
    macArmSub: 'macOS 12+ · M シリーズチップ',
    macIntelTitle: 'macOS 版をダウンロード（Intel）',
    macIntelSub: 'macOS 12+ · Intel チップ',
    winTitle: 'Windows 版をダウンロード（x64 インストーラー）',
    winSub: 'Windows 10 / 11 x64',
    recommended: 'おすすめ',
    unavailable: 'ダウンロードできません：安定版がまだ公開されていません',
    notePre: 'Windows に「PC が保護されました」と表示されたら',
    noteMore: '詳細情報',
    noteRun: '実行',
    notePost: 'をクリック。dl.omnimux.ai からのみダウンロードしてください。',
  },

  footer: {
    brand: 'デスクトップ版',
    tagline: 'バズ動画を発見・再現・拡散。',
    columns: [
      {
        title: '探索',
        links: [
          { label: '料金', href: '/pricing' },
          { label: 'インスピレーション', href: '/inspiration' },
          {
            label: 'AI インフルエンサースタジオ',
            href: '/ai-influencer-studio',
          },
          { label: 'デスクトップ版をダウンロード', href: '#download' },
        ],
      },
      {
        title: 'ツール',
        links: [
          {
            label: 'TikTok 動画ダウンローダー',
            href: '/tools/tiktok-video-downloader',
          },
          {
            label: 'TikTok 写真ダウンローダー',
            href: '/tools/tiktok-photo-downloader',
          },
          { label: '商品から動画', href: '#capabilities' },
          { label: 'バズ動画の再現', href: '#capabilities' },
          { label: '動画をプロンプト化', href: '#capabilities' },
          { label: '商品画像セット', href: '#capabilities' },
          { label: '商品詳細画像', href: '#capabilities' },
          { label: '画像カルーセル', href: '#capabilities' },
          { label: 'デジタルヒューマン', href: '#capabilities' },
          { label: '字幕焼き付け', href: '#capabilities' },
        ],
      },
      {
        title: 'モデル',
        links: [
          { label: 'Seedance 2.5', href: '/pricing' },
          { label: 'Seedance 2.0', href: '/pricing' },
          { label: 'GPT Image 2.5', href: '/pricing' },
          { label: 'Nano Banana', href: '/pricing' },
          { label: 'Suno', href: '/pricing' },
        ],
      },
      {
        title: 'リソース',
        links: [
          {
            label: 'ドキュメント',
            href: docsQuickstartUrl('ja'),
            external: true,
          },
          { label: 'ガイド', href: '/guides', external: true },
          {
            label: 'ステータス',
            href: 'https://status.omnimux.ai',
            external: true,
          },
          {
            label: 'llms.txt',
            href: 'https://omnimux.ai/llms.txt',
            external: true,
          },
        ],
      },
      {
        title: '会社情報',
        links: [
          { label: 'OmniMux について', href: '/about' },
          {
            label: 'お問い合わせ',
            href: 'mailto:hello@omnimux.ai',
            external: true,
          },
          { label: 'プライバシーポリシー', href: '/privacy' },
          { label: '利用規約', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: 'ソースコード',
  },

  toolPage: {
    crumbAria: 'パンくずリスト',
    crumbTools: 'ツール',
    kindAria: 'コンテンツの種類',
    kindVideo: '動画',
    kindPhoto: '写真',
    inputLabel: 'TikTok リンク',
    placeholderVideo: 'TikTok 動画のリンクまたは共有テキストを貼り付け',
    placeholderPhoto: 'TikTok 写真投稿のリンクまたは共有テキストを貼り付け',
    paste: '貼り付け',
    parse: '解析',
    parsing: '解析中',
    verifying: '確認中',
    quotaAnon: '本日 {remaining}/{limit}',
    quotaUser: '本日 {remaining}/{limit}',
    turnstileAria: 'Cloudflare 認証',
    unavailable: '一時的に利用できません',
    downloadMp4: 'MP4 をダウンロード',
    recreateMine: '再現する',
    durationRange: '参考動画は 4〜30 秒にしてください（現在 {duration}）',
    breakdown: 'バズの構成を分析',
    photoCount: '{count} 枚',
    imageAlt: '{n} 枚目',
    downloadImage: '{n} 枚目をダウンロード',
    downloadZip: 'まとめて ZIP でダウンロード',
    zipLocked: 'ログインで利用可',
    recreatePhotos: 'この写真セットを再現',
    ctaTitle: '参考を保存して、自分のキャラクターで再現',
    ctaBody:
      '同じテンポとカットで、あなたの AI キャラクターと商品に置き換えます。',
    ctaStart: '再現をはじめる',
    ctaRef: '参考',
    ctaMine: '再現',
    relatedAria: '関連ツール',
    errors: {
      invalidLink: 'TikTok のリンクを貼り付けてください',
      wrongKindPhoto: 'これは写真投稿のリンクです',
      toPhoto: '写真ダウンローダーへ',
      wrongKindVideo: 'これは動画のリンクです',
      toVideo: '動画ダウンローダーへ',
      unavailable: 'この投稿は非公開・削除済み、または利用できません',
      serviceUnavailable: 'このサイトでは解析を公開していません',
      upstreamFailed:
        'TikTok が応答しません。しばらくしてから再試行してください',
      quotaAnon: '本日の回数を使い切りました。ログインして続けてください',
      quotaUser: '本日の回数を使い切りました。明日またどうぞ',
      busy: '本日のゲスト枠が上限に達しました。ログインして続けてください',
      rateLimited: '操作が速すぎます。少し待ってから再試行してください',
      turnstileFailed: '認証に失敗しました。もう一度お試しください',
      turnstileUnavailable:
        '認証が一時的に利用できません。しばらくしてからお試しください',
      generic:
        'このリンクを解析できませんでした。しばらくしてからお試しください',
    },
  },
}
