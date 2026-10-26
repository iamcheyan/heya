package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/iamcheyan/heya/internal/model"
)

func TestHandheldMenuUsesLegacyGeometry(t *testing.T) {
	t.Setenv("SDL_VIDEODRIVER", "dummy")
	theme := model.DefaultTheme()
	theme.Font = filepath.Join("..", "..", "assets", "JetBrainsMonoNerdFont-Regular.ttf")
	screen, err := Open(Options{Theme: theme, Width: 720, Height: 480, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	defer screen.Close()
	frame := Frame{Title: "Applications", Status: []string{"HEYA", "WIFI: ON", "BT: OFF", "BAT: 82%"},
		Rows: []Row{{Label: "XMPlayer"}, {Label: "Sonata"}}, Selected: 0,
		Hint: "[A] SELECT   [D-PAD] MOVE   [B] BACK"}
	if err := screen.Render(frame); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "handheld.png")
	if err := screen.SavePNG(path); err != nil {
		t.Fatal(err)
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
	for _, sample := range []struct {
		x, y int
		rgb  uint32
	}{
		{50, 110, 0x0078d7},
		{50, 168, 0x202632},
		{5, 10, 0x0e1218},
		{5, 450, 0x0e1218},
	} {
		r, g, b, _ := img.At(sample.x, sample.y).RGBA()
		got := uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
		if got != sample.rgb {
			t.Errorf("pixel (%d,%d) = #%06x, want #%06x", sample.x, sample.y, got, sample.rgb)
		}
	}
}

func TestRenderFrameWritesPNG(t *testing.T) {
	os.Setenv("SDL_VIDEODRIVER", "dummy")
	theme := model.DefaultTheme()
	theme.Font = filepath.Join("..", "..", "assets", "JetBrainsMonoNerdFont-Regular.ttf")
	theme.FontCJK = ""
	if got := resolveCJKFont(theme); got == "" {
		t.Skip("no CJK font available")
	} else {
		theme.FontCJK = got
	}
	if _, err := os.Stat(theme.Font); err != nil {
		t.Skipf("font not available: %v", err)
	}

	screen, err := Open(Options{Theme: theme, Icons: model.Icons{}, Width: 720, Height: 480, Hidden: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer screen.Close()

	frame := Frame{
		Title:    "Applications",
		Status:   []string{"HEYA", "test"},
		Rows:     []Row{{Label: "Clock", Icon: 0xF0954}, {Label: "Hello"}},
		Selected: 0,
		Hint:     "[A] Select",
	}
	if err := screen.Render(frame); err != nil {
		t.Fatalf("render: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "frame.png")
	if err := screen.SavePNG(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 1000 {
		t.Fatalf("frame looks empty: %d bytes", info.Size())
	}
}

// TestCJKCompositesVisibleInk guards the mixed-font composition path: a
// run-by-run canvas that blits in the wrong direction, or loses per-run alpha,
// produces a correctly-sized surface with no visible pixels.
func TestCJKCompositesVisibleInk(t *testing.T) {
	os.Setenv("SDL_VIDEODRIVER", "dummy")
	theme := model.DefaultTheme()
	theme.Font = filepath.Join("..", "..", "assets", "JetBrainsMonoNerdFont-Regular.ttf")
	theme.FontCJK = ""
	if got := resolveCJKFont(theme); got == "" {
		t.Skip("no CJK font available")
	} else {
		theme.FontCJK = got
	}
	if _, err := os.Stat(theme.Font); err != nil {
		t.Skipf("latin font not available: %v", err)
	}
	if _, err := os.Stat(theme.FontCJK); err != nil {
		t.Skipf("CJK font not available: %v", err)
	}

	screen, err := Open(Options{Theme: theme, Icons: model.Icons{}, Width: 720, Height: 480, Hidden: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer screen.Close()

	surface, err := screen.renderText("你好 Hello 日本語", screen.pal.text)
	if err != nil {
		t.Fatalf("renderText: %v", err)
	}
	defer surface.Free()

	if surface.W < 60 {
		t.Fatalf("composed surface too narrow: %d", surface.W)
	}
	ink := 0
	pitch := int(surface.Pitch)
	for y := 0; y < int(surface.H); y++ {
		for x := 0; x < int(surface.W); x++ {
			i := y*pitch + x*4
			if i+3 < len(surface.Pixels()) && surface.Pixels()[i+3] > 32 {
				ink++
			}
		}
	}
	if ink == 0 {
		t.Fatal("mixed CJK/Latin composite has no opaque pixels; text would render invisible")
	}
	if ink < 100 {
		t.Fatalf("only %d ink pixels; composition looks incomplete", ink)
	}
}

// TestLatinUsesPrimaryFont keeps Nerd Font glyphs on the primary face so icons
// are never replaced by the CJK fallback.
func TestLatinUsesPrimaryFont(t *testing.T) {
	if containsCJK("Hello World") || containsCJK("[A] Select") {
		t.Fatal("latin text must not be routed to the CJK font")
	}
	for _, s := range []string{"你好世界", "繁體中文", "日本語", "한국어", "你好 Hello"} {
		if !containsCJK(s) {
			t.Fatalf("expected CJK routing for %q", s)
		}
	}
}

func TestParseColor(t *testing.T) {
	c, err := ParseColor("#0f78d7")
	if err != nil {
		t.Fatal(err)
	}
	if c.R != 0x0f || c.G != 0x78 || c.B != 0xd7 || c.A != 255 {
		t.Fatalf("unexpected colour: %+v", c)
	}
	if _, err := ParseColor("nope"); err == nil {
		t.Fatal("expected error for invalid colour")
	}
}

func TestResolveFontRequiresExistingFile(t *testing.T) {
	if _, err := resolveFont(model.Theme{Font: "/nonexistent/font.ttf"}); err == nil {
		t.Fatal("expected error for missing theme font")
	}
}
