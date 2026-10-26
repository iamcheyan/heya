package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamcheyan/heya/internal/model"
)

func writeApp(t *testing.T, root, dir, body string) {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "app.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanCollectsValidApps(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "music", `{"name":"Music","exec":"/usr/bin/music","icon":"nf-md-music"}`)
	writeApp(t, root, "video", `{"name":"Video","exec":"/usr/bin/video"}`)
	writeApp(t, root, "hidden", `{"name":"Hidden","exec":"/bin/true","hidden":true}`)
	writeApp(t, root, "broken", `{"name":"Broken"}`)
	writeApp(t, root, "badjson", `{"name":`)
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	apps, problems := New(root).Scan()
	if len(apps) != 2 {
		t.Fatalf("got %d apps, want 2: %+v", len(apps), apps)
	}
	if apps[0].Name != "Music" || apps[1].Name != "Video" {
		t.Fatalf("unexpected order: %+v", apps)
	}
	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2 (broken, badjson): %v", len(problems), problems)
	}
}

func TestScanLegacyExamples(t *testing.T) {
	apps, problems := New("../../examples/apps").Scan()
	if len(problems) != 0 || len(apps) != 2 {
		t.Fatalf("legacy examples: %d apps, %v", len(apps), problems)
	}
	if apps[0].ID != "clock" || apps[1].ID != "hello" {
		t.Fatalf("legacy IDs: %+v", apps)
	}
}

func TestScanRejectsUnknownManifestVersion(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "future", `{"manifestVersion":2,"id":"future","name":"Future","entrypoint":"/bin/true"}`)
	apps, problems := New(root).Scan()
	if len(apps) != 0 || len(problems) != 1 {
		t.Fatalf("future manifest should be isolated: %v %v", apps, problems)
	}
}

func TestScanManifestV1AndDuplicateID(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "one", `{"manifestVersion":1,"id":"reader","name":"Reader","entrypoint":"/usr/bin/reader","permissions":["storage"],"features":["resume-support"]}`)
	writeApp(t, root, "two", `{"manifestVersion":1,"id":"reader","name":"Duplicate","entrypoint":"/bin/true"}`)
	apps, problems := New(root).Scan()
	if len(apps) != 1 || apps[0].ID != "reader" || apps[0].Entrypoint != "/usr/bin/reader" {
		t.Fatalf("unexpected apps: %+v", apps)
	}
	if len(problems) != 1 {
		t.Fatalf("duplicate id should be reported: %v", problems)
	}
}

func TestProviderAggregation(t *testing.T) {
	provider := fixedProvider{app: model.App{ID: "book", Name: "Books", ManifestVersion: 1, Entrypoint: "/usr/bin/books"}}
	apps, problems := NewWithProviders(provider).Scan()
	if len(problems) != 0 || len(apps) != 1 || apps[0].Name != "Books" {
		t.Fatalf("provider result: %+v %v", apps, problems)
	}
}

type fixedProvider struct{ app model.App }

func (p fixedProvider) Applications() ([]model.App, []error) { return []model.App{p.app}, nil }

func TestScanMissingRootIsEmpty(t *testing.T) {
	apps, problems := New(filepath.Join(t.TempDir(), "nope")).Scan()
	if len(apps) != 0 || len(problems) != 0 {
		t.Fatalf("missing root should be empty, got %v %v", apps, problems)
	}
}

func TestPageAddsNavItems(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "music", `{"name":"Music","exec":"/usr/bin/music"}`)
	apps, _ := New(root).Scan()
	page := Page("Applications", apps)
	if len(page.Items) != 1 || page.Items[0].Name != "Music" {
		t.Fatalf("unexpected page: %+v", page)
	}
}
