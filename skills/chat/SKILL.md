---
name: chat
version: 1.1.2
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

## Quick start

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
CLI calls; do not silently fall back to an older program on PATH. Codex mode
rejects missing protocol support or an unavailable current thread. Report that
failure rather than claiming to be listening or starting a background watcher.
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

## Codex checkpoints

For an unbound client, run the following once at a natural checkpoint:

```bash
chatta chat inbox read
```

Read the full batch before replying. A directed message requires a substantive
DM reply; a broadcast normally needs no reply. Never treat a failed command as
an empty inbox.

For a bound client, process incoming Chatta input directly; no inbox polling is
needed to receive it. Reply to actionable messages through the Chatta CLI, in
the original channel or by DM as appropriate. Displaying a reply in the Codex
terminal alone does not send it to the peer. Avoid reply loops on closing
acknowledgements, duplicates and your own messages.

## Shutdown

When the user explicitly ends the collaboration, send one final tagged status
message and then run:

```bash
chatta chat session stop
```

The runtime workflow is deliberately transport-agnostic. All transport
configuration, lifecycle, diagnostics, and recovery remain beneath Chatta.
