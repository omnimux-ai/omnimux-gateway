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
import { TOOL_PATHS, type ToolKind, type ToolPageContent } from '../paths'

const JA_FAQ: ToolPageContent['faq'] = [
  {
    q: '登録やアプリのインストールは必要ですか？',
    a: '不要です。公開リンク 1 件をブラウザ上で直接解析でき、初回のみ Cloudflare 認証を行います。写真投稿をまとめて ZIP 保存する場合はログインが必要です。',
  },
  {
    q: '1 日何回まで利用できますか？',
    a: '不正利用を防ぐため、1 日あたりの利用回数には上限があり、毎日 00:00 UTC にリセットされます。ログインすると、より高い日次上限を利用できます。',
  },
  {
    q: 'どのリンクに対応していますか？',
    a: '公開されている TikTok 動画および写真投稿リンク（vm.tiktok.com や vt.tiktok.com の短縮 URL、共有テキスト内のリンクを含む）に対応しています。非公開、友達限定、削除済み、地域制限付きの投稿は解析できません。',
  },
  {
    q: '保存されるファイル形式は？',
    a: '動画は MP4、写真投稿は TikTok が配信する元の画像形式（JPEG または WebP）で投稿順に番号付けされます。',
  },
  {
    q: '保存した素材を商用利用や再投稿できますか？',
    a: '著作権は元のクリエイターに帰属します。利用権限のある素材のみを保存し、参考として扱う際は出典を明記してください。商用可能な素材が必要な場合は OmniMux でご自身のキャラクターから再生成してください。',
  },
  {
    q: '解析したリンクや動画は保存されますか？',
    a: '不正利用防止のため、回数制限とキャッシュ目的でリクエスト数と投稿 ID のみを短期間記録します。動画ファイル本体は保存しません。詳細はプライバシーポリシーをご確認ください。',
  },
]

const JA_TRANSCRIPT_FAQ: ToolPageContent['faq'] = [
  {
    q: '抽出される字幕データはどこから取得していますか？',
    a: '公開 TikTok 動画に含まれる配信済みの WebVTT 字幕トラックを直接読み取ります。作成者字幕と TikTok 自動字幕の両方に対応しています。',
  },
  {
    q: '字幕が見つからない動画があるのはなぜですか？',
    a: '音楽のみの動画、投稿者が字幕をオフにしている動画、写真スライド投稿には TikTok 上に字幕トラックがありません。その場合も投稿テキストのコピーや MP4 の保存を行えます。',
  },
  {
    q: '登録やソフトウェアのインストールは必要ですか？',
    a: '不要です。公開リンクをブラウザ上で直接解析でき、初回のみ Cloudflare 認証を行います。',
  },
  {
    q: '1 日何回まで抽出できますか？',
    a: '不正利用を防ぐため、1 日あたりの利用回数には上限があり、TikTok ツール全体で日次上限を共有します。00:00 UTC にリセットされ、ログインするとより高い上限を利用できます。',
  },
  {
    q: '対応している書き出し形式は？',
    a: 'タイムスタンプ付きまたはプレーンテキストの直接コピーに加え、標準の TXT、SRT、VTT 字幕ファイルをダウンロードできます。',
  },
  {
    q: '解析した動画や字幕をサーバーに保管しますか？',
    a: '動画ファイルは保管しません。悪用防止と負荷軽減のため、投稿メタデータのみ最大 30 分間キャッシュし、日次カウントを記録します。詳細はプライバシーポリシーをご覧ください。',
  },
]

