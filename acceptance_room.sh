#!/usr/bin/env bash
set -euo pipefail

# AIChatMatrix acceptance room script (manual-action coverage)
# Verifies: ADD_AI / REMOVE_AI / private-chat / STOP_DISCUSSION / NO_OP cleanup
#
# Usage:
#   PROVIDER_ID=<provider-id> ./acceptance_room.sh
# Optional:
#   BASE_URL=http://localhost:8080
#   TIMEOUT_SEC=180

BASE_URL="${BASE_URL:-http://localhost:8080}"
PROVIDER_ID="${PROVIDER_ID:-}"
TIMEOUT_SEC="${TIMEOUT_SEC:-180}"

if [[ -z "$PROVIDER_ID" ]]; then
  echo "[ERR] PROVIDER_ID is required"
  echo "      Example: PROVIDER_ID=your-provider-id ./acceptance_room.sh"
  exit 1
fi

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "[ERR] missing command: $1"; exit 1; }
}
need_cmd curl
need_cmd python3

json_eval() {
  # json_eval '<json>' '<python_expr_using_obj>'
  local json="$1"
  local expr="$2"
  python3 -c 'import json,sys; obj=json.load(sys.stdin); expr=sys.argv[1]; print(eval(expr, {"__builtins__": {}}, {"obj": obj}))' "$expr" <<<"$json"
}

api_post() {
  local path="$1"
  local payload="$2"
  curl -sS -X POST "$BASE_URL$path" -H 'Content-Type: application/json' -d "$payload"
}

room_json() {
  curl -sS "$BASE_URL/api/rooms/$ROOM_ID"
}

msgs_json() {
  curl -sS "$BASE_URL/api/rooms/$ROOM_ID/messages"
}

push_syscmd() {
  local text="$1"
  api_post "/api/rooms/$ROOM_ID/syscmd" "{\"text\":\"$text\"}" >/dev/null
}

echo "[1/9] Create room"
ROOM_PAYLOAD='{"name":"验收房间-动作覆盖","topic":"验证新增/移除/私聊/停房","rules":"裁判需按系统要求执行动作，禁止输出 [NO_OP]","max_messages":80}'
CREATE_JSON=$(api_post "/api/rooms" "$ROOM_PAYLOAD")
ROOM_ID=$(json_eval "$CREATE_JSON" "obj.get('id','')")
if [[ -z "$ROOM_ID" ]]; then
  echo "[ERR] failed creating room: $CREATE_JSON"
  exit 1
fi
echo "      room_id=$ROOM_ID"

add_agent() {
  local name="$1"
  local personality="$2"
  local extra_json="${3:-}"
  local payload
  if [[ -n "$extra_json" ]]; then
    payload=$(printf '{"name":"%s","provider_id":"%s","personality":"%s",%s}' "$name" "$PROVIDER_ID" "$personality" "$extra_json")
  else
    payload=$(printf '{"name":"%s","provider_id":"%s","personality":"%s"}' "$name" "$PROVIDER_ID" "$personality")
  fi
  api_post "/api/rooms/$ROOM_ID/agents" "$payload" >/dev/null
}

echo "[2/9] Add agents (manual-like baseline)"
add_agent "甲方" "偏重商业目标、成本与上线节奏。收到定向私聊要求时使用 @目标名 内容。"
add_agent "乙方" "偏重技术可行性、实现代价与风险。收到定向私聊要求时使用 @目标名 内容。"
add_agent "观察员1" "仅在必要时给简短观察。收到定向私聊要求时使用 @目标名 内容。" '"is_observer":true'
add_agent "总裁判" "你是裁判：必须执行动作验收。按系统要求完成 [ADD_AI]、[REMOVE_AI]、定向私聊调度，并在完成后输出 STOP_DISCUSSION。禁止输出 [NO_OP]。" '"is_referee":true'

echo "[3/9] Start room"
START_JSON=$(api_post "/api/rooms/$ROOM_ID/start" '{}')
echo "      $START_JSON"

echo "[4/9] Stage-1 force ADD_AI + private-chat directive"
push_syscmd "给总裁判：请立即执行 [ADD_AI] 临时审计员 | 关注验证流程与一致性 | 验收新增动作。然后下发定向私聊指令：甲方使用 @总裁判 内容；乙方使用 @甲方 内容；观察员1使用 @乙方 内容。禁止输出 [NO_OP]。"

