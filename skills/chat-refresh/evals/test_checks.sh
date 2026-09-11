#!/bin/sh
set -eu

cd "$(dirname "$0")"

if CHAT_REFRESH_PRIVACY_DENYLIST=Atlas python3 check_privacy.py >/dev/null 2>&1; then
  echo "[fail] privacy checker accepted an injected private term"
  exit 1
fi

python3 check_privacy.py

for id in 1 2 3 4 5 6; do
  python3 check_transcript.py "$id" "fixtures/case-$id.txt"
done

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

sed 's/chatta chat inbox read/chatta chat inbox watch/' fixtures/case-1.txt > "$tmp_dir/invalid-monitor.txt"
if python3 check_transcript.py 1 "$tmp_dir/invalid-monitor.txt" >/dev/null 2>&1; then
  echo "[fail] checker accepted Monitor/watch behavior"
  exit 1
fi

sed 's#/workspace/skills/chat/assets/quickstart.sh#python3 /workspace/skills/chat/scripts/agent_chat.py poll#' \
  fixtures/case-1.txt > "$tmp_dir/invalid-legacy.txt"
if python3 check_transcript.py 1 "$tmp_dir/invalid-legacy.txt" >/dev/null 2>&1; then
  echo "[fail] checker accepted the legacy Python wrapper"
  exit 1
fi

echo "chat-refresh eval checks: PASS"
