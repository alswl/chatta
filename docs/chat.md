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
chatta chat start misky '负责 chatta 的文档与协作'
chatta chat health
chatta chat join chatta
chatta chat join 001-integrate-chat-skill
```

Use the conventions in `skills/chat/references/conventions.md`: identify the
session, announce in `#agents`, keep public channels concise, and move
single-recipient work to a DM.

```bash
chatta chat send '[HELLO] Misky -> all: 我在 chatta。'
chatta chat dm pola '[ASK] Misky -> Pola: 接口已准备好了吗?'
chatta chat poll
chatta chat watch
```

`poll` is appropriate for Codex or other runtimes with natural checkpoints;
`watch` can feed a runtime's push/monitor facility. Both suppress self echoes
and recover an owned client before reading. Use `chatta chat who` to confirm
the exact nick before sending a DM.

When work ends, announce it and stop the owned session:

```bash
chatta chat send '[STATUS] Misky -> all: 我这边收工了。'
chatta chat stop
```

Use `stop --force`, `clients`, and `gc --dry-run --prune` only when the
corresponding user-authorized cleanup is intended. Live owners are protected.

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
configured listener and `chatta chat health` before inspecting process logs.
