package setting

// TaskProxyEnabled 全量启用任务媒体结果本地反代（隐藏上游真实渠道直链）
// 开启后，对客户端输出的视频/图片/音频链接统一转换为本站反代地址：
// - /v1/videos/:task_id/content
// - /v1/images/:task_id/content
var TaskProxyEnabled = true
