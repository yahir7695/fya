# Claude PTY Print Wrapper

## Overview
- Build `fya` as a Go CLI that acts as a drop-in `claude -p` replacement for consumers such as Ralphex.
- The wrapper starts a fresh interactive Claude Code process inside a PTY, types the prompt like a human, reads Claude Code transcript JSONL, and emits Claude-compatible `stream-json` output.
- The design is strictly ephemeral: one invocation, one prompt, streamed result, then the Claude process exits or is killed.
- This solves the case where a caller expects `claude --print --output-format stream-json`, but we need to exercise Claude Code through its interactive terminal path.

## Context (from discovery)
- `fya` is an empty Git repo with remote `git@github.com:umputun/fya.git`; this plan includes initial project scaffolding.
- `melonamin/agentrun` provides useful patterns: Claude flag parsing, transcript discovery under `~/.claude/projects`, transcript idle completion, and stream-json compatibility checks.
- Ralphex invokes Claude-compatible providers as `<claude_command> <claude_args...> [--model <m>] [--effort <e>] --print`, passes the prompt on stdin, and reads JSONL from stdout.
- Ralphex only requires `content_block_delta` text events plus a final `result` event, though matching Claude result metadata improves compatibility.
- The core implementation areas are CLI parsing, PTY lifecycle, human typing simulation, transcript tailing, stream-json synthesis, and tests with fake Claude/transcript fixtures.

## Acceptance Criteria
- `fya` can be used as Ralphex's `claude_command` with default Claude args and a prompt passed on stdin.
- `fya --print --output-format stream-json` starts interactive `claude` inside a PTY, not `claude --print`.
- Prompt text is entered through the PTY at roughly 100 WPM by default, with configurable WPM and jitter.
- stdout is valid Claude-compatible JSONL and includes streamed text plus one final `result` event.
- Completion is detected from Claude Code transcript JSONL under `~/.claude/projects`.
- The Claude process tree is cleaned up after the final event or on timeout/cancellation.
- v1 supports Unix/macOS only; Windows PTY support is out of scope.

## Development Approach
- **testing approach**: regular iterative implementation with focused tests for behavior changes.
- Complete each task fully before moving to the next.
- Make small, focused changes.
- Add or update tests when code behavior changes; docs/build-only changes use build/lint verification.
- All tests must pass before starting the next implementation task.
- **CRITICAL: update this plan file when scope changes during implementation**.
- Run tests after each task.
- Maintain drop-in compatibility with Ralphex's Claude provider contract.

## Testing Strategy
- **unit tests**: required for every package.
- **fake command tests**: use a fake `claude` executable or test helper subprocess to validate stdin, args, PTY startup, and cleanup without spending Claude quota.
- **transcript fixture tests**: validate discovery, offset handling, assistant extraction, tool-use pending tracking, idle completion, and result completion.
- **Ralphex contract tests**: pipe wrapper output into event-shape assertions matching `pkg/executor/ClaudeExecutor` expectations.
- **optional live tests**: a manually invoked parity script comparing core fields against real `claude -p`; this must be opt-in because it uses Claude quota.

## Progress Tracking
- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with `+` prefix.
- Document issues/blockers with `WARNING` prefix.
- Update plan if implementation deviates from original scope.
- Keep plan in sync with actual work done.

## Solution Overview
- `app/` is the user-facing executable and composition root, matching the local `revdiff` single-binary structure.
- CLI parsing accepts common `claude -p` flags and ignores or forwards Claude launch flags appropriately.
- The prompt is read from stdin first, with positional prompt support retained for direct use.
- Implementation packages grow under `app/<concern>` until there is a real need for public packages.
- `app/ptyrun` starts `claude` in a PTY in the selected cwd, drains terminal output, waits for readiness, types prompt runes with jitter, sends Enter, and handles process-group cleanup.
- `app/transcript` locates and tails Claude Code JSONL for the current cwd.
- `app/stream` converts transcript events into Claude-compatible stdout JSONL.
- The wrapper exits after emitting the final result event and cleaning up the Claude process.

