package connectivity

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestWiFiSettings(t *testing.T) {
	ap := AccessPoint{SSID: "a:'\\ network", Security: "WPA/WPA2"}
	s, err := wifiSettings(ap, "quote' $() pass")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(value[[]byte](s["802-11-wireless"], "ssid")); got != ap.SSID {
		t.Fatal(got)
	}
	if value[string](s["802-11-wireless-security"], "psk") != "quote' $() pass" {
		t.Fatal("password corrupted")
	}
	if _, err := wifiSettings(ap, "short"); err == nil {
		t.Fatal("short key accepted")
	}
	ap.Security = "Enterprise"
	if _, err := wifiSettings(ap, "password"); err == nil {
		t.Fatal("enterprise incorrectly treated as PSK")
	}
}
func TestSecurityAndDedup(t *testing.T) {
	if securityType(1, 0, 0x200) != "Enterprise" || securityType(1, 0, 0x400) != "WPA3" || securityType(0, 0, 0) != "Open" {
		t.Fatal("security flags")
	}
	aps := uniqueAPs([]AccessPoint{{SSID: "same", Strength: 80}, {SSID: "same", Strength: 30, Connected: true}, {SSID: "other", Strength: 90}})
	if len(aps) != 2 || aps[0].SSID != "same" || !aps[0].Connected {
		t.Fatal(aps)
	}
}
func TestAgentRejectsOtherDeviceAndCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &pairAgent{ctx: ctx, device: "/device", prompt: func(p Prompt) { p.Reply <- "yes" }}
	if a.RequestConfirmation("/other", 123456) == nil {
		t.Fatal("accepted another device")
	}
	if err := a.RequestConfirmation("/device", 123456); err != nil {
		t.Fatal(err)
	}
	a.prompt = func(p Prompt) { cancel() }
	if a.RequestAuthorization("/device") == nil {
		t.Fatal("accepted canceled request")
	}
}
func TestDeviceProperties(t *testing.T) {
	list := deviceList(objects{"/device": {bluez + ".Device1": {"Name": dbus.MakeVariant("Pad"), "Connected": dbus.MakeVariant(true), "Paired": dbus.MakeVariant(true)}}})
	if len(list) != 1 || list[0].Name != "Pad" || !list[0].Connected {
		t.Fatal(list)
	}
}

type fakeBus struct {
	call func(context.Context, string, dbus.ObjectPath, string, ...interface{}) ([]interface{}, error)
}

func (b fakeBus) Call(ctx context.Context, d string, p dbus.ObjectPath, m string, a ...interface{}) ([]interface{}, error) {
	return b.call(ctx, d, p, m, a...)
}
func TestConnectWaitsForActivation(t *testing.T) {
	ap := AccessPoint{SSID: "test:ssid", Security: "WPA/WPA2", Path: "/ap", Device: "/wlan"}
	activated := false
	c := Client{Bus: fakeBus{call: func(ctx context.Context, d string, p dbus.ObjectPath, m string, a ...interface{}) ([]interface{}, error) {
		switch m {
		case nm + ".Settings.ListConnections":
			return []interface{}{[]dbus.ObjectPath{}}, nil
		case nm + ".AddAndActivateConnection":
			settings := a[0].(map[string]map[string]dbus.Variant)
			if string(value[[]byte](settings["802-11-wireless"], "ssid")) != ap.SSID {
				t.Fatal(settings)
			}
			if value[string](settings["802-11-wireless-security"], "psk") != "password123" {
				t.Fatal("key lost")
			}
			activated = true
			return []interface{}{dbus.ObjectPath("/profile"), dbus.ObjectPath("/active")}, nil
		case "org.freedesktop.DBus.Properties.GetAll":
			return []interface{}{map[string]dbus.Variant{"State": dbus.MakeVariant(uint32(2))}}, nil
		default:
			t.Fatalf("unexpected %s", m)
			return nil, nil
		}
	}}}
	if err := c.ConnectWiFi(context.Background(), ap, "password123"); err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("did not activate")
	}
}
func TestCancelDuringStateReadDisconnects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	disconnected := false
	c := Client{Bus: fakeBus{call: func(ctx context.Context, d string, p dbus.ObjectPath, m string, a ...interface{}) ([]interface{}, error) {
		switch m {
		case nm + ".Settings.ListConnections":
			return []interface{}{[]dbus.ObjectPath{}}, nil
		case nm + ".AddAndActivateConnection":
			return []interface{}{dbus.ObjectPath("/profile"), dbus.ObjectPath("/active")}, nil
		case "org.freedesktop.DBus.Properties.GetAll":
			cancel()
			return nil, context.Canceled
		case nm + ".Device.Disconnect":
			if ctx.Err() != nil {
				t.Fatal("cleanup used canceled context")
			}
			disconnected = true
			return nil, nil
		default:
			t.Fatalf("unexpected %s", m)
			return nil, nil
		}
	}}}
	if err := c.ConnectWiFi(ctx, AccessPoint{SSID: "open", Security: "Open", Device: "/wlan", Path: "/ap"}, ""); err == nil {
		t.Fatal("canceled activation succeeded")
	}
	if !disconnected {
		t.Fatal("connection left running")
	}
}
func TestProfilesWithSameSSIDKeepSecuritySeparate(t *testing.T) {
	settings, err := wifiSettings(AccessPoint{SSID: "same", Security: "WPA/WPA2"}, "password123")
	if err != nil {
		t.Fatal(err)
	}
	p := profile{settings: settings}
	if !matches(p, AccessPoint{SSID: "same", Security: "WPA/WPA2"}) {
		t.Fatal("PSK profile not found")
	}
	for _, security := range []string{"Open", "Enterprise", "WPA3"} {
		if matches(p, AccessPoint{SSID: "same", Security: security}) {
			t.Fatalf("matched %s", security)
		}
	}
}

