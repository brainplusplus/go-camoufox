# Docker

The Docker image packages the `go-camoufox` CLI and Linux runtime dependencies
needed to launch Camoufox headlessly. It does not bake a Camoufox browser binary
into the default image; use a mounted cache or fetch on startup.

## Build

```bash
docker build -t go-camoufox:local .
```

## Run With Existing Cache

```bash
docker run --rm -it \
  -v go-camoufox-cache:/home/camoufox/.cache/go-camoufox \
  go-camoufox:local \
  server --headless --no-default-addons --os windows --i-know-what-im-doing
```

The image binds the server to `0.0.0.0:9222` by default, so host clients can
use:

```text
ws://127.0.0.1:9222/session
```

Run:

```bash
docker run --rm -it -p 9222:9222 \
  -v go-camoufox-cache:/home/camoufox/.cache/go-camoufox \
  go-camoufox:local
```

## Fetch On Start

```bash
docker run --rm -it \
  -e GO_CAMOUFOX_FETCH_ON_START=1 \
  -e GO_CAMOUFOX_FETCH_VERSION=135.0.1-beta.24 \
  -v go-camoufox-cache:/home/camoufox/.cache/go-camoufox \
  go-camoufox:local
```

If `GO_CAMOUFOX_FETCH_VERSION` is omitted, the launcher fetches the latest
version available from `repos.yml`.

## Compose

```bash
docker compose up --build
```

Use the named volume `go-camoufox-cache` to persist downloaded browser builds.

## Notes For Agents

- Prefer mounting a pre-populated `GO_CAMOUFOX_CACHE` for repeatable runs.
- Keep browser download separate from normal test loops.
- Public-site automation can fail due to selectors, network, or anti-bot
  behavior. Validate the container first with `examples/go/web_form_demo`.
