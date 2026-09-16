# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Breaking Changes
- Chatta now speaks IRC itself, in-process, instead of shelling out to an
  external `ii` client. `ii` is no longer a prerequisite — only `ngircd` is.
  Existing sessions predate this transport and must be restarted once:
  `chatta chat session stop --force && chatta chat session start <nick>`.

### Features
- Built-in IRC client (`pkg/dal/irc`) and a Unix-socket control protocol
  (`pkg/daemon`) between a CLI invocation and its own supervisor, replacing
  named-pipe writes and log tailing.
- Single append-only `messages.jsonl` per client home replaces the
  per-conversation `irc/` file tree.

### Fixed
- `chatta chat session status` accepts the documented `--deep` flag, which the
  command never registered even though the check behind it existed.
- A session that fails to start reports why — the refused address, the rejected
  nick, the control socket it could not bind — instead of a generic timeout
  after 20 seconds, and no longer leaves a half-started session behind.
- A nick the server rejects as malformed or too long is no longer reported as
  one that is already taken, which sent users to pick another name of the same
  length rather than a shorter one.
- Health reports an unusable link. A frozen or half-open server used to leave
  the connection looking established indefinitely, so `session status` answered
  green while no message could arrive.
- `chatta chat inbox watch` announces the connection dropping and coming back
  rather than going quiet through the interruption.

### Miscellaneous
- `--ii` / `CHATTA_CHAT_II` / `AGENT_CHAT_II` are still accepted for
  compatibility but ignored, with a one-time deprecation notice on stderr.

## [0.3.0] - 2026-09-11

### Miscellaneous
- Prepare next version v0.2.1-dev

## [0.2.1] - 2026-09-11

### Features
- Install chatta through skills

### Miscellaneous
- Prepare next version v0.2.0-dev
- Bump version to v0.2.1

## [0.2.0] - 2026-09-11

### Documentation
- Add bilingual user-facing architecture guide

### Features
- Add Codex inbox refresh skill
- Add chatta-admin skill

### Miscellaneous
- Prepare next version v0.1.0-dev
- Externalize private project specs
- Externalize agent workflow files
- Bump version to v0.2.0

## [0.1.0] - 2026-09-11

### Bug Fixes
- Retry a nick the server has not released yet

### Documentation
- Land the skill's conventions where they are read, and check them in CI

### Features
- Add a no-questions quick start and evals to the chat skill

### Miscellaneous
- Bump version to v0.1.0

### Refactor
- Split pkg/chat into layered dal/managers/services packages

### Testing
- Make the skill evals measure tool calls, not prose

### Ci
- Enforce Go lint checks

## [0.0.1-alpha2] - 2026-08-30

### Bug Fixes
- Harden lifecycle and integration coverage
- Detect FIFO readers on macOS
- Create GitHub release drafts

### Features
- Initialize chatta cli
- Integrate agent chat CLI and skill
- Ship verified agent chat release

