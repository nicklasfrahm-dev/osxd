// Package apps discovers installed desktop applications and searches them.
package apps

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// App is one launchable .desktop entry.
type App struct {
	ID       string // file name, e.g. firefox.desktop
	Path     string
	Name     string
	Comment  string
	Keywords string
	Icon     string // icon theme name or absolute path
}

// Dirs returns the application directories in XDG priority order.
func Dirs() []string {
	home, _ := os.UserHomeDir()
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	dirs := []string{filepath.Join(data, "applications")}
	sys := os.Getenv("XDG_DATA_DIRS")
	if sys == "" {
		sys = "/usr/local/share:/usr/share"
	}
	for _, d := range strings.Split(sys, ":") {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "applications"))
		}
	}
	return dirs
}

// Scan loads every visible application from dirs. Earlier dirs win on ID clashes.
func Scan(dirs []string) []App {
	seen := map[string]bool{}
	var out []App
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.desktop"))
		for _, path := range files {
			id := filepath.Base(path)
			if seen[id] {
				continue
			}
			seen[id] = true
			if a, ok := parse(path); ok {
				a.ID = id
				out = append(out, a)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func parse(path string) (App, bool) {
	f, err := os.Open(path)
	if err != nil {
		return App{}, false
	}
	defer f.Close()

	a := App{Path: path}
	var isApp, hidden, inEntry bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k { // unlocalised keys only
		case "Type":
			isApp = v == "Application"
		case "Name":
			a.Name = v
		case "Comment":
			a.Comment = v
		case "Icon":
			a.Icon = v
		case "Keywords":
			a.Keywords = v
		case "NoDisplay", "Hidden":
			if v == "true" {
				hidden = true
			}
		}
	}
	return a, isApp && !hidden && a.Name != ""
}

// Search returns up to limit apps matching query, best first.
// An empty query matches nothing.
func Search(all []App, query string, limit int) []App {
	q := strings.ToLower(strings.Join(strings.Fields(query), ""))
	if q == "" {
		return nil
	}
	type hit struct {
		app   App
		score int
	}
	var hits []hit
	for _, a := range all {
		if s := score(a, q); s > 0 {
			hits = append(hits, hit{a, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	var out []App
	for i := 0; i < len(hits) && i < limit; i++ {
		out = append(out, hits[i].app)
	}
	return out
}

// score ranks an app for query q (lowercase, spaces removed); 0 means no match.
//
// The name is matched fuzzily: the query letters only have to appear in order,
// so "ffx" finds Firefox and "vsc" finds Visual Studio Code. Keywords and
// comments must match as substrings and count for less than any name hit.
func score(a App, q string) int {
	best := 0
	if s := fuzzy(q, strings.ToLower(a.Name)); s > 0 {
		best = 100 + s
	}
	if strings.Contains(strings.ToLower(a.Keywords), q) && best < 50 {
		best = 50
	}
	if strings.Contains(strings.ToLower(a.Comment), q) && best < 20 {
		best = 20
	}
	return best
}

// fuzzy returns how well pattern matches text as an in-order subsequence, or 0
// if it doesn't. It finds the best alignment: matches at the start of a word
// and runs of consecutive letters score higher, and gaps cost a little.
func fuzzy(pattern, text string) int {
	p, t := []rune(pattern), []rune(text)
	if len(p) == 0 || len(p) > len(t) {
		return 0
	}
	const (
		wordStart   = 6
		consecutive = 5
		gapCost     = 1
		maxLead     = 3
	)
	isSep := func(r rune) bool { return r == ' ' || r == '-' || r == '_' || r == '.' || r == ';' || r == '/' }
	bonus := func(j int) int { // j indexes t
		b := 1
		if j == 0 || isSep(t[j-1]) {
			b += wordStart
		}
		return b
	}

	// prev[j]: best score with the previous pattern letter matched at t[j].
	const none = -1 << 30
	prev := make([]int, len(t))
	for j := range t {
		prev[j] = none
		if t[j] == p[0] {
			lead := j
			if lead > maxLead {
				lead = maxLead
			}
			prev[j] = bonus(j) - lead*gapCost
		}
	}
	for i := 1; i < len(p); i++ {
		cur := make([]int, len(t))
		for j := range t {
			cur[j] = none
			if t[j] != p[i] {
				continue
			}
			for k := 0; k < j; k++ {
				if prev[k] == none {
					continue
				}
				s := prev[k] + bonus(j)
				if k == j-1 {
					s += consecutive
				} else {
					s -= (j - k - 1) * gapCost
				}
				if s > cur[j] {
					cur[j] = s
				}
			}
		}
		prev = cur
	}
	best := none
	for _, s := range prev {
		if s > best {
			best = s
		}
	}
	if best == none {
		return 0
	}
	if best < 1 {
		best = 1
	}
	return best
}
