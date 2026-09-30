// Command osxd is a desktop daemon that brings macOS-style features to GNOME.
// It provides a Spotlight-like launcher for apps, files, websites, web
// searches and calculations, and macOS-style Super shortcuts such as Super+C
// to copy.
//
// Run it once to start the resident instance; running it again (which is what
// the Super+Space keybinding does) toggles the window.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/nicklasfrahm/osxd/pkg/config"
	"github.com/nicklasfrahm/osxd/pkg/features/hotkeys"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/apps"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/calc"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/files"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/preview"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/shortcut"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/web"
	"github.com/nicklasfrahm/osxd/pkg/gsettings"
)

const appID = "dev.nicklasfrahm.Osxd"

// reindexEvery is how often the file index is rebuilt in the background.
const reindexEvery = 10 * time.Minute

// maxListHeight caps the result list at about eight rows; longer lists
// scroll so that the window, and with it the search field, stops growing.
const maxListHeight = 400

// previewDelay is how long typing must pause before a link is fetched, so
// that "github.c" and "github.co" are not requested on the way to
// "github.com".
const previewDelay = 400 // ms

// css gives the launcher its Spotlight look: a rounded card, a
// large borderless search field and accent-highlighted result rows.
const css = `
window.spotlight { background: transparent; }
.spotlight-card {
	background: #262629;
	border: 1px solid rgba(255, 255, 255, 0.12);
	border-radius: 20px;
	padding: 8px;
}
.spotlight-card entry.spotlight-search {
	background: transparent;
	border: none;
	box-shadow: none;
	outline: none;
	color: white;
	font-size: 26px;
	font-weight: 300;
	min-height: 48px;
	padding: 0 8px;
}
.spotlight-card entry.spotlight-search image { color: rgba(255, 255, 255, 0.55); margin-right: 6px; }
.spotlight-card list { background: transparent; margin-top: 4px; }
.spotlight-card row {
	background: transparent;
	color: white;
	border-radius: 10px;
	padding: 6px 10px;
	margin: 1px 4px;
	font-size: 15px;
}
.spotlight-card row:selected { background: #0a64d8; color: white; }
.spotlight-card row .detail { color: rgba(255, 255, 255, 0.55); font-size: 12px; }
.spotlight-card row:selected .detail { color: rgba(255, 255, 255, 0.8); }
.preview-card {
	background: rgba(255, 255, 255, 0.06);
	border-radius: 12px;
	padding: 10px;
	margin: 6px 4px 2px 4px;
	color: white;
}
.preview-card picture { border-radius: 8px; }
.preview-card .site { color: rgba(255, 255, 255, 0.55); font-size: 12px; }
.preview-card .title { font-size: 15px; font-weight: bold; }
.preview-card .description { color: rgba(255, 255, 255, 0.75); font-size: 13px; }
`

