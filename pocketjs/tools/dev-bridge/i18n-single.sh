#!/bin/sh
# 阶段5：单应用确定性渲染检测。逐个替换 Registry 目录内容，Guest 只显示一个应用（选中卡片），
# 检测名称区域是否有渲染的字形像素，判断是否缺字/tofu。
# 需要：POCKETJS_ROOT, CHROME_PATH, HEYA_BRIDGE_BIN, HEYA_REGISTRY_SCANNER
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
pocketjs=${POCKETJS_ROOT:?}
chrome=${CHROME_PATH:?}
bridge=${HEYA_BRIDGE_BIN:?}
scanner=${HEYA_REGISTRY_SCANNER:?}
port=${PORT:-8192}
bridge_port=${BRIDGE_PORT:-8193}
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
mkdir -p "$apps/app"
cp "$repo/pocketjs/src/heya-registry.ts" "$pocketjs/apps/heya-pocket/heya-registry.ts"
cp "$repo/pocketjs/src/heya-bridge.ts" "$pocketjs/apps/heya-pocket/heya-bridge.ts"
cp "$repo/pocketjs/heya-pocket-main.tsx" "$pocketjs/apps/heya-pocket/main.tsx"
cp "$repo/pocketjs/src/apps.generated.ts" "$pocketjs/apps/heya-pocket/apps.generated.ts"
cd "$pocketjs"
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국스어테트한HelloWorldcafénaïveÜnïcödé你好世界中文測試日本語こんにちは한국어테스트안녕하세요繁體🚀✅❤😀🎉" >/dev/null 2>&1
printf '{"id":"app","name":"Hello","icon":"test","exec":"/usr/bin/true"}\n' > "$apps/app/app.json"
"$bridge" --apps "$apps" --listen "127.0.0.1:$bridge_port" > "$tmp/bridge.log" 2>&1 &
bridge_pid=$!
HEYA_BRIDGE_BASE="http://127.0.0.1:$bridge_port" HEYA_ROOT="$repo" HEYA_APPS="$apps" HEYA_REGISTRY_SCANNER="$scanner" HEYA_REGISTRY_BRIDGE="$repo/pocketjs/tools/dev-bridge/server.ts" PORT="$port" ~/.bun/bin/bun tools/dev/web/serve.ts > "$tmp/server.log" 2>&1 &
server_pid=$!
node - "$port" "$repo/pocketjs/screenshots" "$chrome" "$apps" <<'JS'
const { chromium } = require("playwright");
const fs = require("node:fs");
const [port, shots, executablePath, appsDir] = process.argv.slice(2);
const base = `http://127.0.0.1:${port}`;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const samples = [
  ["ascii", "Hello World"],
  ["utf8", "café naïve Ünïcödé"],
  ["chinese", "你好世界 中文测试"],
  ["japanese", "日本語テスト こんにちは"],
  ["korean", "한국어 테스트 안녕하세요"],
  ["emoji", "😀🎉🚀✅❤️"],
  ["mixed", "Hello 你好 日本語 한국어 😀"],
  ["traditional", "繁體中文測試"],
];
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath, args: ["--no-sandbox"] });
  const errors = [];
  const out = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(`${base}/?demo=heya-pocket-main`, { waitUntil: "networkidle" });
    await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
    await sleep(900);
    for (const [label, name] of samples) {
      // 替换唯一应用名并触发 rescan（bridge 每次 rescan 重新扫描目录）
      fs.writeFileSync(`${appsDir}/app/app.json`, JSON.stringify({ id: "app", name, icon: "test", exec: "/usr/bin/true" }));
      const reply = await (await fetch(`${base}/heya/bridge/cmd`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ t: "rescan", id: 1 }) })).json();
      // 触发 Guest rescan：重新加载页面以确保 chart 读取最新 Registry
      await page.reload({ waitUntil: "networkidle" });
      await page.waitForFunction(() => document.querySelector("#log").textContent.includes("loaded heya-pocket-main"));
      await sleep(700);
      fs.writeFileSync(`${shots}/i18n-single-${label}.png`, await page.locator("canvas").screenshot());
      // 名称区域（480x272 canvas → 放大约 2x）：中心卡片文字带
      const probe = await page.evaluate(() => {
        const c = document.querySelector("canvas"), d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
        const W = c.width, H = c.height;
        let whitish = 0, colored = 0;
        const y0 = Math.floor(H * 0.42), y1 = Math.floor(H * 0.60);
        const x0 = Math.floor(W * 0.28), x1 = Math.floor(W * 0.72);
        for (let y = y0; y < y1; y++) for (let x = x0; x < x1; x++) {
          const i = (y * W + x) * 4, r = d[i], g = d[i+1], b = d[i+2];
          if (r > 170 && g > 170 && b > 170 && Math.max(r,g,b) - Math.min(r,g,b) < 40) whitish++;
          if (Math.max(r,g,b) - Math.min(r,g,b) > 60) colored++;
        }
        return { whitish, colored, W, H };
      });
      out.push({ label, name, ...probe });
    }
    console.log(JSON.stringify({ results: out, errors }, null, 1));
    if (errors.length) throw new Error(`guest errors: ${errors.join("; ")}`);
  } finally { await browser.close(); }
})().catch((e) => { console.error(e); process.exitCode = 1; });
JS
