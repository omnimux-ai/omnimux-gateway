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

const ES_FAQ: ToolPageContent['faq'] = [
  {
    q: '¿Necesito registrarme o instalar algo?',
    a: 'No. Un enlace público se analiza directamente en tu navegador, con una comprobación única de Cloudflare en el primer uso. Para bajar todo un carrusel de fotos en ZIP hace falta iniciar sesión.',
  },
  {
    q: '¿Cuántas veces puedo usarlo al día?',
    a: '5 veces al día sin cuenta y 50 veces al día tras iniciar sesión. El límite se reinicia a las 00:00 UTC.',
  },
  {
    q: '¿Qué enlaces son compatibles?',
    a: 'Enlaces públicos de vídeos y publicaciones de fotos de TikTok, incluidos enlaces cortos vm.tiktok.com y vt.tiktok.com y texto compartido que incluya un enlace. No se pueden analizar publicaciones privadas, solo para amigos, eliminadas o bloqueadas por región.',
  },
  {
    q: '¿En qué formato se descargan los archivos?',
    a: 'Los vídeos se guardan en MP4. Las publicaciones de fotos conservan el formato original de TikTok (JPEG o WebP), numerado según el orden del carrusel.',
  },
  {
    q: '¿Puedo usar las descargas comercialmente o republicarlas?',
    a: 'Los derechos pertenecen al creador original. Guarda solo contenido que tengas derecho a usar y cita la fuente como referencia. Para material comercial, genera tu propia versión con tu personaje en OmniMux.',
  },
  {
    q: '¿Guardan los enlaces que analizo?',
    a: 'Para evitar abusos registramos brevemente recuentos de solicitudes e IDs de publicación para límites y caché. No almacenamos archivos de vídeo. Consulta la política de privacidad para más detalles.',
  },
]

const ES_TRANSCRIPT_FAQ: ToolPageContent['faq'] = [
  {
    q: '¿De dónde salen las transcripciones?',
    a: 'OmniMux lee las pistas WebVTT publicadas dentro de un vídeo público de TikTok. Muestra tanto subtítulos del creador como subtítulos automáticos cuando están disponibles.',
  },
  {
    q: '¿Por qué un vídeo aparece sin subtítulos nativos?',
    a: 'Los clips solo con música, vídeos sin subtítulos habilitados y carruseles de fotos no contienen pista de subtítulos en TikTok. En ese caso puedes copiar la descripción o guardar el MP4.',
  },
  {
    q: '¿Necesito registrarme o instalar algo?',
    a: 'No. Cada enlace público se procesa en tu navegador con una verificación única de Cloudflare al primer análisis.',
  },
  {
    q: '¿Cuántas veces puedo usarlo al día?',
    a: '5 veces al día sin cuenta y 50 veces al día al iniciar sesión, compartidas entre las herramientas de TikTok. Se reinicia a las 00:00 UTC.',
  },
  {
    q: '¿Qué formatos de exportación admite?',
    a: 'Copia líneas con marcas de tiempo o texto plano directamente, o exporta archivos estándar TXT, SRT y VTT.',
  },
  {
    q: '¿Almacenan los vídeos o subtítulos que analizo?',
    a: 'No almacenamos los archivos de vídeo. Los metadatos de la publicación se guardan en caché un máximo de 30 minutos junto con el conteo diario. Consulta la política de privacidad.',
  },
]