func loadCSS() {
	p := gtk.NewCSSProvider()
	p.LoadFromData(css)
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), p, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func main() {
	cfgPath, err := config.Path()
	if err != nil {
		fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "--restore" {
		prev := shortcut.Previous{
			Bindings:         cfg.PreviousBindings,
			CenterNewWindows: cfg.PreviousCenterNewWindows,
		}
		if cfg.PreviousInputSwitch != "" {
			prev.Bindings = append(prev.Bindings, shortcut.InputSwitch(cfg.PreviousInputSwitch))
		}
		if err := shortcut.Restore(gsettings.Run, prev, func() { time.Sleep(time.Second) }); err != nil {
			fatal(err)
		}
		if err := hotkeys.DisableExtension(gsettings.Run); err != nil {
			fmt.Fprintln(os.Stderr, "could not disable the GNOME Shell extension:", err)
		}
		cfg.Consent, cfg.PreviousBindings, cfg.PreviousInputSwitch, cfg.PreviousCenterNewWindows = config.Unasked, nil, "", ""
		if err := config.Save(cfgPath, cfg); err != nil {
			fatal(err)
		}
		fmt.Println("Super+Space restored to its previous bindings and the GNOME Shell extension disabled.")
		return
	}

	// --setup asks again even if the shortcut was already granted.
	setup := hasArg("--setup")
	// --background starts the resident instance without opening the window
	// (used by the systemd unit); the prompt still shows if not yet granted.
	background := hasArg("--background")

	self, err := os.Executable()
	if err != nil {
		fatal(err)
	}

	// The launcher brings its own styling, so pin GTK's built-in theme rather
	// than parsing the user's: third-party themes often target newer GTK
	// releases (e.g. color-mix() needs 4.16) and flood the log with
	// "Theme parser error" warnings. An explicit GTK_THEME still wins.
	if os.Getenv("GTK_THEME") == "" {
		os.Setenv("GTK_THEME", "Adwaita:dark")
	}

	app := gtk.NewApplication(appID, gio.ApplicationFlagsNone)
	var l *launcher
	app.ConnectActivate(func() {
		if l == nil {
			gtk.WindowSetDefaultIconName(appID)
			app.Hold() // stay resident while the window is hidden
			var fetcher *preview.Fetcher
			if !cfg.DisableLinkPreviews {
				fetcher = preview.NewFetcher()
			}
			l = newLauncher(app, cfg.SearchURL, startIndex(), fetcher)
			if !cfg.DisableHotkeys {
				startHotkeys(cfg.Terminals)
			}
			if cfg.Consent == config.Granted && cfg.PreviousCenterNewWindows == "" {
				// Granted before centring existed; apply it now.
				if prev, err := shortcut.Center(gsettings.Run); err == nil {
					cfg.PreviousCenterNewWindows = prev
					_ = config.Save(cfgPath, cfg)
				}
			}
			if setup || cfg.Consent != config.Granted {
				askConsent(app, func(granted bool) {
					cfg = applyConsent(cfg, cfgPath, self, granted)
					l.show()
				})
				return
			}
			if !background {
				l.show()
			}
			return
		}
		l.toggle()
	})
	os.Exit(app.Run([]string{os.Args[0]}))
}

// askConsent shows the first-run question and reports the answer.
func askConsent(app *gtk.Application, done func(granted bool)) {
	title := gtk.NewLabel("Use Super+Space for the osxd launcher?")
	title.AddCSSClass("title-3")
	detail := gtk.NewLabel("Super+Space usually switches the keyboard input source. " +
		"Allow osxd to take it over from whatever uses it now? This also makes GNOME open new windows centred (needed on Wayland). You can undo this any time with `osxd --restore`.")
	detail.SetWrap(true)
	detail.SetMaxWidthChars(48)

	no := gtk.NewButtonWithLabel("Not now")
	yes := gtk.NewButtonWithLabel("Overwrite shortcut")
	yes.AddCSSClass("suggested-action")
	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	buttons.SetHAlign(gtk.AlignEnd)
	buttons.Append(no)
	buttons.Append(yes)

	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginTop(18)
	box.SetMarginBottom(18)
	box.SetMarginStart(18)
	box.SetMarginEnd(18)
	box.Append(title)
	box.Append(detail)
	box.Append(buttons)

	win := gtk.NewApplicationWindow(app)
	win.SetTitle("osxd")
	win.SetResizable(false)
	win.SetChild(box)

	answered := false
	answer := func(granted bool) {
		if answered {
			return
		}
		answered = true
		win.Close()
		done(granted)
	}
	yes.ConnectClicked(func() { answer(true) })
	no.ConnectClicked(func() { answer(false) })
	win.ConnectCloseRequest(func() bool { answer(false); return false })
	win.Present()
}

// applyConsent records the answer and, if granted, installs the keybinding.
func applyConsent(cfg config.Config, path, self string, granted bool) config.Config {
	if granted {
		prev, err := shortcut.Install(gsettings.Run, self)
		if err != nil {
			fmt.Fprintln(os.Stderr, "could not install shortcut:", err)
		} else {
			// Keep the originals from the first grant; re-running setup would
			// otherwise record our own modified values as the "previous" ones.
			if cfg.Consent != config.Granted {
				cfg.PreviousBindings, cfg.PreviousCenterNewWindows = prev.Bindings, prev.CenterNewWindows
			}
			cfg.Consent = config.Granted
		}
	}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "could not save config:", err)
	}
	return cfg
}

// startHotkeys remaps Super shortcuts in the background for as long as osxd
// runs. Failing to start, usually for lack of access to the keyboards, only
// disables the hotkeys.
func startHotkeys(terminals []string) {
	if err := hotkeys.EnableExtension(gsettings.Run); err != nil {
		fmt.Fprintln(os.Stderr, "could not enable the GNOME Shell extension:", err)
	}
	if hotkeys.UserExtensionsDisabled(gsettings.Run) {
		fmt.Fprintln(os.Stderr, "hotkeys: GNOME is set to load no extensions (org.gnome.shell disable-user-extensions), so terminals cannot be told apart")
	}
	go func() {
		if err := hotkeys.Run(terminals); err != nil {
			fmt.Fprintln(os.Stderr, "hotkeys disabled:", err)
		}
	}()
}

