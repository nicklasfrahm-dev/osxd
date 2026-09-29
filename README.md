# osxd

A small Go daemon that brings macOS-style features to GNOME.

## Features

- [Spotlight](docs/spotlight.md): an application launcher with fuzzy search, opened with **Super+Space**.

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

See each feature's documentation for first-run setup.

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

Code is organized by feature under `pkg/features/<feature>/`. Only `cmd/osxd/main.go` needs GTK.

`gotk4` is pinned to v0.3.1 because v0.4 needs a newer GLib than Ubuntu 24.04 ships.

## Known limits

- GNOME on Wayland only. Other desktops are not supported.

Feature-specific limits are listed in each feature's documentation.
