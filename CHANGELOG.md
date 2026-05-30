# Changelog

## v0.2.1 - 2026-05-30

### Bug Fixes

- Fix fya one-shot completion edge cases: prompt source selection no longer blocks on open stdin, tool-use turns wait for the post-tool `end_turn`, and text output ends with one newline.

## v0.2.0 - 2026-05-29

### New Features

- `--max-wpm-size` flag: paste prompts longer than N words (default 100) in a single write instead of typing them rune-by-rune, removing the multi-minute typing latency on large prompts. `--max-wpm-size=0` keeps rune-by-rune typing.

### Improvements

- Normalize internal CRLF and lone CR to LF when resolving the prompt, so a bare carriage return cannot submit a multiline prompt early.
- Internal cleanups to helper ownership and wrapper plumbing.

## v0.1.1 - 2026-05-24

### Bug Fixes

- Switch Homebrew installation from cask to formula to avoid macOS Gatekeeper quarantine prompts.

## v0.1.0 - 2026-05-24

Initial public release.

### New Features

- PTY-backed `claude --print` compatibility wrapper.
- `text`, `json`, and `stream-json` output modes.
- Claude Code transcript discovery and tailing.
- Ralphex-compatible streamed text deltas and final result events.
- Prompt typing controls for WPM, jitter, readiness timeout, turn timeout, and idle timeout.
- Child environment filtering for fya-private variables.
- Release pipeline for GitHub archives, deb/rpm packages, and Homebrew formula installation.
