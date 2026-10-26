package launcher

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/iamcheyan/heya/internal/connectivity"
	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/ui"
)

type networkResult struct {
	generation int
	kind       string
	aps        []connectivity.AccessPoint
	devices    []connectivity.Device
	prompt     *connectivity.Prompt
	err        error
}
type networkUI struct {
	backend    connectivity.Backend
	aps        []connectivity.AccessPoint
	devices    []connectivity.Device
	ap         connectivity.AccessPoint
	device     connectivity.Device
	results    chan networkResult
	generation int
	cancel     context.CancelFunc
	busy       bool
	setting    bool
	cancelling bool
	prompt     *connectivity.Prompt
	editor     *textEditor
}
type textEditor struct {
	title, text     string
	numeric         bool
	page, row, col  int
	submit          func(string)
}

func (l *Launcher) SetConnectivity(backend connectivity.Backend) {
	l.network = &networkUI{backend: backend, results: make(chan networkResult, 16)}
}
func (l *Launcher) net() *networkUI {
	if l.network == nil {
		l.SetConnectivity(connectivity.Client{})
	}
	return l.network
}
func (l *Launcher) startNetworkJob(kind string, fn func(context.Context) networkResult) {
	n := l.net()
	if n.busy {
		return
	}
	n.generation++
	generation := n.generation
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	n.cancel = cancel
	n.busy = true
	n.setting = kind == "setting"
	l.message = "Working... [B] Cancel"
	go func() {
		defer cancel()
		result := fn(ctx)
		result.generation = generation
		result.kind = kind
		select {
		case n.results <- result:
		case <-ctx.Done():
			// Always deliver completion, including an expired operation.
			select {
			case n.results <- result:
			default:
			}
		}
	}()
}
func (l *Launcher) Poll() {
	n := l.network
	if n == nil {
		return
	}
	for {
		select {
		case r := <-n.results:
			if r.generation != n.generation {
				continue
			}
			if r.prompt != nil {
				if n.cancelling {
					continue
				}
				if r.prompt.Canceled {
					n.prompt = nil
					n.editor = nil
					l.message = "Pairing canceled by device"
					continue
				}
				n.prompt = r.prompt
				if r.prompt.Entry {
					prompt := r.prompt
					n.editor = &textEditor{title: prompt.Title, numeric: prompt.Numeric, submit: func(text string) { prompt.Reply <- text; n.prompt = nil }}
				}
				continue
			}
			if n.cancelling {
				n.cancelling = false
				n.busy = false
				n.cancel = nil
				n.generation++
				l.message = "Canceled"
				continue
			}
			n.busy = false
			n.cancel = nil
			n.prompt = nil
			n.editor = nil
			if r.err != nil {
				l.message = "Failed: " + r.err.Error() + " [SELECT] Retry"
				continue
			}
			switch r.kind {
			case "wifi-scan":
				n.aps = r.aps
				for _, ap := range r.aps {
					if ap.SSID == n.ap.SSID && ap.Security == n.ap.Security {
						n.ap = ap
					}
				}
				l.message = fmt.Sprintf("%d networks [A] Open [SELECT] Scan", len(r.aps))
			case "bt-scan":
				n.devices = r.devices
				for _, device := range r.devices {
					if device.Path == n.device.Path {
						n.device = device
					}
				}
				l.message = fmt.Sprintf("%d devices [A] Open [SELECT] Scan", len(r.devices))
			default:
				l.message = "Done [SELECT] Refresh"
				l.RefreshStatus()
				if l.currentPage() == "wifi-device" || l.currentPage() == "bluetooth-device" {
					l.stack = l.stack[:len(l.stack)-1]
					l.scanNetwork()
				}
			}
			l.clampOffset(l.currentPage(), len(l.rows(l.currentPage())))
		default:
			return
		}
	}
}
func (l *Launcher) Close() {
	if n := l.network; n != nil && n.cancel != nil {
		n.cancel()
		n.generation++
	}
}
func (l *Launcher) networkPage() bool {
	switch l.currentPage() {
	case "wifi", "wifi-device", "bluetooth", "bluetooth-device":
		return true
	}
	return false
}
func (l *Launcher) pushNetwork(page string) {
	l.net()
	l.stack = append(l.stack, page)
	l.index[page] = 0
	l.offset[page] = 0
	if page == "wifi" || page == "bluetooth" {
		l.scanNetwork()
	}
}
func (l *Launcher) scanNetwork() {
	n := l.net()
	if l.currentPage() == "wifi" || l.currentPage() == "wifi-device" {
		l.startNetworkJob("wifi-scan", func(ctx context.Context) networkResult {
			aps, err := n.backend.WiFi(ctx)
			return networkResult{aps: aps, err: err}
		})
	} else if l.currentPage() == "bluetooth" || l.currentPage() == "bluetooth-device" {
		l.startNetworkJob("bt-scan", func(ctx context.Context) networkResult {
			devices, err := n.backend.Bluetooth(ctx)
			return networkResult{devices: devices, err: err}
		})
	}
}
func (l *Launcher) networkRows(page string) []ui.Row {
	n := l.net()
	rows := []ui.Row{}
	switch page {
	case "wifi":
		for _, ap := range n.aps {
			state := ap.Security
			if ap.Connected {
				state = "Connected"
			} else if ap.Saved {
				state += " / Saved"
			}
			rows = append(rows, ui.Row{Label: ap.SSID, Value: fmt.Sprintf("%d%% %s", ap.Strength, state)})
		}
	case "bluetooth":
		for _, d := range n.devices {
			state := d.Address
			if d.Connected {
				state = "Connected"
			} else if d.Paired {
				state = "Paired"
			}
			rows = append(rows, ui.Row{Label: d.Name, Value: state})
		}
	case "wifi-device":
		rows = []ui.Row{{Label: n.ap.SSID, Value: n.ap.Security}, {Label: "Connect / reconnect"}, {Label: "Enter / change password"}, {Label: "Disconnect"}, {Label: "Forget saved network"}}
	case "bluetooth-device":
		rows = []ui.Row{{Label: n.device.Name, Value: n.device.Address}, {Label: "Pair and connect"}, {Label: "Disconnect"}, {Label: "Remove pairing"}}
	}
	if len(rows) == 0 {
		rows = append(rows, ui.Row{Label: "No results", Value: "SELECT to scan"})
	}
	return rows
}
func (l *Launcher) handleNetwork(action input.Action) bool {
	n := l.net()
	if n.editor != nil {
		l.handleEditor(action)
		return true
	}
	if n.prompt != nil {
		if action == input.Cancel {
			l.cancelNetwork()
		} else if action == input.Confirm && !n.prompt.Display {
			if l.confirm != nil {
				confirm := l.confirm
				l.confirm = nil
				confirm()
			} else {
				n.prompt.Reply <- "yes"
				n.prompt = nil
			}
		}
		return true
	}
	if n.busy {
		if n.setting {
			l.message = "Applying setting..."
			return true
		}
		if action == input.Cancel {
			l.cancelNetwork()
		}
		return true
	}
	if action == input.Select {
		l.scanNetwork()
		return true
	}
	if action != input.Confirm {
		return false
	}
	index := l.index[l.currentPage()]
	switch l.currentPage() {
	case "wifi":
		if index < len(n.aps) {
			n.ap = n.aps[index]
			l.pushNetwork("wifi-device")
		}
	case "bluetooth":
		if index < len(n.devices) {
			n.device = n.devices[index]
			l.pushNetwork("bluetooth-device")
		}
	case "wifi-device":
		switch index {
		case 1:
			if !n.ap.Saved && n.ap.Security != "Open" && n.ap.Security != "OWE" {
				l.passwordEditor()
			} else {
				l.connectWiFi("")
			}
		case 2:
			l.passwordEditor()
		case 3:
			ap := n.ap
			l.startNetworkJob("disconnect", func(ctx context.Context) networkResult { return networkResult{err: n.backend.DisconnectWiFi(ctx, ap)} })
		case 4:
			ap := n.ap
			l.confirmOperation("Forget network?", func() {
				l.startNetworkJob("forget", func(ctx context.Context) networkResult { return networkResult{err: n.backend.ForgetWiFi(ctx, ap)} })
			})
		}
	case "bluetooth-device":
		d := n.device
		switch index {
		case 1:
			generation := n.generation + 1
			l.startNetworkJob("pair", func(ctx context.Context) networkResult {
				// Only this operation sends pairing prompts, tagged to reject late replies.
				err := n.backend.Pair(ctx, d, func(p connectivity.Prompt) {
					select {
					case n.results <- networkResult{generation: generation, prompt: &p}:
					case <-ctx.Done():
					}
				})
				return networkResult{err: err}
			})
		case 2:
			l.startNetworkJob("disconnect", func(ctx context.Context) networkResult {
				return networkResult{err: n.backend.DisconnectBluetooth(ctx, d)}
			})
		case 3:
			l.confirmOperation("Remove pairing?", func() {
				l.startNetworkJob("forget", func(ctx context.Context) networkResult { return networkResult{err: n.backend.ForgetBluetooth(ctx, d)} })
			})
		}
	}
	return true
}
func (l *Launcher) cancelNetwork() {
	n := l.net()
	l.confirm = nil
	n.prompt = nil
	n.editor = nil
	if n.busy && n.cancel != nil {
		n.cancelling = true
		n.cancel()
		l.message = "Canceling..."
		return
	}
	n.generation++
	n.cancel = nil
	l.message = "Canceled"
}

