package connectivity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/godbus/dbus/v5"
)

const nm = "org.freedesktop.NetworkManager"
const nmPath dbus.ObjectPath = "/org/freedesktop/NetworkManager"
const wireless = nm + ".Device.Wireless"
const apInterface = nm + ".AccessPoint"
const settingsInterface = nm + ".Settings.Connection"

type AccessPoint struct {
	Path, Device     dbus.ObjectPath
	SSID             string
	Strength         byte
	Security         string
	Connected, Saved bool
}

type Device struct {
	Path              dbus.ObjectPath
	Name, Address     string
	Paired, Connected bool
}

type Prompt struct {
	Title, Text                       string
	Numeric, Entry, Display, Canceled bool
	Reply                             chan string
}

type Backend interface {
	WiFi(context.Context) ([]AccessPoint, error)
	ConnectWiFi(context.Context, AccessPoint, string) error
	DisconnectWiFi(context.Context, AccessPoint) error
	ForgetWiFi(context.Context, AccessPoint) error
	Bluetooth(context.Context) ([]Device, error)
	Pair(context.Context, Device, func(Prompt)) error
	DisconnectBluetooth(context.Context, Device) error
	ForgetBluetooth(context.Context, Device) error
}

type Bus interface {
	Call(context.Context, string, dbus.ObjectPath, string, ...interface{}) ([]interface{}, error)
}

type Client struct{ Bus Bus }
type systemBus struct{ conn *dbus.Conn }

func (b systemBus) Call(ctx context.Context, dest string, path dbus.ObjectPath, method string, args ...interface{}) ([]interface{}, error) {
	call := b.conn.Object(dest, path).CallWithContext(ctx, method, 0, args...)
	return call.Body, call.Err
}

func connectBus() (*dbus.Conn, error) {
	if os.Getenv("HEYA_SIMULATOR") == "1" {
		return nil, errors.New("preview: use demo devices; system radios are unchanged")
	}
	return dbus.ConnectSystemBus()
}

func (c Client) withBus(fn func(Bus) error) error {
	if c.Bus != nil {
		return fn(c.Bus)
	}
	conn, err := connectBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(systemBus{conn})
}

