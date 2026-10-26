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

func TestHandlerResolvesH700Buttons(t *testing.T) {
	handler, err := NewHandler(DefaultMapping())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[sdl.GameControllerButton]Action{
		sdl.CONTROLLER_BUTTON_A:             Confirm,
		sdl.CONTROLLER_BUTTON_B:             Cancel,
		sdl.CONTROLLER_BUTTON_DPAD_UP:       Up,
		sdl.CONTROLLER_BUTTON_DPAD_DOWN:     Down,
		sdl.CONTROLLER_BUTTON_DPAD_LEFT:     Left,
		sdl.CONTROLLER_BUTTON_DPAD_RIGHT:    Right,
		sdl.CONTROLLER_BUTTON_LEFTSHOULDER:  PageUp,
		sdl.CONTROLLER_BUTTON_RIGHTSHOULDER: PageDown,
		sdl.CONTROLLER_BUTTON_START:         Start,
		sdl.CONTROLLER_BUTTON_BACK:          Select,
		sdl.CONTROLLER_BUTTON_X:             None,
		sdl.CONTROLLER_BUTTON_Y:             None,
	}
	for button, want := range cases {
		if got := handler.Button(button); got != want {
			t.Errorf("button %v = %s, want %s", button, got, want)
		}
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
