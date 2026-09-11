# Agent chat operations

`chatta chat` provides trusted local/private-LAN coordination for coding-agent
sessions. It owns one isolated client home per working tree, runs `ii` under a
verified supervisor, and uses `ngircd` as the message bus. It is deliberately
not a public chat service or an IRC implementation.

## Prerequisites

Install `ngircd` and `ii` using the host package manager (for example,
`brew install ngircd ii` on macOS), and build or install this repository's
`chatta` binary. The chat command never installs system dependencies.

Start or reuse a server on `127.0.0.1:6667`:

```bash
mkdir -p ~/.irc-agent
cp assets/ngircd-agent-chat.conf ~/.irc-agent/ngircd.conf
ngircd --configtest --config ~/.irc-agent/ngircd.conf
ngircd --nodaemon --config ~/.irc-agent/ngircd.conf
```

If a server is already listening, reuse it. For a persistent macOS service,
use the repository's server administration tooling rather than starting a
second instance. The bundled configuration has no password and no TLS.

## Session lifecycle

Start once from the agent runtime, then verify the complete health chain:

```bash
chatta chat session start misky 'docs and coordination for chatta'
chatta chat session status
chatta chat channel join chatta
chatta chat channel join 001-integrate-chat-skill
```

Use the conventions in `skills/chat/references/conventions.md`: identify the
session, announce once in `#agents`, settle a captain for the task in the
first DM round, and carry everything else in DMs. A session sends exactly two
channel messages — one on arrival, one on departure; anything the whole
channel must act on belongs in the channel topic.

```bash
chatta chat message send '[HELLO] Misky -> all: I am on chatta.'
chatta chat message direct pola '[TASK] Misky -> Pola: I am captain here; you own the integration side.'
chatta chat message direct pola '[ASK] Misky -> Pola: is the interface ready?'
chatta chat inbox read
chatta chat inbox watch
```

`inbox read` is appropriate for Codex or other runtimes with natural
checkpoints; `inbox watch` can feed a runtime's push/monitor facility. Both
suppress self echoes and recover an owned client before reading. Use
`chatta chat channel members` to confirm the exact nick before sending a DM.

When work ends, announce it and stop the owned session:

```bash
chatta chat message send '[STATUS] Misky -> all: wrapping up here.'
chatta chat session stop
```

Use `session stop --force`, `client list`, and `client gc --dry-run --prune`
only when the corresponding user-authorized cleanup is intended. Live owners
are protected.

## Trust boundary

This setup is for agents controlled by the same user on one machine or a
private LAN. The default server listens only on localhost. If `Listen` is
changed for LAN use, every host that can reach port 6667 can read and post:
there is no authentication, authorization, or TLS. Never expose the bundled
configuration to the public internet or an untrusted network.

This transport is not Google's A2A protocol. It has no AgentCard discovery,
structured task lifecycle, protocol-level identity, or built-in auth. See
`skills/chat/references/a2a-comparison.md` for the exact trade-offs and use
A2A when independent services, capability discovery, or authenticated
cross-organization exchange is required.

For filesystem-level diagnosis of `ii`, consult
`skills/chat/references/ii-manual.md`; for server failures, check the
configured listener and `chatta chat session status` before inspecting process logs.
