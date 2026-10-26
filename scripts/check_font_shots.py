#!/usr/bin/env python3
"""Verify heya font-review screenshots contain rendered text.

A screenshot can be the right size and still be empty: the mixed CJK/Latin
composition path once produced a correctly-sized but fully transparent surface.
Ink floors scale with the font size that produced each frame.
"""
import re
import sys
from pathlib import Path
from PIL import Image

# Page backgrounds, status/hint bars and the selection fill are chrome, not ink.
CHROME = {
    "dark": {(20, 24, 32), (14, 18, 24), (32, 38, 50), (0, 120, 215)},
    "light": {(245, 242, 234), (228, 232, 238), (216, 223, 232), (0, 120, 215)},
}

# Ink floor for a 16px frame; other scales are proportional.
MIN_INK = {
    "font-sample": 300,
    "01-apps": 3000,
    "02-system": 3000,
    "03-about": 2000,
}


def theme_of(path: Path) -> str:
    return "light" if "light" in str(path).lower() else "dark"


def scale_of(path: Path, root: Path) -> int:
    m = re.search(r"scale-(\d+)", str(path.relative_to(root)))
    return int(m.group(1)) if m else 16


def ink(path: Path, top: int, bottom: int) -> int:
    """Count pixels that are not page chrome in [top, bottom)."""
    im = Image.open(path).convert("RGB")
    try:
        band = im.crop((0, top, im.width, min(bottom, im.height)))
        raw = band.tobytes()
        chrome = CHROME[theme_of(path)]
        return sum(1 for i in range(0, len(raw), 3)
                   if (raw[i], raw[i + 1], raw[i + 2]) not in chrome)
    finally:
        im.close()


def main(root: Path) -> int:
    bad = []
    for path in sorted(root.rglob("*.png")):
        kind = next((k for k in MIN_INK if path.name.startswith(k)), None)
        if kind is None:
            continue
        im = Image.open(path)
        width, height = im.size
        im.close()
        need = MIN_INK[kind] * scale_of(path, root) // 16
        # A specimen is drawn in the title band, so measure the upper half; that
        # keeps stray chrome lower on the page from satisfying the check.
        bottom = height // 2 if kind == "font-sample" else height
        got = ink(path, 0, bottom)
        status = "ok" if got >= need else "FAIL"
        print(f"{status}\t{path.relative_to(root)}\tink={got}\tneed={need}\t{width}x{height}")
        if got < need:
            bad.append(path)
    if bad:
        print(f"\n{len(bad)} screenshot(s) look empty", file=sys.stderr)
        return 1
    print("\nall screenshots contain rendered text")
    return 0


if __name__ == "__main__":
    sys.exit(main(Path(sys.argv[1] if len(sys.argv) > 1 else "build/font-review")))
