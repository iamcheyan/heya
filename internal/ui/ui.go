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
	background    sdl.Color
	text          sdl.Color
	muted         sdl.Color
	accent        sdl.Color
	selection     sdl.Color
	panel         sdl.Color
	bar           sdl.Color
	info          sdl.Color
	success       sdl.Color
	selectionText sdl.Color
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
	for field, value := range map[*sdl.Color]string{
		&p.panel: theme.Panel, &p.bar: theme.Bar, &p.info: theme.Info,
		&p.success: theme.Success, &p.selectionText: theme.SelectionText,
	} {
		if *field, err = ParseColor(value); err != nil {
			return p, err
		}
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

// resolveCJKFont finds the CJK fallback face. A missing theme entry is not
// fatal: heya still runs, it only loses non-Latin coverage. Relative theme
// paths resolve against the primary font's directory, the local and installed
// asset directories, then the system candidates.
func resolveCJKFont(theme model.Theme) string {
	if base := theme.FontCJK; base != "" {
		candidates := []string{base}
		if !filepath.IsAbs(base) {
			for _, dir := range []string{filepath.Dir(theme.Font), "assets", "/usr/share/heya/assets"} {
				candidates = append(candidates, filepath.Join(dir, base))
			}
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	for _, candidate := range DefaultCJKFontCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// Row is one selectable line.
type Row struct {
	Cells        []string
	CellSelected int
	Label        string
	Value        string
	Icon         rune
}

// Frame is everything the renderer needs for one picture.
type Frame struct {
	Title           string
	TerminalProfile bool
	Rows            []Row
	Selected        int
	Offset          int
	Visible         int
	Status          []string
	Hint            string
	Message         string
}

// UI owns the window, renderer, font and text cache.
type UI struct {
	win         *sdl.Window
	ren         *sdl.Renderer
	font        *ttf.Font
	cjkFont     *ttf.Font
	theme       model.Theme
	pal         palette
	icons       model.Icons
	cache       map[string]*sdl.Texture
	textWidths  map[string]int32
	textHeights map[string]int32
	Width       int32
	Height      int32
	scale       int32
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
	fontSize := opts.Theme.Scale * 3 / 2
	font, err := ttf.OpenFont(fontPath, fontSize)
	if err != nil {
		return nil, fmt.Errorf("open font %s: %w", fontPath, err)
	}
	font.SetStyle(ttf.STYLE_BOLD)
	var cjkFont *ttf.Font
	if cjkPath := resolveCJKFont(opts.Theme); cjkPath != "" {
		// A CJK face is an enhancement, never a boot requirement: an unreadable
		// file leaves Latin rendering intact.
		if cjkFont, err = ttf.OpenFont(cjkPath, fontSize); err != nil {
			fmt.Fprintf(os.Stderr, "heya: CJK font %s: %v\n", cjkPath, err)
			cjkFont = nil
		}
		if cjkFont != nil {
			cjkFont.SetStyle(ttf.STYLE_BOLD)
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
		icons: opts.Icons, cache: map[string]*sdl.Texture{}, textWidths: map[string]int32{}, textHeights: map[string]int32{},
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
	u.textHeights = map[string]int32{}
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
	surfaces, err := u.renderText(text, c)
	if err != nil {
		return nil, err
	}
	defer surfaces.Free()
	texture, err := u.ren.CreateTextureFromSurface(surfaces)
	if err != nil {
		return nil, err
	}
	u.cache[key] = texture
	u.textWidths[key] = int32(surfaces.W)
	u.textHeights[key] = int32(surfaces.H)
	return texture, nil
}

func (u *UI) renderText(text string, c sdl.Color) (*sdl.Surface, error) {
	if u.cjkFont == nil || !containsCJK(text) {
		return u.font.RenderUTF8Blended(text, c)
	}
	// Render text runs with the best matching face; Latin runs retain Nerd Font icons.
	type run struct {
		text string
		font *ttf.Font
	}
	var runs []run
	for _, r := range text {
		font := u.font
		if isCJKRune(r) {
			font = u.cjkFont
		}
		if len(runs) > 0 && runs[len(runs)-1].font == font {
			runs[len(runs)-1].text += string(r)
		} else {
			runs = append(runs, run{text: string(r), font: font})
		}
	}
	parts := make([]*sdl.Surface, 0, len(runs))
	width := int32(0)
	height := u.scale * 2
	for _, item := range runs {
		part, renderErr := item.font.RenderUTF8Blended(item.text, c)
		if renderErr != nil {
			for _, surface := range parts {
				surface.Free()
			}
			return nil, renderErr
		}
		parts = append(parts, part)
		width += part.W
		if part.H > height {
			height = part.H
		}
	}
	if width < 1 {
		width = 1
	}
	// ARGB8888 matches SDL_ttf's blended output, so per-run pixels (colour plus
	// coverage) copy straight into the canvas without re-blending.
	result, err := sdl.CreateRGBSurfaceWithFormat(0, width, height, 32, uint32(sdl.PIXELFORMAT_ARGB8888))
	if err != nil {
		for _, surface := range parts {
			surface.Free()
		}
		return nil, err
	}
	// Blank the canvas; per-run pixels then overwrite their own coverage.
	if err := result.FillRect(nil, 0); err != nil {
		for _, surface := range parts {
			surface.Free()
		}
		result.Free()
		return nil, err
	}
	x := int32(0)
	for _, part := range parts {
		// The receiver is the blit *source*: copy each rendered run onto the
		// canvas, preserving its coverage so the composed texture is visible.
		dst := sdl.Rect{X: x, Y: 0, W: part.W, H: part.H}
		if err := part.Blit(nil, result, &dst); err != nil {
			for _, surface := range parts {
				surface.Free()
			}
			result.Free()
			return nil, err
		}
		x += part.W
		part.Free()
	}
	return result, nil
}

func containsCJK(text string) bool {
	for _, r := range text {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

func isCJKRune(r rune) bool {
	return (r >= 0x2e80 && r <= 0x9fff) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0x3040 && r <= 0x30ff) || (r >= 0xac00 && r <= 0xd7af)
}

func (u *UI) drawText(text string, x, y int32, c sdl.Color) int32 {
	if text == "" {
		return 0
	}
	texture, err := u.texture(text, c)
	if err != nil {
		return 0
	}
	key := text + "|" + strconv.Itoa(int(c.R)) + "," + strconv.Itoa(int(c.G)) + "," + strconv.Itoa(int(c.B))
	dst := sdl.Rect{X: x, Y: y, W: u.textWidths[key], H: u.textHeights[key]}
	_ = u.ren.Copy(texture, nil, &dst)
	return u.textWidths[key]
}

type menuLayout struct {
	top, bottom, listTop, left, width, rowHeight, gap int32
	unit                                              float64
}

func (u *UI) layout() menuLayout {
	unit := float64(u.Width) / 720
	if vertical := float64(u.Height) / 480; vertical < unit {
		unit = vertical
	}
	px := func(n int32) int32 { return max(1, int32(float64(n)*unit)) }
	rowHeight := max(px(48), int32(u.font.Height())+px(12))
	return menuLayout{top: px(36), bottom: px(38), listTop: px(92), left: px(45),
		width: u.Width - px(100), rowHeight: rowHeight, gap: px(10), unit: unit}
}

func (u *UI) VisibleRows() int {
	layout := u.layout()
	return max(1, int((u.Height-layout.bottom-layout.listTop+layout.gap)/(layout.rowHeight+layout.gap)))
}

func (u *UI) fill(rect sdl.Rect, c sdl.Color) {
	_ = u.ren.SetDrawColor(c.R, c.G, c.B, 255)
	_ = u.ren.FillRect(&rect)
}

func (u *UI) textSize(text string, c sdl.Color) (int32, int32) {
	if _, err := u.texture(text, c); err != nil {
		return 0, 0
	}
	key := text + "|" + strconv.Itoa(int(c.R)) + "," + strconv.Itoa(int(c.G)) + "," + strconv.Itoa(int(c.B))
	return u.textWidths[key], u.textHeights[key]
}

func (u *UI) textInBox(text string, box sdl.Rect, c sdl.Color, centered bool) {
	if text == "" || box.W <= 0 || box.H <= 0 {
		return
	}
	texture, err := u.texture(text, c)
	if err != nil {
		return
	}
	w, h := u.textSize(text, c)
	if w <= 0 || h <= 0 {
		return
	}
	ratio := min(1.0, float64(box.H)/float64(h), float64(box.W)/float64(w))
	w, h = int32(float64(w)*ratio), int32(float64(h)*ratio)
	x := box.X
	if centered {
		x += (box.W - w) / 2
	}
	dst := sdl.Rect{X: x, Y: box.Y + (box.H-h)/2, W: w, H: h}
	_ = u.ren.Copy(texture, nil, &dst)
}

func (u *UI) rowText(text string, box sdl.Rect, c sdl.Color, selected bool, right bool) {
	if text == "" || box.W <= 0 || box.H <= 0 {
		return
	}
	texture, err := u.texture(text, c)
	if err != nil {
		return
	}
	w, h := u.textSize(text, c)
	if w <= 0 || h <= 0 {
		return
	}
	ratio := min(1.0, float64(box.H)/float64(h))
	w, h = int32(float64(w)*ratio), int32(float64(h)*ratio)
	x := box.X
	if right && w <= box.W {
		x += box.W - w
	}
	if w > box.W && selected {
		distance := w - box.W
		phase := int32(sdl.GetTicks()/30) % (2*distance + 120)
		offset := min(distance, max(0, phase-60))
		if phase > distance+120 {
			offset = max(0, 2*distance+120-phase)
		}
		x -= offset
	}
	_ = u.ren.SetClipRect(&box)
	dst := sdl.Rect{X: x, Y: box.Y + (box.H-h)/2, W: w, H: h}
	_ = u.ren.Copy(texture, nil, &dst)
	_ = u.ren.SetClipRect(nil)
}

// Render draws the same handheld menu layout on every page.
func (u *UI) Render(frame Frame) error {
	_ = u.ren.SetDrawColor(u.pal.background.R, u.pal.background.G, u.pal.background.B, 255)
	_ = u.ren.Clear()
	layout := u.layout()
	px := func(n int32) int32 { return max(1, int32(float64(n)*layout.unit)) }
	if frame.TerminalProfile {
		u.renderSystemProfile(frame, px)
		return nil
	}

	u.fill(sdl.Rect{W: u.Width, H: layout.top}, u.pal.bar)
	slotWidth := u.Width / 4
	for i, segment := range frame.Status {
		if i >= 4 {
			break
		}
		c := u.pal.text
		switch i {
		case 0:
			c = u.pal.info
		case 1, 2:
			c = u.pal.muted
			if strings.HasSuffix(segment, "ON") {
				c = u.pal.success
			}
			if i == 2 && strings.HasSuffix(segment, "ON") {
				c = u.pal.accent
			}
		}
		u.textInBox(segment, sdl.Rect{X: int32(i)*slotWidth + px(20), Y: px(6), W: slotWidth - px(32), H: layout.top - px(12)}, c, false)
	}

	title := strings.ToUpper(frame.Title)
	if title == "APPLICATIONS" {
		title = "HEYA LAUNCHER"
	}
	u.textInBox("=== "+title+" ===", sdl.Rect{X: px(45), Y: px(44), W: u.Width - px(90), H: px(30)}, u.pal.accent, true)

	visible := u.VisibleRows()
	if frame.Visible > 0 {
		visible = min(visible, frame.Visible)
	}
	offset := min(max(0, frame.Offset), max(0, len(frame.Rows)-visible))
	for i := 0; i < visible && offset+i < len(frame.Rows); i++ {
		index := offset + i
		row := frame.Rows[index]
		selected := index == frame.Selected
		rect := sdl.Rect{X: layout.left, Y: layout.listTop + int32(i)*(layout.rowHeight+layout.gap), W: layout.width, H: layout.rowHeight}
		background, label := u.pal.panel, u.pal.muted
		if selected {
			background, label = u.pal.selection, u.pal.selectionText
			u.textInBox(">", sdl.Rect{X: px(16), Y: rect.Y, W: px(22), H: rect.H}, u.pal.accent, true)
		}
		u.fill(rect, background)
		if len(row.Cells) > 0 {
			cellW := rect.W / int32(len(row.Cells))
			for j, key := range row.Cells {
				cell := sdl.Rect{X: rect.X + int32(j)*cellW + px(2), Y: rect.Y + px(3), W: cellW - px(4), H: rect.H - px(6)}
				color := u.pal.text
				if j == row.CellSelected {
					u.fill(cell, u.pal.accent)
					color = u.pal.bar
				}
				u.textInBox(key, cell, color, true)
			}
			continue
		}
		textX := rect.X + px(15)
		if row.Icon != 0 {
			u.textInBox(string(row.Icon), sdl.Rect{X: textX, Y: rect.Y, W: px(26), H: rect.H}, u.pal.accent, true)
			textX += px(38)
		}
		textWidth := rect.X + rect.W - px(20) - textX
		box := sdl.Rect{X: textX, Y: rect.Y + px(6), W: textWidth, H: rect.H - px(12)}
		if row.Value != "" {
			box.W = textWidth * 46 / 100
			valueBox := sdl.Rect{X: textX + textWidth/2, Y: box.Y, W: textWidth / 2, H: box.H}
			u.rowText(row.Value, valueBox, u.pal.accent, selected, true)
		}
		u.rowText(row.Label, box, label, selected, false)
	}

	if len(frame.Rows) > visible {
		track := sdl.Rect{X: u.Width - px(26), Y: layout.listTop, W: px(6), H: int32(visible)*(layout.rowHeight+layout.gap) - layout.gap}
		u.fill(track, u.pal.panel)
		thumbHeight := max(px(20), track.H*int32(visible)/int32(len(frame.Rows)))
		thumbY := track.Y + (track.H-thumbHeight)*int32(offset)/int32(len(frame.Rows)-visible)
		u.fill(sdl.Rect{X: track.X, Y: thumbY, W: track.W, H: thumbHeight}, u.pal.accent)
	}

	footer := sdl.Rect{Y: u.Height - layout.bottom, W: u.Width, H: layout.bottom}
	u.fill(footer, u.pal.bar)
	line, c := frame.Hint, u.pal.muted
	if frame.Message != "" {
		line, c = frame.Message, u.pal.accent
	}
	u.textInBox(line, sdl.Rect{X: px(16), Y: footer.Y + px(6), W: u.Width - px(32), H: footer.H - px(12)}, c, true)
	u.ren.Present()
	return nil
}

func (u *UI) renderSystemProfile(frame Frame, px func(int32) int32) {
	black := sdl.Color{R: 0, G: 0, B: 0, A: 255}
	green := sdl.Color{R: 112, G: 220, B: 151, A: 255}
	softWhite := sdl.Color{R: 220, G: 228, B: 224, A: 255}
	muted := sdl.Color{R: 92, G: 112, B: 103, A: 255}
	cyan := sdl.Color{R: 95, G: 205, B: 220, A: 255}
	_ = u.ren.SetDrawColor(black.R, black.G, black.B, black.A)
	_ = u.ren.Clear()
	margin := px(28)
	u.drawText("heya@rocknix:~$ system-profile", margin, px(18), green)
	u.fill(sdl.Rect{X: margin, Y: px(54), W: u.Width - 2*margin, H: px(1)}, muted)

	logo := []string{
		"  _   _",
		" | | | |",
		" | |_| |",
		" |  _  |",
		" |_| |_|",
		"   HEYA",
	}
	logoY := px(150)
	for i, line := range logo {
		u.drawText(line, margin, logoY+int32(i)*px(30), green)
	}
	u.drawText("SYSTEM", margin, px(365), cyan)
	u.drawText("INFORMATION", margin, px(392), muted)

	profileX := max(px(245), u.Width*39/100)
	rowTop := px(91)
	rowGap := px(32)
	labelWidth := px(140)
	valueX := profileX + labelWidth
	for i, row := range frame.Rows {
		y := rowTop + int32(i)*rowGap
		u.textInBox(row.Label, sdl.Rect{X: profileX, Y: y, W: labelWidth - px(10), H: rowGap}, cyan, false)
		u.textInBox(row.Value, sdl.Rect{X: valueX, Y: y, W: u.Width - margin - valueX, H: rowGap}, softWhite, false)
	}
	u.fill(sdl.Rect{X: margin, Y: u.Height - px(48), W: u.Width - 2*margin, H: px(1)}, muted)
	u.drawText("[B] BACK", margin, u.Height-px(34), muted)
	u.textInBox("ROCKNIX HANDHELD", sdl.Rect{X: u.Width - margin - px(220), Y: u.Height - px(39), W: px(220), H: px(24)}, muted, false)
	u.ren.Present()
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
