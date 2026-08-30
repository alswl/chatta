#!/usr/bin/env bash
# Post-actions hook for bump.sh. Sync version to external config files.
# github.com/alswl/makefile-go
#
# Author: alswl
# Version: 0.1.0

# cd root of the repo
pushd "$(dirname "$0")/.." > /dev/null

set -e
cat package.json | jq -M --raw-output ".version=\"$(cat VERSION | head -n 1 | sed 's/^v//g')\"" | sponge package.json
popd > /dev/null
