# IRC agent chat vs. A2A (Agent2Agent Protocol)

A2A is Google's open protocol (donated to the Linux Foundation) for
letting independent agents — often from different vendors, running as
separate services — discover each other's capabilities and exchange tasks
over HTTP, using JSON-RPC 2.0 and Server-Sent Events for streaming. This
skill's IRC approach solves a much narrower problem: giving a handful of
*trusted* agent sessions on the *same machine or LAN* a shared channel to
talk in, with almost no setup. They rhyme (both are "agents send each
other structured-ish messages"), but they are not interchangeable. Use
this table to explain the tradeoffs accurately rather than overselling IRC
as "A2A but simpler."

| Concern | A2A | IRC (this skill) |
|---|---|---|
| Transport | HTTP + JSON-RPC 2.0, SSE for streaming updates, optional gRPC | Plain-text IRC protocol over TCP (RFC 1459/2812-ish, as implemented by ngircd) |
| Message shape | Structured JSON `Message`/`Task`/`Artifact` objects with defined fields | Freeform text lines; this skill layers a `[TAG] sender: text` convention on top, but nothing enforces it |
| Discovery | Agents publish an **AgentCard** (JSON: name, description, skills, auth requirements) at a well-known URL; clients fetch it to learn what an agent can do | No equivalent. The closest approximation this skill offers is the **channel topic** as a pinned, shared string (`echo '/t role=...; caps=...' > <ircdir>/#agents/in`) — but it's just text a human/agent has to read and parse, not a machine-checkable schema |
| Task lifecycle | Explicit states: `submitted → working → input-required → completed / failed / canceled`, tracked server-side per task ID | None. A message is the only primitive. "Task state" is whatever the agents agree to say in `[TASK]`/`[STATUS]`/`[DONE]` messages — entirely convention, not enforced or queryable |
| Multi-turn / long-running work | First-class: a task can go `input-required` and resume later; clients can poll or subscribe via SSE | Approximated by keeping the `inbox watch` process running and having agents `inbox read` their log — works, but there's no concept of "this task is still open" independent of the messages themselves |
| Auth / security | Defined per AgentCard (API keys, OAuth2, etc.); enterprise-grade by design, meant for cross-organization use | **None.** ngircd here is configured with no password and no TLS, for localhost/LAN trusted use only. Anyone who can reach the port can join, read, and post. Never expose this setup to an untrusted network — it is not a substitute for A2A's auth model |
| Group communication | Fundamentally point-to-point (client task → remote agent); broadcasting to many agents means the client fans out multiple calls itself | **A real strength of IRC here.** A channel is native group broadcast — one `PRIVMSG #agents` reaches every listening agent, no fan-out logic needed. If the actual need is "N agents coordinating live," IRC's model fits more naturally than A2A's request/response shape |
| Setup cost | Needs an HTTP server implementing the spec (or a framework like `a2a-sdk`), plus AgentCard hosting | One `ngircd` binary and one `ii` binary (both already packaged for brew/apt/etc.), one config file, and a chatta wrapper. Minutes, and nothing hand-rolled — ii is the IRC client |
| Interop | Designed for agents from different vendors/frameworks to talk without prior coordination, as long as both speak A2A | Only works if every participant follows this skill's ad hoc conventions (nick scheme, tags, channel names) — there's no protocol-level guarantee two arbitrary IRC clients "mean" the same thing by a message |

## When each one actually fits

- **Use A2A** (or point the user to it) when agents are separate services
  that need to discover each other's capabilities without prior
  coordination, potentially across organizations or vendors, and need
  auth, structured task tracking, or long-lived task handoff.
- **Use this IRC skill** when the situation is "I have a couple of agent
  CLI sessions I'm running myself, on hardware I control, and I just want
  them to tell each other what they're doing" — the setup cost of A2A
  isn't worth it, and IRC's group-channel model is actually a better fit
  for that kind of live, informal coordination than A2A's point-to-point
  task calls.

## Honest gaps if someone wants to push this further

If a user wants to close the gap toward A2A-like behavior, the natural
next steps (not implemented by this skill, but worth naming so the
limitation is explicit rather than silent) are:

- A small bot/service that owns capability info per nick and answers a
  `!whois <nick>` style query — an AgentCard-lite, machine-parseable
  instead of a free-text topic.
- A shared task-state file (or a dedicated `#tasks` channel with a strict
  `[TASK:<id>:<state>]` grammar) that something can index, instead of
  relying on agents to remember conversational context.
- TLS + a shared password (`PAM`/`MyPassword` in ngircd) once this leaves
  a single trusted machine — still nowhere near A2A's per-agent auth
  model, but better than nothing on a shared LAN.

None of these are hard, but they're deliberately left out here to keep the
skill's core loop (`message send` / `inbox watch` / `inbox read`) small and
dependency-free.