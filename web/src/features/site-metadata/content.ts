// fork: the homepage is the desktop landing page (2026-10-27), so the public
// description leads with the desktop workflow. The character studio keeps its
// own description below for /ai-influencer-studio and the About page.
export const HOMEPAGE_DESCRIPTION =
  'Your local AI content commerce team. Break down viral product videos and remake them with your product and AI character. No subscription; trial packs are paid.'

export const HOMEPAGE_STEPS = [
  'Break a reference video into its hook, selling points and buying guide.',
  'Generate a new video from a product image or recreate the reference.',
  'Review the result, then publish it or schedule it from the desktop app.',
] as const

export const PRODUCT_DESCRIPTION =
  'OmniMux helps you create AI characters, build multi-view reference sheets, and generate character videos from reference clips.'

export const ABOUT_TITLE = 'About OmniMux'
export const ABOUT_TUTORIAL_TITLE = 'How to create a character video'
export const ABOUT_STEPS = [
  'Create a character from a description or feature choices in the workspace.',
  'Generate a multi-view reference sheet from your own successfully generated character image.',
  'Choose or upload a reference video, generate your character video, then review and download the result.',
] as const
export const ABOUT_RIGHTS =
  'Use only images, videos, likenesses, and voices you are authorized to use. Generated results may vary.'
// fork: crawler-facing and visible About billing sentence (seo-copy).
// There is no subscription and trial packs are paid. The visible /about
// body uses the same SEARCH_PRICING sentence as the prerendered pages.
export const SEARCH_PRICING =
  'Top up credits and pay per generation. No subscription; trial packs are paid. Check current prices before you generate.'
export const ABOUT_PRICING = SEARCH_PRICING
