// Package files keeps a searchable index of the user's files.
//
// The index lives in memory, is saved to disk so that it is available
// immediately after a restart, and is rebuilt in the background.
package files

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/fuzzy"
)

// MaxEntries caps the index so that a huge home directory cannot exhaust
// memory or make every keystroke slow.
const MaxEntries = 500_000

// skipNames are directories that hold generated or third-party content.
// Hidden files and directories are always skipped.
var skipNames = map[string]bool{
	"node_modules":  true,
	"__pycache__":   true,
	"site-packages": true,
	"venv":          true,
}

type entry struct {
	path string
	name string // lowercase base name
}

// Index is a list of file and directory paths under a set of roots.
type Index struct {
	roots   []string
	exclude []string
	cache   string

	mu      sync.RWMutex
	entries []entry

	scanning atomic.Bool
}

// New returns an index of roots, skipping the exclude paths, and loads the
// last saved index from cache if there is one. Call Refresh or Run to scan.
func New(roots, exclude []string, cache string) *Index {
	ix := &Index{roots: roots, exclude: exclude, cache: cache}
	if paths, err := load(cache); err == nil {
		ix.set(paths)
	}
	return ix
}

// DefaultCache returns where the index is saved between runs.
func DefaultCache() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "osxd", "files.txt"), nil
}

// Len returns the number of indexed paths.
func (ix *Index) Len() int {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return len(ix.entries)
}

// Run rebuilds the index now and then every interval, until stop is closed.
func (ix *Index) Run(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		_ = ix.Refresh()
		select {
		case <-t.C:
		case <-stop:
			return
		}
	}
}

// Refresh rescans the roots and saves the result. It does nothing if a scan
// is already running.
func (ix *Index) Refresh() error {
	if !ix.scanning.CompareAndSwap(false, true) {
		return nil
	}
	defer ix.scanning.Store(false)
	paths := ix.scan()
	ix.set(paths)
	if ix.cache == "" {
		return nil
	}
	return save(ix.cache, paths)
}

func (ix *Index) scan() []string {
	excluded := map[string]bool{}
	for _, e := range ix.exclude {
		excluded[filepath.Clean(e)] = true
	}
	var paths []string
	for _, root := range ix.roots {
		root = filepath.Clean(root)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if len(paths) >= MaxEntries {
				return filepath.SkipAll
			}
			if path == root {
				return nil
			}
			if err != nil || skip(d, excluded[path]) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			paths = append(paths, path)
			return nil
		})
	}
	return paths
}

func skip(d fs.DirEntry, excluded bool) bool {
	name := d.Name()
	if excluded || strings.HasPrefix(name, ".") {
		return true
	}
	if d.IsDir() {
		return skipNames[name]
	}
	// Symlinks and sockets are left out; WalkDir never follows links anyway.
	return !d.Type().IsRegular()
}

func (ix *Index) set(paths []string) {
	entries := make([]entry, len(paths))
	for i, p := range paths {
		entries[i] = entry{path: p, name: strings.ToLower(filepath.Base(p))}
	}
	ix.mu.Lock()
	ix.entries = entries
	ix.mu.Unlock()
}

// Search returns up to limit paths whose base name matches query, best first.
// Queries shorter than two characters match nothing.
//
// Matching is stricter than for apps: with this many names, letters scattered
// across a long name ("invoice" in "inventory-device") are almost always
// noise, so a match must average 3.5 points per query letter.
func (ix *Index) Search(query string, limit int) []string {
	q := strings.ToLower(strings.Join(strings.Fields(query), ""))
	n := len([]rune(q))
	if n < 2 {
		return nil
	}
	type hit struct {
		path  string
		score int
	}
	var hits []hit
	ix.mu.RLock()
	for _, e := range ix.entries {
		if s := fuzzy.Score(q, e.name); s*2 >= 7*n {
			if e.name == q {
				s += 50
			}
			hits = append(hits, hit{e.path, s})
		}
	}
	ix.mu.RUnlock()
	// Prefer better matches, then paths closer to the root, then shorter ones.
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.score != b.score {
			return a.score > b.score
		}
		da, db := strings.Count(a.path, "/"), strings.Count(b.path, "/")
		if da != db {
			return da < db
		}
		if len(a.path) != len(b.path) {
			return len(a.path) < len(b.path)
		}
		return a.path < b.path
	})
	var out []string
	for i := 0; i < len(hits) && i < limit; i++ {
		out = append(out, hits[i].path)
	}
	return out
}

func load(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var paths []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() && len(paths) < MaxEntries {
		if line := sc.Text(); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, sc.Err()
}

// save writes one path per line, replacing the file atomically.
func save(path string, paths []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".files-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := bufio.NewWriter(tmp)
	for _, p := range paths {
		if strings.ContainsRune(p, '\n') {
			continue // cannot be stored one per line
		}
		w.WriteString(p)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
