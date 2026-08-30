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
chatta chat session start misky '负责 chatta 的聊天集成'
chatta chat session status
chatta chat channel join chatta
chatta chat channel members
chatta chat message send '[HELLO] Misky -> all: 我在 chatta，负责聊天集成。'
```

Use a work or spec channel when appropriate, then communicate directly with an individual peer for focused coordination:

```sh
chatta chat channel join 001-integrate-chat-skill
chatta chat message direct pola '[ASK] Misky -> Pola: 你负责的接口是否已确定？'
chatta chat inbox read
```

Run `chatta chat inbox watch` under a runtime-supported command monitor for push-style notifications. Runtimes without push monitoring should run `chatta chat inbox read` at natural work checkpoints.

## Automated verification fixture

Normal sessions must be started from a real Codex or Claude runtime. External
black-box verification can instead bind the client to an explicit, disposable
fixture process. This is only for automated tests: it does not replace the
normal runtime-owner requirement for interactive use.

```sh
sleep 300 &
fixture_owner=$!
export CHATTA_CHAT_TEST_OWNER_PID="$fixture_owner"
chatta chat start verifier 'automated verification fixture'
# Run verification commands with the same environment, then reclaim it:
kill "$fixture_owner"
```

The fixture PID is recorded with its process-start fingerprint, so a dead or
recycled PID is rejected just like a normal owner. Use a temporary
`CHATTA_CHAT_HOME` and an isolated `ngircd` port for each verification run.

When the collaboration ends, announce the handoff if needed and close the client:

```sh
chatta chat message send '[STATUS] Misky -> all: 我这边收工了。'
chatta chat session stop
```

## Verify the implementation

```sh
go test ./...
go test -tags=integration ./tests/chat/...
```

The tagged integration suite starts an isolated local `ngircd` and exercises two `ii` clients, channel/DM delivery, and owner-death handling. When either prerequisite is unavailable it reports an explicit skip rather than fail unrelated development work.

Validation on this development machine: `go test -tags=integration ./tests/chat/...` passed against an isolated daemon fixture.

For complete agent identity, tag, reply, trust-boundary, troubleshooting, and A2A guidance, use the delivered [`skills/chat/SKILL.md`](../../skills/chat/SKILL.md).
