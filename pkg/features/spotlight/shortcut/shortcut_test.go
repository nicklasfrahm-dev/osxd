package shortcut

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func fakeRunner(store map[string]string, log *[]string) Runner {
	return func(args ...string) (string, error) {
		*log = append(*log, strings.Join(args, " "))
		switch args[0] {
		case "get":
			return store[args[1]+" "+args[2]], nil
		case "set":
			store[args[1]+" "+args[2]] = args[3]
		case "list-schemas":
			seen := map[string]bool{}
			for k := range store {
				if s := strings.Fields(k)[0]; !strings.Contains(s, ":") {
					seen[s] = true
				}
			}
			var out []string
			for s := range seen {
				out = append(out, s)
			}
			sort.Strings(out)
			return strings.Join(out, "\n"), nil
		case "list-recursively":
			var out []string
			for k, v := range store {
				if strings.HasPrefix(k, args[1]+" ") {
					out = append(out, k+" "+v)
				}
			}
			sort.Strings(out)
			return strings.Join(out, "\n"), nil
		}
		return "", nil
	}
}

func TestInstallAndRestore(t *testing.T) {
	store := map[string]string{
		wmSchema + " " + inputKey:      "['<Super>space', 'XF86Keyboard']",
		mediaSchema + " " + customKey:  "@as []",
		mutterSchema + " " + centerKey: "false",
	}
	var log []string
	run := fakeRunner(store, &log)

	prev, err := Install(run, "/bin/osxd")
	if err != nil {
		t.Fatal(err)
	}
	want := []Setting{InputSwitch("['<Super>space', 'XF86Keyboard']")}
	if !reflect.DeepEqual(prev.Bindings, want) || prev.CenterNewWindows != "false" {
		t.Fatalf("previous = %+v", prev)
	}
	if got := store[mutterSchema+" "+centerKey]; got != "true" {
		t.Errorf("center = %q", got)
	}
	if got := store[wmSchema+" "+inputKey]; got != "['XF86Keyboard']" {
		t.Errorf("input switch = %q", got)
	}
	if got := store[mediaSchema+" "+customKey]; got != "['"+customPath+"']" {
		t.Errorf("custom list = %q", got)
	}
	if got := store[customSchema+" binding"]; got != Binding {
		t.Errorf("binding = %q", got)
	}

	// Installing twice must not duplicate the path.
	if _, err := Install(run, "/bin/osxd"); err != nil {
		t.Fatal(err)
	}
	if got := parseList(store[mediaSchema+" "+customKey]); len(got) != 1 {
		t.Errorf("duplicate paths: %v", got)
	}

	if err := Restore(run, prev, func() {}); err != nil {
		t.Fatal(err)
	}
	if got := store[wmSchema+" "+inputKey]; got != "['<Super>space', 'XF86Keyboard']" {
		t.Errorf("restored = %q", got)
	}
	if got := store[mutterSchema+" "+centerKey]; got != "false" {
		t.Errorf("center after restore = %q", got)
	}
	if got := store[mediaSchema+" "+customKey]; got != "@as []" {
		t.Errorf("custom list after restore = %q", got)
	}
}

// Super+Space is freed from, and given back to, whatever used it, not only
// input-source switching.
func TestInstallFreesAnyBinding(t *testing.T) {
	other := "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom0/"
	shell := "org.gnome.shell.keybindings"
	store := map[string]string{
		wmSchema + " " + inputKey:                 "['XF86Keyboard']",
		wmSchema + " close":                       "['<Alt>F4']",
		shell + " toggle-overview":                "['<Super>Space', '<Super>s']",
		mediaSchema + " " + customKey:             "['" + other + "']",
		mediaSchema + " volume-step":              "6",
		customPrefix + other + " binding":         "'<Super>space'",
		mutterSchema + " " + centerKey:            "false",
		"org.gnome.desktop.interface font-name":   "'<Super>space'",
		"org.gnome.mutter.keybindings toggle-foo": "@as []",
	}
	orig := map[string]string{}
	for k, v := range store {
		orig[k] = v
	}
	var log []string
	run := fakeRunner(store, &log)

	prev, err := Install(run, "/bin/osxd")
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		wmSchema + " " + inputKey:               "['XF86Keyboard']",
		wmSchema + " close":                     "['<Alt>F4']",
		shell + " toggle-overview":              "['<Super>s']",
		customPrefix + other + " binding":       "''",
		"org.gnome.desktop.interface font-name": "'<Super>space'",
	} {
		if got := store[k]; got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if len(prev.Bindings) != 2 {
		t.Errorf("bindings = %+v", prev.Bindings)
	}

	if err := Restore(run, prev, func() {}); err != nil {
		t.Fatal(err)
	}
	for k, want := range orig {
		if got := store[k]; got != want {
			t.Errorf("after restore %s = %q, want %q", k, got, want)
		}
	}
}

func TestParseList(t *testing.T) {
	for in, want := range map[string][]string{
		"@as []":     nil,
		"['a']":      {"a"},
		"['a', 'b']": {"a", "b"},
	} {
		if got := parseList(in); !reflect.DeepEqual(got, want) {
			t.Errorf("parseList(%q) = %v, want %v", in, got, want)
		}
	}
}

// The custom keybinding must be released before the input switch is restored.
func TestRestoreReleasesBindingFirst(t *testing.T) {
	store := map[string]string{
		wmSchema + " " + inputKey:     "['XF86Keyboard']",
		mediaSchema + " " + customKey: "['" + customPath + "']",
	}
	var log []string
	run := fakeRunner(store, &log)

	prev := Previous{Bindings: []Setting{InputSwitch("['<Super>space', 'XF86Keyboard']")}}
	settled := -1
	if err := Restore(run, prev, func() { settled = len(log) }); err != nil {
		t.Fatal(err)
	}
	idx := func(prefix string) int {
		for i, l := range log {
			if strings.HasPrefix(l, prefix) {
				return i
			}
		}
		return -1
	}
	release := idx("set " + customSchema + " binding")
	restore := idx("set " + wmSchema + " " + inputKey)
	if settled < 0 || settled > restore {
		t.Errorf("settled at %d, input switch restored at %d", settled, restore)
	}
	if release < 0 || restore < 0 || release > restore {
		t.Errorf("binding released at %d, input switch restored at %d\n%v", release, restore, log)
	}
}
