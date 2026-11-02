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
// fork: single source for the TikTok downloader tool pages' indexable text.
// The React page and the build-time static SEO HTML both read this module, so
// it must stay plain data: no React/JSX and no '@/…' alias imports
// (rsbuild.config.ts imports it with a relative path).

export type ToolKind = 'video' | 'photo'
export type ToolLocale = 'en' | 'zh-CN'

export interface ToolPageContent {
  path: string
  /** SEO <title>. */
  title: string
  /** SEO meta description. */
  description: string
  h1: string
  subtitle: string
  stepsTitle: string
  steps: { title: string; body: string }[]
  faqTitle: string
  faq: { q: string; a: string }[]
  related: { label: string; href: string }[]
}

export const TOOL_PATHS: Record<ToolLocale, Record<ToolKind, string>> = {
  en: {
    video: '/tools/tiktok-video-downloader',
    photo: '/tools/tiktok-photo-downloader',
  },
  'zh-CN': {
    video: '/zh/tools/tiktok-video-downloader',
    photo: '/zh/tools/tiktok-photo-downloader',
  },
}

const EN_FAQ: ToolPageContent['faq'] = [
  {
    q: 'Do I need to sign up or install anything?',
    a: 'No. A single public link is parsed right in your browser, and the first use runs a one-time Cloudflare check. Downloading a whole photo post as a ZIP requires signing in.',
  },
  {
    q: 'How many times can I use it per day?',
    a: '5 times a day without an account and 50 times a day after signing in. The count resets at 00:00 UTC.',
  },
  {
    q: 'Which links are supported?',
    a: 'Public TikTok video and photo post links, including vm.tiktok.com and vt.tiktok.com short links and share text that contains a link. Private, friends-only, removed or region-restricted posts cannot be parsed.',
  },
  {
    q: 'What format are the downloaded files?',
    a: 'Videos are MP4. Photo posts keep the original image format provided by TikTok (JPEG or WebP), numbered in post order.',
  },
  {
    q: 'Can I use the downloads commercially or repost them?',
    a: 'The copyright belongs to the original creator. Only save content you have the right to use, and credit the source when you use it as a reference. For commercially usable material, regenerate it with your own character in OmniMux.',
  },
  {
    q: 'Do you keep the links I parse?',
    a: 'To prevent abuse we briefly record request counts and post IDs for limits and caching. We do not store the video files. See the privacy policy for details.',
  },
]

const ZH_FAQ: ToolPageContent['faq'] = [
  {
    q: '需要注册或安装软件吗？',
    a: '不需要。单条公开链接直接在浏览器里解析，首次使用会经过一次 Cloudflare 人机验证；图文打包下载需要登录。',
  },
  {
    q: '每天能用几次？',
    a: '未登录每天 5 次，登录后每天 50 次，次日 0 点（UTC）重置。',
  },
  {
    q: '支持哪些链接？',
    a: '公开的 TikTok 视频、图文链接，包括 vm.tiktok.com、vt.tiktok.com 短链接和带文字的分享内容。私密、仅好友可见、已删除或有地区限制的内容无法解析。',
  },
  {
    q: '下载的文件是什么格式？',
    a: '视频为 MP4，图文为 TikTok 提供的原始图片格式（JPEG 或 WebP），按帖子顺序编号。',
  },
  {
    q: '可以拿去商用或二次发布吗？',
    a: '视频版权属于原作者。请只保存你有权使用的内容，用于学习参考时注明出处；需要可商用的素材，可以用 OmniMux 以你自己的角色重新生成。',
  },
  {
    q: '会保存我解析过的链接吗？',
    a: '为防止滥用，我们会短期记录请求次数和视频编号用于限额与缓存，不保存视频文件。详见隐私政策。',
  },
]

export const TOOL_PAGES: Record<
  ToolLocale,
  Record<ToolKind, ToolPageContent>
