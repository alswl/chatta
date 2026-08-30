# Data Model: Integrated Agent Chat

## ChatSession

The durable state for one agent's local chat client. It is stored in the client home as versioned JSON and is protected by an exclusive supervisor lock.

| Field | Type | Validation / behavior |
|---|---|---|
| SchemaVersion | integer | Required; supports safe future migrations. |
| Nick | string | Required, non-empty IRC nickname; must be unique on the connected server. |
| Role | string | Non-empty human-readable role; defaults to `agent`. |
| Host | string | Required local/private server host. |
| Port | integer | Required valid TCP port. |
| HomeChannel | ChannelMembership | Required; normalized; cannot be left with `part`. |
| Channels | ordered set of ChannelMembership | Contains HomeChannel first; no duplicates; restored after recovery. |
| Owner | OwnerBinding | Required for a live client; rejected when it cannot be verified. |
| SessionID | string | Optional stable runtime identifier used to distinguish live owners and read cursors. |
| SupervisorPID | integer | Transient process reference; must be validated by executable identity before a signal is sent. |
| StartedAt | timestamp | Informational creation time. |

### State transitions

```text
Absent -> Starting -> Active <-> Recovering
                   -> Stopping -> Stopped
Active/Recovering -> Orphaned -> Cleaned
```

- `Starting` becomes `Active` only after `ii` is running and the server confirms home-channel membership.
- `Recovering` can repair missing memberships or replace a dead client only while `Owner` is live.
- A missing/expired owner transitions the session to `Orphaned`; its supervisor must stop `ii` rather than revive it.
- `Cleaned` is available only to conservative cleanup after the owner is demonstrably absent. A live owner is never cleaned.

## OwnerBinding

| Field | Type | Validation / behavior |
|---|---|---|
| PID | integer | Positive process identifier found from a recognized agent runtime. |
| StartFingerprint | string | Required process-start marker paired with PID to resist PID reuse. |
| Runtime | string | Informational owner runtime classification. |

The binding is live only when both PID existence and start fingerprint match. An unowned client is invalid. A second live session at the same client home is a conflict unless the caller performs an explicit takeover.

## ChannelMembership

| Field | Type | Validation / behavior |
|---|---|---|
| Name | string | One leading `#`; lowercased; whitespace, commas, colons, and slashes flatten to `-`; invalid characters flatten to `-`; maximum 48 characters after normalization. |
| Kind | enum | `lobby`, `project`, `work`, or `custom`; informational only. |
| JoinedAt | timestamp | Informational; membership is authoritative only after server confirmation. |

Channels are an ordered set. Send operations are allowed only to an existing confirmed membership. Leaving the home channel is rejected; stopping leaves the session cleanly.

## Conversation and MessageCursor

| Entity | Fields | Rules |
|---|---|---|
| Conversation | Name, SourceKind (`channel` or `direct`), transcript path | Discovered from all active `ii` conversation directories, so new direct-message peers are included automatically. |
| MessageCursor | InvokerKey, offsets by Conversation Name | Stored separately for each agent session; cursor resets to zero only when the observed transcript has been replaced or shortened. |

Incoming output excludes messages whose sender nick equals the current session nick. Rendered lines include local time and source (`#channel` or `DM`). Replay intentionally ignores saved cursors.

## HealthReport

An ephemeral result rendered by `health` and reused by recovery.

| Check | Healthy condition |
|---|---|
| Owner heartbeat | OwnerBinding is live. |
| Supervisor | Recorded process exists and is the expected hidden supervisor. |
| Joined channels | Each remembered channel exposes an `ii` input endpoint. |
| Client reader | Nonblocking FIFO probe finds a reader. |
| Server link | A server time response arrives within the health deadline. |
| Membership | Server names response includes the current nick when an operation requires it. |

Any failed required check produces a non-success command result with the named failed link.

## ClientSurvey

An ephemeral fleet record used by `clients` and `gc`.

| Field | Meaning |
|---|---|
| ClientHome | Discovered state directory under the local agent-chat root. |
| SessionSummary | Nick, owner state, and supervisor state. |
| ClientProcessState | Healthy, stopped, or stray client process identifiers. |
| CleanupEligibility | Live owner, ended owner, unbound, orphaned client, or dead directory. |

Garbage collection may stop ended/unbound owners and reap orphaned client processes. `--dry-run` reports all decisions. `--prune` removes only confirmed-dead state directories.
