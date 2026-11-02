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
along with this program. If not, see <http://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

/**
 * Catalog-id normalisation shared by the playground's declared tables.
 *
 * The catalog spells the same model two ways — `seedance-2.5` and
 * `seedance-2-5` are both in the wild, while `wan-3.0` only ever keeps its dot.
 * Trying the id as written before the hyphenated variant resolves both without
 * rewriting one into the other.
 */
function idCandidates(modelId: string): string[] {
  const base = modelId.trim().toLowerCase()
  const hyphenated = base.replaceAll('.', '-')
  return base === hyphenated ? [base] : [base, hyphenated]
}

/** First declared entry matching any spelling of `modelId`. */
export function resolveDeclared<T>(
  modelId: string,
  table: Record<string, T>
): T | undefined {
  for (const candidate of idCandidates(modelId)) {
    const found = table[candidate]
    if (found !== undefined) return found
  }
  return undefined
}

/** True when any spelling of `modelId` is a member of `table`. */
export function isDeclared(modelId: string, table: Set<string>): boolean {
  return idCandidates(modelId).some((candidate) => table.has(candidate))
}