export const ja: Record<ToolKind, ToolPageContent> = {
  video: {
    path: TOOL_PATHS.ja.video,
    title: 'TikTok 動画保存 — MP4 | OmniMux',
    description:
      '公開 TikTok 動画のリンクを貼り付けてブラウザで直接解析し MP4 として保存。構成を参考に OmniMux で自社キャラクター動画へ再制作できます。',
    h1: 'TikTok 動画ダウンローダー',
    subtitle: '公開 TikTok 動画のリンクを貼り付けて MP4 として保存',
    stepsTitle: '3 ステップで TikTok 動画を保存',
    steps: [
      {
        title: 'リンクをコピー',
        body: 'TikTok 動画で「共有」→「リンクをコピー」をタップ。短縮 URL と通常 URL の両方に対応します。',
      },
      {
        title: '貼り付けて解析',
        body: '上の入力欄にリンクを貼り付けて「解析」をクリックします。',
      },
      {
        title: 'プレビューして保存',
        body: '内容を確認し MP4 を端末に保存します。',
      },
    ],
    faqTitle: 'よくある質問',
    faq: JA_FAQ,
    related: [
      { label: 'TikTok 写真ダウンローダー', href: TOOL_PATHS.ja.photo },
      { label: 'TikTok 字幕抽出', href: TOOL_PATHS.ja.transcript },
      { label: 'AI インフルエンサースタジオ', href: '/ai-influencer-studio' },
    ],
  },
  photo: {
    path: TOOL_PATHS.ja.photo,
    title: 'TikTok 写真保存 — 画像保存 | OmniMux',
    description:
      '公開 TikTok 写真投稿の元画像を投稿順そのままにブラウザで保存。ログインすれば ZIP で一括保存でき、OmniMux で動画化にも展開できます。',
    h1: 'TikTok 写真ダウンローダー',
    subtitle: '公開 TikTok 写真投稿のリンクから元画像を投稿順に保存',
    stepsTitle: '3 ステップで TikTok 写真を保存',
    steps: [
      {
        title: '投稿リンクをコピー',
        body: 'TikTok の写真投稿で「共有」→「リンクをコピー」をタップします。',
      },
      {
        title: '貼り付けて解析',
        body: '上の入力欄に貼り付けて「解析」をクリックします。',
      },
      {
        title: '個別または ZIP で保存',
        body: '1 枚ずつ保存するか、ログインしてすべてを ZIP でまとめて保存できます。',
      },
    ],
    faqTitle: 'よくある質問',
    faq: JA_FAQ,
    related: [
      { label: 'TikTok 動画ダウンローダー', href: TOOL_PATHS.ja.video },
      { label: 'TikTok 字幕抽出', href: TOOL_PATHS.ja.transcript },
      { label: 'AI インフルエンサースタジオ', href: '/ai-influencer-studio' },
    ],
  },
  transcript: {
    path: TOOL_PATHS.ja.transcript,
    title: 'TikTok 字幕抽出 — TXT/SRT | OmniMux',
    description:
      '公開 TikTok 動画のリンクからタイムスタンプ付き文字起こしと字幕を抽出し、テキストコピーや TXT・SRT・VTT 書き出しをブラウザで行えます。',
    h1: 'TikTok 字幕抽出ツール',
    subtitle:
      '公開 TikTok 動画のリンクからタイムスタンプ付き字幕やテキストを抽出',
    stepsTitle: '3 ステップで TikTok 字幕を抽出',
    steps: [
      {
        title: '動画リンクをコピー',
        body: 'TikTok 動画で「共有」→「リンクをコピー」をタップするか、共有テキストを貼り付けます。',
      },
      {
        title: '字幕を読み込む',
        body: '「字幕を抽出」を押して配信済みの字幕トラックを読み込み言語を切り替えます。',
      },
      {
        title: 'コピーまたは書き出し',
        body: 'タイムスタンプ表示とテキストを切り替え、行を検索して TXT・SRT・VTT を保存します。',
      },
    ],
    faqTitle: 'よくある質問',
    faq: JA_TRANSCRIPT_FAQ,
    related: [
      { label: 'TikTok 動画ダウンローダー', href: TOOL_PATHS.ja.video },
      { label: 'TikTok 写真ダウンローダー', href: TOOL_PATHS.ja.photo },
      { label: 'AI インフルエンサースタジオ', href: '/ai-influencer-studio' },
    ],
  },
}
