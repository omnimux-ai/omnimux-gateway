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
// fork: English copy for the Agent entry pages. Every command below exists in
// cli/README.md; do not add a capability line without checking the CLI.

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: 'Install the CLI',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: 'Verify with omnimux help. The CLI defaults to omnimux.ai.',
  },
  {
    title: 'Get an API key',
    body: 'Open the console, go to API Keys, and create a key. Copy it once.',
    action: { label: 'Create API key', href: '/keys' },
  },
  {
    title: 'Configure the key',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: 'Add this line to ~/.zshrc or ~/.bashrc, reload the shell, then restart your agent.',
  },
]

/** Step 1 differs per page: the CLI installer vs the skill installer. */
const SKILL_STEP: AgentStep = {
  title: 'Install skills',
  command: 'omnimux skill install --user',
  body: 'Install the CLI first. Skills land in ~/.agents/skills; restart your agent to pick them up.',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: 'Analyze a TikTok video',
    body: 'Break down the script, shots, hook structure and conversion logic of any TikTok.',
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
    title: 'Generate short videos',
    body: 'Turn a script or a creative brief into a TikTok-style short video.',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: 'Generate AI images',
    body: 'Create high-quality images for marketing, social or a creative project.',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: 'Use a digital avatar',
    body: 'Turn a photo into a consistent on-camera persona for talking-head videos.',
    mock: { type: 'voice' },
  },
  {
    title: 'Enhance video',
    body: 'Strip subtitles and watermarks, then upscale rough footage into publishable HD.',
    mock: {
      type: 'chips',
      chips: ['Remove subtitles', 'Remove watermark', 'Upscale'],
    },
  },
  {
    title: 'Publish to TikTok',
    body: 'Publish TikTok Shop and regular videos or photo posts from an agent over the Open API and CLI.',
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
        'TikTok Shop content',
        'Video and photo posts',
        'Web, Open API and CLI',
      ],
    },
  },
]

/** The Skills page adds two cards the CLI page does not carry. */
const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'AI video analysis',
    body: 'Break down the opening, lines and shots of a reference video, with a focus on TikTok product ads, then steer the direction in conversation.',
    mock: {
      type: 'chips',
      chips: ['Opening structure', 'Line breakdown', 'Shot reconstruction'],
    },
  },
  {
    title: 'TikTok carousel generator',
    body: 'Turn a product photo into a cover, page-by-line copy and a full carousel, adjusting each page’s selling point and narrative order.',
    mock: { type: 'chips', chips: ['Cover', 'Page copy', 'Full carousel'] },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: 'How do I analyze a TikTok video with an AI agent?',
    a: 'Install the CLI and its skills, then paste a TikTok link and say “analyze this video”. The agent calls the video-analysis skill and returns the hook formula, script structure and a reusable template.',
  },
  {
    q: 'What is a TikTok hook, and why does it matter?',
    a: 'The hook is what decides whether a viewer stays through the first one to three seconds. OmniMux names the opening pattern, pacing and trigger points of a video that worked.',
  },
  {
    q: 'What is OmniMux?',
    a: 'OmniMux is an AI character generation and reference-video recreation platform for TikTok cross-border sellers, reachable through the web console, the Open API, and this CLI with its skills.',
  },
  {
    q: 'What are skills?',
    a: 'Skills are capability packs an agent can load. Running omnimux skill install --user places them under ~/.agents/skills so your agent can call the tools and APIs behind them without you writing code.',
  },
  {
    q: 'Does it cost extra?',
    a: 'The CLI and skills add no separate fee; usage draws on the same balance as the website. Codex, Claude Code and other agents set their own prices.',
  },
  {
    q: 'How long does one analysis take?',
    a: 'Usually 30 seconds to a minute, including transcription, visual understanding and the structured write-up.',
  },
  {
    q: 'Can the CLI publish TikTok videos?',
    a: 'Yes, through the omnimux social commands. Connect the account first and pass the provider that owns it.',
  },
  {
    q: 'What if it does not work after installing?',
    a: 'Restart the agent, run omnimux skill status to confirm where skills landed, and omnimux balance to confirm the key is valid.',
  },
]

