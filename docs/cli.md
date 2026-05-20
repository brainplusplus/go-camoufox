# go-camoufox CLI

`go-camoufox` mirrors the Python Camoufox launcher commands while keeping the
Phase 4 server path native to Go.

## Commands

- `go-camoufox fetch [--version VERSION] [--replace]`
- `go-camoufox fetch --list`
- `go-camoufox list`
- `go-camoufox path`
- `go-camoufox info`
- `go-camoufox version`
- `go-camoufox run [--headless] [--url URL]`
- `go-camoufox server [options]`

## Server

`go-camoufox server` launches Camoufox with the shared launch-options pipeline,
starts a local WebDriver BiDi WebSocket endpoint, prints that endpoint to
stdout, and writes diagnostics to stderr.

Common options:

```bash
go-camoufox server \
  --listen 127.0.0.1:0 \
  --headless \
  --browser beta.25 \
  --os windows \
  --locale en-US \
  --proxy-server socks5://127.0.0.1:1080
```

Flexible parity flags:

- `--config KEY=VALUE` sets Camoufox config values.
- `--pref KEY=VALUE` sets Firefox user prefs.
- `--env KEY=VALUE` sets environment variables.
- `--arg VALUE` appends raw browser args.
- `--options-json VALUE` accepts inline JSON or a path to a JSON file matching
  the Go `LaunchOptions` field names.

The server currently supports the WebDriver BiDi commands implemented by the
bundled Camoufox/Firefox Remote Agent. The v1.0 smoke scope is session,
browsing context, script, and log/event flows.
