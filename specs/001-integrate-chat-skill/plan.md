# Implementation Plan: Integrated Agent Chat

**Branch**: `001-integrate-chat-skill` | **Date**: 2026-08-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-integrate-chat-skill/spec.md`

## Summary

Absorb the standalone agent-chat wrapper into `chatta` as a `chatta chat` command group and a self-contained `chat` SKILL. Keep `ii` as the IRC client and `ngircd` as the local message bus; port the wrapper's session ownership, FIFO I/O, connection recovery, state, safety, and inspection logic into Go rather than implementing IRC. The delivery also ships the local-server template, operational references, and automated unit/component/integration coverage.

## Technical Context

**Language/Version**: Go 1.22
**Primary Dependencies**: Existing Cobra and Viper; Go standard library; promote `golang.org/x/sys` to a direct dependency for POSIX file locking, nonblocking FIFO access, and process-group operations
**Storage**: Versioned JSON state, JSON read cursors, lock files, logs, and `ii` conversation files under the configured client home
**Testing**: Go `testing` package for unit and component tests; tagged local-server smoke tests that skip with an explicit reason when `ii` or `ngircd` is unavailable
**Target Platform**: macOS and Linux on POSIX filesystems; Windows is explicitly unsupported for this feature
**Project Type**: Single Go CLI project
**Performance Goals**: A healthy start/join/health flow completes in under 15 seconds; the next command detects and recovers a failed client or returns a named failed link within 15 seconds
**Constraints**: Preserve the source wrapper's trusted-local security model; never implement IRC or auto-install system dependencies; never block indefinitely on a FIFO or signal a process based only on a recycled PID
**Scale/Scope**: Tens of concurrent local agent sessions per host, each with a distinct worktree client home, lobby/project/spec channels, and direct-message conversations

## Constitution Check

The project constitution is an uncustomized template and defines no enforceable principles or gates. This plan therefore passes the gate while applying the feature's explicit quality constraints:

- Keep command parsing in `cmd/chatta` and lifecycle/transport orchestration in `pkg/chat`.
- Preserve safe ownership, locking, process identity, and nonblocking FIFO invariants from the source.
- Cover pure parsing/state logic with automated tests and isolate tests needing `ii`/`ngircd` from normal test runs.
- Document that the transport is for trusted local/private-LAN coordination only and is not A2A or a public service.

**Post-design re-check**: PASS. The data model, command contract, state boundaries, and test layers below retain these constraints; no exception or additional complexity justification is required.

## Project Structure

### Documentation (this feature)

```text
specs/001-integrate-chat-skill/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── chatta-chat-cli.md
└── tasks.md                 # Generated later by /speckit-tasks
```

### Source Code (repository root)

```text
cmd/chatta/
├── root.go                  # Existing root command and shared configuration
├── chat.go                  # `chat` command group and shared chat flags
└── chat_*.go                # Thin Cobra adapters for chat subcommands

pkg/config/
└── config.go                # Extend with chat configuration and precedence

pkg/chat/
├── model.go                 # State, health, survey, cursor, and result types
├── paths.go                 # Worktree-derived homes and on-disk paths
├── state.go                 # Validation, atomic persistence, and cursors
├── channel.go               # Channel normalization and membership rules
├── owner_posix.go           # Agent-owner discovery and PID fingerprints
├── process_posix.go         # Locks, process inspection, groups, signals
├── ii.go                    # `ii` launch, FIFO I/O, and log parsing
├── supervisor.go            # Hidden supervisor and restart/rejoin lifecycle
├── health.go                # Health, TIME/NAMES validation, Ensure recovery
├── messages.go              # Send, DM, poll, watch, rendering, UTF-8 splitting
├── fleet.go                 # Client survey and conservative garbage collection
└── *_test.go                # Unit and POSIX component coverage

assets/
└── ngircd-agent-chat.conf   # Trusted-local server template

skills/chat/
├── SKILL.md
├── assets/
│   └── ngircd-agent-chat.conf
└── references/
    ├── conventions.md
    ├── ii-manual.md
    └── a2a-comparison.md

docs/
└── chat.md                  # Project-level operational and trust-boundary guide

tests/chat/
└── integration_test.go      # Tagged two-client local-server smoke tests
```

**Structure Decision**: Use one cohesive `pkg/chat` package to own all persistent state and `ii` lifecycle behavior. The Cobra layer remains intentionally thin, so unit/component tests can exercise the manager directly and the CLI contract does not duplicate lifecycle logic. Platform-specific process mechanics stay in POSIX-suffixed files; source-derived assets and agent guidance are shipped separately from Go code but live in this repository.

## Implementation Approach

1. Establish configuration, paths, state schema, channel/message helpers, and process abstractions first. Default to a deterministic per-worktree client home under the legacy-compatible `~/.irc-agent/clients/` root; use `CHATTA_CHAT_*` settings as the primary configuration surface and accept documented `AGENT_CHAT_*` aliases during migration.
2. Implement the hidden supervisor with an exclusive client-home lock. Bind it to a verifiable Codex/Claude owner fingerprint, run `ii` in its own process group, restart/rejoin only while that owner remains live, and stop the group when the owner exits.
3. Build `Ensure`, health, FIFO, `TIME`, and `NAMES` behavior before any public message command. Every normal operation goes through this path so commands cannot send to a dead reader or an unconfirmed channel.
4. Add the public command family according to the CLI contract: lifecycle, channel/DM messaging, poll/watch, participant lookup, stop, fleet inspection, and conservative cleanup.
5. Port the server configuration and operational documentation into the repository, then write the `chat` SKILL around `chatta chat` rather than the old Python script. Preserve the conventions and truthful A2A boundary.
6. Build tests from pure deterministic tests outward: parsing/state, fake-`ii` process/FIFO components, then opt-in `ngircd` plus two-client smoke coverage. Keep normal `go test ./...` independent of locally installed daemon/client binaries.

## Complexity Tracking

No constitution violations require justification.
