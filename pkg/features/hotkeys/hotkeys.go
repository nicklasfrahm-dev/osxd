// Package hotkeys makes macOS-style Super shortcuts (Super+C to copy and so
// on) work in every application, including terminals.
//
// Wayland does not let applications see or send other applications' key
// presses, so osxd works one layer below: it takes over the keyboards in
// /dev/input and re-sends their key presses, rewritten by package remap,
// through a virtual keyboard made with /dev/uinput.
package hotkeys

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/nicklasfrahm/osxd/pkg/features/hotkeys/evdev"
	"github.com/nicklasfrahm/osxd/pkg/features/hotkeys/focus"
	"github.com/nicklasfrahm/osxd/pkg/features/hotkeys/remap"
	"github.com/nicklasfrahm/osxd/pkg/gsettings"
)

// rescanEvery is how often new keyboards, such as a USB keyboard plugged in
// later, are looked for.
const rescanEvery = 2 * time.Second

const (
	shellSchema    = "org.gnome.shell"
	enabledKey     = "enabled-extensions"
	disabledKey    = "disabled-extensions"
	userExtsOffKey = "disable-user-extensions"
	extensionUUID  = focus.ExtensionUUID
)

// EnableExtension turns on the GNOME Shell extension that reports the
// focused window. GNOME on Wayland only loads a newly installed extension
// after logging out and back in.
func EnableExtension(run gsettings.Runner) error {
	if err := edit(run, disabledKey, false); err != nil {
		return err
	}
	return edit(run, enabledKey, true)
}

// DisableExtension undoes EnableExtension.
func DisableExtension(run gsettings.Runner) error { return edit(run, enabledKey, false) }

// edit adds the extension to, or removes it from, the list in key.
func edit(run gsettings.Runner, key string, add bool) error {
	raw, err := run("get", shellSchema, key)
	if err != nil {
		return err
	}
	list := gsettings.ParseList(raw)
	has := slices.Contains(list, extensionUUID)
	switch {
	case add && !has:
		list = append(list, extensionUUID)
	case !add && has:
		list = slices.DeleteFunc(list, func(s string) bool { return s == extensionUUID })
	default:
		return nil
	}
	_, err = run("set", shellSchema, key, gsettings.FormatList(list))
	return err
}

// UserExtensionsDisabled reports whether GNOME is set to load no user
// extensions at all, which also stops osxd's.
func UserExtensionsDisabled(run gsettings.Runner) bool {
	v, err := run("get", shellSchema, userExtsOffKey)
	return err == nil && v == "true"
}

// Run takes over all keyboards and remaps their Super shortcuts until an
// error stops it. terminals are extra WM classes to treat as terminals.
func Run(terminals []string) error {
	virtual, err := evdev.NewVirtual()
	if err != nil {
		return err
	}
	defer virtual.Close()

	client, err := focus.Connect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hotkeys: cannot reach the session bus:", err)
	}
	var warned bool
	mode := func() remap.Mode {
		if client == nil {
			return remap.Unknown
		}
		class, err := client.WMClass()
		if err != nil {
			// Swallow the key rather than risk sending Ctrl+C to a
			// terminal.
			if !warned {
				warned = true
				fmt.Fprintln(os.Stderr, "hotkeys: cannot tell which window has focus, so Super+A, C, X, V, F, Z, Y and S do nothing. "+
					"Log out and back in once to load the osxd GNOME Shell extension:", err)
			}
			return remap.Unknown
		}
		warned = false
		if focus.IsTerminal(class, terminals) {
			return remap.Terminal
		}
		return remap.App
	}

	kb := &keyboards{grabbed: map[string]*evdev.Keyboard{}, events: make(chan remap.Event, 256)}
	go kb.forwardLEDs(virtual)
	go func() {
		for {
			kb.scan()
			time.Sleep(rescanEvery)
		}
	}()

	r := remap.New(mode)
	for e := range kb.events {
		for _, o := range r.Process(e) {
			if err := virtual.Key(o.Code, o.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

// keyboards are the grabbed physical keyboards. Their key events are merged
// into one channel, like a single keyboard.
type keyboards struct {
	mu      sync.Mutex
	grabbed map[string]*evdev.Keyboard
	events  chan remap.Event
}

// scan grabs keyboards that are not grabbed yet.
func (k *keyboards) scan() {
	paths, err := evdev.Keyboards()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hotkeys:", err)
		return
	}
	for _, p := range paths {
		k.mu.Lock()
		_, ok := k.grabbed[p]
		k.mu.Unlock()
		if ok {
			continue
		}
		dev, err := evdev.Grab(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hotkeys:", err)
			continue
		}
		k.mu.Lock()
		k.grabbed[p] = dev
		k.mu.Unlock()
		go k.read(dev)
	}
}

// read forwards dev's key events until it is unplugged.
func (k *keyboards) read(dev *evdev.Keyboard) {
	held := map[uint16]bool{}
	for {
		evs, err := dev.Read()
		if err != nil {
			break
		}
		for _, e := range evs {
			if e.Type != evdev.EvKey {
				continue
			}
			held[e.Code] = e.Value != remap.Release
			k.events <- remap.Event{Code: e.Code, Value: e.Value}
		}
	}
	// Release what was held when the keyboard went away, or those keys
	// would stay pressed.
	for code, down := range held {
		if down {
			k.events <- remap.Event{Code: code, Value: remap.Release}
		}
	}
	k.mu.Lock()
	delete(k.grabbed, dev.Path)
	k.mu.Unlock()
	dev.Close()
}

// forwardLEDs mirrors the virtual keyboard's LEDs, such as Caps Lock, on the
// physical keyboards.
func (k *keyboards) forwardLEDs(virtual *evdev.Virtual) {
	for {
		leds, err := virtual.ReadLEDs()
		if err != nil {
			return
		}
		k.mu.Lock()
		for _, dev := range k.grabbed {
			for _, l := range leds {
				dev.SetLED(l.Code, l.Value != 0)
			}
		}
		k.mu.Unlock()
	}
}
