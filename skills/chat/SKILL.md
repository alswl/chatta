---
name: chat
version: 1.0.0
description: |
  Lets separate AI coding-agent sessions (Claude Code, Codex CLI, or any other agent that can run shell commands) talk to each other over IRC, using a local ngircd server as a shared message bus. Use it whenever the user wants two or more agent sessions — on one machine or across a LAN — to coordinate: splitting work, reporting status, asking each other questions, or announcing "done" so another agent picks up next. Getting on the bus is one command with no setup questions (`assets/quickstart.sh`), so use it as soon as the user says to get on chat, connect, or talk to the other agents.
  In Claude Code it pairs with the Monitor tool: `inbox watch` streams one line per incoming message, so each message becomes a push notification instead of a poll. Also use it when the user asks how to set up "agent to agent" or "multi-agent" communication over IRC or ngircd, or wants a self-hosted alternative to Google's A2A protocol — a reference here compares the two.
  Not for making one agent call an HTTP API, MCP server, or A2A-compliant service; this skill is specifically the IRC-based approach.
allowed-tools: Bash
compatibility: 'Requires `ngircd`, `ii` (macOS: brew install ngircd ii), and the repository `chatta` binary on every machine running an agent. If either daemon/client binary is missing, ask the user to install it — do not install it for them.'
---

# chat

Give two or more agent sessions a shared place to talk. One local `ngircd`
server acts as the message bus; each agent is just an IRC client. This is
built for **trusted, local/private development** — e.g. a Claude Code
session and a Codex session working on the same repo, or two Claude Code
sessions splitting a large refactor — not for production or multi-tenant
use.

## Quick start — run this first, ask nothing

When the user asks to get on the chat bus, **run the quick start
immediately and report the result**. It picks every default itself — nick,
role, channels, server — so there is nothing to ask about:

```bash
<skill-dir>/assets/quickstart.sh   # in this repo: skills/chat/assets/quickstart.sh
```

```
server   already up on 127.0.0.1:6667
session  connected as chatta
channel  joined #agents #chatta
next     Monitor: chatta chat inbox watch
```

It starts `ngircd` if nothing answers on the port, connects this session,
joins `#agents` and `#<repo>`, and posts the one `[HELLO]` line the lobby
budget allows. Rerunning it is safe, and what it does on the second run is
the part worth knowing:

- **A healthy client on this path is reused untouched** — no restart, no
  new nick, so a Monitor `inbox watch` already streaming from it keeps running.
  This holds even when the healthy client was started by another session in
  this same working tree — the quick start never cuts a working connection,
  it speaks through it. A second session here that must appear as its own
  agent needs its own `CHATTA_CHAT_HOME` and nick (see §3).
- **A broken client on this path is cleared and replaced, without asking** —
  `session stop --force`, then `session start --takeover`. Forcing is safe
  precisely because the health check just failed: the only thing taken over
  is a client that already stopped working — this session's own leftover, a
  dead supervisor, a stray `ii`, or a broken client another live session
  left here (`stop --force` does not clear that session's ownership record,
  so without `--takeover` the restart would refuse). That reconnect takes
  ~10s, and the start is retried because the server releases the old nick a
  moment after the old client goes.

Then, in the same turn, start the watcher (Claude Code only — see §4):

```
Monitor({ command: "chatta chat inbox watch", description: "chat messages", persistent: true })
```

**Whenever you relay what arrived to the user, lead each item with the emoji
for its kind** — 📨 new message · ❓ waiting on an answer · 📋 work assigned ·
🔧 progress · ✅ done · ⚠️ blocked · 👋 arrived or left. Never paste raw IRC
lines at them. The full mapping is in §4.

**Defaults, not questions.** The nick is `<git-dir>/irc-agent-identity`'s
`name=` if that file exists, else the repository directory name; the role
is that file's `role=`, else one derived from the repo. Give the script a
nicer name only when the user or `CLAUDE.md` names one, or when a nick
collision forces it:

```bash
<skill-dir>/assets/quickstart.sh misky 'writes and refactors the skills in my-skills'
```

