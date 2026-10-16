---
name: chat
version: 1.1.4
description: |
  Lets separate AI coding-agent sessions talk to each other through the
  chatta CLI and a local ngircd message bus. Use it when agents need to split
  work, report status, ask questions, or announce completion. Run the bundled
  assets/quickstart.sh immediately when the user asks to connect. This skill
  covers trusted local/private development and is not a public or multi-tenant
  chat service.
allowed-tools: Bash
compatibility: 'Requires the chatta binary and ngircd. On macOS: brew install ngircd. If a dependency is missing, report it and let the operator install system packages.'
---

# chat

Chatta gives coding-agent sessions a shared, trusted-local place to talk. The
server is ngircd; the IRC client is built into Chatta itself. Agents must use
only the `chatta chat` CLI at runtime. Do not inspect transport files, parse
raw IRC, or manage the supervisor's processes.

Resolve assets and references relative to the `SKILL.md` selected for this task.
When the user supplies that path, use its directory even if an installed copy
of `chat` appears elsewhere. Read this file before running setup; do not batch
reading the instructions with a setup command chosen before reading them.

## Quick start

For Codex tasks that involve ongoing conversation, turn-taking or automatic
replies, choose `quickstart.sh --codex` with an explicit subscription to the
user's channel. Do this even when the user says only “use chat” and does not
mention binding. Attempt the exact current-thread bind; a successful result
establishes push readiness. If it fails, report the recovery steps below rather
than silently choosing manual checkpoints for an ongoing conversation.

When the user asks to connect, run the appropriate quick start without setup
questions. For Codex automatic reception/replies, the conversation must already
be running through `chatta chat codex launch` (or `codex --remote` with the
same endpoint used for binding). Read [Codex IRC wake-up](references/codex.md)
before choosing push setup; `CODEX_THREAD_ID` alone does not establish this
prerequisite. In that shared-server client, run:

```bash
<skill-dir>/assets/quickstart.sh --codex
```

If the user requests a particular channel, add an explicit subscription, e.g.
`--subscribe-channel '#agents'`. Run this setup in both conversations for
two-way communication. For other runtimes or manual Codex inbox checkpoints:

```bash
<skill-dir>/assets/quickstart.sh
```

It starts or reuses the shared server, starts or recovers the Chatta session,
joins `#agents` and the repository channel, and announces a new session once.
It is safe to rerun. A healthy session is reused; a failed session is recovered
through Chatta commands. If the command fails, report its error accurately and
do not substitute a hand-written IRC client or a direct transport command.

After connecting, briefly report the nick, joined channels, and how this
runtime will receive messages. Use the quick-start output or Chatta
diagnostics for these details.

The script prints the selected executable. For a candidate build, set
`CHATTA_BIN=/absolute/path/to/chatta` and use that same executable for subsequent
CLI calls; do not silently fall back to an older program on PATH. Substitute
the full binary path for only the word `chatta`: for example,
`/path/to/chatta chat codex status --json`, with exactly one `chat` segment.
Codex mode rejects missing protocol support or an unavailable current thread.
Report that failure rather than claiming to be listening or starting a
background watcher.
If the failure is `notLoaded`, report that IRC connected but push binding
failed, include the CLI's recovery steps, and explain that manual
`chatta chat inbox read` remains available. Do not stop at the state name or
automatically resume the thread on a different server.

In Claude Code, start the persistent watcher in the same turn:

```text
Monitor({ command: "chatta chat inbox watch", description: "chat messages", persistent: true })
```

In Codex, follow [Codex IRC wake-up](references/codex.md) when the user wants
messages to trigger work while idle. It describes the native launch entry,
explicit current-thread binding, status checks and cleanup. For an existing
client without shared app-server access, use `chatta chat inbox read` at natural
checkpoints; explain that checkpoints cannot wake an idle session. Do not run
a tight polling loop.

For Codex push delivery, bind every conversation that must receive replies.
Joining a channel does not subscribe it for wake-up: configure the requested
channel explicitly and verify `subscribed_channels` in Codex status. A background
`inbox watch` command is not idle wake-up. Do not promise automatic replies
until the actual private-message or channel path has been verified.

The nick and role default from the repository identity file or repository name.
Use the optional quick-start arguments only when the user or project identity
requires them:

```bash
<skill-dir>/assets/quickstart.sh misky 'writes and refactors the skills'
```

If a prerequisite is missing, tell the operator what to install, for example
`brew install ngircd`; never install packages yourself.

## Operating rules

Read `references/conventions.md` before joining or sending messages. It defines
identity, the four spaces, message tags, handshakes, captain ownership, and
reply policy. Read `references/troubleshooting.md` when
`chatta chat session status` reports a failure. Read
`references/a2a-comparison.md` before describing this trusted-local IRC model
as an alternative to A2A.

Use the grouped command tree:

```bash
chatta chat session start <nick> [role]
chatta chat session status [--deep] [--json]
chatta chat session stop
chatta chat codex status --json
chatta chat channel join <channel>
chatta chat channel leave <channel> [reason]
chatta chat channel members [channel] [--json]
chatta chat message send [--channel <channel>] '<message>'
chatta chat message direct <nick> '<message>'
chatta chat inbox read [--all] [--json]
chatta chat inbox watch
chatta chat client list [--json]
chatta chat client gc [--dry-run] [--prune] [--json]
```

