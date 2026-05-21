# Go vs Python Camoufox Parity Audit

This audit compares `go-camoufox` against the pinned upstream Python reference
under `references/camoufox`.

## Current Parity Status

| Area | Status | Notes |
| --- | --- | --- |
| Launch option shape | Mostly matched | Go exposes the main Python `launch_options()` inputs as typed `LaunchOptions`, including the `allow_webgl` compatibility alias. |
| Browser executable resolution | Matched | Go resolves installed browser versions and explicit executable paths. |
| Firefox version default | Matched for installed Go cache | Go now derives the default Firefox major from the installed browser metadata when possible, matching Python's installed-version behavior more closely. Fallback remains `150`. |
| Fingerprint presets | Matched | Preset loading, v150 preset selection, conversion, fonts, voices, and seed fields are covered by parity tests. |
| Default synthetic fingerprint | Matched for Camoufox-used fields | Go now uses `github.com/brainplusplus/go-browserforge`, which embeds the same Apify BrowserForge network data and generates Firefox BrowserForge fingerprints before applying Camoufox's `browserforge.yml` mapping. |
| `screen` and `window` launch options | Matched for supported fields | Go honors `Screen` constraints and `Window`, and emits `window.outer*`, `window.inner*`, and `window.screen*` config fields. |
| WebGL config | Mostly matched | Go samples matching WebGL data and sets WebGL prefs. |
| GeoIP | Mostly matched | Go supports explicit IP and auto lookup through proxy, sets WebRTC and geolocation config, and preserves user config precedence. |
| Locale | Mostly matched | Go routes explicit locales through the locale handler. |
| Addons | Mostly matched | Default addon handling, exclusion, and path validation are present. |
| Fonts and voices | Mostly matched | Random subsets and marker fonts are covered by tests. |
| Headless and virtual display | Mostly matched | Go supports `false`, `true`, and Linux virtual-display mode. |
| Linux fontconfig env | Matched | On Linux, Go now sets `FONTCONFIG_FILE` from bundled Camoufox fontconfig when available, with user env overrides preserved. |
| Persistent context | Improved | Go passes generated screen, viewport, user agent, timezone, and locale into Playwright persistent context options. |
| Per-context fingerprinting | Mostly matched | Go exposes `Browser.NewBrowserContext` / `NewContextFromBrowser`, applies generated context options, and injects the init script like Python `NewContext(browser, ...)`. |
| Native BiDi server | Go-specific | This is Phase 4 functionality and not a direct upstream Python API. |

## Known Remaining Gaps

- `go-browserforge` is an idiomatic Go port, not a line-for-line Python API
  clone. The Camoufox-relevant behavior uses the same BrowserForge/Apify model
  files, header/fingerprint consistency constraints, and screen/window mapping.
- BrowserForge screen constraints are matched to upstream behavior: if a
  non-strict constraint cannot be satisfied by the model, BrowserForge relaxes
  it instead of clamping to a synthetic impossible screen size.
- Python accepts arbitrary extra Playwright launch options through
  `**launch_options`. Go stores `Extra`, but only forwards the typed options it
  explicitly supports.
- Python's full async API has no direct Go equivalent. Go users rely on Go
  goroutines plus Playwright-Go or native BiDi instead.

## Verification Notes

- `go test ./...` covers the core parity helpers and launch option builder.
- `go vet ./...` should remain clean before release.
- Live Google comparison is not a reliable anti-detection verdict by itself:
  both upstream Python and Go can hit Google challenges depending on IP,
  cookies, timing, and target-side risk scoring.
