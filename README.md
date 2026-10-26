# heya

A lightweight, JSON-driven launcher shell for ARM64 Linux handhelds
(ROCKNIX, Knulli, muOS; H700-class devices such as RG34XX / RG35XX / RG40XX).

Not a game frontend: games are just one kind of application. The shape we are aiming
for is Kindle + iPod + Linux launcher + Pocket OS.

- Go + SDL2, single binary, ARM64 cross-compiled
- JSON pages, JSON themes, Nerd Font glyph icons (no PNG assets)
- CJK text alongside the icon font, with a theme-configurable fallback face
- 720x480 landscape, integer scaling
- Apps: drop a directory under `/storage/heya/apps` with an `app.json`; no code change

## Build and run

```sh
make build                      # ./heya (amd64)
./heya --theme themes/default/theme.json --apps examples/apps
make dump                       # render every page to build/frames/*.png (no display needed)
make test                       # unit tests
make dump-fonts                 # CJK/Latin font review shots, both themes and scales
make check-fonts                # fail if any review shot rendered no text
```

Flags: `--apps`, `--theme`, `--icons`, `--input`, `--font`, `--font-cjk`, `--scale`,
`--width`, `--height`, `--dump DIR`, `--verbose`.

On a device the binary reads `/storage/heya/apps` and seeds
`/storage/heya/themes/theme.json` on first boot.

## Layout

| Path | Purpose |
|---|---|
| `cmd/heya` | bootstrap, flags, event loop, exec, frame dump |
| `internal/input` | SDL events → semantic actions; configurable key/pad mapping |
| `internal/ui` | renderer, layout, text cache, theme colours, font fallback |
| `internal/registry` | scan `apps/*/app.json` |
| `internal/launcher` | page stack, selection, commands |
| `internal/system` | battery, temperature, memory, storage, network, volume, brightness, ZRAM |
| `assets/` | icon map + JetBrainsMono Nerd Font (OFL) |
| `themes/` | `default` and `light` theme JSON |
| `examples/apps/` | sample applications |
| `examples/font-samples/` | one app per script, for font review |
| `integrations/rocknix/` | package.mk, systemd unit, `start_es.sh` patch |

## Documentation

- [PROJECT_PLAN.md](PROJECT_PLAN.md) — architecture, module boundaries, JSON protocol, milestones, risks
- [`docs/fonts.md`](docs/fonts.md) — Latin/CJK font fallback, theme fields, review images
- [`docs/rocknix-architecture.md`](docs/rocknix-architecture.md) — ROCKNIX boot chain, frontend entry points, game-launch contract
- [`integrations/rocknix/README.md`](integrations/rocknix/README.md) — installing heya as the boot frontend

## Screenshots

| Apps (default theme) | System | Light theme | About |
|---|---|---|---|
| ![apps](docs/screenshots/apps.png) | ![system](docs/screenshots/system.png) | ![light](docs/screenshots/apps-light-theme.png) | ![about](docs/screenshots/about.png) |

Rendered by `make dump` at 720x480 with the bundled Nerd Font. The app list
carries CJK labels, drawn through the fallback face. Dumps use a neutral demo
device, so no real hostname, address or memory layout is baked into the images.

Font review sheet — both themes, one specimen per script, plus scale 8 and 32:

![font review](docs/screenshots/fonts/review-contact-sheet.png)

Individual shots live in [`docs/screenshots/fonts/`](docs/screenshots/fonts/);
regenerate them with `make dump-fonts`.

## Status

Milestones M1–M5 are implemented and verified on a Linux PC (menu rendering, app
launching and return, JSON themes, app scanning, system page). M6 (replacing the
ROCKNIX frontend) ships as integration files; it needs an image build and a device
run to be accepted.
