# Spotlight

A Spotlight-like application launcher, opened with **Super+Space**.

## Usage

Press **Super+Space** to toggle the launcher, then type to search installed apps.

| Key | Action |
| --- | --- |
| Up / Down | Move the selection |
| Enter | Launch the selected app |
| Esc, or click away | Dismiss |

Search is fuzzy: `ffx` finds Firefox and `vsc` finds Visual Studio Code.

## Setup

On first start, osxd asks whether it may take over Super+Space. It asks again on every start until you agree. Run `osxd --setup` to be asked again after agreeing.

## How the shortcut works

Wayland does not let applications grab global hotkeys, so osxd changes GNOME's own settings when you agree:

- removes `<Super>space` from every GNOME keybinding that uses it, usually `org.gnome.desktop.wm.keybindings switch-input-source`, including other custom keybindings
- adds a custom keybinding that runs `osxd`, which toggles the running instance
- enables `org.gnome.mutter center-new-windows`, because Wayland apps cannot position their own windows. This centres new windows in **all** applications.

The original values are saved in `~/.config/osxd/config.json`. `osxd --restore` (run by `make uninstall`) gives Super+Space back to whatever used it before and resets the centring setting.

## Code

| Package | Purpose |
| --- | --- |
| [`pkg/features/spotlight/apps`](../pkg/features/spotlight/apps) | Discovers installed `.desktop` entries and searches them |
| [`pkg/features/spotlight/shortcut`](../pkg/features/spotlight/shortcut) | Binds Super+Space through GNOME settings and restores them |

The search and shortcut logic are plain Go with unit tests. The GTK window lives in `cmd/osxd/main.go`.

## Known limits

- The launcher has a fixed dark theme and no background blur.
- Search covers installed applications only.
