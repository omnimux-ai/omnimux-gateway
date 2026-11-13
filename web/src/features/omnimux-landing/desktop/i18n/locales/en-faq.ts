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
  h2: string
  lead: string
  items: { q: string; a: string }[]
}

export const enFaq: LandingFaqCopy = {
  h2: 'Frequently asked questions',
  lead: 'What e-commerce sellers ask most',
  items: [
    {
      q: 'Is OmniMux open source?',
      a: 'Yes. The desktop client, the plugins and the gateway source are published per release at github.com/omnimux-ai/omnimux-ai under AGPL-3.0. The hosted service keeps its channel credentials, pricing ledger, operations and user data out of scope.',
    },
    {
      q: 'What can the OmniMux Agent do?',
      a: 'The OmniMux Agent analyzes reference videos, writes scripts, generates videos and images, and helps publish to the platforms you have connected. Describe your product and goal in plain language and it picks the right tool for each step.',
    },
    {
      q: 'Will the Agent generate or publish without my approval?',
      a: 'No. Before generating video or images — and before publishing — the Agent shows you the parameters and estimated credit cost and waits for your confirmation. You can adjust the parameters first. If a task fails for system reasons, the charged credits are refunded.',
    },
    {
      q: 'How do I make a shoppable video with AI?',
      a: 'Paste a product link or pick a viral reference, generate an original or recreated video, then publish to the platforms you have connected. No filming or editing needed.',
    },
    {
      q: 'Can generated videos be published directly?',
      a: 'Yes. Connect your account once in the desktop app and publish directly — no manual uploads. You can add the AI-content declaration required by each platform when publishing.',
    },
    {
      q: 'Will AI-made videos get my account banned?',
      a: 'OmniMux cannot guarantee protection from bans — account safety depends on your content and account behavior. When publishing you can add an AI-generated-content disclosure to your video; please follow each platform’s policies.',
    },
    {
      q: 'Can it turn product images into video?',
      a: 'Yes. Upload a product image or paste a product link and the AI generates a publish-ready showcase video — no filming required.',
    },
    {
      q: 'How do I recreate a competitor’s viral video?',
      a: 'The AI studies their craft — pacing, framing, tone — then generates a brand-new video with your product and original footage. You recreate the approach, not the original clip.',
    },
    {
      q: 'Can I use generated videos commercially?',
      a: 'Commercial usage is governed by the Terms of Service. For uploaded portraits, product images and reference videos, you confirm you hold the rights to use them and comply with each platform’s rules.',
    },
  ],
}
