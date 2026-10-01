# Spotlight

A Spotlight-like launcher for apps, files, websites, web searches and calculations, opened with **Super+Space**.

## Usage

Press **Super+Space** to toggle the launcher, then type. Results appear in this order:

1. **Calculator**: if what you typed is a calculation, such as `2 * (3 + 4)`, its result. Press Enter to copy it.
2. **Website**: if what you typed looks like a link (`github.com`, `https://go.dev/doc`, `localhost:8080`, `mailto:…`), open it in your default browser.
3. **Apps**: installed applications.
4. **Files**: files and folders in your home directory, opened with their default app.
5. **Web search**: search the web for what you typed.

Long result lists scroll instead of growing the window.

| Key | Action |
| --- | --- |
| Up / Down | Move the selection; it wraps around from the last result to the first and back |
| Enter | Open the selected result, or copy the calculator result |
| Esc, or click away | Dismiss |

Search is fuzzy: `ffx` finds Firefox and `vsc` finds Visual Studio Code. File search is stricter, because a home directory has far more names than there are apps.

### Calculator

- Operators: `+ - * / % ^` (also `×`, `÷` and `**`), with parentheses and decimals such as `1.5`
- Constants: `pi`, `e`
- Functions: `sqrt`, `abs`, `ln`, `log` (base 10), `sin`, `cos`, `tan` (radians), `floor`, `ceil`, `round`

Results show at most 12 significant digits, so `0.1 + 0.2` gives `0.3`. A bare number such as `42` is not a calculation and shows no result, nor does an unfinished one such as `1.5 +`.

### File index

osxd indexes your home directory when it starts and every 10 minutes after that. It skips:

- hidden files and folders, such as `~/.cache` and `.git`
- `node_modules`, `__pycache__`, `site-packages` and `venv` folders
- `~/snap` and `~/go/pkg`

The index holds up to 500,000 paths and is saved in `~/.cache/osxd/files.txt`, so search works as soon as osxd restarts. Files created since the last scan appear at the next scan.

### Websites

A name ending in a common file extension, such as `notes.md` or `run.sh`, is not treated as a website. Type the scheme to open one anyway: `https://example.sh`.

When the selected result is a website, osxd fetches the page once you stop typing and shows a preview card with its image, site name, title and description, like chat apps do for pasted links. The card uses the page's OpenGraph or Twitter card tags, or its title and icon if it has none. Fetching sends a request to that site, as opening it would.

### Settings

Set these in `~/.config/osxd/config.json` and restart osxd:

| Key | Default | Effect |
| --- | --- | --- |
| `search_url` | `https://duckduckgo.com/?q=%s` | Web search engine; `%s` is replaced with the query |
| `disable_link_previews` | `false` | Stop fetching pages for preview cards |

## Setup

On first start, osxd asks whether it may take over Super+Space. It asks again on every start until you agree. Run `osxd --setup` to be asked again after agreeing.

## How the shortcut works

Wayland does not let applications grab global hotkeys, so osxd changes GNOME's own settings when you agree:

- removes `<Super>space` from every GNOME keybinding that uses it, usually `org.gnome.desktop.wm.keybindings switch-input-source`, including other custom keybindings
- adds a custom keybinding that runs `osxd`, which toggles the running instance
- enables `org.gnome.mutter center-new-windows`, because Wayland apps cannot position their own windows. This centres new windows in **all** applications.

osxd also writes the file index to `~/.cache/osxd/files.txt`.

The original values are saved in `~/.config/osxd/config.json`. `osxd --restore` (run by `make uninstall`) gives Super+Space back to whatever used it before and resets the centring setting.

## Code

| Package | Purpose |
| --- | --- |
| [`pkg/features/spotlight/apps`](../pkg/features/spotlight/apps) | Discovers installed `.desktop` entries and searches them |
| [`pkg/features/spotlight/shortcut`](../pkg/features/spotlight/shortcut) | Binds Super+Space through GNOME settings and restores them |
| [`pkg/features/spotlight/files`](../pkg/features/spotlight/files) | Indexes the home directory in the background and searches it |
| [`pkg/features/spotlight/web`](../pkg/features/spotlight/web) | Recognises links and builds web search URLs |
| [`pkg/features/spotlight/preview`](../pkg/features/spotlight/preview) | Fetches a page's title, description and image for link previews |
| [`pkg/features/spotlight/calc`](../pkg/features/spotlight/calc) | Evaluates arithmetic typed into the launcher |
| [`pkg/features/spotlight/fuzzy`](../pkg/features/spotlight/fuzzy) | Fuzzy matching shared by app and file search |

The search, calculator, preview and shortcut logic are plain Go with unit tests. The GTK window lives in `cmd/osxd/main.go`.

## Known limits

- The launcher has a fixed dark theme and no background blur.
- New files only become searchable at the next scan, up to 10 minutes later.
- Only the home directory is indexed, and file contents are not searched.
