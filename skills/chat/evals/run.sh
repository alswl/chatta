#!/usr/bin/env bash
# evals/run.sh -- run the chat skill's behaviour cases, capture each transcript
# and put it through the deterministic checks.
#
# Usage:
#   evals/run.sh                # every case
#   evals/run.sh 1 4 7          # only these ids
#   ALLOW_STALE=1 evals/run.sh  # run even though the deployed copy is stale
#
# Output lands in evals/runs/<timestamp>/:
#   case-<id>.txt   the full reply
#   report.txt      the summary
#
# Everything runs on an isolated bus -- port 6767 with its own CHATTA_CHAT_HOME
# and irc directory -- so the real 127.0.0.1:6667 bus is never touched.
#
# L1 (what was and wasn't run) is decided by check_skill.py,
# check_quickstart.sh and check_transcript.py. L2 (quality) is scored by a
# person or a grader model against rubric.md and the expectations in
# evals.json -- this script does not score it for you.

set -uo pipefail

cd "$(dirname "$0")/.."
SKILL_DIR=$PWD
EVALS=evals/evals.json
OUT="evals/runs/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUT"

command -v claude >/dev/null || { echo "claude CLI not found"; exit 1; }
command -v jq >/dev/null || { echo "jq is required to read evals.json"; exit 1; }
command -v chatta >/dev/null || { echo "chatta must be on PATH"; exit 1; }

# ---- The deployed copy must match this repository, or the run measures an
# ---- older version of the skill --------------------------------------------

deployed=$(readlink -f "$HOME/.claude/skills/chat" 2>/dev/null || true)
if [ -n "$deployed" ] && [ -d "$deployed" ]; then
  if ! diff -rq -x meta.json -x evals -x runs "$deployed" "$SKILL_DIR" >/dev/null 2>&1; then
    echo "the deployed skill differs from this repository: $deployed"
    echo "sync it first, or the run measures the older version:"
    echo "  rsync -a --delete --exclude evals '$SKILL_DIR/' '$deployed/'"
    [ "${ALLOW_STALE:-0}" = 1 ] || exit 1
    echo "ALLOW_STALE=1, continuing."
  fi
fi

# ---- Isolated bus -----------------------------------------------------------

export CHATTA_CHAT_HOST=127.0.0.1
export CHATTA_CHAT_PORT=6767
export CHATTA_IRC_ADMIN_HOME="$PWD/$OUT/irc"
export CHATTA_CHAT_HOME="$PWD/$OUT/client"
mkdir -p "$CHATTA_IRC_ADMIN_HOME" "$CHATTA_CHAT_HOME"

cleanup() {
  chatta chat session stop --force >/dev/null 2>&1
  pkill -f "ngircd-$CHATTA_CHAT_PORT.conf" >/dev/null 2>&1
  [ -n "${FAKE_OWNER:-}" ] && kill "$FAKE_OWNER" >/dev/null 2>&1
  return 0
}
trap cleanup EXIT

start_test_server() {
  nc -z "$CHATTA_CHAT_HOST" "$CHATTA_CHAT_PORT" 2>/dev/null && return 0
  local conf="$CHATTA_IRC_ADMIN_HOME/ngircd-$CHATTA_CHAT_PORT.conf"
  sed -E -e "s/^([[:space:]]*)Listen = .*/\1Listen = $CHATTA_CHAT_HOST/" \
         -e "s/^([[:space:]]*)Ports = .*/\1Ports = $CHATTA_CHAT_PORT/" \
         assets/ngircd-agent-chat.conf > "$conf"
  nohup ngircd --nodaemon --config "$conf" >>"$CHATTA_IRC_ADMIN_HOME/ngircd.log" 2>&1 &
  for _ in $(seq 1 20); do
    nc -z "$CHATTA_CHAT_HOST" "$CHATTA_CHAT_PORT" 2>/dev/null && return 0
    sleep 0.25
  done
  echo "the test ngircd did not come up; see $CHATTA_IRC_ADMIN_HOME/ngircd.log"
  return 1
}

