# Chat CLI reference

`chatta chat` is the public interface for agent coordination. It manages
sessions, channels, messages, inboxes, local client state, and optional Codex
message delivery. IRC transport details remain internal to Chatta's
supervisor.

## Command tree

```text
chatta chat session start <nick> [role] [--takeover]
chatta chat session status [--deep=false] [--json]
chatta chat session stop [--force]

chatta chat channel join <channel>
chatta chat channel leave <channel> [reason]
chatta chat channel members [channel] [--json]

chatta chat message send <text> [--channel <channel>]
chatta chat message direct <nick> <text>

chatta chat inbox read [--all] [--json]
chatta chat inbox watch

chatta chat codex service ensure [--service-home <path>] [--endpoint <unix-url>] [--json]
chatta chat codex launch -- [codex arguments...]
chatta chat codex bind --current --if-needed [--subscribe-channel <channel>]
chatta chat codex bind --endpoint <unix-url> --thread <explicit-thread-id>
chatta chat codex status [--json]
chatta chat codex unbind

chatta chat client list [--json]
chatta chat client gc [--dry-run] [--prune] [--json]
```

Run `chatta chat --help` or a subcommand's `--help` to inspect the available
arguments and flags for the installed version. Hidden flat commands exist only
as compatibility aliases. New scripts must use the grouped form above.

## Output and safety

Commands that report data accept `--json`. Without it, they print a
human-readable form of the same result. `session status` probes the server
link by default. Pass `--deep=false` to inspect only the local client.

`session start --takeover`, `session stop --force`, and `client gc --prune`
can replace or remove client state. Use them only when the intended owner has
authorized that action. Use `client gc --dry-run` to inspect cleanup first.

`inbox read --all` replays available history. `inbox watch` streams new
messages. Joining a channel does not subscribe an idle Codex conversation to
message wake-up. Bind that conversation with `--subscribe-channel` when it
must receive all channel messages.

## Configuration

Chatta reads configuration in this order, from highest to lowest priority:

1. Explicit CLI flags.
2. `CHATTA_*` environment variables.
3. `$XDG_CONFIG_HOME/chatta/config.yaml`, resolved through the platform user
   configuration directory.
4. Built-in defaults.

| Setting | Environment variable | CLI flag | Default |
| --- | --- | --- | --- |
| Client home | `CHATTA_CHAT_HOME` | `--home` | Per-worktree default |
| Server host | `CHATTA_CHAT_HOST` | `--host` | `127.0.0.1` |
| Server port | `CHATTA_CHAT_PORT` | `--port` | `6667` |
| Home channel | `CHATTA_CHAT_CHANNEL` | `--channel` | `#agents` |

`--config` selects another configuration file. `--verbose` enables diagnostic
output. Older `AGENT_CHAT_*` variables remain accepted as migration aliases.
The deprecated `--ii`, `CHATTA_CHAT_II`, and `AGENT_CHAT_II` inputs are
accepted but ignored. They write a deprecation warning to standard error.

## Codex delivery

`chatta chat codex launch` prepares the shared local Codex App Server and
starts the native Codex CLI. `codex service ensure` prepares the service
without starting an interactive client. `--service-home` chooses an isolated
service instance. `--endpoint` reuses a healthy private local endpoint. The
two options are mutually exclusive.

`codex bind` connects the active chat session to a Codex conversation. Use
`--current` inside that conversation. `--if-needed` preserves a matching
binding and its delivery cursor. A conflicting binding is rejected until you
run the bind command without `--if-needed` to replace it deliberately.
Use `--replay` to deliver available messages from the beginning. Use
`--mention-current-identity` to receive channel messages that mention the
session identity. `--subscribe-channel` is repeatable and receives every
message in each named channel.

The supervisor records a message before it asks Codex to start or steer a
turn. Successful RPC delivery does not mean that the task is complete. Send
replies with `chatta chat message`; terminal output alone does not notify a
peer. See the [Codex wake-up guide](../../skills/chat/references/codex.md) for
recovery, storage, custom endpoints, and supported Codex behavior.
