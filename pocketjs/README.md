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

- The browser prototype's `/heya/registry` endpoint runs `pocketjs/tools/dump-apps` on every request. It scans the configured app directory through `internal/registry.Registry.Scan()` and returns current visible apps as JSON.
- `src/apps.generated.ts` remains the startup fallback if the runtime endpoint is unavailable; the menu's Rescan action replaces the cards with the latest Registry response.
- Run the isolated browser bridge with `HEYA_ROOT=/absolute/path/to/heya HEYA_APPS=/absolute/path/to/apps PORT=8130 bun tools/dev/web/serve.ts` from the PocketJS 0.15.0 checkout after adding the route documented in this CPR72 branch. The route is prototype-only and is not part of PocketJS upstream.
- The guest does not start real heya apps. Launch/return behavior, status IPC, native host integration, performance measurement, and RG34XX verification remain open.

Regenerate the checked-in fallback fixture from this checkout:

```sh
go run ./pocketjs/tools/dump-apps -dir examples/font-samples > /tmp/heya-apps.json
```

The generator emits JSON. Convert the JSON to the typed `apps.generated.ts` module only when intentionally updating the startup fallback.

See [`../docs/pocketjs-evaluation.md`](../docs/pocketjs-evaluation.md) for the assessment and remaining CPR69 acceptance items.