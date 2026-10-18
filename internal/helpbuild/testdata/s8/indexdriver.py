"""Python twin of internal/helpbuild/parity_s8_test.go (slice S8).

Run with `python3 -I -B indexdriver.py TOOLS MODE BASE` (see
../parity/driver.py). Every mode runs one builder on the data root BASE and
prints JSON lines (json.dumps defaults) that the Go test rebuilds:
["out", line] per printed line, then ["exit", status] (0 when the builder
returns) or ["raise", exception class name], then ["took", seconds] (the
builder's own run time, not compared).

  class    build_class_index.main()   indexes/class-index.json (+ jdk-class-index.json)
  inherit  inheritance_builder.main() indexes/inheritance.json
  method   method_indexer's __main__  indexes/method-index.json

testdata/s8/golden/ holds the output of every mode on a copy of
testdata/s8/root/ (the copy's path printed as <base>): regenerate it with
`go test -run TestParityS8Synthetic -args -update-s8` when the Python
builders change; TestBuildClassIndexFixture, TestBuildInheritanceFixture and
TestBuildMethodIndexFixture compare the Go builders with it without Python.
"""

import contextlib
import io
import json
import sys
import time

sys.path.insert(0, sys.argv[1])

import build_class_index  # noqa: E402
import inheritance_builder  # noqa: E402
import method_indexer  # noqa: E402


def run(mode, base):
    out = io.StringIO()
    start = time.perf_counter()
    try:
        with contextlib.redirect_stdout(out):
            try:
                if mode == "class":
                    sys.argv = ["build_class_index.py", base]
                    build_class_index.main()
                elif mode == "inherit":
                    sys.argv = ["inheritance_builder.py", base]
                    inheritance_builder.main()
                elif mode == "method":
                    method_indexer.build_method_index(base)
                else:
                    raise SystemExit("unknown mode " + mode)
                last = ["exit", 0]
            except SystemExit as e:
                last = ["exit", e.code]
    except Exception as e:  # noqa: BLE001 - the class is the result
        last = ["raise", type(e).__name__]
    took = time.perf_counter() - start
    for line in out.getvalue().splitlines():
        print(json.dumps(["out", line]))
    print(json.dumps(last))
    print(json.dumps(["took", round(took, 3)]))


if __name__ == "__main__":
    run(sys.argv[2], sys.argv[3])
