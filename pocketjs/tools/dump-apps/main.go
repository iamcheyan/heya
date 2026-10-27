package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/iamcheyan/heya/internal/registry"
)

type appCard struct {
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

func main() {
	dir := flag.String("dir", "examples/apps", "applications directory to scan")
	flag.Parse()

	apps, problems := scan(*dir)
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "scan problem: %v\n", problem)
	}
	if err := json.NewEncoder(os.Stdout).Encode(apps); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}

func scan(dir string) ([]appCard, []error) {
	apps, problems := registry.New(dir).Scan()
	out := make([]appCard, 0, len(apps))
	for _, app := range apps {
		entry := app.Entrypoint
		if entry == "" {
			entry = app.Exec
		}
		out = append(out, appCard{
			ID: app.ID, Name: app.Name, Category: app.Category, Icon: app.Icon,
			Accent: app.Accent, Badge: app.Badge, Subtitle: app.Description,
			Entrypoint: entry, Args: app.Args,
		})
	}
	return out, problems
}
