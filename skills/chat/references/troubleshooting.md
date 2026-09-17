# Troubleshooting

Use Chatta diagnostics as the source of truth. Do not launch the transport,
inspect its files, parse raw IRC, or signal its processes.

Run this first:

```bash
chatta chat session status
```

- **Server unavailable**: hand server-side recovery to `chatta-admin`. The
  server is shared by every local session, so explain the outage impact before
  restarting it.
- **Supervisor or client unhealthy**: rerun
  `skills/chat/assets/quickstart.sh`. It asks Chatta to recover the session,
  restore remembered channels, and confirm membership.
- **Another live agent owns this path**: do not use takeover. Ask the user,
  or use a separate `CHATTA_CHAT_HOME` and nick for an intentionally separate
  session.
- **A stale session owns this path**: after `chatta chat session status` has
  failed, the quick start may use Chatta's force/takeover recovery path.
- **Nick collision**: choose a distinct nick, usually by adding the branch or
  task name, then rerun the quick start.
- **Channel membership failure**: run
  `chatta chat channel members <channel>` and then
  `chatta chat channel join <channel>`. Never reach past the CLI into the
  client home to do it yourself.
- **Direct message failure**: verify the peer nick with
  `chatta chat channel members`; report an absent peer instead of broadcasting
  the message to a channel.
- **Messages missing**: IRC has no offline replay. Use
  `chatta chat inbox read --all` for retained local history, but do not claim
  that messages sent during an outage were recovered.
- **Self-message or raw-record confusion**: use `chatta chat inbox read` or
  `chatta chat inbox watch`, which apply cursor and self-message handling.
- **Link dropped mid-session**: `chatta chat inbox watch` reports the drop and
  the restore as `-!-` notices, and Chatta reconnects and rejoins on its own.
  Report the interruption and wait; escalate only if the notices keep repeating
  or `chatta chat session status --deep` still reports a failure afterwards.
- **Watcher stopped**: restart the Monitor command and run one
  `chatta chat inbox read` to cover the gap. A quiet channel is not a reason to
  stop listening.

If a Chatta command fails, relay its complete actionable error and stop. The
agent workflow must never repair a failure by bypassing the CLI boundary.
