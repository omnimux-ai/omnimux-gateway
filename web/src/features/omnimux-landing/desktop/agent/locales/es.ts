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
// fork: textos en español de las páginas de entrada del Agent. La terminología
// sigue la landing existente (vídeo, imágenes, descargar).

import {
  AGENT_PATHS,
  type AgentCard,
  type AgentPageContent,
  type AgentStep,
} from '../content'

const AGENTS_STEPS: AgentStep[] = [
  {
    title: 'Instala la CLI',
    command: 'curl -fsSL https://omnimux.ai/install.sh | bash',
    body: 'Comprueba con omnimux help. La CLI apunta a omnimux.ai por defecto.',
  },
  {
    title: 'Obtén una API key',
    body: 'Abre la consola, entra en API Keys y crea una key. Cópiala una sola vez.',
    action: { label: 'Crear API key', href: '/keys' },
  },
  {
    title: 'Configura la key',
    command: 'export OMNIMUX_ACCESS_TOKEN="sk-xxx"',
    body: 'Añade esta línea a ~/.zshrc o ~/.bashrc, recarga la shell y reinicia tu agent.',
  },
]

const SKILL_STEP: AgentStep = {
  title: 'Instala los skills',
  command: 'omnimux skill install --user',
  body: 'Instala antes la CLI. Los skills quedan en ~/.agents/skills; reinicia el agent para usarlos.',
}

const CORE_CARDS: AgentCard[] = [
  {
    title: 'Analiza un vídeo de TikTok',
    body: 'Desglosa el guion, los planos, la estructura del hook y la lógica de conversión de cualquier TikTok.',
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
    title: 'Genera vídeos cortos',
    body: 'Convierte un guion o un brief creativo en un vídeo corto al estilo TikTok.',
    mock: {
      type: 'prompt',
      text: 'Generate a 30s TikTok selling video about beauty device…',
      icon: 'video',
    },
  },
  {
    title: 'Genera imágenes con IA',
    body: 'Crea imágenes de alta calidad para marketing, redes sociales o un proyecto creativo.',
    mock: {
      type: 'prompt',
      text: 'A product photo of skincare serum, studio light, clean white background…',
      icon: 'img',
    },
  },
  {
    title: 'Usa un avatar digital',
    body: 'Convierte una foto en un personaje consistente para vídeos con locución.',
    mock: { type: 'voice' },
  },
  {
    title: 'Mejora el vídeo',
    body: 'Quita subtítulos y marcas de agua y escala el material para publicarlo en HD.',
    mock: {
      type: 'chips',
      chips: ['Quitar subtítulos', 'Quitar marca', 'Escalar'],
    },
  },
  {
    title: 'Publica en TikTok',
    body: 'Publica contenido de TikTok Shop y vídeos o fotos normales desde un agent con la Open API y la CLI.',
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
      chips: ['Contenido TikTok Shop', 'Vídeos y fotos', 'Web, Open API y CLI'],
    },
  },
]

const SKILL_EXTRA_CARDS: AgentCard[] = [
  {
    title: 'Análisis de vídeo con IA',
    body: 'Desglosa la apertura, los diálogos y los planos de un vídeo de referencia, con foco en anuncios de producto de TikTok, y ajusta la dirección conversando.',
    mock: {
      type: 'chips',
      chips: [
        'Estructura de apertura',
        'Desglose de diálogo',
        'Reconstrucción de planos',
      ],
    },
  },
  {
    title: 'Generador de carrusel de TikTok',
    body: 'Convierte una foto de producto en portada, textos página a página y un carrusel completo, ajustando el argumento y el orden narrativo de cada página.',
    mock: {
      type: 'chips',
      chips: ['Portada', 'Texto por página', 'Carrusel completo'],
    },
  },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: '¿Cómo analizo un vídeo de TikTok con un agent de IA?',
    a: 'Instala la CLI y sus skills, pega un enlace de TikTok y di «analiza este vídeo». El agent llama al skill de análisis y devuelve la fórmula del hook, la estructura del guion y una plantilla reutilizable.',
  },
  {
    q: '¿Qué es un hook de TikTok y por qué importa?',
    a: 'El hook decide si alguien se queda durante los primeros uno a tres segundos. OmniMux identifica el patrón de apertura, el ritmo y los puntos de activación de un vídeo que funcionó.',
  },
  {
    q: '¿Qué es OmniMux?',
    a: 'OmniMux es una plataforma de generación de personajes con IA y recreación de vídeos de referencia para vendedores transfronterizos de TikTok, disponible en la consola web, la Open API y esta CLI con sus skills.',
  },
  {
    q: '¿Qué son los skills?',
    a: 'Los skills son paquetes de capacidades que un agent puede cargar. omnimux skill install --user los deja en ~/.agents/skills para que el agent use las herramientas y APIs sin que escribas código.',
  },
  {
    q: '¿Cuesta más?',
    a: 'La CLI y los skills no añaden coste aparte; el consumo sale del mismo saldo que la web. Codex, Claude Code y otros agents fijan sus propios precios.',
  },
  {
    q: '¿Cuánto tarda un análisis?',
    a: 'Normalmente entre 30 segundos y un minuto, incluida la transcripción, la comprensión visual y el informe estructurado.',
  },
  {
    q: '¿Puede la CLI publicar vídeos en TikTok?',
    a: 'Sí, con los comandos omnimux social. Conecta antes la cuenta e indica el provider al que pertenece.',
  },
  {
    q: '¿Y si no funciona tras instalarlo?',
    a: 'Reinicia el agent, ejecuta omnimux skill status para ver dónde quedaron los skills y omnimux balance para comprobar que la key es válida.',
  },
]