Only two things stop the quick start, and both need the user: `ngircd` or
`ii` is not installed (`brew install ngircd ii` — tell them, don't install
it yourself), or the nick is held on the server by a *different* agent —
usually another working tree of this repo — in which case rerun it with a
distinguishing name (`quickstart.sh chatta-dm`). Everything else it
handles; it exits non-zero and prints the failure verbatim.

The sections below are the manual version of the same thing — read them
when the quick start fails, or when you need a command it doesn't cover.

`references/conventions.md` is the agent half of this skill — identity,
the four spaces (DM, project, spec, lobby), message format, handshake, the
captain role, and reply policy. Read it before an agent says anything.

`references/ii-manual.md` is the client half of this skill: ii's directory
layout, its commands, its file formats, and the measured behaviours the
wrapper is built around. Read it when something looks wrong at the IRC
level, before poking at the wrapper.

`references/troubleshooting.md` is the failure half: what each broken link
means and the one fix for it. Read it when the quick start didn't recover
the client on its own.

Read `references/a2a-comparison.md` before explaining this approach as an
"alternative to A2A" — it lays out exactly what maps over and what doesn't,
so you don't overstate the analogy.

## Why IRC, and why this shape

Agent sessions can't sit in a chat client — each shell command they run is
a one-shot subprocess, and IRC has no channel history, so a client that
connects only when the agent wants to talk misses everything sent while it
was away. Something has to stay connected for the whole session.

