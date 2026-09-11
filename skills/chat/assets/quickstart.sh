#!/bin/sh
# chatta chat quick start: connect this agent session to the bus with no
# questions asked. Safe to rerun — a healthy client is left alone.
#
#   assets/quickstart.sh [nick] [role]
#
# Defaults: nick and role come from <git-dir>/irc-agent-identity if it exists,
# otherwise from the repository directory name. Server host/port come from
# CHATTA_CHAT_HOST / CHATTA_CHAT_PORT (127.0.0.1:6667 by default).
set -eu

host=${CHATTA_CHAT_HOST:-127.0.0.1}
port=${CHATTA_CHAT_PORT:-6667}
irc_home=${CHATTA_IRC_ADMIN_HOME:-$HOME/.irc-agent}
skill_assets=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
repo=$(basename "$repo_root" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-')
repo=${repo%-}

nick=${1:-}
role=${2:-}
identity=$(git rev-parse --git-dir 2>/dev/null || echo .git)/irc-agent-identity
if [ -r "$identity" ]; then
  [ -n "$nick" ] || nick=$(sed -n 's/^name=//p' "$identity" | head -1 | tr '[:upper:]' '[:lower:]')
  [ -n "$role" ] || role=$(sed -n 's/^role=//p' "$identity" | head -1)
fi
[ -n "$nick" ] || nick=$repo
[ -n "$role" ] || role="works on ${repo}"

# 1. Server. Start one only if nothing answers on the agreed port.
if ! nc -z "$host" "$port" 2>/dev/null; then
  command -v ngircd >/dev/null || { echo "ngircd is not installed; ask the user to run: brew install ngircd" >&2; exit 1; }
  mkdir -p "$irc_home"
  # The bundled config listens on 127.0.0.1:6667; a different host/port gets
  # its own rendered copy so CHATTA_CHAT_HOST/PORT actually take effect.
  conf=$irc_home/ngircd.conf
  [ "$host:$port" = "127.0.0.1:6667" ] || conf=$irc_home/ngircd-$port.conf
  if [ ! -f "$conf" ]; then
    sed -E -e "s/^([[:space:]]*)Listen = .*/\1Listen = $host/" \
           -e "s/^([[:space:]]*)Ports = .*/\1Ports = $port/" \
           "$skill_assets/ngircd-agent-chat.conf" > "$conf"
  fi
  nohup ngircd --nodaemon --config "$conf" >>"$irc_home/ngircd.log" 2>&1 &
  i=0
  while [ "$i" -lt 20 ]; do
    nc -z "$host" "$port" 2>/dev/null && break
    i=$((i + 1))
    sleep 0.25
  done
  nc -z "$host" "$port" 2>/dev/null || { echo "chat server did not come up; see $irc_home/ngircd.log" >&2; exit 1; }
  echo "server   started on $host:$port"
else
  echo "server   already up on $host:$port"
fi

# 2. Session. A healthy client on this path is reused untouched, so a Monitor
#    watch already streaming from it keeps running and no nick changes hands.
fresh=0
if chatta chat session status >/dev/null 2>&1; then
  echo "session  reusing the healthy client on this path"
else
  # Nothing here works: this session's own leftover client, a dead supervisor,
  # a stray ii holding the nick, or a broken client another live session left
  # behind. Clear it and reconnect — forcing is safe precisely because the
  # health check just failed, so nothing that works is being taken away. The
  # server releases the old nick asynchronously, so a rejected start is retried.
  chatta chat session stop --force >/dev/null 2>&1 || true
  err=$(mktemp)
  attempt=0
  while [ "$attempt" -lt 4 ]; do
    # --takeover because stop --force leaves state.json's owner in place: if
    # that owner is another live session, start refuses until told otherwise.
    if chatta chat session start "$nick" "$role" --takeover >/dev/null 2>"$err"; then fresh=1; break; fi
    # start sometimes reports "nick already in use" after the client did come
    # up; the health check is the authority, not the message.
    if chatta chat session status >/dev/null 2>&1; then fresh=1; break; fi
    attempt=$((attempt + 1))
    sleep 3
  done
  if [ "$fresh" != 1 ]; then
    cat "$err" >&2
    rm -f "$err"
    echo "quick start failed; see the message above (a nick held by someone else → rerun with a distinct nick)" >&2
    exit 1
  fi
  rm -f "$err"
  echo "session  connected as $nick"
fi

# 3. Project channel, then the one arrival message the lobby budget allows.
chatta chat channel join "$repo" >/dev/null
echo "channel  joined #agents #${repo}"
if [ "$fresh" = 1 ]; then
  chatta chat message send "[HELLO] ${nick} -> all: I am on ${repo} (#${repo}); ${role}." >/dev/null
fi

echo "next     Monitor: chatta chat inbox watch"
