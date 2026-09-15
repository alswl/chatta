#!/usr/bin/env bash
# evals/check_quickstart.sh -- scenario regression for assets/quickstart.sh (L1c).
#
# Eight scenarios on an isolated bus: port 6768 and a private client home
# under a temporary directory, so the real 127.0.0.1:6667 bus is never touched.
# Each scenario asserts what the script did, needs no model, and runs in about
# a minute -- run it after every change to quickstart.sh.
#
# Usage:
#   evals/check_quickstart.sh                          # every scenario
#   QS=/path/to/variant.sh evals/check_quickstart.sh   # test a modified copy
#
# Exit status: 0 = all green; 1 = at least one FAIL.
set -uo pipefail
SKILL=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
QS=${QS:-$SKILL/assets/quickstart.sh}
tmp=${TMPDIR:-/tmp}
BASE=${tmp%/}/chatta-chat-eval   # trailing slash stripped: the built-in client receives the normalized path
export CHATTA_CHAT_HOST=127.0.0.1
export CHATTA_CHAT_PORT=6768
export CHATTA_IRC_ADMIN_HOME=$BASE/irc
export CHATTA_CHAT_HOME=$BASE/client
pass=0; fail=0
ok(){ echo "  PASS $1"; pass=$((pass+1)); }
no(){ echo "  FAIL $1 -- $2"; fail=$((fail+1)); }

nuke(){
  chatta chat session stop --force >/dev/null 2>&1
  pkill -f "$BASE/irc/ngircd" >/dev/null 2>&1   # any conf under the test home, however named
  pkill -f "$BASE.*_supervise" >/dev/null 2>&1
  rm -rf "$BASE"; mkdir -p "$CHATTA_IRC_ADMIN_HOME" "$CHATTA_CHAT_HOME"
}
server_up(){ nc -z 127.0.0.1 6768 2>/dev/null; }
# clientpid: the supervisor process IS the client now -- it holds the IRC
# socket itself rather than spawning a separate transport process.
clientpid(){ pgrep -f "$BASE.*_supervise" | head -1; }

echo "S1 cold start (no server, no client)"
nuke
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
[ $rc -eq 0 ] || no S1.exit "rc=$rc: $out"
[ $rc -eq 0 ] && { server_up && ok S1.server || no S1.server "nothing on 6768"; }
[ $rc -eq 0 ] && { chatta chat session status >/dev/null 2>&1 && ok S1.health || no S1.health "unhealthy after start"; }
[ $rc -eq 0 ] && ok S1.exit
grep -q '"#chatta"' "$CHATTA_CHAT_HOME/state.json" 2>/dev/null && ok S1.project-channel || no S1.project-channel "project channel not joined"
echo "$out" | grep -q "connected as" && ok S1.one-hello || no S1.one-hello "did not take the fresh (HELLO-sending) path"

echo "S2 idempotent rerun (client untouched, no second HELLO)"
before=$(clientpid)
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
after=$(clientpid)
[ $rc -eq 0 ] && ok S2.exit || no S2.exit "rc=$rc: $out"
[ -n "$before" ] && [ "$before" = "$after" ] && ok S2.same-client || no S2.same-client "managed client pid $before -> $after"
echo "$out" | grep -q reusing && ok S2.reuse-path || no S2.reuse-path "did not take the reuse branch"
echo "$out" | grep -q "connected as" && no S2.no-second-hello "took the fresh path again on a rerun" || ok S2.no-second-hello

echo "S3 recover from a dead supervisor"
pkill -f _supervise >/dev/null 2>&1; sleep 1
chatta chat session status >/dev/null 2>&1 && no S3.precondition "still healthy after kill" || ok S3.precondition
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
[ $rc -eq 0 ] && ok S3.exit || no S3.exit "rc=$rc: $out"
chatta chat session status >/dev/null 2>&1 && ok S3.health || no S3.health "not healthy after recovery"

echo "S4 foreign healthy client is reused, not killed"
nuke
conf=$CHATTA_IRC_ADMIN_HOME/ngircd-6768.conf
sed -E -e "s/^([[:space:]]*)Listen = .*/\1Listen = 127.0.0.1/" -e "s/^([[:space:]]*)Ports = .*/\1Ports = 6768/" \
  "$SKILL/assets/ngircd-agent-chat.conf" > "$conf"
