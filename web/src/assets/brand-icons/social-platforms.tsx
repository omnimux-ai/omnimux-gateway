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

type SocialBrandIconComponent = ComponentType<{
  size?: number | string
  className?: string
  title?: string
}>

function renderSocialBrandIcon(
  Icon: SocialBrandIconComponent,
  defaultTitle: string,
  { size = 20, className, title }: BrandIconProps
) {
  return (
    <Icon
      size={size}
      className={className}
      title={title || defaultTitle}
      aria-label={title || defaultTitle}
    />
  )
}

/** Vendor icon keys for getLobeIcon CUSTOM_ICONS (social data platforms). */
export function IconTikTok(props: BrandIconProps) {
  return renderSocialBrandIcon(SiTiktok, 'TikTok', props)
}
/** Douyin shares the same mark family as TikTok for UI consistency. */
export function IconDouyin(props: BrandIconProps) {
  return renderSocialBrandIcon(SiTiktok, 'Douyin', props)
}
export function IconInstagram(props: BrandIconProps) {
  return renderSocialBrandIcon(SiInstagram, 'Instagram', props)
}
export function IconYouTube(props: BrandIconProps) {
  return renderSocialBrandIcon(SiYoutube, 'YouTube', props)
}
export function IconX(props: BrandIconProps) {
  return renderSocialBrandIcon(SiX, 'X', props)
}
export function IconBilibili(props: BrandIconProps) {
  return renderSocialBrandIcon(SiBilibili, 'Bilibili', props)
}
export function IconXiaohongshu(props: BrandIconProps) {
  return renderSocialBrandIcon(SiXiaohongshu, 'Xiaohongshu', props)
}
export function IconKuaishou(props: BrandIconProps) {
  return renderSocialBrandIcon(SiKuaishou, 'Kuaishou', props)
}
export function IconWeibo(props: BrandIconProps) {
  return renderSocialBrandIcon(SiSinaweibo, 'Weibo', props)
}
export function IconThreads(props: BrandIconProps) {
  return renderSocialBrandIcon(SiThreads, 'Threads', props)
}
export function IconReddit(props: BrandIconProps) {
  return renderSocialBrandIcon(SiReddit, 'Reddit', props)
}
