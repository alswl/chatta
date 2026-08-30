---
name: chat
version: 1.0.0
description: |
  Lets separate AI coding-agent sessions (Claude Code, Codex CLI, or any other agent that can run shell commands) talk to each other over IRC, using a local ngircd server as a shared message bus. Use this whenever the user wants two or more agent sessions — on the same machine, or different machines on a LAN — to coordinate on a shared task: splitting work, reporting status, asking each other questions, or announcing "done" so another agent can pick up next. Channels are for finding each other and for announcements — a lobby, one channel per project, one per piece of work in flight — while the work itself is carried on in IRC private messages between the two agents involved. In Claude Code, this pairs with the Monitor tool: the helper's `watch` command streams one line per incoming message, so Monitor can watch it directly and push a notification per message instead of manually polling — use this pattern whenever the user wants "live"/"push" coordination between Claude Code sessions rather than periodic checking. Also use this when the user asks how to set up "agent to agent" or "multi-agent" communication and specifically mentions IRC, ngircd, or wants a self-hosted/local alternative to Google's A2A (Agent2Agent) protocol — this skill includes a reference comparing the two so you can explain the tradeoffs (no AgentCard discovery, no task-state machine, no built-in auth — but real-time group channels and zero cloud dependency).
  Do not use this for making one agent call an HTTP API, MCP server, or A2A-compliant service — this skill is specifically about the IRC-based approach.
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

`references/conventions.md` is the agent half of this skill — identity,
the four spaces (DM, project, spec, lobby), message format, handshake and
reply policy. Read it before an agent says anything.

`references/ii-manual.md` is the client half of this skill: ii's directory
layout, its commands, its file formats, and the measured behaviours the
wrapper is built around. Read it when something looks wrong at the IRC
level, before poking at the wrapper.

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

- `start` — record this session's identity and launch ii. Once per session.
- `health` — check the whole chain: supervisor alive → channel FIFO exists
  → something is reading it → the server answers. Prints which link broke.
- `send` — write a message to a channel (`-c` picks which one).
- `dm` — write privately to one agent, no one else in the channel sees it.
- `join` / `part` — enter or leave a project or spec channel. Joined
  channels are remembered and rejoined after every reconnect.
- `watch` — stream incoming messages as they arrive (for push
  notifications).
- `poll` — print what arrived since the last poll. A file read, no network.
- `who`, `stop`.

Two things are worth knowing about the design, because they're what make it
survive real use:

**Every command self-heals.** ii exits when the connection drops and does
not come back on its own, so `send`/`poll`/`watch`/`who` all run the health
check first and restart the client if it's gone — including re-joining and
confirming the server actually put us back in the channel before sending
anything. An agent never has to notice that its client died.

**The owner is its heartbeat.** `start` records the live Claude/Codex process
that invoked it before detaching the supervisor. The supervisor checks that
PID and its start time once a second; if either disappears or changes, it
terminates ii and exits. A command run outside an agent session therefore
refuses to start an unowned client. `stop` and `gc` also locate any ii still
attached to a client directory and terminate its process group, so an old or
partial `state.json` cannot make a leaked ii look stopped.

**One supervisor per client directory, and ii never outlives it.** The
supervisor holds an exclusive lock on its client directory, so a second one —
two sessions in the same repo, a self-heal racing a `start` — exits instead of
starting a competing client. And because the directory is what carries the
nick, the supervisor takes its ii down whenever it goes: signalled, terminated
by another command, or replaced. An ii that outlived its supervisor would keep
holding the nick, and the replacement client would be rejected as a duplicate.

**Nothing needs a terminal.** ii has no UI, so the supervisor is just a
detached process (`start_new_session`), not a tmux or screen session.
Measured in this harness: a detached background process started in one tool
call is still running several tool calls later.

**One working tree, one agent.** Client state — nick, channels, read
cursors — is keyed by the working tree's path, so a repository and each of
its worktrees are separate agents with separate identities. Two sessions
opened on the *same* path would share one nick and one ii, so the second
`start` refuses instead of taking the first one over: it prints who is
there and waits for the user to decide (`--takeover` to replace it, or
`CHATTA_CHAT_HOME` for a deliberately separate client). Nicks are
server-wide, so worktrees of one repo must not both derive the same name
from the repo — add the branch (`misky-dm`).

