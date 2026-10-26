package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/iamcheyan/heya/internal/model"
)

// DefaultFontCandidates are tried in order when the theme names no font.
var DefaultFontCandidates = []string{
	"assets/JetBrainsMonoNerdFont-Regular.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
}

// DefaultCJKFontCandidates support user-visible Chinese/Japanese/Korean text.
var DefaultCJKFontCandidates = []string{
	"/usr/share/fonts/truetype/noto-cjk/NotoSansCJKsc-Regular.otf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	"/usr/share/heya/assets/NotoSansCJKsc-Regular.otf",
	"/usr/share/heya/assets/NotoSansCJK-Regular.ttc",
}

// ParseColor converts "#rrggbb" into an SDL colour.
func ParseColor(value string) (sdl.Color, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 {
		return sdl.Color{}, fmt.Errorf("invalid colour %q", value)
	}
	v, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return sdl.Color{}, fmt.Errorf("invalid colour %q: %w", value, err)
	}
	return sdl.Color{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, nil
}

// palette is the resolved theme.
type palette struct {
	background sdl.Color
	text       sdl.Color
	muted      sdl.Color
	accent     sdl.Color
	selection  sdl.Color
}

func resolvePalette(theme model.Theme) (palette, error) {
	var p palette
	var err error
	if p.background, err = ParseColor(theme.Background); err != nil {
		return p, err
	}
	if p.text, err = ParseColor(theme.Text); err != nil {
		return p, err
	}
	if p.muted, err = ParseColor(theme.Muted); err != nil {
		return p, err
	}
	if p.accent, err = ParseColor(theme.Accent); err != nil {
		return p, err
	}
	if p.selection, err = ParseColor(theme.Selection); err != nil {
		return p, err
	}
	return p, nil
}

func resolveFont(theme model.Theme) (string, error) {
	if theme.Font != "" {
		if _, err := os.Stat(theme.Font); err != nil {
			return "", fmt.Errorf("theme font %q: %w", theme.Font, err)
		}
		return theme.Font, nil
	}
	for _, candidate := range DefaultFontCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no font found; tried %v", DefaultFontCandidates)
}

func needsCJKFont(text string) bool {
	for _, r := range text {
		if r > 0x024f {
			return true
		}
	}
	return false
}

// Row is one selectable line.
type Row struct {
	Label string
	Value string
	Icon  rune
}

// Frame is everything the renderer needs for one picture.
type Frame struct {
	Title    string
	Rows     []Row
	Selected int
	Offset   int
	Visible  int
	Status   []string
	Hint     string
	Message  string
}

// UI owns the window, renderer, font and text cache.
type UI struct {
	win        *sdl.Window
	ren        *sdl.Renderer
	font       *ttf.Font
	cjkFont    *ttf.Font
	theme      model.Theme
	pal        palette
	icons      model.Icons
	cache      map[string]*sdl.Texture
	textWidths map[string]int32
	Width      int32
	Height     int32
	scale      int32
}

// Options configures UI creation.
type Options struct {
	Theme  model.Theme
	Icons  model.Icons
	Width  int32
	Height int32
	Hidden bool
}