## Technical Details
- Prompt input:
  - Primary path: stdin, because Ralphex sends prompts via stdin to avoid command-line length limits.
  - Secondary path: positional arguments joined with spaces for direct CLI use.
- Accepted compatibility flags:
  - `-p`, `--print`
  - `--output-format text|json|stream-json`
  - `--input-format text|stream-json`
  - `--include-partial-messages`
  - `--include-hook-events`
  - `--replay-user-messages`
- Consumed flags are handled by `fya` and are never passed to interactive Claude:
  - `-p`, `--print`
  - `--output-format`, `--input-format`
  - `--include-partial-messages`, `--include-hook-events`, `--replay-user-messages`
  - `--idle-timeout`, `--turn-timeout`, `--cwd`, `--typing-wpm`, `--typing-jitter`, `--readiness-timeout`
- Forwarded Claude launch flags include at least `--model`, `--effort`, `--permission-mode`, `--allowed-tools`, `--allowedTools`, `--disallowed-tools`, `--add-dir`, `--mcp-config`, `--settings`, `--dangerously-skip-permissions`, `--verbose`, and other explicitly recognized interactive Claude flags.
- Unknown flags are rejected with a Claude-style error instead of blindly forwarding, so typos fail fast and print-only flags cannot leak into the interactive Claude process.
- PTY typing:
  - Default target rate: 100 words per minute.
  - Assume 5 characters per word, so average delay is about 120ms per rune.
  - Add configurable jitter around that average.
  - Internal prompt newlines are typed as line breaks without submitting early.
  - Final submit is sent as Enter after a short settle delay.
  - Estimate typing duration before starting; if estimated typing time exceeds `--turn-timeout`, fail before launching Claude.
  - If estimated typing time exceeds a warning threshold, print a warning to stderr without corrupting stdout JSONL.
- Transcript path:
  - Default root: `~/.claude`.
  - Override root: `FYA_CLAUDE_DIR`.
  - Current cwd path encoding follows Claude Code's project directory style: absolute path with non-alphanumeric characters replaced by `-`.
- Completion:
  - Prefer transcript `type == "result"` when present.
  - Otherwise complete when assistant text has appeared, there are no pending tool calls, and the transcript file has been idle for `--idle-timeout`.
- Output:
  - `stream-json`: emit `content_block_delta` events as assistant text appears, then a final `result`.
  - `text`: collect assistant text and print plain text at the end.
  - `json`: emit one final Claude-like result JSON object.
- Stream-json input:
  - `fya` accepts exactly one user message from `--input-format stream-json`.
  - If multiple user messages are provided, fya rejects the input with a clear error instead of trying to simulate multi-turn history in a single ephemeral interactive session.
  - `--replay-user-messages` echoes the accepted raw user event before assistant output.
- Environment:
  - Strip `CLAUDECODE` from the child environment to avoid nested-session errors.
  - Preserve `ANTHROPIC_API_KEY` by default for drop-in compatibility with users who authenticate Claude Code via API key.
  - Add an explicit opt-out env/flag only if implementation discovers a real need.
- Platform:
  - v1 targets Unix/macOS. Windows files and behavior are intentionally deferred.

## What Goes Where
- **Implementation Steps** (`[ ]` checkboxes): code changes, tests, docs, and verification inside `fya`.
- **Post-Completion** (no checkboxes): manual live Claude checks and consuming-project configuration.

## Implementation Steps

### Task 1: Scaffold Go CLI Project

