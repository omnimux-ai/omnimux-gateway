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
import { docsQuickstartUrl } from '../../../lib/landing-urls'
import type { LandingCopy } from '../types'

export const id: LandingCopy = {
  meta: {
    title:
      'OmniMux Desktop — Temukan, Buat Ulang & Gandakan Video Jualan Viral',
  },
  skip: 'Lewati ke konten',

  nav: {
    capabilities: 'Fitur',
    workflow: 'Alur kerja',
    inspiration: 'Inspirasi',
    downloadDesktop: 'Unduh aplikasi desktop',
    menuClose: 'Tutup menu',
    brand: 'Desktop',
    home: 'Beranda',
    models: 'Model',
    tools: 'Alat',
    ext: 'Ekstensi',
    pricing: 'Harga',
    resources: 'Sumber daya',
    download: 'Gunakan gratis',
    login: 'Masuk',
    menuOpen: 'Buka menu',
    langTitle: 'Bahasa halaman',
    brandAria: 'Beranda OmniMux Desktop',
    navAria: 'Navigasi utama',
    toolsMenu: {
      aria: 'Menu alat',
      group: 'Kreatif',
      video: 'Unduh video TikTok',
      photo: 'Unduh foto TikTok',
      agentGroup: 'Agen',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: 'Menu sumber daya',
      inspiration: 'Komunitas inspirasi',
      docs: 'Dokumentasi developer',
    },
    account: {
      aria: 'Menu akun',
      dashboard: 'Konsol',
      profile: 'Profil',
      security: 'Keamanan & akses',
      wallet: 'Dompet',
      systemSettings: 'Pengaturan sistem',
      signOut: 'Keluar',
    },
  },

  signOut: {
    title: 'Keluar',
    body: 'Anda perlu masuk kembali untuk mengakses akun Anda.',
    cancel: 'Batal',
    confirm: 'Keluar',
    busy: 'Keluar…',
    failed: 'Gagal keluar',
    done: 'Sudah keluar',
    close: 'Tutup',
  },

  hero: {
    eyebrow: 'Untuk penjual lintas negara dan penjual yang mengandalkan konten',
    secondaryCta: 'Lihat cara membuat satu video',
    mobileCopyLabel: 'Salin tautan unduhan',
    mobileCopiedLabel: 'Tautan disalin · Buka di komputer',
    mobileHint:
      'Aplikasi desktop dapat dipasang di macOS atau Windows. Salin tautan ini dan buka di browser komputer Anda.',
    microTrust:
      'macOS 12+ / Windows 10 / 11 · Unduhan gratis · Bayar sesuai pemakaian tanpa langganan',
    trustAria: 'Informasi produk dan platform',
    trustStrip: ['Kode sumber terbuka · AGPL-3.0', 'macOS + Windows'],
    mockAria: 'Pratinjau antarmuka OmniMux Desktop',
    mockBadge: 'Pratinjau antarmuka',
    mockTitle: 'OmniMux Desktop · Chat baru',
    mockSide: [
      'Chat baru',
      'Aplikasi',
      'Produk',
      'Aset',
      'Inspirasi',
      'Publikasi',
      'Kanvas',
      'Editor',
    ],
    mockSidebarGroup: 'Ruang kerja',
    mockUserAvatar: 'L',
    mockPrompt:
      'Pakai referensi viral ini untuk membuat 3 video berdurasi 15 detik untuk @Serum. Awali dengan hasil saat dipakai di kulit.',
    mockLinkCard:
      'tiktok.com/@pet.diary/video/73421 · Sehari bersama kucing, buka paket makanan',
    mockSteps: [
      'Video referensi dianalisis: pembuka, keunggulan produk, dan ajakan membeli',
      'Keunggulan produk dirangkum: cepat meresap, tidak menggumpal, dan cocok untuk kulit kering',
      'Naskah dan storyboard untuk 3 video selesai',
    ],
    mockTags: [
      'Pembuka · Sebelum dan sesudah dalam 2 detik pertama',
      'Keunggulan · Sorotan saat produk dioleskan',
      'Ajakan membeli · Tautan produk di akhir',
    ],
    mockTileBadge: 'Selesai dibuat',
    mockNote:
      'Pratinjau antarmuka · Contoh statis · Contoh aset dari ruang kerja',
    titleA: 'Temukan, buat ulang & gandakan',
    titleB: 'video jualan viral',
    subPre:
      'Satu sistem content commerce yang membawa keunggulan produk Anda ke setiap video. Gunakan ',
    subHighlight: 'Agent yang sudah Anda kenal',
    subPost:
      ' untuk membuat video iklan, jualan, dan ulasan produk dalam satu aplikasi, dari analisis referensi hingga penjadwalan publikasi.',
    tabsAria: 'Cara membuat',
    tabs: { agent: 'Agent', video: 'Video', image: 'Gambar' },
    inputLabel: 'Deskripsikan konten yang ingin dibuat',
    attachAria: 'Tambahkan video atau gambar referensi',
    sendAria: 'Mulai di desktop',
    dlMac: 'Gunakan gratis',
    dlWin: 'Gunakan gratis',
    dlMenuAria: 'Pilih versi unduhan',
    ghStar: 'Beri bintang di GitHub',
    dlMacArm: 'macOS (Apple Silicon)',
    dlMacIntel: 'macOS (Intel)',
    dlWinX64: 'Windows (64-bit)',
    dlUnavailable: 'Unduhan tidak tersedia saat versi stabil sedang dirilis',
    skillsLabel: 'Skill',
    skillsAria: 'Skill',
    skills: [
      {
        icon: 'scan',
        label: 'Bedah video/postingan',
        fill: 'Bedah kenapa video ini viral — hook pembuka, selling point, dan alur pembelian https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: 'Buat ulang video',
        fill: 'Buat ulang video viral ini untuk @Serum, pertahankan ritme dan struktur shot, 15 detik',
      },
      {
        icon: 'clap',
        label: 'Buat video jualan',
        fill: 'Buat video jualan 15 detik untuk @Serum, buka dengan efek di kulit',
      },
      {
        icon: 'img',
        label: 'Buat ulang postingan jualan',
        fill: 'Jadikan set postingan Instagram ini referensi — buat 6 gambar jualan untuk @Serum',
      },
      {
        icon: 'wand',
        label: 'Video ke prompt',
        fill: 'Ubah video ini jadi prompt yang bisa dipakai ulang, rinci per shot',
      },
    ],
    modes: {
      agent: {
        prompts: [
          'Bedah kenapa video ini viral https://www.tiktok.com/@pet.diary/video/73421',
          'Pakai video viral ini untuk buat 3 video @Serum, siap dipublikasikan, 15 detik',
          'Analisis strategi @SkinDiary 30 hari terakhir',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          'Buat video jualan 15 detik untuk @Serum — buka dengan efek di kulit',
          'Uji jatuh earbuds Bluetooth dari ketinggian, tutup dengan flash sale',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          'Buat set 8 gambar produk untuk @Serum: hero, manfaat, detail, spesifikasi, suasana',
          'Ganti foto produk ini jadi suasana meja rias marmer',
        ],
      },
    },
    reelAria: 'Contoh template video jadi bawaan',
    reel: [
      { title: 'Efek partikel 3D', tag: 'Makanan hewan' },
      { title: 'Ganti outfit runway', tag: 'Overall' },
      { title: 'Uji jatuh ketahanan', tag: 'Earbuds' },
      { title: 'Pitch selfie kreator', tag: 'Probiotik' },
      { title: 'Kejar produk raksasa', tag: 'Tas belanja' },
      { title: 'Demo web mobile', tag: 'Aplikasi travel' },
      { title: 'Iklan promo 15 detik', tag: 'Snack' },
    ],
  },

  agentDemo: {
    h2: 'Buat ulang video jualan viral, cukup dalam satu chat',
    lead: 'Beri tahu apa yang kamu jual. Ia membedah video referensi, menulis naskah, membuat konten, dan menjadwalkan posting.',
    windowAria: 'Demo OmniMux Desktop Agent',
    windowTitle: 'Buat ulang video viral ini untuk 3 produk laris saya',
    windowStatus: '3 video berhasil dibuat',
    replay: 'Putar ulang demo',
    replayAria: 'Putar ulang demo',
    side: [
      'Chat baru',
      'Aplikasi',
      'Canvas',
      'Pustaka aset',
      'Inspirasi',
      'Produk',
      'Publikasi',
      'Data',
    ],
    me: 'L',
    msgPre: 'Pakai video viral ini untuk membuat satu video',
    msgProducts: ['@MakananHewan', '@Overall', '@Earbuds'],
    msgPost: '— siap dipublikasikan, masing-masing 15 detik.',
    linkCard:
      'tiktok.com/@pet.diary/video/73421 · kucing lucu + unboxing cat food',
    doneBreakdown: 'Sudah dibedah kenapa video referensi ini laku',
    kv: [
      { k: 'Hook', v: 'Before/after di 2 detik pertama' },
      { k: 'Bukti', v: 'Close-up saat dipakai' },
      { k: 'Penutup', v: 'Arahkan klik ke keranjang kuning' },
    ],
    doneProducts: 'Sudah menganalisis 3 produk dan selling point Anda',
    doneScripts: 'Sudah menulis naskah untuk tiap produk',
    confirmTitle: 'Buat 3 video?',
    confirmNeed: 'Perlu konfirmasi Anda',
    confirmCost: 'Estimasi kredit terpakai, dipotong setelah konfirmasi',
    confirmGo: 'Konfirmasi & buat',
    confirmEdit: 'Ubah',
    inputPh: 'Kirim pesan ke OmniMux — @ untuk mereferensikan produk atau aset',
    resultsTitle: 'Hasil akhir',
    resultsMeta: '3 video · 9:16',
    generating: 'Membuat…',
    vidNames: ['Makanan hewan', 'Overall', 'Earbuds'],
    vidHooks: [
      'Hook · kontras konsep',
      'Hook · ganti outfit',
      'Hook · close-up',
    ],
    schedTitle: 'Posting terjadwal',
    schedOk: 'Terjadwal',
    schedPlatform: 'TikTok',
    schedTimes: ['Selasa 19:00', 'Rabu 12:30', 'Kamis 19:00'],
    schedAll: 'Jadwalkan semua',
  },

  flow: {
    kicker: 'Alur kerja',
    lead: 'Temukan referensi, analisis, buat, publikasikan, dan evaluasi dalam satu ruang kerja desktop. Lihat masukan, hasil, dan langkah berikutnya di setiap tahap.',
    h2: 'Dari ide produk ke video jualan dalam 5 langkah',
    lineAria: 'Temukan · Analisis · Buat · Publikasi · Evaluasi',
    steps: [
      {
        kind: 'Temukan',
        title: 'Cari referensi di kategori produk Anda',
        desc: 'Kumpulkan video referensi yang Anda temukan dan pelajari struktur video populer di kategori yang sama.',
        chips: ['Video jualan & postingan viral'],
        primary: 'Bedah satu video referensi',
        secondary: 'Kumpulkan inspirasi lewat ekstensi browser',
      },
      {
        kind: 'Analisis',
        title: 'Susun kerangka naskah yang bisa dipakai ulang',
        desc: 'Bedah pembuka, keunggulan produk, dan ajakan membeli. Susun hasilnya menjadi kerangka naskah yang bisa dipakai ulang.',
        chips: ['Bedah naskah', 'Akun kompetitor', 'Tren kategori'],
        primary: 'Bedah satu video viral',
      },
      {
        kind: 'Buat',
        title: 'Buat video dengan produk atau karakter',
        desc: 'Buat atau buat ulang video jualan dengan gambar produk atau aset karakter, lalu selesaikan di kanvas dan editor.',
        primary: 'Buat video',
        secondary: 'Buat ulang video viral',
      },
      {
        kind: 'Publikasi',
        title: 'Hubungkan akun dan jadwalkan posting',
        desc: 'Hubungkan akun untuk menjadwalkan posting ke TikTok, Instagram, Facebook, dan YouTube.',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: 'Publikasikan video',
        secondary: 'Postingan video · Postingan gambar',
      },
      {
        kind: 'Evaluasi',
        title: 'Gunakan data untuk video berikutnya',
        desc: 'Bawa data publikasi kembali ke ruang kerja untuk memandu analisis dan memperbaiki video berikutnya.',
        primary: 'Analisis akun',
      },
    ],

    discover: {
      title: 'Contoh referensi · alur ilustrasi',
      meta: '7 hari terakhir',
      savesPrefix: 'disimpan',
      items: [
        {
          who: 'taotao.ootd',
          views: '46 rb',
          length: 'Postingan · 6 gambar',
          title: 'Satu dress polkadot, tiga gaya — kantor sampai kencan #ootd',
          likes: '46 rb',
          saves: '31 rb',
        },
        {
          who: 'commute10',
          views: '2,7 jt',
          length: '00:28',
          title: 'Touch-up makeup di mobil cukup satu tangan #makeupcommute',
          likes: '89 rb',
          saves: '24 rb',
        },
        {
          who: 'dryskin.lab',
          views: '1,6 jt',
          length: '00:34',
          title: 'Serum pipet untuk kulit kering musiman #kulitsensitif',
          likes: '52 rb',
          saves: '18 rb',
        },
      ],
    },

    breakdown: {
      tabsAria: 'Objek yang dibedah',
      tabs: {
        video: 'Video',
        account: 'Akun kompetitor · Contoh',
        category: 'Tren kategori · Contoh',
      },
      statsViews: '15 jt+ views',
      statsLikes: '320 rb+ like',
      why: 'Kenapa video ini laku',
      rows: [
        {
          label: 'Hook pembuka',
          time: '0–2s',
          desc: 'Hasil duluan — tunjukkan efek sebelum produk',
        },
        {
          label: 'Selling point',
          time: '3–9s',
          desc: 'Close-up uji langsung, voice-over tiga poin',
        },
        {
          label: 'Alur pembelian',
          time: '10–14s',
          desc: 'Promo terbatas, arahkan klik keranjang kuning',
        },
      ],
      reuse: 'Struktur yang bisa dipakai ulang',
      chain: ['Hasil duluan', 'Bukti uji langsung', 'Promo + keranjang'],
      accountName: '@SkinDiary',
      accountMeta: 'Beauty & personal care',
      thumbs: ['2,1 jt', '1,4 jt', '860 rb', '510 rb'],
      metrics: [
        { label: 'Video diposting · 30 hari terakhir', value: '42' },
        { label: 'Rata-rata views', value: '186 rb' },
        { label: 'Video jualan', value: '71%' },
      ],
      insights: [
        {
          k: 'Strategi',
          v: '7 dari 10 video teratas dibuka dengan hook "hasil duluan" lalu cut ke close-up uji langsung — itu pola utama akun ini.',
        },
        {
          k: 'Waktu',
          v: 'Video yang diposting jam 19–21 views-nya sekitar 40% lebih tinggi dari jam lain.',
        },
        {
          k: 'Peluang',
          v: 'Belum ada yang menggabungkan before/after dengan anchor harga di kategori ini — layak dicoba duluan.',
        },
      ],
      accountCta: 'Pakai strategi ini untuk buat 3 video @Serum',
      catTitle: 'Beauty & personal care',
      catMeta: '7 hari terakhir · top 200 · Data contoh',
      bars: [
        { label: 'Before/after', value: '38%' },
        { label: 'Unboxing & coba', value: '24%' },
        { label: 'Close-up uji langsung', value: '19%' },
        { label: 'Voice-over pitch', value: '12%' },
        { label: 'Plot twist', value: '7%' },
      ],
      catInsights: [
        {
          k: 'Hook',
          v: 'Pembuka before/after ada di 38% video top dan punya rata-rata views tertinggi.',
        },
        {
          k: 'Durasi',
          v: 'Kebanyakan hits 15–25 detik; di atas 40 detik turun drastis.',
        },
        {
          k: 'Peluang',
          v: 'Kombo close-up uji langsung + promo terbatas masih jarang — cocok untuk produk baru.',
        },
      ],
      catCta: 'Buatkan rencana benchmarking dari ini',
    },

    make: {
      tabsAria: 'Cara membuat',
      tabs: { generate: 'Buat langsung', replicate: 'Buat ulang viral' },
      capProduct: 'Produk',
      capResult: 'Hasil akhir',
      capRef: 'Referensi viral',
      capMine: 'Produk Anda',
      badgeGenerated: 'Sudah dibuat',
      badgeRef: '▶ TikTok · 2,1 jt',
      arrowReplicateA: 'Tiru strukturnya',
      arrowReplicateB: 'ganti produkmu',
      promptLabel: 'Prompt',
      promptGenerate:
        'Earbuds Bluetooth: uji jatuh dari ketinggian, kamera mengikuti momen mendarat, tetap utuh, tutup dengan flash sale — gaya voice-over orang asli.',
      intentLabel: 'Maksud penulisan ulang',
      promptReplicate:
        'Pertahankan ritme voice-over selfie video referensi: buka dengan keluhan, tengah demo genggam, tutup dengan kesan jujur. Yang ditiru adalah idenya, bukan footage aslinya.',
      altProduct: 'Foto produk earbuds Bluetooth',
      altRef: 'Cover video referensi viral',
      altCan: 'Foto produk probiotik',
    },

    publish: {
      title: 'Rencana publikasi',
      meta: 'Minggu ini',
      posts: [
        {
          title: 'Keripik yang bikin nagih — teman maraton nonton',
          platformLabel: 'TikTok',
          account: '@snacklab',
          time: 'Hari ini 18:30',
          state: 'Terpublikasi',
        },
        {
          title: 'Anjing picky eater pun berebut',
          platformLabel: 'TikTok',
          account: '@furbowl',
          time: 'Hari ini 20:00',
          state: 'Terjadwal',
        },
        {
          title: 'Satu overall, tujuh outfit',
          platformLabel: 'Instagram',
          account: '@ootd_daily',
          time: 'Besok 12:00',
          state: 'Siap',
        },
        {
          title: 'Sebulan pakai probiotik — review jujur',
          platformLabel: 'YouTube',
          account: '@goodfinds',
          time: 'Besok 19:00',
          state: 'Diproses',
        },
      ],
      feats: [
        {
          title: 'Kirim ke semua platform sekaligus',
          desc: 'TikTok, Instagram, Facebook, dan YouTube — pilih akun, kirim sekali, dan lacak status tiap akun.',
        },
        {
          title: 'Semua akun dalam satu tempat',
          desc: 'Hubungkan semua akun platform di aplikasi desktop, lalu tinggal pilih saat publikasi — tanpa bolak-balik.',
        },
        {
          title: 'Penjadwalan & retry otomatis',
          desc: 'Setiap tugas tercatat statusnya dan yang gagal bisa dicoba ulang.',
        },
      ],
    },

    review: {
      title: 'Performa akun',
      synced: 'Tersinkron · baru saja',
      meta: '7 hari terakhir',
      kpis: [
        { label: 'Views', value: '128 rb', delta: '+18%' },
        { label: 'Like', value: '8.640', delta: '+24%' },
        { label: 'Engagement', value: '5,2%', delta: '+0,8pt' },
      ],
      days: ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'],
      splitTitle: 'Sumber views · per tipe hook',
      splitLegs: ['Hook before/after 63%', 'Hook voice-over 37%'],
      note: 'Pembuka before/after menyumbang 63% views; posting jam 19–21 perform-nya paling baik.',
      cta: 'Ronde berikutnya: buat 3 lagi dengan hook ini',
    },
  },

  create: {
    tag: 'Creator Agent',
    h2: 'Serahkan ke Agent, atau rangkai alurnya sendiri',
    lead: 'Buat konten lewat percakapan dengan OmniMux Agent, atau hubungkan alat gambar, video, dan suara di Canvas untuk membangun alur kerja yang dapat digunakan kembali.',
    msgProductCard: 'Minuman energi · gambar utama',
    msgAsk: 'Buatkan set gambar produk untuk minuman energi ini.',
    doneAnalyze: 'Produk dianalisis: bentuk, label & palet warna',
    donePlan:
      'Merencanakan set 8 gambar: utama, manfaat, detail, spesifikasi, suasana',
    doneConsistent: 'Menjaga produk identik di setiap gambar',
    confirmTitle: 'Buat 8 gambar?',
    confirmNeed: 'Perlu konfirmasi Anda',
    paramCount: '8 gambar',
    confirmGo: 'Konfirmasi & buat',
    confirmEdit: 'Ubah',
    doneResult: 'Set selesai — 8 gambar masuk ke Canvas',
    tabsAria: 'Workspace',
    tabs: {
      canvas: 'Canvas',
      assets: 'Pustaka aset',
      inspiration: 'Inspirasi',
      products: 'Produk',
    },
    groupLabel: 'Set produk · 8',
    setCaptions: [
      'Utama',
      'Manfaat',
      'Detail',
      'Spesifikasi',
      'Suasana',
      'Suasana',
      'Mood',
      'Poster',
    ],
    libAssets: [
      { label: 'Karakter', sub: 'Deer · 3 sudut' },
      { label: 'Suasana', sub: 'Dapur terbuka' },
      { label: 'Gaya', sub: 'Neon cyber' },
      { label: 'Properti', sub: 'Overall' },
    ],
    libInspiration: [
      { label: 'Swatch lip tint', sub: 'TikTok · 3 jt views' },
      { label: 'Touch-up di mobil', sub: 'TikTok · 2,7 jt views' },
      { label: 'Uji serum langsung', sub: 'Instagram · 18 rb disimpan' },
      { label: 'Kejar layar', sub: 'Sudah dibedah' },
    ],
    libProducts: [
      {
        name: 'Minuman energi',
        sub: '0 gula · menyegarkan · sensasi dingin',
        count: 'set 8 gambar',
      },
      {
        name: 'Earbuds',
        sub: 'ANC · baterai 30 jam · tahan jatuh',
        count: '2 video',
      },
      {
        name: 'Probiotik',
        sub: 'Sehari sesendok · sachet · praktis',
        count: '1 video',
      },
    ],
    capAgentTitle: 'Sebutkan kebutuhanmu — Agent menyelesaikan semuanya',
    capAgentDesc:
      'Dari gambar produk, video, sampai naskah, Agent merencanakan tiap langkah dan menjaga produk yang sama konsisten di semua konten.',
    capAgentCta: 'Mulai membuat',
    capCanvasTitle: 'Canvas: alur kerja kreatif yang visual',
    capCanvasDesc:
      'Hubungkan node gambar, video, dan voice-over jadi alur kerja — tiap langkah bisa disesuaikan dan dijalankan ulang.',
    capCanvasCta: 'Buka Canvas',
    canvasAria: 'Contoh alur kerja Canvas',
    nodeImage: 'Gambar',
    nodeText: 'Teks',
    nodeTextBody:
      'Lampu neon, latar gelap, close-up kaleng, kabut es dan tetesan air',
    nodeAIImage: 'Gambar AI',
    nodeAIVideo: 'Video AI',
    nodeCut: 'Edit & ekspor',
    nodeDone: 'Selesai',
    nodeCutBodyA: 'Tambah subtitle & musik',
    nodeCutBodyB: 'Ekspor vertikal 9:16',
  },

  surfaces: {
    h2: 'Pakai di tempat yang sudah biasa kamu pakai',
    lead: 'Agent yang sama di aplikasi desktop, Codex, Claude Code, dan browser.',
    desktop: {
      title: 'OmniMux Desktop',
      desc: 'Workspace lengkap: canvas, aset, produk, dan semua alat — di macOS dan Windows.',
      input: 'Buatkan satu video untuk tiap 3 serum saya…',
      cta: 'Unduh Desktop',
    },
    cli: {
      title: 'Codex, Claude Code & agent lain',
      desc: 'Instal OmniMux CLI lalu panggil alat video, gambar, dan analisis dari agent yang sudah kamu pakai.',
      cta: 'Instal CLI',
    },
    ext: {
      title: 'Ekstensi browser',
      desc: 'Nemu video viral di web — simpan ke Inspirasi dengan satu klik, lanjut bedah di desktop.',
      capSave: 'Simpan ke Inspirasi',
      capScan: 'Bedah',
      toast: 'Tersimpan ke Inspirasi',
      cta: 'Instal ekstensi di aplikasi',
    },
  },

  tools: {
    h2: 'Buat video jualan, gambar, dan voice-over dengan AI',
    lead: 'Buat aset produk, konten multi-platform, dan pakai ulang video lama — semuanya dalam satu workspace.',
    centerLabel: 'Pembuatan konten',
    groups: [
      {
        title: 'Listing produk',
        lead: 'Bikin gambar produk layak tayang',
        items: [
          { icon: 'img', label: 'Foto latar putih' },
          { icon: 'layers', label: 'Foto produk multi-angle' },
          { icon: 'grid', label: 'Set gambar produk' },
          { icon: 'box', label: 'Gambar halaman detail' },
          { icon: 'spark', label: 'Gambar suasana' },
          { icon: 'wand', label: 'Restyle dari referensi' },
        ],
      },
      {
        title: 'Video jualan',
        lead: 'Buat dari nol, atau ikuti video viral',
        items: [
          { icon: 'link', label: 'Produk ke video' },
          { icon: 'video', label: 'Pembuatan video' },
          { icon: 'flame', label: 'Buat ulang viral' },
          { icon: 'wand', label: 'Video ke prompt' },
          { icon: 'grid', label: 'Naskah storyboard' },
          { icon: 'img', label: 'Video dari frame awal' },
        ],
      },
      {
        title: 'Postingan gambar jualan',
        lead: 'Satu set gambar juga bisa jualan',
        items: [
          { icon: 'layers', label: 'Carousel gambar' },
          { icon: 'scan', label: 'Remake gambar referensi' },
          { icon: 'img', label: 'Pembuatan gambar' },
          { icon: 'shirt', label: 'Virtual try-on' },
        ],
      },
      {
        title: 'Suara & presenter',
        lead: 'Suara, karakter, dan talent di kamera',
        items: [
          { icon: 'user', label: 'Digital human' },
          { icon: 'users', label: 'Karakter multi-angle' },
          { icon: 'mic', label: 'Voice-over & SFX' },
          { icon: 'music', label: 'Musik AI' },
        ],
      },
      {
        title: 'Fit multi-platform',
        lead: 'Satu set aset, diadaptasi ke tiap platform',
        items: [
          { icon: 'crop', label: 'Fit rasio aspek' },
          { icon: 'hash', label: 'Judul & hashtag' },
        ],
      },
      {
        title: 'Remix footage',
        lead: 'Edit, potong, dan burn subtitle ke footage yang sudah ada.',
        items: [
          { icon: 'cut', label: 'Edit video' },
          { icon: 'scan', label: 'Pecah scene' },
          { icon: 'subs', label: 'Burn-in subtitle' },
        ],
      },
    ],
    modelsTitle: 'Model AI top, disetel untuk jualan',
    modelsAria: 'Model yang tersedia',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: 'Video' },
      { mark: 'S', name: 'Seedance 2.0', kind: 'Video' },
      { mark: 'G', name: 'GPT Image 2.5', kind: 'Gambar' },
      { mark: 'N', name: 'Nano Banana', kind: 'Gambar' },
      { mark: '♪', name: 'Suno', kind: 'Musik' },
      { mark: 'G', name: 'Gemini', kind: 'Pemahaman & naskah' },
    ],
    chipsAria: 'Kemampuan model',
    capabilityRows: [
      [
        { icon: 'monitor', label: '480p / 720p' },
        { icon: 'crop', label: '9:16' },
        { icon: 'crop', label: '16:9' },
        { icon: 'crop', label: '1:1' },
        { icon: 'crop', label: '3:4' },
        { icon: 'crop', label: '4:3' },
        { icon: 'clock', label: 'Output 4–30 detik' },
      ],
      [
        { icon: 'layers', label: 'Referensi multimodal' },
        { icon: 'user', label: 'Konsistensi karakter' },
        { icon: 'img', label: 'Kontrol frame awal' },
        { icon: 'music', label: 'Audio native' },
        { icon: 'spark', label: 'Konsistensi produk' },
      ],
      [
        { icon: 'cut', label: 'Edit lokal' },
        { icon: 'scan', label: 'Pecah scene' },
        { icon: 'subs', label: 'Burn-in subtitle' },
        { icon: 'mic', label: 'Ekstrak audio' },
        { icon: 'layers', label: 'Postingan gambar ke video' },
      ],
    ],
  },

  workspace: {
    h2: 'Satu workspace, semua aset dalam jangkauan',
    lead: 'Produk, aset, inspirasi, dan riwayat publikasi ada di satu aplikasi desktop — selalu siap, tanpa perlu mencari-cari.',
    list: [
      {
        name: 'Pustaka produk',
        desc: 'Selling point, audiens & gambar produk',
      },
      { name: 'Pustaka aset', desc: 'Karakter, suasana, gaya & prompt' },
      { name: 'Inspirasi', desc: 'Video viral tersimpan & hasil bedah' },
      { name: 'Pusat publikasi', desc: 'Draf, jadwal & riwayat' },
    ],
    note: 'Tersedia untuk macOS dan Windows.',
    tableTitle: 'Pustaka produk',
    addProduct: 'Tambah produk',
    cols: ['Produk', 'Selling point', 'Konten bulan ini'],
    rows: [
      { name: 'Minuman energi', category: 'Makanan & minuman', points: '3' },
      { name: 'Earbuds', category: 'Elektronik', points: '4' },
      { name: 'Overall', category: 'Fashion', points: '3' },
      { name: 'Probiotik', category: 'Suplemen', points: '2' },
    ],
  },

  capabilities: {
    kicker: 'Fitur',
    h2: 'Tonjolkan keunggulan produk di setiap video',
    lead: 'Kelola aset, karakter, dan alat dalam satu alur, dari pustaka produk hingga pusat publikasi.',
    cards: [
      {
        title: 'Pustaka produk',
        body: 'Simpan foto produk, keunggulan, target audiens, dan aset di satu tempat. Agent dapat langsung menggunakannya saat membuat video.',
      },
      {
        title: 'Analisis referensi dan pembuatan ulang',
        body: 'Bedah pembuka, keunggulan produk, dan ajakan membeli. Buat ulang strukturnya dengan produk atau karakter Anda.',
      },
      {
        title: 'Karakter yang konsisten dari berbagai sudut',
        body: 'Pakai ulang karakter AI dan referensi dari beberapa sudut di berbagai video. Konsistensi visual bergantung pada masukan dan pengaturan pembuatan.',
      },
    ],
  },

  proof: {
    kicker: 'Untuk penjual lintas negara',
    source:
      'Kode sumber terbuka · AGPL-3.0 · Desktop beserta plugin dan gateway diterbitkan di repositori GitHub terpisah',
    h2: 'Cari referensi, analisis, buat, dan publikasikan dari satu aplikasi desktop',
    lead: 'Siapkan konten dengan skill bawaan dan templat video, lalu jadwalkan di pusat publikasi.',
    facts: [
      { value: '17', label: 'skill bawaan' },
      { value: '7', label: 'templat video jadi' },
      { value: '4', label: 'platform publikasi' },
    ],
    platformsAria: 'Platform publikasi yang didukung',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: [
      'Konfirmasi parameter dan kredit sebelum membuat konten',
      'Pengembalian kredit mengikuti catatan tugas yang gagal',
    ],
  },

  pricingTeaser: {
    kicker: 'Harga',
    h2: 'Bayar sesuai pemakaian tanpa langganan',
    lead: 'Isi kredit untuk membuat konten. Mulai dengan isi ulang kecil untuk video pertama, lalu tambah sesuai kebutuhan. Lihat halaman harga dan dompet untuk nominal, jumlah kredit, dan diskon yang berlaku.',
    ctaPrimary: 'Lihat semua harga',
    ctaSecondary: 'Unduh sebelum isi ulang',
    cardSubtitle: 'Bayar per pembuatan · Kredit berlaku untuk semua model',
    cardPriceTitle: '{credits} kredit',
    cardPriceSub: 'Pilihan isi ulang mulai dari {amount}',
    tierCredits: '{credits} kredit',
    tierSave: 'Hemat {percent}%',
    cardCta: 'Lihat semua pilihan isi ulang',
    cardFootnote:
      '1 kredit = {price} · Diskon berlaku untuk nominal isi ulang yang tersedia',
  },

  faq: {
    kicker: 'Pertanyaan yang sering diajukan',
    h2: 'Yang paling sering ditanyakan penjual e-commerce',
    lead: 'Cakupan kode terbuka, platform publikasi, dan biaya pembuatan konten.',
    items: [
      {
        q: 'Apakah kode sumber OmniMux terbuka?',
        a: 'Ya. Kode sumber aplikasi desktop dan plugin tersedia di github.com/omnimux-ai/omnimux-client. Kode sumber gateway tersedia di github.com/omnimux-ai/omnimux-gateway. Keduanya berlisensi AGPL-3.0. Publikasi kode sumber tidak mencakup kredensial kanal, catatan harga, operasional, dan data pengguna layanan terkelola.',
      },
      {
        q: 'Bisakah langsung membuat konten setelah mengunduh?',
        a: 'Unduh, instal, dan masuk untuk membuka ruang kerja. Pembuatan konten menggunakan kredit sesuai saldo dompet dan catatan tugas. Konfirmasikan parameter dan penggunaan kredit sebelum membuat konten.',
      },
      {
        q: 'Platform mana saja yang didukung untuk publikasi?',
        a: 'Jadwalkan posting ke TikTok, Instagram, Facebook, dan YouTube dari pusat publikasi. Integrasi langsung TikTok nonaktif secara bawaan. Setelan privasi bawaannya adalah SELF_ONLY. Saat ini Douyin, Xiaohongshu, WeChat Channels, Kuaishou, dan Bilibili belum didukung.',
      },
      {
        q: 'Bisakah mengetahui biaya pasti sebelum membuat video?',
        a: 'Studio gambar menampilkan perkiraan sebagai acuan. Biaya video mengikuti catatan tugas dan tagihan aktual. Parameter dan penggunaan kredit dikonfirmasi sebelum pembuatan. Tarif pasti mengikuti setelan dompet.',
      },
      {
        q: 'Apakah hasilnya dijamin viral?',
        a: 'Ruang kerja OmniMux menyediakan alat untuk menganalisis referensi, membuat ulang struktur, dan membuat video. Penayangan, penjualan, dan hasil viral berada di luar cakupan jaminan. Keamanan akun juga bergantung pada konten dan cara Anda menggunakan akun.',
      },
    ],
  },

  finalCta: {
    h2: 'Video viral berikutnya, serahkan ke OmniMux',
    lead: 'Bedah hits, buat video, publikasikan, lalu evaluasi — satu Agent menjalankan seluruh siklus.',
    dlMac: 'Unduh untuk macOS',
    dlWin: 'Unduh untuk Windows',
    note: 'macOS 12+ (universal Apple silicon & Intel) · Windows 10 / 11 x64',
  },

  download: {
    title: 'Lanjutkan di OmniMux Desktop',
    desc: 'Unduh, instal, dan masuk untuk mulai membuat.',
    version: 'Versi stabil {version}',
    close: 'Tutup',
    macArmTitle: 'Unduh untuk macOS (Apple Silicon)',
    macArmSub: 'macOS 12+ · chip seri M',
    macIntelTitle: 'Unduh untuk macOS (Intel)',
    macIntelSub: 'macOS 12+ · chip Intel',
    winTitle: 'Unduh untuk Windows (installer x64)',
    winSub: 'Windows 10 / 11 x64',
    recommended: 'Direkomendasikan',
    unavailable: 'Unduhan tidak tersedia: versi stabil belum dirilis',
    notePre: 'Jika Windows menampilkan "Windows protected your PC": klik',
    noteMore: 'More info',
    noteRun: 'Run anyway',
    notePost: '. Unduh hanya dari dl.omnimux.ai.',
  },

  footer: {
    brand: 'Desktop',
    tagline: 'Temukan, buat ulang & gandakan video jualan viral.',
    columns: [
      {
        title: 'Jelajahi',
        links: [
          { label: 'Harga', href: '/pricing' },
          { label: 'Inspirasi', href: '/inspiration' },
          { label: 'Studio Influencer AI', href: '/ai-influencer-studio' },
          { label: 'Unduh Desktop', href: '#download' },
        ],
      },
      {
        title: 'Alat',
        links: [
          {
            label: 'Pengunduh Video TikTok',
            href: '/tools/tiktok-video-downloader',
          },
          {
            label: 'Pengunduh Foto TikTok',
            href: '/tools/tiktok-photo-downloader',
          },
          { label: 'Produk ke video', href: '#capabilities' },
          { label: 'Buat ulang viral', href: '#capabilities' },
          { label: 'Video ke prompt', href: '#capabilities' },
          { label: 'Set gambar produk', href: '#capabilities' },
          { label: 'Gambar halaman detail', href: '#capabilities' },
          { label: 'Carousel gambar', href: '#capabilities' },
          { label: 'Digital human', href: '#capabilities' },
          { label: 'Burn-in subtitle', href: '#capabilities' },
        ],
      },
      {
        title: 'Model',
        links: [
          { label: 'Seedance 2.5', href: '/pricing' },
          { label: 'Seedance 2.0', href: '/pricing' },
          { label: 'GPT Image 2.5', href: '/pricing' },
          { label: 'Nano Banana', href: '/pricing' },
          { label: 'Suno', href: '/pricing' },
        ],
      },
      {
        title: 'Sumber daya',
        links: [
          {
            label: 'Dokumentasi',
            href: docsQuickstartUrl('id'),
            external: true,
          },
          { label: 'Panduan', href: '/guides', external: true },
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
        title: 'Tentang',
        links: [
          { label: 'Tentang OmniMux', href: '/about' },
          { label: 'Kontak', href: 'mailto:hello@omnimux.ai', external: true },
          { label: 'Kebijakan privasi', href: '/privacy' },
          { label: 'Ketentuan layanan', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: 'Kode sumber',
  },

  toolPage: {
    crumbAria: 'Breadcrumb',
    crumbTools: 'Alat',
    kindAria: 'Jenis konten',
    kindVideo: 'Video',
    kindPhoto: 'Foto',
    inputLabel: 'Tautan TikTok',
    placeholderVideo: 'Tempel tautan video TikTok atau teks bagikan',
    placeholderPhoto: 'Tempel tautan postingan foto TikTok atau teks bagikan',
    paste: 'Tempel',
    parse: 'Urai',
    parsing: 'Mengurai',
    verifying: 'Memverifikasi',
    quotaAnon: 'Hari ini {remaining}/{limit}',
    quotaUser: 'Hari ini {remaining}/{limit}',
    turnstileAria: 'Verifikasi Cloudflare',
    unavailable: 'Sementara tidak tersedia',
    downloadMp4: 'Unduh MP4',
    recreateMine: 'Buat ulang',
    durationRange: 'Video referensi harus 4–30 detik (saat ini {duration})',
    breakdown: 'Bedah struktur viral',
    photoCount: '{count} foto',
    imageAlt: 'Foto {n}',
    downloadImage: 'Unduh foto {n}',
    downloadZip: 'Unduh semua sebagai ZIP',
    zipLocked: 'Masuk untuk memakai',
    recreatePhotos: 'Buat ulang set foto ini',
    ctaTitle: 'Simpan referensi, lalu buat ulang dengan karaktermu',
    ctaBody:
      'Ritme dan shot yang sama, dengan karakter AI dan produkmu sendiri.',
    ctaStart: 'Mulai buat ulang',
    ctaRef: 'Referensi',
    ctaMine: 'Hasil',
    relatedAria: 'Alat terkait',
    errors: {
      invalidLink: 'Tempel tautan TikTok',
      wrongKindPhoto: 'Ini tautan postingan foto',
      toPhoto: 'Ke Pengunduh Foto',
      wrongKindVideo: 'Ini tautan video',
      toVideo: 'Ke Pengunduh Video',
      unavailable: 'Postingan ini privat, dihapus atau tidak tersedia',
      serviceUnavailable: 'Urai belum dibuka di situs ini',
      upstreamFailed: 'TikTok tidak merespons, coba lagi sebentar lagi',
      quotaAnon: 'Kuota hari ini habis — masuk untuk lanjut',
      quotaUser: 'Kuota hari ini habis — kembali besok',
      busy: 'Kapasitas tamu hari ini penuh — masuk untuk lanjut',
      rateLimited: 'Terlalu cepat, coba lagi sebentar',
      turnstileFailed: 'Verifikasi gagal, coba lagi',
      turnstileUnavailable:
        'Verifikasi sementara tidak tersedia, coba lagi nanti',
      generic: 'Gagal mengurai tautan ini, coba lagi nanti',
    },
  },
}