// Open initialises SDL, the window and the font.
func Open(opts Options) (*UI, error) {
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_GAMECONTROLLER); err != nil {
		return nil, fmt.Errorf("sdl init: %w", err)
	}
	if err := ttf.Init(); err != nil {
		return nil, fmt.Errorf("ttf init: %w", err)
	}

	opts.Theme.Normalize()
	fontPath, err := resolveFont(opts.Theme)
	if err != nil {
		return nil, err
	}
	font, err := ttf.OpenFont(fontPath, opts.Theme.Scale)
	if err != nil {
		return nil, fmt.Errorf("open font %s: %w", fontPath, err)
	}
	cjkPath := opts.Theme.FontCJK
	if cjkPath != "" && !filepath.IsAbs(cjkPath) {
		if _, statErr := os.Stat(cjkPath); statErr != nil {
			for _, candidate := range []string{filepath.Join(filepath.Dir(opts.Theme.Font), filepath.Base(cjkPath)), filepath.Join("assets", filepath.Base(cjkPath)), filepath.Join("/usr/share/heya/assets", filepath.Base(cjkPath))} {
				if _, candidateErr := os.Stat(candidate); candidateErr == nil {
					cjkPath = candidate
					break
				}
			}
		}
	}
	if cjkPath == "" {
		for _, candidate := range DefaultCJKFontCandidates {
			if _, statErr := os.Stat(candidate); statErr == nil {
				cjkPath = candidate
				break
			}
		}
	}
	var cjkFont *ttf.Font
	if cjkPath != "" {
		cjkFont, err = ttf.OpenFont(cjkPath, opts.Theme.Scale)
		if err != nil {
			font.Close()
			return nil, fmt.Errorf("open CJK font %s: %w", cjkPath, err)
		}
	}

	flags := uint32(sdl.WINDOW_SHOWN)
	if opts.Hidden {
		flags = sdl.WINDOW_HIDDEN
	}
	win, err := sdl.CreateWindow("heya", sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
		opts.Width, opts.Height, flags)
	if err != nil {
		return nil, fmt.Errorf("create window: %w", err)
	}

	ren, err := sdl.CreateRenderer(win, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	if err != nil {
		ren, err = sdl.CreateRenderer(win, -1, sdl.RENDERER_SOFTWARE)
		if err != nil {
			return nil, fmt.Errorf("create renderer: %w", err)
		}
	}

	pal, err := resolvePalette(opts.Theme)
	if err != nil {
		return nil, err
	}

	return &UI{
		win: win, ren: ren, font: font, cjkFont: cjkFont, theme: opts.Theme, pal: pal,
		icons: opts.Icons, cache: map[string]*sdl.Texture{}, textWidths: map[string]int32{},
		Width: opts.Width, Height: opts.Height, scale: int32(opts.Theme.Scale),
	}, nil
}

// Close releases SDL resources.
func (u *UI) Close() {
	for _, texture := range u.cache {
		texture.Destroy()
	}
	u.cache = map[string]*sdl.Texture{}
	u.textWidths = map[string]int32{}
	if u.font != nil {
		u.font.Close()
		u.font = nil
	}
	if u.cjkFont != nil {
		u.cjkFont.Close()
		u.cjkFont = nil
	}
	if u.ren != nil {
		u.ren.Destroy()
		u.ren = nil
	}
	if u.win != nil {
		u.win.Destroy()
		u.win = nil
	}
	ttf.Quit()
	sdl.Quit()
}

func (u *UI) texture(text string, c sdl.Color) (*sdl.Texture, error) {
	key := text + "|" + strconv.Itoa(int(c.R)) + "," + strconv.Itoa(int(c.G)) + "," + strconv.Itoa(int(c.B))
	if texture, ok := u.cache[key]; ok {
		return texture, nil
	}
	font := u.font
	if u.cjkFont != nil && needsCJKFont(text) {
		font = u.cjkFont
	}
	surface, err := font.RenderUTF8Blended(text, c)
	if err != nil {
		return nil, err
	}
	defer surface.Free()
	width := surface.W
	texture, err := u.ren.CreateTextureFromSurface(surface)
	if err != nil {
		return nil, err
	}
	u.cache[key] = texture
	u.textWidths[key] = int32(width)
	return texture, nil
}

func (u *UI) drawText(text string, x, y int32, c sdl.Color) int32 {
	if text == "" {
		return 0
	}
	texture, err := u.texture(text, c)
	if err != nil {
		return 0
	}
	_, _, _, h, err := texture.Query()
	if err != nil {
		return 0
	}
	key := text + "|" + strconv.Itoa(int(c.R)) + "," + strconv.Itoa(int(c.G)) + "," + strconv.Itoa(int(c.B))
	w := u.textWidths[key]
	dst := sdl.Rect{X: x, Y: y, W: w, H: h}
	_ = u.ren.Copy(texture, nil, &dst)
	return w
}

func (u *UI) rowHeight() int32 {
	return u.scale + u.scale/2
}

