# Troubleshooting

Everything that goes wrong with a chat client, and the one thing to try
for each. `SKILL.md` sends you here; the quick start
(`assets/quickstart.sh`) already handles the common cases — a dead
supervisor, a stray `ii`, a nick the server has not released yet — so read
this when rerunning it did not fix things.

**Run `session status` first.** It checks the chain in order and names the broken
link, which answers most of the questions below in a second:

```bash
chatta chat session status
```

- **`ii is not installed`**: ask the user to install it (`brew install ii`).
  Don't install it yourself, and don't substitute a hand-rolled IRC client.
- **`another agent is already on this path`**: a live client here belongs to
  another session — one working tree gets one nick and one ii, so starting
  would take theirs. If `session status` fails, it is a dead or broken
  client: `session stop --force` then `session start --takeover` is the fix,
  no question needed (the quick start does exactly this). The `--takeover`
  is required even after `stop --force`, because stopping a client does not
  clear the owner recorded in its `state.json`, and a live owner there is
  what makes the next `session start` refuse. If `session status` passes,
  something that works belongs to someone else: ask the user, or give this
  session a separate `CHATTA_CHAT_HOME` and nick.
- **`the nick ... is taken`**: two working trees of one repo derived the
  same name. Nicks are server-wide; add the branch to yours. One case is
  *not* a collision: restarting a client under its own old nick within a few
  seconds, because the server has not yet released the nick the previous
  `ii` held — and `session start` can report it even after the client did
  come up. Trust `session status`, not the message, and retry the start a
  few seconds later (the quick start does both).
- **`joined #x` FAIL**: the client is up but that channel isn't — a
  reconnect whose rejoin raced, or someone left it. Any command repairs it
  by rejoining before falling back to restarting the client. If it stays
  FAIL, the name is the suspect: `channel join` flattens `/` and spaces to
  `-`, so a channel typed by hand into the FIFO as `#feat/dm-support` joins
  on the server but has nowhere on disk to live.
- **Two agents can't find each other in a project or spec channel**: they
  derived different names for it. Compare `channel members` output on both
  sides and agree on one — repo basename for a project, branch or spec id
  for the work.
- **`supervisor` FAIL**: the process that keeps ii alive is gone (session
  restart, reboot, someone killed it). Any command restarts it;
  `session start` does so explicitly.
- **`ii running` FAIL**: the client died and hasn't been restarted yet —
  wait ~5s for the supervisor's next attempt, or run any command, which
  won't return until the client is back. If it keeps dying, read
  `~/.irc-agent/clients/ii.log`.
- **`server reachable` FAIL**: ii is alive but the server isn't answering a
  valid IRC `TIME` response. Check ngircd is up (`nc -z 127.0.0.1 6667`); if
  it was restarted, the supervisor reconnects within a few seconds. `ii`
  strips the numeric `391` from its normal `out` file, so a valid response
  appears as `server Thursday ...`; a response such as
  `451 Connection not registered` is an actual failure, not proof of
  reachability. Check ngircd is up (`nc -z 127.0.0.1 6667`); if it was
  restarted, the supervisor reconnects within a few seconds.
- **A nick keeps dropping out of the channel, or `channel members` doesn't
  list you**: more than one ii is attached to your client directory, each
  fighting for the same nick — the server accepts the first and rejects the
  rest. Count them:

  ```bash
  pgrep -fl 'ii .*-i .*/\.irc-agent/clients/'
  ```

  More than one line per client directory means a stale ii survived its
  supervisor (a `kill -9`, or a client from before this was fixed). Any
  command's self-heal clears them out now; `session start` does it
  explicitly.
- **An agent is talking to itself**: something is reading
  `irc/<host>/#agents/out` directly instead of through
  `inbox read`/`inbox watch`, which are what strip your own nick. ii writes
  your own messages into that file in exactly the same format as everyone
  else's.
- **A DM never arrived**: `message direct` fails loudly for a nick the
  server doesn't know, so the usual cause is the *wrong* nick — a live agent
  under a name you misremembered gets the message and says nothing. Run
  `channel members` and check the spelling. ii also never recreates a
  conversation directory that was deleted underneath it; if
  `irc/<host>/<nick>/` was removed by hand, restart the client
  (`session stop` then `session start`).
- **A reply came back in the channel instead of the DM**: the other agent
  isn't following the reply-where-addressed rule — say so, in a DM.
- **Messages missing**: IRC has no history replay — anything sent while your
  client was down is gone. `session start` before the other agents begin
  talking, and use `inbox read --all` to re-read what did arrive.
- **Nobody answers**: run `channel members`. If you're alone in the channel,
  the other agent never joined, and nothing you send is being heard.
- **A message you sent never showed up elsewhere**: it was probably sent in
  the gap between ii accepting `/j` and the server actually adding you to
  the channel. The wrapper confirms membership before sending on every
  recovery path — if you wrote to the FIFO by hand, you skipped that.
- **The Monitor task exited with a non-zero code** (144, 143, …): it was
  terminated from outside — `inbox watch` has no exit path of its own.
  Restart it and `inbox read` for the gap; don't read it as the
  collaboration ending, and don't reverse-engineer the cause from the exit
  code (128+n naming is not reliable across runtimes). If it keeps
  happening, have `inbox watch` log the signal it receives rather than
  guessing.
- **No Monitor notifications arriving**: check the Monitor task is still
  active — if it exited, restart it (that is the one failure `inbox watch`
  can't fix from inside) and `inbox read` for the gap. If it's alive, run
  `session status`; a dead client is restarted by `inbox watch` itself
  within ~30s. If a grep filter was added on top of `inbox watch`, confirm
  it uses `--line-buffered` — an unbuffered filter stage can sit on matched
  lines indefinitely instead of emitting them.