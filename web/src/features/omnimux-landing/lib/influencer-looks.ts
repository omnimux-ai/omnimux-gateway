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
import type { GuestDraft } from '@/features/influencer/lib/guest'
import type { Selection } from '@/features/influencer/types'

import lookAfro from '../assets/influencer/look-goth-afro.webp'
import lookCorkscrews from '../assets/influencer/look-sporty-corkscrews.webp'
import lookBob from '../assets/influencer/look-y2k-bob.webp'
import workSporty from '../assets/influencer/work-sporty-mushroom.webp'
import workRetro from '../assets/influencer/work-retro-mullet.webp'
import workY2k from '../assets/influencer/work-y2k-braids.webp'
import workStairs from '../assets/influencer/work-retro-stairs.webp'
import mvBefore from '../assets/influencer/mv-before.webp'
import mvAfter from '../assets/influencer/mv-after.webp'
import strip01 from '../assets/influencer/strip-01.webp'
import strip02 from '../assets/influencer/strip-02.webp'
import strip03 from '../assets/influencer/strip-03.webp'
import strip04 from '../assets/influencer/strip-04.webp'
import strip05 from '../assets/influencer/strip-05.webp'
import strip06 from '../assets/influencer/strip-06.webp'
import strip07 from '../assets/influencer/strip-07.webp'
import strip08 from '../assets/influencer/strip-08.webp'
import strip09 from '../assets/influencer/strip-09.webp'
import strip10 from '../assets/influencer/strip-10.webp'
import strip11 from '../assets/influencer/strip-11.webp'
import strip12 from '../assets/influencer/strip-12.webp'
import strip13 from '../assets/influencer/strip-13.webp'
import strip14 from '../assets/influencer/strip-14.webp'
import strip15 from '../assets/influencer/strip-15.webp'
import strip16 from '../assets/influencer/strip-16.webp'
import strip17 from '../assets/influencer/strip-17.webp'
import strip18 from '../assets/influencer/strip-18.webp'
import strip19 from '../assets/influencer/strip-19.webp'
import strip20 from '../assets/influencer/strip-20.webp'
import heroAnyVideo from '../assets/influencer/hero-any-video.mp4'
import heroAnyVideoPoster from '../assets/influencer/hero-any-video-poster.webp'
import heroCharacter from '../assets/influencer/hero-character.webp'
import heroYourVideo from '../assets/influencer/hero-your-video.mp4'
import heroYourVideoPoster from '../assets/influencer/hero-your-video-poster.webp'

/** 一个可演示的角色：夸张度 + 特征 + 成品图，全部来自工作台的真实生成记录。 */
export interface InfluencerLook {
  /** i18n key suffix under influencer_landing.example.* / influencer_landing.work.* */
  key: 'a' | 'b' | 'c' | 'd'
  tier: 'normal' | 'freak' | 'total'
  selection: Selection
  image: string
}

export const HERO_LOOKS: InfluencerLook[] = [
  {
    key: 'a',
    tier: 'normal',
    image: lookAfro,
    selection: {
      accessory: ['acc_none'],
      aesthetic: ['goth'],
      age: ['adult'],
      body_type: ['body_muscular'],
      distinctive: ['df_nobrows'],
      ethnicity_origin_base: ['east_asian'],
      eye_color: ['eye_hazel'],
      eye_shape: ['es_large'],
      facial_hair: ['fh_goatee'],
      freak_face: ['ff_lips_4'],
      freak_head: ['head_standard'],
      gender: ['male'],
      hair: ['hair_afro'],
      hair_colour: ['hc_red'],
      height: ['h_avg'],
      proportions: ['pr_standard'],
      skin_tone: ['st_porcelain'],
    },
  },
  {
    key: 'b',
    tier: 'freak',
    image: lookCorkscrews,
    selection: {
      accessory: ['acc_hat'],
      aesthetic: ['sporty'],
      age: ['senior'],
      body_type: ['body_heavy'],
      distinctive: ['df_ears'],
      ethnicity_origin_base: ['middle_eastern'],
      eye_color: ['eye_grey'],
      eye_shape: ['es_large'],
      facial_hair: ['fh_moustache'],
      freak_face: ['ff_nose_1'],
      freak_head: ['head_heart'],
      freak_neck: ['neck_long'],
      gender: ['male'],
      hair: ['hs_corkscrews'],
      hair_colour: ['hc_blonde'],
      height: ['h_avg'],
      proportions: ['pr_waist_tiny'],
      skin_tone: ['st_brown_deep'],
    },
  },
  {
    key: 'c',
    tier: 'total',
    image: lookBob,
    selection: {
      accessory: ['acc_glasses'],
      aesthetic: ['y2k'],
      age: ['senior'],
      body_type: ['body_ultra'],
      distinctive: ['df_ears'],
      ethnicity_origin_base: ['european'],
      eye_color: ['eye_hazel'],
      eye_shape: ['es_downturned'],
      facial_hair: ['fh_none'],
      freak_face: ['ff_nose_3'],
      freak_head: ['head_tiny'],
      freak_neck: ['neck_standard'],
      gender: ['female'],
      hair: ['hs_lampshade'],
      hair_colour: ['hc_blue'],
      height: ['h_tall'],
      proportions: ['pr_shoulders_wide'],
      skin_tone: ['st_brown'],
    },
  },
]

