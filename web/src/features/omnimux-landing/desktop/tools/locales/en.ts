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

const EN_TRANSCRIPT_FAQ: ToolPageContent['faq'] = [
  {
    q: 'Where do the transcripts come from?',
    a: 'OmniMux reads the published WebVTT caption tracks included in a public TikTok video post. Both creator captions and TikTok auto captions appear when available.',
  },
  {
    q: 'Why does a video show no native captions?',
    a: 'Music-only clips, videos with captions disabled by the creator, and photo slideshows have no subtitle track on TikTok. In that case, you can copy the post description or save the MP4.',
  },
  {
    q: 'Do I need to sign up or install anything?',
    a: 'No. Single public links extract right in your browser, with a one-time Cloudflare check on the first parse.',
  },
  {
    q: 'How many times can I use it per day?',
    a: '5 times a day without an account and 50 times a day after signing in, shared across TikTok tools. The count resets at 00:00 UTC.',
  },
  {
    q: 'Which transcript export formats are supported?',
    a: 'Copy timestamped lines or plain text directly, or export standard TXT, SRT, and VTT subtitle files.',
  },
  {
    q: 'Do you store the videos or captions I parse?',
    a: 'We do not store the video files. Post metadata is cached briefly up to 30 minutes alongside daily request counts to prevent abuse. See the privacy policy for details.',
  },
]

export const en: Record<ToolKind, ToolPageContent> = {
  video: {
    path: TOOL_PATHS.en.video,
    title: 'TikTok Video Downloader — Save Videos as MP4 | OmniMux',
    description:
      'Paste a public TikTok video link and save it as an MP4 in your browser. Then recreate the post as an AI product video in OmniMux.',
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
      {
        label: 'TikTok Transcript Generator',
        href: TOOL_PATHS.en.transcript,
      },
      { label: 'AI Influencer Studio', href: '/ai-influencer-studio' },
    ],
  },
  photo: {
    path: TOOL_PATHS.en.photo,
    title: 'TikTok Photo Downloader — Save Slideshow Images | OmniMux',
    description:
      'Save every image of a public TikTok photo post in order, right in your browser. Then recreate the post as an AI product video in OmniMux.',
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
      {
        label: 'TikTok Transcript Generator',
        href: TOOL_PATHS.en.transcript,
      },
      { label: 'AI Influencer Studio', href: '/ai-influencer-studio' },
    ],
  },
  transcript: {
    path: TOOL_PATHS.en.transcript,
    title: 'TikTok Transcript Generator — Extract Captions | OmniMux',
    description:
      'Paste a public TikTok video link to extract timestamped captions, copy plain text, or export TXT, SRT, and VTT files in your browser.',
    h1: 'TikTok Transcript Generator',
    subtitle:
      'Paste a public TikTok video link to extract captions with timestamps or plain text',
    stepsTitle: 'Extract a TikTok transcript in 3 steps',
    steps: [
      {
        title: 'Copy the video link',
        body: 'On the TikTok video, tap Share → Copy link, or paste the share text.',
      },
      {
        title: 'Extract captions',
        body: 'Click Get transcript to load published TikTok caption tracks and switch languages.',
      },
      {
        title: 'Copy or export',
        body: 'Switch between timestamps and plain text, filter lines, or download TXT, SRT, and VTT.',
      },
    ],
    faqTitle: 'FAQ',
    faq: EN_TRANSCRIPT_FAQ,
    related: [
      { label: 'TikTok Video Downloader', href: TOOL_PATHS.en.video },
      { label: 'TikTok Photo Downloader', href: TOOL_PATHS.en.photo },
      { label: 'AI Influencer Studio', href: '/ai-influencer-studio' },
    ],
  },
}
