// Package remap turns macOS-style Super shortcuts into the Ctrl shortcuts
// that Linux applications understand.
//
// While Super is held, pressing one of the mapped keys swaps Super for Ctrl
// (Ctrl+Shift in terminals) on the output, so Super+C reaches an editor as
// Ctrl+C and a terminal as Ctrl+Shift+C. Pressing any other key while Super
// is still held swaps back, so Super+Left and friends keep working.
//
// Ctrl is pressed before Super is released: GNOME opens the Activities
// overview when Super is pressed and released on its own, and any other key
// in between cancels that.
package remap

import "slices"

// Linux input key codes, from linux/input-event-codes.h.
const (
	KeyLeftCtrl   uint16 = 29
	KeyLeftShift  uint16 = 42
	KeyRightShift uint16 = 54
	KeyLeftAlt    uint16 = 56
	KeyRightCtrl  uint16 = 97
	KeyRightAlt   uint16 = 100
	KeyLeftMeta   uint16 = 125
	KeyRightMeta  uint16 = 126

	KeyA uint16 = 30
	KeyC uint16 = 46
	KeyF uint16 = 33
	KeyS uint16 = 31
	KeyV uint16 = 47
	KeyX uint16 = 45
	KeyY uint16 = 21
	KeyZ uint16 = 44
)

// Key event values.
const (
	Release int32 = 0
	Press   int32 = 1
	Repeat  int32 = 2
)

// Mapped are the keys that Super turns into Ctrl shortcuts: select all,
// copy, cut, paste, find, undo, redo and save.
var Mapped = map[uint16]bool{KeyA: true, KeyC: true, KeyX: true, KeyV: true, KeyF: true, KeyZ: true, KeyY: true, KeyS: true}

// Mode says how the focused window wants its shortcuts.
type Mode int

const (
	// Unknown swallows mapped keys, for when the focused window cannot be
	// told apart: Super+C must neither type a "c" (as passing it on would
	// in many terminals) nor send Ctrl+C, which would interrupt a terminal.
	Unknown Mode = iota
	// App sends Ctrl+key.
	App
	// Terminal sends Ctrl+Shift+key, because terminals pass Ctrl+key to the
	// program running in them (Ctrl+C would interrupt it).
	Terminal
)

// Event is a key event: a key code and Release, Press or Repeat.
type Event struct {
	Code  uint16
	Value int32
}

// Remapper rewrites a stream of key events. It is not safe for concurrent
// use; feed it the events of all keyboards from one goroutine.
type Remapper struct {
	// Mode is asked for the focused window's mode when a swap starts.
	Mode func() Mode

	in       map[uint16]bool // keys held on the physical keyboards
	out      map[uint16]bool // keys held on the output
	swapped  bool            // Super is replaced by Ctrl on the output
	swallow  bool            // mapped keys are dropped during this swap
	injected []uint16        // modifiers pressed on the output for the swap
}

// New returns a Remapper that asks mode for the focused window's mode.
func New(mode func() Mode) *Remapper {
	return &Remapper{Mode: mode, in: map[uint16]bool{}, out: map[uint16]bool{}}
}

func isSuper(code uint16) bool { return code == KeyLeftMeta || code == KeyRightMeta }

// isModifier reports whether code is Ctrl, Shift or Alt, which can join a
// swapped shortcut (Super+Shift+Z) without ending the swap.
func isModifier(code uint16) bool {
	switch code {
	case KeyLeftCtrl, KeyRightCtrl, KeyLeftShift, KeyRightShift, KeyLeftAlt, KeyRightAlt:
		return true
	}
	return false
}

func (r *Remapper) superHeld() bool { return r.in[KeyLeftMeta] || r.in[KeyRightMeta] }

// Process takes one physical key event and returns the events to emit.
func (r *Remapper) Process(e Event) []Event {
	var out []Event
	emit := func(code uint16, value int32) {
		switch {
		case value == Press && r.out[code], value != Press && !r.out[code]:
			return // already down, or not down to release or repeat
		}
		r.out[code] = value != Release
		out = append(out, Event{code, value})
	}

	if e.Value == Repeat {
		// Keys the output holds only because of the swap never repeat.
		if !r.swapped || !isSuper(e.Code) && !r.isInjected(e.Code) {
			emit(e.Code, Repeat)
		}
		return out
	}
	r.in[e.Code] = e.Value == Press

	switch {
	case e.Value == Release:
		if isSuper(e.Code) && r.swapped {
			if !r.superHeld() {
				r.unswap(emit, false)
			}
			return out
		}
		if r.swapped && r.isInjected(e.Code) {
			return out // the swap still needs it; unswap releases it
		}
		emit(e.Code, Release)

	case isSuper(e.Code):
		if !r.swapped {
			emit(e.Code, Press)
		}

	case Mapped[e.Code] && r.superHeld():
		if !r.swapped {
			r.swap(emit)
		}
		if !r.swallow {
			emit(e.Code, Press)
		}

	default:
		if r.swapped && !isModifier(e.Code) {
			r.unswap(emit, true)
		}
		emit(e.Code, Press)
	}
	return out
}

// swap replaces Super with Ctrl (and Shift in terminals) on the output. If
// the focused window's mode is Unknown, Ctrl is still pressed, so that
// releasing Super does not open the Activities overview, but the mapped keys
// are swallowed until the swap ends.
func (r *Remapper) swap(emit func(uint16, int32)) {
	mode := Unknown
	if r.Mode != nil {
		mode = r.Mode()
	}
	r.swallow = mode == Unknown
	mods := []uint16{KeyLeftCtrl}
	if mode == Terminal {
		mods = append(mods, KeyLeftShift)
	}
	r.injected = r.injected[:0]
	for _, m := range mods {
		if !r.out[m] {
			emit(m, Press)
			r.injected = append(r.injected, m)
		}
	}
	for _, s := range []uint16{KeyLeftMeta, KeyRightMeta} {
		emit(s, Release)
	}
	r.swapped = true
}

// unswap undoes swap. If restoreSuper is set, the Super keys still held are
// pressed on the output again.
func (r *Remapper) unswap(emit func(uint16, int32), restoreSuper bool) {
	if restoreSuper {
		for _, s := range []uint16{KeyLeftMeta, KeyRightMeta} {
			if r.in[s] {
				emit(s, Press)
			}
		}
	}
	for _, m := range r.injected {
		if !r.in[m] {
			emit(m, Release)
		}
	}
	r.injected = r.injected[:0]
	r.swapped, r.swallow = false, false
}

func (r *Remapper) isInjected(code uint16) bool {
	return slices.Contains(r.injected, code)
}
