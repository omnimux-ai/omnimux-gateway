-- seed.sql — 开发/演示种子数据（幂等：ON DUPLICATE / 先清后插）
-- 仅用于本地与 CI，不进生产。

INSERT INTO tags (name) VALUES
  ('爆款拆解'), ('好物分享'), ('vlog'), ('小红书'), ('TikTok'), ('AI 工具')
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO inspirations (type, title, content, source_url, source_url_hash, cover_key, media_keys, hot_score, is_favorite, analysis, created_at, updated_at) VALUES
  ('video', '3 步拍出电影感人像', '灯光+运镜+调色全流程，新手可直接复制的参数', 'https://www.tiktok.com/@demo/video/7300000000000000001', 'seed-hash-0001', 'seed/cover-01.jpg', '["seed/video-01.mp4"]', 982, 1,
   '{"hook": "你拍照总是差点意思？", "structure": ["痛点", "三步法", "成品对比"], "cta": "收藏慢慢练"}', NOW(), NOW()),
  ('image', '小红书封面排版 5 个万能模板', '大字报/对角线/拼图式，直接套用', 'https://xiaohongshu.com/explore/demo0002', 'seed-hash-0002', 'seed/cover-02.jpg', '["seed/slide-01.jpg","seed/slide-02.jpg"]', 761, 0,
   '{"hook": "封面决定点击率", "structure": ["模板展示", "套用示范"], "cta": "截图保存"}', NOW(), NOW()),
  ('link', 'AI 工具导航：视频创作者版', '剪辑/字幕/配音/混剪，30 个工具分类清单', 'https://example.com/ai-tools-for-video', 'seed-hash-0003', '', '[]', 455, 0,
   '{"hook": "别再一个个搜了", "structure": ["分类清单"], "cta": "转发团队"}', NOW(), NOW()),
  ('video', '好物开箱脚本模板：痛点三连', '痛点→场景→反转→产品，完播率模板', 'https://www.tiktok.com/@demo/video/7300000000000000004', 'seed-hash-0004', 'seed/cover-04.jpg', '["seed/video-04.mp4"]', 1103, 0,
   '{"hook": "还在硬夸产品？", "structure": ["痛点三连", "场景植入", "反转收尾"], "cta": "评论区扣模板"}', NOW(), NOW())
ON DUPLICATE KEY UPDATE updated_at = NOW();

INSERT INTO inspiration_tags (inspiration_id, tag_id)
SELECT i.id, t.id FROM inspirations i, tags t
WHERE (i.source_url_hash = 'seed-hash-0001' AND t.name IN ('爆款拆解','TikTok'))
   OR (i.source_url_hash = 'seed-hash-0002' AND t.name IN ('小红书','好物分享'))
   OR (i.source_url_hash = 'seed-hash-0003' AND t.name IN ('AI 工具'))
   OR (i.source_url_hash = 'seed-hash-0004' AND t.name IN ('好物分享','TikTok','爆款拆解'))
ON DUPLICATE KEY UPDATE inspiration_id = inspiration_id;
