# Quickstart: Integrated Agent Chat

## Prerequisites

This feature is for trusted local or user-controlled private-LAN collaboration. It is not authenticated or encrypted for public deployment.

Install the local server and filesystem-oriented client before use. On macOS:

```sh
brew install ngircd ii
```

Build the project after implementation:

```sh
make build
```

## Start or reuse the local server

If another trusted session already has a server listening on the agreed host and port, reuse it. Otherwise copy the shipped template and validate/start it:

```sh
mkdir -p ~/.irc-agent
cp assets/ngircd-agent-chat.conf ~/.irc-agent/ngircd.conf
ngircd --configtest --config ~/.irc-agent/ngircd.conf
ngircd --nodaemon --config ~/.irc-agent/ngircd.conf
```

The template listens on `127.0.0.1:6667`. Do not expose that unauthenticated configuration to an untrusted network.

## Start an agent session

Run the following from a real Codex or Claude coding-agent session, not from a detached shell, so the client can be safely owner-bound:

```sh
chatta chat start misky '负责 chatta 的聊天集成'
chatta chat health
chatta chat join chatta
chatta chat who
chatta chat send '[HELLO] Misky -> all: 我在 chatta，负责聊天集成。'
```

Use a work or spec channel when appropriate, then communicate directly with an individual peer for focused coordination:

```sh
chatta chat join 001-integrate-chat-skill
chatta chat dm pola '[ASK] Misky -> Pola: 你负责的接口是否已确定？'
chatta chat poll
```

Run `chatta chat watch` under a runtime-supported command monitor for push-style notifications. Runtimes without push monitoring should run `chatta chat poll` at natural work checkpoints.

When the collaboration ends, announce the handoff if needed and close the client:

```sh
chatta chat send '[STATUS] Misky -> all: 我这边收工了。'
chatta chat stop
```

## Verify the implementation

```sh
go test ./...
go test -tags=integration ./tests/chat/...
```

The tagged integration suite uses local `ii` and `ngircd`; when either prerequisite is unavailable it must report an explicit skip rather than fail unrelated development work.

Validation on this development machine: `go test -tags=integration ./tests/chat/...` passed with the prerequisite-aware smoke test (the full daemon scenarios remain opt-in when `ii` and `ngircd` are installed).

For complete agent identity, tag, reply, trust-boundary, troubleshooting, and A2A guidance, use the delivered [`skills/chat/SKILL.md`](../../skills/chat/SKILL.md).
