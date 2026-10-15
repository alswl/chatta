#!/bin/sh
# chatta chat quick start: connect this agent session to the bus with no
# questions asked. Safe to rerun — a healthy client is left alone.
#
#   assets/quickstart.sh [--codex] [nick] [role] [--subscribe-channel '#channel']...
#
# Defaults: nick and role come from <git-dir>/irc-agent-identity if it exists,
# otherwise from the repository directory name. Server host/port come from
# CHATTA_CHAT_HOST / CHATTA_CHAT_PORT (127.0.0.1:6667 by default).
set -eu

codex=0
if [ "${1:-}" = --codex ]; then codex=1; shift; fi
nick=
role=
case "${1:-}" in ''|--*) ;; *) nick=$1; shift ;; esac
case "${1:-}" in ''|--*) ;; *) role=$1; shift ;; esac
validate_options() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --subscribe-channel)
        [ "$#" -ge 2 ] && [ "$codex" = 1 ] || return 1
        case "$2" in \#?*) ;; *) return 1 ;; esac
        shift 2 ;;
      --mention-current-identity)
        [ "$codex" = 1 ] || return 1
        shift ;;
      *) return 1 ;;
    esac
  done
}
validate_options "$@" || { echo "usage: quickstart.sh [--codex] [nick] [role] [--subscribe-channel '#channel']... [--mention-current-identity]" >&2; exit 2; }
chatta_bin=$(command -v "${CHATTA_BIN:-chatta}") || { echo "Chatta executable not found; set CHATTA_BIN to the installed or candidate executable" >&2; exit 1; }
chatta() { "$chatta_bin" "$@"; }
echo "binary   $chatta_bin"
if [ "$codex" = 1 ]; then
  [ -n "${CODEX_THREAD_ID:-}" ] || { echo "CODEX_THREAD_ID is unavailable; run --codex inside the target Codex conversation" >&2; exit 1; }
  help=$(chatta chat codex bind --help) || exit 1
  case "$help" in *--if-needed*) ;; *) echo "Selected Chatta lacks Codex startup support; set CHATTA_BIN to a compatible build" >&2; exit 1 ;; esac
fi

host=${CHATTA_CHAT_HOST:-127.0.0.1}
port=${CHATTA_CHAT_PORT:-6667}
irc_home=${CHATTA_IRC_ADMIN_HOME:-$HOME/.irc-agent}
skill_assets=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
repo=$(basename "$repo_root" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-')
repo=${repo%-}

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
  # a stale transport client holding the nick, or a broken client another live session left
  # behind. Clear it and reconnect — forcing is safe precisely because the
  # health check just failed, so nothing that works is being taken away.
  chatta chat session stop --force >/dev/null 2>&1 || true
  err=$(mktemp)
  # --takeover because stop --force leaves state.json's owner in place: if that
  # owner is another live session, start refuses until told otherwise. Retrying
  # a nick the server has not released yet is session start's own job.
  if chatta chat session start "$nick" "$role" --takeover >/dev/null 2>"$err"; then fresh=1; fi
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
join_subscriptions() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --subscribe-channel)
        chatta chat channel join "$2" >/dev/null
        echo "channel  joined $2 for Codex subscription"
        shift 2 ;;
      *) shift ;;
    esac
  done
}
join_subscriptions "$@"
if [ "$fresh" = 1 ]; then
  chatta chat message send "[HELLO] ${nick} -> all: I am on ${repo} (#${repo}); ${role}." >/dev/null
fi

if [ "$codex" = 1 ]; then
  chatta chat codex bind --current --if-needed "$@"
  chatta chat codex status
  echo "next     Codex push configured; reply through the Chatta CLI, not only in this terminal"
elif [ -n "${CODEX_THREAD_ID:-}" ]; then
  echo "next     Codex checkpoints: chatta chat inbox read (cannot wake an idle session); use --codex for push"
else
  echo "next     Claude Code Monitor: chatta chat inbox watch; other runtimes use inbox checkpoints"
fi