func properties(ctx context.Context, b Bus, dest string, path dbus.ObjectPath, iface string) (map[string]dbus.Variant, error) {
	body, err := b.Call(ctx, dest, path, "org.freedesktop.DBus.Properties.GetAll", iface)
	if err != nil {
		return nil, err
	}
	var props map[string]dbus.Variant
	err = dbus.Store(body, &props)
	return props, err
}
func value[T any](p map[string]dbus.Variant, key string) T { v, _ := p[key].Value().(T); return v }
func paths(body []interface{}, err error) ([]dbus.ObjectPath, error) {
	if err != nil {
		return nil, err
	}
	var result []dbus.ObjectPath
	err = dbus.Store(body, &result)
	return result, err
}
func pause(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type profile struct {
	path     dbus.ObjectPath
	settings map[string]map[string]dbus.Variant
}

func profiles(ctx context.Context, b Bus) ([]profile, error) {
	list, err := paths(b.Call(ctx, nm, "/org/freedesktop/NetworkManager/Settings", nm+".Settings.ListConnections"))
	if err != nil {
		return nil, err
	}
	result := []profile{}
	for _, p := range list {
		body, err := b.Call(ctx, nm, p, settingsInterface+".GetSettings")
		if err != nil {
			return nil, err
		}
		var s map[string]map[string]dbus.Variant
		if err := dbus.Store(body, &s); err != nil {
			return nil, err
		}
		result = append(result, profile{p, s})
	}
	return result, nil
}
func matches(p profile, ap AccessPoint) bool {
	if string(value[[]byte](p.settings["802-11-wireless"], "ssid")) != ap.SSID {
		return false
	}
	security := p.settings["802-11-wireless-security"]
	key := value[string](security, "key-mgmt")
	switch ap.Security {
	case "Open":
		return key == "" && len(security) == 0
	case "OWE":
		return key == "owe"
	case "WPA/WPA2":
		return key == "wpa-psk"
	case "WPA3":
		return key == "sae"
	case "Enterprise":
		return key == "wpa-eap" || key == "wpa-eap-suite-b-192"
	case "WEP":
		return key == "none"
	}
	return false
}

func (c Client) WiFi(ctx context.Context) (result []AccessPoint, err error) {
	err = c.withBus(func(b Bus) error {
		devices, err := paths(b.Call(ctx, nm, nmPath, nm+".GetDevices"))
		if err != nil {
			return err
		}
		saved, err := profiles(ctx, b)
		if err != nil {
			return err
		}
		found := false
		for _, device := range devices {
			prop, err := properties(ctx, b, nm, device, nm+".Device")
			if err != nil {
				continue
			}
			if value[uint32](prop, "DeviceType") != 2 {
				continue
			}
			found = true
			w, err := properties(ctx, b, nm, device, wireless)
			if err != nil {
				return err
			}
			before := value[int64](w, "LastScan")
			_, scanErr := b.Call(ctx, nm, device, wireless+".RequestScan", map[string]dbus.Variant{})
			if scanErr == nil {
				for i := 0; i < 16; i++ {
					if err := pause(ctx, 500*time.Millisecond); err != nil {
						return err
					}
					w, err = properties(ctx, b, nm, device, wireless)
					if err != nil {
						return err
					}
					if value[int64](w, "LastScan") != before {
						break
					}
				}
			}
			active := value[dbus.ObjectPath](w, "ActiveAccessPoint")
			aps, err := paths(b.Call(ctx, nm, device, wireless+".GetAllAccessPoints"))
			if err != nil {
				return err
			}
			for _, path := range aps {
				prop, err := properties(ctx, b, nm, path, apInterface)
				if err != nil {
					continue
				}
				ssid := string(value[[]byte](prop, "Ssid"))
				if ssid == "" {
					continue
				}
				security := securityType(value[uint32](prop, "Flags"), value[uint32](prop, "WpaFlags"), value[uint32](prop, "RsnFlags"))
				ap := AccessPoint{Path: path, Device: device, SSID: ssid, Strength: value[byte](prop, "Strength"), Security: security, Connected: path == active}
				for _, p := range saved {
					if matches(p, ap) {
						ap.Saved = true
					}
				}
				result = append(result, ap)
			}
		}
		if !found {
			return errors.New("no Wi-Fi adapter; enable Wi-Fi first")
		}
		result = uniqueAPs(result)
		return nil
	})
	return
}

func securityType(flags, wpa, rsn uint32) string {
	combined := wpa | rsn
	if combined&0x200 != 0 {
		return "Enterprise"
	}
	if combined&0x100 != 0 {
		return "WPA/WPA2"
	}
	if combined&0x400 != 0 {
		return "WPA3"
	}
	if combined&0x800 != 0 {
		return "OWE"
	}
	if flags&1 != 0 {
		return "WEP"
	}
	return "Open"
}
func uniqueAPs(list []AccessPoint) []AccessPoint {
	seen := map[string]AccessPoint{}
	for _, ap := range list {
		key := ap.SSID + "\x00" + ap.Security
		old, exists := seen[key]
		if !exists || ap.Connected || (!old.Connected && ap.Strength > old.Strength) {
			seen[key] = ap
		}
	}
	result := make([]AccessPoint, 0, len(seen))
	for _, ap := range seen {
		result = append(result, ap)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Connected != result[j].Connected {
			return result[i].Connected
		}
		if result[i].Strength != result[j].Strength {
			return result[i].Strength > result[j].Strength
		}
		return result[i].SSID < result[j].SSID
	})
	return result
}