const SHELL = {
  stepLabel: 'Setup',
  compatTitle: 'Works with every major agent',
  compatLead:
    'Codex, Claude Code, Hermes, OpenClaw and most other agents are supported.',
  flowLabels: {
    agents: 'Your local agent',
    hub: 'OmniMux CLI',
    caps: 'OmniMux capabilities',
  },
  coreTitle: 'Core capabilities',
  coreLead:
    'The capabilities that matter most: TikTok analysis, publishing, AI video, image creation and digital avatars.',
  copy: 'Copy',
  copied: 'Copied',
  kindAria: 'Agent entry',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: 'One key, every capability',
  ctaBody:
    'Create a key, hand it to your agent, and let AI break down the next TikTok hit for you.',
  ctaButton: 'Create API key',
  faqTitle: 'FAQ',
}

export const en: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS.en.cli,
    title:
      'OmniMux CLI — TikTok analysis, publishing and AI generation from your agent',
    description:
      'Install the OmniMux CLI and call TikTok analysis, AI video and image generation, and publishing from Codex, Claude Code, OpenClaw and other agents.',
    badge: 'AI Agent CLI for TikTok',
    h1: 'One command, OmniMux capabilities in any AI',
    subtitle:
      'Analyze TikTok videos, generate images and video, and publish to TikTok.',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: 'One install · one key',
    flowCaps: [
      {
        title: 'Find what works',
        body: 'Break down the hook, structure and script of any TikTok',
      },
      {
        title: 'Make content',
        body: 'AI video, images, digital avatars and video enhancement',
      },
      {
        title: 'Publish and run',
        body: 'TikTok Shop and regular videos, scheduled',
      },
    ],
    cards: CORE_CARDS,
    tryTitle: 'Try it',
    tryHeading: 'From prompt to footage in one line',
    tryBody:
      'Send the idea to your agent; it calls OmniMux and comes back with video and shot images in one pass.',
    tryPrompts: [
      {
        label: 'Analyze structure',
        text: 'Break down the structure of this TikTok: https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: 'Generate shot images',
        text: 'Storyboard a 30s TikTok selling video for a beauty device with a strong hook',
      },
      {
        label: 'Generate the video',
        text: 'Good, now generate the video from this storyboard',
      },
    ],
    tryChat: {
      prompt:
        'Break down the structure of this TikTok: https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: 'Problem → Solution' },
        { label: 'Structure', value: 'Hook → Value → CTA' },
        { label: 'Timeline', value: '0:02 · 0:11 · 0:24' },
        { label: 'Template', value: 'Reusable' },
      ],
      follow:
        'Storyboard a 30s TikTok selling video for a beauty device with a strong hook',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS.en.skills,
    title:
      'OmniMux Skills — TikTok analysis and short-video generation for any agent',
    description:
      'Give Codex, OpenClaw, Claude Code and other agents OmniMux skills to analyze TikTok hits and generate short videos.',
    badge: 'AI Agent Skills for TikTok',
    h1: 'Give any AI OmniMux skills',
    subtitle:
      'Install OmniMux skills into Codex, OpenClaw, Claude Code and others to analyze TikTok hits and generate short videos.',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: 'One install · one key',
    flowCaps: [
      {
        title: 'Find what works',
        body: 'Break down the hook, structure and script of any TikTok',
      },
      {
        title: 'Recreate a hit',
        body: 'Use a competitor or reference video as the template for your own product',
      },
      {
        title: 'Make content',
        body: 'AI video, images, digital avatars and video enhancement',
      },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: 'Try it',
    tryHeading: 'One link, a full breakdown',
    tryBody:
      'Send the prompt below to your agent; it transcribes, reads the footage, then returns the script and hook analysis.',
    tryPrompts: [
      {
        label: 'Analyze this video',
        text: 'Analyze this video: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: 'Recreate the structure',
        text: 'Write a 30s script for my serum using this structure',
      },
    ],
    tryChat: {
      prompt:
        'Analyze this video: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: 'Write a 30s script for my serum using this structure',
    },
    faq: FAQ,
  },
}
