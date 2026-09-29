package shortcut

import (
	"reflect"
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
	if prev.InputSwitch != "['<Super>space', 'XF86Keyboard']" || prev.CenterNewWindows != "false" {
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
	if got := store[wmSchema+" "+inputKey]; got != prev.InputSwitch {
		t.Errorf("restored = %q", got)
	}
	if got := store[mutterSchema+" "+centerKey]; got != "false" {
		t.Errorf("center after restore = %q", got)
	}
	if got := store[mediaSchema+" "+customKey]; got != "@as []" {
		t.Errorf("custom list after restore = %q", got)
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

	prev := Previous{InputSwitch: "['<Super>space', 'XF86Keyboard']"}
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
