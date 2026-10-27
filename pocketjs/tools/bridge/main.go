package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iamcheyan/heya/internal/model"
	"github.com/iamcheyan/heya/internal/registry"
)

type event struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
	App  string `json:"app_id,omitempty"`
	PID  int    `json:"pid,omitempty"`
}

type status struct {
	Running   bool      `json:"running"`
	AppID     string    `json:"app_id,omitempty"`
	PID       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

type reply struct {
	OK     bool   `json:"ok"`
	ID     uint64 `json:"id,omitempty"`
	Apps   any    `json:"apps,omitempty"`
	Status status `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

type bridge struct {
	mu      sync.Mutex
	appsDir string
	events  []event
	nextID  uint64
	current status
	process *exec.Cmd
}

func newBridge(appsDir string) *bridge { return &bridge{appsDir: appsDir} }

func (b *bridge) emit(kind, app string, pid int) {
	b.nextID++
	b.events = append(b.events, event{ID: b.nextID, Type: kind, App: app, PID: pid})
	if len(b.events) > 256 {
		b.events = b.events[len(b.events)-256:]
	}
}

func (b *bridge) scan() []model.App {
	apps, problems := registry.New(b.appsDir).Scan()
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "bridge: scan problem: %v\n", problem)
	}
	return apps
}

func (b *bridge) command(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var request struct {
		Type  string `json:"t"`
		ID    uint64 `json:"id"`
		AppID string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, reply{Error: "invalid JSON"})
		return
	}
	switch request.Type {
	case "launch":
		b.mu.Lock()
		busy := b.process != nil
		b.mu.Unlock()
		if busy {
			writeJSON(w, http.StatusConflict, reply{ID: request.ID, Error: "an application is already running"})
			return
		}
		var app *model.App
		for _, candidate := range b.scan() {
			if candidate.ID == request.AppID {
				copy := candidate
				app = &copy
				break
			}
		}
		if app == nil {
			writeJSON(w, http.StatusNotFound, reply{ID: request.ID, Error: "application not found"})
			return
		}
		program := app.Entrypoint
		if program == "" {
			program = app.Exec
		}
		cmd := exec.Command(program, app.Args...)
		cmd.Env = mergeEnvironment(os.Environ(), app.Environment)
		cmd.Dir = app.WorkingDirectory
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		b.mu.Lock()
		if b.process != nil {
			b.mu.Unlock()
			writeJSON(w, http.StatusConflict, reply{ID: request.ID, Error: "an application is already running"})
			return
		}
		if err := cmd.Start(); err != nil {
			b.mu.Unlock()
			writeJSON(w, http.StatusInternalServerError, reply{ID: request.ID, Error: err.Error()})
			return
		}
		b.process = cmd
		b.current = status{Running: true, AppID: app.ID, PID: cmd.Process.Pid, StartedAt: time.Now().UTC()}
		b.emit("app_started", app.ID, cmd.Process.Pid)
		b.emit("status_changed", app.ID, cmd.Process.Pid)
		current := b.current
		go b.wait(cmd, app.ID)
		b.mu.Unlock()
		writeJSON(w, http.StatusOK, reply{ID: request.ID, OK: true, Status: current})
	case "rescan":
		apps := b.scan()
		b.mu.Lock()
		b.emit("registry_updated", "", 0)
		b.mu.Unlock()
		writeJSON(w, http.StatusOK, reply{ID: request.ID, OK: true, Apps: apps})
	case "get_status":
		b.mu.Lock()
		current := b.current
		b.mu.Unlock()
		writeJSON(w, http.StatusOK, reply{ID: request.ID, OK: true, Status: current})
	default:
		writeJSON(w, http.StatusBadRequest, reply{ID: request.ID, Error: "unknown command"})
	}
}

func mergeEnvironment(base, overrides []string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	for _, item := range append(append([]string(nil), base...), overrides...) {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, found := values[key]; !found {
			order = append(order, key)
		}
		values[key] = item
	}
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, values[key])
	}
	return result
}

func (b *bridge) wait(cmd *exec.Cmd, appID string) {
	err := cmd.Wait()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.process != cmd {
		return
	}
	pid := cmd.Process.Pid
	b.process = nil
	b.current = status{}
	b.emit("app_exited", appID, pid)
	b.emit("status_changed", appID, pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bridge: app %s exited: %v\n", appID, err)
	}
}

func (b *bridge) eventsSince(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	since, err := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event cursor"})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	oldest := b.nextID + 1
	if len(b.events) > 0 {
		oldest = b.events[0].ID
	}
	if since+1 < oldest {
		writeJSON(w, http.StatusGone, map[string]any{"error": "event history expired", "oldest": oldest, "next": b.nextID})
		return
	}
	out := make([]event, 0)
	for _, item := range b.events {
		if item.ID > since {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Next   uint64  `json:"next"`
		Events []event `json:"events"`
	}{Next: b.nextID, Events: out})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	appsDir := flag.String("apps", "examples/apps", "directory containing heya application manifests")
	listen := flag.String("listen", "127.0.0.1:8132", "HTTP listen address")
	flag.Parse()
	abs, err := filepath.Abs(*appsDir)
	if err != nil {
		fatal(err)
	}
	b := newBridge(abs)
	mux := http.NewServeMux()
	mux.HandleFunc("/cmd", b.command)
	mux.HandleFunc("/events", b.eventsSince)
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("heya bridge listening on http://%s (apps=%s)\n", *listen, abs)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal(err)
	}
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