export const es: Record<ToolKind, ToolPageContent> = {
  video: {
    path: TOOL_PATHS.es.video,
    title: 'Descargar vídeos de TikTok — MP4 | OmniMux',
    description:
      'Pega un enlace público de TikTok y guárdalo como MP4 en tu navegador. Después puedes recrear la estructura del vídeo con IA en OmniMux.',
    h1: 'Descargador de vídeos de TikTok',
    subtitle: 'Pega un enlace público de TikTok y guárdalo como MP4',
    stepsTitle: 'Guarda un vídeo de TikTok en 3 pasos',
    steps: [
      {
        title: 'Copia el enlace',
        body: 'En el vídeo de TikTok, pulsa Compartir → Copiar enlace. Funcionan tanto enlaces cortos como completos.',
      },
      {
        title: 'Pega y analiza',
        body: 'Pégalo en el cuadro superior y pulsa Analizar.',
      },
      {
        title: 'Guarda',
        body: 'Previsualízalo y descarga el archivo MP4 a tu dispositivo.',
      },
    ],
    faqTitle: 'Preguntas frecuentes',
    faq: ES_FAQ,
    related: [
      { label: 'Descargador de fotos de TikTok', href: TOOL_PATHS.es.photo },
      {
        label: 'Extractor de subtítulos de TikTok',
        href: TOOL_PATHS.es.transcript,
      },
      { label: 'Estudio de influencers IA', href: '/ai-influencer-studio' },
    ],
  },
  photo: {
    path: TOOL_PATHS.es.photo,
    title: 'Descargar fotos de TikTok | OmniMux',
    description:
      'Guarda en orden todas las imágenes originales de un carrusel público de TikTok desde tu navegador o descárgalas en ZIP al iniciar sesión.',
    h1: 'Descargador de fotos de TikTok',
    subtitle:
      'Pega el enlace público de fotos de TikTok y guarda las imágenes originales en orden',
    stepsTitle: 'Guarda fotos de TikTok en 3 pasos',
    steps: [
      {
        title: 'Copia el enlace',
        body: 'En la publicación de fotos de TikTok, pulsa Compartir → Copiar enlace.',
      },
      {
        title: 'Pega y analiza',
        body: 'Pégalo arriba y pulsa Analizar.',
      },
      {
        title: 'Guarda',
        body: 'Descarga las imágenes una a una o inicia sesión para bajarlas todas en un ZIP.',
      },
    ],
    faqTitle: 'Preguntas frecuentes',
    faq: ES_FAQ,
    related: [
      { label: 'Descargador de vídeos de TikTok', href: TOOL_PATHS.es.video },
      {
        label: 'Extractor de subtítulos de TikTok',
        href: TOOL_PATHS.es.transcript,
      },
      { label: 'Estudio de influencers IA', href: '/ai-influencer-studio' },
    ],
  },
  transcript: {
    path: TOOL_PATHS.es.transcript,
    title: 'Subtítulos de TikTok — TXT y SRT | OmniMux',
    description:
      'Extrae subtítulos con marcas de tiempo, copia el texto completo o exporta archivos TXT, SRT y VTT desde vídeos públicos de TikTok.',
    h1: 'Extractor de transcripción de TikTok',
    subtitle:
      'Pega un enlace público de TikTok para extraer subtítulos con marcas de tiempo o texto plano',
    stepsTitle: 'Extrae una transcripción de TikTok en 3 pasos',
    steps: [
      {
        title: 'Copia el enlace del vídeo',
        body: 'En el vídeo de TikTok, pulsa Compartir → Copiar enlace o pega el texto compartido.',
      },
      {
        title: 'Extrae los subtítulos',
        body: 'Pulsa Extraer texto para cargar las pistas de subtítulos disponibles y cambiar de idioma.',
      },
      {
        title: 'Copia o exporta',
        body: 'Alterna entre tiempo y texto plano, filtra líneas o descarga TXT, SRT y VTT.',
      },
    ],
    faqTitle: 'Preguntas frecuentes',
    faq: ES_TRANSCRIPT_FAQ,
    related: [
      { label: 'Descargador de vídeos de TikTok', href: TOOL_PATHS.es.video },
      { label: 'Descargador de fotos de TikTok', href: TOOL_PATHS.es.photo },
      { label: 'Estudio de influencers IA', href: '/ai-influencer-studio' },
    ],
  },
}
