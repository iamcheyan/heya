// Package capabilities describes device support independently of device models.
package capabilities

type Device interface {
	SupportsWiFi() bool
	SupportsBluetooth() bool
	SupportsBrightness() bool
	SupportsSuspend() bool
	SupportsBattery() bool
}