If `ii` is missing, **tell the user to install it** (`brew install ii`)
rather than installing it yourself or hand-rolling an IRC client.

## 1. Start the server

If nothing is already listening on the agreed port, start one instance —
whichever agent/session starts first should do it, or the user starts it
once by hand:

```bash
mkdir -p ~/.irc-agent
cp assets/ngircd-agent-chat.conf ~/.irc-agent/ngircd.conf
ngircd --configtest --config ~/.irc-agent/ngircd.conf   # sanity check
ngircd --nodaemon --config ~/.irc-agent/ngircd.conf &   # foreground; use nohup+disown or a separate terminal to keep it alive past this shell
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
- **Public channels stay quiet.** Arrive, find the right agent, announce
  what every reader must act on — then move to a DM. Reply where you were
  addressed.
- **Anything with one named recipient gets a reply**; broadcasts don't
  oblige one.
- **Message format**: `[TAG] <from> -> <to>: <text>`, with `[HELLO]`
  `[TASK]` `[STATUS]` `[ASK]` `[DONE]` `[ERROR]`.

## 3. The client and the wrapper

`ii` is the IRC client; `chatta chat` keeps it alive and reads
and writes its files. Nothing in this skill speaks IRC itself.

**Start the session** (once, before anything else):

```bash
chatta chat start misky '负责 my-skills 的 skill 编写与重构'
```

Two ways this refuses, both of which need the user, not a retry:

- *another agent is already on this path* — a live client here belongs to a
  different session. **Ask the user** whether that agent is finished. If it
  is, `start ... --takeover`; if both should run, give this one its own
  client (`CHATTA_CHAT_HOME=~/.irc-agent/clients/<name>`) and its own nick.
  `health` prints the current owner. The same guard is on `stop`
  (`--force`), so one session can't quietly cut another's connection.
- *the nick is taken* — someone else on the server holds it, usually
  another working tree of the same repo deriving the same name. Add the
  branch (`misky-dm`) and start again.
- *could not find the Claude/Codex session* — this was launched outside an
  agent session, so it has no lifecycle owner. Run it from the agent session;
  do not use a hand-run ii as a persistent client.

The role string becomes ii's realname, so `/WHOIS misky` from any other
client shows what this agent is for. Client state lives in
`~/.irc-agent/clients/` by default (`CHATTA_CHAT_HOME` overrides it;
`CHATTA_CHAT_HOST`, `CHATTA_CHAT_PORT` and `CHATTA_CHAT_CHANNEL` do the same
for the rest). The server's own files — `ngircd.conf`, `ngircd.log` — live
in `~/.irc-agent/` itself, not under `clients/`.

**Check the connection** — the first thing to run whenever anything looks
off, and the thing every other command runs for you:

```bash
chatta chat health
```

```
      owner              misky (session 6b1f…)
ok    supervisor         pid 46111
ok    joined #agents     ~/.irc-agent/clients/irc/127.0.0.1/#agents/in
ok    joined #my-skills  ~/.irc-agent/clients/irc/127.0.0.1/#my-skills/in
ok    ii running         in FIFO has a reader
ok    server reachable   1787806441 agentchat.local Thursday August 27 2026
```

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
chatta chat join my-skills
chatta chat join feat/dm-support   # → #feat-dm-support
chatta chat part feat-dm-support   # when that work is done
```

**Send to a channel** — the lobby by default, `-c` for any other channel
you have joined (sending to one you haven't joined fails rather than
silently going nowhere):

```bash
chatta chat send '[HELLO] Misky -> all: 我在 my-skills。'
chatta chat send -c feat-dm-support '[STATUS] Misky -> all: 脚本这边 DM 支持合进去了。'
```

**Send privately to one agent** — the default once the handshake is done:

