GO_DIR  := chesstui
BINARY  := build/chesstui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build run test race vet format shots wasm live snapshot clean

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

# Build the browser demo for web/public/demo. Bubble Tea has no js/wasm
# port, so a copy patched with cmd/chesstui-web/_overlay is swapped in
# through a separate go.mod; the main module is untouched.
WEB_BUILD := $(abspath build/web)
wasm:
	$(RM) -r $(WEB_BUILD) && mkdir -p $(WEB_BUILD) web/public/demo
	cp -R "$$(cd $(GO_DIR) && go list -m -f '{{.Dir}}' charm.land/bubbletea/v2)" $(WEB_BUILD)/bubbletea
	chmod -R u+w $(WEB_BUILD)/bubbletea
	cp $(GO_DIR)/cmd/chesstui-web/_overlay/bubbletea_js.go $(WEB_BUILD)/bubbletea/
	cp $(GO_DIR)/go.mod $(WEB_BUILD)/web.mod && cp $(GO_DIR)/go.sum $(WEB_BUILD)/web.sum
	cd $(GO_DIR) && go mod edit -modfile=$(WEB_BUILD)/web.mod -replace charm.land/bubbletea/v2=$(WEB_BUILD)/bubbletea
	cd $(GO_DIR) && GOOS=js GOARCH=wasm go build -modfile=$(WEB_BUILD)/web.mod -trimpath -ldflags "-s -w" -o ../web/public/demo/chesstui.wasm ./cmd/chesstui-web
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/public/demo/

# Hit the real lichess.org public API (about eight requests). Run rarely.
live:
	cd $(GO_DIR) && go test -tags live -run Live ./internal/lichess

# Build every release archive locally without publishing anything.
snapshot:
	goreleaser release --snapshot --clean

clean:
	$(RM) -r $(BINARY) dist
