PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
UNITDIR := $(HOME)/.config/systemd/user
BIN     := osxd

.PHONY: build test install uninstall clean

build:
	go build -o $(BIN) ./cmd/osxd

test:
	go test ./...

# Installs the binary and a systemd user service, then enables and (re)starts it.
install: build
	install -Dm755 $(BIN) $(BINDIR)/$(BIN)
	install -d $(UNITDIR)
	sed 's|@BINDIR@|$(BINDIR)|' contrib/osxd.service > $(UNITDIR)/osxd.service
	systemctl --user daemon-reload
	systemctl --user enable osxd.service
	systemctl --user restart osxd.service

# Gives Super+Space back to its previous bindings, then removes the service
# and binary.
uninstall:
	-$(BINDIR)/$(BIN) --restore
	-systemctl --user disable --now osxd.service
	rm -f $(UNITDIR)/osxd.service $(BINDIR)/$(BIN)
	systemctl --user daemon-reload

clean:
	rm -f $(BIN)
