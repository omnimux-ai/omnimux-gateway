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
import appdemoPoster from '../../assets/cn/posters/appdemo.jpg'
import chasingPoster from '../../assets/cn/posters/chasing.jpg'
import drinkPoster from '../../assets/cn/posters/drink_ad_right.jpg'
import durabilityPoster from '../../assets/cn/posters/durability.jpg'
import redPoster from '../../assets/cn/posters/red.jpg'
import selfiePoster from '../../assets/cn/posters/selfie.jpg'
import serumPoster from '../../assets/cn/posters/serum_main.jpg'
import spotlightPoster from '../../assets/cn/posters/spotlight.jpg'
import tryonPoster from '../../assets/cn/posters/tryon.jpg'
import ugcPoster from '../../assets/cn/posters/ugc_demo_left.jpg'
import vfxPoster from '../../assets/cn/posters/vfx.jpg'
import kitchenB from '../../assets/cn/land/kitchen-b.webp'
import kitchen from '../../assets/cn/land/kitchen.webp'
import pizza from '../../assets/cn/land/pizza.webp'
import polkaB from '../../assets/cn/land/polka-b.webp'
import polka from '../../assets/cn/land/polka.webp'
import redB from '../../assets/cn/land/red-b.webp'
import drinkAd from '../../assets/cn/land/drink_ad_right.mp4'
import red from '../../assets/cn/land/red.mp4'
import serumMain from '../../assets/cn/land/serum_main.mp4'
import ugcDemo from '../../assets/cn/land/ugc_demo_left.mp4'
import canSq from '../../assets/cn/set/can-sq.jpg'
import can from '../../assets/cn/set/can.jpg'
import d1 from '../../assets/cn/set/d1.jpg'
import d2 from '../../assets/cn/set/d2.jpg'
import d3 from '../../assets/cn/set/d3.jpg'
import d4 from '../../assets/cn/set/d4.jpg'
import d5 from '../../assets/cn/set/d5.jpg'
import d6 from '../../assets/cn/set/d6.jpg'
import d7 from '../../assets/cn/set/d7.jpg'
import d8 from '../../assets/cn/set/d8.jpg'
import d9 from '../../assets/cn/set/d9.jpg'
import appdemo from '../../assets/cn/tpl/appdemo.mp4'
import chasing from '../../assets/cn/tpl/chasing.mp4'
import durabilityProduct from '../../assets/cn/tpl/durability-product.webp'
import durability from '../../assets/cn/tpl/durability.mp4'
import selfie from '../../assets/cn/tpl/selfie.mp4'
import spotlight from '../../assets/cn/tpl/spotlight.mp4'
import tryonProduct from '../../assets/cn/tpl/tryon-product.webp'
import tryon from '../../assets/cn/tpl/tryon.mp4'
import vfx from '../../assets/cn/tpl/vfx.mp4'

export const DOWNLOAD_MAC =
  'https://dl.omnimux.ai/releases/latest/OmniMux-universal.dmg'
export const DOWNLOAD_MAC_ARM64 =
  'https://dl.omnimux.ai/releases/latest/OmniMux-arm64.dmg'
export const DOWNLOAD_MAC_X64 =
  'https://dl.omnimux.ai/releases/latest/OmniMux-x64.dmg'
export const DOWNLOAD_WIN =
  'https://dl.omnimux.ai/releases/latest/OmniMux-x64-Setup.exe'
// The public source repository. The hero links here for readers who want the
// code before the binary; it is not a download.
export const SOURCE_REPO = 'https://github.com/omnimux-ai/omnimux-ai'

export type IconName =
  | 'flame'
  | 'scan'
  | 'clap'
  | 'send'
  | 'chart'
  | 'spark'
  | 'link'
  | 'at'
  | 'plus'
  | 'book'
  | 'up'
  | 'right'
  | 'check'
  | 'cal'
  | 'img'
  | 'video'
  | 'mic'
  | 'cut'
  | 'layers'
  | 'grid'
  | 'folder'
  | 'bulb'
  | 'box'
  | 'chat'
  | 'flow'
  | 'apps'
  | 'term'
  | 'puzzle'
  | 'x'
  | 'eye'
  | 'heart'
  | 'mark'
  | 'cart'
  | 'shield'
  | 'undo'
  | 'music'
  | 'shirt'
  | 'crop'
  | 'hash'
  | 'type'
  | 'zoom'
  | 'user'
  | 'users'
  | 'subs'
  | 'pointer'
  | 'hand'
  | 'dl'
  | 'copy'
  | 'menu'
  | 'monitor'
  | 'clock'
  | 'wand'
  | 'apple'
  | 'win'

export type Mode = 'agent' | 'video' | 'image'

/**
 * LobeHub icon spec per model row, keyed by the brand name in
 * `copy.models[].name` (identical across locales). Rows missing here
 * fall back to `model.mark`.
 */
export const MODEL_BRAND_ICONS: Record<string, string> = {
  'Seedance 2.5': 'Doubao.Color',
  'Seedance 2.0': 'Doubao.Color',
  'GPT Image 2.5': 'OpenAI',
  'Nano Banana': 'Gemini.Color',
  Suno: 'Suno',
  Gemini: 'Gemini.Color',
}

/**
 * Media for the hero reel / agent demo tiles, ordered to match
 * `copy.hero.reel` in each locale.
 */
export const REEL_MEDIA = [
  { src: vfx, poster: vfxPoster },
  { src: tryon, poster: tryonPoster },
  { src: durability, poster: durabilityPoster },
  { src: selfie, poster: selfiePoster },
  { src: chasing, poster: chasingPoster },
  { src: appdemo, poster: appdemoPoster },
  { src: spotlight, poster: spotlightPoster },
] as const

/**
 * Media for the discover-step trending cards, ordered to match
 * `copy.flow.discover.items`.
 */
export const TRENDING_MEDIA = [
  { image: polka, video: undefined, poster: undefined, avatar: '#FF8BA7' },
  { image: undefined, video: red, poster: redPoster, avatar: '#FFB627' },
  {
    image: undefined,
    video: serumMain,
    poster: serumPoster,
    avatar: '#7FC8FF',
  },
] as const

/**
 * Media + styling keys for the publish-step rows, ordered to match
 * `copy.flow.publish.posts`.
 */
export const POSTS_MEDIA = [
  { poster: spotlightPoster, platform: 'tiktok', stateClass: 's-pub' },
  { poster: vfxPoster, platform: 'tiktok', stateClass: 's-sch' },
  { poster: tryonPoster, platform: 'instagram', stateClass: 's-chk' },
  { poster: selfiePoster, platform: 'youtube', stateClass: 's-busy' },
] as const

export const ASSETS = {
  d1,
  d2,
  d3,
  d4,
  d5,
  d6,
  d7,
  d8,
  d9,
  can,
  canSq,
  kitchen,
  kitchenB,
  pizza,
  polka,
  polkaB,
  redB,
  drinkAd,
  drinkPoster,
  durability,
  durabilityPoster,
  selfie,
  selfiePoster,
  redPoster,
  serumPoster,
  durabilityProduct,
  tryonProduct,
  ugcDemo,
  ugcPoster,
} as const
