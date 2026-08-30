#!/usr/bin/env bash
# Install development tools required by the Makefile.
# github.com/alswl/makefile-go
#
# Author: alswl
# Version: 0.1.0

go install golang.org/x/tools/cmd/goimports@latest

# install golangci-lint
if ! command -v golangci-lint &> /dev/null; then
  echo "Installing golangci-lint"
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0
fi
