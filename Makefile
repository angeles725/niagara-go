# All binaries are static (CGO_ENABLED=0) so they run on any Linux or Windows host.
GO      ?= go
BIN     ?= bin
LDFLAGS := -s -w
# Analysis tools run through `go run` at an exact release; they are not module
# dependencies (go.mod stays free of them).
STATICCHECK ?= honnef.co/go/tools/cmd/staticcheck@v0.8.2
DEADCODE    ?= golang.org/x/tools/cmd/deadcode@v0.50.0
COVERPROFILE ?= $(BIN)/cover.out
# staticcheck is installed once as a host binary so it can analyse other GOOS
# targets: `GOOS=windows go run ...` would build a Windows staticcheck instead.
TOOLBIN := $(abspath $(BIN))/tools
export CGO_ENABLED=0

.PHONY: all build cross test test-short vet fmt-check check lint deadcode cover ci wintest parity hooks clean

all: check build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/ ./cmd/...

cross:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/linux-amd64/ ./cmd/...
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/windows-amd64/ ./cmd/...

test:
	$(GO) test ./...

test-short:
	$(GO) test -short ./...

vet:
	$(GO) vet ./...

fmt-check:
	@out=$$(gofmt -l $$(git ls-files '*.go')); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

check: fmt-check vet test

# Lints the host build, then the Windows build so *_windows.go files are
# checked too (they are invisible to a Linux-only pass). The pinned staticcheck is
# installed into bin/tools only when missing or a different version, so lint works offline.
lint:
	@$(TOOLBIN)/staticcheck -version 2>/dev/null | grep -qF "($(patsubst v%,%,$(lastword $(subst @, ,$(STATICCHECK)))))" || \
		GOOS= GOARCH= GOBIN=$(TOOLBIN) $(GO) install $(STATICCHECK)
	$(TOOLBIN)/staticcheck ./...
	GOOS=windows GOARCH=amd64 $(TOOLBIN)/staticcheck ./...

# Report-only: lists functions unreachable from any cmd/ main (deadcode exits 0
# on findings; only a load error fails). Test-only helpers such as internal/golden
# are listed too.
deadcode:
	$(GO) run $(DEADCODE) ./...

# Packages are mostly exercised from other packages' tests, so coverage is
# measured across ./internal/... rather than per package.
cover:
	@mkdir -p $(BIN)
	$(GO) test -short -coverpkg=./internal/... -coverprofile=$(COVERPROFILE) ./...
	@$(GO) tool cover -func=$(COVERPROFILE) > $(COVERPROFILE).func
	@tail -n 1 $(COVERPROFILE).func

ci: check lint cross

# Not part of ci: cross-compiles each package's tests for Windows and runs them on the
# Windows host through WSL interop (skips cleanly elsewhere). See scripts/wintest.sh.
wintest:
	GO=$(GO) scripts/wintest.sh

# Not part of ci: checks against the real niagara-research corpus (NIAGARA_RESEARCH, the checkout).
# The search-quality gate of tools/searchdoor/golden.py: hit@1/hit@5 per source must not fall
# below the floor in internal/searchdoor/quality_test.go. See docs/goldens.md.
NIAGARA_RESEARCH ?= $(HOME)/niagara-research
parity:
	NSEARCH_QUALITY_ROOT=$(abspath $(NIAGARA_RESEARCH)) $(GO) test -count=1 -v -timeout 30m \
		-run '^(TestSearchQuality|TestQualityFixtureMatchesResearch)$$' ./internal/searchdoor
	MODULE_NAVIGATOR_DIR=$(abspath $(NIAGARA_RESEARCH))/module-navigator $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/nav
	NIAGARA_HELP_DIR=$(abspath $(NIAGARA_RESEARCH))/niagara-help $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/help

hooks:
	git config core.hooksPath scripts/git-hooks

clean:
	rm -rf $(BIN)
