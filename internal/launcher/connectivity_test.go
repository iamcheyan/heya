package launcher

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iamcheyan/heya/internal/connectivity"
	"github.com/iamcheyan/heya/internal/input"
)

type fakeNetwork struct {
	connect func(context.Context, connectivity.AccessPoint, string) error
	pair    func(context.Context, connectivity.Device, func(connectivity.Prompt)) error
}

func (f fakeNetwork) WiFi(context.Context) ([]connectivity.AccessPoint, error) {
	return []connectivity.AccessPoint{{SSID: "Wi-Fi : test", Security: "WPA/WPA2", Strength: 77}}, nil
}
func (f fakeNetwork) ConnectWiFi(ctx context.Context, a connectivity.AccessPoint, p string) error {
	if f.connect != nil {
		return f.connect(ctx, a, p)
	}
	return nil
}
func (f fakeNetwork) DisconnectWiFi(context.Context, connectivity.AccessPoint) error { return nil }
func (f fakeNetwork) ForgetWiFi(context.Context, connectivity.AccessPoint) error     { return nil }
func (f fakeNetwork) Bluetooth(context.Context) ([]connectivity.Device, error) {
	return []connectivity.Device{{Name: "Gamepad", Path: "/pad"}}, nil
}
func (f fakeNetwork) Pair(ctx context.Context, d connectivity.Device, p func(connectivity.Prompt)) error {
	if f.pair != nil {
		return f.pair(ctx, d, p)
	}
	return nil
}
func (f fakeNetwork) DisconnectBluetooth(context.Context, connectivity.Device) error { return nil }
func (f fakeNetwork) ForgetBluetooth(context.Context, connectivity.Device) error     { return nil }
func waitNetwork(t *testing.T, l *Launcher) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for l.net().busy {
		l.Poll()
		if time.Now().After(deadline) {
			t.Fatal("job did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestWiFiKeyboardAndFailure(t *testing.T) {
	received := make(chan string, 1)
	l := testLauncher()
	l.SetConnectivity(fakeNetwork{connect: func(ctx context.Context, ap connectivity.AccessPoint, password string) error {
		received <- password
		return errors.New("wrong password")
	}})
	l.pushNetwork("wifi")
	waitNetwork(t, l)
	l.Handle(input.Confirm)
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	if !l.TextEntryActive() {
		t.Fatal("password editor did not open")
	}
	l.TextInput("secret'12")
	frame := l.Frame()
	visible := false
	for _, row := range frame.Rows {
		if row.Label == "Input" && row.Value == "secret'12" {
			visible = true
		}
	}
	if !visible {
		t.Fatalf("password should be visible in the editor: %+v", frame.Rows)
	}
	l.Handle(input.Start)
	waitNetwork(t, l)
	if password := <-received; password != "secret'12" {
		t.Fatal(password)
	}
	if !strings.Contains(l.Message(), "wrong password") {
		t.Fatal(l.Message())
	}
	if l.net().editor != nil {
		t.Fatal("retained password editor")
	}
}
func TestCancelConnectionRejectsLateResult(t *testing.T) {
	done := make(chan struct{})
	l := testLauncher()
	l.SetConnectivity(fakeNetwork{connect: func(ctx context.Context, _ connectivity.AccessPoint, _ string) error {
		<-ctx.Done()
		close(done)
		return ctx.Err()
	}})
	l.pushNetwork("wifi")
	waitNetwork(t, l)
	l.Handle(input.Confirm)
	l.net().ap.Saved = true
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	l.Handle(input.Cancel)
	<-done
	waitNetwork(t, l)
	l.Poll()
	if l.net().busy || l.Message() != "Canceled" {
		t.Fatal(l.Message())
	}
}
func TestBluetoothConfirmation(t *testing.T) {
	approved := make(chan bool, 1)
	l := testLauncher()
	l.SetConnectivity(fakeNetwork{pair: func(ctx context.Context, d connectivity.Device, show func(connectivity.Prompt)) error {
		reply := make(chan string, 1)
		show(connectivity.Prompt{Title: "Compare code", Text: "123456", Reply: reply})
		select {
		case value := <-reply:
			approved <- value == "yes"
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	l.pushNetwork("bluetooth")
	waitNetwork(t, l)
	l.Handle(input.Confirm)
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	deadline := time.Now().Add(time.Second)
	for l.net().prompt == nil {
		l.Poll()
		if time.Now().After(deadline) {
			t.Fatal("missing prompt")
		}
		time.Sleep(time.Millisecond)
	}
	if l.Frame().Title != "Compare code" {
		t.Fatal(l.Frame())
	}
	l.Handle(input.Confirm)
	waitNetwork(t, l)
	if !<-approved {
		t.Fatal("not approved")
	}
}
func TestForgetRequiresConfirmation(t *testing.T) {
	l := testLauncher()
	l.SetConnectivity(fakeNetwork{})
	l.pushNetwork("wifi")
	waitNetwork(t, l)
	l.Handle(input.Confirm)
	for i := 0; i < 4; i++ {
		l.Handle(input.Down)
	}
	l.Handle(input.Confirm)
	if l.net().prompt == nil || l.net().busy {
		t.Fatal("missing confirmation")
	}
	l.Handle(input.Cancel)
	if l.net().prompt != nil {
		t.Fatal("prompt retained")
	}
}
func TestKeyboardEveryPrintableCharacter(t *testing.T) {
	found := map[rune]bool{}
	for _, page := range keyboardPages {
		for _, r := range page {
			found[r] = true
		}
	}
	for r := ' '; r <= '~'; r++ {
		if !found[r] {
			t.Errorf("missing key %q", r)
		}
	}
	l := testLauncher()
	l.net().editor = &textEditor{title: "PIN", numeric: true}
	l.TextInput("a1x2!3")
	if l.net().editor.text != "123" {
		t.Fatal(l.net().editor.text)
	}
}
