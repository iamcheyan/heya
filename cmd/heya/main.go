// Command heya is a lightweight, JSON-driven launcher shell.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/sdl"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/launcher"
	"github.com/iamcheyan/heya/internal/layout"
	"github.com/iamcheyan/heya/internal/model"
	"github.com/iamcheyan/heya/internal/registry"
	"github.com/iamcheyan/heya/internal/system"
	"github.com/iamcheyan/heya/internal/ui"
)

const version = "0.2.0"

type options struct {
	appsDir           string
	launchEnvironment []string
	launchDirectory   string
	themePath         string
	iconsPath         string
	inputPath         string
	fontPath          string
	width             int
	height            int
	dumpDir           string
	verbose           bool
}

func main() {
	var opts options
	flag.StringVar(&opts.appsDir, "apps", defaultAppsDir(), "directory containing one subdirectory per application")
	flag.StringVar(&opts.themePath, "theme", "", "theme JSON file (default: built-in theme)")
	flag.StringVar(&opts.iconsPath, "icons", "", "icon map JSON file (default: assets/icons.json)")
	flag.StringVar(&opts.inputPath, "input", "", "input mapping JSON file (default: built-in mapping)")
	flag.StringVar(&opts.fontPath, "font", "", "font file overriding the theme font")
	flag.IntVar(&opts.width, "width", 720, "window width")
	flag.IntVar(&opts.height, "height", 480, "window height")
	flag.StringVar(&opts.dumpDir, "dump", "", "render each page to PNG in this directory and exit")
	flag.BoolVar(&opts.verbose, "verbose", false, "log input and lifecycle events")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "heya: %v\n", err)
		os.Exit(1)
	}
}

func defaultAppsDir() string {
	if _, err := os.Stat("/storage/heya/apps"); err == nil {
		return "/storage/heya/apps"
	}
	return "examples/apps"
}

// dataRoot is where heya keeps its own state on a handheld.
const dataRoot = layout.Root

// ensureDataRoot creates the launcher's directory layout on first boot and
// seeds a theme the systemd unit can point at.
func ensureDataRoot() {
	if _, err := os.Stat("/storage"); err != nil {
		return
	}
	for _, dir := range layout.Directories {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "heya: %v\n", err)
		}
	}
	themePath := filepath.Join(layout.Themes, "theme.json")
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		theme := model.DefaultTheme()
		theme.Font = "/usr/share/heya/assets/JetBrainsMonoNerdFont-Regular.ttf"
		theme.FontCJK = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
		if data, err := json.MarshalIndent(theme, "", "  "); err == nil {
			_ = os.WriteFile(themePath, append(data, '\n'), 0o644)
		}
	}
}

func run(opts options) error {
	ensureDataRoot()
	if opts.themePath == "" {
		candidate := filepath.Join(layout.Themes, "theme.json")
		if _, err := os.Stat(candidate); err == nil {
			opts.themePath = candidate
		}
	}
	theme, icons, mapping, err := loadConfig(opts)
	if err != nil {
		return err
	}

	handler, err := input.NewHandler(mapping)
	if err != nil {
		return fmt.Errorf("input mapping: %w", err)
	}

	probe := system.NewProbe()
	appsDir := opts.appsDir
	rescan := func() model.Page {
		apps, problems := registry.New(appsDir).Scan()
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "heya: skipping app: %v\n", problem)
		}
		return appsPage(registry.Page("Applications", apps))
	}

	l := launcher.New(rescan(), probe, icons)
	l.AddPage("system", model.Page{Title: "System"})
	l.AddPage("about", aboutPage(opts))
	l.Refresh = rescan

	if opts.dumpDir != "" {
		return dumpPages(l, theme, icons, opts)
	}
	return eventLoop(l, handler, theme, icons, opts)
}

// appsPage appends the built-in navigation entries to the scanned apps.
func appsPage(page model.Page) model.Page {
	page.Items = append(page.Items,
		model.Item{Name: "System", Icon: "nf-md-cog", Page: "system"},
		model.Item{Name: "About heya", Icon: "nf-md-information", Page: "about"},
	)
	return page
}

func aboutPage(opts options) model.Page {
	host, _ := os.Hostname()
	return model.Page{
		Title: "About",
		Items: []model.Item{
			{Name: "heya", Value: version},
			{Name: "Host", Value: host},
			{Name: "Apps directory", Value: opts.appsDir},
			{Name: "Display", Value: fmt.Sprintf("%dx%d", opts.width, opts.height)},
			{Name: "Data root", Value: "/storage/heya"},
		},
	}
}

