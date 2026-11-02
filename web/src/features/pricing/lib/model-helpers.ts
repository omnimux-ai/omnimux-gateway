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
import { EXCLUDED_GROUPS, FILTER_ALL, QUOTA_TYPE_VALUES } from '../constants'
import type { PricingModel } from '../types'

// ----------------------------------------------------------------------------
// Model Helper Utilities
// ----------------------------------------------------------------------------

/**
 * Get available groups for a model
 */
export function getAvailableGroups(
  model: PricingModel,
  usableGroup: Record<string, string>
): string[] {
  const modelEnableGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []

  return Object.keys(usableGroup)
    .filter((g) => !EXCLUDED_GROUPS.includes(g))
    .filter((g) => modelEnableGroups.includes(g))
}

/**
 * Read a configured group ratio while preserving valid zero ratios.
 */
export function getConfiguredGroupRatio(
  groupRatio: Record<string, number>,
  group: string
): number {
  const ratio = groupRatio[group]
  return typeof ratio === 'number' && Number.isFinite(ratio) ? ratio : 1
}

/**
 * Resolve the group ratio used by model square summary prices.
 *
 * When no specific group is selected, the model square shows the best price
 * available to the viewer. When a group filter is active, it shows that
 * group's price instead.
 */
export function getDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  const modelEnableGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []
  const groupRatio = model.group_ratio || {}

  if (
    selectedGroup &&
    selectedGroup !== FILTER_ALL &&
    modelEnableGroups.includes(selectedGroup)
  ) {
    return getConfiguredGroupRatio(groupRatio, selectedGroup)
  }

  if (modelEnableGroups.length === 0) {
    return 1
  }

  let minRatio = Number.POSITIVE_INFINITY

  for (const group of modelEnableGroups) {
    const ratio = groupRatio[group]
    if (
      typeof ratio === 'number' &&
      Number.isFinite(ratio) &&
      ratio < minRatio
    ) {
      minRatio = ratio
    }
  }

  return minRatio === Number.POSITIVE_INFINITY ? 1 : minRatio
}

/**
 * Replace model placeholder in endpoint path
 */
export function replaceModelInPath(path: string, modelName: string): string {
  return path.replaceAll('{model}', modelName)
}

/**
 * Check if model is token-based pricing
 */
export function isTokenBasedModel(model: PricingModel): boolean {
  return model.quota_type === QUOTA_TYPE_VALUES.TOKEN
}

/**
 * Format a raw group identifier into its concise/short form for display.
 * If the group name starts with the model name prefix (e.g. `${modelName}-economy`),
 * strips the prefix to return the concise tier name (e.g. `economy`).
 * Otherwise returns the raw group name (e.g. `default`, `vip`).
 */
export function formatGroupShortName(group: string, modelName?: string): string {
  if (!group) return ''
  if (modelName) {
    const prefixHyphen = `${modelName.toLowerCase()}-`
    const prefixUnderscore = `${modelName.toLowerCase()}_`
    const lowerGroup = group.toLowerCase()
    if (lowerGroup.startsWith(prefixHyphen) && group.length > prefixHyphen.length) {
      return group.slice(prefixHyphen.length)
    }
    if (lowerGroup.startsWith(prefixUnderscore) && group.length > prefixUnderscore.length) {
      return group.slice(prefixUnderscore.length)
    }
  }
  return group
}

/**
 * Pick the group with the lowest ratio among available groups for a model.
 * Defaults to 'default' if no groups are available.
 */
export function getCheapestAvailableGroup(
  model: PricingModel,
  usableGroup: Record<string, string>,
  groupRatio: Record<string, number>
): string {
  const groups = getAvailableGroups(model, usableGroup)
  if (groups.length === 0) return 'default'
  if (groups.length === 1) return groups[0]

  let bestGroup = groups[0]
  let minRatio = getConfiguredGroupRatio(groupRatio, bestGroup)

  for (let i = 1; i < groups.length; i++) {
    const g = groups[i]
    const r = getConfiguredGroupRatio(groupRatio, g)
    if (r < minRatio || (r === minRatio && g.localeCompare(bestGroup) < 0)) {
      minRatio = r
      bestGroup = g
    }
  }
  return bestGroup
}

