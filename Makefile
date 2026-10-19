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

.PHONY: all build cross test vet fmt-check check lint deadcode cover ci wintest parity realdata pyparity hooks clean

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
# on findings; only a load error fails). The test-only packages internal/golden,
# internal/pyref and internal/help/fts/ftsfixture (with rawfixture) are filtered out:
# no command imports them by design, so every function in them would be listed.
DEADCODE_TESTONLY := ^internal/(golden|pyref|help/fts/ftsfixture)/
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
# its floor), the mnav and nh real-corpus goldens, course, bog recall and hdbread. `make pyparity`
# runs the suites that compare Go with the frozen Python reference (internal/pyref, D47), kept until
# D47 is closed. `make parity` runs both. See docs/goldens.md.
NIAGARA_RESEARCH ?= $(HOME)/niagara-research
RESEARCH := $(abspath $(NIAGARA_RESEARCH))
# Live/station suites (course, bog) read host-specific data that never enters the repo, so they are
# opt-in: export their env to run them, otherwise they skip visibly.
#   course: COURSE_TRAINING_STATION=<...>/TrainingStation/config.bog [COURSE_NIAGARA_INSTALL=<install>]
#   bog recall: BOG_RECALL_FILES=<bog>[:<bog>...]  bog parity: BOG_PARITY_FILES + BOG_PARITY_DIR
#     (the dump dir that internal/bog/testdata/gen_fixtures.py --dump wrote, kept outside the repo)
# hdbread real history: HDBREAD_REAL_HDB, by default the HARBOR UMA14 Ret history of the checkout
# (682 records); an empty value skips it, a missing file fails. Write a `$` of an overriding path
# as `$$`, on the command line or in the environment (docs/goldens.md).
HDBREAD_REAL_HDB ?= $(RESEARCH)/clients/distech-merida-harbor/HM_BMS_backup_08_09_2026/HM_BMS/history/station/seg1/UMA14$$20Ret.hdb
parity: realdata pyparity

realdata:
	NSEARCH_QUALITY_ROOT=$(RESEARCH) $(GO) test -count=1 -v -timeout 30m \
		-run '^(TestSearchQuality|TestQualityFixtureMatchesResearch)$$' ./internal/searchdoor
	MODULE_NAVIGATOR_DIR=$(RESEARCH)/module-navigator $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/nav
	NIAGARA_HELP_DIR=$(RESEARCH)/niagara-help $(GO) test -count=1 -timeout 30m \
		-run '^TestGoldenReal$$' ./internal/help
	NIAGARA_RESEARCH_DIR=$(RESEARCH) $(GO) test -count=1 -timeout 30m \
		-run '^(TestCheckLiveStations|TestVerifyPredictionsTrainingStation)$$' ./internal/course/check
	$(GO) test -count=1 -timeout 30m -run '^TestRecallRealBogs$$' ./internal/bog
	HDBREAD_REAL_HDB='$(HDBREAD_REAL_HDB)' $(GO) test -count=1 -timeout 5m \
		-run '^TestRealHarborHistory$$' ./internal/hdbread

# PYREF_REQUIRED=1: a missing pre-go-retire Python reference (internal/pyref, D47) fails, never skips.
pyparity: export PYREF_REQUIRED = 1
pyparity: parity-helpbuild
	NIAGARA_RESEARCH=$(RESEARCH) $(GO) test -count=1 -timeout 30m \
		-run '^TestParity' ./internal/navbuild
	NIAGARA_HELP_DIR=$(RESEARCH)/niagara-help $(GO) test -count=1 -timeout 90m -parallel 2 \
		-run '^TestBuildMatchesPythonOnRegisteredVersions$$' ./internal/helpbuild/fulltext
	$(GO) test -count=1 -timeout 30m -run '^TestParityRealBogs$$' ./internal/bog

hooks:
	git config core.hooksPath scripts/git-hooks

clean:
	rm -rf $(BIN)

# Real-corpus parity (Python vs Go on the installs under /mnt/c and the niagara-help
# versions/ trees): off in `test` and `ci`; `make pyparity` (and so `make parity`) runs it first.
# The S8 and S9 indexes (class, inheritance, method; source, slots, xref) are compared on
# three versions (about 50 s), the coverage reports (coverage, coverage <module>,
# --doc-jars) on two (2 to 4 min, mostly the --doc-jars reads over /mnt/c), the
# whole-install sweep gate (S11a) on 4.15.6.20 and 4.3.58.18 (about 90 s over a warm
# jar-listing cache, 6 min cold), and rebuild-all (S10a) rebuilds a copy of 4.3.58.18 on
# each side, PDF manuals included (about 4 min), and versions add (S10b-2) adds a subset
# copy of the 4.3.58.18 install on each side (about 90 s).
# A green run prints the result lines (subtests included) and the skips; a failing run
# prints the whole -v output but the === progress lines, so every failure message and
# diff is kept. The exit status is go test's.
.PHONY: parity-helpbuild
parity-helpbuild:
	@out=$$(HELPBUILD_PARITY_REAL=1 HELPBUILD_PARITY_FULL=1 HELPBUILD_PARITY_415=1 \
		HELPBUILD_PARITY_INDEXES=4.14.0.162:4.15.6.20:4.3.58.18 \
		HELPBUILD_PARITY_COVERAGE=4.14.0.162:4.15.6.20 \
		HELPBUILD_PARITY_SWEEP=4.15.6.20:4.3.58.18 \
		HELPBUILD_PARITY_REBUILD=4.3.58.18 \
		HELPBUILD_PARITY_VERSIONS_ADD=/mnt/c/Niagara/EC-Net4-4.3.58.18 \
		$(GO) test -count=1 -timeout 60m -run Parity -v ./internal/helpbuild/ 2>&1); st=$$?; \
		if [ $$st -eq 0 ]; then \
			printf '%s\n' "$$out" | grep -E '^ *(--- |ok|FAIL|PASS)|real-data parity|not mounted|not found|parity driver python'; \
		else \
			printf '%s\n' "$$out" | grep -vE '^ *=== (RUN|PAUSE|CONT|NAME)'; \
		fi; exit $$st
