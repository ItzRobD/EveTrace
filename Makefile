# EveTrace build & release
# -----------------------------------------------------------------------------
# EveTrace ships as a single self-contained binary: the Angular frontend is
# compiled to static files and embedded into the Go binary (build tag `embed`),
# and the SQLite driver is pure Go (modernc.org/sqlite), so every target
# cross-compiles from any machine with no C toolchain.
#
# Common commands:
#   make build        Build a binary for THIS machine into ./dist
#   make run          Build and run it
#   make release      Cross-compile every platform + package archives + AppImage
#   make appimage     Build just the Linux AppImage
#   make clean        Remove build output
#   make help         List all targets
# -----------------------------------------------------------------------------

# Version string baked into the binary (`evetrace -version`). Uses the latest
# git tag, else the short commit hash; a "-dirty" suffix means uncommitted work.
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

BACKEND  := backend
CMD      := ./cmd/EveTrace
DIST     := dist
BIN      := evetrace
TAGS     := embed

# -s -w strips debug info (smaller binary); -X injects the version variable.
LDFLAGS  := -s -w -X main.version=$(VERSION)
# CGO off = pure static binary that cross-compiles freely. -trimpath drops
# local filesystem paths from the binary for reproducible, cleaner builds.
GOBUILD   = CGO_ENABLED=0 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)"

# Host OS/ARCH (for `make build`).
HOST_OS   := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)

.DEFAULT_GOAL := help

# ─── Frontend ────────────────────────────────────────────────────────────────
# Angular builds into backend/web/dist (see web/angular.json outputPath), which
# is what the `embed` build tag bundles.
web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

.PHONY: frontend
frontend: web/node_modules ## Build the Angular frontend into backend/web/dist
	cd web && npm run build

# ─── Local build ─────────────────────────────────────────────────────────────
.PHONY: build
build: frontend ## Build a binary for the current machine into ./dist
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN) $(CMD)
	@echo "built $(DIST)/$(BIN)  (version $(VERSION))"

.PHONY: run
run: build ## Build and run EveTrace locally
	./$(DIST)/$(BIN)

# ─── Cross-compiled release binaries ─────────────────────────────────────────
# Each target writes a raw binary into ./dist. `release` bundles them.
.PHONY: linux-amd64 linux-arm64 windows-amd64 darwin-amd64 darwin-arm64

linux-amd64: frontend ## Build the Linux x86-64 binary
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN)-linux-amd64 $(CMD)

linux-arm64: frontend ## Build the Linux ARM64 binary
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN)-linux-arm64 $(CMD)

windows-amd64: frontend ## Build the Windows x86-64 binary (.exe, keeps a console window)
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN)-windows-amd64.exe $(CMD)

darwin-amd64: frontend ## Build the macOS Intel binary
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN)-darwin-amd64 $(CMD)

darwin-arm64: frontend ## Build the macOS Apple-Silicon binary
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go -C $(BACKEND) build -tags "$(TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o ../$(DIST)/$(BIN)-darwin-arm64 $(CMD)

.PHONY: binaries
binaries: linux-amd64 linux-arm64 windows-amd64 darwin-amd64 darwin-arm64 ## Build every platform binary

# ─── Packaging ───────────────────────────────────────────────────────────────
# tar.gz for Linux/macOS, zip for Windows, each with LICENSE + README.
.PHONY: package
package: binaries ## Bundle release archives into ./dist
	@cd $(DIST) && for f in $(BIN)-linux-amd64 $(BIN)-linux-arm64 $(BIN)-darwin-amd64 $(BIN)-darwin-arm64; do \
		tar -czf $$f-$(VERSION).tar.gz --transform 's,.*/,,;s,^,,' $$f ../LICENSE ../README.md ; \
		echo "packaged $(DIST)/$$f-$(VERSION).tar.gz"; \
	done
	@cd $(DIST) && cp ../LICENSE ../README.md . && \
		zip -q $(BIN)-windows-amd64-$(VERSION).zip $(BIN)-windows-amd64.exe LICENSE README.md && \
		rm -f LICENSE README.md && echo "packaged $(DIST)/$(BIN)-windows-amd64-$(VERSION).zip"

.PHONY: appimage
appimage: linux-amd64 ## Build the Linux x86-64 AppImage into ./dist
	VERSION=$(VERSION) BIN=$(DIST)/$(BIN)-linux-amd64 OUT=$(DIST) packaging/build-appimage.sh

.PHONY: release
release: package appimage ## Full release: all binaries, archives, and the AppImage
	@echo ""
	@echo "Release $(VERSION) built in ./$(DIST):"
	@ls -1 $(DIST)/*.tar.gz $(DIST)/*.zip $(DIST)/*.AppImage 2>/dev/null || true

# ─── Housekeeping ────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run the Go test suite
	go -C $(BACKEND) test ./...

.PHONY: clean
clean: ## Remove build output
	rm -rf $(DIST) $(BACKEND)/web/dist

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
