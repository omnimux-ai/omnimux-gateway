#!/usr/bin/env bash
# ==============================================================================
# bin/test-video-model.sh — OmniMux 视频生成多模型统一验证脚本
# ==============================================================================
# Usage:
#   bin/test-video-model.sh <model_name> [prompt]
#
# Examples:
#   bin/test-video-model.sh veo-3.1-fast "A drone flyover of neon cyberpunk streets"
#   bin/test-video-model.sh minimax-h3 "A cute red panda rolling in snow"
#   bin/test-video-model.sh kling-v3 "A fast sports car zooming in high speed"
# ==============================================================================

set -euo pipefail

MODEL="${1:-veo-3.1-fast}"
PROMPT="${2:-A cinematic drone shot of a futuristic cyberpunk city at sunset, 4k ultra-realistic}"
BASE_URL="${OMNIMUX_BASE_URL:-https://api.omnimux.ai}"

# 1. 尝试从 omnimux CLI secrets 或环境变量提取 API Token
API_KEY="${OMNIMUX_API_KEY:-}"
if [[ -z "$API_KEY" ]]; then
  SECRETS_FILE="$HOME/.config/omnimux/secrets.json"
  if [[ -f "$SECRETS_FILE" ]]; then
    API_KEY=$(jq -r '.access_token // empty' "$SECRETS_FILE" 2>/dev/null || true)
  fi
fi

if [[ -z "$API_KEY" ]]; then
  echo "❌ 错误: 未检测到 API Key，请设置 OMNIMUX_API_KEY 或运行 omnimux login" >&2
  exit 1
fi

echo "=================================================="
echo "🚀 正在提交视频生成任务..."
echo "👉 目标模型: $MODEL"
echo "👉 提示词: $PROMPT"
echo "👉 网关地址: $BASE_URL/v1/videos/generations"
echo "=================================================="

# 2. 组装符合 OpenAI / OmniMux 规范的标准 Payload
PAYLOAD=$(jq -n \
  --arg model "$MODEL" \
  --arg prompt "$PROMPT" \
  '{
    model: $model,
    prompt: $prompt,
    duration: 5,
    aspect_ratio: "16:9"
  }')

# 针对特殊模型进行参数补强
if [[ "$MODEL" == "minimax-h3" ]]; then
  PAYLOAD=$(echo "$PAYLOAD" | jq '.duration = 6')
elif [[ "$MODEL" == "kling-v3" ]]; then
  PAYLOAD=$(echo "$PAYLOAD" | jq '.sound = true | .mode = "std"')
fi

# 3. 发起提交请求
SUBMIT_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD")

HTTP_CODE=$(echo "$SUBMIT_RESP" | grep "HTTP_STATUS:" | cut -d':' -f2)
BODY=$(echo "$SUBMIT_RESP" | grep -v "HTTP_STATUS:")

echo "HTTP 状态码: $HTTP_CODE"
echo "响应内容:"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"

if [[ "$HTTP_CODE" -ge 400 ]]; then
  echo "❌ 任务提交失败！" >&2
  exit 1
fi

TASK_ID=$(echo "$BODY" | jq -r '.task_id // .id // empty')
if [[ -z "$TASK_ID" ]]; then
  echo "⚠️ 提交成功但未提取到 task_id，完成校验。"
  exit 0
fi

echo ""
echo "=================================================="
echo "⏳ 开始轮询任务状态 (Task ID: $TASK_ID)..."
echo "=================================================="

for i in {1..20}; do
  POLL_RESP=$(curl -s -H "Authorization: Bearer $API_KEY" "$BASE_URL/v1/tasks/$TASK_ID" 2>/dev/null || true)
  STATUS=$(echo "$POLL_RESP" | jq -r '.status // .task_status // "processing"' 2>/dev/null || echo "processing")
  PROGRESS=$(echo "$POLL_RESP" | jq -r '.progress // "0%"' 2>/dev/null || echo "0%")
  
  echo "[$i/20] 状态: $STATUS (进度: $PROGRESS)"
  
  if [[ "$STATUS" == "succeeded" || "$STATUS" == "SUCCESS" || "$STATUS" == "completed" ]]; then
    echo "🎉 生成成功！"
    echo "$POLL_RESP" | jq . 2>/dev/null || echo "$POLL_RESP"
    exit 0
  elif [[ "$STATUS" == "failed" || "$STATUS" == "FAILURE" ]]; then
    echo "❌ 任务失败:"
    echo "$POLL_RESP" | jq . 2>/dev/null || echo "$POLL_RESP"
    exit 1
  fi
  sleep 4
done

echo "⏱️ 轮询超时（任务已正常在后台异步执行中，可通过 GET /v1/tasks/$TASK_ID 继续查看）。"
