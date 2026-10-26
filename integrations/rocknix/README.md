# ROCKNIX integration

The authoritative package recipe, heya-control helper, first-boot app seeder and
H700 frontend quirk live in [iamcheyan/rocknix](https://github.com/iamcheyan/rocknix/tree/heya-core-minimal/projects/ROCKNIX/packages/apps/heya).
ROCKNIX downloads a pinned heya source archive and builds it with its ARM64 Go/SDL2
toolchain. The local desktop preview builds the sibling heya checkout instead.

The boot sequence is rocknix-autostart → H700 quirks → app seeder → sway + heya.
Do not enable heya independently before the user configuration has been seeded.
The unit here mirrors the package unit; no global start_es.sh patch is needed.

Applications live in /storage/heya/apps/<id>/app.json. The active JSON theme is
/storage/heya/themes/theme.json. Existing user manifests and themes are preserved.
Wi-Fi profiles are saved by NetworkManager, Bluetooth bonds by BlueZ, and basic
controls persist through the ROCKNIX settings helper.

See the ROCKNIX device checklist for HX90 builds and hardware validation. Source
and desktop tests do not establish that a device image boots or plays audio.
