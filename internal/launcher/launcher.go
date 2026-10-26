// Package launcher owns navigation state and turns input actions into
// commands. It never renders and never touches the filesystem directly.
package launcher

import (
	"fmt"
	"strings"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/model"
	"github.com/iamcheyan/heya/internal/system"
	"github.com/iamcheyan/heya/internal/ui"
)

// CommandKind tells the host what to do after an action.
type CommandKind int

const (
	CmdNone CommandKind = iota
	CmdExec
	CmdQuit
)

// Command is the launcher's output.
type Command struct {
	Kind             CommandKind
	Exec             string
	Args             []string
	Environment      []string
	WorkingDirectory string
}

// Launcher holds the page stack and selection.
type Launcher struct {
	apps    model.Page
	pages   map[string]model.Page
	stack   []string
	index   map[string]int
	offset  map[string]int
	probe   *system.Probe
	status  system.Status
	icons   model.Icons
	message string
	Refresh func() model.Page
}

// New builds a launcher with the apps page as root.
func New(apps model.Page, probe *system.Probe, icons model.Icons) *Launcher {
	l := &Launcher{
		apps:   apps,
		pages:  map[string]model.Page{},
		stack:  []string{"apps"},
		index:  map[string]int{"apps": 0},
		offset: map[string]int{"apps": 0},
		probe:  probe,
		icons:  icons,
	}
	if probe != nil {
		l.status = probe.Read()
	}
	return l
}

// AddPage registers a static page.
func (l *Launcher) AddPage(id string, page model.Page) {
	l.pages[id] = page
	l.index[id] = 0
	l.offset[id] = 0
}

// SetApps replaces the root page (used after a rescan).
func (l *Launcher) SetApps(page model.Page) {
	l.apps = page
	if idx := l.index["apps"]; idx >= len(l.rows("apps")) {
		l.index["apps"] = 0
	}
}

// currentPage returns the page id on top of the stack.
func (l *Launcher) currentPage() string {
	return l.stack[len(l.stack)-1]
}

// rows returns the display rows for a page.
func (l *Launcher) rows(pageID string) []ui.Row {
	if pageID == "apps" {
		rows := make([]ui.Row, 0, len(l.apps.Items))
		for _, item := range l.apps.Items {
			value := pageHint(item)
			if item.Exec != "" {
				value = strings.Join(item.Permissions, ",")
				if len(item.Features) > 0 {
					if value != "" {
						value += " · "
					}
					value += strings.Join(item.Features, ",")
				}
			}
			rows = append(rows, ui.Row{Label: item.Name, Icon: l.glyph(item.Icon), Value: value})
		}
		return rows
	}
	if pageID == "system" {
		return l.systemRows()
	}
	page, ok := l.pages[pageID]
	if !ok {
		return nil
	}
	rows := make([]ui.Row, 0, len(page.Items))
	for _, item := range page.Items {
		rows = append(rows, ui.Row{Label: item.Name, Icon: l.glyph(item.Icon), Value: item.Value})
	}
	return rows
}

func (l *Launcher) glyph(name string) rune {
	if name == "" || l.icons == nil {
		return 0
	}
	if glyph := l.icons.Glyph(name); glyph != 0 {
		return glyph
	}
	return 0
}

func pageHint(item model.Item) string {
	if item.Page != "" {
		return "›"
	}
	return ""
}

func (l *Launcher) systemRows() []ui.Row {
	s := l.status
	unknown := "Unavailable"

	battery := unknown
	if s.BatteryKnown {
		battery = fmt.Sprintf("%d%%", s.BatteryPercent)
	}
	charging := unknown
	if s.ChargingKnown {
		if s.Charging {
			charging = "Charging"
		} else {
			charging = "On battery"
		}
	}
	temp := unknown
	if s.TempKnown {
		temp = fmt.Sprintf("%.1f C", s.TempC)
	}
	memory := unknown
	if s.MemTotalMB > 0 {
		memory = fmt.Sprintf("%d / %d MB", s.MemAvailableMB, s.MemTotalMB)
	}
	storage := unknown
	if s.StorageKnown {
		storage = fmt.Sprintf("%d / %d MB free", s.StorageFreeMB, s.StorageTotalMB)
	}
	ip := s.IP
	if ip == "" {
		ip = "Offline"
	}
	wifi := unknown
	if s.WifiKnown {
		wifi = onOff(s.WifiOn)
	}
	bt := unknown
	if s.BtKnown {
		bt = onOff(s.BtOn)
	}
	volume := unknown
	if s.VolumeKnown {
		volume = fmt.Sprintf("%d%%", s.Volume)
	}
	brightness := unknown
	if s.BrightnessKnown {
		brightness = fmt.Sprintf("%d%%", s.Brightness)
	}
	zram := s.Zram
	if zram == "" {
		zram = unknown
	}
	uptime := s.Uptime
	if uptime == "" {
		uptime = unknown
	}
	kernel := s.Kernel
	if kernel == "" {
		kernel = unknown
	}

	return []ui.Row{
		{Label: "Battery", Value: battery},
		{Label: "Power", Value: charging},
		{Label: "CPU temperature", Value: temp},
		{Label: "Memory", Value: memory},
		{Label: "Storage", Value: storage},
		{Label: "IP address", Value: ip},
		{Label: "Wi-Fi", Value: wifi},
		{Label: "Bluetooth", Value: bt},
		{Label: "Volume", Value: volume},
		{Label: "Brightness", Value: brightness},
		{Label: "ZRAM", Value: zram},
		{Label: "Uptime", Value: uptime},
		{Label: "Kernel", Value: kernel},
	}
}

