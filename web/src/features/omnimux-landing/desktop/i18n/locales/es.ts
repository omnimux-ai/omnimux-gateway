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

export const es: LandingCopy = {
  meta: {
    title:
      'OmniMux Desktop — Descubre, recrea y escala vídeos virales de venta',
  },
  skip: 'Saltar al contenido',

  nav: {
    brand: 'Desktop',
    home: 'Inicio',
    models: 'Modelos',
    tools: 'Herramientas',
    ext: 'Extensión',
    pricing: 'Precios',
    resources: 'Recursos',
    download: 'Descargar',
    login: 'Iniciar sesión',
    menuOpen: 'Abrir menú',
    langTitle: 'Idioma de la página',
    brandAria: 'Inicio de OmniMux Desktop',
    navAria: 'Navegación principal',
    toolsMenu: {
      aria: 'Menú de herramientas',
      group: 'Creativo',
      video: 'Descargar vídeo de TikTok',
      photo: 'Descargar fotos de TikTok',
      agentGroup: 'Agente',
      cli: 'CLI',
      skills: 'Skills',
    },
    resourcesMenu: {
      aria: 'Menú de recursos',
      inspiration: 'Comunidad de inspiración',
      docs: 'Documentación para desarrolladores',
    },
    account: {
      aria: 'Menú de cuenta',
      dashboard: 'Consola',
      profile: 'Perfil',
      security: 'Seguridad y acceso',
      wallet: 'Cartera',
      systemSettings: 'Configuración del sistema',
      signOut: 'Cerrar sesión',
    },
  },

  signOut: {
    title: 'Cerrar sesión',
    body: 'Tendrás que iniciar sesión de nuevo para acceder a tu cuenta.',
    cancel: 'Cancelar',
    confirm: 'Cerrar sesión',
    busy: 'Cerrando sesión…',
    failed: 'Error al cerrar sesión',
    done: 'Sesión cerrada',
    close: 'Cerrar',
  },

  hero: {
    titleA: 'Descubre, recrea y escala',
    titleB: 'vídeos virales de venta',
    subPre:
      'Un sistema de comercio de contenidos que lleva los puntos fuertes de tu producto a cada vídeo. Usa ',
    subHighlight: 'los Agents que ya conoces',
    subPost:
      ' para crear anuncios, vídeos de venta y reseñas en un solo lugar.',
    tabsAria: 'Modo de creación',
    tabs: { agent: 'Agent', video: 'Vídeo', image: 'Imagen' },
    inputLabel: 'Describe lo que quieres crear',
    attachAria: 'Añadir un vídeo o imagen de referencia',
    sendAria: 'Empezar en desktop',
    dlMac: 'Descargar para macOS',
    dlWin: 'Descargar para Windows',
    dlMenuAria: 'Elegir versión de descarga',
    ghStar: 'Danos una estrella en GitHub',
    dlMacArm: 'macOS (Apple Silicon)',
    dlMacIntel: 'macOS (Intel)',
    dlWinX64: 'Windows (64 bits)',
    skillsLabel: 'Skills',
    skillsAria: 'Skills',
    skills: [
      {
        icon: 'scan',
        label: 'Analizar vídeo/post',
        fill: 'Analiza por qué este vídeo se hizo viral: gancho inicial, argumentos de venta y guía de compra https://www.tiktok.com/',
      },
      {
        icon: 'video',
        label: 'Recrear vídeo',
        fill: 'Recrea este vídeo viral para @Sérum, conserva el ritmo y la estructura de planos, 15 s',
      },
      {
        icon: 'clap',
        label: 'Crear vídeo de venta',
        fill: 'Crea un vídeo de venta de 15 s para @Sérum, abre mostrando el efecto sobre la piel',
      },
      {
        icon: 'img',
        label: 'Recrear post de venta',
        fill: 'Usa este set de posts de Instagram como referencia: crea 6 imágenes de venta para @Sérum',
      },
      {
        icon: 'wand',
        label: 'Vídeo a prompt',
        fill: 'Descompón este vídeo en prompts reutilizables, plano a plano',
      },
    ],
    modes: {
      agent: {
        prompts: [
          'Analiza por qué este vídeo se hizo viral https://www.tiktok.com/@pet.diary/video/73421',
          'Usa este vídeo viral para crear 3 para @Sérum, listos para publicar, 15 s',
          'Analiza la estrategia de @SkinDiary de los últimos 30 días',
        ],
      },
      video: {
        model: 'Seedance 2.5 · 9:16 · 15s · 720p',
        prompts: [
          'Crea un vídeo de venta de 15 s para @Sérum — abre con el efecto sobre la piel',
          'Prueba de caída de auriculares Bluetooth desde lo alto, revela la oferta flash al final',
        ],
      },
      image: {
        model: 'GPT Image 2.5 · 1:1',
        prompts: [
          'Crea un set de 8 imágenes de producto para @Sérum: principal, beneficios, detalles, especificaciones, escenas',
          'Coloca esta foto de producto en una escena de tocador de mármol',
        ],
      },
    },
    reelAria: 'Ejemplos de plantillas de vídeo integradas',
    reel: [
      { title: 'Efectos de partículas 3D', tag: 'Comida de mascotas' },
      { title: 'Cambio de outfit en pasarela', tag: 'Overol' },
      { title: 'Prueba de caída y durabilidad', tag: 'Auriculares' },
      { title: 'Pitch selfie de creador', tag: 'Probióticos' },
      { title: 'Persecución de producto gigante', tag: 'Bolsa de compra' },
      { title: 'Demo de web móvil', tag: 'App de viajes' },
      { title: 'Anuncio promocional de 15 s', tag: 'Snacks' },
    ],
  },

  agentDemo: {
    h2: 'Recrea un vídeo de venta viral en un solo chat',
    lead: 'Dile qué vendes. Analiza el vídeo de referencia, escribe guiones, genera contenido y programa publicaciones.',
    windowAria: 'Demo del Agent de OmniMux Desktop',
    windowTitle: 'Recrea este vídeo viral para mis 3 éxitos',
    windowStatus: '3 vídeos generados',
    replay: 'Repetir demo',
    replayAria: 'Repetir demo',
    side: [
      'Nuevo chat',
      'Apps',
      'Canvas',
      'Biblioteca de assets',
      'Inspiración',
      'Productos',
      'Publicar',
      'Datos',
    ],
    me: 'L',
    msgPre: 'Usa este vídeo viral para crear un vídeo de 15 s para',
    msgProducts: ['@ComidaMascota', '@Overol', '@Auriculares'],
    msgPost: 'y publícalos, 15 s cada uno.',
    linkCard: 'tiktok.com/@pet.diary/video/73421 · gato jugando + unboxing de pienso',
    doneBreakdown: 'Analizado por qué vende el vídeo de referencia',
    kv: [
      { k: 'Gancho', v: 'Antes/después en los primeros 2 s' },
      { k: 'Prueba', v: 'Primer plano en uso' },
      { k: 'Cierre', v: 'Dirige clics al carrito del vídeo' },
    ],
    doneProducts: 'Analizados tus 3 productos y sus argumentos de venta',
    doneScripts: 'Escrito un guion para cada producto',
    confirmTitle: '¿Generar 3 vídeos?',
    confirmNeed: 'Necesita tu confirmación',
    confirmCost: 'Coste estimado de créditos, se cobra tras confirmar',
    confirmGo: 'Confirmar y generar',
    confirmEdit: 'Editar',
    inputPh:
      'Envía un mensaje a OmniMux — @ para referenciar productos o assets',
    resultsTitle: 'Cortes finales',
    resultsMeta: '3 vídeos · 9:16',
    generating: 'Generando…',
    vidNames: ['Comida de mascotas', 'Overol', 'Auriculares'],
    vidHooks: [
      'Gancho · contraste de concepto',
      'Gancho · cambio de outfit',
      'Gancho · primer plano',
    ],
    schedTitle: 'Publicaciones programadas',
    schedOk: 'Programado',
    schedPlatform: 'TikTok',
    schedTimes: ['Mar 19:00', 'Mié 12:30', 'Jue 19:00'],
    schedAll: 'Programar todo',
  },

  flow: {
    h2: 'De la idea de producto al vídeo de venta',
    lineAria: 'Descubrir · Analizar · Crear · Publicar · Revisar',
    steps: [
      {
        kind: 'Descubrir',
        title: 'Encuentra vídeos virales de venta en tu categoría',
        desc: 'Recopila vídeos de referencia de donde los encuentres y analiza su gancho y sus puntos de venta.',
        chips: ['Vídeos y posts virales de venta'],
        primary: 'Analizar un vídeo de referencia',
        secondary: 'Guardar inspiración con la extensión',
      },
      {
        kind: 'Analizar',
        title: 'Entiende por qué vende',
        desc: 'Descompón el gancho, los argumentos de venta y la guía de compra de un vídeo de éxito en una estructura reutilizable. También puedes analizar cuentas de la competencia.',
        chips: [
          'Desglose de guiones',
          'Cuentas de la competencia',
          'Tendencias por categoría',
        ],
        primary: 'Analizar un vídeo viral',
      },
      {
        kind: 'Crear',
        title: 'Convierte tu producto en un vídeo de venta',
        desc: 'Genera vídeos originales a partir de una foto o un enlace de producto, o recrea un éxito viral con tu propio producto.',
        primary: 'Generar vídeo',
        secondary: 'Recrear un vídeo viral',
      },
      {
        kind: 'Publicar',
        title: 'Publica en TikTok, Instagram, Facebook y YouTube',
        desc: 'Conecta tus cuentas y publica vídeos y posts de imágenes de venta al instante o de forma programada.',
        chips: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
        primary: 'Publicar vídeo',
        secondary: 'Posts de vídeo · Posts de imágenes',
      },
      {
        kind: 'Revisar',
        title: 'Convierte el feedback real en tu próximo vídeo',
        desc: 'Con las cuentas conectadas, las visualizaciones, likes y comentarios se sincronizan solos — y el Agent te dice qué crear después.',
        primary: 'Analizar cuenta',
      },
    ],

    discover: {
      title: 'Ejemplos de referencia · flujo ilustrado',
      meta: 'Últimos 7 días',
      savesPrefix: 'guardados',
      items: [
        {
          who: 'taotao.ootd',
          views: '46 mil',
          length: 'Post · 6 imágenes',
          title:
            'Un vestido de lunares, tres looks — de la oficina a la cita #ootd',
          likes: '46 mil',
          saves: '31 mil',
        },
        {
          who: 'commute10',
          views: '2,7 M',
          length: '00:28',
          title:
            'Retoque de maquillaje en el coche con una sola mano #maquillaje',
          likes: '89 mil',
          saves: '24 mil',
        },
        {
          who: 'dryskin.lab',
          views: '1,6 M',
          length: '00:34',
          title:
            'Sérum con gotero para la piel seca de temporada #pielsensible',
          likes: '52 mil',
          saves: '18 mil',
        },
      ],
    },

    breakdown: {
      tabsAria: 'Qué analizar',
      tabs: {
        video: 'Vídeo',
        account: 'Cuenta de la competencia · Ejemplo',
        category: 'Tendencias por categoría · Ejemplo',
      },
      statsViews: '15 M+ visualizaciones',
      statsLikes: '320 mil+ likes',
      why: 'Por qué vende',
      rows: [
        {
          label: 'Gancho inicial',
          time: '0–2s',
          desc: 'Resultado primero: muestra el efecto antes que el producto',
        },
        {
          label: 'Argumentos de venta',
          time: '3–9s',
          desc: 'Primer plano de la prueba, voz en off con tres puntos',
        },
        {
          label: 'Guía de compra',
          time: '10–14s',
          desc: 'Oferta por tiempo limitado, dirige clics al carrito',
        },
      ],
      reuse: 'Estructura reutilizable',
      chain: ['Resultado primero', 'Prueba en uso', 'Oferta + carrito'],
      accountName: '@SkinDiary',
      accountMeta: 'Belleza y cuidado personal',
      thumbs: ['2,1 M', '1,4 M', '860 mil', '510 mil'],
      metrics: [
        { label: 'Vídeos publicados · últimos 30 días', value: '42' },
        { label: 'Media de visualizaciones', value: '186 mil' },
        { label: 'Vídeos de venta', value: '71%' },
      ],
      insights: [
        {
          k: 'Estrategia',
          v: '7 de los 10 vídeos top abren con un gancho de "resultado primero" y luego cortan a un primer plano de prueba — es la jugada principal de esta cuenta.',
        },
        {
          k: 'Horario',
          v: 'Los vídeos publicados entre las 19 y las 21 h consiguen ~40% más visualizaciones que en otras franjas.',
        },
        {
          k: 'Oportunidad',
          v: 'Nadie en esta categoría combina aún antes/después con un ancla de precio — vale la pena probarlo primero.',
        },
      ],
      accountCta: 'Usa esta estrategia para crear 3 para @Sérum',
      catTitle: 'Belleza y cuidado personal',
      catMeta: 'Últimos 7 días · top 200 · Datos de muestra',
      bars: [
        { label: 'Antes/después', value: '38%' },
        { label: 'Unboxing y prueba', value: '24%' },
        { label: 'Primer plano de prueba', value: '19%' },
        { label: 'Pitch con voz en off', value: '12%' },
        { label: 'Giro argumental', value: '7%' },
      ],
      catInsights: [
        {
          k: 'Gancho',
          v: 'Las aperturas de antes/después están en el 38% de los vídeos top y tienen la mayor media de visualizaciones.',
        },
        {
          k: 'Duración',
          v: 'La mayoría de éxitos dura 15–25 s; más de 40 s cae en picado.',
        },
        {
          k: 'Oportunidad',
          v: 'El combo primer plano de prueba + oferta limitada aún es raro — buen punto de entrada para productos nuevos.',
        },
      ],
      catCta: 'Redacta un plan de referencia a partir de esto',
    },

    make: {
      tabsAria: 'Cómo crearlo',
      tabs: { generate: 'Generar', replicate: 'Recrear viral' },
      capProduct: 'Producto',
      capResult: 'Corte final',
      capRef: 'Referencia viral',
      capMine: 'Tu producto',
      badgeGenerated: 'Generado',
      badgeRef: '▶ TikTok · 2,1 M',
      arrowReplicateA: 'Copia la estructura',
      arrowReplicateB: 'cambia el producto',
      promptLabel: 'Prompt',
      promptGenerate:
        'Auriculares Bluetooth: prueba de caída desde altura, la cámara sigue el aterrizaje, intactos, revela una oferta flash al final — estilo voz en off con persona real.',
      intentLabel: 'Intención de reescritura',
      promptReplicate:
        'Conserva el ritmo de voz en off selfie del vídeo de referencia: apertura con el problema, demo en mano en el medio, opinión sincera al final. Recreamos el enfoque, no el metraje.',
      altProduct: 'Foto de producto de auriculares Bluetooth',
      altRef: 'Portada del vídeo de referencia viral',
      altCan: 'Foto de producto de probióticos',
    },

    publish: {
      title: 'Plan de publicación',
      meta: 'Esta semana',
      posts: [
        {
          title: 'Patatas que enganchan — compañía para maratones',
          platformLabel: 'TikTok',
          account: '@snacklab',
          time: 'Hoy 18:30',
          state: 'Publicado',
        },
        {
          title: 'Hasta los perros más quisquillosos lo devoran',
          platformLabel: 'TikTok',
          account: '@furbowl',
          time: 'Hoy 20:00',
          state: 'Programado',
        },
        {
          title: 'Un overol, siete outfits',
          platformLabel: 'Instagram',
          account: '@ootd_daily',
          time: 'Mañana 12:00',
          state: 'Listo',
        },
        {
          title: 'Un mes con probióticos — reseña honesta',
          platformLabel: 'YouTube',
          account: '@goodfinds',
          time: 'Mañana 19:00',
          state: 'Procesando',
        },
      ],
      feats: [
        {
          title: 'Envía a todas las plataformas de una vez',
          desc: 'TikTok, Instagram, Facebook y YouTube: elige cuentas, envía una vez y sigue el estado de cada cuenta por separado.',
        },
        {
          title: 'Todas las cuentas en un solo lugar',
          desc: 'Conecta todas las cuentas de plataforma en la app de escritorio y solo elige una al publicar — sin ir y venir.',
        },
        {
          title: 'Programación y reintento automático',
          desc: 'Cada tarea guarda su estado y las fallidas se pueden reintentar.',
        },
      ],
    },

    review: {
      title: 'Rendimiento de la cuenta',
      synced: 'Sincronizado · ahora mismo',
      meta: 'Últimos 7 días',
      kpis: [
        { label: 'Visualizaciones', value: '128 mil', delta: '+18%' },
        { label: 'Likes', value: '8.640', delta: '+24%' },
        { label: 'Engagement', value: '5,2%', delta: '+0,8 pt' },
      ],
      days: ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'],
      splitTitle: 'Visualizaciones por tipo de gancho',
      splitLegs: ['Ganchos antes/después 63%', 'Ganchos con voz en off 37%'],
      note: 'Las aperturas de antes/después aportaron el 63% de las visualizaciones; publicar entre las 19 y las 21 h rinde mejor.',
      cta: 'Siguiente ronda: crea 3 más con este gancho',
    },
  },

  create: {
    tag: 'Creator Agent',
    h2: 'Déjaselo al Agent, o monta el flujo tú mismo',
    lead: 'Crea contenido conversando con el Agent de OmniMux, o conecta herramientas de imagen, vídeo y voz en Canvas para construir un flujo de trabajo reutilizable.',
    msgProductCard: 'Bebida energética · imagen principal',
    msgAsk: 'Crea un set de imágenes de producto para esta bebida energética.',
    doneAnalyze: 'Producto analizado: forma, etiqueta y paleta',
    donePlan:
      'Set de 8 imágenes planificado: principal, beneficios, detalles, especificaciones, escenas',
    doneConsistent: 'Manteniendo el producto idéntico en cada imagen',
    confirmTitle: '¿Generar 8 imágenes?',
    confirmNeed: 'Necesita tu confirmación',
    paramCount: '8 imágenes',
    confirmGo: 'Confirmar y generar',
    confirmEdit: 'Editar',
    doneResult: 'Set completo — 8 imágenes añadidas al Canvas',
    tabsAria: 'Espacio de trabajo',
    tabs: {
      canvas: 'Canvas',
      assets: 'Biblioteca de assets',
      inspiration: 'Inspiración',
      products: 'Productos',
    },
    groupLabel: 'Set de producto · 8',
    setCaptions: [
      'Principal',
      'Beneficios',
      'Detalles',
      'Especificaciones',
      'Escena',
      'Escena',
      'Ambiente',
      'Póster',
    ],
    libAssets: [
      { label: 'Personaje', sub: 'Deer · 3 ángulos' },
      { label: 'Escena', sub: 'Cocina abierta' },
      { label: 'Estilo', sub: 'Neón cyber' },
      { label: 'Atrezo', sub: 'Overol' },
    ],
    libInspiration: [
      {
        label: 'Muestras de tinte de labios',
        sub: 'TikTok · 3 M visualizaciones',
      },
      { label: 'Retoque en el coche', sub: 'TikTok · 2,7 M visualizaciones' },
      { label: 'Prueba de sérum', sub: 'Instagram · 18 mil guardados' },
      { label: 'Persecución en pantalla', sub: 'Analizado' },
    ],
    libProducts: [
      {
        name: 'Bebida energética',
        sub: '0 azúcar · energizante · acabado helado',
        count: 'set de 8 imágenes',
      },
      {
        name: 'Auriculares',
        sub: 'ANC · 30 h de batería · anticaídas',
        count: '2 vídeos',
      },
      {
        name: 'Probióticos',
        sub: 'Una cucharada al día · monodosis · portátil',
        count: '1 vídeo',
      },
    ],
    capAgentTitle: 'Di lo que necesitas — el Agent hace todo el trabajo',
    capAgentDesc:
      'De imágenes de producto y vídeos a guiones: el Agent planifica cada paso y mantiene el mismo producto consistente en todo el contenido.',
    capAgentCta: 'Empezar a crear',
    capCanvasTitle: 'Canvas: flujos de trabajo creativos visuales',
    capCanvasDesc:
      'Conecta nodos de imagen, vídeo y voz en off en un flujo de trabajo — ajusta cualquier paso y vuelve a ejecutarlo.',
    capCanvasCta: 'Abrir Canvas',
    canvasAria: 'Ejemplo de flujo de trabajo en Canvas',
    nodeImage: 'Imagen',
    nodeText: 'Texto',
    nodeTextBody:
      'Luces de neón, fondo oscuro, primer plano de la lata, niebla helada y gotas',
    nodeAIImage: 'Imagen IA',
    nodeAIVideo: 'Vídeo IA',
    nodeCut: 'Edición y exportación',
    nodeDone: 'Hecho',
    nodeCutBodyA: 'Añadir subtítulos y música',
    nodeCutBodyB: 'Exportar vertical 9:16',
  },

  surfaces: {
    h2: 'Úsalo donde ya trabajas',
    lead: 'El mismo Agent en la app de escritorio, Codex, Claude Code y el navegador.',
    desktop: {
      title: 'OmniMux Desktop',
      desc: 'El espacio de trabajo completo: canvas, assets, productos y todas las herramientas — en macOS y Windows.',
      input: 'Crea un vídeo para cada uno de mis 3 sérums…',
      cta: 'Descargar Desktop',
    },
    cli: {
      title: 'Codex, Claude Code y otros agents',
      desc: 'Instala OmniMux CLI y llama a las herramientas de vídeo, imagen y análisis desde los agents que ya usas.',
      cta: 'Instalar la CLI',
    },
    ext: {
      title: 'Extensión de navegador',
      desc: '¿Viste un vídeo viral en la web? Guárdalo en Inspiración con un clic y sigue el análisis en el escritorio.',
      capSave: 'Guardar en Inspiración',
      capScan: 'Analizar',
      toast: 'Guardado en Inspiración',
      cta: 'Instalar la extensión en la app',
    },
  },

  tools: {
    h2: 'Genera vídeos de venta, imágenes y voces en off con IA',
    lead: 'Crea assets de producto, contenido multiplataforma y reutiliza vídeos existentes — todo en un mismo espacio de trabajo.',
    centerLabel: 'Creación de contenido',
    groups: [
      {
        title: 'Listings de producto',
        lead: 'Lleva las imágenes de producto al nivel de publicación',
        items: [
          { icon: 'img', label: 'Fotos sobre fondo blanco' },
          { icon: 'layers', label: 'Fotos de producto multiángulo' },
          { icon: 'grid', label: 'Sets de imágenes de producto' },
          { icon: 'box', label: 'Imágenes de página de detalle' },
          { icon: 'spark', label: 'Imágenes de escena' },
          { icon: 'wand', label: 'Reestilizar desde referencia' },
        ],
      },
      {
        title: 'Vídeo de venta',
        lead: 'Genera desde cero o sigue un éxito viral',
        items: [
          { icon: 'link', label: 'Producto a vídeo' },
          { icon: 'video', label: 'Generación de vídeo' },
          { icon: 'flame', label: 'Recreación viral' },
          { icon: 'wand', label: 'Vídeo a prompt' },
          { icon: 'grid', label: 'Guiones de storyboard' },
          { icon: 'img', label: 'Vídeo desde primer fotograma' },
        ],
      },
      {
        title: 'Posts de imágenes de venta',
        lead: 'Un set de imágenes también vende',
        items: [
          { icon: 'layers', label: 'Carruseles de imágenes' },
          { icon: 'scan', label: 'Remake de imagen de referencia' },
          { icon: 'img', label: 'Generación de imágenes' },
          { icon: 'shirt', label: 'Prueba virtual' },
        ],
      },
      {
        title: 'Voz y presentadores',
        lead: 'Voces, personajes y talento ante cámara',
        items: [
          { icon: 'user', label: 'Humanos digitales' },
          { icon: 'users', label: 'Personajes multiángulo' },
          { icon: 'mic', label: 'Voz en off y SFX' },
          { icon: 'music', label: 'Música IA' },
        ],
      },
      {
        title: 'Fit multiplataforma',
        lead: 'Un set de assets, adaptado a cada plataforma',
        items: [
          { icon: 'crop', label: 'Fit de relación de aspecto' },
          { icon: 'hash', label: 'Títulos y hashtags' },
        ],
      },
      {
        title: 'Remix de metraje',
        lead: 'Edita, divide y graba subtítulos sobre metraje existente.',
        items: [
          { icon: 'cut', label: 'Edición de vídeo' },
          { icon: 'scan', label: 'División de escenas' },
          { icon: 'subs', label: 'Subtítulos incrustados' },
        ],
      },
    ],
    modelsTitle: 'Modelos de IA top, ajustados para vender',
    modelsAria: 'Modelos disponibles',
    models: [
      { mark: 'S', name: 'Seedance 2.5', kind: 'Vídeo' },
      { mark: 'S', name: 'Seedance 2.0', kind: 'Vídeo' },
      { mark: 'G', name: 'GPT Image 2.5', kind: 'Imagen' },
      { mark: 'N', name: 'Nano Banana', kind: 'Imagen' },
      { mark: '♪', name: 'Suno', kind: 'Música' },
      { mark: 'G', name: 'Gemini', kind: 'Comprensión y guiones' },
    ],
    chipsAria: 'Capacidades del modelo',
    capabilityRows: [
      [
        { icon: 'monitor', label: '480p / 720p' },
        { icon: 'crop', label: '9:16' },
        { icon: 'crop', label: '16:9' },
        { icon: 'crop', label: '1:1' },
        { icon: 'crop', label: '3:4' },
        { icon: 'crop', label: '4:3' },
        { icon: 'clock', label: 'Salida de 4–30 s' },
      ],
      [
        { icon: 'layers', label: 'Referencias multimodales' },
        { icon: 'user', label: 'Consistencia de personaje' },
        { icon: 'img', label: 'Control del primer fotograma' },
        { icon: 'music', label: 'Audio nativo' },
        { icon: 'spark', label: 'Consistencia de producto' },
      ],
      [
        { icon: 'cut', label: 'Edición local' },
        { icon: 'scan', label: 'División de escenas' },
        { icon: 'subs', label: 'Subtítulos incrustados' },
        { icon: 'mic', label: 'Extracción de audio' },
        { icon: 'layers', label: 'Post de imágenes a vídeo' },
      ],
    ],
  },

  workspace: {
    h2: 'Un espacio de trabajo, todos los assets a mano',
    lead: 'Productos, assets, inspiración e historial de publicación viven en una sola app de escritorio — siempre listos, sin buscar.',
    list: [
      {
        name: 'Biblioteca de productos',
        desc: 'Argumentos de venta, audiencias e imágenes',
      },
      {
        name: 'Biblioteca de assets',
        desc: 'Personajes, escenas, estilos y prompts',
      },
      { name: 'Inspiración', desc: 'Vídeos virales guardados y análisis' },
      {
        name: 'Centro de publicación',
        desc: 'Borradores, programación e historial',
      },
    ],
    note: 'Funciona en macOS y Windows.',
    tableTitle: 'Biblioteca de productos',
    addProduct: 'Añadir producto',
    cols: ['Producto', 'Argumentos de venta', 'Contenido este mes'],
    rows: [
      {
        name: 'Bebida energética',
        category: 'Alimentación y bebidas',
        points: '3',
      },
      { name: 'Auriculares', category: 'Electrónica', points: '4' },
      { name: 'Overol', category: 'Moda', points: '3' },
      { name: 'Probióticos', category: 'Suplementos', points: '2' },
    ],
  },

  proof: {
    h2: 'Hecho para vendedores transfronterizos',
    lead: 'Encuentra éxitos, analízalos, crea y publica — todo dentro de una app de escritorio.',
    facts: [
      { value: '17', label: 'skills integradas' },
      { value: '7', label: 'plantillas de vídeo' },
      { value: '4', label: 'plataformas de publicación' },
    ],
    platformsAria: 'Plataformas de publicación compatibles',
    platforms: ['TikTok', 'Instagram', 'Facebook', 'YouTube'],
    badges: [
      'Confirma parámetros y créditos antes de generar',
      'Créditos reembolsados automáticamente en tareas fallidas',
    ],
  },

  faq: {
    h2: 'Preguntas frecuentes',
    lead: 'Lo que más preguntan los vendedores de e-commerce',
    items: [
      {
        q: '¿OmniMux es de código abierto?',
        a: 'Sí. El cliente de escritorio, los plugins y el código del gateway se publican por versión en github.com/omnimux-ai/omnimux-ai bajo AGPL-3.0. El servicio alojado mantiene fuera de alcance las credenciales de canales, el libro de precios, la operación y los datos de usuarios.',
      },
      {
        q: '¿Qué puede hacer el Agent de OmniMux?',
        a: 'El Agent de OmniMux analiza vídeos de referencia, escribe guiones, genera vídeos e imágenes y ayuda a publicar en las plataformas que hayas conectado. Describe tu producto y tu objetivo en lenguaje cotidiano y elegirá la herramienta adecuada para cada paso.',
      },
      {
        q: '¿El Agent generará o publicará sin mi aprobación?',
        a: 'No. Antes de generar vídeo o imágenes — y antes de publicar — el Agent muestra los parámetros y el coste estimado de créditos y espera tu confirmación. Puedes ajustar los parámetros antes. Si una tarea falla por motivos del sistema, los créditos cobrados se reembolsan.',
      },
      {
        q: '¿Cómo hago un vídeo de venta con IA?',
        a: 'Pega un enlace de producto o elige una referencia viral, genera un vídeo original o recreado y publícalo en las plataformas que hayas conectado. Sin grabar ni editar.',
      },
      {
        q: '¿Los vídeos generados se pueden publicar directamente?',
        a: 'Sí. Conecta tu cuenta una vez en la app de escritorio y publica directamente, sin subidas manuales. Puedes añadir la declaración de contenido IA que exige cada plataforma al publicar.',
      },
      {
        q: '¿Los vídeos hechos con IA pueden hacer que me baneen la cuenta?',
        a: 'OmniMux no puede garantizar protección contra baneos: la seguridad de la cuenta depende de tu contenido y tu comportamiento. Al publicar puedes añadir una declaración de contenido generado por IA al vídeo; respeta las políticas de cada plataforma.',
      },
      {
        q: '¿Puede convertir imágenes de producto en vídeo?',
        a: 'Sí. Sube una imagen de producto o pega un enlace de producto y la IA genera un vídeo de presentación listo para publicar, sin grabación.',
      },
      {
        q: '¿Cómo recreo el vídeo viral de un competidor?',
        a: 'La IA estudia su forma de crear — ritmo, encuadre, tono — y genera un vídeo nuevo con tu producto y metraje original. Recreas el enfoque, no el clip original.',
      },
      {
        q: '¿Puedo usar los vídeos generados con fines comerciales?',
        a: 'El uso comercial se rige por los Términos de Servicio. Para retratos, imágenes de producto y vídeos de referencia que subas, confirmas que tienes derechos de uso y cumples las reglas de cada plataforma.',
      },
    ],
  },

  finalCta: {
    h2: 'Tu próximo vídeo viral — déjaselo a OmniMux',
    lead: 'Analiza éxitos, genera vídeos, publica y revisa — un solo Agent ejecuta todo el ciclo.',
    dlMac: 'Descarga gratis para macOS',
    dlWin: 'Descargar para Windows',
    note: 'macOS 12+ (universal Apple silicon e Intel) · Windows 10 / 11 x64',
  },

  download: {
    title: 'Continúa en OmniMux Desktop',
    desc: 'Descarga, instala e inicia sesión para empezar a crear.',
    close: 'Cerrar',
    macTitle: 'Descargar para macOS (Universal)',
    macSub: 'macOS 12+ · Apple silicon e Intel',
    winTitle: 'Descargar para Windows (instalador x64)',
    winSub: 'Windows 10 / 11 x64',
    recommended: 'Recomendado',
    notePre: 'Si Windows dice "Windows protected your PC": haz clic en',
    noteMore: 'Más información',
    noteRun: 'Ejecutar de todos modos',
    notePost: '. Descarga solo desde dl.omnimux.ai.',
  },

  footer: {
    brand: 'Desktop',
    tagline: 'Descubre, recrea y escala vídeos virales de venta.',
    columns: [
      {
        title: 'Explorar',
        links: [
          { label: 'Precios', href: '/pricing' },
          { label: 'Inspiración', href: '/inspiration' },
          { label: 'Descargar Desktop', href: '#download' },
        ],
      },
      {
        title: 'Herramientas',
        links: [
          {
            label: 'Descargador de vídeos de TikTok',
            href: '/tools/tiktok-video-downloader',
          },
          {
            label: 'Descargador de fotos de TikTok',
            href: '/tools/tiktok-photo-downloader',
          },
          { label: 'Producto a vídeo', href: '#tools' },
          { label: 'Recreación viral', href: '#tools' },
          { label: 'Vídeo a prompt', href: '#tools' },
          { label: 'Sets de imágenes de producto', href: '#tools' },
          { label: 'Imágenes de página de detalle', href: '#tools' },
          { label: 'Carruseles de imágenes', href: '#tools' },
          { label: 'Humanos digitales', href: '#tools' },
          { label: 'Subtítulos incrustados', href: '#tools' },
        ],
      },
      {
        title: 'Modelos',
        links: [
          { label: 'Seedance 2.5', href: '#models' },
          { label: 'Seedance 2.0', href: '#models' },
          { label: 'GPT Image 2.5', href: '#models' },
          { label: 'Nano Banana', href: '#models' },
          { label: 'Suno', href: '#models' },
        ],
      },
      {
        title: 'Recursos',
        links: [
          {
            label: 'Documentación',
            href: 'https://docs.omnimux.ai',
            external: true,
          },
          {
            label: 'Estado',
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
        title: 'Acerca de',
        links: [
          {
            label: 'Contacto',
            href: 'mailto:hello@omnimux.ai',
            external: true,
          },
          { label: 'Política de privacidad', href: '/privacy' },
          { label: 'Términos de servicio', href: '/terms' },
        ],
      },
    ],
    powered: 'Powered by New API',
    source: 'Código fuente',
  },

  toolPage: {
    crumbAria: 'Ruta de navegación',
    crumbTools: 'Herramientas',
    kindAria: 'Tipo de contenido',
    kindVideo: 'Vídeo',
    kindPhoto: 'Fotos',
    inputLabel: 'Enlace de TikTok',
    placeholderVideo: 'Pega un enlace de vídeo de TikTok o el texto compartido',
    placeholderPhoto:
      'Pega un enlace de publicación de fotos de TikTok o el texto compartido',
    paste: 'Pegar',
    parse: 'Analizar',
    parsing: 'Analizando',
    verifying: 'Verificando',
    quotaAnon: 'Gratis hoy {remaining}/{limit}',
    quotaUser: 'Hoy {remaining}/{limit}',
    turnstileAria: 'Verificación de Cloudflare',
    unavailable: 'No disponible temporalmente',
    downloadMp4: 'Descargar MP4',
    recreateMine: 'Recrear con mi personaje',
    breakdown: 'Desglosar la estructura viral',
    photoCount: '{count} fotos',
    imageAlt: 'Foto {n}',
    downloadImage: 'Descargar foto {n}',
    downloadZip: 'Descargar todo en ZIP',
    zipLocked: 'Inicia sesión para usarlo',
    recreatePhotos: 'Recrear este set de fotos',
    ctaTitle: 'Guarda la referencia y recréala con tu personaje',
    ctaBody:
      'El mismo ritmo y los mismos planos, con tu propio personaje de IA y tu producto.',
    ctaStart: 'Empezar a recrear',
    ctaRef: 'Referencia',
    ctaMine: 'Recreado',
    relatedAria: 'Herramientas relacionadas',
    errors: {
      invalidLink: 'Pega un enlace de TikTok',
      wrongKindPhoto: 'Este es un enlace de fotos',
      toPhoto: 'Ir al descargador de fotos',
      wrongKindVideo: 'Este es un enlace de vídeo',
      toVideo: 'Ir al descargador de vídeos',
      unavailable:
        'Esta publicación es privada, se eliminó o no está disponible',
      serviceUnavailable: 'El análisis aún no está abierto en este sitio',
      upstreamFailed: 'TikTok no respondió, inténtalo de nuevo en un momento',
      quotaAnon:
        'Se agotaron los análisis gratis de hoy — inicia sesión para continuar',
      quotaUser: 'Se agotaron los análisis de hoy — vuelve mañana',
      busy: 'La capacidad gratuita de hoy está llena — inicia sesión para continuar',
      rateLimited: 'Demasiado rápido, inténtalo en un momento',
      turnstileFailed: 'La verificación falló, inténtalo de nuevo',
      turnstileUnavailable:
        'La verificación no está disponible por ahora, inténtalo más tarde',
      generic: 'No se pudo analizar este enlace, inténtalo más tarde',
    },
  },
}