func (l *Launcher) confirmOperation(title string, yes func()) {
	n := l.net()
	reply := make(chan string, 1)
	n.prompt = &connectivity.Prompt{Title: title, Text: "[A] Confirm   [B] Cancel", Reply: reply}
	// Confirmation has no worker: Poll checks it on the UI thread.
	l.confirm = func() { n.prompt = nil; yes() }
}
func (l *Launcher) passwordEditor() {
	n := l.net()
	if n.ap.Security != "WPA/WPA2" && n.ap.Security != "WPA3" {
		l.message = "This security type needs a system profile"
		return
	}
	n.editor = &textEditor{title: "Wi-Fi password", submit: l.connectWiFi}
	l.message = ""
}
func (l *Launcher) connectWiFi(password string) {
	n := l.net()
	ap := n.ap
	l.startNetworkJob("connect", func(ctx context.Context) networkResult {
		return networkResult{err: n.backend.ConnectWiFi(ctx, ap, password)}
	})
}
func (l *Launcher) adjustSetting(action input.Action) bool {
	rows := l.rows("system")
	idx := l.index["system"]
	if idx >= len(rows) {
		return false
	}
	label := rows[idx].Label
	if action == input.Confirm && (label == "Wi-Fi" || label == "Bluetooth") {
		if label == "Wi-Fi" {
			l.pushNetwork("wifi")
		} else {
			l.pushNetwork("bluetooth")
		}
		return true
	}
	if action != input.Left && action != input.Right {
		return false
	}
	s := l.status
	control := ""
	level := 0
	known := false
	value := ""
	switch label {
	case "Wi-Fi":
		control = "wifi"
		known = s.WifiKnown
	case "Bluetooth":
		control = "bluetooth"
		known = s.BtKnown
	case "Volume":
		control = "volume"
		level = s.Volume
		known = s.VolumeKnown
	case "Brightness":
		control = "brightness"
		level = s.Brightness
		known = s.BrightnessKnown
	default:
		return false
	}
	if !known || l.probe == nil {
		l.message = "Control unavailable on this device"
		return true
	}
	if control == "wifi" || control == "bluetooth" {
		value = "off"
		if action == input.Right {
			value = "on"
		}
	} else {
		if action == input.Left {
			level -= 5
		} else {
			level += 5
		}
		minimum := 0
		if control == "brightness" {
			minimum = 5
		}
		if level < minimum {
			level = minimum
		}
		if level > 100 {
			level = 100
		}
		value = strconv.Itoa(level)
	}
	probe := l.probe
	l.startNetworkJob("setting", func(ctx context.Context) networkResult { return networkResult{err: probe.Control(control, value)} })
	return true
}

