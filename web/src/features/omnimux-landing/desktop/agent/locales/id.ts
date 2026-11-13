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
// fork: teks Bahasa Indonesia untuk halaman masuk Agent. Istilah mengikuti
// landing yang sudah ada (video, gambar, unduh).

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: 'Pasang CLI',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: 'Cek dengan omnimux help. CLI terhubung ke omnimux.ai secara default.',
  },
  {
    title: 'Dapatkan API key',
    body: 'Buka konsol, masuk ke API Keys, lalu buat key. Salin sekali saja.',
    action: { label: 'Buat API key', href: '/keys' },
  },
  {
    title: 'Atur key',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: 'Tambahkan baris ini ke ~/.zshrc atau ~/.bashrc, muat ulang shell, lalu mulai ulang agent kamu.',
  },
]

const SKILL_STEP: AgentStep = {
  title: 'Pasang skills',
  command: 'omnimux skill install --user',
  body: 'Pasang CLI dulu. Skills masuk ke ~/.agents/skills; mulai ulang agent untuk memakainya.',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: 'Analisis video TikTok',
    body: 'Bedah naskah, shot, struktur hook, dan logika konversi video TikTok mana pun.',
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
    title: 'Buat video pendek',
    body: 'Ubah naskah atau brief kreatif menjadi video pendek bergaya TikTok.',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: 'Buat gambar AI',
    body: 'Buat gambar berkualitas untuk pemasaran, media sosial, atau proyek kreatif.',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: 'Pakai avatar digital',
    body: 'Ubah foto menjadi sosok yang konsisten untuk video bicara di depan kamera.',
    mock: { type: 'voice' },
  },
  {
    title: 'Tingkatkan video',
    body: 'Hapus subtitle dan watermark, lalu naikkan kualitas materi jadi HD siap tayang.',
    mock: {
      type: 'chips',
      chips: ['Hapus subtitle', 'Hapus watermark', 'Naikkan resolusi'],
    },
  },
  {
    title: 'Publikasikan ke TikTok',
    body: 'Publikasikan konten TikTok Shop serta video atau foto biasa dari agent lewat Open API dan CLI.',
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
      chips: ['Konten TikTok Shop', 'Video dan foto', 'Web, Open API, dan CLI'],
    },
  },
]

const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'Analisis video AI',
    body: 'Bedah pembuka, dialog, dan shot video referensi, dengan fokus pada iklan produk TikTok, lalu atur arah lewat percakapan.',
    mock: {
      type: 'chips',
      chips: ['Struktur pembuka', 'Bedah dialog', 'Rekonstruksi shot'],
    },
  },
  {
    title: 'Pembuat carousel TikTok',
    body: 'Ubah foto produk menjadi cover, teks per halaman, dan satu set carousel, dengan penyesuaian daya tarik dan urutan cerita tiap halaman.',
    mock: {
      type: 'chips',
      chips: ['Cover', 'Teks per halaman', 'Carousel lengkap'],
    },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: 'Bagaimana cara menganalisis video TikTok dengan agent AI?',
    a: 'Pasang CLI dan skills-nya, tempel tautan TikTok, lalu bilang “analisis video ini”. Agent memanggil skill analisis video dan mengembalikan rumus hook, struktur naskah, dan template yang bisa dipakai ulang.',
  },
  {
    q: 'Apa itu hook TikTok dan kenapa penting?',
    a: 'Hook menentukan apakah penonton bertahan di satu sampai tiga detik pertama. OmniMux menunjukkan pola pembuka, tempo, dan pemicu dari video yang berhasil.',
  },
  {
    q: 'Apa itu OmniMux?',
    a: 'OmniMux adalah platform pembuatan karakter AI dan rekreasi video referensi untuk penjual lintas negara TikTok, tersedia lewat konsol web, Open API, serta CLI dengan skills-nya.',
  },
  {
    q: 'Apa itu skills?',
    a: 'Skills adalah paket kemampuan yang bisa dimuat agent. omnimux skill install --user menaruhnya di ~/.agents/skills agar agent bisa memanggil alat dan API di baliknya tanpa kamu menulis kode.',
  },
  {
    q: 'Apakah ada biaya tambahan?',
    a: 'CLI dan skills tidak menambah biaya sendiri; pemakaian memakai saldo yang sama dengan situs. Codex, Claude Code, dan agent lain menetapkan harganya masing-masing.',
  },
  {
    q: 'Berapa lama satu analisis?',
    a: 'Biasanya 30 detik sampai satu menit, termasuk transkripsi, pemahaman visual, dan laporan terstruktur.',
  },
  {
    q: 'Bisakah CLI memublikasikan video TikTok?',
    a: 'Bisa, lewat perintah omnimux social. Hubungkan akun dulu dan sebutkan provider yang memilikinya.',
  },
  {
    q: 'Bagaimana kalau tidak berjalan setelah dipasang?',
    a: 'Mulai ulang agent, jalankan omnimux skill status untuk memastikan lokasi skills, dan omnimux balance untuk memastikan key valid.',
  },
]

