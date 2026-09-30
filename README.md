# osxd

A small Go daemon that brings macOS-style features to GNOME. It currently provides a Spotlight-like launcher for apps, files, websites and web searches, opened with **Super+Space**.

## Requirements

- GNOME on Wayland (tested on Ubuntu 24.04, GNOME 46)
- Go 1.27 or newer (see `go.mod`)
- GTK 4 and GLib development headers:

  ```bash
  sudo apt install libgtk-4-dev libglib2.0-dev libgirepository1.0-dev
  ```

## Install

```bash
make install
```

This builds `osxd`, installs it to `~/.local/bin`, and installs, enables and starts a systemd user service (`osxd.service`) that runs with your graphical session. Close any copy you started by hand first, or the service exits immediately.

On first start, osxd asks whether it may take over Super+Space. It asks again on every start until you agree. Run `osxd --setup` to be asked again after agreeing.

## Usage

Press **Super+Space** to toggle the launcher, then type. Results appear in this order:

1. **Website**: if what you typed looks like a link (`github.com`, `https://go.dev/doc`, `localhost:8080`, `mailto:…`), open it in your default browser.
2. **Apps**: installed applications.
3. **Files**: files and folders in your home directory, opened with their default app.
4. **Web search**: search the web for what you typed.

| Key | Action |
| --- | --- |
| Up / Down | Move the selection |
| Enter | Open the selected result |
| Esc, or click away | Dismiss |

Search is fuzzy: `ffx` finds Firefox and `vsc` finds Visual Studio Code. File search is stricter, because a home directory has far more names than there are apps.

### File index

osxd indexes your home directory when it starts and every 10 minutes after that. It skips:

- hidden files and folders, such as `~/.cache` and `.git`
- `node_modules`, `__pycache__`, `site-packages` and `venv` folders
- `~/snap` and `~/go/pkg`

The index holds up to 500,000 paths and is saved in `~/.cache/osxd/files.txt`, so search works as soon as osxd restarts. Files created since the last scan appear at the next scan.

### Web search

Web searches use DuckDuckGo. To use another engine, set `search_url` in `~/.config/osxd/config.json`, with `%s` where the query goes, and restart osxd:

```json
"search_url": "https://www.google.com/search?q=%s"
```

### Link previews

When the selected result is a website, osxd fetches the page once you stop typing and shows a preview card with its image, site name, title and description, like chat apps do for pasted links. The card uses the page's OpenGraph or Twitter card tags, or its title and icon if it has none.

Fetching sends a request to that site, as opening it would. To turn previews off, set `"disable_link_previews": true` in `~/.config/osxd/config.json` and restart osxd.

A name ending in a common file extension, such as `notes.md` or `run.sh`, is not treated as a website. Type the scheme to open one anyway: `https://example.sh`.

## How the shortcut works

Wayland does not let applications grab global hotkeys, so osxd changes GNOME's own settings when you agree:

- removes `<Super>space` from every GNOME keybinding that uses it, usually `org.gnome.desktop.wm.keybindings switch-input-source`, including other custom keybindings
- adds a custom keybinding that runs `osxd`, which toggles the running instance
- enables `org.gnome.mutter center-new-windows`, because Wayland apps cannot position their own windows. This centres new windows in **all** applications.

The original values are saved in `~/.config/osxd/config.json`.

## Uninstall

```bash
make uninstall
```

This runs `osxd --restore` first, which gives Super+Space back to whatever used it before (usually input-source switching) and resets the centring setting, then removes the service and binary.

## Development

```bash
make build   # build ./osxd
make test    # run the tests
```

The search and shortcut logic are plain Go with unit tests. Only `cmd/osxd/main.go` needs GTK.

`gotk4` is pinned to v0.3.1 because v0.4 needs a newer GLib than Ubuntu 24.04 ships.

## Known limits

- GNOME on Wayland only. Other desktops are not supported.
- The launcher has a fixed dark theme and no background blur.
- New files only become searchable at the next scan, up to 10 minutes later.
- Only the home directory is indexed, and file contents are not searched.
