# go-camoufox examples

These examples cover the Phase 4 native WebDriver BiDi server.

## Start a server

CLI:

```bash
go run ./cmd/go-camoufox server --headless --no-default-addons --os windows
```

The command prints a WebSocket endpoint such as:

```text
ws://127.0.0.1:50123/session
```

Use that value as `CAMOUFOX_BIDI_ENDPOINT` for the client examples:

```bash
export CAMOUFOX_BIDI_ENDPOINT=ws://127.0.0.1:50123/session
```

PowerShell:

```powershell
$env:CAMOUFOX_BIDI_ENDPOINT = "ws://127.0.0.1:50123/session"
```

## Examples

- `go/launch_server`: launch a native BiDi server from the Go API.
- `go/raw_bidi_client`: connect to a BiDi endpoint using a plain WebSocket.
- `go/web_form_demo`: automate a real local HTML form, click a button, and read DOM output.
- `python/raw_bidi_client.py`: Python raw WebSocket BiDi smoke flow.
- `python/wikipedia_extract.py`: open a real Wikipedia page and extract heading, paragraph, and links.
- `python/search_duckduckgo_html.py`: search DuckDuckGo HTML and extract the first result cards.
- `node/raw_bidi_client.mjs`: Node raw WebSocket BiDi smoke flow.
- `node/form_interaction_local.mjs`: fill and submit a local checkout form.
- `node/hacker_news_titles.mjs`: open Hacker News and print the top story titles.
- `bidi/*`: examples for common automation libraries.
- `reference_ports/creepjs_playwright`: Go equivalent of `references/camoufox/example/example.py`.
- `reference_ports/creepjs_simple_playwright`: Go equivalent of `example_simple.py`.
- `reference_ports/httpbin_concurrent_playwright`: Go equivalent of `async_example.py`.
- `reference_ports/creepjs_bidi`: Phase 4 native BiDi version of the CreepJS example.
- `reference_ports/test_google_playwright`: Playwright-style Google smoke example close to the Python sync API sample.
- `reference_ports/test_google_python_reference.py`: upstream Python Camoufox version of the Google smoke example for side-by-side comparison.

Raw BiDi examples exercise the supported v1.0 subset directly:

- `session.status`
- `session.new`
- `browsingContext.create`
- `browsingContext.navigate`
- `script.evaluate`
- `browsingContext.close`
- `session.end`

## Run A Practical Example

Terminal 1:

```bash
go run ./cmd/go-camoufox server --headless --no-default-addons --os windows
```

Terminal 2:

```bash
export CAMOUFOX_BIDI_ENDPOINT=ws://127.0.0.1:50123/session
go run ./examples/go/web_form_demo
```

Python:

```bash
python -m pip install websockets
python examples/python/wikipedia_extract.py
SEARCH_QUERY="Camoufox browser" python examples/python/search_duckduckgo_html.py
```

Node:

```bash
npm install ws
node examples/node/form_interaction_local.mjs
node examples/node/hacker_news_titles.mjs
```

The local form examples are deterministic and are the best starting point for
debugging your setup. The public-site examples are intentionally small because
real websites can change selectors or add bot checks over time.

## Python Reference Ports

The closest Go version of this Python sample:

```python
from camoufox.sync_api import Camoufox

with Camoufox(headless=False) as browser:
    page = browser.new_page()
    page.goto("https://abrahamjuliot.github.io/creepjs/")
```

is:

```bash
go run ./examples/reference_ports/creepjs_playwright
```

That example uses the Playwright compatibility API, so it looks and behaves
closest to Python Camoufox. For the Phase 4 native WebDriver BiDi path, run:

```bash
go run ./examples/reference_ports/creepjs_bidi
```

The concurrent async Python reference example maps to goroutines:

```bash
go run ./examples/reference_ports/httpbin_concurrent_playwright
```
