# Contributing to fya

## Development Setup

1. Clone the repository.
2. Install Go 1.26+.
3. Run `make test` to verify setup.

## Code Style

- Follow standard Go conventions.
- Keep changes focused.
- Add tests for behavior changes.
- Keep stdout machine-readable during normal execution. Use stderr/logging for diagnostics.
- Update README.md or ARCHITECTURE.md when behavior, flags, or output contracts change.

## Pull Requests

1. Create a feature branch from master.
2. Make your changes with tests.
3. Run `make test lint` before submitting.
4. Submit a PR with a clear description.

## Reporting Issues

Please include:

- Go version
- OS and architecture
- Claude Code version
- command you ran
- expected vs actual behavior
- relevant stderr output with secrets removed
