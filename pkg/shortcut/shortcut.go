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
	customPrefix = mediaSchema + ".custom-keybinding:"
	customPath   = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/osxd/"
	customSchema = customPrefix + customPath

	mutterSchema = "org.gnome.mutter"
	centerKey    = "center-new-windows"
)

// Setting is the original value of a gsettings key that Install changed.
type Setting struct {
	Schema string `json:"schema"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// Previous holds the settings Install changed, so Restore can put them back.
type Previous struct {
	// Bindings are the keys that used Super+Space before Install freed it.
	Bindings         []Setting
	CenterNewWindows string
}

// InputSwitch returns the Setting for an original switch-input-source value,
// as recorded by versions that only freed that one key.
func InputSwitch(value string) Setting {
	return Setting{Schema: wmSchema, Key: inputKey, Value: value}
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

// Install frees Super+Space from every keybinding that uses it (usually
// input-source switching), binds it to command and centres new windows. It
// returns the original values for Restore.
func Install(run Runner, command string) (Previous, error) {
	var p Previous
	var err error
	if p.CenterNewWindows, err = Center(run); err != nil {
		return p, err
	}
	if p.Bindings, err = free(run); err != nil {
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

// free removes Super+Space from every keybinding schema and from other
// custom keybindings, returning the original value of each key it changed.
func free(run Runner) ([]Setting, error) {
	var changed []Setting
	release := func(s Setting) error {
		v, ok := without(s.Value, Binding)
		if !ok {
			return nil
		}
		if _, err := run("set", s.Schema, s.Key, v); err != nil {
			return err
		}
		changed = append(changed, s)
		return nil
	}

	raw, err := run("list-schemas")
	if err != nil {
		return nil, err
	}
	for _, schema := range strings.Fields(raw) {
		if !strings.HasSuffix(schema, ".keybindings") && schema != mediaSchema {
			continue
		}
		// Each line is `schema key value`.
		lines, err := run("list-recursively", schema)
		if err != nil {
			return changed, err
		}
		for _, line := range strings.Split(lines, "\n") {
			f := strings.SplitN(line, " ", 3)
			if len(f) != 3 {
				continue
			}
			if err := release(Setting{Schema: f[0], Key: f[1], Value: f[2]}); err != nil {
				return changed, err
			}
		}
	}

	raw, err = run("get", mediaSchema, customKey)
	if err != nil {
		return changed, err
	}
	for _, path := range parseList(raw) {
		if path == customPath {
			continue
		}
		value, err := run("get", customPrefix+path, "binding")
		if err != nil {
			return changed, err
		}
		if err := release(Setting{Schema: customPrefix + path, Key: "binding", Value: value}); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// without returns value with accel removed and reports whether it was present.
// value is a gsettings string array or string; other types never match.
func without(value, accel string) (string, bool) {
	isList := strings.HasPrefix(value, "[") || strings.HasPrefix(value, "@as")
	if !isList && !strings.HasPrefix(value, "'") {
		return value, false
	}
	var kept []string
	found := false
	for _, it := range parseList(value) {
		if strings.EqualFold(it, accel) {
			found = true
		} else {
			kept = append(kept, it)
		}
	}
	switch {
	case !found:
		return value, false
	case isList:
		return formatList(kept), true
	default:
		return "''", true
	}
}

// Restore undoes Install. previous is the value Install returned.
//
// Order matters: the custom keybinding must release Super+Space before the
// original bindings take it back, otherwise GNOME's grab for them fails and
// the key stays dead until the next login. Clearing the binding first makes
// the settings daemon drop its grab, but it does so asynchronously, so settle
// is called to give it time before the original bindings are written back.
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

	for _, s := range previous.Bindings {
		if _, err := run("set", s.Schema, s.Key, s.Value); err != nil {
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
