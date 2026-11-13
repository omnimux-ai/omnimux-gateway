# 火山引擎官方通用精品音色库检索手册（Agent 索引规范）

本文档为 Agent、业务系统及开发者提供火山引擎（豆包语音大模型）官方通用精品音色库的速查表与调用指引。
完整数据源：`docs/assets/volcengine-voices.json`（收录 509 款官方音色）。

---

## 一、 调用技术规范（极简调用，免克隆）

* **请求方式**：无需训练，直接调用语音合成接口（`POST https://openspeech.bytedance.com/api/v3/tts/unidirectional`）
* **Resource ID**：
  * 大模型 2.0 音色：`X-Api-Resource-Id: seed-tts-2.0`
  * 大模型 1.0 音色：`X-Api-Resource-Id: seed-tts-1.0`
* **Speaker**：直接传入官方 `voice_type` 代码（或在 CLI 运维工具中使用中文名别名，工具会自动解析）。

---

## 二、 核心爆款音色速查表（按场景分类）

### 1. 视频带货与广告营销（高转化、强感染力）
| 中文音色名 | 官方 voice_type 代号 | 推荐模型 | 特色标签 | 适用内容 |
| :--- | :--- | :--- | :--- | :--- |
| **广告解说 2.0** | `zh_male_guanggaojieshuo_uranus_bigtts` | `seed-tts-2.0` | 剪映同款 | 电商带货、口播广告、促销催单、爆款混剪 |
| **顾姐 2.0** | `zh_female_gujie_uranus_bigtts` | `seed-tts-2.0` | 抖音/剪映同款 | 大女主带货、职场逆袭、女性健康、情感认知 |
| **广告解说** | `zh_male_chunhui_mars_bigtts` | `seed-tts-1.0` | 剪映同款 | 经典稳健商业广告配音 |
| **营销小楠 2.0** | `ICL_uranus_zh_female_yingxiaokefu_v2_tob` | `seed-tts-2.0` | 官方精选 | 售前咨询、产品卖点宣讲、温和促单 |

### 2. 短视频影视与资讯解说
| 中文音色名 | 官方 voice_type 代号 | 推荐模型 | 特色标签 | 适用内容 |
| :--- | :--- | :--- | :--- | :--- |
| **解说小明** | `zh_male_jieshuoxiaoming_moon_bigtts` | `seed-tts-1.0` | 抖音/豆包同款 | “注意看，这个男人叫小帅”影视解说、冷知识科普 |
| **悬疑解说** | `zh_male_changtianyi_mars_bigtts` | `seed-tts-1.0` | 抖音/剪映同款 | 奇案破案、恐怖故事、悬疑解说 |
| **爽快思思** | `zh_female_shuangkuaisisi_moon_bigtts` | `seed-tts-1.0` | 豆包同款 | 节奏轻快、干脆利落的日常口播与种草 |
| **渊博小叔 2.0** | `zh_male_yuanboxiaoshu_uranus_bigtts` | `seed-tts-2.0` | 豆包/剪映同款 | 严肃科普、科技数码、财经商业分析 |

### 3. 通用陪伴、知性与品牌播报
| 中文音色名 | 官方 voice_type 代号 | 推荐模型 | 特色标签 | 适用内容 |
| :--- | :--- | :--- | :--- | :--- |
| **知性女声 2.0** | `zh_female_zhixingnv_uranus_bigtts` | `seed-tts-2.0` | 官方精品 | 品牌故事、企业宣传片、知性新闻播报 |
| **亲切女声 2.0** | `zh_female_qinqienv_uranus_bigtts` | `seed-tts-2.0` | 豆包同款 | 助手对话、长篇陪伴、温馨提示 |
| **清爽男大 2.0** | `zh_male_qingshuangnanda_uranus_bigtts` | `seed-tts-2.0` | 豆包同款 | 青春阳光，年轻态潮流穿搭、校园生活 |
| **柔美女友 2.0** | `zh_female_roumeinvyou_uranus_bigtts` | `seed-tts-2.0` | 官方精品 | 情感电台、温柔对白、剧情角色扮演 |

### 4. TikTok 全球化出海多语种音色
| 音色名称 | 语种 | 官方 voice_type 代号 | 推荐模型 |
| :--- | :--- | :--- | :--- |
| **Charlie 2.0** | 美式英语 (女) | `ICL_uranus_en_female_charlie_tob` | `seed-tts-2.0` |
| **Noah 2.0** | 美式英语 (男) | `ICL_uranus_en_male_noah_tob` | `seed-tts-2.0` |
| **Adam** | 美式英语 (男) | `en_male_adam_mars_bigtts` | `seed-tts-1.0` |
| **Alastor 2.0** | 英式英语 (男) | `ICL_uranus_en_male_alastor_tob` | `seed-tts-2.0` |
| **Ethan 2.0** | 澳洲英语 (男) | `ICL_uranus_en_male_ethan_tob` | `seed-tts-2.0` |
| **Poppy** | 日语 (女) | `ja_female_bv520_uranus_bigtts` | `seed-tts-2.0` |
| **Ken** | 日语 (男) | `ja_male_bv524_uranus_bigtts` | `seed-tts-2.0` |
| **Momo** | 韩语 (女) | `ko_female_bv546_uranus_bigtts` | `seed-tts-2.0` |
| **Rosa** | 墨西哥西语 (女) | `mx_female_bv166emotion_uranus_bigtts` | `seed-tts-2.0` |
| **Diego** | 墨西哥西语 (男) | `mx_male_bv165dialogue_uranus_bigtts` | `seed-tts-2.0` |
| **Sam** | 巴西葡萄牙语 (男) | `pt_male_bv172_uranus_bigtts` | `seed-tts-2.0` |
| **Lola** | 巴西葡萄牙语 (女) | `pt_female_bv173emotion_uranus_bigtts` | `seed-tts-2.0` |

---

## 三、 Agent 消费与执行指引

### 1. 命令行快速合成
```bash
node scripts/ops/volcengine-voice/voice-tts.js \
  --speaker "广告解说 2.0" \
  --text "限时特惠，今天下单买一送一，马上点击左下角链接！" \
  --output ./ad_output.mp3
```

### 2. 网关 API 消费契约
Agent 调用 OmniMux 网关的标准 `/v1/audio/speech` 接口时，可直接在 `voice` 字段填入上述任意音色代号：
```json
{
  "model": "seed-audio-1.0",
  "input": "欢迎收听今天的节目。",
  "voice": "zh_male_guanggaojieshuo_uranus_bigtts"
}
```