// startIndex loads the saved file index and keeps it up to date in the
// background for as long as osxd runs.
func startIndex() *files.Index {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "file search disabled:", err)
		return files.New(nil, nil, "")
	}
	cache, err := files.DefaultCache()
	if err != nil {
		fmt.Fprintln(os.Stderr, "file index will not be saved:", err)
	}
	// Snap app data and the Go module cache are large and never what you
	// are looking for.
	exclude := []string{filepath.Join(home, "snap"), filepath.Join(home, "go", "pkg")}
	ix := files.New([]string{home}, exclude, cache)
	go ix.Run(reindexEvery, nil)
	return ix
}

// result is one row in the launcher.
type result struct {
	title  string
	detail string
	icon   func(*gtk.Image)
	open   []string // command that opens the result
	copy   string   // text to copy to the clipboard instead of opening
	link   string   // http(s) page to preview while the row is selected
}

type launcher struct {
	win       *gtk.ApplicationWindow
	entry     *gtk.SearchEntry
	list      *gtk.ListBox
	scroll    *gtk.ScrolledWindow
	all       []apps.App
	files     *files.Index
	searchURL string
	results   []result

	fetcher *preview.Fetcher // nil when previews are disabled
	card    *previewCard
	// gen identifies the latest preview request; answers to older ones
	// are dropped.
	gen    int
	cancel context.CancelFunc
	timer  glib.SourceHandle
}

func newLauncher(app *gtk.Application, searchURL string, index *files.Index, fetcher *preview.Fetcher) *launcher {
	loadCSS()
	l := &launcher{all: apps.Scan(apps.Dirs()), files: index, searchURL: searchURL, fetcher: fetcher}

	l.entry = gtk.NewSearchEntry()
	l.entry.SetPlaceholderText("Spotlight Search")
	l.entry.AddCSSClass("spotlight-search")
	l.list = gtk.NewListBox()
	l.list.SetSelectionMode(gtk.SelectionBrowse)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("spotlight-card")
	box.Append(l.entry)
	l.scroll = gtk.NewScrolledWindow()
	l.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	l.scroll.SetPropagateNaturalHeight(true)
	l.scroll.SetMaxContentHeight(maxListHeight)
	l.scroll.SetChild(l.list)
	box.Append(l.scroll)
	l.card = newPreviewCard()
	box.Append(l.card.box)

	l.win = gtk.NewApplicationWindow(app)
	l.win.SetTitle("osxd")
	l.win.AddCSSClass("spotlight")
	l.win.SetDecorated(false)
	l.win.SetResizable(false)
	l.win.SetDefaultSize(700, -1)
	l.win.SetChild(box)

	l.entry.ConnectSearchChanged(l.refresh)
	l.entry.ConnectActivate(l.launchSelected)
	l.list.ConnectRowActivated(func(*gtk.ListBoxRow) { l.launchSelected() })
	l.list.ConnectRowSelected(func(*gtk.ListBoxRow) { l.updatePreview() })

	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Escape:
			l.hide()
		case gdk.KEY_Down:
			l.move(1)
		case gdk.KEY_Up:
			l.move(-1)
		default:
			return false
		}
		return true
	})
	l.entry.AddController(keys)

	// Behave like Spotlight: dismiss when focus moves elsewhere.
	l.win.NotifyProperty("is-active", func() {
		if !l.win.IsActive() && l.win.IsVisible() {
			l.hide()
		}
	})
	return l
}

func (l *launcher) show() {
	l.entry.SetText("")
	l.refresh()
	l.win.Present()
	l.entry.GrabFocus()
}

func (l *launcher) hide() {
	l.stopPreview()
	l.win.SetVisible(false)
}

func (l *launcher) toggle() {
	if l.win.IsVisible() {
		l.hide()
		return
	}
	l.show()
}

func (l *launcher) refresh() {
	l.list.RemoveAll()
	l.results = l.search(l.entry.Text())
	for _, r := range l.results {
		l.list.Append(resultRow(r))
	}
	l.scroll.SetVisible(len(l.results) > 0)
	l.scroll.VAdjustment().SetValue(0)
	if len(l.results) > 0 {
		l.list.SelectRow(l.list.RowAtIndex(0))
	}
	l.updatePreview()
}

// stopPreview hides the card and abandons any preview being fetched.
func (l *launcher) stopPreview() {
	l.gen++
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	if l.timer != 0 {
		glib.SourceRemove(l.timer)
		l.timer = 0
	}
	l.card.box.SetVisible(false)
}

