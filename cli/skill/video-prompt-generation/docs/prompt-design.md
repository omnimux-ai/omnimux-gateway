# 视频提示词设计原则

## 先保住画面，再谈气质

一个可执行的视频 prompt，先要回答四件事：

1. 主体是谁或是什么
2. 场景在哪里
3. 动作按什么顺序发生
4. 镜头要优先让人看到什么

风格词、情绪词、广告感、电影感，都应该压在这四件事之后。

## Prompt 不是关键词堆

以下写法通常不够：

- cinematic
- premium
- aesthetic
- viral
- high quality

这些词可以保留，但只能当修饰层，不能替代主体、动作和镜头逻辑。

## 推荐组织顺序

1. **目标与主体**
2. **场景与环境**
3. **动作顺序**
4. **镜头重点**
5. **风格与光线**
6. **限制条件**

如果是短视频带货内容，优先明确：

- 开头钩子
- 中段证明
- 结尾记忆点或行动收束

## 冲突处理原则

遇到以下情况，要做最小修正并说明：

- 固定机位和大幅环绕运镜同时出现
- 极简背景和复杂人群街景同时出现
- 很短时长里塞入太多动作节点
- 想要真实感，又要求明显超现实效果

处理顺序：

1. 保留用户核心创意
2. 删掉最影响执行性的矛盾项
3. 把压缩或调整过的假设写出来

## 约束要写在明面上

稳定结果通常需要两类约束：

- **正向约束**：必须出现什么
- **负向约束**：不要出现什么

常见负向约束包括：

- 人物或产品外观漂移
- 手部异常
- 背景抢主体
- 材质失真
- 镜头顺序太碎
- 夸张且不可信的宣传表达

## Talking-head / UGC 口播

竖屏口播、talking-head monologue、旁白 + 产品/b-roll 时使用。成片执行合同真源在 sibling `video-creation`（Talking-head UGC execution contract）；节奏完整表在 `video-script-creation/docs/talking-head-pacing.md`。

### 主体锁定

- 同一任务多 take：用同一张人脸参考 / 同一人物描述锁定身份，避免每段换脸。
- 长独白或旁白 b-roll：优先人脸静帧 URL 或用户提供的照片，不要只靠空泛外貌词。

### Look 三档（写进 prompt 的示例意图）

| Look | 可写进 prompt 的方向 |
| --- | --- |
| `natural` | soft natural daylight, candid UGC framing, natural skin texture |
| `commercial` | clean brand-ready lighting, controlled background, polished but still human |
| `raw_iphone` | unpolished phone-shot feel via framing and light; believable imperfection |

避免塑料感 AI 脸；强调自然肤质与可信瑕疵即可。

### 禁词与改写

- 默认不要写：`selfie`、`holding a phone`、`phone camera`（除非产品本身是手机演示）。
- 改写为：`talking to camera`、`eye contact with the lens`、`handheld framing feel`。

### 词量与时长（摘要，可独立用）

| 语言 | 启发式 |
| --- | --- |
| EN | ~2.0–2.5 words/s；1–11 words ≈5s；12–22 ≈10s；23–33 ≈15s；34+ 拆 take |
| 中文 | ~3.5–5 字/s；≤18 字 ≈5s；19–40 ≈10s；41–60 ≈15s；61+ 拆 take |

用户硬性时长优先。过长口播写进 prompt 时声明 multi-take 切分，不静默删论点。

### 字幕

- 默认 **不** 在 prompt 里要求 burned-in captions / 花字。
- 仅当用户明确要字幕时再写，并可选用风格词：`hormozi` | `tiktok` | `minimal`（模型无枚举则改成画面描述）。