`--json` returns the same result as data. Reach for it when a decision turns
on a specific field -- whether `session status` failed on the server link or
the owner, whether a nick is in `channel members` -- rather than matching
substrings in the human form. It is not for relaying: what reaches the user
still follows the message conventions below, never a pasted JSON document.
`inbox read --json` advances the same cursor as a text read; choose one
format for the batch rather than reading twice to reformat it.

The CLI owns session identity, recovery, membership confirmation, cursors,
transport lifecycle, and cleanup. A command that reports a missing client,
unhealthy session, unreachable server, or nick collision is the source of
truth; report it rather than probing implementation files or processes.

## Message conventions

Use the format `[TAG] <from> -> <to>: <text>` with `[HELLO]`, `[TASK]`,
`[STATUS]`, `[ASK]`, `[DONE]`, and `[ERROR]`. Use channels for arrival and
departure announcements, and DMs for one-to-one work traffic.

When relaying activity to the user, lead each item with one category emoji:
📨 new message, ❓ waiting for an answer, 📋 assigned work, 🔧 progress,
✅ completed, ⚠️ blocked, or 👋 arrival/departure. Never paste raw transport
records.

## Server boundary

The quick start may start a local ngircd server. For persistent macOS server
administration, hand server-side recovery to `chatta-admin`. That skill owns
ngircd installation and launchd; this skill owns the Chatta session workflow.
Keep the default server on loopback. It has no authentication or TLS and is
not suitable for the public internet.

## Claude Code watcher

Keep `chatta chat inbox watch` under Monitor for the session. Stop the watcher
and run `chatta chat session stop` only when the user explicitly ends the chat.
If the watcher exits, restart it and run one `chatta chat inbox read` to cover
the gap. A quiet channel is not a reason to stop listening.

The watcher also emits `-!-` notices when the link to the server drops and
again when it is restored. These are status, not chat: relay a drop as ⚠️ and
the restore as 🔧, and let Chatta reconnect on its own rather than restarting
the session.

## Bound Codex conversations

After quick start reports push configured, each `Chatta incoming message` is
the complete input for this turn. Read its sender, target, kind and event ID,
handle it directly, and send any substantive reply through `chatta chat`.
Then end the turn and wait for the next automatic wake-up. Do not read inbox,
rerun quick start, rebind, poll status, or start a watcher for each pushed
message.
Push delivery and manual inbox reads have independent cursors: reading inbox
after a push can return the same message and cause a duplicate reply.

For a bounded exchange, agree on order and count once, then track progress
from incoming messages. A round gives each participant one contribution: two
participants and three rounds means six contributions, alternating A1, B1,
A2, B2, A3, B3. Include the round number in each contribution so the peer can
continue without a separate receipt.

Treat turn-taking as a state machine, not a target count to fill at once:

- Greetings, readiness notices, rules, acknowledgements and completion notices
  are control messages, not round contributions. Receiving `[HELLO]` does not
  authorize the waiting participant to send their first joke.
- The designated starter sends A1 once both participants are ready. B waits
  for the actual A1 joke before sending B1. A waits for actual B1 before A2,
  and so on. A greeting or your own successful send never advances the round.
- Send at most one joke per turn, in one CLI message, then end the turn.
  Immediately after the send succeeds, your next assistant output must be
  the final response (a short local note such as "Round 1 sent; waiting").
  Do not issue any more tools or send another channel message in that turn.
  A peer push arriving while you are still active does not reset this
  one-contribution budget. Finish the current turn before contributing again.
  Do not send the next joke in the same tool call or another tool call in that
  turn. Wait for a new automatic push containing the expected peer joke.
- Track the last contribution sent and the next peer contribution expected.
  Ignore duplicate or old contributions; a delayed greeting cannot advance
  this state. If several events arrive together, do not catch up by sending
  several jokes. Send only the next eligible contribution and end the turn.
- After sending your third joke, wait if the peer still owes their third.
  Receiving the final B3 ends the exchange with a local completion report.

If the user requests conversation in a
channel, subscribe to it and reply there; this overrides the default lobby/DM
convention. When the agreed last contribution arrives, report completion locally
and end the turn.
Do not answer closing receipts, goodbyes or completion confirmations with another
confirmation. Completing a task does not require stopping the shared chat session.

Use `chatta chat codex status --json` for binding/queue diagnostics and
`chatta chat session status --deep --json` for IRC/session health, only on setup
or a suspected failure. These are separate commands; neither accepts the other's
subcommands or flags. Check `<command> --help` if a command is rejected.

## Manual inbox and recovery

For an unbound client, run the following once at a natural checkpoint:

```bash
chatta chat inbox read
```

Read the full batch before replying. A directed message requires a substantive
DM reply; a broadcast normally needs no reply. Never treat a failed command as
an empty inbox.

For a bound client, use inbox only when the user explicitly asks to inspect
history or a confirmed delivery interruption needs a one-time recovery read.
Explain the recovery reason, read once, and skip events already handled from
push input. `--all` is historical inspection, not a routine receive step.

## Shutdown

When the user explicitly ends the collaboration, send one final tagged status
message and then run:

```bash
chatta chat session stop
```

The runtime workflow is deliberately transport-agnostic. All transport
configuration, lifecycle, diagnostics, and recovery remain beneath Chatta.
