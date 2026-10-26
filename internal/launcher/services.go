package launcher

import (
	"context"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/ui"
)

// servicePageID backs the System page's "Services" entry. It is not registered
// in l.pages: the rows come from the device on every refresh.
const servicePageID = "services"

type serviceEntry struct {
	Key     string
	Label   string
	On      bool
	Known   bool
	Pending bool
}

func (l *Launcher) readServices() ([]serviceEntry, bool) {
	if l.probe == nil {
		return nil, false
	}
	services, err := l.probe.Services()
	if err != nil {
		return nil, false
	}
	entries := make([]serviceEntry, 0, len(services))
	for _, service := range services {
		entries = append(entries, serviceEntry{Key: service.Key, Label: service.Label, On: service.On, Known: service.Known})
	}
	return entries, true
}

// serviceRows renders the cached snapshot. Toggling refreshes the cache so the
// page never shells out while a frame is being drawn.
func (l *Launcher) serviceRows() []ui.Row {
	if len(l.services) == 0 {
		return []ui.Row{{Label: "Services", Value: "Unavailable"}}
	}
	rows := make([]ui.Row, 0, len(l.services))
	for _, service := range l.services {
		value := "Unavailable"
		switch {
		case service.Pending:
			value = "Working..."
		case service.Known:
			value = onOff(service.On)
		}
		rows = append(rows, ui.Row{Label: service.Label, Value: value})
	}
	return rows
}

// pushServices loads the table once, then opens the page.
func (l *Launcher) pushServices() {
	l.services, _ = l.readServices()
	l.stack = append(l.stack, servicePageID)
	l.index[servicePageID], l.offset[servicePageID] = 0, 0
}

// adjustService applies a left/right toggle to the highlighted service.
func (l *Launcher) adjustService(action input.Action) bool {
	if action != input.Left && action != input.Right {
		return false
	}

	index := l.index[servicePageID]
	if index < 0 || index >= len(l.services) {
		return true
	}
	service := l.services[index]
	if !service.Known {
		l.message = "Control unavailable on this device"
		return true
	}

	if service.Pending {
		return true
	}
	// Left turns off, right turns on; repeating the direction that already
	// holds the state changes nothing.
	want := "on"
	if service.On {
		want = "off"
	}
	if (action == input.Right) != (want == "on") {
		return true
	}

	l.services[index].Pending = true
	key := service.Key
	probe := l.probe
	l.startNetworkJob("service", func(ctx context.Context) networkResult {
		return networkResult{err: probe.SetService(key, want)}
	})
	return true
}

// refreshServices re-reads the table after a toggle completes.
// refreshServices re-reads the states but keeps the current selection and the
// labels the page already shows.
func (l *Launcher) refreshServices() {
	states, err := l.probe.Services()
	if err != nil {
		return
	}
	byKey := make(map[string]bool, len(states))
	for _, state := range states {
		byKey[state.Key] = state.On
	}
	for i := range l.services {
		if on, ok := byKey[l.services[i].Key]; ok {
			l.services[i].On = on
		}
		l.services[i].Pending = false
	}
}