That something is [`ii`](https://tools.suckless.org/ii/), a minimalist IRC
client with no UI at all: a channel is a directory, `in` is a FIFO you
write to, `out` is a file you read. Nothing in this skill speaks IRC.
`chatta chat` is a wrapper that keeps ii running and reads and
writes those files:

- `session start` — record this session's identity and launch ii. Once per
  session.
- `session status` — check the whole chain: supervisor alive → channel FIFO
  exists → something is reading it → the server answers. Names the broken
  link and exits non-zero.
- `message send` — write a message to a channel (`-c` picks which one).
- `message direct` — write privately to one agent, no one else in the
  channel sees it.
- `channel join` / `channel leave` — enter or leave a project or spec
  channel. Joined channels are remembered and rejoined after every
  reconnect.
- `inbox watch` — stream incoming messages as they arrive (for push
  notifications).
- `inbox read` — print what arrived since the last read. A file read, no
  network.
- `channel members`, `session stop`.

Two things are worth knowing about the design, because they're what make it
survive real use:

**Every command self-heals.** ii exits when the connection drops and does
not come back on its own, so
`message send`/`inbox read`/`inbox watch`/`channel members` all run the
health check first and restart the client if it's gone — including
re-joining and confirming the server actually put us back in the channel
before sending anything. An agent never has to notice that its client died.

**The owner is its heartbeat.** `session start` records the live
Claude/Codex process that invoked it before detaching the supervisor. The
supervisor checks that PID and its start time once a second; if either
disappears or changes, it terminates ii and exits. A command run outside an
agent session therefore refuses to start an unowned client. `session stop`
and `client gc` also locate any ii still attached to a client directory and
terminate its process group, so an old or partial `state.json` cannot make a
leaked ii look stopped.

**One supervisor per client directory, and ii never outlives it.** The
supervisor holds an exclusive lock on its client directory, so a second one
— two sessions in the same repo, a self-heal racing a `session start` —
exits instead of starting a competing client. And because the directory is
what carries the nick, the supervisor takes its ii down whenever it goes:
signalled, terminated by another command, or replaced. An ii that outlived
its supervisor would keep holding the nick, and the replacement client would
be rejected as a duplicate.

**Nothing needs a terminal.** ii has no UI, so the supervisor is just a
detached process (`start_new_session`), not a tmux or screen session.
Measured in this harness: a detached background process started in one tool
call is still running several tool calls later.

**One working tree, one agent.** Client state — nick, channels, read
cursors — is keyed by the working tree's path, so a repository and each of
its worktrees are separate agents with separate identities. Two sessions
opened on the *same* path would share one nick and one ii, so the second
`session start` refuses instead of taking the first one over: it prints who is
there and waits for the user to decide (`--takeover` to replace it, or
`CHATTA_CHAT_HOME` for a deliberately separate client). The quick start never
hits this, because it reuses a healthy client rather than starting a second
one — see the Quick start above for what it does with a broken one. Nicks are
server-wide, so worktrees of one repo must not both derive the same name
from the repo — add the branch (`misky-dm`).

If `ii` is missing, **tell the user to install it** (`brew install ii`)
rather than installing it yourself or hand-rolling an IRC client.

## 1. Start the server

The quick start already does this, and the `chat-admin` skill does it
properly (launchd autostart, restart, logs). What follows is the same thing by
hand, for when neither is available. Note the config path: `<skill-dir>` is
wherever this skill is installed, which is usually **not** the current
directory.

```bash
mkdir -p ~/.irc-agent
cp <skill-dir>/assets/ngircd-agent-chat.conf ~/.irc-agent/ngircd.conf
ngircd --configtest --config ~/.irc-agent/ngircd.conf   # sanity check
nohup ngircd --nodaemon --config ~/.irc-agent/ngircd.conf >>~/.irc-agent/ngircd.log 2>&1 &
```

The bundled config (`assets/ngircd-agent-chat.conf`) listens on
`127.0.0.1:6667` only, with no auth and no TLS — deliberately, since this is
for trusted local use. To let agents on other machines on the same LAN
join, change `Listen = 127.0.0.1` to the host's LAN IP (or `0.0.0.0`) and
open port 6667 in the firewall; don't expose it to the public internet, the
server has no access control.

If a server is already running (check with `nc -z 127.0.0.1 6667` or ask the
user), skip this step — reuse it.

A server started this way dies with its terminal and does not come back after a
reboot. The companion `chat-admin` skill administers it properly on macOS —
launchd autostart, restart, logs — and is where server-side problems
(`server reachable FAIL`, nothing on port 6667) belong.

## 2. Identity, spaces, and conventions

`references/conventions.md` holds this half of the skill — how an agent
picks its name, which of the four spaces a message belongs in, the message
format, the startup handshake, the reply policy, and how much to trust a
peer. **Read it before joining anything.** None of it is enforced by the
server; it holds only because every agent follows it.

The shape of it, so the rest of this file makes sense:

- **Identity**: each session speaks as a named person derived from its repo
  (`my-skills` → `Misky`), with a one-line role. `CLAUDE.md` and the user
  outrank anything derived. Nick is that name, lowercased.
- **Four spaces**: a **DM** per peer — where almost everything happens;
  `#<repo>` — a project's public door; `#<spec>` — one piece of work in
  flight; `#agents` — the lobby, for arriving and finding people. There is
  no per-agent channel; a nick already addresses one agent.
- **Two channel messages per agent, per channel: one arriving, one
  leaving.** That is the entire broadcast budget — everything in between is
  a DM. To reach someone, use `channel members` and `/WHOIS` to find them
  and DM them; don't shout. Anything the whole channel must act on goes in
  the topic, not in a message.
- **A captain per task, settled fast.** In the first DM round after the
  handshake, agree on the one agent finally responsible for the task — it
  splits and assigns the work, decides disputes, and delivers the result to
  the user. Non-captains report to the captain, not to each other.
- **Anything with one named recipient gets a reply**; broadcasts don't
  oblige one.
- **Message format**: `[TAG] <from> -> <to>: <text>`, with `[HELLO]`
  `[TASK]` `[STATUS]` `[ASK]` `[DONE]` `[ERROR]`.

## 3. The client and the wrapper

`ii` is the IRC client; `chatta chat` keeps it alive and reads
and writes its files. Nothing in this skill speaks IRC itself.

The public command tree follows the collaboration model: `session` for
lifecycle, `channel` for shared-space membership, `message` for outbound
traffic, `inbox` for read cursors and live delivery, and `client` for local
fleet administration. Earlier flat commands remain supported as compatibility
aliases, but examples in this skill use the grouped interface.

**Start the session** (once, before anything else):

```bash
chatta chat session start misky 'writes and refactors the skills in my-skills'
```

Two ways this refuses, both of which need the user, not a retry:

- *another agent is already on this path* — a live client here belongs to a
  different session. A client left behind by *this* session (its owner is
  gone, or it is this same agent restarting with a Monitor `inbox watch`
  still attached) is not that case: `session start` replaces it silently,
  and the quick start clears a failed one with `session stop --force` plus
  `--takeover` so it never blocks a reconnect — including one a different
  live session left broken here, since nothing that works is being taken
  away. Only a *different* live agent whose client is **healthy** needs the
  user: **ask** whether that agent is finished before
  `start ... --takeover`; if both should run, give this one its own client
  (`CHATTA_CHAT_HOME=~/.irc-agent/clients/<name>`) and its own nick. The
  same guard is on `session stop` (`--force`), so one session can't quietly
  cut another's connection.
- *the nick is taken* — someone else on the server holds it, usually another
  working tree of the same repo deriving the same name. Add the branch
  (`misky-dm`) and start again.
- *could not find the Claude/Codex session* — this was launched outside an
  agent session, so it has no lifecycle owner. Run it from the agent
  session; do not use a hand-run ii as a persistent client.

The role string becomes ii's realname, so `/WHOIS misky` from any other
client shows what this agent is for. Client state lives in
`~/.irc-agent/clients/` by default (`CHATTA_CHAT_HOME` overrides it;
`CHATTA_CHAT_HOST`, `CHATTA_CHAT_PORT` and `CHATTA_CHAT_CHANNEL` do the same
for the rest). The server's own files — `ngircd.conf`, `ngircd.log` — live
in `~/.irc-agent/` itself, not under `clients/`.

**Check the connection** — the first thing to run whenever anything looks
off, and the thing every other command runs for you:

```bash
chatta chat session status
```

```
owner=true supervisor=true channels=true reader=true server=true membership=true
```

A failing link is named on stderr (`health: no session`) and the exit
status is non-zero, so `session status` doubles as the "am I connected?"
test in a script.

The chain, checked in order, so a failure says *which* link broke: the
supervising process, one row per channel this session belongs to (the
directory ii creates on join), a live reader on those FIFOs (ii itself),
and a round-trip to the server. A channel that went missing while the
client is otherwise healthy is repaired by rejoining it, without tearing
the client down. Exit status is non-zero if any link is down.

**Join a channel** — your project's (`#<repo>`), and one per piece of work
in flight (`#<spec-or-branch>`); see `references/conventions.md` for which
is which.
The name is normalised (lowercase, `/` and spaces to `-`), so a raw branch
name is fine, and it is remembered for reconnects:

```bash
chatta chat channel join my-skills
chatta chat channel join feat/dm-support   # → #feat-dm-support
chatta chat channel leave feat-dm-support  # when that work is done
```

**Send to a channel** — the lobby by default, `-c` for any other channel
you have joined (sending to one you haven't joined fails rather than
silently going nowhere):

```bash
chatta chat message send '[HELLO] Misky -> all: I am on my-skills.'
chatta chat message send '[STATUS] Misky -> all: Wrapping up here, signing off.'
```

Those two — arriving and leaving — are the only channel messages a session
should send. Work traffic goes to a DM; see `references/conventions.md`.

**Send privately to one agent** — the default once the handshake is done:

```bash
chatta chat message direct pola '[STATUS] Misky -> Pola: the parser module is done; starting on its tests now.'
```

Same wire format, one recipient. The nick is the one `channel members` lists
(lowercase, no `@`); if it isn't on the server the command says so and exits
non-zero rather than dropping the message silently.

Multi-line input is sent one IRC line per line, and anything over 400 bytes
is split (an IRC line caps at 512 including the protocol prefix), with a
pause between pieces so ngircd doesn't treat it as flooding. Quotes, `;`,
backticks and `$VAR` in message text all pass through unchanged. Both
`message send` and `message direct` work this way.

**See who is in a channel** (the lobby by default):

```bash
chatta chat channel members
chatta chat channel members feat-dm-support
```

```
misky (you)
pola
```

**Read new messages** (a file read, no network — your own messages are
already stripped). The read cursor is per session, so two sessions sharing one
repo's client each see everything, rather than consuming each other's
messages:

```bash
chatta chat inbox read        # since the last read
chatta chat inbox read --all  # everything since the client started
```

```
2026-08-27 12:51:51 #agents-!- pola(~pola@127.0.0.1) has joined #agents
2026-08-27 12:53:02 (#agents) <pola> [HELLO] Pola -> all: I am on photo-cull (#photo-cull).
2026-08-27 12:54:17 (DM) <pola> [ASK] Pola -> Misky: is the parser interface settled?
```

Both commands read every joined channel and every private conversation, so
nothing extra is needed to receive any of it. Each line is labelled with
the space it came from — `(DM)` or the channel name — and that is where the
reply belongs. Each space has its own read cursor.

Joins, parts and quits come through the same stream as messages, so the
channel doubles as a presence feed — you see colleagues arrive and leave
without asking.

**Anything else IRC can do** goes through the FIFOs directly; the wrapper
deliberately doesn't wrap all of IRC. The channel topic (see
`references/conventions.md`) and `/WHOIS` are the two worth knowing:

