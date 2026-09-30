PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
UNITDIR := $(HOME)/.config/systemd/user
DATADIR := $(HOME)/.local/share
APPID   := dev.nicklasfrahm.Osxd
ICON    := $(DATADIR)/icons/hicolor/scalable/apps/$(APPID).svg
DESKTOP := $(DATADIR)/applications/$(APPID).desktop
EXTDIR  := $(DATADIR)/gnome-shell/extensions/osxd@nicklasfrahm.dev
UDEV    := /etc/udev/rules.d/70-osxd.rules
BIN     := osxd

.PHONY: build test install uninstall clean

build:
	go build -o $(BIN) ./cmd/osxd

test:
	go test ./...

# Installs the binary, its icon and desktop entry, the GNOME Shell extension
# and udev rule the hotkeys use, and a systemd user service, then enables and
# (re)starts it. The desktop entry lets GNOME show the icon for osxd's windows.
# The udev rule gives the logged-in user access to keyboards and /dev/uinput;
# it is written with sudo, which asks for your password only when the rule is
# missing or changed. It goes in before the service starts, which needs it.
install: build
	cmp -s contrib/70-osxd.rules $(UDEV) || { \
		sudo install -Dm644 contrib/70-osxd.rules $(UDEV) && \
		sudo udevadm control --reload && \
		sudo udevadm trigger --settle --subsystem-match=input --subsystem-match=misc; }
	install -Dm755 $(BIN) $(BINDIR)/$(BIN)
	install -Dm644 -t $(EXTDIR) contrib/gnome-shell-extension/metadata.json contrib/gnome-shell-extension/extension.js
	install -Dm644 contrib/$(APPID).svg $(ICON)
	install -d $(dir $(DESKTOP))
	sed 's|@BINDIR@|$(BINDIR)|' contrib/$(APPID).desktop > $(DESKTOP)
	-gtk4-update-icon-cache -qtf $(DATADIR)/icons/hicolor
	install -d $(UNITDIR)
	sed 's|@BINDIR@|$(BINDIR)|' contrib/osxd.service > $(UNITDIR)/osxd.service
	systemctl --user daemon-reload
	systemctl --user enable osxd.service
	systemctl --user restart osxd.service

# Gives Super+Space back to its previous bindings and disables the extension,
# then removes the service, binary, icon, desktop entry, extension and udev
# rule.
uninstall:
	-$(BINDIR)/$(BIN) --restore
	-systemctl --user disable --now osxd.service
	rm -f $(UNITDIR)/osxd.service $(BINDIR)/$(BIN) $(ICON) $(DESKTOP)
	rm -rf $(EXTDIR)
	systemctl --user daemon-reload
	! test -e $(UDEV) || { \
		sudo rm -f $(UDEV) && \
		sudo udevadm control --reload && \
		sudo udevadm trigger --subsystem-match=input --subsystem-match=misc; }

clean:
	rm -f $(BIN)
