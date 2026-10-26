package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/system"
)

// servicesProbe points a probe at a fixture image that ships the service table.
func servicesProbe(t *testing.T) *system.Probe {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, system.ServiceListPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "adbd.enabled|adbd|ADB\nftp.enabled|ftp|FTP\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return &system.Probe{Root: root, Run: func(name string, args ...string) (string, error) {
		if len(args) == 2 && args[0] == "status" {
			return args[1] + "=1", nil
		}
		return "", nil
	}}
}

func servicesLauncher(t *testing.T) *Launcher {
	t.Helper()
	l := testLauncher()
	l.probe = servicesProbe(t)
	return l
}

func systemIndex(l *Launcher, label string) int {
	for i, row := range l.Frame().Rows {
		if row.Label == label {
			return i
		}
	}
	return -1
}

func TestServicesEntryAppearsOnlyWithAServiceTable(t *testing.T) {
	plain := testLauncher()
	plain.Handle(input.Down)
	plain.Handle(input.Confirm)
	if systemIndex(plain, "Services") != -1 {
		t.Fatal("Services entry shown without a service table")
	}

	l := servicesLauncher(t)
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	if systemIndex(l, "Services") == -1 {
		t.Fatal("Services entry missing with a service table")
	}
	if systemIndex(l, "USB settings") == -1 {
		t.Fatal("USB settings entry was displaced")
	}
}

func TestServicesPageListsLabelsAndStates(t *testing.T) {
	l := servicesLauncher(t)
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	for l.Frame().Rows[l.Frame().Selected].Label != "Services" {
		l.Handle(input.Down)
		if l.Frame().Selected == 0 {
			t.Fatal("Services row is not reachable")
		}
	}
	l.Handle(input.Confirm)
	if l.Path() != "apps>system>services" {
		t.Fatalf("path = %s", l.Path())
	}
	if l.Title() != "Services" {
		t.Fatalf("title = %s", l.Title())
	}
	rows := l.Frame().Rows
	if len(rows) != 2 || rows[0].Label != "ADB" || rows[0].Value != "On" || rows[1].Label != "FTP" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestServicesToggleRunsRocknixServicesAndRefreshes(t *testing.T) {
	l := servicesLauncher(t)
	var toggled []string
	inner := l.probe.Run
	l.probe.Run = func(name string, args ...string) (string, error) {
		if len(args) == 3 && args[0] == "set" {
			toggled = append(toggled, strings.Join(append([]string{name}, args...), " "))
		}
		return inner(name, args...)
	}

	l.Handle(input.Down)
	l.Handle(input.Confirm)
	for l.Frame().Rows[l.Frame().Selected].Label != "Services" {
		l.Handle(input.Down)
	}
	l.Handle(input.Confirm)

	// Opening the page loads the states; drain that first result.
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		l.Poll()
	}
	if row := l.Frame().Rows[0]; row.Label != "ADB" || row.Value != "On" {
		t.Fatalf("initial row = %+v", row)
	}

	l.Handle(input.Left)
	if row := l.Frame().Rows[0]; row.Value != "Working..." {
		t.Fatalf("pending row = %+v", row)
	}
	// Wait for the in-flight toggle and for the status re-read it triggers.
	for i := 0; i < 100; i++ {
		time.Sleep(20 * time.Millisecond)
		l.Poll()
		if len(toggled) > 0 && l.Frame().Message != "Working... [B] Cancel" && l.Frame().Rows[0].Value != "Working..." {
			break
		}
	}
	if len(toggled) != 1 || toggled[0] != "rocknix-services set adbd.enabled off" {
		t.Fatalf("toggled = %v", toggled)
	}
	if row := l.Frame().Rows[0]; row.Label != "ADB" || row.Value != "On" {
		t.Fatalf("row after toggle = %+v", row)
	}
}

// openServices walks from the apps page to the Services page.
func openServices(t *testing.T, l *Launcher) {
	t.Helper()
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	for i := 0; i < 40; i++ {
		if l.Frame().Rows[l.Frame().Selected].Label == "Services" {
			l.Handle(input.Confirm)
			if l.Path() != "apps>system>services" {
				t.Fatalf("path = %s", l.Path())
			}
			return
		}
		l.Handle(input.Down)
	}
	t.Fatal("Services row is not reachable")
}

// settle polls until done reports true or the deadline passes.
func settle(t *testing.T, l *Launcher, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// Let the background worker run before draining its result; a hot
		// poll loop starves it on a single-CPU test host.
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < 50; i++ {
			l.Poll()
		}
		if done() && l.Frame().Message != "Working... [B] Cancel" {
			return
		}
	}
}

func TestServicesLeftAndRightAreDirectional(t *testing.T) {
	l := servicesLauncher(t)
	openServices(t, l)
	// ADB starts on; pressing right again must not issue another command.
	l.Handle(input.Right)
	if l.Frame().Rows[0].Value != "On" {
		t.Fatalf("row = %+v", l.Frame().Rows[0])
	}
	if l.Frame().Message == "Working... [B] Cancel" {
		t.Fatal("right on an enabled service started a job")
	}
}
