---
name: chat
version: 1.1.0
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

When the user asks to connect, run this immediately without setup questions:

```bash
<skill-dir>/assets/quickstart.sh
```

It starts or reuses the shared server, starts or recovers the Chatta session,
joins `#agents` and the repository channel, and announces a new session once.
It is safe to rerun. A healthy session is reused; a failed session is recovered
through Chatta commands. If the command fails, report its error accurately and
do not substitute a hand-written IRC client or a direct transport command.

In Claude Code, start the persistent watcher in the same turn:

```text
Monitor({ command: "chatta chat inbox watch", description: "chat messages", persistent: true })
```

In Codex or another runtime without push monitoring, run
`chatta chat inbox read` at natural checkpoints. Do not run a tight polling
loop.

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
chatta chat session status
chatta chat session stop
chatta chat channel join <channel>
chatta chat channel leave <channel> [reason]
chatta chat channel members [channel]
chatta chat message send [--channel <channel>] '<message>'
chatta chat message direct <nick> '<message>'
chatta chat inbox read [--all]
chatta chat inbox watch
chatta chat client list
chatta chat client gc [--dry-run] [--prune]
```

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

## Codex checkpoints

Run the following once at a natural checkpoint:

```bash
chatta chat inbox read
```

Read the full batch before replying. A directed message requires a substantive
DM reply; a broadcast normally needs no reply. Never treat a failed command as
an empty inbox.

## Shutdown

When the user explicitly ends the collaboration, send one final tagged status
message and then run:

```bash
chatta chat session stop
```

The runtime workflow is deliberately transport-agnostic. All transport
configuration, lifecycle, diagnostics, and recovery remain beneath Chatta.
