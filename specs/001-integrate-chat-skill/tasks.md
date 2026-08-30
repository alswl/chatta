# Tasks: Integrated Agent Chat

**Input**: Design documents from `/specs/001-integrate-chat-skill/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [chatta-chat-cli.md](./contracts/chatta-chat-cli.md), [quickstart.md](./quickstart.md)

**Tests**: Included. FR-019 explicitly requires automated smoke coverage for normal lifecycle, interruption recovery, and safety-sensitive failure cases. Unit tests and POSIX component tests run normally; real `ii` + `ngircd` smoke tests are build-tagged and skip explicitly when their prerequisites are unavailable.

**Organization**: Tasks are grouped by user story. No task changes the old standalone skill directory; all delivered code, assets, and guidance live in this repository.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can be worked on in parallel because it uses different files and has no unfinished prerequisite in this list.
- **[US#]**: Maps the task to a user story in [spec.md](./spec.md).

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the dependency and test scaffolding needed by every implementation phase.

- [X] T001 [P] Promote `golang.org/x/sys` to a direct POSIX dependency in `go.mod` and refresh `go.sum`.
- [X] T002 [P] Create the integration-test package and build-tag guard in `tests/chat/integration_test.go` so normal `go test ./...` does not require `ii` or `ngircd`.
- [X] T003 [P] Add chat-specific environment examples and configuration comments to `.env.example` for `CHATTA_CHAT_HOME`, `CHATTA_CHAT_HOST`, `CHATTA_CHAT_PORT`, `CHATTA_CHAT_CHANNEL`, and `CHATTA_CHAT_II`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish validated state, POSIX process safety, and `ii` filesystem primitives that every user story uses.

**⚠️ CRITICAL**: Complete this phase before starting user-story command work.

- [X] T004 Extend `pkg/config/config.go` with chat configuration, flag/environment/config-file/default precedence, and documented `AGENT_CHAT_*` migration aliases.
- [X] T005 [P] Define `ChatSession`, `OwnerBinding`, `HealthReport`, `Conversation`, `MessageCursor`, and `ClientSurvey` types in `pkg/chat/model.go` from `data-model.md`.
- [X] T006 [P] Write deterministic worktree-home and path-resolution cases in `pkg/chat/paths_test.go`.
- [X] T007 Implement deterministic per-worktree homes, configurable path resolution, and conversation-directory discovery in `pkg/chat/paths.go`.
- [X] T008 [P] Write channel-normalization and home-channel validation cases in `pkg/chat/channel_test.go`.
- [X] T009 Implement normalized channel names, ordered membership validation, and home-channel protection in `pkg/chat/channel.go`.
- [X] T010 Write state-schema, atomic-save, malformed-state, per-session-cursor, and truncated-log reset cases in `pkg/chat/state_test.go`.
- [X] T011 Implement versioned state validation, atomic persistence, exclusive state helpers, and cursor-map persistence in `pkg/chat/state.go`.
- [X] T012 Write POSIX owner-discovery, PID/start-fingerprint, and foreign-owner conflict cases in `pkg/chat/owner_posix_test.go`.
- [X] T013 Implement recognized-agent owner discovery and live PID/start-fingerprint validation in `pkg/chat/owner_posix.go`.
- [X] T014 Write verified-supervisor, process-group termination, and stale-PID safety cases in `pkg/chat/process_posix_test.go`.
- [X] T015 Implement POSIX advisory locking, process inspection, detached process groups, and validated TERM/KILL helpers in `pkg/chat/process_posix.go`.
- [X] T016 Write nonblocking-FIFO, `ii` output parsing, self-echo filtering, `/TIME`, and `/NAMES` response cases in `pkg/chat/ii_test.go`.
- [X] T017 Implement `ii` command launching, nonblocking FIFO write/probe, transcript tailing, and IRC response parsing in `pkg/chat/ii.go`.

**Checkpoint**: State, ownership, process, FIFO, and parsing primitives are testable in isolation; no public chat command exists yet.

---

## Phase 3: User Story 1 - Start a durable agent chat session (Priority: P1) 🎯 MVP

**Goal**: An agent can start, assess, and safely close a worktree-isolated session that is tied to the owner runtime and its client process group.

**Independent Test**: From a supported agent-runtime fixture, start a session with a fake/real `ii`, receive healthy owner/supervisor/client/server results, prove a foreign live owner is refused, then end the owner and verify both supervisor and client terminate.

### Tests for User Story 1

- [X] T018 [P] [US1] Write start/takeover/nickname-conflict and owner-death lifecycle component tests in `pkg/chat/supervisor_test.go`.
- [X] T019 [P] [US1] Write health-chain and named-failure-report tests in `pkg/chat/health_test.go`.
- [X] T020 [P] [US1] Write Cobra contract tests for `chat start`, `chat health`, and hidden `_supervise` visibility in `cmd/chatta/chat_lifecycle_test.go`.

### Implementation for User Story 1

- [X] T021 [US1] Implement the one-lock-per-home detached supervisor, owner heartbeat loop, `ii` process-group lifecycle, and safe start/takeover behavior in `pkg/chat/supervisor.go`.
- [X] T022 [US1] Implement health checks for owner, verified supervisor, joined FIFO, client reader, and `/TIME` server round trip in `pkg/chat/health.go`.
- [X] T023 [US1] Add the `chat` command group, shared chat configuration flags, and hidden `_supervise` command in `cmd/chatta/chat.go`.
- [X] T024 [US1] Add `chat start` and `chat health` adapters with contract-compatible output and errors in `cmd/chatta/chat.go`.
- [X] T025 [US1] Add prerequisite-aware start/health/owner-death integration smoke-test harness in `tests/chat/integration_test.go`.

**Checkpoint**: US1 is a usable MVP: a real agent session can safely start, verify, and naturally clean up a local presence.

---

## Phase 4: User Story 2 - Coordinate in channels and direct messages (Priority: P1)

**Goal**: An active agent can reliably use project/work channels and direct messages, discover peers, and consume only incoming traffic with independent cursors.

**Independent Test**: Start two isolated client homes, have both join a work channel, exchange channel and direct messages, list membership, poll unread traffic, and confirm self echoes and one session's cursor do not hide data from the other.

### Tests for User Story 2

- [X] T026 [P] [US2] Write join/part, remembered-membership, unjoined-send, and confirmed-`NAMES` race tests in `pkg/chat/channel_test.go`.
- [X] T027 [P] [US2] Write multi-line UTF-8 chunking, channel send, first-DM, absent-peer, poll, source-label, and cursor-isolation tests in `pkg/chat/messages_test.go`.
- [X] T028 [P] [US2] Write Cobra contract tests for `join`, `part`, `send`, `dm`, `poll`, and `who` in `cmd/chatta/chat_messaging_test.go`.

### Implementation for User Story 2

- [X] T029 [US2] Extend `pkg/chat/channel.go` with persistent join/part operations, server membership confirmation, and unjoined-send rejection.
- [X] T030 [US2] Implement channel sends, private-query opening, unknown-nick detection, UTF-8-safe chunking/pacing, transcript rendering, poll cursors, and participant lookup in `pkg/chat/messages.go`.
- [X] T031 [US2] Add `chat join`, `part`, `send`, `dm`, `poll`, and `who` Cobra adapters in `cmd/chatta/chat.go`.
- [X] T032 [US2] Extend the prerequisite-aware two-client smoke-test harness in `tests/chat/integration_test.go`.

**Checkpoint**: US1 and US2 together provide normal, independently verifiable agent coordination without silently dropping messages to unjoined channels or absent peers.

---

## Phase 5: User Story 3 - Recover from interrupted connections (Priority: P2)

**Goal**: Operational commands repair a failed but still-owned client, restore channels safely, and provide conservative inspection and cleanup of abandoned clients.

**Independent Test**: Kill a live owner's `ii`, invoke a collaboration command, confirm reconnection and rejoin before delivery; separately verify watch recovery, stale-process cleanup, and the guarantee that garbage collection leaves a live owner alone.

### Tests for User Story 3

- [X] T033 [P] [US3] Write Ensure recovery, rejoin-before-send, and long-running watch recovery/owner-exit tests in `pkg/chat/health_test.go`.
- [X] T034 [P] [US3] Write client survey, dry-run, ended-owner cleanup, stray-client reap, and live-owner no-op tests in `pkg/chat/fleet_test.go`.
- [X] T035 [P] [US3] Write Cobra contract tests for `watch`, `stop --force`, `clients`, and `gc` in `cmd/chatta/chat_operations_test.go`.

### Implementation for User Story 3

- [X] T036 [US3] Add Ensure recovery, all-channel rejoin, `/NAMES` confirmation before sends, and periodic watch health repair to `pkg/chat/health.go`.
- [X] T037 [US3] Add incoming-only stream watching, dynamic direct-conversation discovery, owner-exit shutdown, and safe clean stop behavior to `pkg/chat/messages.go`.
- [X] T038 [US3] Implement all-home inspection and conservative `clients`/`gc --dry-run --prune` behavior in `pkg/chat/fleet.go`.
- [X] T039 [US3] Add `chat watch`, `stop`, `clients`, and `gc` Cobra adapters in `cmd/chatta/chat.go`.
- [X] T040 [US3] Extend prerequisite-aware daemon smoke coverage in `tests/chat/integration_test.go`.

**Checkpoint**: US3 protects agents from ordinary client interruptions and stale local state without risking another live agent's session.

---

## Phase 6: User Story 4 - Use the chat skill consistently (Priority: P2)

**Goal**: An agent can operate the repository-owned CLI through a standalone SKILL that establishes collaboration conventions and does not overstate the transport's trust or A2A capabilities.

**Independent Test**: In a fresh agent session, follow only the delivered SKILL to check prerequisites, start/reuse the local bus, establish identity and a startup handshake, choose watch or poll, exchange a tagged message, and close cleanly.

- [X] T041 [P] [US4] Add the trusted-local `ngircd` template in `assets/ngircd-agent-chat.conf` and mirror it in `skills/chat/assets/ngircd-agent-chat.conf`.
- [X] T042 [P] [US4] Port identity, spaces, tagged-message, startup-handshake, reply, and trust conventions into `skills/chat/references/conventions.md`.
- [X] T043 [P] [US4] Port `ii` behavior/troubleshooting and honest A2A comparison references into `skills/chat/references/ii-manual.md` and `skills/chat/references/a2a-comparison.md`.
- [X] T044 [US4] Write `skills/chat/SKILL.md` around `chatta chat`, with prerequisite checks, server reuse/setup, command examples, monitor-versus-poll workflows, safety rules, and shutdown behavior.
- [X] T045 [US4] Write the project operational guide and LAN/public-service boundary in `docs/chat.md`.
- [X] T046 [US4] Validate every command and relative asset/reference path in `skills/chat/SKILL.md` against `contracts/chatta-chat-cli.md` and record the result in `skills/chat/SKILL.md`.

**Checkpoint**: The repository independently ships the code, template, and agent guidance formerly spread across the standalone chat skill.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verify the complete feature against contracts, quality gates, and the quickstart.

- [X] T047 [P] Add prerequisite-aware end-to-end CLI validation harness in `tests/chat/integration_test.go`.
- [X] T048 [P] Add focused regression coverage for malformed state and oversized Unicode text in `pkg/chat/regression_test.go`.
- [X] T049 Update CLI help text and configuration documentation to match the shipped contract in `cmd/chatta/chat.go` and `docs/chat.md`.
- [X] T050 Run formatting and normal automated tests with `gofmt` on `cmd/chatta/*.go` and `pkg/chat/*.go`, then `go test ./...`; fix any failures in the affected source files.
- [X] T051 Run the real-daemon verification prescribed by `specs/001-integrate-chat-skill/quickstart.md` and document any explicit prerequisite skip or failure in `specs/001-integrate-chat-skill/quickstart.md`.
- [X] T052 Verify all requirements FR-001 through FR-019 against the delivered code, assets, SKILL, and tests; update `specs/001-integrate-chat-skill/checklists/requirements.md` with the final evidence.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 — Setup**: No dependencies; T001–T003 can run in parallel.
- **Phase 2 — Foundational**: Depends on setup; T004–T017 block every story command.
- **Phase 3 — US1**: Depends on Phase 2 and is the MVP.
- **Phase 4 — US2**: Depends on US1's session/supervisor lifecycle and Phase 2.
- **Phase 5 — US3**: Depends on US1/US2 operational commands and Phase 2 because recovery must prove it can resume normal work.
- **Phase 6 — US4**: Can begin its reference/template tasks after Phase 1; T044–T046 depend on the completed CLI contract from US1–US3.
- **Phase 7 — Polish**: Depends on all desired stories.

### User Story Completion Order

```text
Setup -> Foundational -> US1 (MVP) -> US2 -> US3
                       \-> US4 documentation/reference work (then validate against US1–US3)
US2 + US3 + US4 -> Polish
```

### Parallel Opportunities

- Setup tasks T001–T003 are independent.
- In the foundation, model/path/channel work and their test files can begin in parallel where marked `[P]`; complete each test/code pair before the layers that consume it.
- Within each story, explicitly marked test files can be written in parallel before the implementation tasks they specify.
- US4 template and reference tasks T041–T043 can run alongside lifecycle/messaging implementation, while the SKILL itself waits for final command behavior.
- Final integration and regression test additions T047–T048 can run in parallel once their target packages exist.

## Parallel Example: User Story 2

```text
Task: "Write join/part and confirmed-NAMES race tests in pkg/chat/channel_test.go"
Task: "Write message, DM, poll, and cursor tests in pkg/chat/messages_test.go"
Task: "Write Cobra messaging contract tests in cmd/chatta/chat_messaging_test.go"
```

After those tests are in place, implement `pkg/chat/channel.go` (T029), then `pkg/chat/messages.go` (T030), followed by the Cobra adapters (T031) and real-daemon smoke extension (T032).

## Implementation Strategy

### MVP First

1. Finish Setup and Foundational phases.
2. Complete US1 through T025.
3. Run the US1 component and smoke tests, including owner-death cleanup.
4. Demonstrate a real agent session start/health/stop flow before adding messaging.

### Incremental Delivery

1. US1 delivers safe, durable presence.
2. US2 adds the normal collaboration loop: channels, DMs, polling, and presence lookup.
3. US3 makes that loop resilient to interrupted clients and abandoned state.
4. US4 makes the resulting CLI usable by other agent runtimes without the old standalone directory.
5. Polish validates the full contract and quickstart.

## Notes

- Every task follows the required checkbox, sequential ID, optional parallel marker, story label (for user-story work), and exact-path format.
- `start --takeover`, `stop --force`, and mutating cleanup remain explicitly user-authorized operations; no task may weaken those guards.
- The implementation must use argument-vector process execution and validated process identities; do not add shell interpolation or a native IRC protocol implementation.
