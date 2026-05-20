# Migration From Python Camoufox

Python:

```python
from camoufox import launch_server

launch_server(headless=True, os=["windows"], locale=["en-US"])
```

Go CLI:

```bash
go-camoufox server --headless --os windows --locale en-US
```

Go API:

```go
headless := camoufox.HeadlessTrue
endpoint, err := camoufox.LaunchServer(ctx, &camoufox.LaunchOptions{
    Headless: &headless,
    OS:       []string{"windows"},
    Locale:   []string{"en-US"},
})
```

The Phase 4 server does not require Python or Node in the main runtime path.
It still uses the Camoufox Firefox binary from the Python project release
artifacts. Install or update that binary with `go-camoufox fetch`.

Supported v1.0 BiDi smoke scope:

- `session.status`
- `session.new`
- `session.end` / `session.delete` as supported by the browser endpoint
- `browsingContext.create`
- `browsingContext.close`
- `browsingContext.getTree`
- `browsingContext.navigate`
- `browsingContext.reload`
- `script.evaluate`
- `script.callFunction`
- browser-emitted log/events for those flows

Unsupported commands are passed through to Firefox. If the underlying
Camoufox/Firefox build does not support a command, the browser returns the BiDi
error directly.
