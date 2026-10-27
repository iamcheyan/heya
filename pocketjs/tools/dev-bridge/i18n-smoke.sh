#!/bin/sh
# 阶段5国际化实测：在 PocketJS 浏览器 host 上渲染各类字符集并验证。
# 需要：POCKETJS_ROOT, CHROME_PATH, HEYA_BRIDGE_BIN, HEYA_REGISTRY_SCANNER
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
pocketjs=${POCKETJS_ROOT:?set POCKETJS_ROOT}
chrome=${CHROME_PATH:?set CHROME_PATH}
bridge=${HEYA_BRIDGE_BIN:?set HEYA_BRIDGE_BIN}
scanner=${HEYA_REGISTRY_SCANNER:?set HEYA_REGISTRY_SCANNER}
port=${PORT:-8190}
bridge_port=${BRIDGE_PORT:-8191}
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
mk() { mkdir -p "$apps/$1"; printf '{"id":"%s","name":"%s","icon":"test","exec":"/usr/bin/true"}\n' "$1" "$2" > "$apps/$1/app.json"; }
mk ascii "Hello World"
mk utf8 "café naïve Ünïcödé"
mk chinese "你好世界 中文测试"
mk japanese "日本語テスト こんにちは"
mk korean "한국어 테스트 안녕하세요"
mk emoji "😀🎉🚀✅❤️"
mk mixed "Hello 你好 日本語 한국어 😀"
mk traditional "繁體中文測試"
cp "$repo/pocketjs/src/heya-registry.ts" "$pocketjs/apps/heya-pocket/heya-registry.ts"
cp "$repo/pocketjs/src/heya-bridge.ts" "$pocketjs/apps/heya-pocket/heya-bridge.ts"
cp "$repo/pocketjs/heya-pocket-main.tsx" "$pocketjs/apps/heya-pocket/main.tsx"
cp "$repo/pocketjs/src/apps.generated.ts" "$pocketjs/apps/heya-pocket/apps.generated.ts"
cd "$pocketjs"
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국스어테트한HelloWorldcafénaïveÜnïcödé你好世界中文測試日本語こんにちは한국어테스트안녕하세요繁體🚀✅❤😀🎉" >/dev/null 2>&1
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
  const errors = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`${base}/?demo=heya-pocket-main`, { waitUntil: "networkidle" });
    await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
    await sleep(1000);
    // 逐个应用：按 RIGHT 遍历 8 个卡片，每个截图
    const results = [];
    for (let i = 0; i < 8; i++) {
      await page.keyboard.down("ArrowRight"); await sleep(150); await page.keyboard.up("ArrowRight"); await sleep(450);
      await page.waitForTimeout(200);
      fs.writeFileSync(`${shots}/i18n-card-${i}.png`, await page.locator("canvas").screenshot());
      // 统计该帧非背景像素（粗略判断有字形渲染）
      const lit = await page.evaluate(() => {
        const c = document.querySelector("canvas"), d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
        let n = 0; for (let i = 0; i < d.length; i += 4) if (d[i] > 150 && d[i+1] > 150 && d[i+2] > 150) n++;
        return n;
      });
      results.push({ card: i, litPixels: lit });
    }
    const registry = await (await fetch(`${base}/heya/bridge/cmd`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ t: "rescan", id: 99 }) })).json();
    console.log(JSON.stringify({
      appCount: registry.apps?.length,
      appNames: registry.apps?.map((a) => a.name),
      cards: results,
      errors,
    }, null, 1));
    if (errors.length) throw new Error(`guest errors: ${errors.join("; ")}`);
  } finally { await browser.close(); }
})().catch((e) => { console.error(e); process.exitCode = 1; });
JS
