// Package model holds the pure data types shared across heya modules:
// applications, pages, themes, icon maps and input mappings.
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// App is one discoverable application, described by app.json.
type App struct {
	Name     string   `json:"name"`
	Icon     string   `json:"icon"`
	Exec     string   `json:"exec"`
	Args     []string `json:"args,omitempty"`
	Category string   `json:"category,omitempty"`
	Hidden   bool     `json:"hidden,omitempty"`
}

// Validate rejects entries that cannot be launched safely.
func (a App) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("app name is empty")
	}
	if a.Exec == "" {
		return fmt.Errorf("app %q has no exec", a.Name)
	}
	if !filepath.IsAbs(a.Exec) {
		return fmt.Errorf("app %q exec must be an absolute path", a.Name)
	}
	return nil
}

// Item is one row in a page. Either Exec (launch) or Page (navigate) is set.
type Item struct {
	Name   string   `json:"name"`
	Icon   string   `json:"icon,omitempty"`
	Exec   string   `json:"exec,omitempty"`
	Args   []string `json:"args,omitempty"`
	Page   string   `json:"page,omitempty"`
	Action string   `json:"action,omitempty"`
	Value  string   `json:"value,omitempty"`
}

// Page is a menu page; static pages come from JSON, dynamic pages from the registry.
type Page struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

// Theme controls colours, font and integer scale factor.
type Theme struct {
	Background string `json:"background"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Accent     string `json:"accent"`
	Selection  string `json:"selection"`
	Font       string `json:"font"`
	Scale      int    `json:"scale"`
}

// DefaultTheme returns the built-in fallback theme.
func DefaultTheme() Theme {
	return Theme{
		Background: "#0b0e14",
		Text:       "#f2f4f8",
		Muted:      "#8b94a7",
		Accent:     "#ffc400",
		Selection:  "#0f78d7",
		Font:       "",
		Scale:      16,
	}
}

// Normalize fills unset fields from the default theme and clamps the scale.
func (t *Theme) Normalize() {
	d := DefaultTheme()
	if t.Background == "" {
		t.Background = d.Background
	}
	if t.Text == "" {
		t.Text = d.Text
	}
	if t.Muted == "" {
		t.Muted = d.Muted
	}
	if t.Accent == "" {
		t.Accent = d.Accent
	}
	if t.Selection == "" {
		t.Selection = d.Selection
	}
	switch t.Scale {
	case 8, 16, 32:
	default:
		t.Scale = d.Scale
	}
}

// Icons maps icon names such as "nf-md-music" to a hex Nerd Font codepoint.
type Icons map[string]string

// Glyph returns the rune for an icon name, or 0 when unknown.
func (i Icons) Glyph(name string) rune {
	hex, ok := i[name]
	if !ok {
		return 0
	}
	var v int64
	if _, err := fmt.Sscanf(strings.TrimPrefix(hex, "0x"), "%x", &v); err != nil {
		return 0
	}
	return rune(v)
}

// LoadJSON decodes a JSON file into dst.
func LoadJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