func TestConnectUpdatesExistingProfileCleansLegacyIP(t *testing.T) {
	ap := AccessPoint{SSID: "test:ssid", Security: "WPA/WPA2", Path: "/ap", Device: "/wlan"}
	settings, err := wifiSettings(ap, "oldpassword")
	if err != nil {
		t.Fatal(err)
	}
	settings["ipv6"]["addresses"] = dbus.MakeVariant([][]byte{{0xfe, 0x80}})
	settings["ipv6"]["routes"] = dbus.MakeVariant([]string{"default"})
	settings["ipv4"]["addresses"] = dbus.MakeVariant([]uint32{0x01020304})
	settings["ipv4"]["routes"] = dbus.MakeVariant([]string{"default"})

	updated := false
	c := Client{Bus: fakeBus{call: func(ctx context.Context, d string, p dbus.ObjectPath, m string, a ...interface{}) ([]interface{}, error) {
		switch m {
		case nm + ".Settings.ListConnections":
			return []interface{}{[]dbus.ObjectPath{"/profile/1"}}, nil
		case settingsInterface + ".GetSettings":
			return []interface{}{settings}, nil
		case settingsInterface + ".Update":
			upSettings := a[0].(map[string]map[string]dbus.Variant)
			if _, ok := upSettings["ipv6"]["addresses"]; ok {
				t.Fatal("ipv6 addresses not cleaned")
			}
			if _, ok := upSettings["ipv6"]["routes"]; ok {
				t.Fatal("ipv6 routes not cleaned")
			}
			if _, ok := upSettings["ipv4"]["addresses"]; ok {
				t.Fatal("ipv4 addresses not cleaned")
			}
			if _, ok := upSettings["ipv4"]["routes"]; ok {
				t.Fatal("ipv4 routes not cleaned")
			}
			if value[string](upSettings["802-11-wireless-security"], "psk") != "newpassword" {
				t.Fatal("password not updated")
			}
			updated = true
			return []interface{}{}, nil
		case nm + ".ActivateConnection":
			return []interface{}{dbus.ObjectPath("/active")}, nil
		case "org.freedesktop.DBus.Properties.GetAll":
			return []interface{}{map[string]dbus.Variant{"State": dbus.MakeVariant(uint32(2))}}, nil
		default:
			t.Fatalf("unexpected %s", m)
			return nil, nil
		}
	}}}
	if err := c.ConnectWiFi(context.Background(), ap, "newpassword"); err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("profile not updated")
	}
}
