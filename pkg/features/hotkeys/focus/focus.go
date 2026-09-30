// Package focus asks GNOME Shell which window has the keyboard focus.
//
// Wayland does not tell applications about other applications' windows, so
// osxd ships a small GNOME Shell extension (contrib/gnome-shell-extension)
// that answers over D-Bus.
package focus

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// ExtensionUUID identifies the GNOME Shell extension.
const ExtensionUUID = "osxd@nicklasfrahm.dev"

const (
	dest   = "org.gnome.Shell"
	path   = "/dev/nicklasfrahm/Osxd"
	method = "dev.nicklasfrahm.Osxd.Focus.WMClass"
)

// timeout bounds a lookup: the keyboard waits for the answer.
const timeout = 150 * time.Millisecond

// Terminals are the WM classes (Wayland app IDs) of common terminal
// emulators, in lower case.
var Terminals = []string{
	"org.gnome.terminal", "gnome-terminal", "gnome-terminal-server",
	"org.gnome.ptyxis", "org.gnome.ptyxis.devel",
	"org.gnome.console", "kgx",
	"com.mitchellh.ghostty", "kitty", "alacritty", "org.wezfurlong.wezterm",
	"foot", "footclient", "com.gexperts.tilix", "tilix", "terminator",
	"org.kde.konsole", "konsole", "com.raggesilver.blackbox",
	"io.elementary.terminal", "xfce4-terminal", "xterm", "uxterm", "urxvt",
	"tabby", "rio",
}

// Client talks to the extension over the session bus.
type Client struct{ obj dbus.BusObject }

// Connect connects to the session bus.
func Connect() (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	return &Client{obj: conn.Object(dest, path)}, nil
}

// WMClass returns the WM class of the focused window, or "" if no window
// has focus. It fails if the extension is not running.
func (c *Client) WMClass() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var class string
	err := c.obj.CallWithContext(ctx, method, 0).Store(&class)
	return class, err
}

// IsTerminal reports whether wmClass belongs to a terminal emulator, either
// one of Terminals or one of extra.
func IsTerminal(wmClass string, extra []string) bool {
	c := strings.ToLower(wmClass)
	return slices.Contains(Terminals, c) || slices.ContainsFunc(extra, func(e string) bool { return strings.EqualFold(e, c) })
}
