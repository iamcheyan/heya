package registry

import (
	"os"
	"path/filepath"
	"testing"
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
