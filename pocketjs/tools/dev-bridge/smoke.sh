#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
scanner=${HEYA_REGISTRY_SCANNER:?set HEYA_REGISTRY_SCANNER to a compiled dump-apps binary}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
apps="$tmp/apps"
mkdir -p "$apps/alpha"
printf '{"id":"alpha","name":"Alpha","exec":"/usr/bin/true"}\n' > "$apps/alpha/app.json"
cd "$repo"
HEYA_ROOT="$repo" HEYA_APPS="$apps" HEYA_REGISTRY_SCANNER="$scanner" bun -e '
import { registryResponse } from "./pocketjs/tools/dev-bridge/server.ts";
const apps = JSON.parse(await (await registryResponse()).text());
if (apps.map((app) => app.id).join(",") !== "alpha") throw new Error(`initial scan: ${JSON.stringify(apps)}`);
console.log(`initial: ${apps.map((app) => app.id).join(",")}`);
await Bun.write(`${process.env.HEYA_APPS}/beta/app.json`, JSON.stringify({ id: "beta", name: "Beta", exec: "/usr/bin/true" }));
const added = JSON.parse(await (await registryResponse()).text());
if (added.map((app) => app.id).join(",") !== "alpha,beta") throw new Error(`after add: ${JSON.stringify(added)}`);
console.log(`after add: ${added.map((app) => app.id).join(",")}`);
await Bun.$`rm -r ${process.env.HEYA_APPS}/alpha`;
const removed = JSON.parse(await (await registryResponse()).text());
if (removed.map((app) => app.id).join(",") !== "beta") throw new Error(`after remove: ${JSON.stringify(removed)}`);
console.log(`after remove: ${removed.map((app) => app.id).join(",")}`);
'
