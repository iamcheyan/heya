# heya × PocketJS prototype (CPR69)

This is an isolated feasibility experiment for comparing PocketJS components with heya's SDL2 renderer. The production SDL2 UI remains intact.

## PocketJS runtime

The PocketJS guest is `heya-pocket-main.tsx`. To build and run it, use a PocketJS 0.15.0 source checkout with Bun and Rust's `wasm32-unknown-unknown` target:

```sh
bun install --frozen-lockfile
bun tools/wasm.ts
mkdir -p apps/heya-pocket
cp heya-pocket-main.tsx apps/heya-pocket/main.tsx
cp apps.generated.ts apps/heya-pocket/apps.generated.ts
bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국스어테트한"
PORT=8130 bun tools/dev/web/serve.ts
```

Select `heya-pocket-main` in the browser host and click Run. The host maps arrows to D-pad, `X` to CROSS, `A` to SQUARE, `S` to TRIANGLE, Enter or `Z` to CIRCLE, Shift to SELECT and Space to START. The guest uses PocketJS `Focusable`, `FocusGrid`, `focus:` style variants and the native `spring()` API. Its data is real heya Registry output; launching an app is still a placeholder.

The repository's `pocket.json` records the app identity and 720×480 viewport. The public Linux app backend is not ready to package this guest as a ROCKNIX app; the current verified runtime is the official browser/WASM host at its stock 480×272 viewport.

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

- `pocketjs/tools/dump-apps/main.go` scans heya's own `internal/registry` and emits JSON; `src/apps.generated.ts` is the generated fixture.
- `src/heya-registry.ts` maps the existing heya app metadata shape to card data; no live Go-to-JavaScript bridge exists.
- heya Launcher, Registry, System, app scanner, metadata and SDL2 UI remain unchanged in this worktree.
- The guest does not start real heya apps yet. Native display/input integration, Registry/System IPC, launch/return behavior, performance measurement, and RG34XX verification remain open.

See [`../docs/pocketjs-evaluation.md`](../docs/pocketjs-evaluation.md) for the assessment and remaining CPR69 acceptance items.