// Render draws a frame.
func (u *UI) Render(frame Frame) error {
	_ = u.ren.SetDrawColor(u.pal.background.R, u.pal.background.G, u.pal.background.B, 255)
	_ = u.ren.Clear()

	topH := u.scale * 2
	botH := u.scale * 2
	titleH := u.scale * 2

	// Status bar
	_ = u.ren.SetDrawColor(18, 22, 30, 255)
	_ = u.ren.FillRect(&sdl.Rect{X: 0, Y: 0, W: u.Width, H: topH})

	x := u.scale / 2
	for i, segment := range frame.Status {
		if segment == "" {
			continue
		}
		c := u.pal.text
		if i == 0 {
			c = u.pal.accent
		}
		w := u.drawText(segment, x, (topH-u.scale)/2, c)
		x += w + u.scale
	}

	// Title
	u.drawText(frame.Title, u.scale, topH+u.scale/2, u.pal.accent)

	// Rows
	listTop := topH + titleH + u.scale
	rowH := u.rowHeight()
	visible := int((u.Height - listTop - botH - u.scale) / rowH)
	if visible < 1 {
		visible = 1
	}
	offset := frame.Offset
	if offset < 0 {
		offset = 0
	}
	for i := 0; i < visible && offset+i < len(frame.Rows); i++ {
		idx := offset + i
		row := frame.Rows[idx]
		y := listTop + int32(i)*rowH
		selected := idx == frame.Selected

		if selected {
			_ = u.ren.SetDrawColor(u.pal.selection.R, u.pal.selection.G, u.pal.selection.B, 255)
			_ = u.ren.FillRect(&sdl.Rect{X: u.scale / 2, Y: y - 2, W: u.Width - u.scale, H: rowH - 2})
		}

		labelColor := u.pal.text
		if !selected {
			labelColor = u.pal.muted
		}
		iconWidth := int32(0)
		if row.Icon != 0 {
			iconWidth = u.drawText(string(row.Icon), u.scale, y, u.pal.accent)
			iconWidth += u.scale / 2
		}
		labelWidth := u.drawText(row.Label, u.scale+iconWidth, y, labelColor)
		if row.Value != "" {
			valueX := u.scale + iconWidth + labelWidth + u.scale
			u.drawText(row.Value, valueX, y, u.pal.accent)
		}
	}

	// Scrollbar
	if len(frame.Rows) > visible {
		trackX := u.Width - u.scale
		trackH := int32(visible) * rowH
		_ = u.ren.SetDrawColor(40, 48, 64, 255)
		_ = u.ren.FillRect(&sdl.Rect{X: trackX, Y: listTop, W: u.scale / 2, H: trackH})
		thumbH := trackH * int32(visible) / int32(len(frame.Rows))
		if thumbH < u.scale {
			thumbH = u.scale
		}
		thumbY := listTop
		if len(frame.Rows) > visible {
			thumbY += (trackH - thumbH) * int32(offset) / int32(len(frame.Rows)-visible)
		}
		_ = u.ren.SetDrawColor(u.pal.accent.R, u.pal.accent.G, u.pal.accent.B, 255)
		_ = u.ren.FillRect(&sdl.Rect{X: trackX, Y: thumbY, W: u.scale / 2, H: thumbH})
	}

	// Message or hint bar
	_ = u.ren.SetDrawColor(18, 22, 30, 255)
	_ = u.ren.FillRect(&sdl.Rect{X: 0, Y: u.Height - botH, W: u.Width, H: botH})
	line := frame.Hint
	color := u.pal.muted
	if frame.Message != "" {
		line = frame.Message
		color = u.pal.accent
	}
	u.drawText(line, u.scale, u.Height-botH+(botH-u.scale)/2, color)

	u.ren.Present()
	return nil
}

// SavePNG writes the current render target to a PNG file.
func (u *UI) SavePNG(path string) error {
	w, h := u.Width, u.Height
	pixels := make([]byte, int(w)*int(h)*4)
	rect := sdl.Rect{X: 0, Y: 0, W: w, H: h}
	if err := u.ren.ReadPixels(&rect, sdl.PIXELFORMAT_ARGB8888, unsafe.Pointer(&pixels[0]), int(w)*4); err != nil {
		return fmt.Errorf("read pixels: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	for y := 0; y < int(h); y++ {
		for x := 0; x < int(w); x++ {
			i := (y*int(w) + x) * 4
			b, g, r, a := pixels[i], pixels[i+1], pixels[i+2], pixels[i+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

// IconGlyph resolves an icon name, falling back to a neutral marker.
func (u *UI) IconGlyph(name string) rune {
	if glyph := u.icons.Glyph(name); glyph != 0 {
		return glyph
	}
	return '▪'
}
