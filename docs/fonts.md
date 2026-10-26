# Fonts: CJK alongside the Nerd Font UI

heya renders Latin text and Nerd Font icons from the primary face, and falls
back to a CJK face for Hanzi, Kana, Hangul and other non-Latin runs. Both faces
come from the theme, so no recompile is needed to change either one.

## Theme fields

```json
{
  "font": "assets/JetBrainsMonoNerdFont-Regular.ttf",
  "font_cjk": "/usr/share/fonts/truetype/noto-cjk/NotoSansCJKsc-Regular.otf",
  "scale": 16
}
```

- `font` — primary face. Owns Latin text and every Nerd Font glyph.
- `font_cjk` — fallback for non-Latin runs. Optional; when the file is missing,
  heya still boots and only loses non-Latin coverage.
- `scale` — 8, 16 or 32. Anything else falls back to 16.

A relative font path is tried as-is, then against the primary font's directory,
`assets/`, and `/usr/share/heya/assets/`. An unset or unreadable `font_cjk`
falls through to the system candidates in `internal/ui.DefaultCJKFontCandidates`
(ROCKNIX's Noto Sans CJK SC, Debian's Noto Sans CJK TTC, WenQuanYi).

## How mixed text is drawn

`internal/ui.renderText` splits a string into runs by script: Latin runs go to
the primary font, CJK runs to the fallback. Each run is rendered with
`SDL_ttf` and copied into one ARGB8888 canvas.

The canvas is the important part. An earlier version composed the runs with the
wrong blit direction, which produced a correctly-sized surface carrying no
pixels — CJK strings rendered as blank space with no error. Two guards keep it
from coming back:

- `TestCJKCompositesVisibleInk` counts opaque pixels in the composed surface.
- `scripts/check_font_shots.py` counts non-chrome pixels in every exported
  screenshot, with the floor scaled by font size.

Per-run rendering also means the composite is measured from the runs
themselves, so a mixed string keeps its own width instead of borrowing metrics
from one face.

## Regenerating the review images

```sh
make dump-fonts     # both themes + scale 8/16/32 into build/font-review
make check-fonts    # fail if any screenshot has no rendered text
```

`docs/screenshots/fonts/review-contact-sheet.png` is the single-page review
grid; `scripts/make_font_contact_sheet.py` builds it from the same folder.

## Status

| Item | State |
|---|---|
| Latin + Nerd Font icons on the primary face | verified on Linux |
| Simplified, Traditional, Japanese, Korean specimens | verified in screenshots |
| Missing CJK font leaves the launcher usable | verified (candidate fallback) |
| `--font` / `--font-cjk` / `--scale` overrides | verified |
| Device check on ROCKNIX image | not done |

The bundled font is JetBrains Mono Nerd Font (OFL, `assets/`). CJK coverage is
not bundled: the ROCKNIX image already installs `noto-sans-cjk` at
`/usr/share/fonts/truetype/noto-cjk/NotoSansCJKsc-Regular.otf`, and heya uses
that path rather than shipping another ~19 MB face.

A seeded `/storage/heya/themes/theme.json` is only written when absent, so a
user's own theme is never overwritten by an update.
