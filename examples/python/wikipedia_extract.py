import asyncio
import json
import os

import websockets


async def main() -> None:
    endpoint = os.environ.get("CAMOUFOX_BIDI_ENDPOINT")
    if not endpoint:
        raise SystemExit("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")

    async with websockets.connect(endpoint) as ws:
        await command(ws, 1, "session.new", {"capabilities": {}})
        created = await command(ws, 2, "browsingContext.create", {"type": "tab"})
        context = created["context"]

        await command(
            ws,
            3,
            "browsingContext.navigate",
            {
                "context": context,
                "url": "https://en.wikipedia.org/wiki/WebDriver",
                "wait": "complete",
            },
        )

        result = await command(
            ws,
            4,
            "script.evaluate",
            {
                "target": {"context": context},
                "awaitPromise": True,
                "expression": """
                    ({
                      title: document.querySelector('h1')?.textContent?.trim(),
                      firstParagraph: Array.from(document.querySelectorAll('p'))
                        .map((p) => p.textContent.trim())
                        .find(Boolean),
                      links: Array.from(document.querySelectorAll('#mw-content-text a[href^="/wiki/"]'))
                        .slice(0, 8)
                        .map((a) => a.textContent.trim())
                        .filter(Boolean)
                    })
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
