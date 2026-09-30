// Command osxd is a desktop daemon that brings macOS-style features to GNOME.
// It currently provides a Spotlight-like application launcher.
//
// Run it once to start the resident instance; running it again (which is what
// the Super+Space keybinding does) toggles the window.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/nicklasfrahm/osxd/pkg/config"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/apps"
	"github.com/nicklasfrahm/osxd/pkg/features/spotlight/shortcut"
)

const appID = "dev.nicklasfrahm.Osxd"

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
`

func loadCSS() {
	p := gtk.NewCSSProvider()
	p.LoadFromData(css)
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), p, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func gsettings(args ...string) (string, error) {
	out, err := exec.Command("gsettings", args...).Output()
	return strings.TrimSpace(string(out)), err
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
		if err := shortcut.Restore(gsettings, prev, func() { time.Sleep(time.Second) }); err != nil {
			fatal(err)
		}
		cfg.Consent, cfg.PreviousBindings, cfg.PreviousInputSwitch, cfg.PreviousCenterNewWindows = config.Unasked, nil, "", ""
		if err := config.Save(cfgPath, cfg); err != nil {
			fatal(err)
		}
		fmt.Println("Super+Space restored to its previous bindings.")
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
			l = newLauncher(app)
			if cfg.Consent == config.Granted && cfg.PreviousCenterNewWindows == "" {
				// Granted before centring existed; apply it now.
				if prev, err := shortcut.Center(gsettings); err == nil {
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
		prev, err := shortcut.Install(gsettings, self)
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

type launcher struct {
	win     *gtk.ApplicationWindow
	entry   *gtk.SearchEntry
	list    *gtk.ListBox
	all     []apps.App
	results []apps.App
}

func newLauncher(app *gtk.Application) *launcher {
	loadCSS()
	l := &launcher{all: apps.Scan(apps.Dirs())}

	l.entry = gtk.NewSearchEntry()
	l.entry.SetPlaceholderText("Spotlight Search")
	l.entry.AddCSSClass("spotlight-search")
	l.list = gtk.NewListBox()
	l.list.SetSelectionMode(gtk.SelectionBrowse)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("spotlight-card")
	box.Append(l.entry)
	box.Append(l.list)

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

func (l *launcher) hide() { l.win.SetVisible(false) }

func (l *launcher) toggle() {
	if l.win.IsVisible() {
		l.hide()
		return
	}
	l.show()
}

func (l *launcher) refresh() {
	l.list.RemoveAll()
	l.results = apps.Search(l.all, l.entry.Text(), 8)
	for _, a := range l.results {
		l.list.Append(resultRow(a))
	}
	l.list.SetVisible(len(l.results) > 0)
	if len(l.results) > 0 {
		l.list.SelectRow(l.list.RowAtIndex(0))
	}
}

func resultRow(a apps.App) *gtk.Box {
	icon := gtk.NewImage()
	if strings.HasPrefix(a.Icon, "/") {
		icon.SetFromFile(a.Icon)
	} else if a.Icon != "" {
		icon.SetFromIconName(a.Icon)
	} else {
		icon.SetFromIconName("application-x-executable")
	}
	icon.SetPixelSize(32)

	label := gtk.NewLabel(a.Name)
	label.SetXAlign(0)
	label.SetHExpand(true)

	row := gtk.NewBox(gtk.OrientationHorizontal, 12)
	row.Append(icon)
	row.Append(label)
	return row
}

func (l *launcher) move(delta int) {
	row := l.list.SelectedRow()
	if row == nil {
		return
	}
	if next := l.list.RowAtIndex(row.Index() + delta); next != nil {
		l.list.SelectRow(next)
	}
}

func (l *launcher) launchSelected() {
	row := l.list.SelectedRow()
	if row == nil {
		return
	}
	a := l.results[row.Index()]
	l.hide()
	// `gio launch` honours Terminal=, DBusActivatable and field codes.
	if err := exec.Command("gio", "launch", a.Path).Start(); err != nil {
		fmt.Fprintln(os.Stderr, "launch failed:", err)
	}
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
