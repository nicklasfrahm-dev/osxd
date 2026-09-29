// Package shortcut binds Super+Space to the launcher on GNOME.
//
// Wayland does not let ordinary applications grab global hotkeys, so the
// binding is registered as a GNOME custom keybinding that runs the launcher
// binary, which then toggles the already-running instance.
package shortcut

import (
	"fmt"
	"strings"
)

const (
	Binding = "<Super>space"

	wmSchema     = "org.gnome.desktop.wm.keybindings"
	inputKey     = "switch-input-source"
	mediaSchema  = "org.gnome.settings-daemon.plugins.media-keys"
	customKey    = "custom-keybindings"
	customPath   = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/osxd/"
	customSchema = mediaSchema + ".custom-keybinding:" + customPath

	mutterSchema = "org.gnome.mutter"
	centerKey    = "center-new-windows"
)

// Previous holds the settings Install changed, so Restore can put them back.
type Previous struct {
	InputSwitch      string
	CenterNewWindows string
}

// Runner executes `gsettings args...` and returns trimmed stdout.
type Runner func(args ...string) (string, error)

// Center makes GNOME open new windows centred (Wayland clients cannot place
// their own windows). It returns the previous value for Restore. This is a
// global setting and affects every application.
func Center(run Runner) (previous string, err error) {
	previous, err = run("get", mutterSchema, centerKey)
	if err != nil {
		return "", err
	}
	_, err = run("set", mutterSchema, centerKey, "true")
	return previous, err
}

// Install frees Super+Space from input-source switching, binds it to command
// and centres new windows. It returns the original values for Restore.
func Install(run Runner, command string) (Previous, error) {
	var p Previous
	var err error
	if p.CenterNewWindows, err = Center(run); err != nil {
		return p, err
	}
	previous, err := run("get", wmSchema, inputKey)
	if err != nil {
		return p, err
	}
	p.InputSwitch = previous
	keys := parseList(previous)
	if _, err = run("set", wmSchema, inputKey, formatList(remove(keys, Binding))); err != nil {
		return p, err
	}

	raw, err := run("get", mediaSchema, customKey)
	if err != nil {
		return p, err
	}
	paths := parseList(raw)
	if !contains(paths, customPath) {
		paths = append(paths, customPath)
	}
	if _, err = run("set", mediaSchema, customKey, formatList(paths)); err != nil {
		return p, err
	}
	for _, kv := range [][2]string{{"name", "osxd launcher"}, {"command", command}, {"binding", Binding}} {
		if _, err = run("set", customSchema, kv[0], kv[1]); err != nil {
			return p, err
		}
	}
	return p, nil
}

// Restore undoes Install. previous is the value Install returned.
//
// Order matters: the custom keybinding must release Super+Space before the
// input-source binding takes it back, otherwise GNOME's grab for the input
// switch fails and the key stays dead until the next login. Clearing the
// binding first makes the settings daemon drop its grab, but it does so
// asynchronously, so settle is called to give it time before the original
// binding is written back.
func Restore(run Runner, previous Previous, settle func()) error {
	if _, err := run("set", customSchema, "binding", ""); err != nil {
		return err
	}
	raw, err := run("get", mediaSchema, customKey)
	if err != nil {
		return err
	}
	paths := remove(parseList(raw), customPath)
	if _, err := run("set", mediaSchema, customKey, formatList(paths)); err != nil {
		return err
	}
	if _, err := run("reset-recursively", customSchema); err != nil {
		return err
	}
	settle()

	if previous.InputSwitch != "" {
		if _, err := run("set", wmSchema, inputKey, previous.InputSwitch); err != nil {
			return err
		}
	}
	if previous.CenterNewWindows != "" {
		if _, err := run("set", mutterSchema, centerKey, previous.CenterNewWindows); err != nil {
			return err
		}
	}
	return nil
}

// parseList parses a gsettings string array such as `['a', 'b']` or `@as []`.
func parseList(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '\'')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(s[i+1:], '\'')
		if j < 0 {
			return out
		}
		out = append(out, s[i+1:i+1+j])
		s = s[i+j+2:]
	}
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "@as []"
	}
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = fmt.Sprintf("'%s'", it)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func contains(items []string, s string) bool {
	for _, it := range items {
		if it == s {
			return true
		}
	}
	return false
}

func remove(items []string, s string) []string {
	var out []string
	for _, it := range items {
		if it != s {
			out = append(out, it)
		}
	}
	return out
}