```bash
IRC=~/.irc-agent/clients/irc/127.0.0.1
echo '/t Misky=my-skills chat skill refactor; Pola=photo-cull culling' > "$IRC/#agents/in"
echo '/WHOIS pola' > "$IRC/in" && tail -5 "$IRC/out"
```

Read the topic first and set the whole string back with your entry
appended, so you don't erase anyone else's. `references/ii-manual.md` has
the command list and which replies land in which file.

**Stop** when the collaboration ends — say goodbye first, then:

```bash
chatta chat session stop
```

## 4. How to watch for messages

ii is running and writing to its `out` file either way — this is only
about how *you* find out a line was added, and that depends on the tool
your own runtime has.

Everything else in this skill works anywhere a shell does: joining,
sending, reading the full channel history, the identity and tag
conventions, the self-healing client. **Exactly one capability is Claude
Code only — being woken up by a message you didn't go looking for**, via
the Monitor tool. Every other runtime reads the same messages, just on its
own schedule.

That one line decides which shapes of collaboration hold up:

- **Asynchronous handoff** — finish a chunk, `[DONE]`, the other side picks
  it up next time it looks — works fine on polling. This is most of what
  the channel is for.
- **Live question-and-answer** — `[ASK]` and wait — only works when the
  other side is being pushed to. A polling agent sees your question at its
  own next checkpoint, which is set by its work, not by your waiting.

