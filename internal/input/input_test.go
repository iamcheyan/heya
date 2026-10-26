package input

import (
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

func TestDefaultMappingValidates(t *testing.T) {
	if err := DefaultMapping().Validate(); err != nil {
		t.Fatalf("default mapping invalid: %v", err)
	}
}

func TestHandlerResolvesKeys(t *testing.T) {
	handler, err := NewHandler(DefaultMapping())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]Action{
		"Return": Confirm,
		"Escape": Cancel,
		"Up":     Up,
		"w":      Up,
		"Down":   Down,
		"q":      PageUp,
		"e":      PageDown,
		"Space":  Start,
		"Tab":    Select,
		"F10":    Quit,
	}
	for name, want := range cases {
		code := sdl.GetKeyFromName(name)
		if code == sdl.K_UNKNOWN {
			t.Fatalf("test key %q unknown to SDL", name)
		}
		if got := handler.Key(code); got != want {
			t.Fatalf("key %q resolved to %s, want %s", name, got, want)
		}
	}
}

func TestHandlerResolvesAnbernicSwap(t *testing.T) {
	handler, err := NewHandler(DefaultMapping())
	if err != nil {
		t.Fatal(err)
	}
	// Physical A on Anbernic hardware reports as SDL button B.
	if got := handler.Button(sdl.CONTROLLER_BUTTON_B); got != Confirm {
		t.Fatalf("pad B = %s, want confirm", got)
	}
	if got := handler.Button(sdl.CONTROLLER_BUTTON_A); got != Cancel {
		t.Fatalf("pad A = %s, want cancel", got)
	}
}

func TestMappingRejectsUnknownNames(t *testing.T) {
	bad := Mapping{Keys: map[string][]string{"fly": {"Up"}}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected unknown action error")
	}
	badButton := Mapping{Pad: map[string][]string{"confirm": {"thumbr"}}}
	if err := badButton.Validate(); err == nil {
		t.Fatal("expected unknown button error")
	}
}
