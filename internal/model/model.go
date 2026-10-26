// Package model holds the pure data types shared across heya modules:
// applications, pages, themes, icon maps and input mappings.
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// App is one discoverable application, described by app.json.
type App struct {
	ManifestVersion  int      `json:"manifestVersion,omitempty"`
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Version          string   `json:"version,omitempty"`
	Author           string   `json:"author,omitempty"`
	License          string   `json:"license,omitempty"`
	Homepage         string   `json:"homepage,omitempty"`
	Icon             string   `json:"icon"`
	IconColor        string   `json:"iconColor,omitempty"`
	Badge            string   `json:"badge,omitempty"`
	Accent           string   `json:"accent,omitempty"`
	Entrypoint       string   `json:"entrypoint,omitempty"`
	Exec             string   `json:"exec,omitempty"`
	Args             []string `json:"args,omitempty"`
	Environment      []string `json:"environment,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	Category         string   `json:"category,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
	Features         []string `json:"features,omitempty"`
	Hidden           bool     `json:"hidden,omitempty"`
}

var identityPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// Normalize applies compatibility defaults to manifests predating versioning.
func (a *App) Normalize(directory string) {
	if a.ManifestVersion == 0 {
		a.ManifestVersion = 1
	}
	if a.ID == "" {
		a.ID = strings.ToLower(directory)
		a.ID = strings.ReplaceAll(a.ID, " ", "-")
	}
	if a.Entrypoint == "" {
		a.Entrypoint = a.Exec
	}
	if a.Exec == "" {
		a.Exec = a.Entrypoint
	}
}

// Validate rejects entries that cannot be launched safely.
func (a App) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("app name is empty")
	}
	if a.ManifestVersion != 0 && a.ManifestVersion != 1 {
		return fmt.Errorf("app %q has unsupported manifestVersion %d", a.Name, a.ManifestVersion)
	}
	if a.ID != "" && !identityPattern.MatchString(a.ID) {
		return fmt.Errorf("app %q has invalid id %q", a.Name, a.ID)
	}
	if a.Entrypoint == "" && a.Exec == "" {
		return fmt.Errorf("app %q has no entrypoint or exec", a.Name)
	}
	if a.Entrypoint != "" && !filepath.IsAbs(a.Entrypoint) {
		return fmt.Errorf("app %q entrypoint must be an absolute path", a.Name)
	}
	if a.Exec != "" && !filepath.IsAbs(a.Exec) {
		return fmt.Errorf("app %q exec must be an absolute path", a.Name)
	}
	if a.Exec != "" && a.Entrypoint != "" && a.Exec != a.Entrypoint {
		return fmt.Errorf("app %q specifies conflicting entrypoint and exec", a.Name)
	}
	if a.WorkingDirectory != "" && !filepath.IsAbs(a.WorkingDirectory) {
		return fmt.Errorf("app %q workingDirectory must be an absolute path", a.Name)
	}
	for _, value := range a.Environment {
		key, _, ok := strings.Cut(value, "=")
		if !ok || key == "" || strings.ContainsAny(key, " \t\x00") {
			return fmt.Errorf("app %q has invalid environment entry %q", a.Name, value)
		}
	}
	return nil
}

type Item struct {
	Name             string   `json:"name"`
	Icon             string   `json:"icon,omitempty"`
	Exec             string   `json:"exec,omitempty"`
	Args             []string `json:"args,omitempty"`
	Environment      []string `json:"environment,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
	Features         []string `json:"features,omitempty"`
	Page             string   `json:"page,omitempty"`
	Action           string   `json:"action,omitempty"`
	Value            string   `json:"value,omitempty"`
}
type Page struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}
type Theme struct {
	Background    string `json:"background"`
	Text          string `json:"text"`
	Muted         string `json:"muted"`
	Accent        string `json:"accent"`
	Selection     string `json:"selection"`
	Panel         string `json:"panel"`
	Bar           string `json:"bar"`
	Info          string `json:"info"`
	Success       string `json:"success"`
	SelectionText string `json:"selection_text"`
	Font          string `json:"font"`
	FontCJK       string `json:"font_cjk"`
	FontLatin     string `json:"font_latin"`
	Scale         int    `json:"scale"`
}

func DefaultTheme() Theme {
	return Theme{Background: "#141820", Text: "#f5f5f5", Muted: "#a0a0a0", Accent: "#ffc400",
		Selection: "#0078d7", Panel: "#202632", Bar: "#0e1218", Info: "#00dcff",
		Success: "#50dc78", SelectionText: "#ffffff", Font: "assets/JetBrainsMonoNerdFont-Regular.ttf",
		FontCJK: "/usr/share/fonts/truetype/noto-cjk/NotoSansCJKsc-Regular.otf", Scale: 16}
}
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
	for field, fallback := range map[*string]string{
		&t.Panel: d.Panel, &t.Bar: d.Bar, &t.Info: d.Info,
		&t.Success: d.Success, &t.SelectionText: d.SelectionText,
	} {
		if *field == "" {
			*field = fallback
		}
	}
	if t.Font == "" {
		t.Font = d.Font
	}
	if t.FontCJK == "" {
		t.FontCJK = d.FontCJK
	}
	if t.FontLatin == "" {
		t.FontLatin = d.FontLatin
	}
	switch t.Scale {
	case 8, 16, 32:
	default:
		t.Scale = d.Scale
	}
}

type Icons map[string]string

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