> = {
  en: {
    video: {
      path: TOOL_PATHS.en.video,
      title: 'TikTok Video Downloader — Save TikTok Videos as MP4 | OmniMux',
      description:
        'Paste a public TikTok video link or share text and save the video as an MP4 file. Free in your browser, nothing to install, preview before you download.',
      h1: 'TikTok Video Downloader',
      subtitle: 'Paste a public TikTok video link and save it as MP4',
      stepsTitle: 'Save a TikTok video in 3 steps',
      steps: [
        {
          title: 'Copy the link',
          body: 'On the TikTok video, tap Share → Copy link. Short and full links both work.',
        },
        {
          title: 'Paste and parse',
          body: 'Paste it into the box above and click Parse.',
        },
        {
          title: 'Save',
          body: 'Preview it, then download the MP4 to your device.',
        },
      ],
      faqTitle: 'FAQ',
      faq: EN_FAQ,
      related: [
        { label: 'TikTok Photo Downloader', href: TOOL_PATHS.en.photo },
        { label: 'Viral video recreation', href: '/influencer' },
        { label: 'AI character generator', href: '/ai-influencer-studio' },
      ],
    },
    photo: {
      path: TOOL_PATHS.en.photo,
      title: 'TikTok Photo Downloader — Save Slideshow Images | OmniMux',
      description:
        'Paste a public TikTok photo post link and save every slideshow image in its original order. Free in your browser; download single images or a ZIP after signing in.',
      h1: 'TikTok Photo Downloader',
      subtitle:
        'Paste a public TikTok photo post link and save the original images in order',
      stepsTitle: 'Save TikTok photos in 3 steps',
      steps: [
        {
          title: 'Copy the link',
          body: 'On the TikTok photo post, tap Share → Copy link.',
        },
        {
          title: 'Paste and parse',
          body: 'Paste it into the box above and click Parse.',
        },
        {
          title: 'Save',
          body: 'Save images one by one, or sign in to download them all as a ZIP.',
        },
      ],
      faqTitle: 'FAQ',
      faq: EN_FAQ,
      related: [
        { label: 'TikTok Video Downloader', href: TOOL_PATHS.en.video },
        { label: 'Viral video recreation', href: '/influencer' },
        { label: 'AI character generator', href: '/ai-influencer-studio' },
      ],
    },
  },
  'zh-CN': {
    video: {
      path: TOOL_PATHS['zh-CN'].video,
      title: 'TikTok 视频下载 — 保存为 MP4 | OmniMux',
      description:
        '粘贴公开 TikTok 视频链接或分享文字，在浏览器里直接解析并保存为 MP4。免费、免安装，预览确认后再下载。',
      h1: 'TikTok 视频下载',
      subtitle: '粘贴公开 TikTok 视频链接，保存为 MP4',
      stepsTitle: '三步保存 TikTok 视频',
      steps: [
        {
          title: '复制链接',
          body: '在 TikTok 视频页点「分享」→「复制链接」，短链接和完整链接都可以',
        },
        { title: '粘贴并解析', body: '粘贴到上方输入框，点「解析」' },
        { title: '保存', body: '预览确认后下载 MP4 到本机' },
      ],
      faqTitle: '常见问题',
      faq: ZH_FAQ,
      related: [
        { label: 'TikTok 图文下载', href: TOOL_PATHS['zh-CN'].photo },
        { label: '爆款复刻', href: '/influencer' },
        { label: 'AI 角色生成', href: '/ai-influencer-studio' },
      ],
    },
    photo: {
      path: TOOL_PATHS['zh-CN'].photo,
      title: 'TikTok 图文下载 — 保存原图 | OmniMux',
      description:
        '粘贴公开 TikTok 图文链接，按帖子顺序保存原图。免费在浏览器里使用，支持单张下载，登录后可打包下载全部图片。',
      h1: 'TikTok 图文下载',
      subtitle: '粘贴公开 TikTok 图文链接，按顺序保存原图',
      stepsTitle: '三步保存 TikTok 图文',
      steps: [
        { title: '复制链接', body: '在 TikTok 图文帖点「分享」→「复制链接」' },
        { title: '粘贴并解析', body: '粘贴到上方输入框，点「解析」' },
        { title: '保存', body: '单张保存，或登录后打包下载全部' },
      ],
      faqTitle: '常见问题',
      faq: ZH_FAQ,
      related: [
        { label: 'TikTok 视频下载', href: TOOL_PATHS['zh-CN'].video },
        { label: '爆款复刻', href: '/influencer' },
        { label: 'AI 角色生成', href: '/ai-influencer-studio' },
      ],
    },
  },
}
