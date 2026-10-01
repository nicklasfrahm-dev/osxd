# Hotkeys

macOS-style editing shortcuts on the Super key, in every application and in terminals.

## Usage

| Keys | Action | Sent to apps | Sent to terminals |
| --- | --- | --- | --- |
| Super+A | Select all | Ctrl+A | Ctrl+Shift+A |
| Super+C | Copy | Ctrl+C | Ctrl+Shift+C |
| Super+X | Cut | Ctrl+X | Ctrl+Shift+X |
| Super+V | Paste | Ctrl+V | Ctrl+Shift+V |
| Super+F | Find | Ctrl+F | Ctrl+Shift+F |
| Super+Z | Undo | Ctrl+Z | Ctrl+Shift+Z |
| Super+Y | Redo | Ctrl+Y | Ctrl+Shift+Y |
| Super+S | Save | Ctrl+S | Ctrl+Shift+S |

Terminals get Ctrl+Shift because they pass plain Ctrl shortcuts to the program running in them: Ctrl+C would interrupt it, Ctrl+Z suspend it and Ctrl+S freeze the output. Ctrl+Shift shortcuts belong to the terminal itself, so Super+C copies the selection and Super+V pastes. Terminals have no cut, undo, redo or save of their own, so those do nothing there unless you bind them in your terminal's settings.

Other held modifiers are kept, so Super+Shift+Z sends Ctrl+Shift+Z, which is redo in most GTK apps.

osxd holds Super back until the next key shows what it is for, so neither GNOME nor the focused application ever sees Super during these shortcuts. Super on its own still opens the Activities overview (it is sent when you let go), and Super with any other key, such as Super+Left, works as before. Holding Super, you can mix both: Super+C then Super+Left.

### Settings

Set these in `~/.config/osxd/config.json` and restart osxd:

| Key | Default | Effect |
| --- | --- | --- |
| `disable_hotkeys` | `false` | Turn the hotkeys off |
| `terminals` | `[]` | Extra WM classes to treat as terminals, such as `["cool-retro-term"]` |

The built-in list covers GNOME Terminal, Ptyxis, Console, Ghostty, kitty, Alacritty, WezTerm, foot, Tilix, Terminator, Konsole, Black Box, xterm and a few others; see [`focus.go`](../pkg/features/hotkeys/focus/focus.go). To find a window's WM class, run this and switch to the window within three seconds:

```bash
sleep 3; gdbus call --session --dest org.gnome.Shell --object-path /dev/nicklasfrahm/Osxd --method dev.nicklasfrahm.Osxd.Focus.WMClass
```

## Setup

1. Install osxd with `make install`. It asks for your password to add a udev rule that gives you access to the keyboards and to `/dev/uinput`.
2. Log out and back in once. GNOME on Wayland only loads a newly installed Shell extension at login; until then Super+A, C, X, V, F, Z, Y and S do nothing.

If osxd cannot open the keyboards, it logs `hotkeys disabled: …` (see `journalctl --user -u osxd`) and the rest of osxd keeps working.

## What it changes

- **Takes over every keyboard** in `/dev/input` (with `EVIOCGRAB`) and re-sends its keys through a virtual keyboard called `osxd virtual keyboard`, made with `/dev/uinput`. Wayland gives applications no other way to rewrite key presses. Keyboards plugged in later are picked up within two seconds. When osxd stops, the keyboards are released at once.
- **Installs a GNOME Shell extension**, `osxd@nicklasfrahm.dev`, in `~/.local/share/gnome-shell/extensions/`, and adds it to `org.gnome.shell enabled-extensions`. It tells osxd which window has focus, over D-Bus, so terminals can be told apart; Wayland hides this from ordinary applications.
- **Writes `/etc/udev/rules.d/70-osxd.rules`** (with sudo, during `make install`), which gives the user logged in at the seat read access to keyboards and to `/dev/uinput`. Any program you run can then read your key presses, as it can for members of the `input` group.

`osxd --restore` (run by `make uninstall`) disables the extension, and `make uninstall` removes it and the udev rule.

## Code

| Package | Purpose |
| --- | --- |
| [`pkg/features/hotkeys`](../pkg/features/hotkeys) | Grabs keyboards, runs the remapper and enables the extension |
| [`pkg/features/hotkeys/remap`](../pkg/features/hotkeys/remap) | Rewrites Super shortcuts as Ctrl ones; plain Go with unit tests |
| [`pkg/features/hotkeys/evdev`](../pkg/features/hotkeys/evdev) | Reads keyboards and drives the virtual keyboard |
| [`pkg/features/hotkeys/focus`](../pkg/features/hotkeys/focus) | Asks the extension for the focused window and spots terminals |
| [`contrib/gnome-shell-extension`](../contrib/gnome-shell-extension) | The GNOME Shell extension |

## Known limits

- Until you log out and back in after installing, or whenever the extension is not running (for example with `org.gnome.shell disable-user-extensions` set), Super+A, C, X, V, F, Z, Y and S do nothing: osxd never sends Ctrl+C to a terminal by mistake, and never lets the bare letter through either, which many terminals would type.
- Super+A no longer opens the app grid, Super+S no longer opens Quick Settings and Super+V no longer opens the notification list, because GNOME now sees Ctrl+A, Ctrl+S and Ctrl+V. Super+M also opens the notification list.
- Redo is Ctrl+Y, which most editors, browsers and office suites use. Apps that only know Ctrl+Shift+Z, such as some GTK apps, need Super+Shift+Z.
- Keyboards that also report pointer movement, such as some with a built-in touchpad, are not taken over, so their Super shortcuts are not remapped.
- Super held on its own for more than 300 ms is sent anyway, so that Super+drag and other Super+mouse gestures keep working. A shortcut typed more slowly than that still works, but GNOME and the application see Super for a moment before it turns into Ctrl.
- Super+Shift or Super+Ctrl pressed and released with no other key sends nothing.
- Only 64-bit Linux is supported.
