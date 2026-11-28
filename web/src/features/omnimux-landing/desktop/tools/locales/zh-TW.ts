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

const ZHTW_FAQ: ToolPageContent['faq'] = [
  {
    q: '需要註冊帳號或安裝外掛嗎？',
    a: '不需要。單條公開連結可直接在瀏覽器完成解析，首次使用會進行一次 Cloudflare 驗證；圖文打包下載需登入帳號。',
  },
  {
    q: '每天可以使用幾次？',
    a: '未登入每日 5 次，登入後每日 50 次，次日 UTC 0 點重置。',
  },
  {
    q: '支援哪些 TikTok 連結？',
    a: '公開的 TikTok 影片與圖文貼文連結，包含 vm.tiktok.com、vt.tiktok.com 短網址與夾帶連結的分享文字。私密、僅限好友、已刪除或設地區限制的貼文無法解析。',
  },
  {
    q: '下載的檔案是什麼格式？',
    a: '影片為 MP4，圖文貼文維持 TikTok 原始圖片格式（JPEG 或 WebP），並依貼文順序編號。',
  },
  {
    q: '可以商用或直接轉發嗎？',
    a: '版權歸原作者所有。請僅儲存你有權使用的素材，作為參考學習時請註明出處；若需可商用素材，可在 OmniMux 以你自己的角色與商品重新生成。',
  },
  {
    q: '會保留我解析過的連結或檔案嗎？',
    a: '為防止濫用，系統僅短期記錄請求次數與貼文編號用於限額及快取，不會保留影片檔案。詳見隱私權政策。',
  },
]

const ZHTW_TRANSCRIPT_FAQ: ToolPageContent['faq'] = [
  {
    q: '提取出的字幕來自哪裡？',
    a: '直接讀取公開 TikTok 影片隨附的 WebVTT 原生字幕軌，涵蓋創作者上傳字幕與平台自動產生的字幕。',
  },
  {
    q: '為什麼部分影片顯示無原生字幕軌？',
    a: '純音樂短片、作者關閉字幕或圖文貼文在 TikTok 原始資料中不含字幕軌。此時頁面仍保留貼文文案複製與 MP4 下載入口。',
  },
  {
    q: '需要註冊或安裝軟體嗎？',
    a: '不需要。單條公開連結直接在瀏覽器提取字幕，首次使用會進行一次 Cloudflare 人機驗證。',
  },
  {
    q: '每天可以使用幾次？',
    a: '未登入每日 5 次，登入後每日 50 次，與 TikTok 下載工具共用每日次數，於 UTC 0 點重置。',
  },
  {
    q: '支援哪些字幕匯出格式？',
    a: '可一鍵複製帶時間軸或純文字全文，也可匯出標準 TXT、SRT 與 VTT 字幕檔。',
  },
  {
    q: '會保存我解析過的連結或影片嗎？',
    a: '不保存影片檔，僅短期記錄請求次數與最多 30 分鐘快取結果用於限流防刷。詳見隱私權政策。',
  },
]

export const zhTW: Record<ToolKind, ToolPageContent> = {
  video: {
    path: TOOL_PATHS['zh-TW'].video,
    title: 'TikTok 影片下載 — 儲存 MP4 | OmniMux',
    description:
      '貼上公開 TikTok 影片連結或短網址，無須安裝外掛即可在瀏覽器解析並儲存為 MP4，下載後可在 OmniMux 拆解結構並用自家角色與商品重製。',
    h1: 'TikTok 影片下載',
    subtitle: '貼上公開 TikTok 影片連結，於瀏覽器儲存為 MP4',
    stepsTitle: '三步儲存 TikTok 影片',
    steps: [
      {
        title: '複製連結',
        body: '在 TikTok 影片點選「分享」→「複製連結」，短網址與完整網址皆支援。',
      },
      {
        title: '貼上並解析',
        body: '貼進上方輸入欄並點擊「解析」，載入預覽資訊。',
      },
      {
        title: '儲存檔案',
        body: '確認預覽後即可下載 MP4 至本機，或帶入工作室以角色復刻。',
      },
    ],
    faqTitle: '常見問題',
    faq: ZHTW_FAQ,
    related: [
      { label: 'TikTok 圖文下載', href: TOOL_PATHS['zh-TW'].photo },
      { label: 'TikTok 字幕提取', href: TOOL_PATHS['zh-TW'].transcript },
      { label: 'AI 網紅工作室', href: '/ai-influencer-studio' },
    ],
  },
  photo: {
    path: TOOL_PATHS['zh-TW'].photo,
    title: 'TikTok 圖文下載 — 儲存原圖 | OmniMux',
    description:
      '貼上公開 TikTok 圖文連結或短網址，依貼文順序在瀏覽器儲存原始 JPEG 與 WebP 圖片，支援單張下載或登入打包為 ZIP。',
    h1: 'TikTok 圖文下載',
    subtitle: '貼上公開 TikTok 圖文連結，依貼文順序儲存原始圖片',
    stepsTitle: '三步儲存 TikTok 圖文',
    steps: [
      {
        title: '複製貼文連結',
        body: '在 TikTok 圖文貼點選「分享」→「複製連結」。',
      },
      {
        title: '貼上並解析',
        body: '將連結貼進上方輸入欄，點擊「解析」。',
      },
      {
        title: '單張或打包下載',
        body: '依序儲存 JPEG 或 WebP 原圖，登入後可一鍵打包為 ZIP。',
      },
    ],
    faqTitle: '常見問題',
    faq: ZHTW_FAQ,
    related: [
      { label: 'TikTok 影片下載', href: TOOL_PATHS['zh-TW'].video },
      { label: 'TikTok 字幕提取', href: TOOL_PATHS['zh-TW'].transcript },
      { label: 'AI 網紅工作室', href: '/ai-influencer-studio' },
    ],
  },
  transcript: {
    path: TOOL_PATHS['zh-TW'].transcript,
    title: 'TikTok 字幕提取 — TXT/SRT | OmniMux',
    description:
      '貼上公開 TikTok 影片連結即可提取多語原生字幕與時間軸文稿，支援純文字複製與 TXT、SRT、VTT 匯出，方便拆解腳本後在 OmniMux 重製。',
    h1: 'TikTok 字幕提取',
    subtitle: '貼上公開 TikTok 影片連結，提取帶時間軸文稿與多語原生字幕',
    stepsTitle: '三步提取 TikTok 影片字幕',
    steps: [
      {
        title: '複製影片連結',
        body: '在 TikTok 影片點選「分享」→「複製連結」，短網址或分享文字皆支援。',
      },
      {
        title: '提取字幕軌',
        body: '點擊「提取字幕」，讀取公開影片隨附的 WebVTT 原生與自動字幕。',
      },
      {
        title: '複製或匯出檔案',
        body: '切換時間軸與純文字檢視，篩選關鍵字並匯出 TXT、SRT 或 VTT。',
      },
    ],
    faqTitle: '常見問題',
    faq: ZHTW_TRANSCRIPT_FAQ,
    related: [
      { label: 'TikTok 影片下載', href: TOOL_PATHS['zh-TW'].video },
      { label: 'TikTok 圖文下載', href: TOOL_PATHS['zh-TW'].photo },
      { label: 'AI 網紅工作室', href: '/ai-influencer-studio' },
    ],
  },
}
