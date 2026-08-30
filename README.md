# chatta

Chatta is a Go command-line system.

## Development

```sh
make test
make build
./bin/chatta --help
./bin/chatta version
```

Configuration is read from `CHATTA_*` environment variables or
`$XDG_CONFIG_HOME/chatta/config.yaml` (via the platform's user config
directory). Explicit flags take precedence over both.

The CLI is organized as `cmd/chatta` plus the shared `pkg/config`,
`pkg/services`, `pkg/managers`, `pkg/dal`, and `pkg/common` layers.