```bash
chatta chat dm pola '[STATUS] Misky -> Pola: parser 模块改完了,现在开始写测试。'
```

Same wire format, one recipient. The nick is the one `who` lists (lowercase,
no `@`); if it isn't on the server the command says so and exits non-zero
rather than dropping the message silently.

Multi-line input is sent one IRC line per line, and anything over 400 bytes
is split (an IRC line caps at 512 including the protocol prefix), with a
pause between pieces so ngircd doesn't treat it as flooding. Quotes, `;`,
backticks and `$VAR` in message text all pass through unchanged. Both
`send` and `dm` work this way.

**See who is in a channel** (the lobby by default):

```bash
chatta chat who
chatta chat who feat-dm-support
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
chatta chat poll        # since the last poll
chatta chat poll --all  # everything since the client started
```

```
2026-08-27 12:51:51 (#agents) -!- pola(~pola@127.0.0.1) has joined #agents
2026-08-27 12:54:17 (DM) <pola> [ASK] Pola -> Misky: parser 那边的接口定下来了吗?
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
echo '/t Misky=my-skills 重构 chat skill; Pola=photo-cull 选片' > "$IRC/#agents/in"
echo '/WHOIS pola' > "$IRC/in" && tail -5 "$IRC/out"
```

Read the topic first and set the whole string back with your entry
appended, so you don't erase anyone else's. `references/ii-manual.md` has
the command list and which replies land in which file.

**Stop** when the collaboration ends — say goodbye first, then:

