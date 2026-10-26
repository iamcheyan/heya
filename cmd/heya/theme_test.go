package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/model"
)

func TestLoadConfigResolvesThemeFontsRelativeToThemeFile(t *testing.T) {
	dir := t.TempDir()
	fonts := filepath.Join(dir, "assets")
	if err := os.MkdirAll(fonts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"latin.ttf", "cjk.ttc"} {
		if err := os.WriteFile(filepath.Join(fonts, name), []byte("font"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	themePath := filepath.Join(dir, "theme.json")
	data := []byte(`{"font":"assets/latin.ttf","font_cjk":"assets/cjk.ttc","font_latin":"assets/latin.ttf"}`)
	if err := os.WriteFile(themePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	theme, _, _, err := loadConfig(options{themePath: themePath})
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []string{theme.Font, theme.FontCJK, theme.FontLatin} {
		if !filepath.IsAbs(got) {
			t.Errorf("font path not resolved: %q", got)
		}
		if _, err := os.Stat(got); err != nil {
			t.Errorf("font path %q: %v", got, err)
		}
	}
}

func TestLoadConfigKeepsDefaultThemeValues(t *testing.T) {
	theme, _, _, err := loadConfig(options{})
	if err != nil {
		t.Fatal(err)
	}
	if theme.Selection == "" || theme.Font == "" {
		t.Fatalf("default theme incomplete: %+v", theme)
	}
	_ = input.DefaultMapping()
	_ = model.DefaultTheme()
}
