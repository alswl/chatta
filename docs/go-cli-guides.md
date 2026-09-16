# Go CLI Development Guide

This project follows the shared Go CLI conventions in `../guides/go-cli-guides.md`.

The adopted baseline is Go 1.22+, Cobra for commands, Viper for configuration,
`log/slog` for structured logging, stdlib `testing` with testify assertions,
and the layered `pkg/` layout: `config`, `services`, `dal`, and `common`.

## Deviations from the shared guide

Two departures, both deliberate and both load-bearing for this codebase.

**No `managers` layer.** The shared guide specifies
`cmd → services → managers → dal`. Chatta collapsed that to
`cmd → services → dal` when the transport moved in-process
(`specs/003-native-irc-client`, Phase 1). Every manager here was a single
entity's logic with exactly one caller, so the layer added a hop without
adding a seam. `services` now holds what they held.

**An extra `pkg/daemon` package.** The resident supervisor process is neither
a use case nor data access: it owns a control socket and a connection that
outlive any single command. It sits beside the layers rather than inside one.

Everything else follows the shared guide, including one file per command and
subcommand under `cmd/chatta/`, each self-registering to its parent in
`init()`.

## Machine-readable output

Commands that report data accept `--json`: `session status`, `channel
members`, `inbox read`, `client list`, and `client gc`. The shared guide pairs
`--json` with `--plain`; the default human form here is already plain and
greppable, so `--plain` would be a synonym for it and is not offered.

Rendering stays in the command layer. Where a service method returned
pre-rendered strings, it now has a structured sibling that is the source of
truth — `Poll`/`PollMessages`, `Who`/`Members`, `GC`/`GCReport` — so the two
output forms cannot drift.
