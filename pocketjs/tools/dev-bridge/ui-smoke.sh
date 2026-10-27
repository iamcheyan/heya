#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
scanner=${HEYA_REGISTRY_SCANNER:?set HEYA_REGISTRY_SCANNER to a compiled dump-apps binary}
pocketjs=${POCKETJS_ROOT:?set POCKETJS_ROOT to the PocketJS 0.15.0 checkout}
bridge=${HEYA_BRIDGE_BIN:?set HEYA_BRIDGE_BIN to a compiled bridge binary}
chrome=${CHROME_PATH:?set CHROME_PATH to a Chromium executable}
port=${PORT:-8142}
bridge_port=${BRIDGE_PORT:-8153}
tmp=$(mktemp -d)
server_pid=
bridge_pid=
cleanup() {
  if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi
  if [ -n "$bridge_pid" ]; then kill "$bridge_pid" 2>/dev/null || true; fi
  rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
apps="$tmp/apps"
mkdir -p "$apps/alpha"
printf '{"id":"alpha","name":"Alpha","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/alpha/app.json"
cp "$repo/pocketjs/src/heya-registry.ts" "$pocketjs/apps/heya-pocket/heya-registry.ts"
cp "$repo/pocketjs/src/heya-bridge.ts" "$pocketjs/apps/heya-pocket/heya-bridge.ts"
cp "$repo/pocketjs/heya-pocket-main.tsx" "$pocketjs/apps/heya-pocket/main.tsx"
cp "$repo/pocketjs/src/apps.generated.ts" "$pocketjs/apps/heya-pocket/apps.generated.ts"
cd "$pocketjs"
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국ス어테트한"
"$bridge" --apps "$apps" --listen "127.0.0.1:$bridge_port" > "$tmp/bridge.log" 2>&1 &
bridge_pid=$!
HEYA_BRIDGE_BASE="http://127.0.0.1:$bridge_port" HEYA_ROOT="$repo" HEYA_APPS="$apps" HEYA_REGISTRY_SCANNER="$scanner" HEYA_REGISTRY_BRIDGE="$repo/pocketjs/tools/dev-bridge/server.ts" PORT="$port" ~/.bun/bin/bun tools/dev/web/serve.ts > "$tmp/server.log" 2>&1 &
server_pid=$!
node - "$port" "$apps" "$repo/pocketjs/screenshots" "$chrome" <<'JS'
const { chromium } = require("playwright");
const fs = require("node:fs");
const port = process.argv[2], apps = process.argv[3], shots = process.argv[4], executablePath = process.argv[5];
const base = `http://127.0.0.1:${port}`;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    const errors = [];
    const events = [];
    const eventResponses = [];
    const failedRequests = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("requestfailed", (request) => failedRequests.push({ url: request.url(), failure: request.failure()?.errorText }));
    page.on("response", async (response) => {
      if (!response.url().includes("/bridge/events")) return;
      try {
        const batch = await response.json();
        eventResponses.push({ url: response.url(), status: response.status(), batch });
        events.push(...batch.events);
      } catch (error) { eventResponses.push({ url: response.url(), status: response.status(), error: String(error) }); }
    });
    const render = async () => {
      const rescanReply = page.waitForResponse((response) => response.url().endsWith("/bridge/cmd") && response.request().postDataJSON().t === "rescan");
      await page.goto(`${base}/?demo=heya-pocket-main`, { waitUntil: "networkidle" });
      await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
      const response = await rescanReply;
      if (!response.ok()) throw new Error(`Guest rescan HTTP ${response.status()}`);
      const reply = await response.json();
      const ids = reply.apps.map((app) => app.id);
      const state = await page.evaluate(() => {
        const canvas = document.querySelector("canvas"), data = canvas.getContext("2d").getImageData(0, 0, canvas.width, canvas.height).data;
        let hash = 5381, lit = 0;
        for (let i = 0; i < data.length; i++) { hash = ((hash * 33) ^ data[i]) >>> 0; if (i % 4 === 0 && data[i] > 160) lit++; }
        return { hash, lit };
      });
      return { ids, state, png: await page.locator("canvas").screenshot() };
    };
    const initial = await render();
    if (initial.ids.join(",") !== "alpha") throw new Error(`startup IDs: ${initial.ids}`);
    fs.writeFileSync(`${shots}/registry-ui-alpha.png`, initial.png);
    const status = await (await fetch(`${base}/heya/bridge/cmd`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ t: "get_status", id: 77 }) })).json();
    if (!status.ok || status.id !== 77 || status.status.running) throw new Error(`initial get_status failed: ${JSON.stringify(status)}`);
    const launchResponse = page.waitForResponse((response) => response.url().endsWith("/bridge/cmd") && response.request().postDataJSON().t === "launch");
    await page.keyboard.down("KeyX"); await sleep(350); await page.keyboard.up("KeyX");
    const response = await launchResponse;
    const launchReply = await response.json();
    if (!response.ok() || !launchReply.ok || launchReply.status.app_id !== "alpha") throw new Error(`UI launch reply failed: ${JSON.stringify(launchReply)}`);
    const until = Date.now() + 5_000;
    while (Date.now() < until && !(events.some((event) => event.type === "app_started") && events.some((event) => event.type === "app_exited"))) await sleep(50);
    if (!events.some((event) => event.type === "app_started") || !events.some((event) => event.type === "app_exited")) {
      const cursor = events.at(-1)?.id ?? 0;
      const observed = await (await fetch(`${base}/heya/bridge/events?since=${cursor}`)).json();
      throw new Error(`UI lifecycle events missing; seen=${JSON.stringify(events)} responses=${JSON.stringify(eventResponses)} failed=${JSON.stringify(failedRequests)} remaining=${JSON.stringify(observed)} log=${await page.locator("#log").textContent()}`);
    }
    await sleep(300);
    fs.writeFileSync(`${shots}/bridge-launch-return.png`, await page.locator("canvas").screenshot());
    const stopped = await (await fetch(`${base}/heya/bridge/cmd`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ t: "get_status", id: 78 }) })).json();
    if (stopped.status.running) throw new Error(`app still running after exit: ${JSON.stringify(stopped.status)}`);
    fs.mkdirSync(`${apps}/beta`);
    fs.writeFileSync(`${apps}/beta/app.json`, JSON.stringify({ id: "beta", name: "Beta", icon: "test", exec: "/usr/bin/true" }));
    const added = await render();
    if (added.ids.join(",") !== "alpha,beta") throw new Error(`IDs after add: ${added.ids}`);
    if (added.state.hash === initial.state.hash || added.state.lit === initial.state.lit) throw new Error("rescan after add did not update Guest canvas");
    fs.writeFileSync(`${shots}/registry-ui-alpha-beta.png`, added.png);
    fs.rmSync(`${apps}/alpha`, { recursive: true });
    const removed = await render();
    if (removed.ids.join(",") !== "beta") throw new Error(`IDs after remove: ${removed.ids}`);
    if (removed.state.hash === added.state.hash || removed.state.lit === added.state.lit) throw new Error("rescan after remove did not update Guest canvas");
    fs.writeFileSync(`${shots}/registry-ui-beta.png`, removed.png);
    if (errors.length) throw new Error(`guest errors: ${errors.join("; ")}`);
    console.log("UI launch -> app_started/app_exited -> returned; get_status; rescan alpha -> alpha,beta -> beta; no page errors");
  } finally { await browser.close(); }
})().catch((error) => { console.error(error); process.exitCode = 1; });
JS
