# Talking-head 口播节奏契约

竖屏口播 / talking-head / UGC monologue 写脚本时使用。剧情多镜头、无对白镜头可参考，不必强制。

本文件是 **口播词数→建议时长** 的完整真源。其它 skill 只应摘要，不要另起完整表。

## 目标语速（启发式）

| 语言 | 建议语速 | 说明 |
| --- | --- | --- |
| English | 约 **2.0–2.5 words/s** | 接近常见 TikTok talking-head 口播密度 |
| 中文 | 约 **3.5–5 字/s** | 启发式，非平台官方；含轻语气词时取偏低 |

用户给了**硬性时长**时，以用户为准；否则用下表估 `suggested_duration` / handoff 时长。

过少词：画面易空、模型/表演易填 “um” 或僵停。  
过多词：抢拍、口型不稳（有声/对口型视频时更明显）。

## 单 take 建议时长（English）

| Script size (EN words) | Suggested take duration |
| --- | --- |
| 1–11 | ~5s |
| 12–22 | ~10s |
| 23–33 | ~15s |
| 34+ | 拆成多个 take；**不要静默裁掉 monologue 语义** |

经验：约 33 个英文词是单段 ~15s 口播的上限附近（≈15 × 2.2）。再长应分 take，而不是硬塞进一段。

### 示例（EN）

- ~5s（9 words）：*This app completely changed my morning routine — try it.*
- ~10s（20 words）：*I've used this for two weeks and it saves me thirty minutes every morning. My coffee is still hot.*
- ~15s（约 33 words）：整段完整卖点句保留，不压缩成半句。

## 单 take 建议时长（中文启发式）

| 汉字约数（去标点粗计） | 建议 take |
| --- | --- |
| ≤18 | ~5s |
| 19–40 | ~10s |
| 41–60 | ~15s |
| 61+ | 分 take 拼接语义，**不砍论点** |

混排中英时：英文按 word 计、中文按字计，再合成总时长预算；不确定时略放宽 take 或拆句。

## 长独白规则

1. 用户给了完整 monologue / 完整口播稿时，**默认保留全部论点与证明句**。
2. 需要适配模型单段时长上限时：声明 **multi-take 切分计划**（每 take 的词数/字数与建议秒数），而不是静默删句。
3. 仅在用户明确要求「压到 N 秒 / 砍掉某段」时才 trim。

## 与输出字段的关系

- 有 `suggested_duration` / `duration_seconds` 时：优先写本表推算值，并在风险/假设里注明「由口播节奏启发式估算」。
- handoff 给分镜 / prompt / 成片时：带上 **总时长预算** 与 **是否 multi-take**。

## 不做

- 不绑定任何第三方 UGC SaaS、credit 表或 exclusive API。
- 不把本表当作计费或 SLA 承诺。
- 不把节奏表当成固定 workflow 或 required-next tool。
