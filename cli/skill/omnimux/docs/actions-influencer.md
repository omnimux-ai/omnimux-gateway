# 角色与参考视频创作

这些命令连接同一账号下的网页角色与作品，不要求选择模型或供应商。能发现命令不代表当前服务已开放生成；先读取可用选项。

## 发现与授权

- `omnimux influencer options`：读取人物三档、拟人猫狗、外观选项、当前规格和创建开关。
- `omnimux schema influencer.video.create`：读取一个动作的参数和风险。
- 沿用一次账号登录。旧凭据需重新登录并明确授权创作读取、生成以及消耗账号积分或订阅额度。
- 三项创建动作会消耗额度，只有已有明确用户授权覆盖相应创作时才使用 `--yes`。实际支付、充值或购买仍由人完成。

## 创建角色

```bash
omnimux influencer character create --type normal --brief '穿蓝色外套的成年人物' --request-id character-001 --dry-run
omnimux influencer character create --type normal --brief '穿蓝色外套的成年人物' --request-id character-001 --yes
omnimux influencer character list --page 1 --page-size 20
omnimux influencer character get <task_id>
omnimux influencer character multiview <task_id> --request-id multiview-001 --yes
```

`--type`：`normal`（默认）、`freak`、`total`、`cats`、`dogs`。`--selection` 是分类到选项编号数组的对象，例如 `'{"gender":["female"]}'`；实际选项从 `options` 读取。可选 `--reference-image` 接受本地图片或 HTTPS 地址，本地图片上限 5 MiB。

角色引用就是成功原始主图的 `task_id`。列表只列可复用的成功主图，失败与排队任务从任务历史查看。多角度图保留来源关系，不能继续作为多角度生成的父作品。

## 创建视频

```bash
omnimux influencer video create --character <task_id> --video reference.mp4 --request-id video-001 --dry-run
omnimux influencer video create --character <task_id> --video reference.mp4 --request-id video-001 --yes
```

- 用 `--character` 复用主图，也可重复使用 `--image` 提供有序图片；没有角色编号时第一张图片是主要角色。角色主图计入总共 1–30 张图片。
- `--video` 为本地参考视频或 HTTPS 地址；也可用互斥的 `--video-task` 引用本人成功视频。本地视频上限 100 MiB，真实时长由后台核验，需 4–30 秒，不填写时长参数。
- 默认 `--mode motion --resolution 480p`。运动交换采用明确指定的画幅，否则由后台按参考视频决定；`--mode replace` 保持来源比例，冲突会拒绝。
- `--aspect-ratio` 有效组合：480p 支持 `16:9`、`9:16`、`3:4`；720p 支持 `16:9`、`9:16`。原声、图片顺序和角色种类会保留。
- `--prompt` 可补充创作要求，不新增配音、发布或批量生成。
- `--input file.json` 可提供相应 HTTP 产品字段，不能与重叠的显式创作字段混用。禁止模型、分组、报价、手填视频时长字段。

## 等待、恢复与下载

```bash
omnimux influencer task list --kind video --status running
omnimux influencer task get <task_id>
omnimux influencer request get video-001
omnimux influencer task wait <task_id> --timeout 600
omnimux influencer task download <task_id> --output ./result.mp4
```

创建命令成功只表示已受理。`task get` 成功可以携带失败任务；`task wait` 只有任务成功且有可用成品时才成功。等待默认十分钟，可中断；超时返回 `api/wait_timeout`，云端任务继续，不重新生成、不取消。

上传前会在私密配置目录保存按账号和服务区分的恢复记录。保留 `request_id`，不要因为网络中断而换新编号：重新执行相同创建时先查回原请求，仅后台明确未受理时才以同号、原始内容重试；不会重新上传。相同编号换要求或更换本地文件内容会报冲突。未知结果先用 `influencer request get`，不要盲目重新收费生成。

下载必须明确 `--output`，已有文件默认拒绝覆盖；确需替换时加 `--overwrite`。下载到临时文件并验证长度、类型和文件头后才完成；失败只重取原任务成品。签名成品地址不会写入本地恢复记录或下载输出，账户凭据不会发给储存地址或随跳转转发。

`--dry-run` 完全离线：只核对本地能判断的内容，不读取账号凭据、不上传、不保存恢复记录、不自动升级。后台素材检查仍为待核验，不代表生成一定可受理或一定成功。

普通创建回执、任务查询、等待和历史列表只返回成品类型等安全信息，包含嵌套作品时也不显示临时访问凭证。公开选项图片仍可查看。取回原始成品请使用 `task download`；签名地址仅在下载内部使用。下载会拒绝分段响应，不能把视频片段当作完整成片。
