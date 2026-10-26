package connectivity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

const bluez = "org.bluez"

type objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

func managed(ctx context.Context, b Bus) (objects, error) {
	body, err := b.Call(ctx, bluez, "/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects")
	if err != nil {
		return nil, err
	}
	var result objects
	err = dbus.Store(body, &result)
	return result, err
}
func adapter(all objects) (dbus.ObjectPath, error) {
	paths := []string{}
	for path, interfaces := range all {
		if props, ok := interfaces[bluez+".Adapter1"]; ok && value[bool](props, "Powered") {
			paths = append(paths, string(path))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return "", errors.New("no powered Bluetooth adapter; enable Bluetooth first")
	}
	return dbus.ObjectPath(paths[0]), nil
}
func deviceList(all objects) []Device {
	result := []Device{}
	for path, interfaces := range all {
		prop, ok := interfaces[bluez+".Device1"]
		if !ok {
			continue
		}
		name := value[string](prop, "Alias")
		if name == "" {
			name = value[string](prop, "Name")
		}
		address := value[string](prop, "Address")
		if name == "" {
			name = address
		}
		result = append(result, Device{Path: path, Name: name, Address: address, Paired: value[bool](prop, "Paired"), Connected: value[bool](prop, "Connected")})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Connected != result[j].Connected {
			return result[i].Connected
		}
		if result[i].Paired != result[j].Paired {
			return result[i].Paired
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Address < result[j].Address
	})
	return result
}
func (c Client) Bluetooth(ctx context.Context) (result []Device, err error) {
	err = c.withBus(func(b Bus) error {
		all, err := managed(ctx, b)
		if err != nil {
			return err
		}
		path, err := adapter(all)
		if err != nil {
			return err
		}
		if _, err = b.Call(ctx, bluez, path, bluez+".Adapter1.StartDiscovery"); err != nil {
			return err
		}
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			b.Call(stopCtx, bluez, path, bluez+".Adapter1.StopDiscovery")
		}()
		if err := pause(ctx, 8*time.Second); err != nil {
			return err
		}
		all, err = managed(ctx, b)
		if err != nil {
			return err
		}
		result = deviceList(all)
		return nil
	})
	return
}

func (c Client) Pair(ctx context.Context, device Device, prompt func(Prompt)) error {
	conn, err := connectBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	b := systemBus{conn}
	pairingStarted, connectionStarted := false, false
	defer func() {
		if ctx.Err() != nil && (pairingStarted || connectionStarted) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			b.Call(cleanupCtx, bluez, device.Path, bluez+".Device1.CancelPairing")
			b.Call(cleanupCtx, bluez, device.Path, bluez+".Device1.Disconnect")
		}
	}()
	agent := &pairAgent{ctx: ctx, device: device.Path, prompt: prompt}
	const agentPath dbus.ObjectPath = "/org/heya/pairing"
	if err := conn.Export(agent, agentPath, bluez+".Agent1"); err != nil {
		return err
	}
	if _, err = b.Call(ctx, bluez, "/org/bluez", bluez+".AgentManager1.RegisterAgent", agentPath, "KeyboardDisplay"); err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		b.Call(cleanupCtx, bluez, "/org/bluez", bluez+".AgentManager1.UnregisterAgent", agentPath)
	}()
	current, err := properties(ctx, b, bluez, device.Path, bluez+".Device1")
	if err != nil {
		return err
	}
	if value[bool](current, "Connected") {
		return nil
	}
	if !value[bool](current, "Paired") {
		pairingStarted = true
		if _, err = b.Call(ctx, bluez, device.Path, bluez+".Device1.Pair"); err != nil {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			b.Call(cancelCtx, bluez, device.Path, bluez+".Device1.CancelPairing")
			return err
		}
	}
	if _, err = b.Call(ctx, bluez, device.Path, "org.freedesktop.DBus.Properties.Set", bluez+".Device1", "Trusted", dbus.MakeVariant(true)); err != nil {
		return err
	}
	connectionStarted = true
	_, err = b.Call(ctx, bluez, device.Path, bluez+".Device1.Connect")
	return err
}
func (c Client) DisconnectBluetooth(ctx context.Context, d Device) error {
	return c.withBus(func(b Bus) error { _, err := b.Call(ctx, bluez, d.Path, bluez+".Device1.Disconnect"); return err })
}
func (c Client) ForgetBluetooth(ctx context.Context, d Device) error {
	return c.withBus(func(b Bus) error {
		all, err := managed(ctx, b)
		if err != nil {
			return err
		}
		props := all[d.Path][bluez+".Device1"]
		path := value[dbus.ObjectPath](props, "Adapter")
		if path == "" {
			return errors.New("device no longer exists")
		}
		_, err = b.Call(ctx, bluez, path, bluez+".Adapter1.RemoveDevice", d.Path)
		return err
	})
}

