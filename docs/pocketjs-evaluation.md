# CPR69: PocketJS feasibility prototype report

Status: PocketJS guest builds and renders through the official PocketJS browser host. This is a framework feasibility prototype, not a replacement of heya's SDL2 frontend; Registry/Launcher integration and target-device acceptance remain open.

## Implemented in this branch

- Branch: `pocketjs-prototype`, in isolated worktree `~/development/heya-pocketjs-prototype` based on the fetched `iamcheyan/heya` `origin/master` at `bca7c0510ac6c857b55c9532b9165dd332b8477c`.
- `pocketjs/heya-pocket-main.tsx`: PocketJS Solid guest with status row, six-app carousel, `Focusable` cards, one-column `FocusGrid`, D-pad/button handlers, `focus:` style variants, a native `spring()` call, quick menu with focus scope, favorite control and compact action footer.
- `pocketjs/src/sandbox.tsx` and `pocketjs/src/style.css`: responsive design-only Vite sandbox for visual exploration; explicitly separate from PocketJS runtime evidence.
- `pocketjs/tools/dump-apps/main.go`: scans heya's own `internal/registry` and emits JSON; `pocketjs/src/apps.generated.ts` is the generated fixture.
- `pocketjs/src/heya-registry.ts`: maps heya app metadata into the card model.
- `pocketjs/README.md`: setup and integration boundary.
- `docs/pocketjs-evaluation.md`: SDL2/PocketJS comparison, current framework limitations, recommendation and remaining acceptance work.

Core Go Launcher, Registry, System, model, scanner, metadata and SDL2 code are unchanged. The main checkout's existing modified `cmd/heya/main.go` and `internal/ui/ui.go` were left untouched.

## PocketJS runtime verification

PocketJS is a native component runtime, not a DOM/CSS framework. The guest uses PocketJS host components and style literals; CSS belongs only to the comparison sandbox. Built with PocketJS 0.15.0 and the official compiler:

```sh
~/.bun/bin/bun tools/wasm.ts
~/.bun/bin/bun tools/build.ts apps/heya-pocket/main.tsx --extra-chars="ステト世中你好文日本界繁語體국스어테트한"
PORT=8130 ~/.bun/bin/bun tools/dev/web/serve.ts
```

Observed build result: 27 compiled style records, 8 `.pak` entries, guest JS 188,230 bytes, `.pak` 306,176 bytes; no unknown-style warnings. The browser host loaded `heya-pocket-main`; the canvas was 480×272, reported 3.6 MB runtime memory, and visibly rendered the status row, carousel, selected card, caption and all four compact action controls. A screenshot was saved outside the repo under `~/.hermes/cache/scratch/heya-pocketjs-review/pocketjs-cjk2.png`. `npx tsc --noEmit` passes for the guest source.

The host chrome displays an emulator keyboard map. In that map, arrows are D-pad, X sends CROSS, A sends SQUARE, S sends TRIANGLE, Enter/Z sends CIRCLE. These are emulator controls, not heya's final controller map.

## Input verification

Held-key pixel hashes showed every control path changing the canvas: `ArrowRight` 589839760→410683622, `KeyX`→3916112886, `KeyS`→160249272, `KeyA`→2478779282, `Enter`→3471984398. Single-frame presses were dropped by the host's per-frame sampling, so the prototype uses held keys for control verification.

## Registry data

`pocketjs/tools/dump-apps/main.go` scans heya's own `internal/registry` and emits JSON; `pocketjs/src/apps.generated.ts` is the generated fixture. A rebuild after changing example apps:

```sh
cd ~/development/heya-pocketjs-prototype
go run ./pocketjs/tools/dump-apps -dir examples/font-samples > pocketjs/src/apps.generated.ts
```

The guest currently renders six apps from `examples/font-samples`, including CJK names such as `你好世界 Hello` and `日本語テスト Hello`. The Noto Sans CJK fallback face is declared in `apps/heya-pocket/fonts.json`; CJK glyphs render as real characters rather than tofu boxes.

## What the experiment says

- PocketJS offers reusable Solid components, native focus traversal, compiled focus variants and motion APIs; layout composition is concise once the guest fits PocketJS's fixed utility/style contract.
- The 480×272 stock browser-host screen can render this interface. The target device is 720×480, so this does not establish target resolution behavior, ARM64 performance, a Linux host backend or ROCKNIX integration.
- PocketJS's documented Linux profile does not currently provide the complete stock backend needed to ship a Linux/ROCKNIX app. The production migration boundary would need PocketJS embedded as a native Linux UI, a JSON/status IPC contract to the existing Go code, and a real heya launch/return bridge.
- The prototype shows the visual/interaction direction and consumes real heya Registry output, but does not execute apps.

## Recommendation

Keep the SDL2 frontend for RG34XX. PocketJS is promising as a component/focus/animation model, but the runtime experiment does not establish that it is ready to replace SDL2 on ROCKNIX. Revisit after a native Linux host/backend and a small Go bridge are demonstrated against the existing Launcher, Registry and System interfaces.

## Remaining before closing issue 69

- Replace mock cards with real Registry data through a documented bridge or fixture generated by the existing scanner.
- Connect launch, return, rescan and status requests to the current Go Launcher/System without modifying their core logic.
- Verify the carousel focus and spring interaction with the official browser-host input path and capture arrow/cross/square/triangle behavior.
- Design and build the native Linux/SDL2 host boundary; measure RAM and frame rate on ARM64.
- Test at 720×480 and then on RG34XX/ROCKNIX; save maintained screenshots only if they are intended project assets.
- Update the evaluation report from actual host/device evidence, then comment on and close CPR69 after all criteria are met.

## Sources

- PocketJS package: https://www.npmjs.com/package/@pocketjs/framework (0.15.0)
- PocketJS docs: https://pocketjs.pocket.nexus/docs/overview/ , https://pocketjs.pocket.nexus/docs/platform-contracts/ , https://pocketjs.pocket.nexus/docs/animation/ , https://pocketjs.pocket.nexus/docs/input-focus/
- heya source: `../README.md`, `../docs/application-manifest-spec.md`, `../internal/model/model.go`, `../internal/launcher/launcher.go`, `../internal/registry/registry.go`, `../internal/system/system.go`