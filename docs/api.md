# Go API Reference

## Launch Options

`camoufox.BuildLaunchOptions(opts)` is the shared parity pipeline used by
Playwright compatibility launches and the native WebDriver BiDi server. It
builds Camoufox config, Firefox prefs, environment chunks, proxy settings,
fingerprints, GeoIP, locales, addons, and virtual display state.

## Browser Launch

`camoufox.New(ctx, opts)` and `camoufox.NewContext(ctx, existing, opts)` remain
available for Phase 1-3 Playwright compatibility.

## Native WebDriver BiDi Server

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

headless := camoufox.HeadlessTrue
endpoint, err := camoufox.LaunchServer(ctx, &camoufox.LaunchOptions{
    Headless: &headless,
    OS:       []string{"windows"},
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(endpoint)
```

`LaunchServer` returns a `ws://127.0.0.1:PORT/session` endpoint. The server
owns the Camoufox process and closes it when the context is cancelled or the
client disconnects.

For embedded lifecycle control, build options first and use:

```go
built, err := camoufox.BuildLaunchOptions(opts)
server, err := camoufox.LaunchServerHandle(ctx, built)
defer server.Close()
```