var keyboardPages = []string{
	"abcdefghijklmnopqrstuvwxyz0123456789-_. ",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_. ",
	"!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~0123456789",
}

func (e *textEditor) keys() []rune {
	if e.numeric {
		return []rune("0123456789")
	}
	return []rune(keyboardPages[e.page])
}
func (e *textEditor) rows() []ui.Row {
	text := e.text
	if text == "" {
		text = "(empty)"
	}
	rows := []ui.Row{{Label: "Input", Value: text}}
	keys := e.keys()
	count := (len(keys) + 9) / 10
	for row := 0; row < count; row++ {
		cells := []string{}
		for col := 0; col < 10 && row*10+col < len(keys); col++ {
			key := string(keys[row*10+col])
			if key == " " {
				key = "SP"
			}
			cells = append(cells, key)
		}
		selected := -1
		if row == e.row {
			selected = e.col
		}
		rows = append(rows, ui.Row{Cells: cells, CellSelected: selected})
	}
	selected := -1
	if e.row == count {
		selected = e.col
	}
	rows = append(rows, ui.Row{Cells: []string{"DEL", "SPACE", "a/A", "#+=", "OK"}, CellSelected: selected})
	return rows
}
func (l *Launcher) TextEntryActive() bool { return l.network != nil && l.network.editor != nil }
func (l *Launcher) TextInput(text string) {
	if !l.TextEntryActive() {
		return
	}
	e := l.network.editor
	for _, r := range text {
		if r < 32 || r > 126 || (e.numeric && (r < '0' || r > '9')) {
			continue
		}
		if len(e.text) < 128 {
			e.text += string(r)
		}
	}
}
func (l *Launcher) DeleteText() {
	if l.TextEntryActive() {
		e := l.network.editor
		r := []rune(e.text)
		if len(r) > 0 {
			e.text = string(r[:len(r)-1])
		}
	}
}
func (l *Launcher) handleEditor(action input.Action) {
	n := l.net()
	e := n.editor
	keys := e.keys()
	count := (len(keys) + 9) / 10
	switch action {
	case input.Cancel:
		if n.busy {
			l.cancelNetwork()
		} else {
			n.editor = nil
		}
		return
	case input.Select:
		l.DeleteText()
	case input.PageUp:
		if !e.numeric {
			e.page = (e.page + 1) % 2
			e.row = 0
			e.col = 0
		}
	case input.PageDown:
		if !e.numeric {
			if e.page == 2 {
				e.page = 0
			} else {
				e.page = 2
			}
			e.row = 0
			e.col = 0
		}
	case input.Up:
		e.row = (e.row + count) % (count + 1)
	case input.Down:
		e.row = (e.row + 1) % (count + 1)
	case input.Left:
		e.col--
	case input.Right:
		e.col++
	case input.Start:
		l.submitEditor()
		return
	case input.Confirm:
		if e.row == count {
			switch e.col {
			case 0:
				l.DeleteText()
			case 1:
				if !e.numeric {
					l.TextInput(" ")
				}
			case 2:
				if !e.numeric {
					e.page = (e.page + 1) % 2
					e.row = 0
				}
			case 3:
				if !e.numeric {
					e.page = 2
					e.row = 0
				}
			case 4:
				l.submitEditor()
				return
			}
		} else if index := e.row*10 + e.col; index < len(keys) {
			l.TextInput(string(keys[index]))
		}
	}
	width := 10
	if e.row == count {
		width = 5
	} else if (e.row+1)*10 > len(e.keys()) {
		width = len(e.keys()) - e.row*10
	}
	if width < 1 {
		width = 1
	}
	e.col = (e.col%width + width) % width
}
func (l *Launcher) submitEditor() {
	n := l.net()
	e := n.editor
	if e == nil {
		return
	}
	text := e.text
	e.text = ""
	n.editor = nil
	e.submit(text)
}
