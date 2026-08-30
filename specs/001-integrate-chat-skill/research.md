# Research: Integrated Agent Chat

## Decision 1: Retain `ii` and `ngircd`; do not implement IRC in `chatta`

**Decision**: Port the standalone wrapper's orchestration logic to Go, while continuing to invoke `ii` for IRC protocol/client behavior and `ngircd` for the local server.

**Rationale**: The source intentionally delegates IRC protocol semantics, connection layout, and conversation files to `ii`. Reimplementing IRC would expand scope, alter measured `ii` behavior relied on by polling/DMs, and add a protocol maintenance burden unrelated to the requested CLI and SKILL.

**Alternatives considered**:

- Native Go IRC client: rejected because it changes the operational model and duplicates behavior provided by `ii`.
- Preserve the Python script as a `chatta` subprocess: rejected because the requested logic would remain outside this project and would retain a Python runtime dependency.

## Decision 2: Make the feature a POSIX-only Go implementation

**Decision**: Support macOS and Linux initially, encapsulating `flock`, nonblocking FIFO access, process groups, and process inspection behind POSIX-specific files.

**Rationale**: The source depends on POSIX FIFOs, signals, process groups, `ps` and `pgrep`; `ii` itself has the same operating assumptions. Making unsupported platforms explicit preserves safety instead of offering a partial lifecycle implementation.

**Alternatives considered**:

- Cross-platform emulation: rejected because it would not support the required `ii` process/FIFO behavior and would dilute test coverage.
- macOS-only: rejected because the source's model is usable on Linux with the same documented dependency family.

## Decision 3: Preserve lifecycle safety as the primary architectural boundary

**Decision**: Represent owner identity as PID plus process-start fingerprint, take an exclusive lock per client home, validate command identity before signalling a supervisor, and terminate the supervisor and its `ii` process group together.

**Rationale**: These controls prevent PID reuse from targeting unrelated processes, prevent same-worktree sessions from silently replacing each other, and prevent an orphaned `ii` process from retaining a nickname after its owner has gone away.

**Alternatives considered**:

- PID-only ownership: rejected because PIDs are recycled.
- One shared client per repository: rejected because worktrees and independent sessions would share identity, messages, and cursors.
- Best-effort process cleanup without validation: rejected because process-group signalling has unacceptable blast radius.

## Decision 4: Use state files owned by `chatta`, with compatibility-aware configuration

**Decision**: Store versioned JSON state, per-session cursor maps, lock files, and logs under a deterministic per-worktree home. Flags override `CHATTA_CHAT_*`, configuration-file values, documented legacy `AGENT_CHAT_*` aliases, and defaults in that order.

**Rationale**: Separate homes retain the source behavior for worktrees and prevent cursor loss between agent sessions, while the primary `CHATTA_` namespace fits this project's Viper configuration convention.

**Alternatives considered**:

- Shared global state: rejected because it breaks the one-worktree/one-agent safety model.
- A database service: rejected as unnecessary for local, file-backed sessions.
- Drop legacy aliases immediately: rejected because the existing SKILL ecosystem already uses them and a low-cost migration path avoids needless breakage.

## Decision 5: Use an explicit health-and-recovery gate for every operational command

**Decision**: `send`, `dm`, `join`, `part`, `poll`, `watch`, and `who` all call a common Ensure path that checks the owner, supervisor, FIFO reader, server `TIME` response, and confirmed `NAMES` membership; it repairs missing membership or safely restarts/rejoins before the requested operation.

**Rationale**: FIFO existence alone is not proof that a message can be delivered. The source has production-learned guards for dead readers, reconnect races, and `ii` behavior where `PING` is not a usable liveness test.

**Alternatives considered**:

- Let each command inspect only its needed file: rejected because recovery would be inconsistent and writes could hang or be dropped.
- Retry messages blindly: rejected because it can send to the wrong lifecycle state and masks the real failed link.

## Decision 6: Keep CLI parsing thin and document the full command schema

**Decision**: Group `chatta chat` commands by the entities they operate on: `session start|status|stop`, `channel join|leave|members`, `message send|direct`, `inbox read|watch`, and `client list|gc`. Keep the former flat commands as hidden compatibility aliases, and reserve `_supervise` as a hidden implementation command.

**Rationale**: The command tree now mirrors the domain model: session lifecycle, channel membership, outbound messages, per-invoker inbox cursors, and local-client administration. This keeps the public help discoverable as the command set grows, while aliases preserve existing SKILL and shell integrations.

**Alternatives considered**:

- Top-level commands: rejected because they would crowd `chatta` as the product grows.
- Breaking rename: rejected because deployed SKILLs and shell scripts need a safe migration path.
- One JSON-driven mega-command: rejected because it obscures shell and SKILL usage.

## Decision 7: Test from deterministic units through opt-in real-daemon smoke tests

**Decision**: Cover parsing, UTF-8 chunking, cursor handling, state validation, owner conflicts, and cleanup selection with normal Go tests. Use fake `ii`/FIFOs for POSIX components. Add build-tagged two-client smoke tests against a temporary `ngircd` configuration, skipping with an explicit reason when external binaries are absent.

**Rationale**: This preserves the original lifecycle smoke coverage without requiring system daemons in every contributor or CI environment, while still validating the real `ii` semantics the wrapper relies on.

**Alternatives considered**:

- Only unit tests: rejected because supervisor/FIFO/reconnect behavior is process-level.
- Require `ii` and `ngircd` in all CI: rejected because it creates a fragile, platform-coupled default test gate.

## Decision 8: Ship a self-contained SKILL with an honest trust boundary

**Decision**: Place the SKILL, local-server template, identity/message conventions, `ii` troubleshooting reference, and A2A comparison under `skills/chat/`, with project-level operational documentation in `docs/chat.md`.

**Rationale**: Agents need the behavioral conventions alongside the CLI. The source accurately limits this design to trusted local/private-LAN use: it has no agent discovery, task state machine, transport authentication, or public-service security model.

**Alternatives considered**:

- Only CLI help: rejected because it cannot establish the collaboration conventions that prevent duplicate or unanswered work.
- Describe the system as an A2A replacement: rejected because it would misrepresent its capability and security boundaries.
