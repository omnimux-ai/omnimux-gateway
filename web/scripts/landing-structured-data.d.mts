export interface LandingBrand {
  brandName: string
  seoTitle: string
  brandDescription: string
  siteUrl: string
  /** The homepage `<CnFaq />` items; omitted or empty means no FAQPage. */
  faqItems?: ReadonlyArray<{ q: string; a: string }>
}

export function landingStructuredData(
  brand: LandingBrand
): Record<string, unknown>

export function serializeStructuredData(graph: Record<string, unknown>): string
