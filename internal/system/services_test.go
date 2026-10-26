package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeServiceTable(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, ServiceListPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const serviceTable = `# comment
adbd.enabled|adbd|ADB
ftp.enabled|ftp|FTP
`

func TestServicesReadTheTableAndAskRocknixServices(t *testing.T) {
	root := t.TempDir()
	writeServiceTable(t, root, serviceTable)
	states := map[string]string{"adbd.enabled": "adbd.enabled=1", "ftp.enabled": "ftp.enabled=0"}
	var calls []string
	p := &Probe{Root: root, Run: func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name != "rocknix-services" || args[0] != "status" {
			t.Fatalf("unexpected command %s %v", name, args)
		}
		return states[args[1]], nil
	}}
	services, err := p.Services()
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 {
		t.Fatalf("services = %+v", services)
	}
	if services[0].Label != "ADB" || !services[0].On || !services[0].Known {
		t.Fatalf("adbd = %+v", services[0])
	}
	if services[1].Label != "FTP" || services[1].On || !services[1].Known {
		t.Fatalf("ftp = %+v", services[1])
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestServicesReportUnavailableWithoutRocknixServices(t *testing.T) {
	root := t.TempDir()
	writeServiceTable(t, root, serviceTable)
	p := &Probe{Root: root, Run: func(string, ...string) (string, error) { return "", os.ErrNotExist }}
	services, err := p.Services()
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range services {
		if service.Known {
			t.Fatalf("%s reported known without rocknix-services", service.Key)
		}
	}
}

func TestServicesWithoutTableAreUnavailable(t *testing.T) {
	if _, err := (&Probe{Root: t.TempDir()}).Services(); err == nil {
		t.Fatal("missing table should be an error so callers can hide the entry")
	}
}

func TestSetServiceRunsTheToggleAndRejectsUnknownKeys(t *testing.T) {
	root := t.TempDir()
	writeServiceTable(t, root, serviceTable)
	var calls []string
	p := &Probe{Root: root, Run: func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return "", nil
	}}
	t.Setenv("HEYA_SIMULATOR", "")
	if err := p.SetService("ftp.enabled", "on"); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || calls[len(calls)-1] != "rocknix-services set ftp.enabled on" {
		t.Fatalf("calls = %v", calls)
	}
	for _, pair := range [][2]string{{"ftp.enabled", "maybe"}, {"nope.enabled", "on"}} {
		if p.SetService(pair[0], pair[1]) == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "rocknix-services set") && call != "rocknix-services set ftp.enabled on" {
			t.Fatalf("invalid toggle executed: %s", call)
		}
	}
}

func TestSetServiceIsRefusedInTheSimulator(t *testing.T) {
	root := t.TempDir()
	writeServiceTable(t, root, serviceTable)
	p := &Probe{Root: root, Run: func(string, ...string) (string, error) { return "", nil }}
	t.Setenv("HEYA_SIMULATOR", "1")
	if err := p.SetService("adbd.enabled", "on"); err == nil {
		t.Fatal("simulator changed host services")
	}
}