nohup ngircd --nodaemon --config "$conf" >>"$CHATTA_IRC_ADMIN_HOME/ngircd.log" 2>&1 &
for _ in $(seq 1 20); do server_up && break; sleep 0.25; done
sleep 900 & FAKE=$!
CHATTA_CHAT_TEST_OWNER_PID=$FAKE chatta chat session start peerowner 'another live session' >/dev/null 2>&1
foreign=$(clientpid)
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
[ $rc -eq 0 ] && ok S4.exit || no S4.exit "rc=$rc: $out"
[ -n "$foreign" ] && [ "$foreign" = "$(clientpid)" ] && ok S4.not-killed || no S4.not-killed "foreign client $foreign -> $(clientpid)"
kill $FAKE >/dev/null 2>&1

echo "S5 custom port is rendered into the ngircd config"
nuke
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
[ $rc -eq 0 ] && ok S5.exit || no S5.exit "rc=$rc: $out"
grep -q "Ports = 6768" "$CHATTA_IRC_ADMIN_HOME/ngircd-6768.conf" 2>/dev/null && ok S5.rendered || no S5.rendered "conf still on 6667"

echo "S6 nick comes from the identity file when present"
nuke
idf=$(git -C "$SKILL" rev-parse --git-dir)/irc-agent-identity
printf 'name=Ablato\nrepo=chatta\nrole=ablation probe\n' > "$idf"
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
rm -f "$idf"
[ $rc -eq 0 ] && ok S6.exit || no S6.exit "rc=$rc: $out"
chatta chat channel members 2>/dev/null | grep -qi '^ablato' && ok S6.nick || no S6.nick "nick is $(chatta chat channel members 2>/dev/null | head -1)"

echo "S7 ngircd missing: refuse, tell the user, install nothing"
nuke
# Drop any PATH entry that holds ngircd; chatta and nc stay reachable.
clean_path=""
IFS=: read -ra dirs <<< "$PATH"
for d in "${dirs[@]}"; do
  [ -x "$d/ngircd" ] && continue
  clean_path="${clean_path:+$clean_path:}$d"
done
out=$(cd "$SKILL" && PATH="$clean_path" sh "$QS" 2>&1); rc=$?
[ $rc -ne 0 ] && ok S7.exit || no S7.exit "exited 0 with ngircd missing"
echo "$out" | grep -q "brew install ngircd" && ok S7.tells-user || no S7.tells-user "never named what to install: $out"
server_up && no S7.no-server "started a server anyway" || ok S7.no-server

echo "S8 another live session left a broken client here: take it over"
nuke
conf=$CHATTA_IRC_ADMIN_HOME/ngircd-6768.conf
sed -E -e "s/^([[:space:]]*)Listen = .*/\1Listen = 127.0.0.1/" -e "s/^([[:space:]]*)Ports = .*/\1Ports = 6768/" \
  "$SKILL/assets/ngircd-agent-chat.conf" > "$conf"
nohup ngircd --nodaemon --config "$conf" >>"$CHATTA_IRC_ADMIN_HOME/ngircd.log" 2>&1 &
for _ in $(seq 1 20); do server_up && break; sleep 0.25; done
sleep 900 & FAKE=$!
CHATTA_CHAT_TEST_OWNER_PID=$FAKE chatta chat session start peerowner 'another live session' >/dev/null 2>&1
pkill -9 -f "$BASE.*_supervise" >/dev/null 2>&1   # SIGKILL: the supervisor is the whole client, so this drops the connection outright
sleep 1
chatta chat session status >/dev/null 2>&1 && no S8.precondition "client is still healthy, scenario did not hold" || ok S8.precondition
out=$(cd "$SKILL" && sh "$QS" 2>&1); rc=$?
[ $rc -eq 0 ] && ok S8.exit || no S8.exit "rc=$rc: $out"
chatta chat session status >/dev/null 2>&1 && ok S8.health || no S8.health "still unhealthy after the takeover"
kill $FAKE >/dev/null 2>&1

nuke
echo "check_quickstart: pass=$pass fail=$fail"
[ $fail -eq 0 ]
