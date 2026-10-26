// Package registry discovers applications under the apps directory.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/iamcheyan/heya/internal/model"
)

// Registry scans a directory of applications, one subdirectory per app.
type Registry struct {
	Root string
}

// New returns a registry rooted at dir.
func New(dir string) *Registry {
	return &Registry{Root: dir}
}

// Scan returns the valid applications sorted by name, plus one error per
// rejected entry. A missing root is not an error: it yields an empty list.
func (r *Registry) Scan() ([]model.App, []error) {
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}

	var apps []model.App
	var problems []error

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
		if err := app.Validate(); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if app.Hidden {
			continue
		}
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Category != apps[j].Category {
			return apps[i].Category < apps[j].Category
		}
		return apps[i].Name < apps[j].Name
	})
	return apps, problems
}

// Page converts scanned apps into a menu page.
func Page(title string, apps []model.App) model.Page {
	page := model.Page{Title: title}
	for _, app := range apps {
		page.Items = append(page.Items, model.Item{
			Name: app.Name,
			Icon: app.Icon,
			Exec: app.Exec,
			Args: app.Args,
		})
	}
	return page
}
