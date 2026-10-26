package connectivity

import (
	"context"
	"errors"
	"os"
)

func NewBackend() Backend {
	if os.Getenv("HEYA_SIMULATOR") == "1" {
		return demo{}
	}
	return Client{}
}

type demo struct{}

func (demo) WiFi(context.Context) ([]AccessPoint, error) {
	return []AccessPoint{
		{SSID: "HEYA Demo Wi-Fi", Security: "WPA/WPA2", Strength: 92},
		{SSID: "Guest (demo)", Security: "Open", Strength: 68},
		{SSID: "WPA3 (demo)", Security: "WPA3", Strength: 45},
	}, nil
}
func demoError() error                                              { return errors.New("preview only; test radio operations on the handheld") }
func (demo) ConnectWiFi(context.Context, AccessPoint, string) error { return demoError() }
func (demo) DisconnectWiFi(context.Context, AccessPoint) error      { return demoError() }
func (demo) ForgetWiFi(context.Context, AccessPoint) error          { return demoError() }
func (demo) Bluetooth(context.Context) ([]Device, error) {
	return []Device{{Name: "Gamepad (demo)", Address: "00:11:22:33:44:55", Path: "/demo/pad"}, {Name: "Headphones (demo)", Address: "00:11:22:33:44:66", Path: "/demo/audio"}}, nil
}
func (demo) Pair(ctx context.Context, d Device, show func(Prompt)) error {
	reply := make(chan string, 1)
	show(Prompt{Title: "Confirm Bluetooth code", Text: "Demo code: 123456", Reply: reply})
	select {
	case <-reply:
		return demoError()
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (demo) DisconnectBluetooth(context.Context, Device) error { return demoError() }
func (demo) ForgetBluetooth(context.Context, Device) error     { return demoError() }
