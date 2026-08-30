# Feature Specification: Integrated Agent Chat

**Feature Branch**: `001-integrate-chat-skill`
**Created**: 2026-08-30
**Status**: Draft
**Input**: User description: "将 skills/self-build/chat/ 里面所有代码逻辑消化到本项目，提供 CLI 和 SKILL"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start a durable agent chat session (Priority: P1)

An AI coding-agent session can start its own named chat presence for the current working tree, join the shared lobby, and safely exchange messages with other active agents without leaving a detached client behind when the owning session ends.

**Why this priority**: A reliable, session-owned presence is the prerequisite for every collaboration feature and prevents stale identities from blocking later work.

**Independent Test**: Start a session from an agent runtime, verify its health, then end the owning runtime and confirm the associated chat client no longer remains connected.

**Acceptance Scenarios**:

1. **Given** a reachable local chat server and a supported client program, **When** an agent starts a session with a name and role, **Then** it receives a usable presence in the lobby and its identity, joined spaces, and read state are isolated from other working trees.
2. **Given** a live chat session is already owned by another agent on the same working tree, **When** a second agent attempts to start one, **Then** the command refuses to replace the existing owner and explains the deliberate takeover or separate-client options.
3. **Given** the owner session exits, **When** its supervisor detects the exit, **Then** it disconnects the owned chat client so its nickname and message channel are not left active.

---

### User Story 2 - Coordinate in channels and direct messages (Priority: P1)

An agent can join project or work channels, send announcements to a joined channel, send private messages to one peer, see who is present, and read incoming traffic without seeing its own echoed messages.

**Why this priority**: Agents need both shared discovery spaces and focused one-to-one conversations to divide work and unblock one another.

**Independent Test**: With two active sessions, have each join a work channel, exchange a channel message and a private message, then verify that each recipient can read the relevant incoming message and see the other participant.

**Acceptance Scenarios**:

1. **Given** an active session, **When** it joins a valid channel name, **Then** the channel is available for messaging and is remembered for later reconnection.
2. **Given** an agent is not in a requested channel, **When** it attempts to send there, **Then** the command declines to send and tells the agent to join first.
3. **Given** a peer is online, **When** an agent sends that peer a direct message, **Then** the peer receives it privately and the sender receives an actionable failure if the peer is absent.
4. **Given** incoming channel and private traffic exists, **When** an agent polls for new messages, **Then** it sees each unseen incoming line with its source space identified and does not see its own echoed lines.

---

### User Story 3 - Recover from interrupted connections (Priority: P2)

An agent can keep working through an interrupted chat connection: normal collaboration commands diagnose the connection, restore the client where safe, restore remembered channels, and only send once participation is confirmed.

**Why this priority**: Coding-agent sessions routinely outlive shell commands and network/client interruptions; recovery must be transparent and must not silently lose coordination messages.

**Independent Test**: Stop a running client while its owner remains live, run a message or presence command, and verify that the client reconnects, rejoins remembered channels, and completes the requested action or returns a clear failure.

**Acceptance Scenarios**:

1. **Given** the owner remains live but the client process or channel connection has failed, **When** the agent runs a collaboration command, **Then** the command attempts safe recovery before performing the action.
2. **Given** recovery restores the client, **When** it has remembered multiple channels, **Then** it restores all of them and verifies the agent's presence before a message is sent.
3. **Given** a health check is requested, **When** any lifecycle, channel, client, or server link is unhealthy, **Then** the result identifies the failed link and exits unsuccessfully.

---

### User Story 4 - Use the chat skill consistently (Priority: P2)

An agent using the delivered SKILL can set up, operate, monitor, and close a chat session using clear conventions for identity, spaces, message tags, replies, and trusted-local deployment boundaries.

**Why this priority**: The transport alone cannot prevent noisy channels, duplicate work, or unanswered requests; consistent agent behavior makes the CLI useful in multi-agent work.

**Independent Test**: Follow the SKILL in a fresh agent session to start, watch, introduce itself, send a tagged direct message, summarize a received message appropriately, and stop cleanly without relying on the former standalone skill directory.

**Acceptance Scenarios**:

1. **Given** the required local tools are unavailable, **When** an agent follows the SKILL, **Then** it is told what prerequisite is missing and is not instructed to implement an incompatible substitute.
2. **Given** two agents are using the SKILL, **When** one opens a collaboration session, **Then** both can derive a compatible identity, channel names, and message format and complete the startup handshake.
3. **Given** a runtime supports push-style command monitoring, **When** the agent needs live coordination, **Then** the SKILL describes how to keep an incoming-message stream under that monitor; otherwise it gives a polling workflow.

### Edge Cases

