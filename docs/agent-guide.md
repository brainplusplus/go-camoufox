# Agent Guide

This guide is for AI agents and automation runners using `go-camoufox`.

## Mental Model

`go-camoufox` is a Go launcher/orchestration layer for the Camoufox Firefox
binary. It does not recompile Firefox. The Phase 4 primary runtime is:

1. Build Camoufox launch config with `BuildLaunchOptions`.
2. Launch the installed Camoufox executable with Firefox WebDriver BiDi enabled.
3. Print or return a `ws://127.0.0.1:PORT/session` endpoint.
4. Drive the browser with WebDriver BiDi commands over WebSocket.

The Playwright path still exists for compatibility, but agents should prefer
the native BiDi server for new work.

## Install Or Locate Browser

List installed browsers:

```bash
go run ./cmd/go-camoufox list
```

Fetch the latest configured release:

```bash
go run ./cmd/go-camoufox fetch
```

Fetch a specific available version:

```bash
go run ./cmd/go-camoufox fetch --version 135.0.1-beta.24
```

Print cache path:

```bash
go run ./cmd/go-camoufox path
```

Useful env vars:

- `GO_CAMOUFOX_CACHE`: override browser/cache directory.
- `GO_CAMOUFOX_LIVE=1`: enable live browser tests.
- `GO_CAMOUFOX_EXECUTABLE`: explicit Camoufox executable for live tests.

## Start Native BiDi Server

Recommended local smoke server:

```bash
go run ./cmd/go-camoufox server --headless --no-default-addons --os windows --i-know-what-im-doing
```

The command prints only the endpoint to stdout. Browser and server diagnostics
go to stderr.

Example endpoint:

```text
ws://127.0.0.1:50123/session
```

Set it for examples:

```bash
export CAMOUFOX_BIDI_ENDPOINT=ws://127.0.0.1:50123/session
```

PowerShell:

```powershell
$env:CAMOUFOX_BIDI_ENDPOINT = "ws://127.0.0.1:50123/session"
```

## Safe Smoke Commands

Run deterministic local form automation:

```bash
go run ./examples/go/web_form_demo
```

Run raw BiDi client:

```bash
go run ./examples/go/raw_bidi_client
```

Run tests:

```bash
go test ./...
go vet ./...
```

Run live BiDi test against an explicit executable:

```bash
GO_CAMOUFOX_LIVE=1 GO_CAMOUFOX_EXECUTABLE=/path/to/camoufox go test ./protocol/bidi -run TestLiveCamoufoxBiDiSmoke -v
```

## BiDi Command Pattern

Every command should include `id`, `method`, and `params`. Even empty params
must be sent as `{}` for Firefox:

```json
{"id":1,"method":"session.status","params":{}}
```

Minimal practical flow:

1. `session.new`
2. `browsingContext.create`
3. `browsingContext.navigate`
4. `script.evaluate`
5. `browsingContext.close`
6. `session.end`

Public websites can change selectors or apply bot checks. Prefer local demo
pages for setup validation and public examples only as best-effort workflows.

## Common Launch Options

CLI:

```bash
go run ./cmd/go-camoufox server \
  --headless \
  --browser beta.24 \
  --os windows \
  --locale en-US \
  --proxy-server socks5://127.0.0.1:1080
```

Flexible flags:

- `--listen HOST:PORT`: bind the local BiDi gateway. Use `0.0.0.0:9222` in Docker.
- `--config KEY=VALUE`: Camoufox config values.
- `--pref KEY=VALUE`: Firefox user prefs.
- `--env KEY=VALUE`: environment variables.
- `--arg VALUE`: raw browser arguments.
- `--options-json VALUE`: inline JSON or path to JSON matching Go `LaunchOptions`.

## Troubleshooting

- `camoufox is not installed`: run `go-camoufox fetch` or set `--executable-path`.
- Server prints no endpoint: inspect stderr; Firefox must support
  `--remote-debugging-port`.
- `Expected "params" to be an object`: send `"params": {}`.
- Public-site example fails: check page selectors and network access first.
- Browser process remains after forced termination: stop the parent server
  gracefully when possible. On Windows, killing a parent process from outside
  can leave child Firefox processes.

## Files Agents Should Read First

- `docs/cli.md`
- `docs/api.md`
- `examples/README.md`
