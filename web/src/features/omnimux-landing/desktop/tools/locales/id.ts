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

const ID_FAQ: ToolPageContent['faq'] = [
  {
    q: 'Apakah harus daftar atau pasang aplikasi?',
    a: 'Tidak. Satu tautan publik langsung diproses di browser dengan pemeriksaan Cloudflare satu kali saat pertama pakai. Mengunduh seluruh set foto sebagai ZIP memerlukan masuk akun.',
  },
  {
    q: 'Berapa kali bisa dipakai setiap hari?',
    a: '5 kali per hari tanpa akun dan 50 kali per hari setelah masuk. Kuota diatur ulang pukul 00:00 UTC.',
  },
  {
    q: 'Tautan apa saja yang didukung?',
    a: 'Tautan video dan postingan foto TikTok publik, termasuk tautan pendek vm.tiktok.com, vt.tiktok.com, serta teks bagikan yang berisi tautan. Konten privat, khusus teman, dihapus, atau dibatasi wilayah tidak dapat diurai.',
  },
  {
    q: 'Apa format file yang diunduh?',
    a: 'Video berformat MP4. Postingan foto mempertahankan format gambar asli dari TikTok (JPEG atau WebP) bernomor sesuai urutan slide.',
  },
  {
    q: 'Bolehkah hasil unduhan dipakai komersial atau diunggah ulang?',
    a: 'Hak cipta tetap milik kreator asli. Simpan hanya materi yang berhak kamu gunakan dan cantumkan sumber bila dijadikan referensi. Untuk aset komersial, buat ulang dengan karaktermu sendiri di OmniMux.',
  },
  {
    q: 'Apakah tautan yang saya urai disimpan?',
    a: 'Untuk mencegah penyalahgunaan, kami hanya mencatat sementara jumlah permintaan dan ID postingan guna pembatasan kuota serta cache. File video tidak disimpan. Lihat kebijakan privasi untuk detailnya.',
  },
]

const ID_TRANSCRIPT_FAQ: ToolPageContent['faq'] = [
  {
    q: 'Dari mana sumber transkrip diambil?',
    a: 'OmniMux membaca jalur takarir WebVTT yang terpasang pada video TikTok publik, mencakup takarir pembuat maupun takarir otomatis jika tersedia.',
  },
  {
    q: 'Mengapa video tampil tanpa takarir asli?',
    a: 'Klip khusus musik, video dengan takarir dinonaktifkan kreator, dan slide foto tidak memiliki jalur takarir di TikTok. Kamu tetap dapat menyalin deskripsi atau menyimpan MP4.',
  },
  {
    q: 'Apakah harus daftar atau pasang aplikasi?',
    a: 'Tidak. Tautan publik langsung diekstrak di browser dengan pemeriksaan Cloudflare satu kali.',
  },
  {
    q: 'Berapa kali pemakaian harian yang tersedia?',
    a: '5 kali per hari tanpa akun dan 50 kali per hari setelah masuk, berbagi kuota antar alat TikTok dan diatur ulang setiap 00:00 UTC.',
  },
  {
    q: 'Format ekspor transkrip apa yang didukung?',
    a: 'Salin baris berstempel waktu atau teks polos secara langsung, atau ekspor file subtitle standar TXT, SRT, dan VTT.',
  },
  {
    q: 'Apakah video atau takarir saya disimpan?',
    a: 'Kami tidak menyimpan file video. Metadata postingan di-cache maksimal 30 menit bersama hitungan permintaan harian. Lihat kebijakan privasi.',
  },
]