const SHELL = {
  stepLabel: 'Configuración',
  compatTitle: 'Compatible con los principales agents',
  compatLead:
    'Admite Codex, Claude Code, Hermes, OpenClaw y la mayoría de los agents.',
  flowLabels: {
    agents: 'Tu agent local',
    hub: 'OmniMux CLI',
    caps: 'Capacidades de OmniMux',
  },
  coreTitle: 'Capacidades principales',
  coreLead:
    'Lo esencial: análisis de TikTok, publicación, vídeo con IA, creación de imágenes y avatares digitales.',
  copy: 'Copiar',
  copied: 'Copiado',
  kindAria: 'Entrada del Agent',
  kindCli: 'CLI',
  kindSkills: 'Skills',
  ctaTitle: 'Una key, todas las capacidades',
  ctaBody:
    'Crea una key, pásasela a tu agent y deja que la IA desglose el próximo éxito de TikTok.',
  ctaButton: 'Crear API key',
  faqTitle: 'Preguntas frecuentes',
}

export const es: Record<'cli' | 'skills', AgentPageContent> = {
  cli: {
    ...SHELL,
    path: AGENT_PATHS.es.cli,
    title: 'OmniMux CLI — análisis de TikTok, publicación y generación con IA',
    description:
      'Instala OmniMux CLI y llama al análisis de TikTok, a la generación de vídeo e imagen con IA y a la publicación desde Codex, Claude Code, OpenClaw y otros agents.',
    badge: 'AI Agent CLI for TikTok',
    h1: 'Un comando y OmniMux en cualquier IA',
    subtitle:
      'Analiza vídeos de TikTok, genera imágenes y vídeo, y publica en TikTok.',
    steps: AGENTS_STEPS,
    hubTitle: 'omnimux',
    hubSub: 'Una instalación · una key',
    flowCaps: [
      {
        title: 'Encuentra lo que funciona',
        body: 'Desglosa el hook, la estructura y el guion de cualquier TikTok',
      },
      {
        title: 'Crea contenido',
        body: 'Vídeo con IA, imágenes, avatares digitales y mejora de vídeo',
      },
      {
        title: 'Publica y opera',
        body: 'TikTok Shop y vídeos normales, programados',
      },
    ],
    cards: CORE_CARDS,
    tryTitle: 'Pruébalo',
    tryHeading: 'Del prompt al material en una línea',
    tryBody:
      'Envía la idea a tu agent; llama a OmniMux y devuelve vídeo e imágenes de plano de una vez.',
    tryPrompts: [
      {
        label: 'Analiza la estructura',
        text: 'Analiza la estructura de este TikTok: https://www.tiktok.com/@user/video/xxxx',
      },
      {
        label: 'Genera imágenes de plano',
        text: 'Haz el storyboard de un vídeo de venta de 30s para un dispositivo de belleza con un hook fuerte',
      },
      {
        label: 'Genera el vídeo',
        text: 'Bien, ahora genera el vídeo a partir de este storyboard',
      },
    ],
    tryChat: {
      prompt:
        'Analiza la estructura de este TikTok: https://www.tiktok.com/@user/video/xxxx',
      done: [
        { label: 'Hook', value: 'Problema → Solución' },
        { label: 'Estructura', value: 'Hook → Valor → CTA' },
        { label: 'Cronología', value: '0:02 · 0:11 · 0:24' },
        { label: 'Plantilla', value: 'Reutilizable' },
      ],
      follow:
        'Haz el storyboard de un vídeo de venta de 30s para un dispositivo de belleza con un hook fuerte',
    },
    faq: FAQ,
  },
  skills: {
    ...SHELL,
    path: AGENT_PATHS.es.skills,
    title:
      'OmniMux Skills — análisis de TikTok y vídeo corto para cualquier agent',
    description:
      'Da a Codex, OpenClaw, Claude Code y otros agents los skills de OmniMux para analizar éxitos de TikTok y generar vídeos cortos.',
    badge: 'AI Agent Skills for TikTok',
    h1: 'Dale a cualquier IA los skills de OmniMux',
    subtitle:
      'Instala los skills de OmniMux en Codex, OpenClaw, Claude Code y otros para analizar éxitos de TikTok y generar vídeos cortos.',
    steps: [SKILL_STEP, AGENTS_STEPS[1], AGENTS_STEPS[2]],
    hubTitle: 'OmniMux Skills',
    hubSub: 'Una instalación · una key',
    flowCaps: [
      {
        title: 'Encuentra lo que funciona',
        body: 'Desglosa el hook, la estructura y el guion de cualquier TikTok',
      },
      {
        title: 'Recrea un éxito',
        body: 'Usa un vídeo de la competencia o de referencia como plantilla para tu producto',
      },
      {
        title: 'Crea contenido',
        body: 'Vídeo con IA, imágenes, avatares digitales y mejora de vídeo',
      },
    ],
    cards: [...CORE_CARDS, ...SKILL_EXTRA_CARDS],
    tryTitle: 'Pruébalo',
    tryHeading: 'Un enlace, el desglose completo',
    tryBody:
      'Envía el prompt a tu agent; transcribe, lee el material y devuelve el guion y el análisis del hook.',
    tryPrompts: [
      {
        label: 'Analiza este vídeo',
        text: 'Analiza este vídeo: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      },
      {
        label: 'Recrea la estructura',
        text: 'Escribe un guion de 30s para mi serum con esta estructura',
      },
    ],
    tryChat: {
      prompt:
        'Analiza este vídeo: https://www.tiktok.com/@fediukstore/video/7611537882628607263',
      done: [],
      follow: 'Escribe un guion de 30s para mi serum con esta estructura',
    },
    faq: FAQ,
  },
}
