# Plan Progress Update Patterns

Use `submit_plan` to update task status at common progression milestones. This file describes progress naming, not a mandatory runtime phase machine or unique next-tool contract.

## Task IDs

| ID        | Title          | Toolset                                                               |
| --------- | -------------- | --------------------------------------------------------------------- |
| `init`    | 准备生成配置   | common tool: `editor-start-slideshow-generate`                        |
| `copy`    | 生成文案内容   | reasoning                                                             |
| `images`  | 准备页面配图   | common tools: `image-match-gallery-batch` / `image-generate-ai-batch` |
| `persist` | 组装并发布模板 | common tool: `workflow-slideshow-persist`                             |

## Update Pattern

When the agent advances to the next common milestone, mark the previous task `completed` and next task `running`:

```json
{
  "operations": [
    { "op": "update-task", "taskId": "<previous>", "patch": { "status": "completed" } },
    { "op": "update-task", "taskId": "<next>", "patch": { "status": "running" } }
  ]
}
```

## Common Transition Sequence

1. After Step 0 (plan created): `init` → running (already set in task_write)
2. After Step 1 (init done): `init` → completed, `copy` → running
3. After Step 2 (copy done): `copy` → completed, `images` → running
4. After Step 3 (images done): `images` → completed, `persist` → running
5. After Step 4 (persist done): `persist` → completed
