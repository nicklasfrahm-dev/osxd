package apps

import "testing"

func TestSearchRanking(t *testing.T) {
	all := []App{
		{Name: "Text Editor"},
		{Name: "Terminal"},
		{Name: "Settings", Keywords: "term;prefs"},
		{Name: "Files", Comment: "Browse your terms"},
	}
	got := Search(all, "term", 10)
	want := []string{"Terminal", "Settings", "Files"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("pos %d = %s, want %s", i, got[i].Name, want[i])
		}
	}
	if Search(all, "  ", 10) != nil {
		t.Error("blank query should match nothing")
	}
}

func TestScanSystem(t *testing.T) {
	if len(Scan(Dirs())) == 0 {
		t.Skip("no desktop entries on this machine")
	}
}

func TestFuzzy(t *testing.T) {
	all := []App{
		{Name: "Firefox"},
		{Name: "Visual Studio Code"},
		{Name: "Files"},
		{Name: "System Monitor"},
	}
	for query, want := range map[string]string{
		"ffx":      "Firefox",
		"vsc":      "Visual Studio Code",
		"vs code":  "Visual Studio Code",
		"sysmon":   "System Monitor",
		"frfx":     "Firefox",
		"studcode": "Visual Studio Code",
	} {
		got := Search(all, query, 5)
		if len(got) == 0 || got[0].Name != want {
			t.Errorf("Search(%q) = %v, want first %s", query, got, want)
		}
	}
	if got := Search(all, "zzz", 5); len(got) != 0 {
		t.Errorf("zzz matched %v", got)
	}
	// A tight match beats a scattered one.
	got := Search([]App{{Name: "Something Else Chrome"}, {Name: "Chrome"}}, "chrome", 5)
	if got[0].Name != "Chrome" {
		t.Errorf("order = %v", got)
	}
}
