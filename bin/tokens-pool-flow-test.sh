#!/bin/bash
# tokens-pool Flow platform debug tool (reusable).
#
# Usage:
#   bin/tokens-pool-flow-test.sh models              # list models the pool currently accepts
#   bin/tokens-pool-flow-test.sh veo [prompt]        # submit a veo_3_1 video task (prints task id)
#   bin/tokens-pool-flow-test.sh task <task_id>      # poll task status/result
#   bin/tokens-pool-flow-test.sh chat [model]        # quick chat probe (default: omni_flash)
#   bin/tokens-pool-flow-test.sh image [model]       # quick image probe (default: nano_banana_2)
#
# Auth resolution: $TOKENS_POOL_TOKEN > macOS Keychain (service=tokens-pool, account=flow_token)
# Base URL:        $TOKENS_POOL_BASE > default production /flow
# The token is never printed.
set -u

BASE="${TOKENS_POOL_BASE:-https://tokens-production-355b.up.railway.app/flow}"
TOKEN="${TOKENS_POOL_TOKEN:-$(security find-generic-password -s tokens-pool -a flow_token -w 2>/dev/null || true)}"

if [ -z "$TOKEN" ]; then
  echo "ERROR: no token. Set TOKENS_POOL_TOKEN or store it in Keychain:" >&2
  echo "  security add-generic-password -U -s tokens-pool -a flow_token -w <token>" >&2
  exit 2
fi

CMD="${1:-}"
case "$CMD" in
  models)
    curl -sS -m 30 "$BASE/v1/models" -H "Authorization: Bearer $TOKEN" | jq .
    ;;
  veo)
    PROMPT="${2:-a calm blue ocean wave, cinematic}"
    curl -sS -m 60 -X POST "$BASE/v1/video/generations" \
      -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
      -d "$(jq -n --arg p "$PROMPT" '{model:"veo_3_1", prompt:$p}')" | jq .
    ;;
  task)
    TID="${2:-}"
    [ -z "$TID" ] && { echo "usage: $0 task <task_id>" >&2; exit 2; }
    curl -sS -m 30 "$BASE/v1/video/generations/$TID" -H "Authorization: Bearer $TOKEN" | jq .
    ;;
  chat)
    MODEL="${2:-omni_flash}"
    curl -sS -m 120 -X POST "$BASE/v1/chat/completions" \
      -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
      -d "$(jq -n --arg m "$MODEL" '{model:$m, messages:[{role:"user",content:"reply with exactly: ok"}], max_tokens:16}')" \
      | jq '{status: .error // "ok", content: .choices[0].message.content, usage}'
    ;;
  image)
    MODEL="${2:-nano_banana_2}"
    curl -sS -m 120 -X POST "$BASE/v1/images/generations" \
      -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
      -d "$(jq -n --arg m "$MODEL" '{model:$m, prompt:"a plain blue square on white background", n:1}')" \
      | jq '{status: .error // "ok", first: (.data[0] | if .url then {url_host: (.url | split("/")[2])} else {b64: (.b64_json != null)} end), usage}'
    ;;
  *)
    echo "usage: $0 {models|veo [prompt]|task <id>|chat [model]|image [model]}" >&2
    exit 2
    ;;
esac
