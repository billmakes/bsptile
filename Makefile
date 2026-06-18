BINARY  = bsptile
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0")
COMMIT  = $(shell git rev-parse --short HEAD 2>/dev/null || echo "local")
DATE    = $(shell date --iso-8601=seconds)
TARGET  = $(shell go env GOOS)-$(shell go env GOARCH)

LDFLAGS = -s -w \
	-X main.name=$(BINARY) \
	-X main.target=$(TARGET) \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build install clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

PREFIX  = $(HOME)/.local

install: build
	install -Dm755 $(BINARY) $(PREFIX)/bin/$(BINARY)

clean:
	rm -f $(BINARY)
