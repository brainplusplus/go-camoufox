import http from "node:http";
import WebSocket from "ws";

const endpoint = process.env.CAMOUFOX_BIDI_ENDPOINT;
if (!endpoint) {
  throw new Error("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
}

const site = http.createServer((_, response) => {
  response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
  response.end(`<!doctype html>
<title>Checkout demo</title>
<h1>Checkout</h1>
<label>Name <input id="name"></label>
<label>Email <input id="email"></label>
<button id="submit">Submit</button>
<output id="summary"></output>
<script>
submit.addEventListener("click", () => {
  summary.value = name.value + " <" + email.value + ">";
});
</script>`);
});

await new Promise((resolve) => site.listen(0, "127.0.0.1", resolve));
const { port } = site.address();
const pageUrl = `http://127.0.0.1:${port}`;

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
const { context } = await command("browsingContext.create", { type: "tab" });
await command("browsingContext.navigate", { context, url: pageUrl, wait: "complete" });

const evaluated = await command("script.evaluate", {
  target: { context },
  awaitPromise: true,
  expression: `
    document.querySelector("#name").value = "Camoufox";
    document.querySelector("#email").value = "demo@example.com";
    document.querySelector("#submit").click();
    document.querySelector("#summary").value;
  `,
});

console.log(evaluated.result.value);

await command("browsingContext.close", { context });
await command("session.end", {});
ws.close();
site.close();

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