START_TS=$(date +%s)
ADDED=0
REMOVED=0
PRIVATE_CHAT=0
STOPPED=0
NOOP_LEAK=0

last_repush_add=0
last_repush_remove=0
last_repush_stop=0

echo "[5/9] Poll and drive remaining actions (timeout=${TIMEOUT_SEC}s)"
while true; do
  now=$(date +%s)
  elapsed=$((now-START_TS))
  if (( elapsed > TIMEOUT_SEC )); then
    break
  fi

  ROOM_STATE=$(room_json)
  MSGS=$(msgs_json)

  IS_RUNNING=$(json_eval "$ROOM_STATE" "bool(obj.get('is_running', False))")
  HAS_ADDED_MSG=$(json_eval "$MSGS" "any('裁判新增机器人：' in (m.get('content','')) for m in obj)")
  HAS_REMOVED_MSG=$(json_eval "$MSGS" "any('裁判移除机器人：' in (m.get('content','')) for m in obj)")
  HAS_PRIVATE_MSG=$(json_eval "$MSGS" "any('【私聊公开】' in (m.get('content','')) for m in obj)")
  HAS_NOOP=$(json_eval "$MSGS" "any('[NO_OP]' in (m.get('content','')) for m in obj)")

  [[ "$HAS_ADDED_MSG" == "True" ]] && ADDED=1
  [[ "$HAS_REMOVED_MSG" == "True" ]] && REMOVED=1
  [[ "$HAS_PRIVATE_MSG" == "True" ]] && PRIVATE_CHAT=1

  if [[ "$HAS_NOOP" == "True" ]]; then
    NOOP_LEAK=1
    break
  fi

  if [[ "$IS_RUNNING" == "False" ]]; then
    STOPPED=1
    break
  fi

  # Stage-2: ensure REMOVE_AI happens after ADD_AI
  if (( ADDED == 1 && REMOVED == 0 )); then
    if (( now - last_repush_remove >= 8 )); then
      push_syscmd "给总裁判：请立即执行 [REMOVE_AI] 临时审计员，并再次下发定向私聊指令（甲方@总裁判，乙方@甲方，观察员1@乙方）。"
      last_repush_remove=$now
    fi
  fi

  # Stage-1补偿：如果新增未触发，重复更强指令
  if (( ADDED == 0 )); then
    if (( now - last_repush_add >= 8 )); then
      push_syscmd "给总裁判：这是验收要求，请现在立刻执行 [ADD_AI] 临时审计员 | 验收角色 | 验收新增动作；随后立即下发一次定向私聊指令。"
      last_repush_add=$now
    fi
  fi

  # Stage-3: once add/remove/private all hit, push stop command
  if (( ADDED == 1 && REMOVED == 1 && PRIVATE_CHAT == 1 && STOPPED == 0 )); then
    if (( now - last_repush_stop >= 8 )); then
      push_syscmd "给总裁判：新增、移除、私聊验收已完成。请立即输出 STOP_DISCUSSION 结束房间。"
      last_repush_stop=$now
    fi
  fi

  sleep 2
done

echo "[6/9] Summary"
echo "      ADDED=$ADDED REMOVED=$REMOVED PRIVATE_CHAT=$PRIVATE_CHAT STOPPED=$STOPPED NOOP_LEAK=$NOOP_LEAK"

if (( NOOP_LEAK == 1 )); then
  echo "[FAIL] found leaked [NO_OP] in room messages"
  echo "       Messages: $BASE_URL/api/rooms/$ROOM_ID/messages"
  exit 2
fi

if (( ADDED == 0 )); then
  echo "[FAIL] ADD_AI action was not observed"
  exit 3
fi
if (( REMOVED == 0 )); then
  echo "[FAIL] REMOVE_AI action was not observed"
  exit 4
fi
if (( PRIVATE_CHAT == 0 )); then
  echo "[FAIL] private-chat action was not observed (no 【私聊公开】 message)"
  exit 5
fi
if (( STOPPED == 0 )); then
  echo "[FAIL] room did not stop in time (STOP_DISCUSSION path not completed)"
  exit 6
fi

echo "[7/9] PASS: all required actions observed"

echo "[8/9] Debug URLs"
echo "      Room detail: $BASE_URL/api/rooms/$ROOM_ID"
echo "      Messages   : $BASE_URL/api/rooms/$ROOM_ID/messages"

echo "[9/9] done"
