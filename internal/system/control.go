package system

import (
	"fmt"
	"os"
	"strconv"
)

func (p *Probe) Control(control, value string) error {
	switch control {
	case "volume", "brightness":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 100 {
			return fmt.Errorf("invalid level")
		}
	case "wifi", "bluetooth":
		if value != "on" && value != "off" {
			return fmt.Errorf("invalid switch")
		}
	default:
		return fmt.Errorf("unknown control")
	}
	if os.Getenv("HEYA_SIMULATOR") == "1" {
		return fmt.Errorf("preview: device controls are unavailable")
	}
	_, err := p.Run("heya-control", control, value)
	return err
}
