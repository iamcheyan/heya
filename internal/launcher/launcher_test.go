package launcher

import (
	"testing"

	"github.com/iamcheyan/heya/internal/input"
	"github.com/iamcheyan/heya/internal/model"
)

func testLauncher() *Launcher {
	page := model.Page{Title: "Applications", Items: []model.Item{
		{Name: "Clock", Exec: "/usr/bin/date"},
		{Name: "System", Page: "system"},
		{Name: "About", Page: "about"},
	}}
	l := New(page, nil, nil)
	l.AddPage("system", model.Page{Title: "System"})
	l.AddPage("about", model.Page{Title: "About"})
	return l
}

func TestNavigationWraps(t *testing.T) {
	l := testLauncher()
	if got := l.Frame().Selected; got != 0 {
		t.Fatalf("initial selection = %d", got)
	}
	l.Handle(input.Up)
	if got := l.Frame().Selected; got != 2 {
		t.Fatalf("up from first should wrap to 2, got %d", got)
	}
	l.Handle(input.Down)
	if got := l.Frame().Selected; got != 0 {
		t.Fatalf("down from last should wrap to 0, got %d", got)
	}
}

func TestConfirmOnExecReturnsCommand(t *testing.T) {
	l := testLauncher()
	cmd := l.Handle(input.Confirm)
	if cmd.Kind != CmdExec || cmd.Exec != "/usr/bin/date" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestPageNavigationAndCancel(t *testing.T) {
	l := testLauncher()
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	if l.Path() != "apps>system" {
		t.Fatalf("path = %s, want apps>system", l.Path())
	}
	cmd := l.Handle(input.Cancel)
	if cmd.Kind != CmdNone {
		t.Fatalf("cancel from subpage should not quit: %+v", cmd)
	}
	cmd = l.Handle(input.Cancel)
	if cmd.Kind != CmdQuit {
		t.Fatalf("cancel at root should quit: %+v", cmd)
	}
}

func TestEmptyPageIsSafe(t *testing.T) {
	l := New(model.Page{Title: "Empty"}, nil, nil)
	if cmd := l.Handle(input.Confirm); cmd.Kind != CmdNone {
		t.Fatalf("confirm on empty page: %+v", cmd)
	}
	if rows := l.Frame().Rows; len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
}

func TestSystemRowsShowUnavailableWithoutProbe(t *testing.T) {
	l := testLauncher()
	l.Handle(input.Down)
	l.Handle(input.Confirm)
	rows := l.Frame().Rows
	if len(rows) == 0 {
		t.Fatal("system page should have rows")
	}
	found := false
	for _, row := range rows {
		if row.Label == "Battery" && row.Value == "Unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Battery=Unavailable, got %+v", rows)
	}
}

func TestSelectRescansApps(t *testing.T) {
	l := testLauncher()
	calls := 0
	l.Refresh = func() model.Page {
		calls++
		return model.Page{Title: "Applications", Items: []model.Item{{Name: "New", Exec: "/bin/true"}}}
	}
	l.Handle(input.Select)
	if calls != 1 {
		t.Fatalf("refresh called %d times, want 1", calls)
	}
	rows := l.Frame().Rows
	if len(rows) != 1 || rows[0].Label != "New" {
		t.Fatalf("apps not replaced: %+v", rows)
	}
	if l.Message() == "" {
		t.Fatal("expected a rescan message")
	}
}
