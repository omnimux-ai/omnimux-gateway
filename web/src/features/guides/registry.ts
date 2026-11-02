import { CHARACTER_MARKDOWN, VIDEO_MARKDOWN } from './content'

export interface GuideMedia {
  id: string
  fileName: string
  url: string
  width: number
  height: number
  alt: string
  caption: string
  shape?: 'tall' | 'control'
}

export interface GuideArticle {
  slug: string
  path: string
  title: string
  summary: string
  intro: string
  category: string
  headingLead: string
  headingAccent: string
  reviewedAt: string
  publishedAt: string
  bodyMarkdown: string
  media: readonly GuideMedia[]
  coverId: string
  provenance: string
  next: {
    id: string
    tocLabel: string
    eyebrow: string
    title: string
    description: string
    relatedSlug: string
  }
}

function media(
  id: string,
  width: number,
  height: number,
  alt: string,
  caption: string,
  shape?: GuideMedia['shape']
): GuideMedia {
  const fileName = `${id}.webp`
  return {
    id,
    fileName,
    url: `/static/guides/${fileName}`,
    width,
    height,
    alt,
    caption,
    ...(shape ? { shape } : {}),
  }
}

/** Build-only article source. Client entrypoints must not import this registry. */
export const GUIDE_REGISTRY: readonly GuideArticle[] = [
  {
    slug: 'create-ai-character',
    path: '/guides/create-ai-character',
    title: 'Create your AI character',
    summary:
      'Choose traits, generate a character image, and review it before using it in a video.',
    intro:
      'Turn a character idea into an image you can review, save, and use as material for a reference video.',
    category: 'Character guide',
    headingLead: 'Create your',
    headingAccent: 'AI character.',
    reviewedAt: '2026-10-20',
    publishedAt: '2026-10-20',
    bodyMarkdown: CHARACTER_MARKDOWN,
    coverId: 'character-result',
    provenance:
      'Illustrated walkthrough using interface examples and a separately generated result. Check the current studio controls and quote before submitting.',
    media: [
      media(
        'character-form',
        960,
        1690,
        'The character creation area with the character form visible',
        'Earlier interface example. In the current studio, choose from the available character type and trait controls.',
        'tall'
      ),
      media(
        'character-traits',
        960,
        1125,
        'Character trait choices in the studio, with selected options visible',
        'Earlier interface example. Review the current type and trait selections before generating.',
        'tall'
      ),
      media(
        'character-generate-control-crop',
        380,
        140,
        'The character generation control in the prepared image workflow',
        'Cropped interface example. Review the current quote in the studio before submitting.',
        'control'
      ),
      media(
        'character-result',
        1440,
        818,
        'The character example shown in a portrait and full-body image',
        'Generated character example. A portrait and a full-body image are shown here; they are not a multi-view reference sheet.'
      ),
    ],
    next: {
      id: 'c-next',
      tocLabel: 'Next: make a video',
      eyebrow: 'Next step',
      title: 'Bring your character into motion.',
      description:
        'Use your reviewed character image together with a reference video in the next walkthrough.',
      relatedSlug: 'recreate-video-with-your-character',
    },
  },
  {
    slug: 'recreate-video-with-your-character',
    path: '/guides/recreate-video-with-your-character',
    title: 'Recreate a reference video with your character',
    summary:
      'Prepare the character and reference, check the inputs, and review the generated motion.',
    intro:
      'Prepare a character and a reference clip, check the inputs, and review the generated motion before downloading.',
    category: 'Reference video guide',
    headingLead: 'Recreate a reference video',
    headingAccent: 'with your character.',
    reviewedAt: '2026-10-20',
    publishedAt: '2026-10-20',
    bodyMarkdown: VIDEO_MARKDOWN,
    coverId: 'video-result',
    provenance:
      'This character replacement example combines interface screenshots with a separately generated result. Check the current settings, wallet entitlements, and task record.',
    media: [
      media(
        'video-inputs',
        1270,
        760,
        'The character and reference video inputs used for the illustrated example',
        'The two inputs have different roles: character appearance and reference motion.'
      ),
      media(
        'video-mode',
        950,
        960,
        'The studio video area selected for character replacement',
        'Interface example showing Object swap. Check the generated scene; background details can change.',
        'tall'
      ),
      media(
        'video-reference',
        1064,
        530,
        'The selected reference video in the studio input area',
        'Check the selected clip itself, including its framing and action.'
      ),
      media(
        'video-ready',
        970,
        1268,
        'The chosen character and reference video prepared in the input area',
        'Review the character and reference together before submitting.',
        'tall'
      ),
      media(
        'video-generate',
        955,
        180,
        'The generation action after preparing the character and reference',
        'Earlier cropped interface example. Check current settings and your wallet before submitting.'
      ),
      media(
        'video-result',
        1270,
        760,
        'A still from the separately generated character replacement video',
        'A still from the generated example. Review the complete video to judge motion quality.'
      ),
    ],
    next: {
      id: 'v-next',
      tocLabel: 'Keep creating',
      eyebrow: 'Keep creating',
      title: 'Start with a character you have checked.',
      description:
        'Return to character creation when the source image needs a clearer look before the next video.',
      relatedSlug: 'create-ai-character',
    },
  },
]

export const GUIDE_HUB = {
  path: '/guides',
  title: 'OmniMux Guides | AI Characters and Reference Videos',
  summary:
    'Start with a character, then bring it into a reference video. Two practical walkthroughs show you what to prepare, where to begin, and what to check.',
} as const
