#!/usr/bin/env python3
"""Compose the heya font-review screenshots into one contact sheet."""
from pathlib import Path
from PIL import Image, ImageDraw

ROOT = Path("build/font-review")
OUT = Path("docs/screenshots/fonts/review-contact-sheet.png")

# (folder, file, caption)
PANELS = [
    ("default", "01-apps.png", "Default theme - app list (CJK labels)"),
    ("light", "01-apps.png", "Light theme - app list (CJK labels)"),
    ("default", "font-sample-01.png", "Latin / Nerd Font"),
    ("default", "font-sample-02.png", "Simplified Chinese"),
    ("default", "font-sample-03.png", "Traditional Chinese"),
    ("default", "font-sample-04.png", "Japanese"),
    ("default", "font-sample-05.png", "Korean"),
    ("default", "font-sample-06.png", "Mixed CJK + Latin"),
    ("scale-8", "01-apps.png", "scale 8"),
    ("scale-32", "01-apps.png", "scale 32"),
    ("default", "02-system.png", "System page"),
    ("default", "03-about.png", "About page"),
]

CELL_W, CELL_H, PAD, LABEL = 360, 240+26, 10, 26
COLS = 3
ROWS = (len(PANELS) + COLS - 1) // COLS

sheet = Image.new("RGB", (COLS * (CELL_W + PAD) + PAD, ROWS * (CELL_H + PAD) + PAD), (34, 38, 46))
draw = ImageDraw.Draw(sheet)

for i, (folder, name, caption) in enumerate(PANELS):
    src = ROOT / folder / name
    if not src.exists():
        raise SystemExit(f"missing screenshot: {src} (run `make dump-fonts` first)")
    img = Image.open(src).convert("RGB")
    img.thumbnail((CELL_W, CELL_H - LABEL - 4), Image.Resampling.NEAREST)
    col, row = i % COLS, i // COLS
    x = PAD + col * (CELL_W + PAD)
    y = PAD + row * (CELL_H + PAD)
    sheet.paste(img, (x, y + LABEL))
    draw.text((x, y + 6), f"{folder}/{name}", fill=(240, 240, 240))
    draw.text((x, y + 18), caption, fill=(170, 180, 196))

OUT.parent.mkdir(parents=True, exist_ok=True)
sheet.save(OUT)
print(f"wrote {OUT} ({sheet.width}x{sheet.height})")
