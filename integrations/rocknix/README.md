# ROCKNIX integration

Files here install heya as the boot frontend on a ROCKNIX tree (fork or upstream).

| File | Purpose |
|---|---|
| `package.mk` | Builds heya with the ROCKNIX Go host toolchain and installs binary, assets, theme and unit |
| `heya.service` | systemd unit: `Requires=sway.service`, `Conflicts=essway.service` |
| `start_es.sh.patch` | Makes `/usr/bin/start_es.sh` exec heya before the bundled launcher and ES |

## Install into a ROCKNIX tree

```sh
# 1. package
mkdir -p projects/ROCKNIX/packages/apps/heya
cp integrations/rocknix/package.mk projects/ROCKNIX/packages/apps/heya/package.mk
# package.mk reads assets/ and themes/ from PKG_DIR, so the package directory
# needs them too:
cp -r assets themes projects/ROCKNIX/packages/apps/heya/
cp -r integrations projects/ROCKNIX/packages/apps/heya/

# 2. frontend handoff
patch -p1 < integrations/rocknix/start_es.sh.patch

# 3. build
make docker-H700
```

## Runtime layout

| Path | Content |
|---|---|
| `/usr/bin/heya` | launcher binary |
| `/usr/share/heya/assets/` | icon map + Nerd Font |
| `/usr/share/heya/themes/default/theme.json` | default theme |
| `/storage/heya/apps/<app>/app.json` | user applications (created on first boot) |
| `/storage/heya/themes/theme.json` | active theme (seeded on first boot) |

heya writes only under `/storage/heya`; it never touches `/storage/.emulationstation`.

## Game launching

heya does not implement game launching itself. To keep the existing contract, an
app entry can call the same helper EmulationStation uses:

```json
{
  "name": "PlayStation",
  "icon": "nf-md-gamepad",
  "exec": "/usr/bin/run",
  "args": ["/storage/roms/psx/Game.chd", "-P", "psx"]
}
```

`runemu.sh` keeps owning performance mode, environment, per-game settings and the
return-to-frontend behaviour.

## Rollback

Remove the `heya` package from the image (or delete `/usr/bin/heya`); `start_es.sh`
falls back to `handheld-launcher`, then to EmulationStation.
