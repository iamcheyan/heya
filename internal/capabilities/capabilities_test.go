package capabilities

import "testing"

type device struct{ wifi, bluetooth, brightness, suspend, battery bool }

func (d device) SupportsWiFi() bool       { return d.wifi }
func (d device) SupportsBluetooth() bool  { return d.bluetooth }
func (d device) SupportsBrightness() bool { return d.brightness }
func (d device) SupportsSuspend() bool    { return d.suspend }
func (d device) SupportsBattery() bool    { return d.battery }

func TestDeviceCapabilityContract(t *testing.T) {
	var d Device = device{wifi: true, battery: true}
	if !d.SupportsWiFi() || !d.SupportsBattery() {
		t.Fatal("expected declared capabilities")
	}
	if d.SupportsBluetooth() || d.SupportsBrightness() || d.SupportsSuspend() {
		t.Fatal("unsupported capabilities reported available")
	}
}
