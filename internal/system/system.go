// Package system reads device state (battery, temperature, memory, storage,
// network, volume, brightness) and applies the few controls heya needs.
//
// Every probe degrades gracefully: a missing file or command yields
// "unknown" rather than a fabricated value, so the UI can show Unavailable.
package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iamcheyan/heya/internal/capabilities"
)

// Status is one snapshot of device state.
type Status struct {
	Hostname string
	Kernel   string
	Uptime   string

	BatteryPercent int
	BatteryKnown   bool
	Charging       bool
	ChargingKnown  bool

	TempC     float64
	TempKnown bool

	MemAvailableMB uint64
	MemTotalMB     uint64

	StorageFreeMB  uint64
	StorageTotalMB uint64
	StorageKnown   bool

	IP        string
	WifiOn    bool
	WifiKnown bool
	BtOn      bool
	BtKnown   bool

	Volume          int
	VolumeKnown     bool
	Brightness      int
	BrightnessKnown bool

	Zram string
}

// Probe reads state from a filesystem root (tests point it at fixtures).
type Probe struct {
	Root string
	Run  func(name string, args ...string) (string, error)
}

// NewProbe returns a probe bound to the real system.
func NewProbe() *Probe {
	return &Probe{Root: "/", Run: runCommand}
}

func runCommand(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *Probe) path(rel string) string {
	return filepath.Join(p.Root, rel)
}

func (p *Probe) readFile(rel string) (string, bool) {
	data, err := os.ReadFile(p.path(rel))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

func (p *Probe) firstGlob(pattern string) (string, bool) {
	matches, err := filepath.Glob(p.path(pattern))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// Read collects a status snapshot.
func (p *Probe) Read() Status {
	var s Status

	if hostname, ok := p.readFile("proc/sys/kernel/hostname"); ok {
		s.Hostname = hostname
	}
	if release, ok := p.readFile("proc/sys/kernel/osrelease"); ok {
		s.Kernel = release
	}
	if uptime, ok := p.readFile("proc/uptime"); ok {
		if fields := strings.Fields(uptime); len(fields) > 0 {
			if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
				d := time.Duration(seconds) * time.Second
				s.Uptime = fmt.Sprintf("%dd %02d:%02d", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60)
			}
		}
	}

	p.readBattery(&s)
	p.readTemperature(&s)
	p.readMemory(&s)
	p.readStorage(&s)
	p.readNetwork(&s)
	p.readVolume(&s)
	p.readBrightness(&s)
	p.readZram(&s)
	return s
}

func (p *Probe) readBattery(s *Status) {
	if value, ok := p.readFile("tmp/battery.percent"); ok {
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 100 {
			s.BatteryPercent, s.BatteryKnown = n, true
		}
	}
	supplies, _ := filepath.Glob(p.path("sys/class/power_supply/*"))
	for _, supply := range supplies {
		read := func(name string) string {
			b, _ := os.ReadFile(filepath.Join(supply, name))
			return strings.TrimSpace(string(b))
		}
		if kind := read("type"); kind != "" && kind != "Battery" {
			continue
		}
		values := map[string]string{}
		for _, line := range strings.Split(read("uevent"), "\n") {
			key, value, _ := strings.Cut(line, "=")
			values[key] = value
		}
		capacity := read("capacity")
		if capacity == "" {
			capacity = values["POWER_SUPPLY_CAPACITY"]
		}
		if capacity == "" {
			continue
		}
		if n, err := strconv.Atoi(capacity); err == nil && n >= 0 && n <= 100 && !s.BatteryKnown {
			s.BatteryPercent, s.BatteryKnown = n, true
		}
		status := read("status")
		if status == "" {
			status = values["POWER_SUPPLY_STATUS"]
		}
		if status != "" {
			s.Charging = status == "Charging" || status == "Full"
			s.ChargingKnown = true
		}
		break
	}
}

func (p *Probe) readTemperature(s *Status) {
	path, ok := p.firstGlob("sys/class/thermal/thermal_zone*/temp")
	if !ok {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	milli, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil {
		return
	}
	s.TempC = milli / 1000.0
	s.TempKnown = true
}

func (p *Probe) readMemory(s *Status) {
	file, err := os.Open(p.path("proc/meminfo"))
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			s.MemTotalMB = value / 1024
		case "MemAvailable:":
			s.MemAvailableMB = value / 1024
		}
	}
}

