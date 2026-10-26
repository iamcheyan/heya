package system

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// Service is one optional background service controlled from heya.
type Service struct {
	Key   string
	Label string
	On    bool
	Known bool
}

// ServiceListPath is the packaged label table: SETTING|KEY|LABEL per line.
const ServiceListPath = "usr/share/heya/settings/services.conf"

var (
	errInvalidSwitch  = errors.New("invalid switch")
	errPreview        = errors.New("preview: device controls are unavailable")
	errUnknownService = errors.New("unknown service")
	errNoServiceTable = errors.New("service table is missing")
)

// Services reads the label table, then asks rocknix-services for each state.
func (p *Probe) Services() ([]Service, error) {
	file, err := os.Open(p.path(ServiceListPath))
	if err != nil {
		return nil, errNoServiceTable
	}
	defer file.Close()

	var services []Service
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, "|", 3)
		if len(fields) != 3 {
			continue
		}
		service := Service{Key: fields[0], Label: fields[2]}
		if p.Run != nil {
			if out, err := p.Run("rocknix-services", "status", service.Key); err == nil {
				if _, value, ok := strings.Cut(out, "="); ok {
					service.On = strings.TrimSpace(value) == "1"
					service.Known = true
				}
			}
		}
		services = append(services, service)
	}
	return services, scanner.Err()
}

// SetService turns one service on or off through rocknix-services.
func (p *Probe) SetService(key, value string) error {
	if value != "on" && value != "off" {
		return errInvalidSwitch
	}
	services, err := p.Services()
	if err != nil {
		return err
	}
	for _, service := range services {
		if service.Key != key {
			continue
		}
		if os.Getenv("HEYA_SIMULATOR") == "1" {
			return errPreview
		}
		if p.Run == nil {
			return errPreview
		}
		_, err := p.Run("rocknix-services", "set", key, value)
		return err
	}
	return errUnknownService
}