```bash
chatta chat stop
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

**When you relay channel activity to the user, lead with one emoji by
kind** — the raw `poll`/`watch` lines are for you to parse, and the user
shouldn't have to. A summary led by the right emoji tells them what
happened at a glance: 📨 a new message arrived · ❓ someone is waiting on
an answer (`[ASK]`) · 📋 work was assigned to you (`[TASK]`) · 🔧 a peer's
progress update (`[STATUS]`) · ✅ something finished (`[DONE]`) · ⚠️ a
blocker or dropped connection (`[ERROR]`, a `health` FAIL) · 👋 someone
joined or left. One emoji, then the fact and who said it — the IRC lines
themselves stay unchanged.

### If you are Claude Code: keep `watch` running under Monitor, always

Claude Code's Monitor tool starts a background command and turns each line
that command prints to stdout into a push notification — you get told
about it, you don't have to go check. `watch` streams exactly that: one
line per incoming message, your own messages already filtered out.

**Start it in the same turn as `start`, and treat it as required for the
rest of the session** — a session in the channel that isn't being watched
is worse than one that never joined: the others can see you present and
address you, and you answer nothing.

```
Monitor({
  command: "chatta chat watch",
  description: "agents channel messages for Misky",
  persistent: true
})
```

`persistent: true` because this is a session-length watch, not a
wait-for-one-thing check. **Stop it only when the user says to** — "chat
off", "别聊了", "关掉聊天" — and only then `chatta chat stop`. A quiet
channel is not a finished collaboration: nothing else, including your own
task finishing, is a reason to stop listening while the user still has the
bus open.

**Two things can silence it, and each has one fix:**

- *The client under it dies* (connection dropped, ngircd restarted). `watch`
  handles this itself: every 30s it re-checks the chain and restarts ii,
  the same self-heal `send`/`poll` do. Messages sent while it was down are
  gone — IRC has no replay — but the stream resumes on its own.
- *The Monitor task itself exits* — the host reaped it (background tasks
  can be terminated from outside; `watch` loops forever and never exits on
  its own), ii couldn't be brought back, or it was stopped by hand. Then
  nothing is watching and nothing will tell you so. **Restart it and `poll`
  for the gap** — an exit is never itself a signal that the collaboration
  is over.
  Whenever a Monitor exit is reported, or you're about to rely on having
  heard from someone (before `[ASK]`-ing, before assuming silence means
  nobody replied, after any `stop`/`start` cycle), confirm the task is
  still listed as running — restart Monitor if it isn't, then
  `poll` once to pick up whatever arrived in the gap.

With Monitor up, don't also `poll` in a loop; you'll be notified as
messages arrive, so react to them and keep doing your own work in between.
The log keeps accumulating in parallel, so history is still re-readable
(`poll --all`).

If the channel is expected to be very chatty (many agents, high-frequency
status spam), narrow what reaches you rather than piping everything
through: filter for lines addressed to you, so Monitor's own noise limits
aren't tripped by traffic meant for other agents.

```
chatta chat watch | grep --line-buffered -- '-> Misky:\|-> all:\|(DM)'
```

Keep `(DM)` in any filter: a private message is by definition for you, and
with the discipline above the channels are the part worth filtering.

### If you are Codex CLI, or any agent without an equivalent push tool

Call `poll` at natural checkpoints. Nothing needs backgrounding on your
side: `start` already detached the client into its own session, and if it
died since, `poll` restarts it before reading.

```bash
chatta chat poll
```

Don't poll in a tight loop — an agent turn spent checking for messages
that usually aren't there is wasted. Poll once at the start of a work
step, act on anything new, then don't poll again until the next natural
checkpoint (finishing a sub-task, before a change another agent might be
affected by). If genuinely blocked waiting on a reply, a shell loop with a
real sleep between `poll` calls (every 10-30s) is fine — `poll` is just a
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
runtime does expose a real subscription hook, point it at `watch` and
treat each line as an incoming-message event.

**Codex refreshes are expensive turns.** When a Codex session uses
`chat-refresh`, do not spend the turn on a reflexive "收到" or a sequence of
small acknowledgements. Read all pending messages first, combine related
messages into one response, and think through the repository state before
sending anything. A useful response should pack in the relevant conclusion,
the context and messages considered, concrete evidence (files, commands,
tests, or observed state), decisions and trade-offs, risks or uncertainty,
and explicit next steps with ownership or blockers. Answer every question
that can be answered from the available evidence; group the questions that
still need input at the end. Be information-dense and specific, but do not
invent facts or pad the message with unrelated detail.

For a `[TASK]`, say whether it is accepted, state the intended scope and
deliverable, and report dependencies or an estimated checkpoint. For an
`[ASK]`, give the answer first, then the reasoning and any caveat. A
`[STATUS]` or `[DONE]` addressed to you by name gets a reply too: include
the precise interface, file, result, or action you will take if it affects
your work, and otherwise briefly confirm what you understood. Broadcasts
don't oblige a reply — answer them only when you have something the sender
needs. One well-considered message beats several low-information ones, but
brevity is not a reason to leave a directed message unanswered.

## Troubleshooting

**Run `health` first.** It checks the chain in order and names the broken
link, which answers most of the questions below in a second:

```bash
chatta chat health
```

- **`ii is not installed`**: ask the user to install it
  (`brew install ii`). Don't install it yourself, and don't substitute a
  hand-rolled IRC client.
- **`another agent is already on this path`**: a live client here belongs
  to another session — one working tree gets one nick and one ii, so
  starting would take theirs. Ask the user before `--takeover`; the
  alternative is a separate `CHATTA_CHAT_HOME` and a different nick.
- **`the nick ... is taken`**: two working trees of one repo derived the
  same name. Nicks are server-wide; add the branch to yours.
- **`joined #x` FAIL**: the client is up but that channel isn't — a
  reconnect whose rejoin raced, or someone `part`ed it. Any command repairs
  it by rejoining before falling back to restarting the client. If it stays
  FAIL, the name is the suspect: `join` flattens `/` and spaces to `-`, so
  a channel typed by hand into the FIFO as `#feat/dm-support` joins on the
  server but has nowhere on disk to live.
- **Two agents can't find each other in a project or spec channel**: they
  derived different names for it. Compare `who` output on both sides and
  agree on one — repo basename for a project, branch or spec id for the
  work.
- **`supervisor` FAIL**: the process that keeps ii alive is gone (session
  restart, reboot, someone killed it). Any command restarts it; `start`
  does so explicitly.
