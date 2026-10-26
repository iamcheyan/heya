package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeNormalizeFillsDefaults(t *testing.T) {
	theme := Theme{}
	theme.Normalize()
	def := DefaultTheme()
	if theme.Background != def.Background || theme.Text != def.Text {
		t.Fatalf("defaults not applied: %+v", theme)
	}
	if theme.Scale != 16 {
		t.Fatalf("scale = %d, want 16", theme.Scale)
	}
	if theme.Font == "" || theme.FontCJK == "" {
		t.Fatalf("font defaults not applied: %+v", theme)
	}
}

func TestThemeNormalizeClampsScale(t *testing.T) {
	theme := Theme{Scale: 13}
	theme.Normalize()
	if theme.Scale != 16 {
		t.Fatalf("scale = %d, want fallback 16", theme.Scale)
	}
	for _, scale := range []int{8, 16, 32} {
		theme := Theme{Scale: scale}
		theme.Normalize()
		if theme.Scale != scale {
			t.Fatalf("scale %d was changed to %d", scale, theme.Scale)
		}
	}
}

func TestIconsGlyph(t *testing.T) {
	icons := Icons{"nf-md-music": "F0759", "nf-md-cog": "0xF0493", "broken": "zz"}
	if got := icons.Glyph("nf-md-music"); got != 0xF0759 {
		t.Fatalf("glyph = %U, want U+F0759", got)
	}
	if got := icons.Glyph("nf-md-cog"); got != 0xF0493 {
		t.Fatalf("0x-prefixed glyph = %U", got)
	}
	if got := icons.Glyph("broken"); got != 0 {
		t.Fatalf("invalid glyph should be 0, got %U", got)
	}
	if got := icons.Glyph("missing"); got != 0 {
		t.Fatalf("missing glyph should be 0, got %U", got)
	}
}

func TestLoadJSONRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	if err := os.WriteFile(path, []byte(`{"name":"x","exec":"/bin/true","bogus":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var app App
	if err := LoadJSON(path, &app); err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestAppNormalizeLegacyAndEntrypoint(t *testing.T) {
	legacy := App{Name: "Music", Exec: "/usr/bin/music"}
	legacy.Normalize("music-dir")
	if legacy.ManifestVersion != 1 || legacy.ID != "music-dir" || legacy.Entrypoint != "/usr/bin/music" {
		t.Fatalf("legacy normalization: %+v", legacy)
	}
	modern := App{Name: "Tool", ManifestVersion: 1, ID: "tool", Entrypoint: "/usr/bin/tool"}
	modern.Normalize("other-dir")
	if modern.Exec != modern.Entrypoint || modern.ID != "tool" {
		t.Fatalf("modern normalization: %+v", modern)
	}
	if err := modern.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAppValidate(t *testing.T) {
	cases := []struct {
		app App
		ok  bool
	}{
		{App{Name: "ok", Exec: "/bin/true"}, true},
		{App{Name: "", Exec: "/bin/true"}, false},
		{App{Name: "noexec"}, false},
		{App{Name: "rel", Exec: "bin/true"}, false},
	}
	for _, c := range cases {
		if err := c.app.Validate(); (err == nil) != c.ok {
			t.Fatalf("Validate(%+v) = %v, want ok=%v", c.app, err, c.ok)
		}
	}
}
