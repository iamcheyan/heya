# heya

A lightweight, JSON-driven launcher shell for ARM64 Linux handhelds
(ROCKNIX, Knulli, muOS; H700-class devices such as RG34XX / RG35XX / RG40XX).

Not a game frontend: games are just one kind of application. The shape we are aiming
for is Kindle + iPod + Linux launcher + Pocket OS.

- Implementation: Go + SDL2, single binary, ARM64 cross-compiled
- UI: JSON-described pages and themes, Nerd Font glyph icons (no PNG assets)
- Display: 720x480 landscape, integer scaling
- Apps: drop a directory under `/storage/heya/apps` with an `app.json`; no code change

## Status

Phase 1 — architecture only. See:

- [`PROJECT_PLAN.md`](PROJECT_PLAN.md) — architecture, module boundaries, directory
  layout, JSON protocol, milestones, risks
- [`docs/rocknix-architecture.md`](docs/rocknix-architecture.md) — ROCKNIX boot chain,
  frontend entry points, config paths, game-launch contract

Coding starts at milestone M1 (menu rendering) once the architecture is agreed.
