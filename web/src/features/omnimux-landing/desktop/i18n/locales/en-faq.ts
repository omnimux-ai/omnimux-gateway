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

/**
 * Homepage FAQ copy rendered by `<CnFaq />`. fork: seo-audit-batch2 — kept in
 * an import-free module so the build-time FAQPage JSON-LD
 * (`web/scripts/landing-structured-data.mjs`) reads the same items.
 */
export interface LandingFaqCopy {
  kicker: string
  h2: string
  lead: string
  items: { q: string; a: string }[]
}

export const enFaq: LandingFaqCopy = {
  kicker: 'Frequently asked questions',
  h2: 'What e-commerce sellers ask most',
  lead: 'Learn about open source, publishing platforms, and generation costs.',
  items: [
    {
      q: 'Is OmniMux open source?',
      a: 'Yes. The desktop client and the plugins are published at github.com/omnimux-ai/omnimux-client, and the gateway source at github.com/omnimux-ai/omnimux-gateway, under AGPL-3.0. The hosted service keeps its channel credentials, pricing ledger, operations and user data out of scope.',
    },
    {
      q: 'Can I start generating right after downloading?',
      a: 'Download, install, and sign in to enter the workspace. Generations deduct credits according to your wallet balance and task records; parameters and credit usage are confirmed beforehand.',
    },
    {
      q: 'Which platforms are supported for publishing?',
      a: 'Schedule posts through the publishing hub to TikTok, Instagram, Facebook, and YouTube. Direct TikTok integration is off by default and defaults to SELF_ONLY privacy; Douyin, Xiaohongshu, WeChat Channels, Kuaishou, and Bilibili are not supported.',
    },
    {
      q: 'Can I see an exact price before generating a video?',
      a: 'The image studio displays a reference quote; video billing follows task records and actual charges. Parameters and credit usage are confirmed beforehand, with exact rates governed by wallet settings.',
    },
    {
      q: 'Do you guarantee viral results?',
      a: 'No. OmniMux provides workspace tools for reference breakdown, recreation, and video creation — it does not guarantee views, sales, or viral results. Account safety depends on your content and account behavior.',
    },
  ],
}