// updatePreview shows a preview card for the selected row if it is a link.
// Pages are fetched off the main thread once typing pauses.
func (l *launcher) updatePreview() {
	l.stopPreview()
	row := l.list.SelectedRow()
	if l.fetcher == nil || row == nil || row.Index() >= len(l.results) {
		return
	}
	link := l.results[row.Index()].link
	if link == "" {
		return
	}
	if p, ok := l.fetcher.Cached(link); ok {
		l.card.show(p)
		return
	}
	l.card.loading(link)

	gen := l.gen
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	l.timer = glib.TimeoutAdd(previewDelay, func() bool {
		l.timer = 0
		go func() {
			p, err := l.fetcher.Get(ctx, link)
			glib.IdleAdd(func() {
				if gen != l.gen {
					return // the query changed meanwhile
				}
				if err != nil {
					l.card.box.SetVisible(false)
					return
				}
				l.card.show(p)
			})
		}()
		return false
	})
}

// previewCard shows a link's image, site, title and description.
type previewCard struct {
	box                      *gtk.Box
	picture                  *gtk.Picture
	site, title, description *gtk.Label
}

func newPreviewCard() *previewCard {
	c := &previewCard{picture: gtk.NewPicture()}
	c.picture.SetCanShrink(true)
	c.picture.SetOverflow(gtk.OverflowHidden)
	c.picture.SetVAlign(gtk.AlignStart)

	label := func(class string, lines int) *gtk.Label {
		l := gtk.NewLabel("")
		l.AddCSSClass(class)
		l.SetXAlign(0)
		l.SetWrap(true)
		l.SetWrapMode(pango.WrapWordChar)
		l.SetLines(lines)
		l.SetEllipsize(pango.EllipsizeEnd)
		l.SetMaxWidthChars(60) // keep long titles from widening the window
		return l
	}
	c.site = label("site", 1)
	c.title = label("title", 2)
	c.description = label("description", 3)

	text := gtk.NewBox(gtk.OrientationVertical, 2)
	text.SetHExpand(true)
	text.SetVAlign(gtk.AlignCenter)
	text.Append(c.site)
	text.Append(c.title)
	text.Append(c.description)

	c.box = gtk.NewBox(gtk.OrientationHorizontal, 12)
	c.box.AddCSSClass("preview-card")
	c.box.Append(c.picture)
	c.box.Append(text)
	c.box.SetVisible(false)
	return c
}

// loading shows the card for link before its page has arrived.
func (c *previewCard) loading(link string) {
	host := link
	if u, err := url.Parse(link); err == nil {
		host = strings.TrimPrefix(u.Hostname(), "www.")
	}
	c.picture.SetVisible(false)
	c.site.SetText(host)
	c.title.SetText("Loading preview\u2026")
	c.description.SetVisible(false)
	c.box.SetVisible(true)
}

func (c *previewCard) show(p preview.Preview) {
	c.site.SetText(p.SiteName)
	c.title.SetText(p.Title)
	c.description.SetText(p.Description)
	c.description.SetVisible(p.Description != "")

	c.picture.SetVisible(false)
	if len(p.Image) > 0 {
		if tex, err := gdk.NewTextureFromBytes(glib.NewBytes(p.Image)); err == nil {
			c.picture.SetPaintable(tex)
			if p.IsIcon {
				c.picture.SetContentFit(gtk.ContentFitContain)
				c.picture.SetSizeRequest(48, 48)
			} else {
				// OpenGraph images are usually 1.91:1.
				c.picture.SetContentFit(gtk.ContentFitCover)
				c.picture.SetSizeRequest(192, 100)
			}
			c.picture.SetVisible(true)
		}
	}
	c.box.SetVisible(true)
}

// search builds the rows for query: the result if the query is a
// calculation, a link to open if it looks like one, then apps, then files,
// and finally a web search.
func (l *launcher) search(query string) []result {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	var out []result
	if v, ok := calc.Evaluate(q); ok {
		out = append(out, result{
			title:  "= " + v,
			detail: "Press Enter to copy",
			icon:   iconName("accessories-calculator"),
			copy:   v,
		})
	}
	if link, ok := web.Link(q); ok {
		preview := ""
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			preview = link
		}
		out = append(out, result{
			title:  "Open " + q,
			detail: link,
			icon:   iconName("web-browser"),
			open:   []string{"gio", "open", link},
			link:   preview,
		})
	}
	for _, a := range apps.Search(l.all, q, 6) {
		out = append(out, appResult(a))
	}
	out = append(out, fileResults(l.files.Search(q, 10), 5)...)

	search := web.SearchURL(l.searchURL, q)
	engine := search
	if u, err := url.Parse(search); err == nil {
		engine = strings.TrimPrefix(u.Hostname(), "www.")
	}
	out = append(out, result{
		title:  fmt.Sprintf("Search the web for \u201c%s\u201d", q),
		detail: engine,
		icon:   iconName("system-search"),
		open:   []string{"gio", "open", search},
	})
	return out
}

