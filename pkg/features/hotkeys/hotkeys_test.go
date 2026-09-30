package hotkeys

import "testing"

func TestExtension(t *testing.T) {
	store := map[string]string{
		enabledKey:  "['ubuntu-dock@ubuntu.com']",
		disabledKey: "['" + extensionUUID + "']",
	}
	run := func(args ...string) (string, error) {
		if args[0] == "set" {
			store[args[2]] = args[3]
		}
		return store[args[2]], nil
	}

	if err := EnableExtension(run); err != nil {
		t.Fatal(err)
	}
	if err := EnableExtension(run); err != nil { // idempotent
		t.Fatal(err)
	}
	if got, want := store[enabledKey], "['ubuntu-dock@ubuntu.com', '"+extensionUUID+"']"; got != want {
		t.Errorf("enabled = %s, want %s", got, want)
	}
	if got := store[disabledKey]; got != "@as []" {
		t.Errorf("disabled = %s, want @as []", got)
	}

	if err := DisableExtension(run); err != nil {
		t.Fatal(err)
	}
	if got, want := store[enabledKey], "['ubuntu-dock@ubuntu.com']"; got != want {
		t.Errorf("enabled = %s, want %s", got, want)
	}
}