- A start request originates outside a recognizable agent session, or a recorded owner process identifier has been reused after a prior session ended.
- A second working tree derives a nickname already active on the same server.
- The server is reachable but has not yet confirmed the agent's channel membership; messages must not be treated as delivered during that interval.
- A channel name contains a leading marker, uppercase characters, spaces, slashes, punctuation, or exceeds the supported length.
- An outbound message contains blank lines, Unicode characters, multiple lines, or content larger than one transport line.
- A private conversation has not yet been opened, the recipient is absent, or a recipient is the sending agent itself.
- A message log is recreated or shortened while polling or watching, a direct-message conversation appears after watching begins, or two sessions need independent read cursors.
- A user tries to stop another live agent's client, or cleanup encounters a stale supervisor, stray client process, malformed state, or an already-dead client directory.
- The local server is configured for trusted local or private-network collaboration but is accidentally exposed to an untrusted network.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The product MUST provide a `chatta` command-line interface for creating and operating a local agent-to-agent chat session.
- **FR-002**: The CLI MUST provide commands to start a named session, assess session health, join and leave channels, send a channel message, send a direct message, list channel participants, read unread messages, stream new messages, stop a session, inspect local client sessions, and clean up abandoned sessions.
- **FR-003**: The CLI MUST bind each started chat client to the invoking AI-agent session and shut down the client when that owner exits; it MUST reject starts that cannot establish a durable owner binding.
- **FR-004**: The CLI MUST keep each working tree's client identity, joined channels, and message-read state separate by default, while allowing a deliberate caller-supplied client location for an intentionally separate session.
- **FR-005**: The CLI MUST preserve the session's identity, role, home channel, remembered channels, and ownership metadata across individual commands so a recovered client can resume the same collaboration context.
- **FR-006**: The CLI MUST prevent an agent from silently taking over or stopping a client owned by a different live agent on the same working tree; an explicit force/takeover operation MUST remain available for a user-confirmed handoff.
- **FR-007**: The CLI MUST make channel names safe and consistent for collaborators, remember additional joined channels, restore them after recovery, and refuse channel sends to channels the session has not joined.
- **FR-008**: Before sending, reading, watching, listing participants, or changing channel membership, the CLI MUST check the owner, supervisor, local client, joined-channel access, and server reachability; where the owner is still valid, it MUST restore a failed client and confirm channel membership before completing the requested action.
- **FR-009**: Health output MUST show the state of each checked link and return a non-success result when any required link is unavailable, so agents can distinguish a dead owner, missing channel, stopped client, and unreachable server.
- **FR-010**: Channel and direct-message commands MUST preserve non-empty message content, support multiple lines and non-ASCII text, divide oversized content without corrupting characters, and avoid overwhelming the transport.
- **FR-011**: Direct messaging MUST reject channel targets and self-targets, establish a private conversation when needed, and report recipient absence instead of claiming a dropped message was sent.
- **FR-012**: Reading commands MUST collect every active joined channel and private conversation, label the source of each displayed line, suppress the session's own echoed messages, and maintain independent unread positions per invoking agent session; a replay option MUST return the available history from session start.
- **FR-013**: The streaming command MUST emit only newly arriving incoming lines, discover newly opened private conversations, re-check and recover the client during long runs, and end when its owning agent session ends.
- **FR-014**: The participant-list command MUST identify the caller among the members and report a clear failure when the server cannot provide a membership response.
- **FR-015**: Stopping a session MUST leave the chat service cleanly, stop the associated client process group, verify that no owned client remains, and preserve safety against stopping another agent's active session.
- **FR-016**: The client-inspection command MUST report all discovered local client homes and their owner, supervisor, and client health. Cleanup MUST support a no-change preview, stop only sessions whose owner has demonstrably exited, reap orphaned clients, and optionally remove confirmed-dead client directories.
- **FR-017**: The project MUST ship a local-server configuration template and usage guidance for a trusted local/private development bus, including its lack of built-in access control and the limits of using it on a LAN; it MUST not present the service as suitable for public or multi-tenant deployment.
- **FR-018**: The project MUST ship a `chat` SKILL that covers prerequisites, server setup/reuse, configuration, CLI command usage, identity and channel conventions, tagged-message and reply policy, live-watch versus polling behavior, safety boundaries, and clean shutdown. It MUST include the reference guidance needed to explain its relationship to A2A and to troubleshoot the underlying client behavior.
- **FR-019**: The project MUST include automated smoke coverage that exercises the supported CLI lifecycle and core collaboration operations against an isolated local server, including interruption recovery and safety-sensitive failure cases.

### Key Entities *(include if feature involves data)*

- **Chat Session**: One agent-owned local collaboration presence, including its identity, role, owner binding, local client location, and lifecycle state.
- **Channel Membership**: A named shared space that a session has joined and should restore following a safe reconnection.
- **Conversation**: Either a shared channel or a private exchange; it supplies the source label and independent unread position for received messages.
- **Message Cursor**: The per-agent-session record of how much of each conversation has been read.
- **Client Health Record**: The inspectable lifecycle and connection status for a local chat client, used by health, inspection, and cleanup operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a ready local environment, an agent can start a session, join a work channel, and receive a successful health result in under 15 seconds.
- **SC-002**: Two active agent sessions can exchange a channel message and a direct message, with each recipient displaying the message in the correct source context within 10 seconds.
- **SC-003**: In automated verification, 100% of the documented CLI command groups complete their normal path and their documented safety/error paths with an actionable result.
- **SC-004**: After a recoverable client interruption, the next collaboration command restores all remembered memberships and either delivers the requested action or explains the specific unrecoverable link within 15 seconds.
- **SC-005**: In repeated automated lifecycle tests, no client owned by an ended session remains connected after its owner is detected as ended, and no live owner is stopped by cleanup.
- **SC-006**: A new coding-agent session can follow the delivered SKILL to complete the primary collaboration flow without referring to files in the former standalone `skills/self-build/chat/` directory.

## Assumptions

- This feature targets trusted local development and user-controlled private LANs; it is not a public, authenticated, or multi-tenant messaging service.
- The host has the supported local chat-server and client prerequisites available; the SKILL will guide prerequisite installation but the CLI will not install system dependencies itself.
- Existing `chatta` configuration conventions remain the source of precedence for command flags, environment variables, and user configuration files.
- The first delivery scope absorbs the standalone `chat` skill's code, configuration template, operational documentation, and smoke-test behavior into this project. Separate server-administration and manual-refresh companion skills are outside this feature unless their behavior is required to make the delivered `chat` SKILL complete.
- Agent users follow the documented identity, tagging, and reply conventions; the local server provides message transport rather than enforcing collaboration etiquette.