func loadConfig(opts options) (model.Theme, model.Icons, input.Mapping, error) {
	theme := model.DefaultTheme()
	if opts.themePath != "" {
		if err := model.LoadJSON(opts.themePath, &theme); err != nil {
			return theme, nil, input.Mapping{}, fmt.Errorf("theme: %w", err)
		}
	}
	theme.Normalize()
	if opts.themePath != "" {
		base := filepath.Dir(opts.themePath)
		for _, font := range []*string{&theme.Font, &theme.FontCJK, &theme.FontLatin} {
			if *font == "" || filepath.IsAbs(*font) {
				continue
			}
			candidate := filepath.Join(base, *font)
			if _, statErr := os.Stat(candidate); statErr == nil {
				*font = candidate
			}
		}
	}
	if opts.fontPath != "" {
		theme.Font = opts.fontPath
	}

	icons := model.Icons{}
	iconsPath := opts.iconsPath
	if iconsPath == "" {
		iconsPath = filepath.Join(assetDir(), "icons.json")
	}
	if _, err := os.Stat(iconsPath); err == nil {
		if err := model.LoadJSON(iconsPath, &icons); err != nil {
			return theme, nil, input.Mapping{}, fmt.Errorf("icons: %w", err)
		}
	}
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "heya: loaded %d icons from %q (asset dir %q)\n", len(icons), iconsPath, assetDir())
	}

	mapping := input.DefaultMapping()
	if opts.inputPath != "" {
		if err := model.LoadJSON(opts.inputPath, &mapping); err != nil {
			return theme, nil, input.Mapping{}, fmt.Errorf("input: %w", err)
		}
	}
	return theme, icons, mapping, nil
}

// assetDir finds the bundled assets directory.
func assetDir() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "assets")
		if _, err := os.Stat(filepath.Join(candidate, "icons.json")); err == nil {
			return candidate
		}
	}
	for _, candidate := range []string{"assets", "/usr/share/heya/assets"} {
		if _, err := os.Stat(filepath.Join(candidate, "icons.json")); err == nil {
			return candidate
		}
	}
	return "assets"
}

func openUI(theme model.Theme, icons model.Icons, opts options) (*ui.UI, error) {
	return ui.Open(ui.Options{
		Theme:  theme,
		Icons:  icons,
		Width:  int32(opts.width),
		Height: int32(opts.height),
		Hidden: opts.dumpDir != "",
	})
}

func eventLoop(l *launcher.Launcher, handler *input.Handler, theme model.Theme, icons model.Icons, opts options) error {
	screen, err := openUI(theme, icons, opts)
	if err != nil {
		return err
	}
	defer func() {
		if screen != nil {
			screen.Close()
		}
	}()

	var controller *sdl.GameController
	if sdl.NumJoysticks() > 0 {
		controller = sdl.GameControllerOpen(0)
	}
	defer func() {
		if controller != nil {
			controller.Close()
		}
	}()

	for {
		if err := screen.Render(l.Frame()); err != nil {
			return err
		}

		for {
			event := sdl.PollEvent()
			if event == nil {
				break
			}
			action := input.None
			switch e := event.(type) {
			case *sdl.QuitEvent:
				return nil
			case *sdl.KeyboardEvent:
				if e.Type == sdl.KEYDOWN {
					action = handler.Key(e.Keysym.Sym)
				}
			case *sdl.ControllerButtonEvent:
				if e.Type == sdl.CONTROLLERBUTTONDOWN {
					action = handler.Button(sdl.GameControllerButton(e.Button))
				}
			case *sdl.ControllerDeviceEvent:
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "heya: controller event type=%d\n", e.Type)
				}
			}
			if action == input.None {
				continue
			}
			if opts.verbose {
				fmt.Fprintf(os.Stderr, "heya: action %s on %s\n", action, l.Path())
			}
			command := l.Handle(action)
			switch command.Kind {
			case launcher.CmdQuit:
				return nil
			case launcher.CmdExec:
				screen.Close()
				screen = nil
				runApp(command.Exec, command.Args, command.Environment, command.WorkingDirectory, opts)
				reopened, err := openUI(theme, icons, opts)
				if err != nil {
					return err
				}
				screen = reopened
			}
		}
		sdl.Delay(16)
	}
}

// mergeEnvironment applies app entries over the inherited process environment.
func mergeEnvironment(base, overrides []string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = item
	}
	for _, item := range overrides {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = item
	}
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, values[key])
	}
	return result
}

func runApp(path string, args []string, environment []string, directory string, opts options) {
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "heya: launching %s %v\n", path, args)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeEnvironment(os.Environ(), environment)
	cmd.Dir = directory
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "heya: %s: %v\n", path, err)
		// Give the user a moment to read the failure before the UI returns.
		time.Sleep(1500 * time.Millisecond)
	}
}

// dumpPages renders every page to PNG, driving navigation through the same
// actions the UI uses. It needs no display (SDL dummy driver).
func dumpPages(l *launcher.Launcher, theme model.Theme, icons model.Icons, opts options) error {
	if err := os.MkdirAll(opts.dumpDir, 0o755); err != nil {
		return err
	}
	os.Setenv("SDL_VIDEODRIVER", "dummy")
	screen, err := openUI(theme, icons, opts)
	if err != nil {
		return err
	}
	defer screen.Close()

	render := func(name string) error {
		if err := screen.Render(l.Frame()); err != nil {
			return err
		}
		path := filepath.Join(opts.dumpDir, name+".png")
		if err := screen.SavePNG(path); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
		return nil
	}

	if err := render("01-apps"); err != nil {
		return err
	}

	l.Handle(input.Down)
	l.Handle(input.Confirm)
	l.Handle(input.Down)
	if err := render("02-system"); err != nil {
		return err
	}

	l.Handle(input.Cancel)
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	if err := render("03-about"); err != nil {
		return err
	}
	return nil
}
