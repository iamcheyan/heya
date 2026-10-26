package layout

import "testing"

func TestStandardDirectories(t *testing.T) {
	if len(Directories) != 6 {
		t.Fatalf("got %d directories, want 6", len(Directories))
	}
	want := []string{"/storage/heya/apps", "/storage/heya/data", "/storage/heya/cache", "/storage/heya/logs", "/storage/heya/themes", "/storage/heya/downloads"}
	for i := range want {
		if Directories[i] != want[i] {
			t.Errorf("directory[%d] = %q, want %q", i, Directories[i], want[i])
		}
	}
}
