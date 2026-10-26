package system

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProbeReadsFixtures(t *testing.T) {
	root := t.TempDir()
	write(t, root, "proc/sys/kernel/hostname", "handheld\n")
	write(t, root, "proc/sys/kernel/osrelease", "6.12.0-test\n")
	write(t, root, "proc/uptime", "90061.22 0.00\n")
	write(t, root, "proc/meminfo", "MemTotal:        2048000 kB\nMemAvailable:     512000 kB\n")
	write(t, root, "sys/class/power_supply/battery/capacity", "77\n")
	write(t, root, "sys/class/power_supply/battery/status", "Charging\n")
	write(t, root, "sys/class/thermal/thermal_zone0/temp", "43000\n")
	write(t, root, "sys/class/backlight/panel/brightness", "128\n")
	write(t, root, "sys/class/backlight/panel/max_brightness", "255\n")
	if err := os.MkdirAll(filepath.Join(root, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}

	probe := &Probe{Root: root, Run: func(string, ...string) (string, error) { return "", os.ErrNotExist }}
	status := probe.Read()

	if status.Hostname != "handheld" {
		t.Fatalf("hostname = %q", status.Hostname)
	}
	if status.Kernel != "6.12.0-test" {
		t.Fatalf("kernel = %q", status.Kernel)
	}
	if status.Uptime != "1d 01:01" {
		t.Fatalf("uptime = %q", status.Uptime)
	}
	if !status.BatteryKnown || status.BatteryPercent != 77 {
		t.Fatalf("battery = %+v", status)
	}
	if !status.ChargingKnown || !status.Charging {
		t.Fatalf("charging = %+v", status)
	}
	if !status.TempKnown || status.TempC != 43 {
		t.Fatalf("temp = %+v", status)
	}
	if status.MemTotalMB != 2000 || status.MemAvailableMB != 500 {
		t.Fatalf("memory = %d/%d", status.MemAvailableMB, status.MemTotalMB)
	}
	if !status.StorageKnown || status.StorageTotalMB == 0 {
		t.Fatalf("storage = %+v", status)
	}
	if !status.BrightnessKnown || status.Brightness != 50 {
		t.Fatalf("brightness = %+v", status)
	}
	if status.WifiKnown {
		t.Fatalf("wifi should be unknown without wl* interface: %+v", status)
	}
}

func TestProbeDegradesWhenFilesMissing(t *testing.T) {
	root := t.TempDir()
	probe := &Probe{Root: root, Run: func(string, ...string) (string, error) { return "", os.ErrNotExist }}
	status := probe.Read()
	if status.BatteryKnown || status.TempKnown || status.VolumeKnown {
		t.Fatalf("expected unknown values, got %+v", status)
	}
}

func TestProbeReadsWifiFromRfkill(t *testing.T) {
	root := t.TempDir()
	probe := &Probe{Root: root, Run: func(name string, _ ...string) (string, error) {
		if name == "rfkill" {
			return "0: hci0: Bluetooth\n\tSoft blocked: yes\n\tHard blocked: no\n1: wlan0: Wireless LAN\n\tSoft blocked: no\n\tHard blocked: no\n", nil
		}
		return "", os.ErrNotExist
	}}
	status := probe.Read()
	if !status.WifiKnown || !status.WifiOn {
		t.Fatalf("wifi = %+v", status)
	}
	if !status.BtKnown || status.BtOn {
		t.Fatalf("bluetooth = %+v", status)
	}
}
