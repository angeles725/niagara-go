#!/usr/bin/env bash
# wintest.sh: cross-compile each package's tests for windows/amd64 and run them on
# the Windows host through WSL interop, so *_windows.go code paths actually execute.
#
# Usage: scripts/wintest.sh            (or: make wintest)
# Environment:
#   WINTEST_PKGS    space-separated package patterns (default: every package with tests)
#   WINTEST_RUN     regex passed as -test.run
#   WINTEST_SHORT   1 (default) passes -test.short; 0 runs the full suites
#   WINTEST_TIMEOUT per-package -test.timeout (default 10m)
#   WINTEST_MIRROR  1 (default) runs each binary inside a copy of the tree on the Windows
#                   disk; 0 runs it in place, from the \\wsl.localhost\... UNC path
#   GO              go command (default: go)
#
# Each test binary runs with its package directory as the working directory, as `go test`
# does, so relative testdata/ reads resolve. In place that directory is a UNC path, where
# t.Chdir panics on restore ("chdir .: The system cannot find the path specified") and
# aborts the package, so by default the tracked and untracked-unignored files are copied to
# %TEMP%\niagara-go-wintest\<tree> first and removed on exit. Exit status is non-zero when
# any package fails.
set -euo pipefail

GO=${GO:-go}
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

if [[ ! -e /proc/sys/fs/binfmt_misc/WSLInterop ]] && ! uname -r | grep -qi microsoft; then
	echo "wintest: skipped (not running under WSL)"
	exit 0
fi

read -r -a patterns <<<"${WINTEST_PKGS:-./...}"
mapfile -t pkgs < <(GOOS=windows GOARCH=amd64 "$GO" list \
	-f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}} {{.Dir}}{{end}}' "${patterns[@]}")

args=(-test.v=false "-test.timeout=${WINTEST_TIMEOUT:-10m}")
if [[ ${WINTEST_SHORT:-1} != 0 ]]; then args+=(-test.short); fi
if [[ -n ${WINTEST_RUN:-} ]]; then args+=("-test.run=$WINTEST_RUN"); fi

runroot=$root
if [[ ${WINTEST_MIRROR:-1} != 0 ]]; then
	wintemp=$(cmd.exe /c 'echo %TEMP%' 2>/dev/null | tr -d '\r') || wintemp=
	if [[ -z $wintemp ]]; then
		echo "wintest: cannot read the Windows %TEMP% through cmd.exe; set WINTEST_MIRROR=0" >&2
		exit 1
	fi
	runroot=$(wslpath -u "$wintemp")/niagara-go-wintest/$(basename "$root")
	winroot=$(wslpath -w "$(dirname "$runroot")")\\$(basename "$runroot")
	# Native Windows tools for the copy and the cleanup: file-by-file work over /mnt/c takes
	# minutes for a few thousand files.
	winrm() { (cd /mnt/c && cmd.exe /c rmdir /s /q "$winroot" >/dev/null 2>&1) || true; rm -f "$runroot.tar"; rmdir "$(dirname "$runroot")" 2>/dev/null || true; }
	winrm
	trap winrm EXIT
	mkdir -p "$runroot"
	git -C "$root" ls-files -z -c -o --exclude-standard | tar -C "$root" --format=pax --null -T - -cf "$runroot.tar"
	# pax keeps non-ASCII names UTF-8 (Windows tar reads plain ustar names in the OEM code page).
	# Windows tar cannot create symlinks without the privilege; those fixtures are reported,
	# not dropped silently, and the tests that need them fail on their own.
	if ! (cd "$runroot" && /mnt/c/Windows/System32/tar.exe -xf "$(wslpath -w "$runroot.tar")") 2>"$runroot.tar.err"; then
		echo "wintest: not copied to the Windows disk (symlinks or unsupported names):"
		grep -o "^[^:]*: Can't create" "$runroot.tar.err" | sed "s/: Can't create//; s/^/    /" || cat "$runroot.tar.err"
	fi
	rm -f "$runroot.tar" "$runroot.tar.err"
	echo "wintest: running from $(wslpath -w "$runroot")"
fi

outdir=$root/bin/wintest
mkdir -p "$outdir"
module=$("$GO" list -m)
total=0 ok=0 failed=0 skipped=0
for line in "${pkgs[@]}"; do
	[[ -z $line ]] && continue
	pkg=${line%% *} dir=${line#* }
	rel=${pkg#"$module"/}
	exe=$outdir/${rel//\//_}.test.exe
	log=$outdir/${rel//\//_}.log
	total=$((total + 1))
	rm -f "$exe"
	if ! GOOS=windows GOARCH=amd64 "$GO" test -c -o "$exe" "$pkg" >"$log" 2>&1; then
		failed=$((failed + 1))
		echo "FAIL $rel (build)"
		tail -n 20 "$log" | sed 's/^/    /'
		continue
	fi
	if [[ ! -f $exe ]]; then
		# go test -c writes nothing when the windows build has no test files.
		skipped=$((skipped + 1))
		echo "skip $rel (no windows test binary)"
		continue
	fi
	start=$(date +%s.%N)
	status=0
	(cd "$runroot${dir#"$root"}" && "$exe" "${args[@]}") >"$log" 2>&1 || status=$?
	secs=$(awk -v a="$start" -v b="$(date +%s.%N)" 'BEGIN { printf "%.1f", b - a }')
	if [[ $status -eq 0 ]]; then
		ok=$((ok + 1))
		echo "ok   $rel ${secs}s"
	else
		failed=$((failed + 1))
		echo "FAIL $rel ${secs}s (exit $status)"
		tail -n 30 "$log" | sed 's/^/    /'
	fi
done

echo "wintest: $total packages, $ok ok, $failed failed, $skipped skipped (logs in bin/wintest/)"
[[ $failed -eq 0 ]]
