<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->

## Development guides

These are binding, not advisory. Read the relevant guide before writing code
in that layer, and follow it strictly.

- **CLI** — `docs/go-cli-guides.md`. Applies to `cmd/` and every Cobra
  command, flag, and terminal output path.
- **Backend services** — `docs/go-server-guides.md`. Applies to the service
  layers under `pkg/` (`config`, `services`, `managers`, `dal`, `common`).

If a guide conflicts with existing code, follow the guide and say so rather
than matching the older style silently.

## Agent chat skill (`skills/chat/`)

- **English only.** The documentation, the example messages, and the eval
  files are written in English; `evals/check_skill.py` errors on any CJK text.
- **Grouped commands only.** `chatta chat <group> <sub>` (`message send`,
  `inbox read`, `channel members`, `session status`, …) is the public
  interface. The flat forms (`send`, `poll`, `who`, `health`, `gc`) are hidden
  compatibility aliases — never write them in docs, examples, or scripts.
- **No machine-specific paths in scripts.** Nothing under `skills/` may
  hardcode a uid-derived temp directory, an absolute home path, or another
  repository's location; derive them instead (`${TMPDIR:-/tmp}` with its
  trailing slash stripped, `$HOME`, the script's own directory).
- **Getting on the bus is `skills/chat/assets/quickstart.sh`** — one command,
  no questions asked, safe to rerun; it starts `ngircd` if nothing answers on
  the port. The manual `session start` sequence is the fallback for when it
  fails.
- **The skill runs from a deployed copy.** `~/.claude/skills/chat` usually
  points outside this repository, so editing `skills/chat/` changes nothing
  until that copy is synced; `skills/chat/evals/run.sh` refuses to run while
  the two differ.
- **After editing the skill**, run `python3 evals/check_skill.py` (static,
  seconds) and `evals/check_quickstart.sh` (eight scenarios on an isolated
  bus, about a minute) from `skills/chat/`.

## Known traps

- **`go test ./...` fails inside an agent session.** `pkg/dal`'s
  `TestInvocationSessionIDPrefersCallingRuntime` reads the real Claude/Codex
  session id and fails; it passes outside one. Don't read it as a regression,
  and don't use the suite as a green baseline from within a session.
- **`session stop --force` cannot disown a client.** It clears the supervisor
  fields but keeps `state.json`'s owner, so when that owner is another live
  session the next `session start` still refuses; pass `--takeover` too.
  Clearing the owner instead is not a fix: `dal.ValidateSession` requires
  `Owner.PID > 0`, so a disowned state cannot be written back at all. Giving
  the schema a way to express "no owner" is the change this needs.
- **A nick is not released the moment its client dies.** Back-to-back
  `session stop --force; session start` fails with "nick already in use" two
  or three times in five: the server holds the nick until the old connection
  closes. Retrying a few seconds later is what works (the quick start does),
  and waiting inside `session start` instead only converts a fast failure into
  a 20-second timeout — measured, not guessed.
