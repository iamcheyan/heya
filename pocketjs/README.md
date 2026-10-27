# heya × PocketJS prototype (CPR69)

This is an isolated feasibility experiment for comparing PocketJS components with heya's SDL2 renderer. The production SDL2 UI remains intact.

## PocketJS runtime

The PocketJS guest is `heya-pocket-main.tsx`. To build and run it, use a PocketJS 0.15.0 source checkout with Bun and Rust's `wasm32-unknown-unknown` target:

```sh
bun install --frozen-lockfile
bun tools/wasm.ts
mkdir -p apps/heya-pocket
cp /path/to/heya-pocketjs-prototype/pocketjs/heya-pocket-main.tsx apps/heya-pocket/main.tsx
cp /path/to/heya-pocketjs-prototype/pocketjs/src/apps.generated.ts apps/heya-pocket/apps.generated.ts
cp /path/to/heya-pocketjs-prototype/pocketjs/src/heya-registry.ts apps/heya-pocket/heya-registry.ts
HEYA_REGISTRY_BRIDGE=/path/to/heya-pocketjs-prototype/pocketjs/tools/dev-bridge/server.ts HEYA_ROOT=/path/to/heya-pocketjs-prototype HEYA_APPS=/path/to/heya-pocketjs-prototype/examples/font-samples HEYA_REGISTRY_SCANNER=/path/to/dump-apps PORT=8130 bun tools/dev/web/serve.ts
```

Select `heya-pocket-main` in the browser host and click Run. The host maps arrows to D-pad, `X` to CROSS, `A` to SQUARE, `S` to TRIANGLE, Enter or `Z` to CIRCLE, Shift to SELECT and Space to START. The guest uses PocketJS `Focusable`, `FocusGrid`, `focus:` style variants, the native `spring()` API, and `net.fetch` for Registry refresh at startup and from the Quick Menu. In the PocketJS 0.15.0 checkout, the optional `HEYA_REGISTRY_BRIDGE` route and the `engine.js` `__heyaRegistryUrl` injection are prototype-only local host changes; they are not upstream PocketJS functionality.

The repository's `pocket.json` records the app identity and 720×480 viewport. The public Linux app backend is not ready to package this guest as a ROCKNIX app; the verified runtime remains the official browser/WASM host at its stock 480×272 viewport.

## Responsive design sandbox

The separate `src/sandbox.tsx` and `src/style.css` are for quick visual iteration. Start them with:

```sh
npm ci
npm run typecheck
npm run build
npm run dev
```

They are not PocketJS runtime evidence. The guest uses PocketJS's fixed set of style utilities and native drawing core; it does not load CSS.

## heya integration boundary

The `HEYA_REGISTRY_BRIDGE` route handles the browser prototype's `/heya/registry` through `pocketjs/tools/dump-apps`, scanning current visible apps on every request. The Guest also sends `launch(app_id)`, `rescan()` and `get_status()` through the JSON IPC sidecar and polls `app_started`, `app_exited`, `registry_updated`, `status_changed` events. The sidecar launches only apps found by heya Registry.
- The optional Go sidecar is `pocketjs/tools/bridge`; it preserves the existing Launcher/Registry/System packages and owns its own child-process lifecycle. It does not drive the current SDL `cmd/heya` event loop or call Launcher APIs, so this verifies the bridge contract but not in-process Launcher integration. See [`../docs/pocketjs-ipc.md`](../docs/pocketjs-ipc.md).
- Browser `/heya/bridge/*` proxy and the PocketJS `engine.js` `__heyaBridgeUrl` injection are local validation-host edits outside this repo. They are prototype-only, not upstream PocketJS functionality.
- Linux native host, integration with the production Launcher process, target-device performance, and RG34XX verification remain open.

Regenerate the checked-in fallback fixture from this checkout:

```sh
go run ./pocketjs/tools/dump-apps -dir examples/font-samples > /tmp/heya-apps.json
```

The generator emits JSON. Convert the JSON to the typed `apps.generated.ts` module only when intentionally updating the startup fallback.

See [`../docs/pocketjs-evaluation.md`](../docs/pocketjs-evaluation.md) for the assessment and remaining CPR69 acceptance items.