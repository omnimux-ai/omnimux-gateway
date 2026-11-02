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
import type { ComponentType } from 'react'
import {
  SiBilibili,
  SiInstagram,
  SiKuaishou,
  SiReddit,
  SiSinaweibo,
  SiThreads,
  SiTiktok,
  SiX,
  SiXiaohongshu,
  SiYoutube,
} from 'react-icons/si'

type BrandIconProps = {
  size?: number
  className?: string
  title?: string
}

function wrapSi(
  Icon: ComponentType<{ size?: number | string; className?: string; title?: string }>,
  defaultTitle: string
) {
  return function SocialBrandIcon({ size = 20, className, title }: BrandIconProps) {
    return (
      <Icon
        size={size}
        className={className}
        title={title || defaultTitle}
        aria-label={title || defaultTitle}
      />
    )
  }
}

/** Vendor icon keys for getLobeIcon CUSTOM_ICONS (social data platforms). */
export const IconTikTok = wrapSi(SiTiktok, 'TikTok')
/** Douyin shares the same mark family as TikTok for UI consistency. */
export const IconDouyin = wrapSi(SiTiktok, 'Douyin')
export const IconInstagram = wrapSi(SiInstagram, 'Instagram')
export const IconYouTube = wrapSi(SiYoutube, 'YouTube')
export const IconX = wrapSi(SiX, 'X')
export const IconBilibili = wrapSi(SiBilibili, 'Bilibili')
export const IconXiaohongshu = wrapSi(SiXiaohongshu, 'Xiaohongshu')
export const IconKuaishou = wrapSi(SiKuaishou, 'Kuaishou')
export const IconWeibo = wrapSi(SiSinaweibo, 'Weibo')
export const IconThreads = wrapSi(SiThreads, 'Threads')
export const IconReddit = wrapSi(SiReddit, 'Reddit')
