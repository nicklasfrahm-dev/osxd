# <img src="contrib/dev.nicklasfrahm.Osxd.svg" alt="osxd icon" align="right" width="100" height="100"> osxd

A small Go daemon that brings macOS-style features to GNOME.

### Features

- [Spotlight](docs/spotlight.md): a launcher with fuzzy search for apps, files, websites with link previews, web searches and a calculator, opened with **Super+Space**.
- [Hotkeys](docs/hotkeys.md): macOS-style **Super+A/C/X/V/F/Z/Y/S** for select all, copy, cut, paste, find, undo, redo and save, in every app and in terminals.

### Requirements

- GNOME on Wayland on Ubuntu 24.04 LTS or 26.04 LTS. Both were confirmed to build; only 24.04 has been run. Ubuntu 22.04 is not supported (see below).
- Go 1.27 or newer (see `go.mod`)
- GTK 4 and GLib development headers:

  ```bash
  sudo apt install libgtk-4-dev libglib2.0-dev libgirepository1.0-dev
  ```

### Install

```bash
make install
```

This builds `osxd`, installs it to `~/.local/bin`, installs its icon, a hidden desktop entry and a GNOME Shell extension under `~/.local/share`, asks for your password to add a udev rule to `/etc/udev/rules.d` that gives you access to keyboards (for the hotkeys), and installs, enables and starts a systemd user service (`osxd.service`) that runs with your graphical session. Close any copy you started by hand first, or the service exits immediately.

See each feature's documentation for first-run setup.

### Uninstall

```bash
make uninstall
```

This runs `osxd --restore` first, which gives Super+Space back to whatever used it before (usually input-source switching), resets the centring setting and disables the GNOME Shell extension, then removes the service, binary, extension and udev rule.

### Development

```bash
make build   # build ./osxd
make test    # run the tests
```

Code is organized by feature under `pkg/features/<feature>/`. Only `cmd/osxd/main.go` needs GTK.

`gotk4` is pinned to v0.3.1. v0.4 needs a newer GLib than Ubuntu 24.04 ships, and v0.3.1 needs GLib 2.76 or newer, which is why Ubuntu 22.04 (GLib 2.72) cannot build it.

### Known limits

- GNOME on Wayland only. Other desktops are not supported.

Feature-specific limits are listed in each feature's documentation.