# Every branch ends deterministically: a pkill with nothing to kill exits 1,
# which would otherwise read as a failed setup.
setup_case() {
  chatta chat session stop --force >/dev/null 2>&1
  rm -rf "$CHATTA_CHAT_HOME"
  mkdir -p "$CHATTA_CHAT_HOME"
  case "$1" in
    clean)                  # no server either: quickstart has to start one
      pkill -f "ngircd-$CHATTA_CHAT_PORT.conf" >/dev/null 2>&1 || true ;;
    connected)
      start_test_server || return 1
      assets/quickstart.sh >/dev/null || return 1 ;;
    broken-client)
      start_test_server || return 1
      assets/quickstart.sh >/dev/null || return 1
      pkill -f _supervise >/dev/null 2>&1 || true ;;
    no-ngircd)             # a real IRC client is built in now, so the only
                            # installable dependency left is ngircd itself
      pkill -f "ngircd-$CHATTA_CHAT_PORT.conf" >/dev/null 2>&1 || true
      CASE_PATH=""
      IFS=: read -ra dirs <<< "$PATH"
      for d in "${dirs[@]}"; do
        [ -x "$d/ngircd" ] && continue
        CASE_PATH="${CASE_PATH:+$CASE_PATH:}$d"
      done ;;
    peer-message)           # connected, with a peer DM and channel line waiting
      start_test_server || return 1
      assets/quickstart.sh >/dev/null || return 1
      mine=$(jq -r .nick "$CHATTA_CHAT_HOME/state.json")
      peer_home=$PWD/$OUT/peer
      mkdir -p "$peer_home"
      sleep 900 & FAKE_OWNER=$!
      CHATTA_CHAT_TEST_OWNER_PID=$FAKE_OWNER CHATTA_CHAT_HOME=$peer_home \
        chatta chat session start pola 'photo-cull side' >/dev/null || return 1
      CHATTA_CHAT_HOME=$peer_home chatta chat message send \
        "[HELLO] Pola -> all: I am on photo-cull." >/dev/null || return 1
      CHATTA_CHAT_HOME=$peer_home chatta chat message direct "$mine" \
        "[ASK] Pola -> ${mine}: is the parser interface settled?" >/dev/null || return 1
      sleep 2
      CHATTA_CHAT_HOME=$peer_home chatta chat session stop --force >/dev/null 2>&1 || true
      kill "$FAKE_OWNER" >/dev/null 2>&1 || true
      unset FAKE_OWNER ;;
    foreign-healthy-client) # another live session holds a healthy client here
      start_test_server || return 1
      sleep 900 & FAKE_OWNER=$!
      CHATTA_CHAT_TEST_OWNER_PID=$FAKE_OWNER \
        chatta chat session start peerowner 'another live session' >/dev/null || return 1 ;;
    *)
      echo "unknown setup: $1"; return 1 ;;
  esac
  return 0
}

if [ $# -gt 0 ]; then ids="$*"; else ids=$(jq -r '.evals[].id' "$EVALS"); fi

echo "=== L1a static checks ===" | tee -a "$OUT/report.txt"
python3 evals/check_skill.py 2>&1 | tee -a "$OUT/report.txt"

echo "" | tee -a "$OUT/report.txt"
echo "=== L1c quickstart scenarios ===" | tee -a "$OUT/report.txt"
bash evals/check_quickstart.sh 2>&1 | grep -E "^S|FAIL|check_quickstart:" | tee -a "$OUT/report.txt"

pass=0; fail=0
for id in $ids; do
  prompt=$(jq -r --argjson i "$id" '.evals[] | select(.id == $i) | .prompt' "$EVALS")
  setup=$(jq -r --argjson i "$id" '.evals[] | select(.id == $i) | .setup' "$EVALS")
  [ -n "$prompt" ] && [ "$prompt" != null ] || { echo "no case with id $id"; continue; }

  echo "" | tee -a "$OUT/report.txt"
  echo "=== case $id ($setup) ===" | tee -a "$OUT/report.txt"
  CASE_PATH=""
  setup_case "$setup" || { echo "  [error] setup failed, skipping" | tee -a "$OUT/report.txt"; continue; }

  # stream-json so the transcript carries the actual tool calls: a reply that
  # only *mentions* a command must not read as having run it. Bash and the
  # chat commands are allowed explicitly -- a non-interactive session has no
  # way to ask for approval, and a blocked run looks like a passing one.
  PATH="${CASE_PATH:-$PATH}" claude -p "$prompt" \
    --output-format stream-json --verbose \
    --allowedTools "Bash" "Monitor" "Read" "Glob" "Grep" "Skill" \
    < /dev/null > "$OUT/case-$id.json" 2>&1
  python3 evals/flatten_transcript.py "$OUT/case-$id.json" > "$OUT/case-$id.txt"
  if python3 evals/check_transcript.py "$id" "$OUT/case-$id.txt" | tee -a "$OUT/report.txt"; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi
done

echo "" | tee -a "$OUT/report.txt"
echo "L1 deterministic checks: pass=$pass fail=$fail -- details in $OUT/report.txt" | tee -a "$OUT/report.txt"
echo "L2 quality: score each expectation against evals/rubric.md by hand or with a grader model"
