package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/iamcheyan/heya/internal/model"
)

// countAccentPixels renders a single row with an icon and reports how many
// pixels in the icon column carry the accent colour.
func countAccentPixels(t *testing.T, icon rune) int {
	t.Helper()
	os.Setenv("SDL_VIDEODRIVER", "dummy")
	theme := model.DefaultTheme()
	theme.Font = filepath.Join("..", "..", "assets", "JetBrainsMonoNerdFont-Regular.ttf")
	theme.FontCJK = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"

	screen, err := Open(Options{Theme: theme, Icons: model.Icons{}, Width: 720, Height: 480, Hidden: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer screen.Close()

	frame := Frame{
		Title:    "T",
		Status:   []string{"HEYA"},
		Rows:     []Row{{Label: "Clock", Icon: icon}},
		Selected: 0,
		Hint:     "hint",
	}
	if err := screen.Render(frame); err != nil {
		t.Fatalf("render: %v", err)
	}
	path := filepath.Join(t.TempDir(), "icon.png")
	if err := screen.SavePNG(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	accent, err := ParseColor(theme.Accent)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for y := 92; y < 140; y++ {
		for x := 60; x < 88; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == accent.R && uint8(g>>8) == accent.G && uint8(b>>8) == accent.B {
				count++
			}
		}
	}
	return count
}

func TestIconGlyphRendersPixels(t *testing.T) {
	withIcon := countAccentPixels(t, 0xF0954)
	withoutIcon := countAccentPixels(t, 0)
	if withIcon == 0 {
		t.Fatalf("icon glyph drew no accent pixels (baseline %d)", withoutIcon)
	}
	if withIcon <= withoutIcon {
		t.Fatalf("icon glyph added no pixels: with=%d without=%d", withIcon, withoutIcon)
	}
	_ = image.Rect
}
