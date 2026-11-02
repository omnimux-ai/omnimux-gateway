# Video Storyboard Examples

## 教程型

输入：

```text
topic: "5个护肤小技巧"
style: "tutorial"
duration: 60
```

输出示意：

```json
{
  "title": "5个护肤小技巧，皮肤变好真的很简单！",
  "hook": "你知道吗？90%的人护肤顺序都是错的",
  "scenes": [
    { "id": "1", "duration": 8, "description": "女生对着镜子涂抹护肤品的特写", "narration": "第一步，清洁..." },
    { "id": "2", "duration": 10, "description": "手持爽肤水轻拍脸部", "narration": "第二步，爽肤水打底..." }
  ],
  "cta": "关注我，教你更多变美技巧！",
  "caption": "护肤小白必看！5个简单技巧让你皮肤变好 #护肤 #变美"
}
```

## 叙事型

输入：

```text
topic: "我的北漂第一年"
style: "narrative"
duration: 90
```

输出示意：

```json
{
  "title": "北漂第一年，我经历了什么...",
  "hook": "一年前，我拖着一个行李箱来到这座城市",
  "scenes": [
    { "id": "1", "duration": 10, "description": "火车站拖着行李箱的年轻人背影", "narration": "那时候我什么都没有，只有一腔勇气..." },
    { "id": "2", "duration": 12, "description": "早高峰地铁拥挤的人潮", "narration": "每天地铁一小时，顶着咸鱼一样的脸..." }
  ],
  "cta": "你的北漂故事是什么？评论区告诉我",
  "caption": "北漂一年，从一无所有到站稳脚跟 #北漂 #成长 #说说你的北漂经历"
}
```

## 推广型

输入：

```text
topic: "新款蓝牙耳机推荐"
style: "promotional"
duration: 45
```

输出示意：

```json
{
  "title": "这款耳机让我彻底放弃 AirPods！",
  "hook": "以前我绝不会相信，300元能买到这种音质",
  "scenes": [
    { "id": "1", "duration": 8, "description": "耳机产品图旋转展示", "narration": "今天要给大家安利一款宝藏耳机..." },
    { "id": "2", "duration": 10, "description": "对比图：与千元耳机并排", "narration": "音质对标千元级，低频有力，中频温暖..." }
  ],
  "cta": "点击小黄车立即下单！",
  "caption": "平价替代来了！300元音质天花板 #耳机推荐 #好物分享"
}
```
