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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CONTENT_TYPE_LABEL_KEYS,
  DURATION_LABEL_KEYS,
  PUBLISHED_LABEL_KEYS,
  REGION_CODES,
  SORT_LABEL_KEYS,
  TRAFFIC_LABEL_KEYS,
  VIEWS_LABEL_KEYS,
  regionLabelKey,
} from '../constants'
import { FILTER_ALL } from '../lib/filter'
import type {
  TikTokContentType,
  TikTokDurationBucket,
  TikTokFilters,
  TikTokPublishedBucket,
  TikTokSort,
  TikTokTrafficType,
  TikTokViewsBucket,
} from '../types'
import { FilterDropdown, type FilterOption } from './filter-bar'

export interface TikTokAdFilterBarProps {
  filters: TikTokFilters
  sort: TikTokSort
  onFiltersChange: (filters: TikTokFilters) => void
  onSortChange: (sort: TikTokSort) => void
}

/**
 * `FilterDropdown` speaks in arrays; these dimensions are single-choice, so the
 * one selected value is the array's only entry and `[]` means "no constraint".
 */
function single(values: string[]): string | null {
  return values[0] ?? null
}

/**
 * The seven controls of the ad library: region, content type, clip length,
 * views, traffic, publish window and sort.
 *
 * Each dropdown is single-choice because that is what the service takes: one
 * `country`, one `type`, one `traffic_type`, one range per numeric dimension.
 * A multi-select would have to either drop the extra choices or widen them into
 * one bounding range and answer with rows nobody selected — so the vocabulary
 * here is the contract's, not a preference.
 *
 * The industry dropdown of the reference layout is gone: two thirds of the
 * corpus carries no usable industry label and the three vocabularies in
 * `category` (content type, English topic, Chinese TikTok Shop category) do not
 * answer one question.
 */
export function TikTokAdFilterBar(props: TikTokAdFilterBarProps) {
  const { t } = useTranslation()
  const filters = props.filters

  const countryOptions = useMemo<FilterOption[]>(
    () =>
      REGION_CODES.map((code) => ({
        value: code,
        label: `${code} · ${t(regionLabelKey(code))}`,
      })),
    [t]
  )

  const contentTypeOptions = useMemo<FilterOption[]>(
    () => [
      { value: FILTER_ALL, label: t('All') },
      { value: 'video', label: t(CONTENT_TYPE_LABEL_KEYS.video) },
      { value: 'image', label: t(CONTENT_TYPE_LABEL_KEYS.image) },
    ],
    [t]
  )

  const durationOptions = useMemo<FilterOption[]>(
    () =>
      (Object.keys(DURATION_LABEL_KEYS) as TikTokDurationBucket[]).map(
        (value) => ({ value, label: t(DURATION_LABEL_KEYS[value]) })
      ),
    [t]
  )

  const viewsOptions = useMemo<FilterOption[]>(
    () =>
      (Object.keys(VIEWS_LABEL_KEYS) as TikTokViewsBucket[]).map((value) => ({
        value,
        label: t(VIEWS_LABEL_KEYS[value]),
      })),
    [t]
  )

  const trafficOptions = useMemo<FilterOption[]>(
    () =>
      (Object.keys(TRAFFIC_LABEL_KEYS) as TikTokTrafficType[]).map((value) => ({
        value,
        label: t(TRAFFIC_LABEL_KEYS[value]),
      })),
    [t]
  )

  const publishedOptions = useMemo<FilterOption[]>(
    () =>
      (Object.keys(PUBLISHED_LABEL_KEYS) as TikTokPublishedBucket[]).map(
        (value) => ({ value, label: t(PUBLISHED_LABEL_KEYS[value]) })
      ),
    [t]
  )

  const sortOptions = useMemo<FilterOption[]>(
    () =>
      (Object.keys(SORT_LABEL_KEYS) as TikTokSort[]).map((value) => ({
        value,
        label: t(SORT_LABEL_KEYS[value]),
      })),
    [t]
  )

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <FilterDropdown
        label={t('Region')}
        options={countryOptions}
        selected={filters.country ? [filters.country] : []}
        multiple={false}
        onChange={(values) =>
          props.onFiltersChange({ ...filters, country: single(values) })
        }
      />
      <FilterDropdown
        label={t('Content type')}
        options={contentTypeOptions}
        selected={[filters.contentType ?? FILTER_ALL]}
        multiple={false}
        onChange={(values) => {
          const next = single(values)
          props.onFiltersChange({
            ...filters,
            contentType:
              next && next !== FILTER_ALL ? (next as TikTokContentType) : null,
          })
        }}
      />
      <FilterDropdown
        label={t('Clip length')}
        options={durationOptions}
        selected={filters.duration ? [filters.duration] : []}
        multiple={false}
        onChange={(values) =>
          props.onFiltersChange({
            ...filters,
            duration: single(values) as TikTokDurationBucket | null,
          })
        }
      />
      <FilterDropdown
        label={t('Views')}
        options={viewsOptions}
        selected={filters.views ? [filters.views] : []}
        multiple={false}
        onChange={(values) =>
          props.onFiltersChange({
            ...filters,
            views: single(values) as TikTokViewsBucket | null,
          })
        }
      />
      <FilterDropdown
        label={t('Traffic')}
        options={trafficOptions}
        selected={filters.trafficType ? [filters.trafficType] : []}
        multiple={false}
        onChange={(values) =>
          props.onFiltersChange({
            ...filters,
            trafficType: single(values) as TikTokTrafficType | null,
          })
        }
      />
      <FilterDropdown
        label={t('Published')}
        options={publishedOptions}
        selected={filters.published ? [filters.published] : []}
        multiple={false}
        onChange={(values) =>
          props.onFiltersChange({
            ...filters,
            published: single(values) as TikTokPublishedBucket | null,
          })
        }
      />
      <FilterDropdown
        label={t('Sort')}
        options={sortOptions}
        selected={[props.sort]}
        onChange={(values) =>
          props.onSortChange((single(values) as TikTokSort | null) ?? 'views')
        }
        multiple={false}
        className='ml-auto'
      />
    </div>
  )
}
