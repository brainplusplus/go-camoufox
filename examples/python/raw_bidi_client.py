import asyncio
import json
import os

import websockets


async def main() -> None:
    endpoint = os.environ.get("CAMOUFOX_BIDI_ENDPOINT")
    if not endpoint:
        raise SystemExit("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")

    async with websockets.connect(endpoint) as ws:
        await send(ws, 1, "session.status", {})
        print("status:", await read_result(ws, 1))

        await send(ws, 2, "session.new", {"capabilities": {}})
        await read_result(ws, 2)

        await send(ws, 3, "browsingContext.create", {"type": "tab"})
        created = await read_result(ws, 3)
        context = created["context"]

        await send(
            ws,
            4,
            "browsingContext.navigate",
            {
                "context": context,
                "url": "data:text/html,<title>go-camoufox</title><h1>hello from Python</h1>",
                "wait": "complete",
            },
        )
        await read_result(ws, 4)

        await send(
            ws,
            5,
            "script.evaluate",
            {
                "expression": "document.querySelector('h1').textContent",
                "target": {"context": context},
                "awaitPromise": True,
            },
        )
        evaluated = await read_result(ws, 5)
        print(evaluated["result"]["value"])

        await send(ws, 6, "browsingContext.close", {"context": context})
        await read_result(ws, 6)
        await send(ws, 7, "session.end", {})
        await read_result(ws, 7)


async def send(ws, command_id: int, method: str, params: dict) -> None:
    await ws.send(json.dumps({"id": command_id, "method": method, "params": params}))


async def read_result(ws, command_id: int) -> dict:
    while True:
        message = json.loads(await ws.recv())
        if message.get("id") != command_id:
            continue
        if message.get("type") != "success":
            raise RuntimeError(json.dumps(message, indent=2))
        return message.get("result", {})


if __name__ == "__main__":
    asyncio.run(main())
