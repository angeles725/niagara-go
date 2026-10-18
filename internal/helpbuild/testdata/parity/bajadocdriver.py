"""Python twin of internal/helpbuild/parity_bajadoc_test.go (slice S7a) and
parity_bajadocxml_test.go (slice S7b).

Run with `python3 -I -B bajadocdriver.py TOOLS MODE ARGS...` (see driver.py).
Every mode prints JSON lines (json.dumps defaults) that the Go test rebuilds.

  run BASE         bajadoc_parser.main() with BASE as its argument (a data
                   root holding bajadoc/javax/baja/): ["out", line] per
                   printed line, then ["exit", status] (0 when main returns)
                   or ["raise", exception class name]
  xmlrun BASE      the same for bajadoc_xml_parser.main() (.bajadoc XML in
                   BASE/devguide/ and BASE/guides/ -> BASE/bajadoc-clean/)

testdata/bajadoc/clean/ and testdata/bajadoc/stdout.txt (the golden output
of TestParseBajadocFixture) are this mode's output on a copy of
testdata/bajadoc/root/, the copy's path printed as <base> and the
"Done in" seconds as <t>; TestParityBajadocSynthetic re-checks them.
testdata/bajadocxml/clean/ and testdata/bajadocxml/stdout.txt are mode
xmlrun's output on a copy of testdata/bajadocxml/root/;
TestParityBajadocXMLSynthetic re-checks them.
"""

import contextlib
import io
import json
import sys

sys.path.insert(0, sys.argv[1])

import bajadoc_parser  # noqa: E402
import bajadoc_xml_parser  # noqa: E402


def run(base, module=bajadoc_parser, script="bajadoc_parser.py"):
    out = io.StringIO()
    try:
        with contextlib.redirect_stdout(out):
            sys.argv = [script, base]
            try:
                module.main()
                last = ["exit", 0]
            except SystemExit as e:
                last = ["exit", e.code]
    except Exception as e:  # noqa: BLE001 - the class is the result
        last = ["raise", type(e).__name__]
    for line in out.getvalue().splitlines():
        print(json.dumps(["out", line]))
    print(json.dumps(last))


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode == "run":
        run(args[0])
    elif mode == "xmlrun":
        run(args[0], bajadoc_xml_parser, "bajadoc_xml_parser.py")
    else:
        raise SystemExit("unknown mode " + mode)


if __name__ == "__main__":
    main()
