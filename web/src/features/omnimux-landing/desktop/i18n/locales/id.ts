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

export const id: LandingCopy = {
  meta: {
    title:
      'OmniMux Desktop — Temukan, Buat Ulang & Gandakan Video Jualan Viral',
  },
  skip: 'Lewati ke konten',

  nav: {
    brand: 'Desktop',
    home: 'Beranda',
    models: 'Model',
    tools: 'Alat',
    ext: 'Ekstensi',
    pricing: 'Harga',
    resources: 'Sumber daya',
    download: 'Unduh',
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
    titleA: 'Temukan, buat ulang & gandakan',
    titleB: 'video jualan viral',
    subPre:
      'Satu sistem content commerce yang membawa keunggulan produk Anda ke setiap video. Gunakan ',
    subHighlight: 'Agent yang sudah Anda kenal',
    subPost:
      ' untuk membuat video iklan, jualan, dan ulasan produk di satu tempat.',
    tabsAria: 'Cara membuat',
    tabs: { agent: 'Agent', video: 'Video', image: 'Gambar' },
    inputLabel: 'Deskripsikan konten yang ingin dibuat',
    attachAria: 'Tambahkan video atau gambar referensi',
    sendAria: 'Mulai di desktop',
    dlMac: 'Unduh untuk macOS',
    dlWin: 'Unduh untuk Windows',
    dlMenuAria: 'Pilih versi unduhan',
    ghStar: 'Beri bintang di GitHub',
    dlMacArm: 'macOS (Apple Silicon)',
    dlMacIntel: 'macOS (Intel)',
    dlWinX64: 'Windows (64-bit)',
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
    linkCard: 'tiktok.com/@pet.diary/video/73421 · kucing lucu + unboxing cat food',
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
    h2: 'Dari ide produk menjadi video jualan',
    lineAria: 'Temukan · Analisis · Buat · Publikasi · Evaluasi',
    steps: [
      {
        kind: 'Temukan',
        title: 'Temukan video jualan viral di kategorimu',
        desc: 'Kumpulkan video referensi dari mana pun kamu menemukannya, lalu bedah hook dan selling point-nya.',
        chips: ['Video jualan & postingan viral'],
        primary: 'Bedah satu video referensi',
        secondary: 'Kumpulkan inspirasi lewat ekstensi browser',
      },
      {
        kind: 'Analisis',
        title: 'Lihat kenapa video itu laku',
        desc: 'Bedah hook, selling point, dan alur pembelian video viral jadi struktur yang bisa dipakai ulang. Bisa juga menganalisis akun kompetitor.',
        chips: ['Bedah naskah', 'Akun kompetitor', 'Tren kategori'],
        primary: 'Bedah satu video viral',
      },
      {
        kind: 'Buat',
        title: 'Ubah produkmu jadi video jualan',
        desc: 'Buat video orisinal dari foto atau tautan produk, atau buat ulang video viral dengan produkmu sendiri.',
        primary: 'Buat video',
        secondary: 'Buat ulang video viral',
      },
      {
        kind: 'Publikasi',
        title: 'Publikasikan ke TikTok, Instagram, Facebook & YouTube',
        desc: 'Hubungkan akunmu, lalu publikasikan video dan postingan gambar jualan secara langsung atau terjadwal.',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: 'Publikasikan video',
        secondary: 'Postingan video · Postingan gambar',
      },
      {
        kind: 'Evaluasi',
        title: 'Ubah feedback nyata jadi video berikutnya',
        desc: 'Setelah akun terhubung, views, like, dan komentar tersinkron otomatis — Agent memberi tahu apa yang harus dibuat selanjutnya.',
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

  proof: {
    h2: 'Dibuat untuk penjual lintas negara',
    lead: 'Temukan hits, bedah, buat, dan publikasikan — semua di satu aplikasi desktop.',
    facts: [
      { value: '17', label: 'skill bawaan' },
      { value: '7', label: 'templat video jadi' },
      { value: '4', label: 'platform publikasi' },
    ],
    platformsAria: 'Platform publikasi yang didukung',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: [
      'Konfirmasi parameter & kredit sebelum membuat',
      'Kredit otomatis dikembalikan untuk tugas yang gagal',
    ],
  },

  faq: {
    h2: 'Pertanyaan yang sering diajukan',
    lead: 'Yang paling sering ditanyakan penjual e-commerce',
    items: [
      {
        q: 'Apakah OmniMux open source?',
        a: 'Ya. Klien desktop, plugin, dan kode gateway diterbitkan per rilis di github.com/omnimux-ai/omnimux-ai dengan lisensi AGPL-3.0. Layanan terkelola tidak termasuk kredensial kanal, buku harga, operasional, dan data pengguna.',
      },
      {
        q: 'Apa yang bisa dilakukan OmniMux Agent?',
        a: 'OmniMux Agent menganalisis video referensi, menulis naskah, membuat video dan gambar, serta membantu mempublikasikan konten ke platform yang sudah kamu hubungkan. Jelaskan produk dan tujuanmu dengan bahasa sehari-hari, dan ia memilih alat yang tepat untuk tiap langkah.',
      },
      {
        q: 'Apakah Agent akan membuat atau mempublikasikan tanpa persetujuan saya?',
        a: 'Tidak. Sebelum membuat video atau gambar — dan sebelum mempublikasikan — Agent menampilkan parameter dan estimasi biaya kredit lalu menunggu konfirmasimu. Kamu bisa menyesuaikan parameter dulu. Jika tugas gagal karena masalah sistem, kredit yang terpotong dikembalikan.',
      },
      {
        q: 'Bagaimana cara membuat video jualan dengan AI?',
        a: 'Tempel tautan produk atau pilih referensi viral, buat video orisinal atau hasil buat ulang, lalu publikasikan ke platform yang sudah kamu hubungkan. Tanpa syuting atau editing.',
      },
      {
        q: 'Bisakah video yang dibuat langsung dipublikasikan?',
        a: 'Bisa. Hubungkan akunmu sekali di aplikasi desktop lalu publikasikan langsung — tanpa upload manual. Kamu bisa menambahkan deklarasi konten AI yang diwajibkan tiap platform saat publikasi.',
      },
      {
        q: 'Apakah video buatan AI bisa membuat akun saya diblokir?',
        a: 'OmniMux tidak bisa menjamin bebas blokir — keamanan akun tergantung konten dan perilaku akunmu. Saat publikasi kamu bisa menambahkan deklarasi konten hasil AI ke video; mohon patuhi kebijakan masing-masing platform.',
      },
      {
        q: 'Bisakah mengubah gambar produk menjadi video?',
        a: 'Bisa. Unggah gambar produk atau tempel tautan produk, dan AI membuat video showcase yang siap dipublikasikan — tanpa syuting.',
      },
      {
        q: 'Bagaimana cara membuat ulang video viral kompetitor?',
        a: 'AI mempelajari cara mereka berkarya — ritme, framing, tone — lalu membuat video baru dengan produk dan footage orisinalmu. Yang kamu tiru adalah pendekatannya, bukan klip aslinya.',
      },
      {
        q: 'Bisakah video yang dibuat dipakai secara komersial?',
        a: 'Penggunaan komersial mengikuti Ketentuan Layanan. Untuk foto orang, gambar produk, dan video referensi yang diunggah, kamu mengonfirmasi memiliki hak penggunaan dan mematuhi aturan tiap platform.',
      },
    ],
  },

  finalCta: {
    h2: 'Video viral berikutnya, serahkan ke OmniMux',
    lead: 'Bedah hits, buat video, publikasikan, lalu evaluasi — satu Agent menjalankan seluruh siklus.',
    dlMac: 'Unduh gratis untuk macOS',
    dlWin: 'Unduh untuk Windows',
    note: 'macOS 12+ (universal Apple silicon & Intel) · Windows 10 / 11 x64',
  },

  download: {
    title: 'Lanjutkan di OmniMux Desktop',
    desc: 'Unduh, instal, dan masuk untuk mulai membuat.',
    close: 'Tutup',
    macTitle: 'Unduh untuk macOS (Universal)',
    macSub: 'macOS 12+ · Apple silicon & Intel',
    winTitle: 'Unduh untuk Windows (installer x64)',
    winSub: 'Windows 10 / 11 x64',
    recommended: 'Direkomendasikan',
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
          { label: 'Produk ke video', href: '#tools' },
          { label: 'Buat ulang viral', href: '#tools' },
          { label: 'Video ke prompt', href: '#tools' },
          { label: 'Set gambar produk', href: '#tools' },
          { label: 'Gambar halaman detail', href: '#tools' },
          { label: 'Carousel gambar', href: '#tools' },
          { label: 'Digital human', href: '#tools' },
          { label: 'Burn-in subtitle', href: '#tools' },
        ],
      },
      {
        title: 'Model',
        links: [
          { label: 'Seedance 2.5', href: '#models' },
          { label: 'Seedance 2.0', href: '#models' },
          { label: 'GPT Image 2.5', href: '#models' },
          { label: 'Nano Banana', href: '#models' },
          { label: 'Suno', href: '#models' },
        ],
      },
      {
        title: 'Sumber daya',
        links: [
          {
            label: 'Dokumentasi',
            href: 'https://docs.omnimux.ai',
            external: true,
          },
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
    quotaAnon: 'Gratis hari ini {remaining}/{limit}',
    quotaUser: 'Hari ini {remaining}/{limit}',
    turnstileAria: 'Verifikasi Cloudflare',
    unavailable: 'Sementara tidak tersedia',
    downloadMp4: 'Unduh MP4',
    recreateMine: 'Buat ulang dengan karakter saya',
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
      quotaAnon: 'Kuota gratis hari ini habis — masuk untuk lanjut',
      quotaUser: 'Kuota hari ini habis — kembali besok',
      busy: 'Kapasitas gratis hari ini penuh — masuk untuk lanjut',
      rateLimited: 'Terlalu cepat, coba lagi sebentar',
      turnstileFailed: 'Verifikasi gagal, coba lagi',
      turnstileUnavailable:
        'Verifikasi sementara tidak tersedia, coba lagi nanti',
      generic: 'Gagal mengurai tautan ini, coba lagi nanti',
    },
  },
}
