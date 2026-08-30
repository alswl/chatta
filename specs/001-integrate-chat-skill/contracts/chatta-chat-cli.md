# CLI Contract: `chatta chat`

## Scope and conventions

`chatta chat` is the public command group for trusted local/private-LAN agent coordination. Human-readable results go to standard output; actionable failures go to standard error and return a non-zero exit code. Existing root `--config`, `--verbose`, and `version` behavior remains unchanged.

Chat flags take precedence over `CHATTA_CHAT_*` values, then config-file values, then documented `AGENT_CHAT_*` migration aliases, then defaults. The default server is `127.0.0.1:6667`, the default home channel is `#agents`, and the default client home is deterministic per working tree under `~/.irc-agent/clients/`.

Common chat flags:

```text
--home PATH       Override the per-worktree client home.
--host HOST       Override the local/private IRC server host.
--port PORT       Override the IRC server port.
--channel NAME    Override the home channel.
--ii PATH         Override the ii executable path.
```

## Commands

```text
chatta chat start <nick> [role] [--takeover]
```

Creates a chat session bound to the invoking Codex/Claude runtime, starts the supervisor/client, and confirms home-channel membership. It refuses an unrecognized owner, a conflicting live owner (unless `--takeover` is explicit), or a server-wide nick collision.

```text
chatta chat health
```

Reports owner heartbeat, supervisor, every joined channel, client reader, and server reachability. A failed check is named and returns non-zero.

```text
chatta chat join <channel>
chatta chat part <channel> [reason]
```

Joins a normalized channel and remembers it for recovery; `part` removes a non-home membership and sends the optional reason. The home channel can only be left by `stop`.

```text
chatta chat send <text> [-c|--channel <channel>]
chatta chat dm <nick> <text>
```

Sends non-empty text to the default/selected joined channel or to one peer. Multi-line and UTF-8 content are preserved across transport-sized chunks. Channel sends reject unjoined targets. Direct messages reject a channel or self target and return non-zero when the recipient is absent.

```text
chatta chat poll [--all]
chatta chat watch
```

`poll` renders unread incoming traffic from every channel and direct conversation; `--all` replays available history. `watch` emits only future incoming lines, discovers new direct conversations, and rechecks/recoveries the client while running. Both suppress self echoes and label their source.

```text
chatta chat who [channel]
```

Lists server-confirmed members of the home or requested channel and marks the caller.

```text
chatta chat stop [--force]
```

Sends a clean leave, stops the verified supervisor/client group, and confirms no owned client remains. It refuses to stop a live foreign owner unless `--force` is explicitly supplied after user confirmation.

```text
chatta chat clients
chatta chat gc [--dry-run] [--prune]
```

`clients` surveys every discoverable local chat client and reports owner, supervisor, and client state. `gc` previews or performs conservative cleanup: it may stop confirmed-ended/unbound owners and reap stray clients; `--prune` additionally removes only confirmed-dead state directories. Live owners are always left alone.

## Hidden supervisor command

The implementation reserves `chatta chat _supervise` for the detached lifecycle process. It is not a supported user interface and must not appear in ordinary help output.

## Output stability

Automated callers may rely on command success/failure, the named health link, source labels (`DM` or channel), and self markers. Decorative prose and process identifiers are diagnostic details and are not a machine-readable API.