func onOff(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

// RefreshStatus re-reads device state.
func (l *Launcher) RefreshStatus() {
	if l.probe != nil {
		l.status = l.probe.Read()
	}
}

// Title returns the current page title.
func (l *Launcher) Title() string {
	switch l.currentPage() {
	case "apps":
		if l.apps.Title != "" {
			return l.apps.Title
		}
		return "Applications"
	case "system":
		return "System"
	default:
		if page, ok := l.pages[l.currentPage()]; ok && page.Title != "" {
			return page.Title
		}
		return l.currentPage()
	}
}

// Frame renders the current state into a UI frame.
func (l *Launcher) Frame() ui.Frame {
	pageID := l.currentPage()
	rows := l.rows(pageID)
	selected := l.index[pageID]
	if selected >= len(rows) {
		selected = len(rows) - 1
	}
	if selected < 0 {
		selected = 0
	}

	status := []string{"HEYA"}
	if l.status.Hostname != "" {
		status = append(status, l.status.Hostname)
	}
	if l.status.BatteryKnown {
		status = append(status, fmt.Sprintf("BAT %d%%", l.status.BatteryPercent))
	}
	if l.status.IP != "" {
		status = append(status, l.status.IP)
	}

	hint := "[A] Select   [B] Back   [D-PAD] Move   [SELECT] Rescan"
	if pageID == "system" {
		hint = "[B] Back   [SELECT] Refresh"
	}

	return ui.Frame{
		Title:    l.Title(),
		Rows:     rows,
		Selected: selected,
		Offset:   l.offset[pageID],
		Status:   status,
		Hint:     hint,
		Message:  l.message,
	}
}

// Handle applies an input action and returns a command for the host.
func (l *Launcher) Handle(action input.Action) Command {
	pageID := l.currentPage()
	rows := l.rows(pageID)
	count := len(rows)

	l.message = ""

	switch action {
	case input.Up:
		if count > 0 {
			l.index[pageID] = (l.index[pageID] - 1 + count) % count
		}
	case input.Down:
		if count > 0 {
			l.index[pageID] = (l.index[pageID] + 1) % count
		}
	case input.PageUp:
		if count > 0 {
			l.index[pageID] -= 5
			if l.index[pageID] < 0 {
				l.index[pageID] = 0
			}
		}
	case input.PageDown:
		if count > 0 {
			l.index[pageID] += 5
			if l.index[pageID] >= count {
				l.index[pageID] = count - 1
			}
		}
	case input.Select:
		if l.Refresh != nil {
			l.SetApps(l.Refresh())
			l.RefreshStatus()
			l.message = fmt.Sprintf("Rescanned: %d apps", len(l.apps.Items))
		}
	case input.Confirm:
		if count == 0 {
			return Command{}
		}
		item := l.currentItem(pageID, l.index[pageID])
		if item == nil {
			return Command{}
		}
		if item.Page != "" {
			l.stack = append(l.stack, item.Page)
			l.index[item.Page] = 0
			l.offset[item.Page] = 0
			if item.Page == "system" {
				l.RefreshStatus()
			}
			return Command{}
		}
		if item.Exec != "" {
			return Command{Kind: CmdExec, Exec: item.Exec, Args: item.Args, Environment: item.Environment, WorkingDirectory: item.WorkingDirectory}
		}
	case input.Cancel:
		if len(l.stack) > 1 {
			l.stack = l.stack[:len(l.stack)-1]
			return Command{}
		}
		return Command{Kind: CmdQuit}
	case input.Quit:
		return Command{Kind: CmdQuit}
	}

	l.clampOffset(pageID, count)
	return Command{}
}

// currentItem resolves the selected item for the current page.
func (l *Launcher) currentItem(pageID string, index int) *model.Item {
	if pageID == "apps" {
		if index < 0 || index >= len(l.apps.Items) {
			return nil
		}
		item := l.apps.Items[index]
		return &item
	}
	page, ok := l.pages[pageID]
	if !ok || index < 0 || index >= len(page.Items) {
		return nil
	}
	item := page.Items[index]
	return &item
}

func (l *Launcher) clampOffset(pageID string, count int) {
	const visible = 6
	index := l.index[pageID]
	offset := l.offset[pageID]
	if index < offset {
		offset = index
	}
	if index >= offset+visible {
		offset = index - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	if max := count - visible; offset > max && max > 0 {
		offset = max
	}
	l.offset[pageID] = offset
}

// Message returns the transient status message.
func (l *Launcher) Message() string { return l.message }

// Status returns the last snapshot.
func (l *Launcher) Status() system.Status { return l.status }

// Path returns the current page stack joined for debugging.
func (l *Launcher) Path() string { return strings.Join(l.stack, ">") }
