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
import { ASSETS } from '../../components/cn/data'
import type { ToolLocale } from '../content'
import type { ToolBreakdownResult, ToolResult, ToolTranscriptCue } from './api'
import { formatCueShortTime } from './transcript-format'

export type TranscriptDemoBundle = {
  result: ToolResult
  translationTarget: ToolLocale
  translation: readonly ToolTranscriptCue[]
  breakdown: ToolBreakdownResult
}

const DEMO_SOURCE_CUES: readonly ToolTranscriptCue[] = [
  {
    start_ms: 0,
    end_ms: 3100,
    text: 'Wait until you see how calm my morning skin looks after one week.',
  },
  {
    start_ms: 3100,
    end_ms: 6200,
    text: 'I swapped thick heavy creams for a single drop of barrier peptide serum.',
  },
  {
    start_ms: 6200,
    end_ms: 9600,
    text: 'Pat two drops on damp cheeks and it absorbs cleanly without sticky film.',
  },
  {
    start_ms: 9600,
    end_ms: 12600,
    text: 'Look at the cheek redness fading right beside the window light.',
  },
  {
    start_ms: 12600,
    end_ms: 15000,
    text: 'Tap the card below if you want the gentle travel duo before it sells out.',
  },
]

const DEMO_TRANSLATIONS: Record<ToolLocale, readonly string[]> = {
  'zh-CN': [
    '等你看清我连续使用一周后早晨皮肤有多稳定，就懂为什么要换护肤流程。',
    '我把厚重面霜换成了这一滴屏障修护肽精华。',
    '趁皮肤微湿轻拍两滴，吸收很快而且完全不黏腻。',
    '迎着自然窗光就能看到两颊泛红明显退下去。',
    '想入手这套温和便携组合的话，趁断货前点下方卡片看看。',
  ],
  'zh-TW': [
    '等你看清我連續使用一週後早晨膚況有多穩定，就明白為何要簡化步驟。',
    '我把厚重乳霜換成了這滴屏障修護胜肽精華。',
    '趁肌膚微濕輕拍兩滴，吸收迅速而且毫不黏膩。',
    '對著自然窗光就能看出雙頰泛紅明顯減輕。',
    '想入手這組溫和旅行裝，趁售罄前點下方卡片查看。',
  ],
  en: [
    'Wait until you see how calm my morning skin looks after one week.',
    'I swapped thick heavy creams for a single drop of barrier peptide serum.',
    'Pat two drops on damp cheeks and it absorbs cleanly without sticky film.',
    'Look at the cheek redness fading right beside the window light.',
    'Tap the card below if you want the gentle travel duo before it sells out.',
  ],
  ja: [
    '1週間続けたあとの朝の肌の落ち着きを見れば、切り替えた理由がわかります。',
    '重たいクリームの代わりに、このバリアペプチド美容液を1滴だけ使いました。',
    '少し湿った肌に2滴なじませると、べたつかずすっと浸透します。',
    '窓際の自然光で見ると、頬の赤みがかなり引いているのがわかります。',
    'やさしいトラベルセットは売り切れ前に下のカードからチェックしてください。',
  ],
  es: [
    'Espera a ver lo calmada que luce mi piel por la mañana tras una semana.',
    'Cambié las cremas densas por una sola gota de sérum reparador con péptidos.',
    'Aplica dos gotas sobre la piel húmeda y se absorbe sin sensación pegajosa.',
    'Mira cómo el enrojecimiento de las mejillas baja bajo la luz natural.',
    'Toca la tarjeta de abajo para conseguir el dúo de viaje antes de que se agote.',
  ],
  id: [
    'Lihat betapa tenangnya kulit pagiku setelah pemakaian selama satu minggu.',
    'Aku mengganti krim tebal dengan satu tetes serum peptida pelindung kulit.',
    'Tepukkan dua tetes saat kulit masih lembap, cepat meresap tanpa rasa lengket.',
    'Di dekat cahaya jendela terlihat kemerahan di pipi jauh lebih mereda.',
    'Ketuk kartu di bawah jika ingin paket travel lembut ini sebelum kehabisan.',
  ],
}

const DEMO_DESCRIPTIONS: Record<ToolLocale, string> = {
  'zh-CN':
    '护肤精华 UGC 短片脚本示例：演示开场钩子、产品引入、质地实测与结尾转化节奏。',
  'zh-TW':
    '護膚精華 UGC 短片腳本範例：示範開場鉤子、產品切入、質地實測與結尾轉換節奏。',
  en: 'Original skincare UGC clip example showing hook, texture proof, and conversion pacing.',
  ja: 'スキンケアUGC動画のサンプル台本：フック、製品導入、質感検証、CTAの流れを確認できます。',
  es: 'Ejemplo original de guion UGC: gancho visual, prueba de textura y cierre comercial.',
  id: 'Contoh naskah UGC perawatan kulit: pembuka kait, demo tekstur, dan ajakan beli.',
}

