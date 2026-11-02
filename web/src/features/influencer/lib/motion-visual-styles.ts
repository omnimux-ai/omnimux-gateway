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

import type { MotionVisualStyle } from '../types'

// Display media is separate from generation inputs. Provenance and the
// non-official, untested adaptation source are recorded in motion-style-provenance.json.
export const MOTION_VISUAL_STYLES: MotionVisualStyle[] = [
  {
    id: 'motion-cel-animation',
    nameKey: 'Hand-Painted Cel Animation',
    summaryKey: 'Clean outlines and flat painted colors',
    prompt:
      '采用手绘二维动画的清晰轮廓、平涂色块与克制分层阴影，背景带轻微水彩纸感，避免写实皮肤与塑料高光。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/3dfb922fa586ea147aaaf3f07720b4e0437266877f658d56d65c65768cf1455b.webp',
    previewVideoUrl:
      '/influencer-styles/e7506410a571b89670b714cd292818036cfc50fa5c8cf1b255dcbb079273d922.mp4?v=identity-20261016',
  },
  {
    id: 'motion-knitted-yarn-world',
    nameKey: 'Knitted Yarn World',
    summaryKey: 'Knitted loops and soft yarn fibers',
    prompt:
      '将可见表面表现为清晰的针织线圈与毛线纤维，保留物体结构和轮廓，针脚尺度稳定，避免将针织效果误做成光滑塑料。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/0575005a6e0d97fd002958c24d232bbbb08f260c7d491ac5b0dd9fc1800682e0.webp',
    previewVideoUrl:
      '/influencer-styles/3d6110f9df9788ce03f0df00ef47f8cc184a135874d98d3807a6877c0b643f91.mp4?v=identity-20261016',
  },
  {
    id: 'motion-expressive-2d-animation',
    nameKey: 'Expressive 2D Animation',
    summaryKey: 'Bold lines and expressive painted shadows',
    prompt:
      '采用利落的二维线稿、带方向感的笔触与硬边块状明暗，形成富有表现力的动画质感；画风变化不增加新的动作或镜头。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/fe437c82639fa59b06a1baf27304d639eaedb278fea38048ba824818e88d9ca6.webp',
    previewVideoUrl:
      '/influencer-styles/f512372409b4c92d944b32275cf747ab871e799dd524bfb27fe863e0324ec17c.mp4?v=identity-20261016',
  },
  {
    id: 'motion-rounded-toy-cgi',
    nameKey: 'Rounded Toy CGI',
    summaryKey: 'Rounded forms and toy-like materials',
    prompt:
      '采用圆润体块、简洁表面和受控的玩具材质反射，保留角色辨识点，不把人物面孔统一替换为样片角色或改变物种。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/882614d8dba6d921f840c35842c226df1110c6f8a4f9ae4a4419a1e73e3bd912.webp',
    previewVideoUrl:
      '/influencer-styles/e5cd4737f5949fa8366856e93ec9fc278373b14ff4380b63130d1ec3d58fc361.mp4?v=identity-20261016',
  },
  {
    id: 'motion-felt-puppet-stop-motion',
    nameKey: 'Felt Puppet Stop Motion',
    summaryKey: 'Soft felt fibers and handcrafted texture',
    prompt:
      '采用细密毛毡绒毛、手工木偶表面与柔和体积阴影，表现手作定格动画的触感；保持原动作时间和镜头节奏，不擅自抽帧。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/4578b9bb40c79f80e10f0e7b1e9db8112ccfec58465059ca63acb9df9f74ae4a.webp',
    previewVideoUrl:
      '/influencer-styles/132dc19352019896dc20f35ab141d40071540ee7625becad2790deade0e44ff3.mp4?v=identity-20261016',
  },
  {
    id: 'motion-grotesque-street-cgi',
    nameKey: 'Grotesque Street CGI',
    summaryKey: 'Exaggerated forms and gritty textures',
    prompt:
      '采用略带夸张的立体造型、街头插画式轮廓和粗粝材质；只调整造型语言与表面表现，不加入样片的街景、服装、音响或额外人物。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/fd9a5c91bcc96fe4b41c34bbf1964691284a550087df4634ac8224918b8836bc.webp',
    previewVideoUrl:
      '/influencer-styles/e2ebfa8458a0e779e56b42db9d4ef23986b2ea7aab0f63fcfc1051c8be6e8673.mp4?v=identity-20261016',
  },
  {
    id: 'motion-warm-painterly-3d-animation',
    nameKey: 'Warm Painterly 3D Animation',
    summaryKey: 'Warm colors and softly painted 3D forms',
    prompt:
      '采用温暖克制的绘画调色、柔和立体明暗和细腻手绘表面，保留原场景的光源方向、时间与人物身份，不把整体画面简单染黄。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/eb3fbb4f025c00f57f81de9a991760de009a5079940a82f8981a416021163412.webp',
    previewVideoUrl:
      '/influencer-styles/302b363a15ee1510bc4c7842233918505b1da06a6b84d6cb91f971069b6d5515.mp4?v=identity-20261016',
  },
  {
    id: 'motion-graphic-cel',
    nameKey: 'Graphic Cel',
    summaryKey: 'Graphic color blocks and crisp cel shadows',
    prompt:
      '采用图形化的赛璐璐色块、干净外轮廓与锐利阴影形状，压缩细碎纹理并保持轮廓辨识，不复制样片的夸张视角或服装设计。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/4c253a18d3e2045a5a2aa4bbd3a419bc4c7fdc925b4d9e041faf6a86782f59e5.webp',
    previewVideoUrl:
      '/influencer-styles/97c9044dcbc1455501bcd7c8e7230b40a255e479dda8ab812c2a29f5d016ea1e.mp4?v=identity-20261016',
  },
  {
    id: 'motion-expressive-gouache-fashion',
    nameKey: 'Expressive Gouache Fashion',
    summaryKey: 'Gouache colors and expressive brushwork',
    prompt:
      '采用不透明水粉颜料的覆盖感、明显笔触和概括性大色块，保留主体服装设计和色彩识别；只借鉴画法，不引入样片人物或配饰。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/3fa04f862d7a27ebe8739e88b1994957f379115010c2864c578ff73604a4dd46.webp',
    previewVideoUrl:
      '/influencer-styles/8de1b178db9b50c9966a19d6ed3053d7af0a80f61805c523e71aadbfb6bc65ce.mp4?v=identity-20261016',
  },
  {
    id: 'motion-sculptural-3d-fashion',
    nameKey: 'Sculptural 3D Fashion',
    summaryKey: 'Sculpted volumes and refined matte surfaces',
    prompt:
      '采用雕塑般的立体体块、清晰材质分区和精致哑光表面，在原有衣物结构上强化体积；不加入巨手、微缩人物或改变场景尺度。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/1aa3ed87b87f98f204eea2303465d9db37988d8dbda93ff2e198d31cd7ea8789.webp',
    previewVideoUrl:
      '/influencer-styles/65ce0522354bb48d5ad8f2f9a38900510c9a800914289211b436fbbc234f71bf.mp4?v=identity-20261016',
  },
  {
    id: 'motion-chunky-textured-low-poly',
    nameKey: 'Chunky Textured Low-Poly',
    summaryKey: 'Chunky low-poly forms and coarse textures',
    prompt:
      '采用可辨识的低多边形切面、厚重体块和粗颗粒手工纹理，明暗沿造型分区；保持关节关系、脸部辨识点和动作连贯。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/7f223e758fa5a4a68c7c73493fbd38185c216532f24d3727258f040bdd94eb65.webp',
    previewVideoUrl:
      '/influencer-styles/32a803285af31ffe3856580aa830e6dc394b16a1106a45fedbd382eb09acab16.mp4?v=identity-20261016',
  },
  {
    id: 'motion-cel-shaded-cg-anime',
    nameKey: 'Cel-Shaded CG Anime',
    summaryKey: '3D depth with anime outlines and cel shading',
    prompt:
      '采用三维体积配合动漫式描线与分层明暗，保留立体空间中的动作和镜头连续性，避免写实皮肤与过强镜面反光。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/596f2ce4c84660a458ea4fd41e50f96b39f425d415a5723e8ce272a2fddf96c1.webp',
    previewVideoUrl:
      '/influencer-styles/cf8cf6899c5378883fb7a78da85229189c8e8fb1265ba717eca2fda68bcb55ff.mp4?v=identity-20261016',
  },
  {
    id: 'motion-painterly-cinematic-3d',
    nameKey: 'Painterly Cinematic 3D',
    summaryKey: 'Cinematic 3D forms and painterly surfaces',
    prompt:
      '采用电影感立体造型与可见绘画纹理，明暗过渡兼顾体积和笔触；延续原视频构图、光源方向与空间，不加入样片中的服饰或布景。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/1b105870b6d9ab58ac36b6181d5eefa2ca587f75d375f1f6dc9c3a61236c9fce.webp',
    previewVideoUrl:
      '/influencer-styles/47966011400250f0d40b417d16f4d706396c0a2133ab7f83eef712b81fbec374.mp4?v=identity-20261016',
  },
  {
    id: 'motion-outline-free-cel',
    nameKey: 'Outline-free Cel',
    summaryKey: 'Flat colors and clean shapes without outlines',
    prompt:
      '采用无黑色外描边的二维平涂，以相邻色块和简洁阴影区分形体，保留干净轮廓与清晰动作，避免线稿边框和照片式皮肤纹理。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/2dec9d22eff935b97b362a591f2a165bcb5d39c0db10fb6fcc750dcd3c6b86a3.webp',
    previewVideoUrl:
      '/influencer-styles/07844572ad3a93beaadf469ecfed1944a1cab1f99e0a0a1e4f5a34ee23b23535.mp4?v=identity-20261016',
  },
  {
    id: 'motion-painterly-sculpted-animation',
    nameKey: 'Painterly Sculpted Animation',
    summaryKey: 'Sculpted forms and oil-painted texture',
    prompt:
      '采用雕塑式体块、手工塑形的起伏表面与油画式笔触，保留原有光照空间逻辑；不从样片借用新场景、道具或人物关系。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/515f34b4164593b53d548d90f9d3c687bca46e7da9468907c64baa19cb6c07f8.webp',
    previewVideoUrl:
      '/influencer-styles/839861df169d8b48d5257f32cfbc3316a14a827e11eeb4bb0ced0b195ad54018.mp4?v=identity-20261016',
  },
  {
    id: 'motion-neon-graphic-cinematic-3d',
    nameKey: 'Neon Graphic Cinematic 3D',
    summaryKey: 'Graphic 3D forms and colored rim light',
    prompt:
      '采用图形化三维切面、绘画式纹理与受控的冷暖色边光，色彩强调服从原场景，不凭空增加霓虹灯、夜景或新的光源。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/bc333d385f393c915336d9693248c1908d2a734988816d13cbb8a0472d970ccc.webp',
    previewVideoUrl:
      '/influencer-styles/665fe1848df7bcb715a3d3ea3b82c220b5be8095fe45ba9dc0c07cd3191ff4d2.mp4?v=identity-20261016',
  },
  {
    id: 'motion-textured-gouache-character-illustration',
    nameKey: 'Textured Gouache Character Illustration',
    summaryKey: 'Gouache shapes and dry-brush texture',
    prompt:
      '采用概括的角色插画轮廓、不透明水粉色块与干刷颗粒，局部保留纸面触感；保持实际动作和人物服饰，不套用示例摄影棚内容。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/04898eb689fb8396cb09e416e0ed26956616dd18c3921ce7816bd509a35b10bd.webp',
    previewVideoUrl:
      '/influencer-styles/76f444ffc6490b0eb9d12d9b1d068342191901df089f06f6f2cb5bfb46cb636b.mp4?v=identity-20261016',
  },
  {
    id: 'motion-comic-3d',
    nameKey: 'Comic 3D',
    summaryKey: 'Comic contours, halftone dots and bold colors',
    prompt:
      '采用漫画化三维造型、强弱分明的轮廓、半色调网点与鲜明色块，保持跨帧材质一致；不新增漫画分格、对话框、字幕或文字。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/76347d0722cf1e7f1790879fd92e2636d82a9369ddf373d5daddd2eae44909de.webp',
    previewVideoUrl:
      '/influencer-styles/b36f1d2b512c6205d3dda90d5f21c729cc4d2bdd30cd6cf54bc4b9846b0213b8.mp4?v=identity-20261016',
  },
  {
    id: 'motion-retro-game-3d',
    nameKey: 'Retro Game 3D',
    summaryKey: 'Low-poly game forms and low-resolution textures',
    prompt:
      '采用早期三维游戏的低面数造型、低分辨率纹理与简洁材质，保持原视频的动作时间和运镜，不加入游戏界面、标志或示例角色。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/4e2b70dbd31ae3ac518f51fed6ef676a6e8cd74135a4dd33c6b0ad48dcde1e72.webp',
    previewVideoUrl:
      '/influencer-styles/d79527d9494d69f71067a4ae6683db338428a8c536140a2717a8bc3d4c1380f2.mp4?v=identity-20261016',
  },
  {
    id: 'motion-nighttime-anime-pixel-art',
    nameKey: 'Nighttime Anime Pixel Art',
    summaryKey: 'Anime pixel grids and limited color shades',
    prompt:
      '采用可辨识的动漫像素网格、有限色阶与克制的抖色阴影，像素尺度在各帧保持一致；不因风格名含夜色就擅自把原场景改为夜晚。人物或动物的身份、物种和辨识点由用户角色参考确定；动作、镜头与节奏由用户参考视频确定。除用户明确要求修改外，不改变这些内容。仅改变线条、配色、纹理、材质与渲染表现，不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标。整个视频保持统一的画风、材质与配色，帧间稳定。',
    mode: 'motion',
    previewImageUrl:
      '/influencer-styles/2aa86941aa8da850d471374e25ddc0f6525d0053c0beb06ff762da6da2195d69.webp',
    previewVideoUrl:
      '/influencer-styles/f23e453f622c76a0970382de53f5c23fde455ede615a580440bdb354032169e6.mp4?v=identity-20261016',
  },
]
