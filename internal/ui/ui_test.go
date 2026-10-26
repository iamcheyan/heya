package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamcheyan/heya/internal/model"
)

func TestRenderFrameWritesPNG(t *testing.T) {
	os.Setenv("SDL_VIDEODRIVER", "dummy")
	theme := model.DefaultTheme()
	theme.Font = filepath.Join("..", "..", "assets", "JetBrainsMonoNerdFont-Regular.ttf")
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
