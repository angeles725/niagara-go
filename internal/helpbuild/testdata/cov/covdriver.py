"""Parity driver of the coverage reports (parity_cov_test.go).

    python3 -I -B covdriver.py <tools dir> docjars <repo> <base> [<vid>]
    python3 -I -B covdriver.py <tools dir> coverage <base> [<module>]
    python3 -I -B covdriver.py <tools dir> guides <base>

Prints JSON lines: ["out", <everything the command printed>], then
["status", <return value>] or, when it raised, ["missing", str(e)] for
index_cache.MissingIndexError and ["raise"] for any other exception.
scope.report_miss prints a ["miss", command, query, scope] marker instead
of the scope block (that block is the CLI's; internal/help owns it).
"""

import contextlib
import io
import json
import sys

sys.path.insert(0, sys.argv[1])

from niagara_help_lib import coverage_report, doc_coverage, scope  # noqa: E402
from niagara_help_lib.index_cache import MissingIndexError  # noqa: E402


def _miss(base_dir, command, query, scope_name):
    print(json.dumps(["miss", command, query, scope_name]))


scope.report_miss = _miss


def main(mode, args):
    buf = io.StringIO()
    result = None
    try:
        with contextlib.redirect_stdout(buf):
            if mode == "docjars":
                repo, base = args[0], args[1]
                vid = args[2] if len(args) > 2 else None
                status = doc_coverage.cmd_doc_jar_coverage(repo, base, vid)
            elif mode == "coverage":
                status = coverage_report.cmd_coverage(
                    args[0], args[1] if len(args) > 1 else None)
            elif mode == "guides":
                status = coverage_report.cmd_guide_coverage(args[0])
            else:
                raise SystemExit("unknown mode " + mode)
        result = ["status", status]
    except MissingIndexError as e:
        result = ["missing", str(e)]
    except Exception:  # noqa: BLE001 - any other raise is compared as such
        result = ["raise"]
    print(json.dumps(["out", buf.getvalue()]))
    print(json.dumps(result))


main(sys.argv[2], sys.argv[3:])
