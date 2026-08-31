<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/001-integrate-chat-skill/plan.md`
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
