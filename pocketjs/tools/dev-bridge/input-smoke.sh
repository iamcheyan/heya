#!/bin/sh
# 阶段4输入系统实测：在 PocketJS 浏览器 host 上逐键验证焦点与菜单行为。
# 需要：POCKETJS_ROOT, CHROME_PATH, HEYA_BRIDGE_BIN, HEYA_REGISTRY_SCANNER
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
pocketjs=${POCKETJS_ROOT:?set POCKETJS_ROOT}
chrome=${CHROME_PATH:?set CHROME_PATH}
bridge=${HEYA_BRIDGE_BIN:?set HEYA_BRIDGE_BIN}
scanner=${HEYA_REGISTRY_SCANNER:?set HEYA_REGISTRY_SCANNER}
port=${PORT:-8180}
bridge_port=${BRIDGE_PORT:-8181}
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
mkdir -p "$apps/alpha" "$apps/beta" "$apps/gamma"
printf '{"id":"alpha","name":"Alpha","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/alpha/app.json"
printf '{"id":"beta","name":"Beta","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/beta/app.json"
printf '{"id":"gamma","name":"Gamma","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/gamma/app.json"
cp "$repo/pocketjs/src/heya-registry.ts" "$pocketjs/apps/heya-pocket/heya-registry.ts"
cp "$repo/pocketjs/src/heya-bridge.ts" "$pocketjs/apps/heya-pocket/heya-bridge.ts"
cp "$repo/pocketjs/heya-pocket-main.tsx" "$pocketjs/apps/heya-pocket/main.tsx"
cp "$repo/pocketjs/src/apps.generated.ts" "$pocketjs/apps/heya-pocket/apps.generated.ts"
cd "$pocketjs"
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국ス어테트한" >/dev/null 2>&1
"$bridge" --apps "$apps" --listen "127.0.0.1:$bridge_port" > "$tmp/bridge.log" 2>&1 &
bridge_pid=$!
HEYA_BRIDGE_BASE="http://127.0.0.1:$bridge_port" HEYA_ROOT="$repo" HEYA_APPS="$apps" HEYA_REGISTRY_SCANNER="$scanner" HEYA_REGISTRY_BRIDGE="$repo/pocketjs/tools/dev-bridge/server.ts" PORT="$port" ~/.bun/bin/bun tools/dev/web/serve.ts > "$tmp/server.log" 2>&1 &
server_pid=$!
node - "$port" "$repo/pocketjs/screenshots" "$chrome" <<'JS'
const { chromium } = require("playwright");
const fs = require("node:fs");
const [port, shots, executablePath] = process.argv.slice(2);
const base = `http://127.0.0.1:${port}`;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath, args: ["--no-sandbox"] });
  const results = [];
  const errors = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`${base}/?demo=heya-pocket-main`, { waitUntil: "networkidle" });
    await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
    await sleep(800);
    const hash = () => page.evaluate(() => {
      const c = document.querySelector("canvas"), d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
      let h = 5381; for (const v of d) h = ((h * 33) ^ v) >>> 0; return h;
    });
    const shot = async (name) => { fs.writeFileSync(`${shots}/${name}.png`, await page.locator("canvas").screenshot()); };
    // 基线
    const base0 = await hash(); await shot("input-00-base");
    // 逐个按键测试（浏览器 host KEYMAP）
    const keys = [
      ["ArrowRight", "RIGHT"], ["ArrowLeft", "LEFT"], ["ArrowUp", "UP"], ["ArrowDown", "DOWN"],
      ["KeyX", "CROSS(A)"], ["KeyZ", "CIRCLE(B)"], ["KeyA", "SQUARE(X)"], ["KeyS", "TRIANGLE(Y)"],
      ["Space", "START"], ["Shift", "SELECT"], ["KeyQ", "L"], ["KeyE", "R"],
    ];
    for (const [key, label] of keys) {
      const before = await hash();
      await page.keyboard.down(key); await sleep(160); await page.keyboard.up(key); await sleep(450);
      const after = await hash();
      results.push({ key, label, changed: before !== after });
      await shot(`input-${key.replace(/[^a-zA-Z0-9]/g, "")}-${label.replace(/[^a-zA-Z]/g, "")}`);
    }
    // 长按（held）测试：CROSS 按住 1.2s
    const beforeHold = await hash();
    await page.keyboard.down("KeyX"); await sleep(1200); const duringHold = await hash();
    await page.keyboard.up("KeyX"); await sleep(400); const afterHold = await hash();
    await shot("input-hold-cross");
    // 连按测试：SQUARE 快速按 5 次（菜单开关）
    let toggles = 0; let prev = await hash();
    for (let i = 0; i < 5; i++) {
      await page.keyboard.down("KeyA"); await sleep(120); await page.keyboard.up("KeyA"); await sleep(300);
      const now = await hash(); if (now !== prev) toggles++; prev = now;
    }
    await shot("input-repeat-square");
    // 焦点切换后菜单：SQUARE 开菜单 -> ArrowDown 移动 -> CIRCLE 关闭
    await page.keyboard.down("KeyA"); await sleep(160); await page.keyboard.up("KeyA"); await sleep(400);
    const menuOpen = await hash(); await shot("input-menu-open");
    await page.keyboard.down("ArrowDown"); await sleep(160); await page.keyboard.up("ArrowDown"); await sleep(350);
    const menuMove = await hash(); await shot("input-menu-focus");
    await page.keyboard.down("KeyZ"); await sleep(160); await page.keyboard.up("KeyZ"); await sleep(400);
    const menuClosed = await hash(); await shot("input-menu-closed");
    console.log(JSON.stringify({
      keys: results,
      hold: { changedDuring: duringHold !== beforeHold, changedAfter: afterHold !== beforeHold },
      repeatToggles: toggles,
      menu: { opened: menuOpen !== base0, moved: menuMove !== menuOpen, closed: menuClosed !== menuMove },
      errors,
    }, null, 1));
    if (errors.length) throw new Error(`guest errors: ${errors.join("; ")}`);
  } finally { await browser.close(); }
})().catch((e) => { console.error(e); process.exitCode = 1; });
JS