export const id: Record<ToolKind, ToolPageContent> = {
  video: {
    path: TOOL_PATHS.id.video,
    title: 'Pengunduh Video TikTok — MP4 | OmniMux',
    description:
      'Tempel tautan video TikTok publik dan simpan langsung sebagai MP4 di browser. Setelahnya, buat ulang strukturnya menjadi video produk AI di OmniMux.',
    h1: 'Pengunduh Video TikTok',
    subtitle: 'Tempel tautan video TikTok publik dan simpan sebagai MP4',
    stepsTitle: 'Simpan video TikTok dalam 3 langkah',
    steps: [
      {
        title: 'Salin tautan',
        body: 'Di video TikTok, ketuk Bagikan → Salin tautan. Tautan pendek maupun lengkap keduanya berfungsi.',
      },
      {
        title: 'Tempel dan urai',
        body: 'Tempelkan ke kolom di atas lalu klik Urai.',
      },
      {
        title: 'Simpan',
        body: 'Tinjau hasilnya lalu unduh MP4 ke perangkatmu.',
      },
    ],
    faqTitle: 'Tanya Jawab',
    faq: ID_FAQ,
    related: [
      { label: 'Pengunduh Foto TikTok', href: TOOL_PATHS.id.photo },
      { label: 'Generator Transkrip TikTok', href: TOOL_PATHS.id.transcript },
      { label: 'Studio Influencer AI', href: '/ai-influencer-studio' },
    ],
  },
  photo: {
    path: TOOL_PATHS.id.photo,
    title: 'Pengunduh Foto TikTok — Simpan Gambar | OmniMux',
    description:
      'Simpan setiap gambar postingan foto TikTok publik sesuai urutan aslinya di browser, atau unduh semua sebagai ZIP setelah masuk.',
    h1: 'Pengunduh Foto TikTok',
    subtitle:
      'Tempel tautan postingan foto TikTok publik dan simpan gambar aslinya berurutan',
    stepsTitle: 'Simpan foto TikTok dalam 3 langkah',
    steps: [
      {
        title: 'Salin tautan',
        body: 'Di postingan foto TikTok, ketuk Bagikan → Salin tautan.',
      },
      {
        title: 'Tempel dan urai',
        body: 'Tempelkan ke kolom di atas lalu klik Urai.',
      },
      {
        title: 'Simpan',
        body: 'Simpan gambar satu per satu, atau masuk untuk mengunduh semua dalam ZIP.',
      },
    ],
    faqTitle: 'Tanya Jawab',
    faq: ID_FAQ,
    related: [
      { label: 'Pengunduh Video TikTok', href: TOOL_PATHS.id.video },
      { label: 'Generator Transkrip TikTok', href: TOOL_PATHS.id.transcript },
      { label: 'Studio Influencer AI', href: '/ai-influencer-studio' },
    ],
  },
  transcript: {
    path: TOOL_PATHS.id.transcript,
    title: 'Transkrip TikTok — TXT, SRT, VTT | OmniMux',
    description:
      'Tempel tautan video TikTok publik untuk mengambil takarir berstempel waktu, salin teks polos, atau ekspor file TXT, SRT, dan VTT di browser.',
    h1: 'Generator Transkrip TikTok',
    subtitle:
      'Tempel tautan video TikTok publik untuk ekstrak takarir dengan stempel waktu atau teks polos',
    stepsTitle: 'Ekstrak transkrip TikTok dalam 3 langkah',
    steps: [
      {
        title: 'Salin tautan video',
        body: 'Di video TikTok, ketuk Bagikan → Salin tautan atau tempel teks bagikan.',
      },
      {
        title: 'Ambil takarir',
        body: 'Klik Ambil transkrip untuk memuat jalur takarir TikTok dan memilih bahasa.',
      },
      {
        title: 'Salin atau ekspor',
        body: 'Ubah tampilan waktu atau teks polos, saring baris, lalu unduh TXT, SRT, dan VTT.',
      },
    ],
    faqTitle: 'Tanya Jawab',
    faq: ID_TRANSCRIPT_FAQ,
    related: [
      { label: 'Pengunduh Video TikTok', href: TOOL_PATHS.id.video },
      { label: 'Pengunduh Foto TikTok', href: TOOL_PATHS.id.photo },
      { label: 'Studio Influencer AI', href: '/ai-influencer-studio' },
    ],
  },
}