type pairAgent struct {
	ctx    context.Context
	device dbus.ObjectPath
	prompt func(Prompt)
}

func rejected() *dbus.Error {
	return dbus.NewError(bluez+".Error.Rejected", []interface{}{"Pairing canceled"})
}
func (a *pairAgent) ask(d dbus.ObjectPath, p Prompt) (string, *dbus.Error) {
	if d != a.device {
		return "", rejected()
	}
	p.Reply = make(chan string, 1)
	a.prompt(p)
	select {
	case reply := <-p.Reply:
		if reply == "" {
			return "", rejected()
		}
		return reply, nil
	case <-a.ctx.Done():
		return "", rejected()
	}
}
func (a *pairAgent) Release() *dbus.Error { return nil }
func (a *pairAgent) Cancel() *dbus.Error  { a.prompt(Prompt{Canceled: true}); return nil }
func (a *pairAgent) RequestPinCode(d dbus.ObjectPath) (string, *dbus.Error) {
	reply, err := a.ask(d, Prompt{Title: "Bluetooth PIN", Text: "Enter the PIN from the device", Entry: true})
	if err != nil {
		return "", err
	}
	if len(reply) < 1 || len(reply) > 16 {
		return "", rejected()
	}
	return reply, nil
}
func (a *pairAgent) RequestPasskey(d dbus.ObjectPath) (uint32, *dbus.Error) {
	reply, err := a.ask(d, Prompt{Title: "Bluetooth passkey", Text: "Enter six digits", Entry: true, Numeric: true})
	if err != nil {
		return 0, err
	}
	n, e := strconv.ParseUint(reply, 10, 32)
	if e != nil || n > 999999 {
		return 0, rejected()
	}
	return uint32(n), nil
}
func (a *pairAgent) RequestConfirmation(d dbus.ObjectPath, n uint32) *dbus.Error {
	_, err := a.ask(d, Prompt{Title: "Confirm Bluetooth code", Text: fmt.Sprintf("Does the other device show %06d?", n)})
	return err
}
func (a *pairAgent) RequestAuthorization(d dbus.ObjectPath) *dbus.Error {
	_, err := a.ask(d, Prompt{Title: "Bluetooth pairing", Text: "Allow this device to pair?"})
	return err
}
func (a *pairAgent) AuthorizeService(d dbus.ObjectPath, uuid string) *dbus.Error {
	if d != a.device {
		return rejected()
	}
	return nil
}
func (a *pairAgent) DisplayPinCode(d dbus.ObjectPath, pin string) *dbus.Error {
	if d != a.device {
		return rejected()
	}
	a.prompt(Prompt{Title: "Bluetooth keyboard", Text: "Type " + pin + " on the other device, then Enter", Display: true})
	return nil
}
func (a *pairAgent) DisplayPasskey(d dbus.ObjectPath, key uint32, entered uint16) *dbus.Error {
	if d != a.device {
		return rejected()
	}
	a.prompt(Prompt{Title: "Bluetooth keyboard", Text: fmt.Sprintf("Type %06d on the other device (%d/6)", key, entered), Display: true})
	return nil
}
