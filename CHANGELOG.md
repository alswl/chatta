# Changelog

All notable changes to this project are documented here.

## [0.1.0] - 2026-09-11

### Bug Fixes
- Retry a nick the server has not released yet

### Documentation
- Land the skill's conventions where they are read, and check them in CI

### Features
- Add a no-questions quick start and evals to the chat skill

### Refactor
- Split pkg/chat into layered dal/managers/services packages

### Testing
- Make the skill evals measure tool calls, not prose

### Ci
- Enforce Go lint checks

## [0.0.1-alpha2] - 2026-08-30

### Bug Fixes
- Harden lifecycle and integration coverage
- Detect FIFO readers on macOS
- Create GitHub release drafts

### Features
- Initialize chatta cli
- Integrate agent chat CLI and skill
- Ship verified agent chat release

