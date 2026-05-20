import { firefox } from "playwright";

const endpoint = process.env.CAMOUFOX_BIDI_ENDPOINT;
if (!endpoint) {
  throw new Error("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
}

const browser = await firefox.connect(endpoint);
const page = await browser.newPage();
await page.goto("https://example.com");
console.log(await page.title());
await browser.close();
