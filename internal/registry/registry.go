// Package registry composes discovery providers into one application list.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iamcheyan/heya/internal/model"
)

// Provider discovers ordinary applications. Games and books use this same contract.
type Provider interface{ Applications() ([]model.App, []error) }
type Registry struct{ providers []Provider }

func New(dir string) *Registry                         { return NewWithProviders(NewApplicationsProvider(dir)) }
func NewWithProviders(providers ...Provider) *Registry { return &Registry{providers: providers} }
func (r *Registry) Scan() ([]model.App, []error) {
	var apps []model.App
	var problems []error
	for _, provider := range r.providers {
		found, errs := provider.Applications()
		apps = append(apps, found...)
		problems = append(problems, errs...)
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Category != apps[j].Category {
			return apps[i].Category < apps[j].Category
		}
		return apps[i].Name < apps[j].Name
	})
	return apps, problems
}

type ApplicationsProvider struct{ Root string }

func NewApplicationsProvider(dir string) *ApplicationsProvider {
	return &ApplicationsProvider{Root: dir}
}
func (r *ApplicationsProvider) Applications() ([]model.App, []error) {
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	var apps []model.App
	var problems []error
	ids := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest := filepath.Join(r.Root, entry.Name(), "app.json")
		var app model.App
		if err := model.LoadJSON(manifest, &app); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		app.Normalize(entry.Name())
		if strings.TrimSpace(app.ID) == "" {
			problems = append(problems, fmt.Errorf("%s: application id is empty", entry.Name()))
			continue
		}
		if err := app.Validate(); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if ids[app.ID] {
			problems = append(problems, fmt.Errorf("%s: duplicate application id %q", entry.Name(), app.ID))
			continue
		}
		ids[app.ID] = true
		if !app.Hidden {
			apps = append(apps, app)
		}
	}
	return apps, problems
}
func Page(title string, apps []model.App) model.Page {
	page := model.Page{Title: title}
	for _, app := range apps {
		page.Items = append(page.Items, model.Item{Name: app.Name, Icon: app.Icon, Exec: app.Entrypoint, Args: app.Args, Environment: app.Environment, WorkingDirectory: app.WorkingDirectory, Permissions: app.Permissions, Features: app.Features})
	}
	return page
}
