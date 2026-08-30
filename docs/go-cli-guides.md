# Go CLI Development Guide

This project follows the shared Go CLI conventions in `../guides/go-cli-guides.md`.

The adopted baseline is Go 1.22+, Cobra for commands, Viper for configuration,
`log/slog` for structured logging, and the layered `pkg/` layout:
`config`, `services`, `managers`, `dal`, and `common`.
