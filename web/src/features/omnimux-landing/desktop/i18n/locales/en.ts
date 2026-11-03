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
import type { LandingCopy } from '../types'

export const en: LandingCopy = {
  meta: {
    title: 'OmniMux Desktop — Find, Recreate & Scale Viral Shoppable Videos',
  },
  skip: 'Skip to content',

  nav: {
    brand: 'Desktop',
    home: 'Home',
    models: 'Models',
    tools: 'Tools',
    ext: 'Extension',
    pricing: 'Pricing',
    resources: 'Resources',
    download: 'Download',
    login: 'Log in',
    menuOpen: 'Open menu',
    langTitle: 'Page language',
    brandAria: 'OmniMux Desktop home',
    navAria: 'Primary navigation',
    toolsMenu: {
      aria: 'Tools menu',
      group: 'Creative',
      video: 'TikTok video download',
      photo: 'TikTok photo download',
      agentGroup: 'Agent',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: 'Resources menu',
      inspiration: 'Inspiration Community',
      docs: 'Developer Docs',
    },
    account: {
      aria: 'Account menu',
      dashboard: 'Console',
      profile: 'Profile',
      security: 'Security & access',
      wallet: 'Wallet',
      systemSettings: 'System settings',
      signOut: 'Log out',
    },
  },

  signOut: {
    title: 'Log out',
    body: 'You will need to log in again to access your account.',
    cancel: 'Cancel',
    confirm: 'Log out',
    busy: 'Logging out…',
    failed: 'Log out failed',
    done: 'Logged out',
    close: 'Close',
  },

  hero: {
    titleA: 'Find, recreate & scale',
    titleB: 'viral shoppable videos',
    subPre:
      "One content commerce system that carries your product's selling points through every video. Use ",
    subHighlight: 'the Agents you already know',
    subPost:
      ' to create ads, shoppable videos, and product reviews—all in one place.',
    tabsAria: 'Creation mode',
    tabs: { agent: 'Agent', video: 'Video', image: 'Image' },
    inputLabel: 'Describe what you want to make',
    attachAria: 'Add a reference video or image',
    sendAria: 'Start on desktop',
    dlMac: 'Download for macOS',
    dlWin: 'Download for Windows',
    dlMenuAria: 'Choose download version',
    ghStar: 'Star on GitHub',
    dlMacArm: 'macOS (Apple Silicon)',
    dlMacIntel: 'macOS (Intel)',
    dlWinX64: 'Windows (64-bit)',
    skillsLabel: 'Skills',
    skillsAria: 'Skills',
    skills: [
      {
        icon: 'scan',
        label: 'Break down video/post',
        fill: 'Break down why this video went viral — hook, selling points and buying guide https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: 'Recreate video',
        fill: 'Recreate this viral video for @Serum, keep the pacing and shot structure, 15s',
      },
      {
        icon: 'clap',
        label: 'Create shoppable video',
        fill: 'Make a 15s shoppable video for @Serum, open with the on-skin effect',
      },
      {
        icon: 'img',
        label: 'Recreate shoppable post',
        fill: 'Use this Instagram post set as reference — make a 6-image shoppable set for @Serum',
      },
      {
        icon: 'wand',
        label: 'Video to prompt',
        fill: 'Break this video into reusable prompts, shot by shot',
      },
    ],
    modes: {
      agent: {
        prompts: [
          'Break down why this video went viral https://www.tiktok.com/@pet.diary/video/73421',
          'Use this viral video to make 3 for @Serum, ready to publish, 15s',
          'Analyze how @SkinDiary played the last 30 days',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          'Make a 15s shoppable video for @Serum — open with the on-skin effect',
          'Drop-test Bluetooth earbuds from height, reveal the flash deal at the end',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          'Make an 8-image product set for @Serum: hero, benefits, details, specs, scenes',
          'Put this product shot on a marble vanity scene',
        ],
      },
    },
    reelAria: 'Built-in finished-video template samples',
    reel: [
      { title: '3D particle FX', tag: 'Pet food' },
      { title: 'Runway outfit swap', tag: 'Overalls' },
      { title: 'Drop durability test', tag: 'Earbuds' },
      { title: 'Creator selfie pitch', tag: 'Probiotics' },
      { title: 'Giant product chase', tag: 'Shopping bag' },
      { title: 'Mobile web demo', tag: 'Travel app' },
      { title: '15s promo ad', tag: 'Snacks' },
    ],
  },

  agentDemo: {
    h2: 'Recreate a viral seller video in one chat',
    lead: 'Tell it what you sell. It breaks down the reference video, writes scripts, generates content and schedules posts.',
    windowAria: 'OmniMux Desktop Agent demo',
    windowTitle: 'Recreate this viral video for my 3 hits',
    windowStatus: '3 videos generated',
    replay: 'Replay demo',
    replayAria: 'Replay demo',
    side: [
      'New chat',
      'Apps',
      'Canvas',
      'Asset library',
      'Inspiration',
      'Products',
      'Publish',
      'Analytics',
    ],
    me: 'L',
    msgPre: 'Recreate this viral video for',
    msgProducts: ['@PetFood', '@Overalls', '@Earbuds'],
    msgPost: '— one 15s video each, ready to publish.',
    linkCard: 'tiktok.com/@pet.diary/video/73421 · cat antics + kibble unboxing',
    doneBreakdown: 'Broke down why the reference video sells',
    kv: [
      { k: 'Hook', v: 'Before/after in the first 2s' },
      { k: 'Proof', v: 'Close-up of it in use' },
      { k: 'Ending', v: 'Drives taps on the in-video cart' },
    ],
    doneProducts: 'Analyzed your 3 products and selling points',
    doneScripts: 'Wrote a script for every product',
    confirmTitle: 'Generate 3 videos?',
    confirmNeed: 'Needs your confirmation',
    confirmCost: 'Estimated credit cost, charged after you confirm',
    confirmGo: 'Confirm & generate',
    confirmEdit: 'Edit',
    inputPh: 'Message OmniMux — @ to reference products or assets',
    resultsTitle: 'Final cuts',
    resultsMeta: '3 videos · 9:16',
    generating: 'Generating…',
    vidNames: ['Pet food', 'Overalls', 'Earbuds'],
    vidHooks: [
      'Hook · concept contrast',
      'Hook · outfit change',
      'Hook · close-up',
    ],
    schedTitle: 'Scheduled posts',
    schedOk: 'Scheduled',
    schedPlatform: 'TikTok',
    schedTimes: ['Tue 19:00', 'Wed 12:30', 'Thu 19:00'],
    schedAll: 'Schedule all',
  },

  flow: {
    h2: 'From product idea to shoppable video',
    lineAria: 'Discover · Analyze · Create · Publish · Review',
    steps: [
      {
        kind: 'Discover',
        title: 'Find viral shoppable videos in your category',
        desc: 'Collect reference videos you find anywhere, then break down their hook and selling points.',
        chips: ['Viral shoppable videos & posts'],
        primary: 'Break down a reference video',
        secondary: 'Collect inspiration with the browser extension',
      },
      {
        kind: 'Analyze',
        title: 'See why it sells',
        desc: 'Break a viral video into its hook, selling points and buying guide, and turn it into a reusable structure. You can also analyze competitor accounts.',
        chips: ['Script breakdown', 'Competitor accounts', 'Category trends'],
        primary: 'Break down a viral video',
      },
      {
        kind: 'Create',
        title: 'Turn your product into a shoppable video',
        desc: 'Generate an original video from a product image or link, or recreate a viral hit with your own product.',
        primary: 'Generate video',
        secondary: 'Recreate a viral video',
      },
      {
        kind: 'Publish',
        title: 'Publish to TikTok, Instagram, Facebook & YouTube',
        desc: 'Connect your accounts and publish shoppable videos and image posts instantly or on a schedule.',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: 'Publish video',
        secondary: 'Video posts · Image posts',
      },
      {
        kind: 'Review',
        title: 'Turn real feedback into your next video',
        desc: 'Once your accounts are connected, views, likes and comments sync automatically — and the Agent tells you what to make next.',
        primary: 'Analyze account',
      },
    ],

    discover: {
      title: 'Reference examples · illustrated flow',
      meta: 'Last 7 days',
      savesPrefix: 'saves',
      items: [
        {
          who: 'taotao.ootd',
          views: '46K',
          length: 'Post · 6 images',
          title: 'One polka-dot dress, three ways — work to date night #ootd',
          likes: '46K',
          saves: '31K',
        },
        {
          who: 'commute10',
          views: '2.7M',
          length: '00:28',
          title: 'One-handed makeup touch-up in the car #commutemakeup',
          likes: '89K',
          saves: '24K',
        },
        {
          who: 'dryskin.lab',
          views: '1.6M',
          length: '00:34',
          title:
            'Dropper serum on real dry skin for seasonal redness #sensitiveskin',
          likes: '52K',
          saves: '18K',
        },
      ],
    },

    breakdown: {
      tabsAria: 'What to break down',
      tabs: {
        video: 'Video',
        account: 'Competitor account · Sample',
        category: 'Category trends · Sample',
      },
      statsViews: '15M+ views',
      statsLikes: '320K+ likes',
      why: 'Why it sells',
      rows: [
        {
          label: 'Opening hook',
          time: '0–2s',
          desc: 'Result first — show the effect before the product',
        },
        {
          label: 'Selling points',
          time: '3–9s',
          desc: 'Hands-on close-up, voiceover covers three points',
        },
        {
          label: 'Buying guide',
          time: '10–14s',
          desc: 'Limited-time deal, drives taps on the in-video cart',
        },
      ],
      reuse: 'Reusable structure',
      chain: ['Result first', 'Hands-on proof', 'Deal + cart'],
      accountName: '@SkinDiary',
      accountMeta: 'Beauty & personal care',
      thumbs: ['2.1M', '1.4M', '860K', '510K'],
      metrics: [
        { label: 'Videos posted · last 30 days', value: '42' },
        { label: 'Avg. views', value: '186K' },
        { label: 'Shoppable videos', value: '71%' },
      ],
      insights: [
        {
          k: 'Playbook',
          v: '7 of the top 10 videos open with a "result first" hook then cut to a hands-on close-up — that is this account’s core play.',
        },
        {
          k: 'Timing',
          v: 'Videos posted between 7–9pm get about 40% more views than other slots.',
        },
        {
          k: 'Opportunity',
          v: 'Nobody in this category pairs before/after with a price anchor yet — worth testing first.',
        },
      ],
      accountCta: 'Use this playbook to make 3 for @Serum',
      catTitle: 'Beauty & personal care',
      catMeta: 'Last 7 days · top 200 · Sample data',
      bars: [
        { label: 'Before/after', value: '38%' },
        { label: 'Unboxing & trial', value: '24%' },
        { label: 'Hands-on close-up', value: '19%' },
        { label: 'Voiceover pitch', value: '12%' },
        { label: 'Story twist', value: '7%' },
      ],
      catInsights: [
        {
          k: 'Hook',
          v: 'Before/after openings appear in 38% of top videos and earn the highest average views.',
        },
        {
          k: 'Length',
          v: 'Most hits run 15–25s; anything over 40s drops off sharply.',
        },
        {
          k: 'Opportunity',
          v: 'The hands-on close-up + limited-time deal combo is still rare — a good entry point for new products.',
        },
      ],
      catCta: 'Draft a benchmarking plan from this',
    },

    make: {
      tabsAria: 'How to make it',
      tabs: { generate: 'Generate', replicate: 'Recreate viral' },
      capProduct: 'Product',
      capResult: 'Final cut',
      capRef: 'Viral reference',
      capMine: 'Your product',
      badgeGenerated: 'Generated',
      badgeRef: '▶ TikTok · 2.1M',
      arrowReplicateA: 'Keep the structure',
      arrowReplicateB: 'swap in your product',
      promptLabel: 'Prompt',
      promptGenerate:
        'Bluetooth earbuds: high-altitude drop test, camera follows the landing, still intact, reveal a flash deal at the end — real-person voiceover style.',
      intentLabel: 'Rewrite intent',
      promptReplicate:
        'Keep the reference video’s selfie-voiceover pacing: pain-point opening, handheld demo in the middle, honest takeaway at the end. We recreate the approach, not the footage.',
      altProduct: 'Bluetooth earbuds product shot',
      altRef: 'Viral reference video cover',
      altCan: 'Probiotics product shot',
    },

    publish: {
      title: 'Publishing plan',
      meta: 'This week',
      posts: [
        {
          title: 'Chips you can’t put down — binge-watch fuel',
          platformLabel: 'TikTok',
          account: '@snacklab',
          time: 'Today 18:30',
          state: 'Published',
        },
        {
          title: 'Even picky dogs fight over this kibble',
          platformLabel: 'TikTok',
          account: '@furbowl',
          time: 'Today 20:00',
          state: 'Scheduled',
        },
        {
          title: 'One pair of overalls, seven outfits',
          platformLabel: 'Instagram',
          account: '@ootd_daily',
          time: 'Tomorrow 12:00',
          state: 'Ready',
        },
        {
          title: 'One month on probiotics — honest review',
          platformLabel: 'YouTube',
          account: '@goodfinds',
          time: 'Tomorrow 19:00',
          state: 'Processing',
        },
      ],
      feats: [
        {
          title: 'Submit to every platform at once',
          desc: 'TikTok, Instagram, Facebook and YouTube — pick accounts, submit once, and track each account separately.',
        },
        {
          title: 'One place for all accounts',
          desc: 'Connect every platform account in the desktop app, then just pick one when publishing — no switching back and forth.',
        },
        {
          title: 'Status tracking & retry',
          desc: 'Every task keeps a status record and failed tasks can be retried.',
        },
      ],
    },

    review: {
      title: 'Account performance',
      synced: 'Synced · just now',
      meta: 'Last 7 days',
      kpis: [
        { label: 'Views', value: '128K', delta: '+18%' },
        { label: 'Likes', value: '8,640', delta: '+24%' },
        { label: 'Engagement', value: '5.2%', delta: '+0.8pt' },
      ],
      days: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
      splitTitle: 'Views by hook type',
      splitLegs: ['Before/after hooks 63%', 'Voiceover hooks 37%'],
      note: 'Before/after openings drove 63% of views; posting between 7–9pm performs best.',
      cta: 'Next round: make 3 more with this hook',
    },
  },

  create: {
    tag: 'Creator Agent',
    h2: 'Hand it to the Agent, or build the flow yourself',
    lead: 'Create through chat with the OmniMux Agent, or connect image, video and voice tools on the Canvas to build a reusable workflow.',
    msgProductCard: 'Energy drink · hero image',
    msgAsk: 'Make a product image set for this energy drink.',
    doneAnalyze: 'Analyzed the product: shape, label & palette',
    donePlan: 'Planned an 8-image set: hero, benefits, details, specs, scenes',
    doneConsistent: 'Keeping the product identical across every image',
    confirmTitle: 'Generate 8 images?',
    confirmNeed: 'Needs your confirmation',
    paramCount: '8 images',
    confirmGo: 'Confirm & generate',
    confirmEdit: 'Edit',
    doneResult: 'Set complete — 8 images added to the Canvas',
    tabsAria: 'Workspace',
    tabs: {
      canvas: 'Canvas',
      assets: 'Asset library',
      inspiration: 'Inspiration',
      products: 'Products',
    },
    groupLabel: 'Product set · 8',
    setCaptions: [
      'Hero',
      'Benefits',
      'Details',
      'Specs',
      'Scene',
      'Scene',
      'Mood',
      'Poster',
    ],
    libAssets: [
      { label: 'Character', sub: 'Deer · 3 angles' },
      { label: 'Scene', sub: 'Open kitchen' },
      { label: 'Style', sub: 'Neon cyber' },
      { label: 'Prop', sub: 'Overalls' },
    ],
    libInspiration: [
      { label: 'Lip tint swatches', sub: 'TikTok · 3M views' },
      { label: 'In-car touch-up', sub: 'TikTok · 2.7M views' },
      { label: 'Serum field test', sub: 'Instagram · 18K saves' },
      { label: 'Screen chase', sub: 'Broken down' },
    ],
    libProducts: [
      {
        name: 'Energy drink',
        sub: '0 sugar · energizing · icy finish',
        count: '8-image set',
      },
      {
        name: 'Earbuds',
        sub: 'ANC · 30h battery · drop-proof',
        count: '2 videos',
      },
      {
        name: 'Probiotics',
        sub: 'Daily scoop · single-serve · portable',
        count: '1 video',
      },
    ],
    capAgentTitle: 'Say what you need — the Agent does the whole job',
    capAgentDesc:
      'From product images and videos to scripts, the Agent plans every step and keeps the same product consistent across all content.',
    capAgentCta: 'Start creating',
    capCanvasTitle: 'Canvas: visual creative workflows',
    capCanvasDesc:
      'Connect image, video and voice-over nodes into a workflow — adjust any step and run it again.',
    capCanvasCta: 'Open Canvas',
    canvasAria: 'Canvas workflow example',
    nodeImage: 'Image',
    nodeText: 'Text',
    nodeTextBody:
      'Neon lighting, dark backdrop, can close-up, ice mist and droplets',
    nodeAIImage: 'AI image',
    nodeAIVideo: 'AI video',
    nodeCut: 'Edit & export',
    nodeDone: 'Done',
    nodeCutBodyA: 'Add subtitles & music',
    nodeCutBodyB: 'Export 9:16 vertical',
  },

  surfaces: {
    h2: 'Use it where you already work',
    lead: 'The same Agent in the desktop app, Codex, Claude Code and the browser.',
    desktop: {
      title: 'OmniMux Desktop',
      desc: 'The full workspace: canvas, assets, products and every tool — on macOS and Windows.',
      input: 'Make a video for each of my 3 serums…',
      cta: 'Download Desktop',
    },
    cli: {
      title: 'Codex, Claude Code & other agents',
      desc: 'Install OmniMux CLI and call video, image and analysis tools from the agents you already use.',
      cta: 'Install the CLI',
    },
    ext: {
      title: 'Browser extension',
      desc: 'Spot a viral video on the web — save it to Inspiration in one click and pick up the breakdown back on desktop.',
      capSave: 'Save to Inspiration',
      capScan: 'Break down',
      toast: 'Saved to Inspiration',
      cta: 'Install the extension in the app',
    },
  },

  tools: {
    h2: 'Generate shoppable videos, images & voice-overs with AI',
    lead: 'Build product assets, multi-platform content and reuse existing videos — all in one workspace.',
    centerLabel: 'Content creation',
    groups: [
      {
        title: 'Product listings',
        lead: 'Bring product images up to listing standard',
        items: [
          { icon: 'img', label: 'White-background shots' },
          { icon: 'layers', label: 'Multi-angle product shots' },
          { icon: 'grid', label: 'Product image sets' },
          { icon: 'box', label: 'Detail-page images' },
          { icon: 'spark', label: 'Scene images' },
          { icon: 'wand', label: 'Restyle from reference' },
        ],
      },
      {
        title: 'Shoppable video',
        lead: 'Generate from scratch, or follow a viral hit',
        items: [
          { icon: 'link', label: 'Product-to-video' },
          { icon: 'video', label: 'Video generation' },
          { icon: 'flame', label: 'Viral recreation' },
          { icon: 'wand', label: 'Video to prompt' },
          { icon: 'grid', label: 'Storyboard scripts' },
          { icon: 'img', label: 'First-frame video' },
        ],
      },
      {
        title: 'Shoppable image posts',
        lead: 'A set of images can sell too',
        items: [
          { icon: 'layers', label: 'Image carousels' },
          { icon: 'scan', label: 'Reference-image remake' },
          { icon: 'img', label: 'Image generation' },
          { icon: 'shirt', label: 'Virtual try-on' },
        ],
      },
      {
        title: 'Voice & presenters',
        lead: 'Voices, characters and on-camera talent',
        items: [
          { icon: 'user', label: 'Digital humans' },
          { icon: 'users', label: 'Multi-angle characters' },
          { icon: 'mic', label: 'Voice-over & SFX' },
          { icon: 'music', label: 'AI music' },
        ],
      },
      {
        title: 'Multi-platform fit',
        lead: 'One set of assets, adapted for every platform',
        items: [
          { icon: 'crop', label: 'Aspect-ratio fit' },
          { icon: 'hash', label: 'Titles & hashtags' },
        ],
      },
      {
        title: 'Footage remix',
        lead: 'Edit, split and burn subtitles into existing footage.',
        items: [
          { icon: 'cut', label: 'Video editing' },
          { icon: 'scan', label: 'Scene splitting' },
          { icon: 'subs', label: 'Subtitle burn-in' },
        ],
      },
    ],
    modelsTitle: 'Top AI models, tuned for selling',
    modelsAria: 'Available models',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: 'Video' },
      { mark: 'S', name: 'Seedance 2.0', kind: 'Video' },
      { mark: 'G', name: 'GPT Image 2.5', kind: 'Image' },
      { mark: 'N', name: 'Nano Banana', kind: 'Image' },
      { mark: '♪', name: 'Suno', kind: 'Music' },
      { mark: 'G', name: 'Gemini', kind: 'Understanding & scripts' },
    ],
    chipsAria: 'Model capabilities',
    capabilityRows: [
      [
        { icon: 'monitor', label: '480p / 720p' },
        { icon: 'crop', label: '9:16' },
        { icon: 'crop', label: '16:9' },
        { icon: 'crop', label: '1:1' },
        { icon: 'crop', label: '3:4' },
        { icon: 'crop', label: '4:3' },
        { icon: 'clock', label: '4–30s output' },
      ],
      [
        { icon: 'layers', label: 'Multimodal references' },
        { icon: 'user', label: 'Character consistency' },
        { icon: 'img', label: 'First-frame control' },
        { icon: 'music', label: 'Native audio' },
        { icon: 'spark', label: 'Product consistency' },
      ],
      [
        { icon: 'cut', label: 'Local editing' },
        { icon: 'scan', label: 'Scene splitting' },
        { icon: 'subs', label: 'Subtitle burn-in' },
        { icon: 'mic', label: 'Audio extraction' },
        { icon: 'layers', label: 'Image-post video' },
      ],
    ],
  },

  workspace: {
    h2: 'One workspace, every asset at hand',
    lead: 'Products, assets, inspiration and publishing history live in one desktop app — always ready, no hunting around.',
    list: [
      {
        name: 'Product library',
        desc: 'Selling points, audiences & product images',
      },
      { name: 'Asset library', desc: 'Characters, scenes, styles & prompts' },
      { name: 'Inspiration', desc: 'Saved viral videos & breakdowns' },
      { name: 'Publishing hub', desc: 'Drafts, schedules & history' },
    ],
    note: 'Works on macOS and Windows.',
    tableTitle: 'Product library',
    addProduct: 'Add product',
    cols: ['Product', 'Selling points', 'Content this month'],
    rows: [
      { name: 'Energy drink', category: 'Food & beverage', points: '3' },
      { name: 'Earbuds', category: 'Electronics', points: '4' },
      { name: 'Overalls', category: 'Apparel', points: '3' },
      { name: 'Probiotics', category: 'Supplements', points: '2' },
    ],
  },

  proof: {
    h2: 'Built for cross-border sellers',
    lead: 'Find hits, break them down, create and publish — all inside one desktop app.',
    facts: [
      { value: '17', label: 'built-in skills' },
      { value: '7', label: 'finished-video templates' },
      { value: '4', label: 'publish platforms' },
    ],
    platformsAria: 'Supported publishing platforms',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: [
      'Confirm parameters & credits before generating',
      'Credits auto-refunded on failed tasks',
    ],
  },

  faq: {
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
  },

  finalCta: {
    h2: 'Your next viral video — leave it to OmniMux',
    lead: 'Break down hits, generate videos, publish and review — one Agent runs the whole loop.',
    dlMac: 'Free download for macOS',
    dlWin: 'Download for Windows',
    note: 'macOS 12+ (Apple silicon & Intel universal) · Windows 10 / 11 x64',
  },

  download: {
    title: 'Continue on OmniMux Desktop',
    desc: 'Download, install and log in to start creating.',
    close: 'Close',
    macTitle: 'Download for macOS (Universal)',
    macSub: 'macOS 12+ · Apple silicon & Intel',
    winTitle: 'Download for Windows (x64 installer)',
    winSub: 'Windows 10 / 11 x64',
    recommended: 'Recommended',
    notePre: 'If Windows says "Windows protected your PC": click',
    noteMore: 'More info',
    noteRun: 'Run anyway',
    notePost: '. Only download from dl.omnimux.ai.',
  },

  footer: {
    brand: 'Desktop',
    tagline: 'Find, recreate & scale viral shoppable videos.',
    columns: [
      {
        title: 'Explore',
        links: [
          { label: 'Pricing', href: '/pricing' },
          { label: 'Inspiration', href: '/inspiration' },
          { label: 'Download Desktop', href: '#download' },
        ],
      },
      {
        title: 'Tools',
        links: [
          {
            label: 'TikTok Video Downloader',
            href: '/tools/tiktok-video-downloader',
          },
          {
            label: 'TikTok Photo Downloader',
            href: '/tools/tiktok-photo-downloader',
          },
          { label: 'Product-to-video', href: '#tools' },
          { label: 'Viral recreation', href: '#tools' },
          { label: 'Video to prompt', href: '#tools' },
          { label: 'Product image sets', href: '#tools' },
          { label: 'Detail-page images', href: '#tools' },
          { label: 'Image carousels', href: '#tools' },
          { label: 'Digital humans', href: '#tools' },
          { label: 'Subtitle burn-in', href: '#tools' },
        ],
      },
      {
        title: 'Models',
        links: [
          { label: 'Seedance 2.5', href: '#models' },
          { label: 'Seedance 2.0', href: '#models' },
          { label: 'GPT Image 2.5', href: '#models' },
          { label: 'Nano Banana', href: '#models' },
          { label: 'Suno', href: '#models' },
        ],
      },
      {
        title: 'Resources',
        links: [
          { label: 'Docs', href: 'https://docs.omnimux.ai', external: true },
          {
            label: 'Status',
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
        title: 'About',
        links: [
          { label: 'Contact', href: 'mailto:hello@omnimux.ai', external: true },
          { label: 'Privacy policy', href: '/privacy' },
          { label: 'Terms of service', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: 'Source code',
  },

  toolPage: {
    crumbAria: 'Breadcrumb',
    crumbTools: 'Tools',
    kindAria: 'Content type',
    kindVideo: 'Video',
    kindPhoto: 'Photo',
    inputLabel: 'TikTok link',
    placeholderVideo: 'Paste a TikTok video link or share text',
    placeholderPhoto: 'Paste a TikTok photo post link or share text',
    paste: 'Paste',
    parse: 'Parse',
    parsing: 'Parsing',
    verifying: 'Verifying',
    quotaAnon: 'Free today {remaining}/{limit}',
    quotaUser: 'Today {remaining}/{limit}',
    turnstileAria: 'Cloudflare verification',
    unavailable: 'Temporarily unavailable',
    downloadMp4: 'Download MP4',
    recreateMine: 'Recreate with my character',
    breakdown: 'Break down the viral structure',
    photoCount: '{count} photos',
    imageAlt: 'Photo {n}',
    downloadImage: 'Download photo {n}',
    downloadZip: 'Download all as ZIP',
    zipLocked: 'Sign in to use',
    recreatePhotos: 'Recreate this photo set',
    ctaTitle: 'Save the reference, then recreate it with your character',
    ctaBody: 'Same pacing and shots, with your own AI character and product.',
    ctaStart: 'Start recreating',
    ctaRef: 'Reference',
    ctaMine: 'Recreated',
    relatedAria: 'Related tools',
    errors: {
      invalidLink: 'Paste a TikTok link',
      wrongKindPhoto: 'This is a photo post link',
      toPhoto: 'Go to Photo Downloader',
      wrongKindVideo: 'This is a video link',
      toVideo: 'Go to Video Downloader',
      unavailable: 'This post is private, removed or unavailable',
      serviceUnavailable: 'Parsing is not open on this site yet',
      upstreamFailed: 'TikTok did not answer, try again shortly',
      quotaAnon: "Today's free parses are used up — sign in to continue",
      quotaUser: "Today's parses are used up — come back tomorrow",
      busy: "Today's free capacity is full — sign in to continue",
      rateLimited: 'Too fast, try again shortly',
      turnstileFailed: 'Verification failed, please try again',
      turnstileUnavailable:
        'Verification is temporarily unavailable, please try again later',
      generic: "Couldn't parse this link, try again later",
    },
  },
}