const DEMO_BREAKDOWNS: Record<ToolLocale, ToolBreakdownResult> = {
  'zh-CN': {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'zh-CN',
    summary:
      '15 秒标准电商口播结构：前三秒用前后反差抛结果，中段用上脸动作建立质感信任，最后配合紧迫感引流下单。',
    sections: [
      {
        stage: 'hook',
        label: '结果前置钩子',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          '开篇直接给出“一周后稳定状态”的预期，让有敏感泛红痛点的受众立刻停滑。',
      },
      {
        stage: 'product_intro',
        label: '方案替换切入',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis:
          '用“一滴替换厚重面霜”做精简护肤对比，突出核心成分与使用轻负担。',
      },
      {
        stage: 'demo_scene',
        label: '质感上脸演示',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis:
          '给出具体用量与肤感描述（微湿轻拍两滴、不黏腻），降低观众对新产品的试用顾虑。',
      },
      {
        stage: 'proof_effect',
        label: '自然光效果证明',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis: '主动强调“窗边自然光”，通过无美颜环境线索增强功效真实感。',
      },
      {
        stage: 'cta',
        label: '低门槛下单号召',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis:
          '以“便携装＋库存提醒”收口，引导从观望转化为点击下方入口下单。',
      },
    ],
  },
  'zh-TW': {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'zh-TW',
    summary:
      '15 秒電商口播範例結構：前三秒以前後變化吸引停留，中段實測質地與自然光佐證建立信任，最後以旅行組推動轉換。',
    sections: [
      {
        stage: 'hook',
        label: '結果前置鉤子',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          '開場先點出「使用一週後的穩定狀態」，迅速留住有肌膚敏弱困擾的受眾。',
      },
      {
        stage: 'product_intro',
        label: '精簡替代切入',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis: '用單滴勝肽精華對比厚重乳霜，凸顯精簡護膚與成分訴求。',
      },
      {
        stage: 'demo_scene',
        label: '使用體感示範',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis: '具體交代兩滴用法與不黏膩吸收速度，幫助買家建立使用想像。',
      },
      {
        stage: 'proof_effect',
        label: '自然光效果驗證',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis: '強調窗邊真實光線檢視泛紅變化，強化實測可信度。',
      },
      {
        stage: 'cta',
        label: '旅行組促購收口',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis: '用低門檻旅行組合搭配庫存提醒，自然帶出點擊卡片的下單動作。',
      },
    ],
  },
  en: {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'en',
    summary:
      'A tight 15-second commerce narrative: open with a 7-day result promise, contrast heavy routines, prove fast absorption, validate in daylight, and close with a travel-kit CTA.',
    sections: [
      {
        stage: 'hook',
        label: 'Result-First Hook',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          'Leads with a visible 7-day payoff to stop shoppers dealing with morning skin irritation.',
      },
      {
        stage: 'product_intro',
        label: 'Routine Simplification',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis:
          'Positions one peptide drop against thick heavy creams to frame the product as effortless.',
      },
      {
        stage: 'demo_scene',
        label: 'Texture & Usage Proof',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis:
          'Gives concrete dosage cues and addresses the main texture objection around stickiness.',
      },
      {
        stage: 'proof_effect',
        label: 'Natural Light Check',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis:
          'Invites viewers to inspect redness beside real window light to reinforce authenticity.',
      },
      {
        stage: 'cta',
        label: 'Low-Friction CTA',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis:
          'Pairs a lower-commitment travel duo with subtle scarcity to drive immediate clicks.',
      },
    ],
  },
  ja: {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'ja',
    summary:
      '15秒で完結する王道EC構成：冒頭で1週間後の変化を示し、使用感の実演と自然光での確認を経て、トライアル導線へつなぎます。',
    sections: [
      {
        stage: 'hook',
        label: '変化の先出しフック',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          '1週間後の落ち着いた肌状態を冒頭に置き、悩みが近い視聴者の離脱を防ぎます。',
      },
      {
        stage: 'product_intro',
        label: '置き換え訴求',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis:
          '重いクリームから1滴の美容液への切り替えというシンプルな比較で魅力を伝えます。',
      },
      {
        stage: 'demo_scene',
        label: '質感と使い方の実演',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis:
          '具体的な適量の目安とべたつかない仕上がりを示し、使用後のイメージを補います。',
      },
      {
        stage: 'proof_effect',
        label: '自然光での仕上がり確認',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis:
          '窓際の自然光に言及することで、加工感のないリアルな証拠として説得力を高めます。',
      },
      {
        stage: 'cta',
        label: 'お試しセット誘導',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis:
          '試しやすいトラベルサイズと在庫訴求で最後の行動を後押しします。',
      },
    ],
  },
  es: {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'es',
    summary:
      'Estructura comercial de 15 segundos: promete calma cutánea al inicio, contrasta texturas, muestra absorción real a luz natural y cierra con un formato de prueba.',
    sections: [
      {
        stage: 'hook',
        label: 'Gancho con resultado',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          'Abre con un cambio visible en siete días para frenar el scroll de quien sufre irritación.',
      },
      {
        stage: 'product_intro',
        label: 'Simplificación de rutina',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis:
          'Contrasta una sola gota frente a cremas densas para posicionar la ligereza del sérum.',
      },
      {
        stage: 'demo_scene',
        label: 'Prueba de absorción',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis:
          'Detalla dosis exacta y elimina la objeción habitual de sensación pegajosa.',
      },
      {
        stage: 'proof_effect',
        label: 'Validación con luz natural',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis:
          'Mencionar la luz de la ventana refuerza la autenticidad visual sin filtros.',
      },
      {
        stage: 'cta',
        label: 'Llamada a probar el dúo',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis:
          'Combina formato de viaje accesible con urgencia suave para dirigir al clic.',
      },
    ],
  },
  id: {
    video_id: 'demo-omnimux-ugc',
    track_id: 'demo-en',
    target_lang: 'id',
    summary:
      'Pola naskah dagang 15 detik: buka dengan hasil satu minggu, bandingkan tekstur, buktikan cepat meresap di cahaya alami, lalu tutup dengan paket hemat.',
    sections: [
      {
        stage: 'hook',
        label: 'Kait Hasil Awal',
        start_ms: 0,
        end_ms: 3100,
        quote: DEMO_SOURCE_CUES[0].text,
        analysis:
          'Langsung memperlihatkan hasil kulit tenang setelah 7 hari untuk menahan penonton yang relevan.',
      },
      {
        stage: 'product_intro',
        label: 'Ganti Rutinitas Berat',
        start_ms: 3100,
        end_ms: 6200,
        quote: DEMO_SOURCE_CUES[1].text,
        analysis:
          'Membandingkan krim tebal dengan satu tetes serum agar terasa ringan dan praktis.',
      },
      {
        stage: 'demo_scene',
        label: 'Demo Tekstur Nyata',
        start_ms: 6200,
        end_ms: 9600,
        quote: DEMO_SOURCE_CUES[2].text,
        analysis:
          'Menjelaskan takaran dua tetes dan penyerapan bersih tanpa rasa lengket.',
      },
      {
        stage: 'proof_effect',
        label: 'Bukti Cahaya Jendela',
        start_ms: 9600,
        end_ms: 12600,
        quote: DEMO_SOURCE_CUES[3].text,
        analysis:
          'Menyorot kondisi kemerahan di dekat jendela agar hasilnya tampak jujur tanpa filter.',
      },
      {
        stage: 'cta',
        label: 'Ajakan Paket Travel',
        start_ms: 12600,
        end_ms: 15000,
        quote: DEMO_SOURCE_CUES[4].text,
        analysis:
          'Mendorong klik lewat opsi ukuran travel yang mudah dicoba sebelum stok habis.',
      },
    ],
  },
}

