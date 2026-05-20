import asyncio
import json
import os
import urllib.parse

import websockets


async def main() -> None:
    endpoint = os.environ.get("CAMOUFOX_BIDI_ENDPOINT")
    if not endpoint:
        raise SystemExit("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")

    query = os.environ.get("SEARCH_QUERY", "WebDriver BiDi Firefox")
    url = "https://html.duckduckgo.com/html/?" + urllib.parse.urlencode({"q": query})

    async with websockets.connect(endpoint) as ws:
        await command(ws, 1, "session.new", {"capabilities": {}})
        context = (await command(ws, 2, "browsingContext.create", {"type": "tab"}))["context"]
        await command(ws, 3, "browsingContext.navigate", {"context": context, "url": url, "wait": "complete"})

        result = await command(
            ws,
            4,
            "script.evaluate",
            {
                "target": {"context": context},
                "awaitPromise": True,
                "expression": """
                    Array.from(document.querySelectorAll('.result')).slice(0, 5).map((result) => ({
                      title: result.querySelector('.result__title')?.textContent?.trim(),
                      url: result.querySelector('.result__url')?.textContent?.trim(),
                      snippet: result.querySelector('.result__snippet')?.textContent?.trim()
                    }))
                """,
            },
        )
        print(json.dumps(result["result"]["value"], indent=2, ensure_ascii=False))

        await command(ws, 5, "browsingContext.close", {"context": context})
        await command(ws, 6, "session.end", {})


async def command(ws, command_id: int, method: str, params: dict) -> dict:
    await ws.send(json.dumps({"id": command_id, "method": method, "params": params}))
    while True:
        message = json.loads(await ws.recv())
        if message.get("id") != command_id:
            continue
        if message.get("type") != "success":
            raise RuntimeError(json.dumps(message, indent=2))
        return message.get("result", {})


if __name__ == "__main__":
    asyncio.run(main())