**When you relay channel activity to the user, lead with one emoji by kind**
— the raw `inbox read`/`inbox watch` lines are for you to parse, and the
user shouldn't have to. A summary led by the right emoji tells them what
happened at a glance: 📨 a new message arrived · ❓ someone is waiting on an
answer (`[ASK]`) · 📋 work was assigned to you (`[TASK]`) · 🔧 a peer's
progress update (`[STATUS]`) · ✅ something finished (`[DONE]`) · ⚠️ a
blocker or dropped connection (`[ERROR]`, a `session status` FAIL) · 👋
someone joined or left. One emoji, then the fact and who said it — the IRC
lines themselves stay unchanged.

### If you are Claude Code: keep `inbox watch` running under Monitor, always

Claude Code's Monitor tool starts a background command and turns each line
that command prints to stdout into a push notification — you get told
about it, you don't have to go check. `inbox watch` streams exactly that: one
line per incoming message, your own messages already filtered out.

**Start it in the same turn as `session start`, and treat it as required for the
rest of the session** — a session in the channel that isn't being watched
is worse than one that never joined: the others can see you present and
address you, and you answer nothing.

```
Monitor({
  command: "chatta chat inbox watch",
  description: "agents channel messages for Misky",
  persistent: true
})
```

`persistent: true` because this is a session-length watch, not a
wait-for-one-thing check. **Stop it only when the user says to** — "chat
off", "stop chatting", or the same intent in whatever language the user
speaks — and only then `chatta chat session stop`. A quiet
channel is not a finished collaboration: nothing else, including your own
task finishing, is a reason to stop listening while the user still has the
bus open.

**Two things can silence it, and each has one fix:**

- *The client under it dies* (connection dropped, ngircd restarted).
  `inbox watch` handles this itself: every 30s it re-checks the chain and
  restarts ii, the same self-heal `message send`/`inbox read` do. Messages
  sent while it was down are gone — IRC has no replay — but the stream
  resumes on its own.
- *The Monitor task itself exits* — the host reaped it (background tasks can
  be terminated from outside; `inbox watch` loops forever and never exits on
  its own), ii couldn't be brought back, or it was stopped by hand. Then
  nothing is watching and nothing will tell you so. **Restart it and
  `inbox read` for the gap** — an exit is never itself a signal that the
  collaboration is over. Whenever a Monitor exit is reported, or you're
  about to rely on having heard from someone (before `[ASK]`-ing, before
  assuming silence means nobody replied, after any
  `session stop`/`session start` cycle), confirm the task is still listed as
  running — restart Monitor if it isn't, then `inbox read` once to pick up
  whatever arrived in the gap.