func appResult(a apps.App) result {
	return result{
		title: a.Name,
		icon: func(img *gtk.Image) {
			switch {
			case strings.HasPrefix(a.Icon, "/"):
				img.SetFromFile(a.Icon)
			case a.Icon != "":
				img.SetFromIconName(a.Icon)
			default:
				img.SetFromIconName("application-x-executable")
			}
		},
		// `gio launch` honours Terminal=, DBusActivatable and field codes.
		open: []string{"gio", "launch", a.Path},
	}
}

// fileResults turns up to limit indexed paths that still exist into rows.
func fileResults(paths []string, limit int) []result {
	home, _ := os.UserHomeDir()
	var out []result
	for _, p := range paths {
		if len(out) == limit {
			break
		}
		info, err := os.Stat(p)
		if err != nil {
			continue // deleted since the last scan
		}
		dir := filepath.Dir(p)
		if home != "" && (dir == home || strings.HasPrefix(dir, home+"/")) {
			dir = "~" + strings.TrimPrefix(dir, home)
		}
		r := result{title: filepath.Base(p), detail: dir, open: []string{"gio", "open", p}}
		if info.IsDir() {
			r.icon = iconName("folder")
		} else {
			_, typ := gio.ContentTypeGuess(p, nil)
			r.icon = func(img *gtk.Image) { img.SetFromGIcon(gio.ContentTypeGetIcon(typ)) }
		}
		out = append(out, r)
	}
	return out
}

func iconName(name string) func(*gtk.Image) {
	return func(img *gtk.Image) { img.SetFromIconName(name) }
}

func resultRow(r result) *gtk.Box {
	icon := gtk.NewImage()
	r.icon(icon)
	icon.SetPixelSize(32)

	title := gtk.NewLabel(r.title)
	title.SetXAlign(0)
	title.SetEllipsize(pango.EllipsizeEnd)

	text := gtk.NewBox(gtk.OrientationVertical, 0)
	text.SetHExpand(true)
	text.SetVAlign(gtk.AlignCenter)
	text.Append(title)
	if r.detail != "" {
		detail := gtk.NewLabel(r.detail)
		detail.AddCSSClass("detail")
		detail.SetXAlign(0)
		detail.SetEllipsize(pango.EllipsizeMiddle)
		text.Append(detail)
	}

	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.Append(icon)
	row.Append(text)
	return row
}

func (l *launcher) move(delta int) {
	row := l.list.SelectedRow()
	if row == nil {
		return
	}
	next := l.list.RowAtIndex(row.Index() + delta)
	if next == nil {
		return
	}
	l.list.SelectRow(next)
	l.scrollTo(next)
}

// scrollTo scrolls the list just enough to show row whole, including its
// margin and rounded corners. Focus stays in the search field, so the list
// does not follow the selection by itself.
func (l *launcher) scrollTo(row *gtk.ListBoxRow) {
	// Measured against the visible area, so the list's own margin and
	// padding cannot offset the result.
	b, ok := row.ComputeBounds(l.scroll)
	if !ok {
		return
	}
	const margin = 2 // the row's CSS margin, plus a pixel of air
	top, bottom := float64(b.Y())-margin, float64(b.Y()+b.Height())+margin
	adj := l.scroll.VAdjustment()
	switch {
	case top < 0:
		adj.SetValue(adj.Value() + top)
	case bottom > float64(l.scroll.Height()):
		adj.SetValue(adj.Value() + bottom - float64(l.scroll.Height()))
	}
}

func (l *launcher) launchSelected() {
	row := l.list.SelectedRow()
	if row == nil {
		return
	}
	r := l.results[row.Index()]
	if r.copy != "" {
		l.win.Clipboard().SetText(r.copy)
		l.hide()
		return
	}
	l.hide()
	cmd := exec.Command(r.open[0], r.open[1:]...)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "open failed:", err)
		return
	}
	go cmd.Wait() // reap it so the resident daemon collects no zombies

}

func hasArg(flag string) bool {
	for _, a := range os.Args[1:] {
		if a == flag {
			return true
		}
	}
	return false
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "osxd:", err)
	os.Exit(1)
}
