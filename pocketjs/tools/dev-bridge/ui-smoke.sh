#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
scanner=${HEYA_REGISTRY_SCANNER:?set HEYA_REGISTRY_SCANNER to a compiled dump-apps binary}
pocketjs=${POCKETJS_ROOT:?set POCKETJS_ROOT to the PocketJS 0.15.0 checkout}
port=${PORT:-8142}
tmp=$(mktemp -d)
server_pid=
cleanup() {
  if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi
  rm -rf "$tmp"
}
trap cleanup EXIT HUP INT TERM
apps="$tmp/apps"
mkdir -p "$apps/alpha"
printf '{"id":"alpha","name":"Alpha","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/alpha/app.json"
cp "$repo/pocketjs/src/heya-registry.ts" "$pocketjs/apps/heya-pocket/heya-registry.ts"
cp "$repo/pocketjs/heya-pocket-main.tsx" "$pocketjs/apps/heya-pocket/main.tsx"
cp "$repo/pocketjs/src/apps.generated.ts" "$pocketjs/apps/heya-pocket/apps.generated.ts"
cd "$pocketjs"
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국스어테트한"
HEYA_ROOT="$repo" HEYA_APPS="$apps" HEYA_REGISTRY_SCANNER="$scanner" HEYA_REGISTRY_BRIDGE="$repo/pocketjs/tools/dev-bridge/server.ts" PORT="$port" ~/.bun/bin/bun tools/dev/web/serve.ts > "$tmp/server.log" 2>&1 &
server_pid=$!
node - "$port" "$apps" "$repo/pocketjs/screenshots" <<'JS'
const { chromium } = require("playwright");
const fs = require("node:fs");
const port = process.argv[2];
const apps = process.argv[3];
const shots = process.argv[4];
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const render = async () => {
      const registryRequest = page.waitForResponse((response) => response.url().endsWith("/heya/registry"));
      await page.goto(`http://127.0.0.1:${port}/?demo=heya-pocket-main`, { waitUntil: "networkidle" });
      await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
      const response = await registryRequest;
      if (!response.ok()) throw new Error(`Guest Registry HTTP ${response.status()}`);
      const ids = (await response.json()).map((app) => app.id);
      const state = await page.evaluate(() => {
        const canvas = document.querySelector("canvas"), data = canvas.getContext("2d").getImageData(0, 0, canvas.width, canvas.height).data;
        let hash = 5381, lit = 0;
        for (let i = 0; i < data.length; i++) { hash = ((hash * 33) ^ data[i]) >>> 0; if (i % 4 === 0 && data[i] > 160) lit++; }
        return { hash, lit };
      });
      await page.waitForTimeout(500);
      return { ids, state, png: await page.locator("canvas").screenshot() };
    };
    const initial = await render();
    if (initial.ids.join(",") !== "alpha") throw new Error(`initial Registry ids: ${initial.ids}`);
    if (!initial.state.lit) throw new Error("initial Registry Guest canvas is blank");
    fs.writeFileSync(`${shots}/registry-ui-alpha.png`, initial.png);
    fs.mkdirSync(`${apps}/beta`);
    fs.writeFileSync(`${apps}/beta/app.json`, JSON.stringify({ id: "beta", name: "Beta", icon: "test", exec: "/usr/bin/true" }));
    const added = await render();
    if (added.ids.join(",") !== "alpha,beta") throw new Error(`Registry ids after add: ${added.ids}`);
    if (added.state.hash === initial.state.hash || added.state.lit === initial.state.lit) throw new Error("adding beta did not change rendered canvas content");
    fs.writeFileSync(`${shots}/registry-ui-alpha-beta.png`, added.png);
    fs.rmSync(`${apps}/alpha`, { recursive: true });
    const removed = await render();
    if (removed.ids.join(",") !== "beta") throw new Error(`Registry ids after remove: ${removed.ids}`);
    if (removed.state.hash === added.state.hash || removed.state.lit === added.state.lit) throw new Error("removing alpha did not change rendered canvas content");
    fs.writeFileSync(`${shots}/registry-ui-beta.png`, removed.png);
    if (errors.length) throw new Error(`guest errors: ${errors.join("; ")}`);
    console.log("initial: alpha; after add: alpha,beta; after remove: beta; host HTTP, nonblank canvas and state changes verified; no page errors");
  } finally { await browser.close(); }
})().catch((error) => { console.error(error); process.exitCode = 1; });
JS
