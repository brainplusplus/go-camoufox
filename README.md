# go-camoufox

`go-camoufox` is a Go launcher and orchestration layer for the Camoufox Firefox
browser binary. It ports the Python Camoufox launch/config pipeline into Go and
adds a native WebDriver BiDi server path for multi-language browser automation.

Current release target: `0.1.0`.

## Status

This project is usable for early adopters, examples, and smoke automation. It
is not yet a `1.0` parity claim.

Implemented:

- Camoufox launch option builder with embedded config/fingerprint assets.
- Browser install/cache helpers.
- Playwright compatibility launch API.
- Native WebDriver BiDi server and CLI command.
- Browser pool, GeoIP/locale helpers, virtual display support, addon helpers.
- Cross-platform build workflow for Linux, macOS, and Windows.
- Docker runtime scaffold.

Known limitations:

- Live smoke has been verified with an installed `135.0.1-beta.24` Camoufox
  binary. The sprint manifest references `v150.0.2-beta.25`, but the current
  configured release list does not resolve that version.
- The native BiDi path is a Go launcher/gateway to Firefox Remote Agent. It
  does not reimplement every WebDriver BiDi command itself.
- Some client libraries assume WebDriver Classic session semantics; raw BiDi
  examples are the most reliable compatibility baseline.
- Docker build requires a running Docker daemon and a fetched/mounted browser
  cache, or `GO_CAMOUFOX_FETCH_ON_START=1`.

## Install Browser

List available releases:

```bash
go run ./cmd/go-camoufox fetch --list
```

Fetch the latest configured release:

```bash
go run ./cmd/go-camoufox fetch
```

List installed browsers:

```bash
go run ./cmd/go-camoufox list
```

## Native WebDriver BiDi Server

Start a local server:

```bash
go run ./cmd/go-camoufox server --headless --no-default-addons --os windows --i-know-what-im-doing
```

The endpoint is printed to stdout:

```text
ws://127.0.0.1:50123/session
```

Use it with examples:

```bash
export CAMOUFOX_BIDI_ENDPOINT=ws://127.0.0.1:50123/session
go run ./examples/go/web_form_demo
```

PowerShell:

```powershell
$env:CAMOUFOX_BIDI_ENDPOINT = "ws://127.0.0.1:50123/session"
go run ./examples/go/web_form_demo
```

For Docker or remote access, bind explicitly:

```bash
go run ./cmd/go-camoufox server --listen 0.0.0.0:9222 --headless --no-default-addons --os windows --i-know-what-im-doing
```

Do not expose a BiDi port to the public internet without network isolation.

## Go API

Playwright compatibility path:

```go
headless := camoufox.HeadlessTrue
browser, err := camoufox.New(ctx, &camoufox.LaunchOptions{
    Headless: &headless,
    OS:       []string{"windows"},
})
if err != nil {
    log.Fatal(err)
}
defer browser.Close(ctx)
```

Native BiDi server:

```go
built, err := camoufox.BuildLaunchOptions(opts)
server, err := camoufox.LaunchServerHandle(ctx, built)
fmt.Println(server.Endpoint())
defer server.Close()
```

## Examples

- `examples/reference_ports/creepjs_playwright`: Go port of the Python CreepJS example.
- `examples/reference_ports/creepjs_bidi`: Native BiDi version of the CreepJS example.
- `examples/reference_ports/httpbin_concurrent_playwright`: goroutine version of the async Python example.
- `examples/go/web_form_demo`: deterministic local form automation.
- `examples/python/*` and `examples/node/*`: raw BiDi public-site and form examples.

See [examples/README.md](examples/README.md).

## Docker

Build:

```bash
docker build -t go-camoufox:local .
```

Run:

```bash
docker run --rm -it -p 9222:9222 \
  -e GO_CAMOUFOX_FETCH_ON_START=1 \
  -v go-camoufox-cache:/home/camoufox/.cache/go-camoufox \
  go-camoufox:local
```

Endpoint:

```text
ws://127.0.0.1:9222/session
```

See [docs/docker.md](docs/docker.md).

## Release Check

Full local check:

```powershell
.\scripts\release-check.ps1
```

Without Docker build:

```powershell
.\scripts\release-check.ps1 -SkipDockerBuild
```

With live browser smoke:

```powershell
.\scripts\release-check.ps1 -SkipDockerBuild -Live -Executable "C:\path\to\camoufox.exe"
```

## Documentation

- [Agent guide](docs/agent-guide.md)
- [CLI reference](docs/cli.md)
- [Go API reference](docs/api.md)
- [Docker](docs/docker.md)
- [Migration from Python](docs/migration.md)
