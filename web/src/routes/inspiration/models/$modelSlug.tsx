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
import { createFileRoute } from '@tanstack/react-router'
import z from 'zod'

import { ModelPromptsPage } from '@/features/inspiration-community'

/**
 * `?quarantine=1` swaps the listing to the rows the catalog holds back because
 * their poster could not be fetched. It exists so the two datasets can be
 * compared side by side; a normal visit never requests or shows them.
 *
 * The router parses search values as JSON, so `1` / `true` arrive as a number
 * and a boolean while `"1"` / `"true"` stay strings — all four spellings mean
 * the same thing, and anything else (`0`, `false`, junk) leaves the mode off.
 */
const quarantineFlag = z
  .union([
    z.literal('1'),
    z.literal('true'),
    z.literal(1),
    z.literal(true),
  ])
  .optional()
  .catch(undefined)

const modelPromptsSearchSchema = z.object({
  quarantine: quarantineFlag,
})

export const Route = createFileRoute('/inspiration/models/$modelSlug')({
  validateSearch: modelPromptsSearchSchema,
  component: ModelPromptsRoute,
})

function ModelPromptsRoute() {
  const { modelSlug } = Route.useParams()
  const { quarantine } = Route.useSearch()

  return (
    <ModelPromptsPage
      modelSlug={modelSlug}
      includeQuarantined={quarantine !== undefined}
    />
  )
}
