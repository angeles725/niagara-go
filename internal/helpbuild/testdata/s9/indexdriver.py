"""Python twin of internal/helpbuild/parity_s9_test.go (slice S9).

Run with `python3 -I -B indexdriver.py TOOLS MODE BASE` (see
../parity/driver.py). Every mode runs one builder on the data root BASE, as
the script's __main__ does with BASE as its argument, and prints JSON lines
(json.dumps defaults) that the Go test rebuilds: ["out", line] per line
printed on stdout, ["err", line] per line printed on stderr (after the
stdout lines), then ["exit", status] (0 when the builder returns) or
["raise", exception class name], then ["took", seconds] (the builder's own
run time, not compared).

  source  source_indexer.index_source()      indexes/source-index.json
  slots   slots_indexer.build_slots_index()  indexes/slots-index.json
  xref    xref_builder.build_xref()          indexes/xref-index.json

testdata/s9/golden/ holds the output of every mode on a copy of
testdata/s9/root/ (the copy's path printed as <base>): regenerate it with
`go test -run TestParityS9Synthetic -args -update-s9` when the Python
builders change; the Build*Fixture tests compare the Go builders with it
without Python.
"""

import contextlib
import io
import json
import sys
import time

sys.path.insert(0, sys.argv[1])

import slots_indexer  # noqa: E402
import source_indexer  # noqa: E402
import xref_builder  # noqa: E402

BUILDERS = {
    "source": source_indexer.index_source,
    "slots": slots_indexer.build_slots_index,
    "xref": xref_builder.build_xref,
}


def run(mode, base):
    out, err = io.StringIO(), io.StringIO()
    start = time.perf_counter()
    try:
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            BUILDERS[mode](base)
        last = ["exit", 0]
    except Exception as e:  # noqa: BLE001 - the class is the result
        last = ["raise", type(e).__name__]
    took = time.perf_counter() - start
    for line in out.getvalue().splitlines():
        print(json.dumps(["out", line]))
    for line in err.getvalue().splitlines():
        print(json.dumps(["err", line]))
    print(json.dumps(last))
    print(json.dumps(["took", round(took, 3)]))


if __name__ == "__main__":
    run(sys.argv[2], sys.argv[3])