func wifiSettings(ap AccessPoint, password string) (map[string]map[string]dbus.Variant, error) {
	s := map[string]map[string]dbus.Variant{
		"connection":      {"id": dbus.MakeVariant(ap.SSID), "type": dbus.MakeVariant("802-11-wireless"), "autoconnect": dbus.MakeVariant(true)},
		"802-11-wireless": {"ssid": dbus.MakeVariant([]byte(ap.SSID)), "mode": dbus.MakeVariant("infrastructure")},
		"ipv4":            {"method": dbus.MakeVariant("auto")}, "ipv6": {"method": dbus.MakeVariant("auto")},
	}
	if ap.Security == "Open" {
		return s, nil
	}
	keyMgmt := "wpa-psk"
	switch ap.Security {
	case "WPA/WPA2":
		if len(password) != 64 && (len(password) < 8 || len(password) > 63) {
			return nil, errors.New("WPA password needs 8-63 characters or 64 hex digits")
		}
	case "WPA3":
		keyMgmt = "sae"
		if len(password) < 1 || len(password) > 63 {
			return nil, errors.New("WPA3 password needs 1-63 characters")
		}
	case "OWE":
		keyMgmt = "owe"
	default:
		return nil, fmt.Errorf("%s needs an existing system profile", ap.Security)
	}
	if ap.Security == "WPA/WPA2" && len(password) == 64 {
		for _, r := range password {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return nil, errors.New("64-character key must be hexadecimal")
			}
		}
	}
	sec := map[string]dbus.Variant{"key-mgmt": dbus.MakeVariant(keyMgmt)}
	if keyMgmt != "owe" {
		sec["psk"] = dbus.MakeVariant(password)
	}
	s["802-11-wireless-security"] = sec
	return s, nil
}

func (c Client) ConnectWiFi(ctx context.Context, ap AccessPoint, password string) error {
	return c.withBus(func(b Bus) error {
		var active dbus.ObjectPath
		complete := false
		activationStarted := false
		defer func() {
			if complete || !activationStarted {
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if ctx.Err() != nil {
				b.Call(cleanupCtx, nm, ap.Device, nm+".Device.Disconnect")
			} else if active != "" {
				b.Call(cleanupCtx, nm, nmPath, nm+".DeactivateConnection", active)
			}
		}()

		list, err := profiles(ctx, b)
		if err != nil {
			return err
		}
		var existing *profile
		for i := range list {
			if matches(list[i], ap) {
				existing = &list[i]
				break
			}
		}
		if existing != nil {
			if password != "" {
				update, err := wifiSettings(ap, password)
				if err != nil {
					return err
				}
				// Keep profile identity and unrelated device options when replacing a key.
				existing.settings["802-11-wireless-security"] = update["802-11-wireless-security"]
				if _, err = b.Call(ctx, nm, existing.path, settingsInterface+".Update", existing.settings); err != nil {
					return err
				}
			}
			activationStarted = true
			body, err := b.Call(ctx, nm, nmPath, nm+".ActivateConnection", existing.path, ap.Device, ap.Path)
			if err != nil {
				return err
			}
			if err = dbus.Store(body, &active); err != nil {
				return err
			}
		} else {
			settings, err := wifiSettings(ap, password)
			if err != nil {
				return err
			}
			activationStarted = true
			body, err := b.Call(ctx, nm, nmPath, nm+".AddAndActivateConnection", settings, ap.Device, ap.Path)
			if err != nil {
				return err
			}
			var path dbus.ObjectPath
			if err = dbus.Store(body, &path, &active); err != nil {
				return err
			}
		}
		for {
			prop, err := properties(ctx, b, nm, active, nm+".Connection.Active")
			if err != nil {
				return errors.New("Wi-Fi connection ended; check password and signal")
			}
			switch value[uint32](prop, "State") {
			case 2:
				complete = true
				return nil
			case 4:
				return errors.New("Wi-Fi connection failed; check password and signal")
			}
			if err := pause(ctx, 500*time.Millisecond); err != nil {
				return err
			}
		}
	})
}
func (c Client) DisconnectWiFi(ctx context.Context, ap AccessPoint) error {
	return c.withBus(func(b Bus) error { _, err := b.Call(ctx, nm, ap.Device, nm+".Device.Disconnect"); return err })
}
func (c Client) ForgetWiFi(ctx context.Context, ap AccessPoint) error {
	return c.withBus(func(b Bus) error {
		list, err := profiles(ctx, b)
		if err != nil {
			return err
		}
		for _, p := range list {
			if matches(p, ap) {
				if _, err := b.Call(ctx, nm, p.path, settingsInterface+".Delete"); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
