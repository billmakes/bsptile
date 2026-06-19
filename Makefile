BINARY  = bsptile
CTL     = bsptilectl
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

.PHONY: build build-daemon build-ctl install test test-integration clean

build: build-daemon build-ctl

build-daemon:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

build-ctl:
	go build -ldflags "$(LDFLAGS)" -o $(CTL) ./cmd/bsptilectl

PREFIX  = $(HOME)/.local

install: build
	install -Dm755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	install -Dm755 $(CTL) $(PREFIX)/bin/$(CTL)

test:
	go test ./...

test-integration: build
	./scripts/test-x11-integration.sh

clean:
	rm -f $(BINARY) $(CTL)
