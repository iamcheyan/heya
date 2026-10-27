// Command dump-apps prints the applications discovered by heya's own registry
// as JSON, so a UI prototype can consume real scanner output instead of
// hand-written mock data. It only reads: no launcher, system or UI code is
// involved, and nothing is executed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/iamcheyan/heya/internal/registry"
)

func main() {
	dir := flag.String("dir", "examples/apps", "applications directory to scan")
	flag.Parse()

	apps, problems := registry.New(*dir).Scan()
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "scan problem: %v\n", problem)
	}

	type card struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Category   string   `json:"category"`
		Icon       string   `json:"icon"`
		Accent     string   `json:"accent"`
		Badge      string   `json:"badge"`
		Subtitle   string   `json:"subtitle"`
		Entrypoint string   `json:"entrypoint"`
		Args       []string `json:"args"`
	}
	out := make([]card, 0, len(apps))
	for _, app := range apps {
		entry := app.Entrypoint
		if entry == "" {
			entry = app.Exec
		}
		out = append(out, card{
			ID: app.ID, Name: app.Name, Category: app.Category, Icon: app.Icon,
			Accent: app.Accent, Badge: app.Badge, Subtitle: app.Description,
			Entrypoint: entry, Args: app.Args,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}
