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

export const zhCN: LandingCopy = {
  meta: { title: 'OmniMux 桌面端 — 发现、复刻和裂变电商爆款视频' },
  skip: '跳到正文',

  nav: {
    brand: '桌面端',
    home: '首页',
    models: '模型',
    tools: '工具',
    ext: '扩展',
    pricing: '价格',
    resources: '资源',
    download: '下载',
    login: '登录',
    menuOpen: '打开菜单',
    langTitle: '页面语言',
    brandAria: 'OmniMux 桌面端首页',
    navAria: '主导航',
    toolsMenu: {
      aria: '工具菜单',
      group: '创意',
      video: 'TikTok 视频下载',
      photo: 'TikTok 图文下载',
      agentGroup: 'Agent',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: '资源菜单',
      inspiration: '灵感社区',
      docs: '开发文档',
    },
    account: {
      aria: '账户菜单',
      dashboard: '控制台',
      profile: '个人资料',
      security: '安全与访问',
      wallet: '钱包',
      systemSettings: '系统设置',
      signOut: '退出登录',
    },
  },

  signOut: {
    title: '退出登录',
    body: '退出后需要重新登录才能访问你的账户。',
    cancel: '取消',
    confirm: '退出登录',
    busy: '退出中…',
    failed: '退出登录失败',
    done: '已退出登录',
    close: '关闭',
  },

  hero: {
    titleA: '发现、复刻和裂变',
    titleB: '电商爆款视频',
    subPre: '一套内容电商系统，让商品卖点贯穿每一次出片。用',
    subHighlight: '你熟悉的 Agent',
    subPost: '，一站式搞定广告、电商与种草视频。',
    tabsAria: '创作方式',
    tabs: { agent: 'Agent', video: '视频', image: '图片' },
    inputLabel: '描述你想做的内容',
    attachAria: '添加参考视频或图片',
    sendAria: '在桌面端开始',
    dlMac: '下载 macOS 版',
    dlWin: '下载 Windows 版',
    dlMenuAria: '选择下载版本',
    ghStar: '在 GitHub 点 Star',
    dlMacArm: 'macOS（Apple 芯片）',
    dlMacIntel: 'macOS（Intel）',
    dlWinX64: 'Windows（64 位）',
    skillsLabel: '技能',
    skillsAria: '技能',
    skills: [
      {
        icon: 'scan',
        label: '拆解视频/图文',
        fill: '拆解这条视频为什么能爆，整理开场钩子、卖点和购买引导 https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: '复刻视频',
        fill: '用 @精华液 复刻这条爆款，保留节奏和镜头结构，15 秒',
      },
      {
        icon: 'clap',
        label: '创作带货视频',
        fill: '给 @精华液 做一条 15 秒带货视频，开头先展示上脸效果',
      },
      {
        icon: 'img',
        label: '复刻带货图文',
        fill: '参考这组 Instagram 图文，给 @精华液 做一套 6 张种草图',
      },
      {
        icon: 'wand',
        label: '视频转提示词',
        fill: '把这条视频拆成可复用的提示词，分镜逐条列出',
      },
    ],
    modes: {
      agent: {
        prompts: [
          '拆解这条视频为什么能爆 https://www.tiktok.com/@pet.diary/video/73421',
          '用这条爆款给 @精华液 做 3 条，可直接发布，15 秒',
          '分析 @小鹿的护肤日记 最近 30 天的打法',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          '给 @精华液 做一条 15 秒带货视频，开头先展示上脸效果',
          '蓝牙耳机高空跌落实测，最后揭晓限时折扣',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          '给 @精华液 做一套 8 张商品套图：主图、卖点、细节、规格、场景',
          '把这张商品图换成大理石梳妆台场景',
        ],
      },
    },
    reelAria: '内置成片模板样片',
    reel: [
      { title: '3D 视效粒子', tag: '宠物粮' },
      { title: '模特走秀换装', tag: '背带裤' },
      { title: '跌落耐用实测', tag: '蓝牙耳机' },
      { title: '达人自拍口播', tag: '益生菌' },
      { title: '巨型商品追逐', tag: '购物袋' },
      { title: '手机网页实机演示', tag: '旅行 App' },
      { title: '15 秒大促广告', tag: '零食' },
    ],
  },

  agentDemo: {
    h2: '一次对话，复刻带货爆款',
    lead: '告诉它你卖什么，它会拆解参考视频、编写脚本、生成内容并安排发布。',
    windowAria: 'OmniMux 桌面端 Agent 演示',
    windowTitle: '为我的 3 个爆品复刻这条爆款',
    windowStatus: '3 条视频已生成',
    replay: '重播演示',
    replayAria: '重播演示',
    side: [
      '新会话',
      '应用',
      '画布',
      '资产库',
      '灵感库',
      '商品库',
      '发布',
      '数据',
    ],
    me: '林',
    msgPre: '用这条爆款给',
    msgProducts: ['@宠物粮', '@背带裤', '@蓝牙耳机'],
    msgPost: '各做一条，可直接发布，15 秒。',
    linkCard: 'tiktok.com/@pet.diary/video/73421 · 撸猫日常，猫粮开箱',
    doneBreakdown: '已拆解参考视频为什么能爆',
    kv: [
      { k: '开头', v: '前 2 秒用前后对比' },
      { k: '证据', v: '使用过程特写' },
      { k: '结尾', v: '引导点击小黄车' },
    ],
    doneProducts: '已分析你的 3 个商品及卖点',
    doneScripts: '已为每个商品写好脚本',
    confirmTitle: '生成 3 条视频？',
    confirmNeed: '需要你确认',
    confirmCost: '预计消耗积分，确认后扣除',
    confirmGo: '确认并生成',
    confirmEdit: '修改',
    inputPh: '给 OmniMux 发消息，@ 引用商品或素材',
    resultsTitle: '成片',
    resultsMeta: '3 条 · 9:16',
    generating: '生成中…',
    vidNames: ['宠物粮', '背带裤', '蓝牙耳机'],
    vidHooks: ['开头 · 概念对比', '开头 · 换装', '开头 · 特写'],
    schedTitle: '排期发布',
    schedOk: '已排期',
    schedPlatform: 'TikTok',
    schedTimes: ['周二 19:00', '周三 12:30', '周四 19:00'],
    schedAll: '全部排期',
  },

  flow: {
    h2: '从商品创意到带货视频',
    lineAria: '对标 · 拆解 · 制作 · 发布 · 复盘',
    steps: [
      {
        kind: '对标',
        title: '找到同类目的爆款带货视频',
        desc: '把在别处刷到的参考视频收进来，拆解它的开场钩子和卖点。',
        chips: ['爆款带货视频和图文'],
        primary: '拆解一条参考视频',
        secondary: '浏览器扩展采集灵感',
      },
      {
        kind: '拆解',
        title: '看懂它为什么能卖',
        desc: '拆解爆款视频的开场钩子、卖点和购买引导，整理成可复用的结构。也能分析竞品账号的打法。',
        chips: ['视频脚本拆解', '竞品账号分析', '品类趋势'],
        primary: '拆解一条爆款',
      },
      {
        kind: '制作',
        title: '把你的商品做成带货视频',
        desc: '用商品图或链接生成原创视频，或用你的商品复刻爆款。',
        primary: '生成视频',
        secondary: '复刻爆款',
      },
      {
        kind: '发布',
        title: '发布到 TikTok、Instagram、Facebook 和 YouTube',
        desc: '连接你的账号，即时或定时发布带货视频和图文。',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: '发布视频',
        secondary: '视频发布 · 图文发布',
      },
      {
        kind: '复盘',
        title: '用真实反馈，开始下一轮',
        desc: '连接账号后，播放、点赞和评论数据会自动同步，Agent 告诉你下一条该做什么。',
        primary: '分析账号',
      },
    ],

    discover: {
      title: '参考视频示例 · 流程示意',
      meta: '近 7 天',
      savesPrefix: '收藏',
      items: [
        {
          who: '小桃的穿搭本',
          views: '4.6万',
          length: '图文 · 6 张',
          title: '波点裙一衣三穿，通勤约会都不踩雷 #穿搭分享',
          likes: '4.6万',
          saves: '3.1万',
        },
        {
          who: '通勤十分钟',
          views: '270万',
          length: '00:28',
          title: '车里补妆神器，单手就能上 #通勤妆容',
          likes: '8.9万',
          saves: '2.4万',
        },
        {
          who: '干皮研究所',
          views: '160万',
          length: '00:34',
          title: '换季泛红就靠它，滴管精华上脸实测 #敏感肌',
          likes: '5.2万',
          saves: '1.8万',
        },
      ],
    },

    breakdown: {
      tabsAria: '拆解对象',
      tabs: { video: '视频', account: '竞品账号 · 示意', category: '品类趋势 · 示意' },
      statsViews: '1500万+ 播放',
      statsLikes: '32万+ 点赞',
      why: '为什么能卖',
      rows: [
        {
          label: '开场钩子',
          time: '0–2s',
          desc: '结果先行，先展示效果再出现商品',
        },
        { label: '产品卖点', time: '3–9s', desc: '实测特写，口播三个卖点' },
        { label: '购买引导', time: '10–14s', desc: '限时优惠，引导点击小黄车' },
      ],
      reuse: '可复用的结构',
      chain: ['结果先行', '实测证据', '优惠 + 小黄车'],
      accountName: '@小鹿的护肤日记',
      accountMeta: '美妆个护',
      thumbs: ['210万', '140万', '86万', '51万'],
      metrics: [
        { label: '发布视频 · 近 30 天', value: '42' },
        { label: '平均播放', value: '18.6万' },
        { label: '带货视频', value: '71%' },
      ],
      insights: [
        {
          k: '打法',
          v: '播放前 10 的视频里有 7 条是「结果先行」开头，再切实测特写，这是这个账号的主要打法。',
        },
        {
          k: '节奏',
          v: '晚上 7–9 点发布的视频，播放比其他时段高约 40%。',
        },
        {
          k: '机会',
          v: '同类目还没人把「前后对比」和「价格锚点」放在一起用，值得先试。',
        },
      ],
      accountCta: '用这套打法给 @精华液 做 3 条',
      catTitle: '美妆个护',
      catMeta: '近 7 天 · 热门 200 条 · 示例数据',
      bars: [
        { label: '前后对比', value: '38%' },
        { label: '开箱试用', value: '24%' },
        { label: '实测特写', value: '19%' },
        { label: '口播种草', value: '12%' },
        { label: '剧情反转', value: '7%' },
      ],
      catInsights: [
        { k: '开头', v: '「前后对比」开头占热门视频的 38%，平均播放最高。' },
        { k: '时长', v: '15–25 秒的视频最多，超过 40 秒的明显变少。' },
        {
          k: '机会',
          v: '「实测特写」+「限时优惠」的组合还不多，适合新品切入。',
        },
      ],
      catCta: '据此给我出一份对标方案',
    },

    make: {
      tabsAria: '制作方式',
      tabs: { generate: '直接生成', replicate: '复刻爆款' },
      capProduct: '商品',
      capResult: '成片',
      capRef: '参考爆款',
      capMine: '你的商品',
      badgeGenerated: '已生成',
      badgeRef: '▶ TikTok · 210万',
      arrowReplicateA: '复刻结构',
      arrowReplicateB: '换成你的商品',
      promptLabel: '提示词',
      promptGenerate:
        '蓝牙耳机：高空跌落实测，镜头跟拍落地瞬间，完好无损，最后揭晓限时折扣，真人出镜口播风格。',
      intentLabel: '改写意图',
      promptReplicate:
        '保留参考视频的自拍口播节奏：开头吐槽痛点，中段手持展示，结尾说出真实感受。复刻的是思路，不是原片。',
      altProduct: '蓝牙耳机商品图',
      altRef: '参考爆款视频封面',
      altCan: '益生菌商品图',
    },

    publish: {
      title: '发布计划',
      meta: '本周',
      posts: [
        {
          title: '第一口就上头的薯片，追剧必备',
          platformLabel: 'TikTok',
          account: '@零食研究所',
          time: '今天 18:30',
          state: '已发布',
        },
        {
          title: '挑食的狗子也会抢着吃',
          platformLabel: 'TikTok',
          account: '@毛孩子的饭盆',
          time: '今天 20:00',
          state: '已排期',
        },
        {
          title: '一条背带裤，一周不重样',
          platformLabel: 'Instagram',
          account: '@穿搭日记',
          time: '明天 12:00',
          state: '就绪',
        },
        {
          title: '吃了一个月益生菌的真实感受',
          platformLabel: 'YouTube',
          account: '@好物种草社',
          time: '明天 19:00',
          state: '处理中',
        },
      ],
      feats: [
        {
          title: '多平台一次提交',
          desc: 'TikTok、Instagram、Facebook、YouTube，选好账号一次提交，每个账号单独跟踪状态。',
        },
        {
          title: '账号统一管理',
          desc: '在桌面端连接各平台账号，发布时直接选择，不用来回切换。',
        },
        {
          title: '状态记录与失败重试',
          desc: '每条任务都有状态记录，失败的任务可以重试。',
        },
      ],
    },

    review: {
      title: '账号表现',
      synced: '已同步 · 刚刚',
      meta: '最近 7 天',
      kpis: [
        { label: '播放量', value: '12.8万', delta: '+18%' },
        { label: '点赞', value: '8,640', delta: '+24%' },
        { label: '互动率', value: '5.2%', delta: '+0.8pt' },
      ],
      days: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
      splitTitle: '播放来源 · 按开头类型',
      splitLegs: ['前后对比开头 63%', '口播开头 37%'],
      note: '「前后对比」开头的视频贡献了 63% 的播放，晚上 7–9 点发布表现最好。',
      cta: '开始下一轮：沿用这个钩子再做 3 条',
    },
  },

  create: {
    tag: '创作 Agent',
    h2: '交给 Agent，或者自己搭流程',
    lead: '通过 OmniMux Agent 对话创作内容，或在画布中连接图片、视频和语音工具，搭建可复用的工作流。',
    msgProductCard: '能量饮料 · 主图',
    msgAsk: '为这款能量饮料做一套商品图。',
    doneAnalyze: '已分析商品：外形、标签与配色',
    donePlan: '已规划 8 张套图：主图、卖点、细节、规格、场景',
    doneConsistent: '每张图里保持商品完全一致',
    confirmTitle: '生成 8 张图片？',
    confirmNeed: '需要你确认',
    paramCount: '8 张',
    confirmGo: '确认并生成',
    confirmEdit: '修改',
    doneResult: '套图已完成，8 张已放上画布',
    tabsAria: '工作区',
    tabs: {
      canvas: '画布',
      assets: '资产库',
      inspiration: '灵感库',
      products: '商品库',
    },
    groupLabel: '商品套图 · 8',
    setCaptions: [
      '主图',
      '卖点',
      '细节',
      '规格',
      '场景',
      '场景',
      '氛围',
      '海报',
    ],
    libAssets: [
      { label: '角色', sub: '小鹿 · 3 个视角' },
      { label: '场景', sub: '开放式厨房' },
      { label: '风格', sub: '霓虹赛博' },
      { label: '道具', sub: '背带裤' },
    ],
    libInspiration: [
      { label: '唇釉试色', sub: 'TikTok · 300万播放' },
      { label: '车内补妆', sub: 'TikTok · 270万播放' },
      { label: '精华实测', sub: 'Instagram · 1.8万收藏' },
      { label: '追逐撞屏', sub: '已拆解' },
    ],
    libProducts: [
      { name: '能量饮料', sub: '0 糖 · 提神 · 冰爽口感', count: '8 张套图' },
      {
        name: '蓝牙耳机',
        sub: '降噪 · 续航 30 小时 · 耐摔',
        count: '2 条视频',
      },
      { name: '益生菌', sub: '每日一勺 · 独立罐装 · 便携', count: '1 条视频' },
    ],
    capAgentTitle: '说出需求，Agent 完成整套创作',
    capAgentDesc:
      '从商品图、视频到脚本，Agent 规划每一步，并让同一商品在整套内容里保持一致。',
    capAgentCta: '开始创作',
    capCanvasTitle: '画布：可视化创作工作流',
    capCanvasDesc:
      '把图片、视频和配音节点连成工作流，任何一步都能单独调整，再次运行。',
    capCanvasCta: '打开画布',
    canvasAria: '画布工作流示例',
    nodeImage: '图片',
    nodeText: '文本',
    nodeTextBody: '霓虹灯光，深色背景，罐身特写，冰雾和水珠',
    nodeAIImage: 'AI 图片',
    nodeAIVideo: 'AI 视频',
    nodeCut: '剪辑导出',
    nodeDone: '已完成',
    nodeCutBodyA: '加字幕、配乐',
    nodeCutBodyB: '导出 9:16 竖版',
  },

  surfaces: {
    h2: '在你常用的地方直接用',
    lead: '同一个 Agent，桌面端、Codex、Claude Code 和浏览器里都能用。',
    desktop: {
      title: 'OmniMux 桌面端',
      desc: '完整工作台：画布、素材、商品和全部工具，macOS 与 Windows 都能用。',
      input: '给我的 3 款精华各做一条视频…',
      cta: '下载桌面端',
    },
    cli: {
      title: 'Codex、Claude Code 等 Agent',
      desc: '安装 OmniMux CLI，在常用的 Agent 中调用视频、图片和分析工具。',
      cta: '安装 CLI',
    },
    ext: {
      title: '浏览器扩展',
      desc: '在网页上刷到爆款，一键收进灵感库，回到桌面端接着拆解，不用来回切换。',
      capSave: '收进灵感库',
      capScan: '拆解',
      toast: '已保存到灵感库',
      cta: '在桌面端安装扩展',
    },
  },

  tools: {
    h2: 'AI 生成带货视频、图片与配音素材',
    lead: '制作商品素材、多平台内容、再利用已有视频——都在同一个工作区完成。',
    centerLabel: '内容制作',
    groups: [
      {
        title: '商品上新',
        lead: '让商品图达到上架标准',
        items: [
          { icon: 'img', label: '白底商品图' },
          { icon: 'layers', label: '多角度产品图' },
          { icon: 'grid', label: '商品套图' },
          { icon: 'box', label: '商品详情图' },
          { icon: 'spark', label: '场景图' },
          { icon: 'wand', label: '参考图改款' },
        ],
      },
      {
        title: '带货视频',
        lead: '从零生成，或照着爆款做',
        items: [
          { icon: 'link', label: '商品生视频' },
          { icon: 'video', label: '视频生成' },
          { icon: 'flame', label: '爆款复刻' },
          { icon: 'wand', label: '视频转提示词' },
          { icon: 'grid', label: '分镜脚本' },
          { icon: 'img', label: '首帧生视频' },
        ],
      },
      {
        title: '图文带货',
        lead: '一组图也能卖货',
        items: [
          { icon: 'layers', label: '图文轮播' },
          { icon: 'scan', label: '参考图复刻' },
          { icon: 'img', label: '图片生成' },
          { icon: 'shirt', label: '服装上身' },
        ],
      },
      {
        title: '口播配音',
        lead: '声音、角色和出镜人',
        items: [
          { icon: 'user', label: '数字人形象' },
          { icon: 'users', label: '多视角角色' },
          { icon: 'mic', label: '配音与音效' },
          { icon: 'music', label: 'AI 配乐' },
        ],
      },
      {
        title: '多平台适配',
        lead: '一套素材，适配多个平台',
        items: [
          { icon: 'crop', label: '画幅适配' },
          { icon: 'hash', label: '标题与话题' },
        ],
      },
      {
        title: '素材二创',
        lead: '剪辑、拆分、字幕烧录，处理已有视频素材。',
        items: [
          { icon: 'cut', label: '视频剪辑' },
          { icon: 'scan', label: '镜头拆分' },
          { icon: 'subs', label: '字幕烧录' },
        ],
      },
    ],
    modelsTitle: '顶级 AI 模型，为卖货而调校',
    modelsAria: '可用模型',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: '视频' },
      { mark: 'S', name: 'Seedance 2.0', kind: '视频' },
      { mark: 'G', name: 'GPT Image 2.5', kind: '图片' },
      { mark: 'N', name: 'Nano Banana', kind: '图片' },
      { mark: '♪', name: 'Suno', kind: '音乐' },
      { mark: 'G', name: 'Gemini', kind: '理解与脚本' },
    ],
    chipsAria: '模型能力',
    capabilityRows: [
      [
        { icon: 'monitor', label: '480p / 720p' },
        { icon: 'crop', label: '9:16' },
        { icon: 'crop', label: '16:9' },
        { icon: 'crop', label: '1:1' },
        { icon: 'crop', label: '3:4' },
        { icon: 'crop', label: '4:3' },
        { icon: 'clock', label: '4–30 秒输出' },
      ],
      [
        { icon: 'layers', label: '多模态参考' },
        { icon: 'user', label: '角色一致性' },
        { icon: 'img', label: '首帧控制' },
        { icon: 'music', label: '原生音频' },
        { icon: 'spark', label: '商品一致性' },
      ],
      [
        { icon: 'cut', label: '本地剪辑' },
        { icon: 'scan', label: '镜头拆分' },
        { icon: 'subs', label: '字幕烧录' },
        { icon: 'mic', label: '音频提取' },
        { icon: 'layers', label: '图文成片' },
      ],
    ],
  },

  workspace: {
    h2: '一个工作区，素材都在手边',
    lead: '商品、素材、灵感和发布记录都在同一个桌面端，随时调用，不用来回找。',
    list: [
      { name: '商品库', desc: '卖点、人群和商品图' },
      { name: '资产库', desc: '角色、场景、风格和提示词' },
      { name: '灵感库', desc: '收藏的爆款和拆解结果' },
      { name: '发布中心', desc: '草稿、排期和发布记录' },
    ],
    note: '支持 macOS 与 Windows。',
    tableTitle: '商品库',
    addProduct: '添加商品',
    cols: ['商品', '卖点', '本月内容'],
    rows: [
      { name: '能量饮料', category: '食品饮料', points: '3 个' },
      { name: '蓝牙耳机', category: '数码', points: '4 个' },
      { name: '背带裤', category: '服饰', points: '3 个' },
      { name: '益生菌', category: '保健食品', points: '2 个' },
    ],
  },

  proof: {
    h2: '为跨境电商卖家打造',
    lead: '从找参考、拆解、制作到发布，在一个桌面端里完成。',
    facts: [
      { value: '17', label: '个内置技能' },
      { value: '7', label: '套成片模板' },
      { value: '4', label: '个发布平台' },
    ],
    platformsAria: '支持发布的平台',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: ['生成前确认参数与积分', '任务失败自动退还积分'],
  },

  faq: {
    h2: '常见问题',
    lead: '电商卖家最关心的问题',
    items: [
      {
        q: 'OmniMux 是开源的吗？',
        a: '是。桌面端、插件与网关源码按版本发布在 github.com/omnimux-ai/omnimux-ai（AGPL-3.0）。托管服务的渠道凭据、定价账本、运维与用户数据不在开源范围内。',
      },
      {
        q: 'OmniMux Agent 能做什么？',
        a: 'OmniMux Agent 可以分析参考视频、编写脚本、生成视频和图片，并协助把内容发布到你已经连接的平台。用日常语言说明你的商品和目标，它会为每个步骤选择合适的工具。',
      },
      {
        q: 'Agent 会不经我同意就生成或发布吗？',
        a: '不会。生成视频或图片、发布内容之前，Agent 都会先展示参数和积分消耗，等你确认；确认前可以修改参数。如果任务因系统问题失败，已扣除的积分会退还。',
      },
      {
        q: '如何用 AI 制作带货视频？',
        a: '粘贴商品链接或选择爆款参考，生成原创或复刻视频，再发布到你已连接的平台。无需拍摄或剪辑。',
      },
      {
        q: '生成的视频可以直接发布吗？',
        a: '可以。在桌面端连接一次账号，就能直接发布，无需手动上传。发布时可按平台要求添加 AI 生成内容声明。',
      },
      {
        q: '用 AI 做的视频会不会导致账号被封？',
        a: 'OmniMux 不承诺防封号。账号是否安全，取决于你的内容和账号行为。发布时可为视频添加 AI 生成内容声明，请遵守各平台规则。',
      },
      {
        q: '是否支持商品图片转视频？',
        a: '支持。上传商品图片或粘贴商品链接，AI 会生成可直接发布的展示视频，无需拍摄。',
      },
      {
        q: '怎么复刻竞品的爆款视频？',
        a: 'AI 学习对方的创作手法（节奏、构图、调性），用你自己的商品和原创画面生成一条全新视频——复刻的是思路，不是原片。',
      },
      {
        q: '生成的视频可以商用吗？',
        a: '商用范围以《服务条款》为准。上传的人物照片、商品图和参考视频，需要由你确认拥有使用授权，并遵守各平台规则。',
      },
    ],
  },

  finalCta: {
    h2: '下一条爆款，交给 OmniMux',
    lead: '拆解爆款、生成视频、发布并复盘，一个 Agent 跑完整个闭环。',
    dlMac: '免费下载 macOS 版',
    dlWin: '下载 Windows 版',
    note: 'macOS 12+（Apple 芯片与 Intel 通用）· Windows 10 / 11 x64',
  },

  download: {
    title: '在 OmniMux 桌面端继续',
    desc: '下载安装后登录，即可开始创作。',
    close: '关闭',
    macTitle: '下载 macOS 版（Universal）',
    macSub: 'macOS 12+ · Apple 芯片与 Intel 通用',
    winTitle: '下载 Windows 版（x64 安装包）',
    winSub: 'Windows 10 / 11 x64',
    recommended: '推荐',
    notePre: '若 Windows 提示「已保护你的电脑」：点击',
    noteMore: '更多信息',
    noteRun: '仍要运行',
    notePost: '。请仅从 dl.omnimux.ai 下载。',
  },

  footer: {
    brand: '桌面端',
    tagline: '发现、复刻和裂变电商爆款视频。',
    columns: [
      {
        title: '探索',
        links: [
          { label: '价格', href: '/pricing' },
          { label: '灵感社区', href: '/inspiration' },
          { label: '下载桌面端', href: '#download' },
        ],
      },
      {
        title: '工具',
        links: [
          {
            label: 'TikTok 视频下载',
            href: '/zh/tools/tiktok-video-downloader',
          },
          {
            label: 'TikTok 图文下载',
            href: '/zh/tools/tiktok-photo-downloader',
          },
          { label: '商品生视频', href: '#tools' },
          { label: '爆款复刻', href: '#tools' },
          { label: '视频转提示词', href: '#tools' },
          { label: '商品套图', href: '#tools' },
          { label: '商品详情图', href: '#tools' },
          { label: '图文轮播', href: '#tools' },
          { label: '数字人形象', href: '#tools' },
          { label: '字幕烧录', href: '#tools' },
        ],
      },
      {
        title: '模型',
        links: [
          { label: 'Seedance 2.5', href: '#models' },
          { label: 'Seedance 2.0', href: '#models' },
          { label: 'GPT Image 2.5', href: '#models' },
          { label: 'Nano Banana', href: '#models' },
          { label: 'Suno', href: '#models' },
        ],
      },
      {
        title: '资源',
        links: [
          { label: '文档', href: 'https://docs.omnimux.ai', external: true },
          {
            label: '服务状态',
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
        title: '关于',
        links: [
          {
            label: '联系我们',
            href: 'mailto:hello@omnimux.ai',
            external: true,
          },
          { label: '隐私政策', href: '/privacy' },
          { label: '服务条款', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: '开源仓库',
  },

  toolPage: {
    crumbAria: '面包屑',
    crumbTools: '工具',
    kindAria: '内容类型',
    kindVideo: '视频',
    kindPhoto: '图文',
    inputLabel: 'TikTok 链接',
    placeholderVideo: '粘贴 TikTok 视频链接或分享文字',
    placeholderPhoto: '粘贴 TikTok 图文链接或分享文字',
    paste: '粘贴',
    parse: '解析',
    parsing: '解析中',
    verifying: '验证中',
    quotaAnon: '今日免费 {remaining}/{limit}',
    quotaUser: '今日 {remaining}/{limit}',
    turnstileAria: 'Cloudflare 人机验证',
    unavailable: '暂不可用',
    downloadMp4: '下载 MP4',
    recreateMine: '用我的角色复刻',
    breakdown: '拆解爆款结构',
    photoCount: '{count} 张',
    imageAlt: '第 {n} 张',
    downloadImage: '下载第 {n} 张',
    downloadZip: '全部下载 ZIP',
    zipLocked: '登录后可用',
    recreatePhotos: '复刻这组图文',
    ctaTitle: '保存参考，再用你的角色复刻',
    ctaBody: '同样的节奏和镜头，换成你自己的 AI 角色和商品。',
    ctaStart: '开始复刻',
    ctaRef: '参考',
    ctaMine: '复刻',
    relatedAria: '相关工具',
    errors: {
      invalidLink: '请粘贴 TikTok 链接',
      wrongKindPhoto: '这是图文链接',
      toPhoto: '去图文下载',
      wrongKindVideo: '这是视频链接',
      toVideo: '去视频下载',
      unavailable: '该内容不公开或已删除，无法解析',
      serviceUnavailable: '解析服务暂未开放',
      upstreamFailed: 'TikTok 暂时没有响应，请稍后再试',
      quotaAnon: '今日免费次数已用完，登录后继续',
      quotaUser: '今日次数已用完，明天再来',
      busy: '今日免费额度已满，登录后继续使用',
      rateLimited: '操作太快，请稍后再试',
      turnstileFailed: '人机验证未通过，请重试',
      turnstileUnavailable: '验证服务暂时不可用，请稍后再试',
      generic: '解析失败，请稍后再试',
    },
  },
}