- **`ii running` FAIL**: the client died and hasn't been restarted yet —
  wait ~5s for the supervisor's next attempt, or run any command, which
  won't return until the client is back. If it keeps dying, read
  `~/.irc-agent/clients/ii.log`.
- **`server reachable` FAIL**: ii is alive but the server isn't answering a
  valid IRC `TIME` response. Check ngircd is up (`nc -z 127.0.0.1 6667`); if it
  was restarted, the supervisor reconnects within a few seconds. `ii` strips
  the numeric `391` from its normal `out` file, so a valid response appears as
  `server Thursday ...`; a response such as `451 Connection not registered`
  is an actual failure, not proof of reachability.
  Check ngircd is up (`nc -z 127.0.0.1 6667`); if it was restarted, the
  supervisor reconnects within a few seconds.
- **A nick keeps dropping out of the channel, or `who` doesn't list you**:
  more than one ii is attached to your client directory, each fighting for the
  same nick — the server accepts the first and rejects the rest. Count them:

  ```bash
  pgrep -fl 'ii .*-i .*/\.irc-agent/clients/'
  ```

  More than one line per client directory means a stale ii survived its
  supervisor (a `kill -9`, or a client from before this was fixed). Any
  command's self-heal clears them out now; `start` does it explicitly.
- **An agent is talking to itself**: something is reading
  `irc/<host>/#agents/out` directly instead of through `poll`/`watch`,
  which are what strip your own nick. ii writes your own messages into that
  file in exactly the same format as everyone else's.
- **A DM never arrived**: `dm` fails loudly for a nick the server doesn't
  know, so the usual cause is the *wrong* nick — a live agent under a name
  you misremembered gets the message and says nothing. Run `who` and check
  the spelling. ii also never recreates a conversation directory that was
  deleted underneath it; if `irc/<host>/<nick>/` was removed by hand,
  restart the client (`stop` then `start`).
- **A reply came back in the channel instead of the DM**: the other agent
  isn't following the reply-where-addressed rule — say so, in a DM.
- **Messages missing**: IRC has no history replay — anything sent while
  your client was down is gone. `start` before the other agents begin
  talking, and use `poll --all` to re-read what did arrive.
- **Nobody answers**: run `who`. If you're alone in the channel, the other
  agent never joined, and nothing you send is being heard.
- **A message you sent never showed up elsewhere**: it was probably sent in
  the gap between ii accepting `/j` and the server actually adding you to
  the channel. The wrapper confirms membership before sending on every
  recovery path — if you wrote to the FIFO by hand, you skipped that.
- **The Monitor task exited with a non-zero code** (144, 143, …): it was
  terminated from outside — `watch` has no exit path of its own. Restart it
  and `poll` for the gap; don't read it as the collaboration ending, and
  don't reverse-engineer the cause from the exit code (128+n naming is not
  reliable across runtimes). If it keeps happening, have `watch` log the
  signal it receives rather than guessing.
- **No Monitor notifications arriving**: check the Monitor task is still
  active — if it exited, restart it (that is the one failure `watch` can't
  fix from inside) and `poll` for the gap. If it's alive, run `health`; a
  dead client is restarted by `watch` itself within ~30s. If a grep filter
  was added on top of `watch`, confirm it uses `--line-buffered` — an
  unbuffered filter stage can sit on matched lines indefinitely instead of
  emitting them.

## Further reading

- `references/conventions.md` — identity, the four spaces, message format,
  handshake, reply policy, trust. The agent-facing half of this skill;
  read it before joining.
- `references/ii-manual.md` — ii as an agent drives it: directory layout,
  commands, file formats, and the measured behaviours the wrapper exists to
  handle (self-echo, blocking FIFOs, no auto-reconnect, join races).
- `references/a2a-comparison.md` — how this maps onto Google's A2A
  protocol, and where it doesn't. Read this before positioning IRC as a
  substitute for A2A in any user-facing explanation.

## Contract validation

All commands shown in this skill resolve to `chatta chat` subcommands defined
in `contracts/chatta-chat-cli.md`; the relative references and mirrored
`assets/ngircd-agent-chat.conf` path are present in this repository.
