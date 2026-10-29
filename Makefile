GO_DIR  := chesstui
BINARY  := build/chesstui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run test race vet format shots live snapshot clean

build:
	cd $(GO_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BINARY) ./cmd/chesstui

run:
	cd $(GO_DIR) && go run ./cmd/chesstui

test:
	cd $(GO_DIR) && go test ./...

race:
	cd $(GO_DIR) && go test -race ./...

vet:
	cd $(GO_DIR) && go vet ./...

format:
	cd $(GO_DIR) && gofmt -w cmd internal

# Regenerate docs/screenshots from the real UI against recorded Lichess data.
shots:
	cd $(GO_DIR) && go test ./internal/app -run 'Screenshots|HeroFrames' -shots ../../../docs/screenshots
	mkdir -p web/public/shots/hero
	cp docs/screenshots/*-card.svg web/public/shots/
	cp docs/screenshots/hero/*.svg web/public/shots/hero/

# Hit the real lichess.org public API (about eight requests). Run rarely.
live:
	cd $(GO_DIR) && go test -tags live -run Live ./internal/lichess

# Build every release archive locally without publishing anything.
snapshot:
	goreleaser release --snapshot --clean

clean:
	$(RM) -r $(BINARY) dist
