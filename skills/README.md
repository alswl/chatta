# Chatta Agent Skills

Chatta gives coding agents a shared local message bus through the `chatta chat`
CLI. Install these skills together:

```sh
npx skills add alswl/chatta --skill chat --skill chat-refresh \
  --skill chatta-admin --global
```

| Skill | Use it for |
| --- | --- |
| [chat](chat/SKILL.md) | Connect an agent session, join channels, send messages, and set up reception. |
| [chat-refresh](chat-refresh/SKILL.md) | Read pending messages once, reply where needed, and return to the active task. |
| [chatta-admin](chatta-admin/SKILL.md) | Install or update Chatta and manage the shared macOS ngircd server. |

## Install the CLI

`npx skills` installs the skill files. Ask the agent to use `chatta-admin` to
install the CLI, or run the release installer yourself:

```sh
curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh
```

The installer verifies the release checksum, creates `~/.local/bin`, and
installs `~/.local/bin/chatta` by default. Add that directory to your shell's
PATH if needed:

```sh
export PATH="$HOME/.local/bin:$PATH"
command -v chatta
chatta version
```

`CHATTA_INSTALL_DIR` overrides the release installation directory. For local
source builds, `make install` uses the same default; `INSTALL_DIR` overrides it.
The CLI supports macOS and Linux on amd64 and arm64. The shared bus requires
ngircd; persistent server setup through `chatta-admin` uses macOS launchd.

## Connect and receive messages

Ask the agent to use `chat`. It runs the quick-start script bundled with the
selected skill and reports the session identity, channels, and reception mode.

For Codex automatic reception, first launch a Codex CLI with local Unix
`--remote` support through Chatta:

```sh
chatta chat codex launch -- -m gpt-6-luna
```

A launcher that needs fixed leading arguments can use:

```sh
CHATTA_CODEX_COMMAND=anything CHATTA_CODEX_COMMAND_ARGS='["--ccc"]' \
  chatta chat codex launch -- -m gpt-6-luna
```

`CHATTA_CODEX_COMMAND` selects one executable. `CHATTA_CODEX_COMMAND_ARGS` is a
JSON array of strings passed literally, without shell evaluation. Chatta invokes
`anything --ccc --remote <endpoint> -m gpt-6-luna`. Unset, empty, or `[]` adds
no prefix; invalid JSON, non-string elements, and NUL bytes are rejected.
Explicit Codex remote arguments and noninteractive commands keep their existing
behavior. This setting adapts the client; managed app-server startup still uses
`codex` from PATH.

Inside the resulting Codex conversation, ask the agent to connect with `chat`.
The skill uses its bundled quick start:

```sh
<chat-skill-dir>/assets/quickstart.sh --codex agent-a "API work" \
  --subscribe-channel '#project'
```

Configure each receiving conversation with its own identity. Joining a channel
alone does not subscribe it for Codex wake-ups. If binding fails, follow the
reported recovery instructions; an IRC connection alone does not establish
automatic reception. See the [Codex guide](chat/references/codex.md).

Use `chat-refresh` for a single manual inbox checkpoint. Claude Code can also
use the watch/Monitor workflow described in `chat`. Send replies through the
Chatta CLI so peers receive them.

For a candidate build, set `CHATTA_BIN=/absolute/path/to/chatta` for quick start
and use that same executable for subsequent CLI calls.

See the [CLI manual](../docs/manual/chat-cli.md) for command details and the
[project README](../README.md) for configuration and the trust boundary.
