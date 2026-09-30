PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
UNITDIR := $(HOME)/.config/systemd/user
DATADIR := $(HOME)/.local/share
APPID   := dev.nicklasfrahm.Osxd
ICON    := $(DATADIR)/icons/hicolor/scalable/apps/$(APPID).svg
DESKTOP := $(DATADIR)/applications/$(APPID).desktop
BIN     := osxd

.PHONY: build test install uninstall clean

build:
	go build -o $(BIN) ./cmd/osxd

test:
	go test ./...

# Installs the binary, its icon and desktop entry, and a systemd user service,
# then enables and (re)starts it. The desktop entry lets GNOME show the icon for
# osxd's windows.
install: build
	install -Dm755 $(BIN) $(BINDIR)/$(BIN)
	install -Dm644 contrib/$(APPID).svg $(ICON)
	install -d $(dir $(DESKTOP))
	sed 's|@BINDIR@|$(BINDIR)|' contrib/$(APPID).desktop > $(DESKTOP)
	-gtk4-update-icon-cache -qtf $(DATADIR)/icons/hicolor
	install -d $(UNITDIR)
	sed 's|@BINDIR@|$(BINDIR)|' contrib/osxd.service > $(UNITDIR)/osxd.service
	systemctl --user daemon-reload
	systemctl --user enable osxd.service
	systemctl --user restart osxd.service

# Gives Super+Space back to its previous bindings, then removes the service,
# binary, icon and desktop entry.
uninstall:
	-$(BINDIR)/$(BIN) --restore
	-systemctl --user disable --now osxd.service
	rm -f $(UNITDIR)/osxd.service $(BINDIR)/$(BIN) $(ICON) $(DESKTOP)
	systemctl --user daemon-reload

clean:
	rm -f $(BIN)
