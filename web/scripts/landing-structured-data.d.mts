export interface LandingBrand {
  brandName: string
  seoTitle: string
  brandDescription: string
  siteUrl: string
}

export function landingStructuredData(
  brand: LandingBrand
): Record<string, unknown>

export function serializeStructuredData(graph: Record<string, unknown>): string