export function getTranscriptDemo(locale: ToolLocale): TranscriptDemoBundle {
  const translationTarget: ToolLocale = locale === 'en' ? 'es' : locale
  const translatedLines = DEMO_TRANSLATIONS[translationTarget]
  return {
    result: {
      kind: 'transcript',
      id: 'demo-omnimux-ugc',
      author: { unique_id: 'omnimux_studio', nickname: 'OmniMux Studio' },
      description: DEMO_DESCRIPTIONS[locale],
      created_at: 0,
      cover_url: ASSETS.ugcPoster,
      video: {
        duration_ms: 15040,
        width: 720,
        height: 1280,
        preview_url: ASSETS.ugcDemo,
        download_url: ASSETS.ugcDemo,
      },
      transcript: {
        has_captions: true,
        tracks: [
          {
            id: 'demo-en',
            lang: 'eng-US',
            language_code: 'en',
            is_auto_generated: false,
            cues: [...DEMO_SOURCE_CUES],
          },
        ],
      },
      quota_exhausted: false,
    },
    translationTarget,
    translation: DEMO_SOURCE_CUES.map((cue, index) => ({
      ...cue,
      text: translatedLines[index] ?? cue.text,
    })),
    breakdown: DEMO_BREAKDOWNS[locale],
  }
}

export function formatBreakdownMarkdown(
  breakdown: ToolBreakdownResult
): string {
  const lines = [breakdown.summary, '']
  for (const section of breakdown.sections) {
    lines.push(
      `## ${section.label} (${formatCueShortTime(section.start_ms)} - ${formatCueShortTime(section.end_ms)})`
    )
    lines.push(`> ${section.quote}`)
    lines.push('')
    lines.push(section.analysis)
    lines.push('')
  }
  return lines.join('\n').trim()
}
