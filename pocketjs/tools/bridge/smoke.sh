#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
bridge=${HEYA_BRIDGE_BIN:?set HEYA_BRIDGE_BIN to a compiled bridge binary}
port=${PORT:-8152}
tmp=$(mktemp -d)
server_pid=
cleanup() { if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi; rm -rf "$tmp"; }
trap cleanup EXIT HUP INT TERM
apps="$tmp/apps"
mkdir -p "$apps/sleeper"
printf '{"id":"sleeper","name":"Sleeper","icon":"test","exec":"/bin/sh","args":["-c","test \"$BRIDGE_TEST\" = active && sleep 0.4"],"environment":["BRIDGE_TEST=active"]}\n' > "$apps/sleeper/app.json"
"$bridge" --apps "$apps" --listen "127.0.0.1:$port" > "$tmp/bridge.log" 2>&1 &
server_pid=$!
node - "$port" <<'JS'
const assert = require("node:assert/strict");
const port = process.argv[2];
const base = `http://127.0.0.1:${port}`;
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function command(t, id, app_id) {
  const response = await fetch(`${base}/cmd`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ t, id, app_id }) });
  return { status: response.status, body: await response.json() };
}
(async () => {
  for (let i = 0; i < 50; i++) { try { if ((await fetch(`${base}/events?since=0`)).ok) break; } catch {} await wait(20); }
  const launch = await command("launch", 1, "sleeper");
  assert.equal(launch.status, 200); assert.equal(launch.body.id, 1); assert.equal(launch.body.status.running, true); assert.equal(launch.body.status.app_id, "sleeper");
  const conflict = await command("launch", 2, "sleeper"); assert.equal(conflict.status, 409); assert.equal(conflict.body.id, 2);
  const status = await command("get_status", 3); assert.equal(status.body.status.running, true);
  await wait(650);
  const stopped = await command("get_status", 4); assert.equal(stopped.body.status.running, false);
  const eventResponse = await fetch(`${base}/events?since=0`); const batch = await eventResponse.json();
  assert.deepEqual(batch.events.map((e) => e.type), ["app_started", "status_changed", "app_exited", "status_changed"]);
  const rescan = await command("rescan", 5); assert.equal(rescan.body.apps[0].id, "sleeper");
  const later = await (await fetch(`${base}/events?since=${batch.next}`)).json(); assert.deepEqual(later.events.map((e) => e.type), ["registry_updated"]);
  const missing = await command("launch", 6, "missing"); assert.equal(missing.status, 404);
  console.log("launch -> running status -> app_exited -> stopped status -> rescan; IDs and ordered events verified");
})().catch((error) => { console.error(error); process.exitCode = 1; });
JS