**Files:**
- Create: `go.mod`
- Create: `app/main.go`
- Create: `app/main_test.go`
- Create: `README.md`
- Create: `AGENTS.md`
- Create: `CONTRIBUTING.md`
- Create: `LICENSE`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.golangci.yml`
- Create: `.github/workflows/ci.yml`
- Create: `.github/dependabot.yml`

- [x] initialize Go module for `github.com/umputun/fya`
- [x] create `app/main.go` with go-flags parsing, lgr setup, version handling, panic logging, signal handling, and a placeholder `run` function from the Go bootstrap skill
- [x] add bootstrap files from Go project guidance: Makefile, README, AGENTS, license, linter config, CI, dependabot, gitignore
- [x] align skeleton with sibling project conventions: `revdiff`-style `app/` composition root, sibling Makefile versioning, and strict linter config
- [x] write tests for startup, help/version plumbing, and missing prompt behavior
- [x] run `go test ./...` - must pass before next task
- [x] run `go test -cover ./...` - coverage is 80.0%
- [x] run `golangci-lint run ./... --allow-parallel-runners` - 0 issues
- [x] run `go build ./app` - scaffold builds
- [x] run `make test` after `app/` alignment - coverage is 82.6%
- [x] run `make build` after `app/` alignment
- [x] run `make lint` after `app/` alignment - 0 issues

### Task 2: Implement Claude-Compatible Option Parsing

**Files:**
- Create: `app/options/options.go`
- Create: `app/options/options_test.go`
- Modify: `app/main.go`

- [x] parse print-compatible flags: `-p`, `--print`, `--output-format`, `--input-format`, `--include-partial-messages`, `--include-hook-events`, `--replay-user-messages`
- [x] parse wrapper controls: `--idle-timeout`, `--turn-timeout`, `--cwd`, `--typing-wpm`, `--typing-jitter`, `--readiness-timeout`
- [x] split flags into consumed `fya` flags and explicitly recognized interactive Claude launch flags
- [x] collect Claude launch flags for interactive `claude`, including `--model`, `--effort`, permissions, tools, MCP/config flags, and `--verbose`
- [x] reject unknown flags with Claude-style errors instead of blindly forwarding them
- [x] validate output/input format values with Claude-style error messages
- [x] write table-driven tests for split flag forms, consumed flags, forwarded flags, rejected unknown flags, invalid formats, and default `-p` text behavior
- [x] run `go test ./...` - passed via `make test`, total coverage 86.6%

### Task 3: Add Prompt Input Handling

**Files:**
- Create: `app/input/input.go`
- Create: `app/input/input_test.go`
- Modify: `app/main.go`

- [x] read prompt from stdin when stdin has data
- [x] join positional arguments as fallback prompt for direct CLI usage
- [x] support `--input-format stream-json` by extracting exactly one `user` message text from JSONL stdin
- [x] reject multiple stream-json user messages with a clear error
- [x] support `--replay-user-messages` by replaying the accepted raw user event to stdout for stream-json compatibility
- [x] write tests for stdin prompt, positional prompt, empty prompt, stream-json user extraction, multiple user rejection, and replay behavior
- [x] run `go test ./...` - passed via `make test`, total coverage 86.5%

### Task 4: Implement PTY Process Driver

**Files:**
- Create: `app/ptyrun/driver.go`
- Create: `app/ptyrun/driver_unix.go`
- Create: `app/ptyrun/driver_test.go`
- Modify: `go.mod`

- [x] add `github.com/creack/pty` dependency
- [x] start interactive `claude` in a PTY with cwd, environment filtering, window size, and process cleanup
- [x] drain PTY output into a bounded ring buffer for readiness/debugging
- [x] strip `CLAUDECODE` while preserving `ANTHROPIC_API_KEY` by default for `claude -p` drop-in compatibility
- [x] add Unix/macOS build tags and explicitly leave Windows unsupported in v1
- [x] write tests using fake commands for startup failure, process cleanup, PTY output draining, env filtering, API-key preservation, and unsupported-platform behavior where testable
- [x] run `go test ./...` - passed via `make test`, total coverage 80.5%

WARNING: local sandbox blocks subprocess execution from PTY tests with `operation not permitted`; those integration assertions skip only on that sandbox denial. Pure driver/config/process tests still run, and live PTY behavior is covered by the same helper tests in normal environments.

### Task 5: Add Readiness Detection

**Files:**
- Create: `app/ready/ready.go`
- Create: `app/ready/ready_test.go`
- Modify: `app/ptyrun/driver.go`

- [x] detect Claude interactive input readiness from PTY ring buffer using current Claude input glyphs and fallback quiet-time heuristics
- [x] make readiness timeout configurable and non-fatal by default, with warning to stderr
- [x] avoid matching stale output from previous runs by using only fresh PTY output from this process
- [x] write tests for glyph readiness, quiet fallback, timeout warning, and process-exit-before-ready
- [x] run `go test ./...` - passed via `make test`, total coverage 80.0%

### Task 6: Implement Human Typing Injector

**Files:**
- Create: `app/typing/typing.go`
- Create: `app/typing/typing_test.go`
- Modify: `app/ptyrun/driver.go`

- [x] calculate per-rune delay from configurable WPM and jitter
- [x] estimate prompt typing duration and fail before launch when it cannot fit inside `--turn-timeout`
- [x] warn to stderr when prompt typing is expected to be unusually long
- [x] type prompt rune-by-rune to the PTY
- [x] preserve multiline prompt content without submitting early
- [x] send final Enter after a settle delay
- [x] write deterministic tests with injectable clock/rand/writer for WPM math, jitter bounds, duration estimation, timeout guard, warning threshold, multiline input, and final Enter behavior
- [x] run `go test ./...` - passed via `make test`, total coverage 80.4%

### Task 7: Implement Transcript Discovery and Tailing

**Files:**
- Create: `app/transcript/path.go`
- Create: `app/transcript/tail.go`
- Create: `app/transcript/events.go`
- Create: `app/transcript/transcript_test.go`

- [x] locate candidate Claude transcript files for the invocation cwd under `~/.claude/projects`
- [x] prefer files changed after prompt start and containing the exact user prompt
- [x] tail JSONL from an offset without scanner line-length limits
- [x] extract assistant text from `assistant`, `result`, and partial assistant events
- [x] track pending tool calls using `tool_use` and `tool_result` events
- [x] write tests for path encoding, transcript selection, large-line reading, text extraction, pending tools, result completion, and idle completion
- [x] run `go test ./...` - passed via `make test`, total coverage 81.0%

### Task 8: Implement Stream-JSON Output Synthesizer

**Files:**
- Create: `app/stream/stream.go`
- Create: `app/stream/stream_test.go`
- Modify: `app/main.go`

- [x] emit Claude-compatible `content_block_delta` events for assistant text chunks
- [x] emit final `result` event with `type`, `subtype`, `is_error`, `result`, `session_id`, `num_turns`, and `terminal_reason`
- [x] define stream-json final `result` behavior so already-streamed text is not appended a second time by Ralphex
- [x] implement `--output-format text` by collecting text and printing at completion
- [x] implement `--output-format json` by emitting one final result object
- [x] honor `--include-hook-events` and `--include-partial-messages` where transcript data allows it
- [x] write tests for stream-json event shape, final result shape, no duplicate Ralphex output, text output, json output, and hook/partial filtering
- [x] run `go test ./...` - passed via `make test`, total coverage 81.4%

### Task 9: Wire End-to-End Turn Execution

**Files:**
- Modify: `app/main.go`
- Modify: `app/main_test.go`
- Create: `app/turn/runner.go`
- Create: `app/turn/runner_test.go`
- Create: `app/turn/mocks/*.go`

- [x] connect option parsing, prompt input, PTY launch, readiness wait, typing, transcript tailing, streaming, and cleanup
- [x] ensure context cancellation and turn timeout kill the whole Claude process group
- [x] emit stderr diagnostics without corrupting stdout JSONL
- [x] ensure final fallback `result` is emitted only when appropriate and never duplicated
- [x] write integration tests with fake Claude/transcript writer covering success, timeout, process failure, and no transcript found
- [x] run `go test ./...` - passed via `make test`, total coverage 80.7%

WARNING: Task 9 uses dependency-injected turn tests with moq-generated mocks rather than a fake Claude binary. This keeps the orchestration testable under the local sandbox, where PTY subprocess execution can be denied.

### Task 10: Add Ralphex Contract Tests

**Files:**
- Create: `app/compat/ralphex_test.go`
- Create: `app/compat/testdata/ralphex/*.jsonl`
- Modify: `README.md`

- [x] assert generated stream output contains only JSONL lines accepted by Ralphex's documented custom-provider contract
- [x] test that `content_block_delta` text preserves `<<<RALPHEX:...>>>` signals verbatim
- [x] test that final `result` does not duplicate already streamed content in Ralphex's accumulated output path
- [x] test command shape compatible with Ralphex defaults: `--dangerously-skip-permissions --output-format stream-json --verbose --print` plus stdin prompt
- [x] document Ralphex configuration example in `README.md`
- [x] run `go test ./...` - passed
- [x] run `make lint` - 0 issues
- [x] run `make test` - total coverage 80.7%

### Task 11: Add Optional Live Parity Script

**Files:**
- Create: `scripts/compare-claude-p.sh`
- Create: `scripts/README.md`
- Modify: `README.md`

- [x] add an opt-in script that runs a small prompt through real `claude -p` and `fya`
- [x] compare text output, JSON result fields, stream-json validity, and final result fields
- [x] clearly mark the script as quota-consuming and not part of default tests
- [x] add a dry-run/static validation path for the script that does not call Claude
- [x] document live parity as a manual quota-consuming check
- [x] run `bash -n scripts/compare-claude-p.sh` - passed
- [x] run `scripts/compare-claude-p.sh --dry-run` - passed, no Claude call
- [x] run `go test ./...` - passed

### Task 12: Verify Acceptance Criteria

- [x] verify wrapper accepts Ralphex's default Claude args and prompt-on-stdin flow
- [x] verify wrapper starts interactive Claude in PTY, not `claude -p`
- [x] verify typing rate defaults to roughly 100 WPM with jitter
- [x] verify large prompts fail before launch when estimated typing time exceeds `--turn-timeout`
- [x] verify stdout is valid JSONL for `--output-format stream-json`
- [x] verify completion is driven by Claude transcript logs
- [x] verify Claude process is cleaned up after final result
- [x] verify v1 clearly reports unsupported Windows builds/usage
- [x] remove bootstrap stdout banner from normal runs so JSONL stdout stays clean
- [x] run full test suite: `go test ./...` - passed
- [x] run `make lint` - 0 issues
- [x] run `make build` - passed
- [x] run `make test` - total coverage 80.9%
- [x] do not run optional live parity script without explicit approval: `scripts/compare-claude-p.sh`

### Task 13: Final Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/20260515-claude-pty-print-wrapper.md`

- [x] document install/build instructions
- [x] document CLI compatibility and supported flags
- [x] document environment variables: `FYA_CLAUDE_DIR`, typing controls, timeout controls
- [x] document known limitations around transcript schema and byte-for-byte stream parity
- [x] move this plan to `docs/plans/completed/` after implementation is complete

## Post-Completion
*Items requiring manual intervention or external systems - no checkboxes*

**Manual verification**
- Run `fya --dangerously-skip-permissions --output-format stream-json --verbose --print` with a simple prompt and inspect JSONL output.
- Configure Ralphex with `claude_command = /path/to/fya` and `claude_args = --dangerously-skip-permissions --output-format stream-json --verbose`.
- Run a small Ralphex plan against a disposable repo and verify progress logs show streamed text plus expected `<<<RALPHEX:...>>>` signal.

**External system updates**
- Decide whether Ralphex documentation should mention `fya` alongside the existing Codex, Copilot, Gemini, and OpenCode wrappers.
- Decide whether live parity checks should be part of a manual release checklist rather than CI.
