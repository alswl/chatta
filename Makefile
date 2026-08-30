# https://github.com/alswl/makefile-go
# Shared Go build, test, install, and release targets.

SHELL := /bin/bash

PROJECT := chatta
TARGETS := chatta
ROOT := github.com/alswl/chatta
CMD_DIR := ./cmd
OUTPUT_DIR := ./bin
BUILD_DIR := ./build
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

COMMIT := $(strip $(shell git rev-parse --short HEAD 2>/dev/null))
COMMIT := $(COMMIT)$(shell [[ -z $$(git status -s) ]] || echo '-dirty')
COMMIT := $(if $(COMMIT),$(COMMIT),unknown)
VERSION_IN_FILE := $(shell cat VERSION)
VERSION ?= $(VERSION_IN_FILE)-$(COMMIT)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOPATH ?= $(shell go env GOPATH)
UT_COVER_PACKAGES := $(shell go list ./pkg/... 2>/dev/null)
IT_COVER_PACKAGES :=
COVERAGE_PACKAGES := $(shell go list ./pkg/... 2>/dev/null | paste -sd, -)
COVERAGE_PROFILING_DIR := $(PROJECT_DIR)/.cover

.PHONY: all
all: fmt test build

include hack/makefile-go/_git.mk
include hack/makefile-go/build.mk
include hack/makefile-go/install.mk
include hack/makefile-go/test.mk
include hack/makefile-go/general.mk
include hack/makefile-go/version.mk
