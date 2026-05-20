import WebSocket from "ws";

const endpoint = process.env.CAMOUFOX_BIDI_ENDPOINT;
if (!endpoint) {
  throw new Error("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
}

const ws = new WebSocket(endpoint);
const pending = new Map();
let nextId = 1;

ws.on("message", (data) => {
  const message = JSON.parse(data.toString());
  if (!message.id || !pending.has(message.id)) return;
  const { resolve, reject } = pending.get(message.id);
  pending.delete(message.id);
  if (message.type === "success") resolve(message.result ?? {});
  else reject(new Error(JSON.stringify(message, null, 2)));
});

await once(ws, "open");
await command("session.new", { capabilities: {} });

const created = await command("browsingContext.create", { type: "tab" });
const context = created.context;

await command("browsingContext.navigate", {
  context,
  url: "https://news.ycombinator.com/",
  wait: "complete",
});

const evaluated = await command("script.evaluate", {
  target: { context },
  awaitPromise: true,
  expression: `
    Array.from(document.querySelectorAll(".athing")).slice(0, 10).map((row) => ({
      rank: row.querySelector(".rank")?.textContent?.trim(),
      title: row.querySelector(".titleline a")?.textContent?.trim(),
      url: row.querySelector(".titleline a")?.href
    }))
  `,
});

console.table(evaluated.result.value);

await command("browsingContext.close", { context });
await command("session.end", {});
ws.close();

function command(method, params = {}) {
  const id = nextId++;
  ws.send(JSON.stringify({ id, method, params }));
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}
