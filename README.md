# osxd

A small Go daemon that brings macOS-style features to GNOME. It currently provides a Spotlight-like application launcher, opened with **Super+Space**.

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

Press **Super+Space** to toggle the launcher, then type to search installed apps.

| Key | Action |
| --- | --- |
| Up / Down | Move the selection |
| Enter | Launch the selected app |
| Esc, or click away | Dismiss |

Search is fuzzy: `ffx` finds Firefox and `vsc` finds Visual Studio Code.

## How the shortcut works

Wayland does not let applications grab global hotkeys, so osxd changes GNOME's own settings when you agree:

- removes `<Super>space` from `org.gnome.desktop.wm.keybindings switch-input-source`
- adds a custom keybinding that runs `osxd`, which toggles the running instance
- enables `org.gnome.mutter center-new-windows`, because Wayland apps cannot position their own windows. This centres new windows in **all** applications.

The original values are saved in `~/.config/osxd/config.json`.

## Uninstall

```bash
make uninstall
```

This runs `osxd --restore` first, which gives Super+Space back to input-source switching and resets the centring setting, then removes the service and binary.

## Development

```bash
make build   # build ./osxd
make test    # run the tests
```

The search and shortcut logic are plain Go with unit tests. Only `main.go` needs GTK.

`gotk4` is pinned to v0.3.1 because v0.4 needs a newer GLib than Ubuntu 24.04 ships.

## Known limits

- GNOME on Wayland only. Other desktops are not supported.
- The launcher has a fixed dark theme and no background blur.
- Search covers installed applications only.