export const WORK_LOOKS: InfluencerLook[] = [
  {
    key: 'a',
    tier: 'freak',
    image: workSporty,
    selection: {
      accessory: ['acc_bag'],
      aesthetic: ['sporty'],
      age: ['adult'],
      body_type: ['body_slim'],
      distinctive: ['df_nobrows'],
      ethnicity_origin_base: ['african'],
      eye_color: ['eye_hazel'],
      eye_shape: ['es_large'],
      facial_hair: ['fh_goatee'],
      freak_face: ['ff_nose_1'],
      freak_head: ['head_tiny'],
      freak_neck: ['neck_standard'],
      gender: ['male'],
      hair: ['hs_lampshade'],
      hair_colour: ['hc_red'],
      height: ['h_very_tall'],
      proportions: ['pr_legs_short'],
      skin_tone: ['st_brown'],
    },
  },
  {
    key: 'b',
    tier: 'normal',
    image: workRetro,
    selection: {
      accessory: ['acc_headphones', 'acc_jewelry'],
      aesthetic: ['retro'],
      age: ['adult'],
      body_type: ['body_heavy'],
      distinctive: ['df_ears', 'df_nobrows'],
      ethnicity_origin_base: ['east_asian'],
      eye_color: ['eye_brown'],
      eye_shape: ['es_large'],
      facial_hair: ['fh_none'],
      freak_face: ['ff_eye_bags', 'df_beauty_mark'],
      freak_head: ['head_heart'],
      freak_neck: ['neck_short'],
      gender: ['male'],
      hair: ['hair_mullet'],
      hair_colour: ['hc_black'],
      height: ['h_avg'],
      proportions: ['pr_waist_tiny'],
      skin_tone: ['st_porcelain'],
    },
  },
  {
    key: 'c',
    tier: 'normal',
    image: workY2k,
    selection: {
      accessory: ['acc_bag'],
      aesthetic: ['y2k'],
      age: ['mature'],
      body_type: ['body_athletic'],
      distinctive: ['df_lashes'],
      ethnicity_origin_base: ['african'],
      eye_color: ['eye_brown'],
      eye_shape: ['es_round'],
      facial_hair: ['fh_none'],
      freak_face: ['ff_brows_2'],
      freak_head: ['head_heart'],
      freak_neck: ['neck_short'],
      gender: ['female'],
      hair: ['hair_braids'],
      hair_colour: ['hc_white'],
      height: ['h_avg'],
      proportions: ['pr_waist_tiny'],
      skin_tone: ['st_ebony'],
    },
  },
  {
    key: 'd',
    tier: 'total',
    image: workStairs,
    selection: {
      accessory: ['acc_glasses'],
      aesthetic: ['retro'],
      body_type: ['body_athletic'],
      distinctive: ['df_grill'],
      eye_color: ['eye_blue'],
      eye_shape: ['es_large'],
      facial_hair: ['fh_none'],
      freak_face: ['ff_nose_1'],
      freak_head: ['head_tiny'],
      freak_neck: ['neck_long'],
      hair: ['hs_stairs'],
      hair_colour: ['hc_ginger'],
      height: ['h_very_tall'],
      proportions: ['pr_legs_short'],
      skin_tone: ['st_tan'],
    },
  },
]

export const MULTIVIEW = { before: mvBefore, after: mvAfter }

/**
 * 首屏等式用的三件真实素材：一个生成出的角色、一段参考视频、一段由此产出的视频。
 * 海报帧在「减弱动态」偏好下替代视频，避免自动播放。
 */
export const HERO_EQUATION = {
  character: heroCharacter,
  anyVideo: { src: heroAnyVideo, poster: heroAnyVideoPoster },
  yourVideo: { src: heroYourVideo, poster: heroYourVideoPoster },
}

export const STRIP_IMAGES = [
  strip01, strip02, strip03, strip04, strip05, strip06, strip07, strip08,
  strip09, strip10, strip11, strip12, strip13, strip14, strip15, strip16,
  strip17, strip18, strip19, strip20,
]

/** 点作品墙/创建器后写入的草稿：与工作台恢复逻辑读同一结构，空字段交给工作台默认。 */
export function draftFromLook(look: InfluencerLook): GuestDraft {
  return {
    tier: look.tier,
    selection: look.selection,
    brief: '',
    seed: '',
    imageUrl: '',
    model: '',
    group: '',
    vendorId: null,
  }
}
