import WebSocket from "ws";

const endpoint = process.env.CAMOUFOX_BIDI_ENDPOINT;
if (!endpoint) {
  throw new Error("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
}

const ws = new WebSocket(endpoint);
const pending = new Map();

ws.on("message", (data) => {
  const message = JSON.parse(data.toString());
  if (!message.id || !pending.has(message.id)) return;
  const { resolve, reject } = pending.get(message.id);
  pending.delete(message.id);
  if (message.type === "success") resolve(message.result ?? {});
  else reject(new Error(JSON.stringify(message, null, 2)));
});

await once(ws, "open");

let nextId = 1;
const command = (method, params = {}) => {
  const id = nextId++;
  ws.send(JSON.stringify({ id, method, params }));
  return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
};

console.log("status:", await command("session.status"));
await command("session.new", { capabilities: {} });

const created = await command("browsingContext.create", { type: "tab" });
const context = created.context;

await command("browsingContext.navigate", {
  context,
  url: "data:text/html,<title>go-camoufox</title><h1>hello from Node</h1>",
  wait: "complete",
});

const evaluated = await command("script.evaluate", {
  expression: "document.querySelector('h1').textContent",
  target: { context },
  awaitPromise: true,
});

console.log(evaluated.result.value);
await command("browsingContext.close", { context });
await command("session.end");
ws.close();

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve);
    emitter.once("error", reject);
  });
}