const SHELL = {
  stepLabel: 'Penyiapan',
  compatTitle: 'Cocok dengan semua agent utama',
  compatLead:
    'Codex, Claude Code, Hermes, OpenClaw, dan hampir semua agent lain didukung.',
  flowLabels: {
    agents: 'Agent lokal kamu',
    hub: 'OmniMux CLI',
    caps: 'Kemampuan OmniMux',
  },
  coreTitle: 'Kemampuan inti',
  coreLead:
    'Yang paling penting: analisis TikTok, publikasi, video AI, pembuatan gambar, dan avatar digital.',
  copy: 'Salin',
  copied: 'Tersalin',
  kindAria: 'Masuk Agent',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: 'Satu key, semua kemampuan',
  ctaBody:
    'Buat key, berikan ke agent kamu, dan biarkan AI membedah video TikTok viral berikutnya.',
  ctaButton: 'Buat API key',
  faqTitle: 'Pertanyaan umum',
}

export const id: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS.id.cli,
    title:
      'OmniMux CLI — analisis TikTok, publikasi, dan generasi AI dari agent',
    description:
      'Pasang OmniMux CLI dan panggil analisis TikTok, pembuatan video dan gambar AI, serta publikasi dari Codex, Claude Code, OpenClaw, dan agent lain.',
    badge: 'AI Agent CLI for TikTok',
    h1: 'Satu perintah, kemampuan OmniMux di AI mana pun',
    subtitle:
      'Analisis video TikTok, buat gambar dan video, lalu publikasikan ke TikTok.',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: 'Satu pemasangan · satu key',
    flowCaps: [
      {
        title: 'Temukan yang berhasil',
        body: 'Bedah hook, struktur, dan naskah video TikTok mana pun',
      },
      {
        title: 'Buat konten',
        body: 'Video AI, gambar, avatar digital, dan peningkatan video',
      },
      {
        title: 'Publikasi dan kelola',
        body: 'TikTok Shop dan video biasa, terjadwal',
      },
    ],
    cards: CORE_CARDS,
    tryTitle: 'Coba',
    tryHeading: 'Dari prompt ke materi video dalam satu baris',
    tryBody:
      'Kirim idenya ke agent kamu; ia memanggil OmniMux dan mengembalikan video serta gambar shot sekaligus.',
    tryPrompts: [
      {
        label: 'Analisis struktur',
        text: 'Bedah struktur video TikTok ini: https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: 'Buat gambar shot',
        text: 'Buat storyboard video jualan TikTok 30 detik untuk alat kecantikan dengan hook kuat',
      },
      {
        label: 'Buat videonya',
        text: 'Bagus, sekarang buat videonya dari storyboard ini',
      },
    ],
    tryChat: {
      prompt:
        'Bedah struktur video TikTok ini: https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: 'Masalah → Solusi' },
        { label: 'Struktur', value: 'Hook → Nilai → CTA' },
        { label: 'Linimasa', value: '0:02 · 0:11 · 0:24' },
        { label: 'Template', value: 'Bisa dipakai ulang' },
      ],
      follow:
        'Buat storyboard video jualan TikTok 30 detik untuk alat kecantikan dengan hook kuat',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS.id.skills,
    title:
      'OmniMux Skills — analisis TikTok dan video pendek untuk agent mana pun',
    description:
      'Beri Codex, OpenClaw, Claude Code, dan agent lain skills OmniMux untuk menganalisis video TikTok viral dan membuat video pendek.',
    badge: 'AI Agent Skills for TikTok',
    h1: 'Beri AI mana pun skills OmniMux',
    subtitle:
      'Pasang skills OmniMux ke Codex, OpenClaw, Claude Code, dan lainnya untuk menganalisis video TikTok viral dan membuat video pendek.',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: 'Satu pemasangan · satu key',
    flowCaps: [
      {
        title: 'Temukan yang berhasil',
        body: 'Bedah hook, struktur, dan naskah video TikTok mana pun',
      },
      {
        title: 'Tiru yang viral',
        body: 'Pakai video pesaing atau referensi sebagai template untuk produkmu',
      },
      {
        title: 'Buat konten',
        body: 'Video AI, gambar, avatar digital, dan peningkatan video',
      },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: 'Coba',
    tryHeading: 'Satu tautan, bedah menyeluruh',
    tryBody:
      'Kirim prompt di bawah ke agent kamu; ia mentranskripsi, membaca materinya, lalu mengembalikan naskah dan analisis hook.',
    tryPrompts: [
      {
        label: 'Analisis video ini',
        text: 'Analisis video ini: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: 'Tiru strukturnya',
        text: 'Tulis naskah 30 detik untuk serum saya dengan struktur ini',
      },
    ],
    tryChat: {
      prompt:
        'Analisis video ini: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: 'Tulis naskah 30 detik untuk serum saya dengan struktur ini',
    },
    faq: FAQ,
  },
}
