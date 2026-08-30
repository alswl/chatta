# chatta

Chatta is a Go command-line system.

## Install

Tagged releases provide checksummed binaries for macOS and Linux on amd64 and
arm64. Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh
```

To install a particular release or choose the destination directory:

```sh
CHATTA_VERSION=v0.1.0 CHATTA_INSTALL_DIR="$HOME/.local/bin" \
  sh -c 'curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh'
```

The agent-chat feature also requires `ngircd` and `ii` on each participating
host (macOS: `brew install ngircd ii`). See [the chat operations guide](docs/chat.md).

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
