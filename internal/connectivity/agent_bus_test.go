package connectivity

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestPairingAgentOverPrivateDBus(t *testing.T) {
	executable, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon unavailable")
	}
	process := exec.Command(executable, "--session", "--nofork", "--print-address=1")
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { process.Process.Kill(); process.Wait() })
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	dial := func() *dbus.Conn {
		connection, err := dbus.Dial(strings.TrimSpace(address))
		if err != nil {
			t.Fatal(err)
		}
		if err := connection.Auth(nil); err != nil {
			t.Fatal(err)
		}
		if err := connection.Hello(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { connection.Close() })
		return connection
	}
	host, remote := dial(), dial()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	prompts := make(chan Prompt, 1)
	agent := &pairAgent{ctx: ctx, device: "/pad", prompt: func(p Prompt) { prompts <- p }}
	path := dbus.ObjectPath("/heya/agent")
	if err := host.Export(agent, path, bluez+".Agent1"); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() {
		completed <- remote.Object(host.Names()[0], path).CallWithContext(ctx, bluez+".Agent1.RequestConfirmation", 0, dbus.ObjectPath("/pad"), uint32(42)).Err
	}()
	select {
	case prompt := <-prompts:
		if !strings.Contains(prompt.Text, "000042") {
			t.Fatal(prompt.Text)
		}
		prompt.Reply <- "yes"
	case <-ctx.Done():
		t.Fatal("agent prompt timed out")
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	call := remote.Object(host.Names()[0], path).CallWithContext(ctx, bluez+".Agent1.RequestConfirmation", 0, dbus.ObjectPath("/other"), uint32(42))
	if call.Err == nil {
		t.Fatal("agent authorized unexpected device")
	}
}
