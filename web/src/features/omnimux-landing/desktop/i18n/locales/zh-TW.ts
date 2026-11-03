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

export const zhTW: LandingCopy = {
  meta: { title: 'OmniMux 桌面版 — 發現、複刻和裂變電商爆款影片' },
  skip: '跳到主要內容',

  nav: {
    brand: '桌面版',
    home: '首頁',
    models: '模型',
    tools: '工具',
    ext: '擴充功能',
    pricing: '價格',
    resources: '資源',
    download: '下載',
    login: '登入',
    menuOpen: '開啟選單',
    langTitle: '頁面語言',
    brandAria: 'OmniMux 桌面版首頁',
    navAria: '主導覽',
    toolsMenu: {
      aria: '工具選單',
      group: '創意',
      video: 'TikTok 影片下載',
      photo: 'TikTok 圖文下載',
      agentGroup: 'Agent',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: '資源選單',
      inspiration: '靈感社群',
      docs: '開發文件',
    },
    account: {
      aria: '帳戶選單',
      dashboard: '控制台',
      profile: '個人資料',
      security: '安全性與存取',
      wallet: '錢包',
      systemSettings: '系統設定',
      signOut: '登出',
    },
  },

  signOut: {
    title: '登出',
    body: '登出後需要重新登入才能存取你的帳戶。',
    cancel: '取消',
    confirm: '登出',
    busy: '登出中…',
    failed: '登出失敗',
    done: '已登出',
    close: '關閉',
  },

  hero: {
    titleA: '發現、複刻和裂變',
    titleB: '電商爆款影片',
    subPre: '一套內容電商系統，讓商品賣點貫穿每一次出片。用',
    subHighlight: '你熟悉的 Agent',
    subPost: '，一站式搞定廣告、電商與種草影片。',
    tabsAria: '創作方式',
    tabs: { agent: 'Agent', video: '影片', image: '圖片' },
    inputLabel: '描述你想做的內容',
    attachAria: '加入參考影片或圖片',
    sendAria: '在桌面版開始',
    dlMac: '下載 macOS 版',
    dlWin: '下載 Windows 版',
    dlMenuAria: '選擇下載版本',
    ghStar: '在 GitHub 點 Star',
    dlMacArm: 'macOS（Apple 晶片）',
    dlMacIntel: 'macOS（Intel）',
    dlWinX64: 'Windows（64 位元）',
    skillsLabel: '技能',
    skillsAria: '技能',
    skills: [
      {
        icon: 'scan',
        label: '拆解影片/圖文',
        fill: '拆解這支影片為什麼會爆，整理開場鉤子、賣點和購買引導 https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: '複刻影片',
        fill: '用 @精華液 複刻這支爆款，保留節奏和鏡頭結構，15 秒',
      },
      {
        icon: 'clap',
        label: '創作帶貨影片',
        fill: '幫 @精華液 做一支 15 秒帶貨影片，開頭先展示上臉效果',
      },
      {
        icon: 'img',
        label: '複刻帶貨圖文',
        fill: '參考這組 Instagram 圖文，幫 @精華液 做一套 6 張種草圖',
      },
      {
        icon: 'wand',
        label: '影片轉提示詞',
        fill: '把這支影片拆成可重複運用的提示詞，分鏡逐條列出',
      },
    ],
    modes: {
      agent: {
        prompts: [
          '拆解這支影片為什麼會爆 https://www.tiktok.com/@pet.diary/video/73421',
          '用這支爆款幫 @精華液 做 3 支，可直接發佈，15 秒',
          '分析 @小鹿的護膚日記 最近 30 天的打法',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          '幫 @精華液 做一支 15 秒帶貨影片，開頭先展示上臉效果',
          '藍牙耳機高空跌落實測，最後揭曉限時折扣',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          '幫 @精華液 做一套 8 張商品套圖：主圖、賣點、細節、規格、場景',
          '把這張商品圖換成大理石梳妝台場景',
        ],
      },
    },
    reelAria: '內建成片模板樣片',
    reel: [
      { title: '3D 視覺特效粒子', tag: '寵物糧' },
      { title: '模特走秀換裝', tag: '吊帶褲' },
      { title: '跌落耐用實測', tag: '藍牙耳機' },
      { title: '達人自拍口播', tag: '益生菌' },
      { title: '巨型商品追逐', tag: '購物袋' },
      { title: '手機網頁實機演示', tag: '旅遊 App' },
      { title: '15 秒大促廣告', tag: '零食' },
    ],
  },

  agentDemo: {
    h2: '一次對話，複刻帶貨爆款',
    lead: '告訴它你賣什麼，它會拆解參考影片、撰寫腳本、生成內容並安排發佈。',
    windowAria: 'OmniMux 桌面版 Agent 演示',
    windowTitle: '為我的 3 個爆品複刻這支爆款',
    windowStatus: '已生成 3 支影片',
    replay: '重播演示',
    replayAria: '重播演示',
    side: [
      '新對話',
      '應用',
      '畫布',
      '資產庫',
      '靈感庫',
      '商品庫',
      '發佈',
      '數據',
    ],
    me: '林',
    msgPre: '用這支爆款幫',
    msgProducts: ['@寵物糧', '@吊帶褲', '@藍牙耳機'],
    msgPost: '各做一支，可直接發佈，15 秒。',
    linkCard: 'tiktok.com/@pet.diary/video/73421 · 擼貓日常，貓糧開箱',
    doneBreakdown: '已拆解參考影片為什麼會爆',
    kv: [
      { k: '開頭', v: '前 2 秒用前後對比' },
      { k: '證據', v: '使用過程特寫' },
      { k: '結尾', v: '引導點擊小黃車' },
    ],
    doneProducts: '已分析你的 3 個商品及賣點',
    doneScripts: '已為每個商品寫好腳本',
    confirmTitle: '生成 3 支影片？',
    confirmNeed: '需要你確認',
    confirmCost: '預計消耗積分，確認後扣除',
    confirmGo: '確認並生成',
    confirmEdit: '修改',
    inputPh: '傳訊息給 OmniMux，@ 引用商品或素材',
    resultsTitle: '成片',
    resultsMeta: '3 支 · 9:16',
    generating: '生成中…',
    vidNames: ['寵物糧', '吊帶褲', '藍牙耳機'],
    vidHooks: ['開頭 · 概念對比', '開頭 · 換裝', '開頭 · 特寫'],
    schedTitle: '排程發佈',
    schedOk: '已排期',
    schedPlatform: 'TikTok',
    schedTimes: ['週二 19:00', '週三 12:30', '週四 19:00'],
    schedAll: '全部排程',
  },

  flow: {
    h2: '從商品創意到帶貨影片',
    lineAria: '對標 · 拆解 · 製作 · 發佈 · 復盤',
    steps: [
      {
        kind: '對標',
        title: '找到同品類的爆款帶貨影片',
        desc: '把在別處刷到的參考影片收進來，拆解它的開場鉤子和賣點。',
        chips: ['爆款帶貨影片和圖文'],
        primary: '拆解一支參考影片',
        secondary: '瀏覽器擴充功能採集靈感',
      },
      {
        kind: '拆解',
        title: '看懂它為什麼能賣',
        desc: '拆解爆款影片的開場鉤子、賣點和購買引導，整理成可重複運用的結構。也能分析競品帳號的打法。',
        chips: ['影片腳本拆解', '競品帳號分析', '品類趨勢'],
        primary: '拆解一支爆款',
      },
      {
        kind: '製作',
        title: '把你的商品做成帶貨影片',
        desc: '用商品圖或連結生成原創影片，或用你的商品複刻爆款。',
        primary: '生成影片',
        secondary: '複刻爆款',
      },
      {
        kind: '發佈',
        title: '發佈到 TikTok、Instagram、Facebook 和 YouTube',
        desc: '連結你的帳號，即時或排程發佈帶貨影片和圖文。',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: '發佈影片',
        secondary: '影片發佈 · 圖文發佈',
      },
      {
        kind: '復盤',
        title: '用真實回饋，開始下一輪',
        desc: '連結帳號後，播放、按讚和留言數據會自動同步，Agent 告訴你下一支該做什麼。',
        primary: '分析帳號',
      },
    ],

    discover: {
      title: '參考影片示例 · 流程示意',
      meta: '近 7 天',
      savesPrefix: '收藏',
      items: [
        {
          who: '小桃的穿搭本',
          views: '4.6萬',
          length: '圖文 · 6 張',
          title: '波點裙一衣三穿，通勤約會都不踩雷 #穿搭分享',
          likes: '4.6萬',
          saves: '3.1萬',
        },
        {
          who: '通勤十分鐘',
          views: '270萬',
          length: '00:28',
          title: '車裡補妝神器，單手就能上 #通勤妝容',
          likes: '8.9萬',
          saves: '2.4萬',
        },
        {
          who: '乾肌研究所',
          views: '160萬',
          length: '00:34',
          title: '換季泛紅就靠它，滴管精華上臉實測 #敏感肌',
          likes: '5.2萬',
          saves: '1.8萬',
        },
      ],
    },

    breakdown: {
      tabsAria: '拆解對象',
      tabs: { video: '影片', account: '競品帳號 · 示意', category: '品類趨勢 · 示意' },
      statsViews: '1500萬+ 播放',
      statsLikes: '32萬+ 按讚',
      why: '為什麼能賣',
      rows: [
        {
          label: '開場鉤子',
          time: '0–2s',
          desc: '結果先行，先展示效果再出現商品',
        },
        { label: '產品賣點', time: '3–9s', desc: '實測特寫，口播三個賣點' },
        { label: '購買引導', time: '10–14s', desc: '限時優惠，引導點擊小黃車' },
      ],
      reuse: '可重複運用的結構',
      chain: ['結果先行', '實測證據', '優惠 + 小黃車'],
      accountName: '@小鹿的護膚日記',
      accountMeta: '美妝個護',
      thumbs: ['210萬', '140萬', '86萬', '51萬'],
      metrics: [
        { label: '發佈影片 · 近 30 天', value: '42' },
        { label: '平均播放', value: '18.6萬' },
        { label: '帶貨影片', value: '71%' },
      ],
      insights: [
        {
          k: '打法',
          v: '播放前 10 的影片裡有 7 支是「結果先行」開頭，再切實測特寫，這是這個帳號的主要打法。',
        },
        {
          k: '節奏',
          v: '晚上 7–9 點發佈的影片，播放比其他時段高約 40%。',
        },
        {
          k: '機會',
          v: '同品類還沒人把「前後對比」和「價格錨點」放在一起用，值得先試。',
        },
      ],
      accountCta: '用這套打法幫 @精華液 做 3 支',
      catTitle: '美妝個護',
      catMeta: '近 7 天 · 熱門 200 支 · 示例資料',
      bars: [
        { label: '前後對比', value: '38%' },
        { label: '開箱試用', value: '24%' },
        { label: '實測特寫', value: '19%' },
        { label: '口播種草', value: '12%' },
        { label: '劇情反轉', value: '7%' },
      ],
      catInsights: [
        { k: '開頭', v: '「前後對比」開頭佔熱門影片的 38%，平均播放最高。' },
        { k: '長度', v: '15–25 秒的影片最多，超過 40 秒的明顯變少。' },
        {
          k: '機會',
          v: '「實測特寫」+「限時優惠」的組合還不多，適合新品切入。',
        },
      ],
      catCta: '據此幫我出一份對標方案',
    },

    make: {
      tabsAria: '製作方式',
      tabs: { generate: '直接生成', replicate: '複刻爆款' },
      capProduct: '商品',
      capResult: '成片',
      capRef: '參考爆款',
      capMine: '你的商品',
      badgeGenerated: '已生成',
      badgeRef: '▶ TikTok · 210萬',
      arrowReplicateA: '複刻結構',
      arrowReplicateB: '換成你的商品',
      promptLabel: '提示詞',
      promptGenerate:
        '藍牙耳機：高空跌落實測，鏡頭跟拍落地瞬間，完好無損，最後揭曉限時折扣，真人出鏡口播風格。',
      intentLabel: '改寫意圖',
      promptReplicate:
        '保留參考影片的自拍口播節奏：開頭吐槽痛點，中段手持展示，結尾說出真實感受。複刻的是思路，不是原片。',
      altProduct: '藍牙耳機商品圖',
      altRef: '參考爆款影片封面',
      altCan: '益生菌商品圖',
    },

    publish: {
      title: '發佈計畫',
      meta: '本週',
      posts: [
        {
          title: '第一口就上頭的洋芋片，追劇必備',
          platformLabel: 'TikTok',
          account: '@零食研究所',
          time: '今天 18:30',
          state: '已發佈',
        },
        {
          title: '挑食的狗子也會搶著吃',
          platformLabel: 'TikTok',
          account: '@毛孩子的飯盆',
          time: '今天 20:00',
          state: '已排程',
        },
        {
          title: '一件吊帶褲，一週不重樣',
          platformLabel: 'Instagram',
          account: '@穿搭日記',
          time: '明天 12:00',
          state: '就緒',
        },
        {
          title: '吃了一個月益生菌的真實感受',
          platformLabel: 'YouTube',
          account: '@好物种草社',
          time: '明天 19:00',
          state: '處理中',
        },
      ],
      feats: [
        {
          title: '多平台一次提交',
          desc: 'TikTok、Instagram、Facebook、YouTube，選好帳號一次提交，每個帳號單獨追蹤狀態。',
        },
        {
          title: '帳號統一管理',
          desc: '在桌面版連結各平台帳號，發佈時直接選擇，不用來回切換。',
        },
        {
          title: '狀態記錄與失敗重試',
          desc: '每個任務都有狀態記錄，失敗的任務可以重試。',
        },
      ],
    },

    review: {
      title: '帳號表現',
      synced: '已同步 · 剛剛',
      meta: '最近 7 天',
      kpis: [
        { label: '播放量', value: '12.8萬', delta: '+18%' },
        { label: '按讚', value: '8,640', delta: '+24%' },
        { label: '互動率', value: '5.2%', delta: '+0.8pt' },
      ],
      days: ['週一', '週二', '週三', '週四', '週五', '週六', '週日'],
      splitTitle: '播放來源 · 按開頭類型',
      splitLegs: ['前後對比開頭 63%', '口播開頭 37%'],
      note: '「前後對比」開頭的影片貢獻了 63% 的播放，晚上 7–9 點發佈表現最好。',
      cta: '開始下一輪：沿用這個鉤子再做 3 支',
    },
  },

  create: {
    tag: '創作 Agent',
    h2: '交給 Agent，或者自己搭流程',
    lead: '透過 OmniMux Agent 對話創作內容，或在畫布中連接圖片、影片和語音工具，搭建可重複運用的工作流程。',
    msgProductCard: '能量飲料 · 主圖',
    msgAsk: '為這款能量飲料做一套商品圖。',
    doneAnalyze: '已分析商品：外型、標籤與配色',
    donePlan: '已規劃 8 張套圖：主圖、賣點、細節、規格、場景',
    doneConsistent: '每張圖裡保持商品完全一致',
    confirmTitle: '生成 8 張圖片？',
    confirmNeed: '需要你確認',
    paramCount: '8 張',
    confirmGo: '確認並生成',
    confirmEdit: '修改',
    doneResult: '套圖已完成，8 張已放上畫布',
    tabsAria: '工作區',
    tabs: {
      canvas: '畫布',
      assets: '資產庫',
      inspiration: '靈感庫',
      products: '商品庫',
    },
    groupLabel: '商品套圖 · 8',
    setCaptions: [
      '主圖',
      '賣點',
      '細節',
      '規格',
      '場景',
      '場景',
      '氛圍',
      '海報',
    ],
    libAssets: [
      { label: '角色', sub: '小鹿 · 3 個視角' },
      { label: '場景', sub: '開放式廚房' },
      { label: '風格', sub: '霓虹賽博' },
      { label: '道具', sub: '吊帶褲' },
    ],
    libInspiration: [
      { label: '唇釉試色', sub: 'TikTok · 300萬播放' },
      { label: '車內補妝', sub: 'TikTok · 270萬播放' },
      { label: '精華實測', sub: 'Instagram · 1.8萬收藏' },
      { label: '追逐撞屏', sub: '已拆解' },
    ],
    libProducts: [
      { name: '能量飲料', sub: '0 糖 · 提神 · 冰爽口感', count: '8 張套圖' },
      {
        name: '藍牙耳機',
        sub: '降噪 · 續航 30 小時 · 耐摔',
        count: '2 支影片',
      },
      { name: '益生菌', sub: '每日一勺 · 獨立罐裝 · 便攜', count: '1 支影片' },
    ],
    capAgentTitle: '說出需求，Agent 完成整套創作',
    capAgentDesc:
      '從商品圖、影片到腳本，Agent 規劃每一步，並讓同一商品在整套內容裡保持一致。',
    capAgentCta: '開始創作',
    capCanvasTitle: '畫布：視覺化創作工作流程',
    capCanvasDesc:
      '把圖片、影片和配音節點連成工作流程，任何一步都能單獨調整，再次執行。',
    capCanvasCta: '開啟畫布',
    canvasAria: '畫布工作流程範例',
    nodeImage: '圖片',
    nodeText: '文字',
    nodeTextBody: '霓虹燈光，深色背景，罐身特寫，冰霧和水珠',
    nodeAIImage: 'AI 圖片',
    nodeAIVideo: 'AI 影片',
    nodeCut: '剪輯輸出',
    nodeDone: '已完成',
    nodeCutBodyA: '加字幕、配樂',
    nodeCutBodyB: '輸出 9:16 直式',
  },

  surfaces: {
    h2: '在你常用的地方直接用',
    lead: '同一個 Agent，桌面版、Codex、Claude Code 和瀏覽器裡都能用。',
    desktop: {
      title: 'OmniMux 桌面版',
      desc: '完整工作台：畫布、素材、商品和全部工具，macOS 與 Windows 都能用。',
      input: '幫我的 3 款精華各做一支影片…',
      cta: '下載桌面版',
    },
    cli: {
      title: 'Codex、Claude Code 等 Agent',
      desc: '安裝 OmniMux CLI，在常用的 Agent 中呼叫影片、圖片和分析工具。',
      cta: '安裝 CLI',
    },
    ext: {
      title: '瀏覽器擴充功能',
      desc: '在網頁上刷到爆款，一鍵收進靈感庫，回到桌面版接著拆解，不用來回切換。',
      capSave: '收進靈感庫',
      capScan: '拆解',
      toast: '已儲存到靈感庫',
      cta: '在桌面版安裝擴充功能',
    },
  },

  tools: {
    h2: 'AI 生成帶貨影片、圖片與配音素材',
    lead: '製作商品素材、多平台內容、再利用已有影片——都在同一個工作區完成。',
    centerLabel: '內容製作',
    groups: [
      {
        title: '商品上新',
        lead: '讓商品圖達到上架標準',
        items: [
          { icon: 'img', label: '白底商品圖' },
          { icon: 'layers', label: '多角度產品圖' },
          { icon: 'grid', label: '商品套圖' },
          { icon: 'box', label: '商品詳情圖' },
          { icon: 'spark', label: '場景圖' },
          { icon: 'wand', label: '參考圖改款' },
        ],
      },
      {
        title: '帶貨影片',
        lead: '從零生成，或照著爆款做',
        items: [
          { icon: 'link', label: '商品生影片' },
          { icon: 'video', label: '影片生成' },
          { icon: 'flame', label: '爆款複刻' },
          { icon: 'wand', label: '影片轉提示詞' },
          { icon: 'grid', label: '分鏡腳本' },
          { icon: 'img', label: '首幀生影片' },
        ],
      },
      {
        title: '圖文帶貨',
        lead: '一組圖也能賣貨',
        items: [
          { icon: 'layers', label: '圖文輪播' },
          { icon: 'scan', label: '參考圖複刻' },
          { icon: 'img', label: '圖片生成' },
          { icon: 'shirt', label: '服裝上身' },
        ],
      },
      {
        title: '口播配音',
        lead: '聲音、角色和出鏡人',
        items: [
          { icon: 'user', label: '數位人形象' },
          { icon: 'users', label: '多視角角色' },
          { icon: 'mic', label: '配音與音效' },
          { icon: 'music', label: 'AI 配樂' },
        ],
      },
      {
        title: '多平台適配',
        lead: '一套素材，適配多個平台',
        items: [
          { icon: 'crop', label: '畫幅適配' },
          { icon: 'hash', label: '標題與話題' },
        ],
      },
      {
        title: '素材二創',
        lead: '剪輯、拆分、字幕燒錄，處理已有影片素材。',
        items: [
          { icon: 'cut', label: '影片剪輯' },
          { icon: 'scan', label: '鏡頭拆分' },
          { icon: 'subs', label: '字幕燒錄' },
        ],
      },
    ],
    modelsTitle: '頂級 AI 模型，為賣貨而調校',
    modelsAria: '可用模型',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: '影片' },
      { mark: 'S', name: 'Seedance 2.0', kind: '影片' },
      { mark: 'G', name: 'GPT Image 2.5', kind: '圖片' },
      { mark: 'N', name: 'Nano Banana', kind: '圖片' },
      { mark: '♪', name: 'Suno', kind: '音樂' },
      { mark: 'G', name: 'Gemini', kind: '理解與腳本' },
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
        { icon: 'clock', label: '4–30 秒輸出' },
      ],
      [
        { icon: 'layers', label: '多模態參考' },
        { icon: 'user', label: '角色一致性' },
        { icon: 'img', label: '首幀控制' },
        { icon: 'music', label: '原生音訊' },
        { icon: 'spark', label: '商品一致性' },
      ],
      [
        { icon: 'cut', label: '本機剪輯' },
        { icon: 'scan', label: '鏡頭拆分' },
        { icon: 'subs', label: '字幕燒錄' },
        { icon: 'mic', label: '音訊提取' },
        { icon: 'layers', label: '圖文成片' },
      ],
    ],
  },

  workspace: {
    h2: '一個工作區，素材都在手邊',
    lead: '商品、素材、靈感和發佈記錄都在同一個桌面版，隨時取用，不用來回找。',
    list: [
      { name: '商品庫', desc: '賣點、受眾和商品圖' },
      { name: '資產庫', desc: '角色、場景、風格和提示詞' },
      { name: '靈感庫', desc: '收藏的爆款和拆解結果' },
      { name: '發佈中心', desc: '草稿、排程和發佈記錄' },
    ],
    note: '支援 macOS 與 Windows。',
    tableTitle: '商品庫',
    addProduct: '新增商品',
    cols: ['商品', '賣點', '本月內容'],
    rows: [
      { name: '能量飲料', category: '食品飲料', points: '3 個' },
      { name: '藍牙耳機', category: '數位產品', points: '4 個' },
      { name: '吊帶褲', category: '服飾', points: '3 個' },
      { name: '益生菌', category: '保健食品', points: '2 個' },
    ],
  },

  proof: {
    h2: '為跨境電商賣家打造',
    lead: '從找參考、拆解、製作到發佈，在一個桌面版裡完成。',
    facts: [
      { value: '17', label: '個內建技能' },
      { value: '7', label: '套成片範本' },
      { value: '4', label: '個發佈平台' },
    ],
    platformsAria: '支援發佈的平台',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: ['生成前確認參數與積分', '任務失敗自動退還積分'],
  },

  faq: {
    h2: '常見問題',
    lead: '電商賣家最關心的問題',
    items: [
      {
        q: 'OmniMux 是開源的嗎？',
        a: '是。桌面端、外掛與閘道原始碼按版本發佈在 github.com/omnimux-ai/omnimux-ai（AGPL-3.0）。託管服務的渠道憑證、定價帳本、維運與使用者資料不在開源範圍內。',
      },
      {
        q: 'OmniMux Agent 能做什麼？',
        a: 'OmniMux Agent 可以分析參考影片、撰寫腳本、生成影片和圖片，並協助把內容發佈到你已經連結的平台。用日常語言說明你的商品和目標，它會為每個步驟選擇合適的工具。',
      },
      {
        q: 'Agent 會不經我同意就生成或發佈嗎？',
        a: '不會。生成影片或圖片、發佈內容之前，Agent 都會先展示參數和積分消耗，等你確認；確認前可以修改參數。如果任務因系統問題失敗，已扣除的積分會退還。',
      },
      {
        q: '如何用 AI 製作帶貨影片？',
        a: '貼上商品連結或選擇爆款參考，生成原創或複刻影片，再發佈到你已連結的平台。無需拍攝或剪輯。',
      },
      {
        q: '生成的影片可以直接發佈嗎？',
        a: '可以。在桌面版連結一次帳號，就能直接發佈，無需手動上傳。發佈時可依平台要求加上 AI 生成內容聲明。',
      },
      {
        q: '用 AI 做的影片會不會導致帳號被封？',
        a: 'OmniMux 不承諾防封號。帳號是否安全，取決於你的內容和帳號行為。發佈時可為影片添加 AI 生成內容聲明，請遵守各平台規則。',
      },
      {
        q: '是否支援商品圖片轉影片？',
        a: '支援。上傳商品圖片或貼上商品連結，AI 會生成可直接發佈的展示影片，無需拍攝。',
      },
      {
        q: '怎麼複刻競品的爆款影片？',
        a: 'AI 學習對方的創作手法（節奏、構圖、調性），用你自己的商品和原創畫面生成一支全新影片——複刻的是思路，不是原片。',
      },
      {
        q: '生成的影片可以商用嗎？',
        a: '商用範圍以《服務條款》為準。上傳的人物照片、商品圖和參考影片，需要由你確認擁有使用授權，並遵守各平台規則。',
      },
    ],
  },

  finalCta: {
    h2: '下一支爆款，交給 OmniMux',
    lead: '拆解爆款、生成影片、發佈並復盤，一個 Agent 跑完整個閉環。',
    dlMac: '免費下載 macOS 版',
    dlWin: '下載 Windows 版',
    note: 'macOS 12+（Apple 晶片與 Intel 通用）· Windows 10 / 11 x64',
  },

  download: {
    title: '在 OmniMux 桌面版繼續',
    desc: '下載安裝後登入，即可開始創作。',
    close: '關閉',
    macTitle: '下載 macOS 版（Universal）',
    macSub: 'macOS 12+ · Apple 晶片與 Intel 通用',
    winTitle: '下載 Windows 版（x64 安裝程式）',
    winSub: 'Windows 10 / 11 x64',
    recommended: '推薦',
    notePre: '若 Windows 提示「已保護你的電腦」：點擊',
    noteMore: '其他資訊',
    noteRun: '仍要執行',
    notePost: '。請僅從 dl.omnimux.ai 下載。',
  },

  footer: {
    brand: '桌面版',
    tagline: '發現、複刻和裂變電商爆款影片。',
    columns: [
      {
        title: '探索',
        links: [
          { label: '價格', href: '/pricing' },
          { label: '靈感社群', href: '/inspiration' },
          { label: '下載桌面版', href: '#download' },
        ],
      },
      {
        title: '工具',
        links: [
          {
            label: 'TikTok 影片下載',
            href: '/zh/tools/tiktok-video-downloader',
          },
          {
            label: 'TikTok 圖文下載',
            href: '/zh/tools/tiktok-photo-downloader',
          },
          { label: '商品生影片', href: '#tools' },
          { label: '爆款複刻', href: '#tools' },
          { label: '影片轉提示詞', href: '#tools' },
          { label: '商品套圖', href: '#tools' },
          { label: '商品詳情圖', href: '#tools' },
          { label: '圖文輪播', href: '#tools' },
          { label: '數位人形象', href: '#tools' },
          { label: '字幕燒錄', href: '#tools' },
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
        title: '資源',
        links: [
          { label: '文件', href: 'https://docs.omnimux.ai', external: true },
          {
            label: '服務狀態',
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
        title: '關於',
        links: [
          {
            label: '聯絡我們',
            href: 'mailto:hello@omnimux.ai',
            external: true,
          },
          { label: '隱私政策', href: '/privacy' },
          { label: '服務條款', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: '開源倉庫',
  },

  toolPage: {
    crumbAria: '麵包屑',
    crumbTools: '工具',
    kindAria: '內容類型',
    kindVideo: '影片',
    kindPhoto: '圖文',
    inputLabel: 'TikTok 連結',
    placeholderVideo: '貼上 TikTok 影片連結或分享文字',
    placeholderPhoto: '貼上 TikTok 圖文連結或分享文字',
    paste: '貼上',
    parse: '解析',
    parsing: '解析中',
    verifying: '驗證中',
    quotaAnon: '今日免費 {remaining}/{limit}',
    quotaUser: '今日 {remaining}/{limit}',
    turnstileAria: 'Cloudflare 人機驗證',
    unavailable: '暫不可用',
    downloadMp4: '下載 MP4',
    recreateMine: '用我的角色復刻',
    breakdown: '拆解爆款結構',
    photoCount: '{count} 張',
    imageAlt: '第 {n} 張',
    downloadImage: '下載第 {n} 張',
    downloadZip: '全部下載 ZIP',
    zipLocked: '登入後可用',
    recreatePhotos: '復刻這組圖文',
    ctaTitle: '保存參考，再用你的角色復刻',
    ctaBody: '同樣的節奏和鏡頭，換成你自己的 AI 角色和商品。',
    ctaStart: '開始復刻',
    ctaRef: '參考',
    ctaMine: '復刻',
    relatedAria: '相關工具',
    errors: {
      invalidLink: '請貼上 TikTok 連結',
      wrongKindPhoto: '這是圖文連結',
      toPhoto: '去圖文下載',
      wrongKindVideo: '這是影片連結',
      toVideo: '去影片下載',
      unavailable: '該內容不公開或已刪除，無法解析',
      serviceUnavailable: '解析服務暫未開放',
      upstreamFailed: 'TikTok 暫時沒有回應，請稍後再試',
      quotaAnon: '今日免費次數已用完，登入後繼續',
      quotaUser: '今日次數已用完，明天再來',
      busy: '今日免費額度已滿，登入後繼續使用',
      rateLimited: '操作太快，請稍後再試',
      turnstileFailed: '人機驗證未通過，請重試',
      turnstileUnavailable: '驗證服務暫時不可用，請稍後再試',
      generic: '解析失敗，請稍後再試',
    },
  },
}
