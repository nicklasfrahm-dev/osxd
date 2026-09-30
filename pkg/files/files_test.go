package files

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIndex(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"Documents/report.pdf",
		"Documents/old/report-draft.pdf",
		"Projects/app/node_modules/report/index.js",
		"Projects/app/.git/report",
		".hidden-report",
		"snap/report.txt",
		"notes.md",
		"inventory-device-type.ts",
		"invoice-2024.pdf",
	} {
		touch(t, filepath.Join(root, p))
	}
	cache := filepath.Join(t.TempDir(), "files.txt")
	ix := New([]string{root}, []string{filepath.Join(root, "snap")}, cache)
	if ix.Len() != 0 {
		t.Fatalf("new index without cache has %d entries", ix.Len())
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}

	got := ix.Search("report", 10)
	want := []string{
		filepath.Join(root, "Documents/report.pdf"),
		filepath.Join(root, "Documents/old/report-draft.pdf"),
	}
	if len(got) != len(want) {
		t.Fatalf("Search(report) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pos %d = %s, want %s", i, got[i], want[i])
		}
	}

	if got := ix.Search("docs", 10); len(got) != 1 || got[0] != filepath.Join(root, "Documents") {
		t.Errorf("Search(docs) = %v, want the Documents folder", got)
	}
	if got := ix.Search("invoice", 10); len(got) != 1 || got[0] != filepath.Join(root, "invoice-2024.pdf") {
		t.Errorf("Search(invoice) = %v, want only the invoice", got)
	}
	if got := ix.Search("n", 10); got != nil {
		t.Errorf("one-letter query matched %v", got)
	}

	// A new index picks up the saved one without scanning.
	if n := New(nil, nil, cache).Len(); n != ix.Len() {
		t.Errorf("reloaded %d entries, want %d", n, ix.Len())
	}
}
