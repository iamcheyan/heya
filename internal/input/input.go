// Package input turns SDL keyboard and controller events into semantic
// actions. Physical button layouts differ per device, so the mapping is data,
// not code.
package input

import (
	"errors"
	"fmt"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

// Action is a device-independent navigation intent.
type Action int

const (
	None Action = iota
	Up
	Down
	Left
	Right
	Confirm
	Cancel
	PageUp
	PageDown
	Start
	Select
	Quit
)

// String makes failures readable in tests and logs.
func (a Action) String() string {
	switch a {
	case Up:
		return "up"
	case Down:
		return "down"
	case Left:
		return "left"
	case Right:
		return "right"
	case Confirm:
		return "confirm"
	case Cancel:
		return "cancel"
	case PageUp:
		return "page-up"
	case PageDown:
		return "page-down"
	case Start:
		return "start"
	case Select:
		return "select"
	case Quit:
		return "quit"
	default:
		return "none"
	}
}

// Mapping binds key names and controller buttons to actions.
type Mapping struct {
	Keys map[string][]string `json:"keys"`
	Pad  map[string][]string `json:"pad"`
}

// DefaultMapping matches the documented heya input standard.
func DefaultMapping() Mapping {
	return Mapping{
		Keys: map[string][]string{
			"confirm":   {"Return"},
			"cancel":    {"Escape", "Backspace"},
			"up":        {"Up", "W", "w"},
			"down":      {"Down", "S", "s"},
			"left":      {"Left", "A", "a"},
			"right":     {"Right", "D", "d"},
			"page-up":   {"Q", "q"},
			"page-down": {"E", "e"},
			"start":     {"Space"},
			"select":    {"Tab"},
			"quit":      {"F10"},
		},
		Pad: map[string][]string{
			"confirm":   {"a"},
			"cancel":    {"b"},
			"up":        {"dpup"},
			"down":      {"dpdown"},
			"left":      {"dpleft"},
			"right":     {"dpright"},
			"page-up":   {"leftshoulder"},
			"page-down": {"rightshoulder"},
			"start":     {"start"},
			"select":    {"back"},
		},
	}
}

var keyActions = map[string]Action{
	"confirm": Confirm, "cancel": Cancel, "up": Up, "down": Down,
	"left": Left, "right": Right, "page-up": PageUp, "page-down": PageDown,
	"start": Start, "select": Select, "quit": Quit,
}

var padButtons = map[string]sdl.GameControllerButton{
	"a":             sdl.CONTROLLER_BUTTON_A,
	"b":             sdl.CONTROLLER_BUTTON_B,
	"x":             sdl.CONTROLLER_BUTTON_X,
	"y":             sdl.CONTROLLER_BUTTON_Y,
	"back":          sdl.CONTROLLER_BUTTON_BACK,
	"start":         sdl.CONTROLLER_BUTTON_START,
	"leftshoulder":  sdl.CONTROLLER_BUTTON_LEFTSHOULDER,
	"rightshoulder": sdl.CONTROLLER_BUTTON_RIGHTSHOULDER,
	"dpup":          sdl.CONTROLLER_BUTTON_DPAD_UP,
	"dpdown":        sdl.CONTROLLER_BUTTON_DPAD_DOWN,
	"dpleft":        sdl.CONTROLLER_BUTTON_DPAD_LEFT,
	"dpright":       sdl.CONTROLLER_BUTTON_DPAD_RIGHT,
}

// Validate reports unknown action names or buttons.
func (m Mapping) Validate() error {
	for name := range m.Keys {
		if _, ok := keyActions[name]; !ok {
			return fmt.Errorf("unknown key action %q", name)
		}
	}
	for name, buttons := range m.Pad {
		if _, ok := keyActions[name]; !ok {
			return fmt.Errorf("unknown pad action %q", name)
		}
		for _, button := range buttons {
			if _, ok := padButtons[strings.ToLower(button)]; !ok {
				return fmt.Errorf("unknown pad button %q", button)
			}
		}
	}
	return nil
}

// Handler resolves SDL events to actions using a mapping.
type Handler struct {
	Mapping Mapping
	keyMap  map[sdl.Keycode]Action
	padMap  map[sdl.GameControllerButton]Action
}

// NewHandler compiles a mapping into lookup tables.
func NewHandler(m Mapping) (*Handler, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	h := &Handler{Mapping: m, keyMap: map[sdl.Keycode]Action{}, padMap: map[sdl.GameControllerButton]Action{}}
	for name, keys := range m.Keys {
		action := keyActions[name]
		for _, key := range keys {
			code := sdl.GetKeyFromName(key)
			if code == sdl.K_UNKNOWN {
				return nil, fmt.Errorf("unknown key name %q", key)
			}
			h.keyMap[code] = action
		}
	}
	for name, buttons := range m.Pad {
		action := keyActions[name]
		for _, button := range buttons {
			h.padMap[padButtons[strings.ToLower(button)]] = action
		}
	}
	if len(h.keyMap) == 0 {
		return nil, errors.New("input mapping has no keys")
	}
	return h, nil
}

// Key resolves a keycode to an action.
func (h *Handler) Key(code sdl.Keycode) Action {
	if action, ok := h.keyMap[code]; ok {
		return action
	}
	return None
}

// Button resolves a controller button to an action.
func (h *Handler) Button(button sdl.GameControllerButton) Action {
	if action, ok := h.padMap[button]; ok {
		return action
	}
	return None
}