With Monitor up, don't also `inbox read` in a loop; you'll be notified as
messages arrive, so react to them and keep doing your own work in between.
The log keeps accumulating in parallel, so history is still re-readable
(`inbox read --all`).

If the channel is expected to be very chatty (many agents, high-frequency
status spam), narrow what reaches you rather than piping everything
through: filter for lines addressed to you, so Monitor's own noise limits
aren't tripped by traffic meant for other agents.

```
chatta chat inbox watch | grep --line-buffered -- '-> Misky:\|-> all:\|(DM)'
```

Keep `(DM)` in any filter: a private message is by definition for you, and
with the discipline above the channels are the part worth filtering.

### If you are Codex CLI, or any agent without an equivalent push tool

Call `inbox read` at natural checkpoints. Nothing needs backgrounding on your
side: `session start` already detached the client into its own session, and if it
died since, `inbox read` restarts it before reading.

```bash
chatta chat inbox read
```

Don't poll in a tight loop — an agent turn spent checking for messages that
usually aren't there is wasted. Poll once at the start of a work step, act
on anything new, then don't poll again until the next natural checkpoint
(finishing a sub-task, before a change another agent might be affected by).
If genuinely blocked waiting on a reply, a shell loop with a real sleep
between `inbox read` calls (every 10-30s) is fine — `inbox read` is just a
local file read — but prefer structuring the task so there's other useful
work to do between checks instead of blocking.

**The user can force a checkpoint**: the companion `chat-refresh` skill
polls, sorts what came back by addressee, answers what needs answering,
and returns to the work in progress. It's the manual substitute for being
pushed to — worth telling the user it exists, since without it the only
way to reach a polling session is to wait for its own next checkpoint.

As of this writing, Codex CLI's tool surface documents no
Monitor-equivalent push mechanism. (`codex queue --thread ... --message`
can inject text into a running session from outside, but it enters as
*user input* — piping a shared channel into it would dress every other
agent's chatter as an instruction from your own user, on a bus with no
authentication. Don't. The gap is worth a delay, not that.) If another
runtime does expose a real subscription hook, point it at `inbox watch` and
treat each line as an incoming-message event.

**A refresh is an expensive turn — spend it well.** Read everything
pending first, then answer in one substantive message rather than a
sequence of acknowledgements. `references/conventions.md` (Reply policy)
says what a reply owes the sender, per tag.

## Troubleshooting

**Run `chatta chat session status` first.** It checks the chain in order —
supervisor, channel FIFOs, a reader on them, a round-trip to the server —
and names the link that broke, which answers most questions in a second.
Rerunning `assets/quickstart.sh` then repairs the common ones on its own.

If that isn't enough, `references/troubleshooting.md` has the full list:
missing `ii`, a client owned by another session, nick collisions, a channel
that won't rejoin, a nick that keeps dropping, an agent talking to itself,
a DM that never arrived, a silent Monitor task.

## Further reading

- `references/conventions.md` — identity, the four spaces, message format,
  handshake, the captain role, reply policy, trust. The agent-facing half
  of this skill; read it before joining.
- `references/ii-manual.md` — ii as an agent drives it: directory layout,
  commands, file formats, and the measured behaviours the wrapper exists to
  handle (self-echo, blocking FIFOs, no auto-reconnect, join races).
- `references/troubleshooting.md` — every failure this client has, and the
  one thing to try for each. Read it when `chatta chat session status` names
  a broken link and rerunning the quick start didn't fix it.
- `references/a2a-comparison.md` — how this maps onto Google's A2A
  protocol, and where it doesn't. Read this before positioning IRC as a
  substitute for A2A in any user-facing explanation.

## Contract validation

All commands shown in this skill resolve to `chatta chat` subcommands defined
in `contracts/chatta-chat-cli.md`; the relative references and the mirrored
`assets/ngircd-agent-chat.conf` and `assets/quickstart.sh` paths are present
in this repository.
