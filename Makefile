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

.PHONY: all build cross test vet fmt-check check lint deadcode cover ci wintest realdata hooks clean

all: check build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/ ./cmd/...

cross:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/linux-amd64/ ./cmd/...
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN)/windows-amd64/ ./cmd/...

test:
	$(GO) test ./...

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
# on findings; only a load error fails). The test-only packages internal/golden and
# internal/help/fts/ftsfixture (with rawfixture) are filtered out: no command imports
# them by design, so every function in them would be listed.
# procgroup Cmd.Output and Cmd.CombinedOutput are filtered too: they intentionally shadow the
# embedded exec.Cmd methods so c.Output() cannot bypass the process-group kill.
DEADCODE_TESTONLY := ^internal/(golden|help/fts/ftsfixture)/|^internal/procgroup/procgroup\.go:[0-9]+:[0-9]+: unreachable func: Cmd\.(Output|CombinedOutput)$$
deadcode:
	@out=$$($(GO) run $(DEADCODE) ./...) || exit $$?; \
		[ -z "$$out" ] || printf '%s\n' "$$out" | grep -vE '$(DEADCODE_TESTONLY)' || true

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
# `make realdata` runs the permanent real-data gates, which need no Python: the search-quality gate
# TestSearchQuality (internal/searchdoor/quality_test.go; hit@1/hit@5 per source must not fall below
# its floor), the mnav and nh real-corpus goldens, helpbuild's real coverage goldens, course, bog
# recall and hdbread. The suites that compared Go with the frozen Python reference retired with D47;
# the committed goldens are the oracle. See docs/goldens.md.
NIAGARA_RESEARCH ?= $(HOME)/niagara-research
RESEARCH := $(abspath $(NIAGARA_RESEARCH))
# Live/station suites (course, bog) read host-specific data that never enters the repo, so they are
# opt-in: export their env to run them, otherwise they skip visibly.
#   course: COURSE_TRAINING_STATION=<...>/TrainingStation/config.bog [COURSE_NIAGARA_INSTALL=<install>]
#   bog recall: BOG_RECALL_FILES=<bog>[:<bog>...]
# hdbread real history: HDBREAD_REAL_HDB, by default the HARBOR UMA14 Ret history of the checkout
# (682 records); an empty value skips it, a missing file fails. Write a `$` of an overriding path
# as `$$`, on the command line or in the environment (docs/goldens.md).
HDBREAD_REAL_HDB ?= $(RESEARCH)/clients/distech-merida-harbor/HM_BMS_backup_08_09_2026/HM_BMS/history/station/seg1/UMA14$$20Ret.hdb

realdata:
	NSEARCH_QUALITY_ROOT=$(RESEARCH) $(GO) test -count=1 -v -timeout 30m \
		-run '^(TestSearchQuality|TestQualityFixtureMatchesResearch)$$' ./internal/searchdoor
	MODULE_NAVIGATOR_DIR=$(RESEARCH)/module-navigator $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/nav
	NIAGARA_HELP_DIR=$(RESEARCH)/niagara-help $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/help
	NIAGARA_HELP_DIR=$(RESEARCH)/niagara-help $(GO) test -count=1 -timeout 30m \
		-run '^TestCoverageRealGoldens$$' ./internal/helpbuild
	NIAGARA_RESEARCH_DIR=$(RESEARCH) $(GO) test -count=1 -timeout 30m \
		-run '^(TestCheckLiveStations|TestVerifyPredictionsTrainingStation)$$' ./internal/course/check
	NIAGARA_RESEARCH_DIR=$(RESEARCH) $(GO) test -count=1 -timeout 30m \
		-run '^(TestRecallReal|TestQuizPoolReal)$$' ./internal/course/key ./internal/course
	$(GO) test -count=1 -timeout 30m -run '^TestRecallRealBogs$$' ./internal/bog
	HDBREAD_REAL_HDB='$(HDBREAD_REAL_HDB)' $(GO) test -count=1 -timeout 5m \
		-run '^TestRealHarborHistory$$' ./internal/hdbread

hooks:
	git config core.hooksPath scripts/git-hooks

clean:
	rm -rf $(BIN)