func (p *Probe) readStorage(s *Status) {
	target := p.path("storage")
	if _, err := os.Stat(target); err != nil {
		target = p.Root
	}
	var stat syscallStatfs
	if err := statfs(target, &stat); err != nil {
		return
	}
	s.StorageFreeMB = stat.freeMB()
	s.StorageTotalMB = stat.totalMB()
	s.StorageKnown = true
}

func (p *Probe) readNetwork(s *Status) {
	matches, err := filepath.Glob(p.path("sys/class/net/*"))
	if err == nil {
		for _, iface := range matches {
			name := filepath.Base(iface)
			if name == "lo" {
				continue
			}
			if !strings.HasPrefix(name, "wl") {
				continue
			}
			if state, err := os.ReadFile(filepath.Join(iface, "operstate")); err == nil {
				if strings.TrimSpace(string(state)) == "up" {
					s.WifiOn = true
					s.WifiKnown = true
				}
			}
		}
	}

	if out, err := p.Run("rfkill", "list"); err == nil {
		section := ""
		for _, line := range strings.Split(out, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Soft blocked:") {
				blocked := strings.Contains(trimmed, "yes")
				lower := strings.ToLower(section)
				switch {
				case strings.Contains(lower, "wlan"):
					s.WifiOn = !blocked
					s.WifiKnown = true
				case strings.Contains(lower, "bluetooth"):
					s.BtOn = !blocked
					s.BtKnown = true
				}
			}
			if trimmed != "" && !strings.HasPrefix(trimmed, "Soft blocked") && !strings.HasPrefix(trimmed, "Hard blocked") {
				section = trimmed
			}
		}
	}

	if s.BtKnown {
		out, err := p.Run("systemctl", "is-active", "bluetooth.service")
		s.BtOn = s.BtOn && err == nil && out == "active"
	}

	if out, err := p.Run("ip", "-4", "addr", "show", "up"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "inet" && !strings.HasPrefix(fields[1], "127.") {
				s.IP = strings.Split(fields[1], "/")[0]
				break
			}
		}
	}
}

func (p *Probe) readVolume(s *Status) {
	if out, err := p.Run("pactl", "get-sink-volume", "@DEFAULT_SINK@"); err == nil {
		for _, token := range strings.Fields(out) {
			if strings.HasSuffix(token, "%") {
				if v, err := strconv.Atoi(strings.TrimSuffix(token, "%")); err == nil {
					s.Volume = v
					s.VolumeKnown = true
					return
				}
			}
		}
	}
	if out, err := p.Run("amixer", "get", "Master"); err == nil {
		for _, token := range strings.Fields(out) {
			if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "%]") {
				if v, err := strconv.Atoi(strings.Trim(token, "[]%")); err == nil {
					s.Volume = v
					s.VolumeKnown = true
					return
				}
			}
		}
	}
}

func (p *Probe) readBrightness(s *Status) {
	path, ok := p.firstGlob("sys/class/backlight/*/brightness")
	if !ok {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	current, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return
	}
	maxRaw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "max_brightness"))
	if err != nil {
		return
	}
	max, err := strconv.Atoi(strings.TrimSpace(string(maxRaw)))
	if err != nil || max == 0 {
		return
	}
	s.Brightness = current * 100 / max
	s.BrightnessKnown = true
}

func (p *Probe) readZram(s *Status) {
	if out, err := p.Run("rocknix-memory-manager", "--get-zram"); err == nil && out != "" {
		s.Zram = strings.Join(strings.Fields(out), " / ")
	}
}

// SupportsWiFi reports whether a wireless network interface was detected.
func (p *Probe) SupportsWiFi() bool {
	matches, _ := filepath.Glob(p.path("sys/class/net/wl*"))
	return len(matches) > 0
}

// SupportsBluetooth reports whether rfkill exposes Bluetooth support.
func (p *Probe) SupportsBluetooth() bool {
	out, err := p.Run("rfkill", "list")
	return err == nil && strings.Contains(strings.ToLower(out), "bluetooth")
}

// SupportsBrightness reports whether a backlight control is present.
func (p *Probe) SupportsBrightness() bool {
	_, ok := p.firstGlob("sys/class/backlight/*/brightness")
	return ok
}

// SupportsSuspend reports whether the kernel exports suspend states.
func (p *Probe) SupportsSuspend() bool { _, ok := p.readFile("sys/power/state"); return ok }

// SupportsBattery reports whether a power supply capacity is available.
func (p *Probe) SupportsBattery() bool {
	_, ok := p.firstGlob("sys/class/power_supply/*/capacity")
	return ok
}

var _ capabilities.Device = (*Probe)(nil)

// HasCommand reports whether a binary is available on this system.
func (p *Probe) HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
