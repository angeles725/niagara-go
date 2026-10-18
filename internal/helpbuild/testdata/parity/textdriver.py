"""Python twin of the S3/S4 parity tests (parity_pdf_test.go,
parity_course_test.go).

Run with `python3 -I -B textdriver.py TOOLS MODE ARGS...`, TOOLS being the
niagara-help tools/ directory (put on sys.path explicitly). Every mode
prints JSON lines the Go test rebuilds from internal/helpbuild.

  pdf BASE [DOCS]   pdf_extractor.main() with argv [BASE] (+ --docs-dir
                    DOCS): ["stdout", text], then ["exit", code] or
                    ["raise", exception class name]. pdftotext is what
                    PATH resolves; $HB_PDF_TIMEOUT, when set, replaces the
                    120 s timeout of one PDF (the timeout fixture), nothing
                    else of subprocess.run changes.
  course BASE       course_extractor.main() with argv [BASE]: ["stdout",
                    text], ["stderr", text], ["exit", code] or ["raise",
                    class name]
  plan SRC          course.plan(SRC): per file [rel, kind, dest, the
                    "(course)" exclude rule id]
"""

import contextlib
import io
import json
import os
import subprocess
import sys

sys.path.insert(0, sys.argv[1])

import course_extractor  # noqa: E402
import pdf_extractor  # noqa: E402
from niagara_help_lib import course, install_rules  # noqa: E402


class _Subprocess:
    """pdf_extractor's subprocess module, with the 120 s pdftotext timeout
    replaced by $HB_PDF_TIMEOUT (the fixture cannot wait two minutes)."""

    def __getattr__(self, name):
        return getattr(subprocess, name)

    def run(self, argv, **kw):
        if kw.get("timeout") == 120 and os.environ.get("HB_PDF_TIMEOUT"):
            kw["timeout"] = float(os.environ["HB_PDF_TIMEOUT"])
        return subprocess.run(argv, **kw)


def _run(main, argv):
    """(stdout, stderr, exit code or exception class name) of main()."""
    out, err = io.StringIO(), io.StringIO()
    sys.argv = argv
    code, raised = 0, None
    try:
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = main()
        code = rc or 0
    except SystemExit as e:
        code = e.code or 0
    except Exception as e:  # noqa: BLE001 - the class is the result
        raised = type(e).__name__
    return out.getvalue(), err.getvalue(), code, raised


def _report(out, err, code, raised, with_stderr):
    print(json.dumps(["stdout", out]))
    if with_stderr:
        print(json.dumps(["stderr", err]))
    print(json.dumps(["raise", raised] if raised else ["exit", code]))


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode == "pdf":
        pdf_extractor.subprocess = _Subprocess()
        argv = ["pdf_extractor.py", args[0]]
        if len(args) > 1:
            argv += ["--docs-dir", args[1]]
        out, err, code, raised = _run(pdf_extractor.main, argv)
        _report(out, err, code, raised, False)
    elif mode == "course":
        _report(*_run(course_extractor.main, ["course_extractor.py", args[0]]), True)
    elif mode == "plan":
        rules = install_rules.default()
        for rel, kind, dest in course.plan(args[0]):
            rule = rules.exclude_rule(course.COURSE_PSEUDO_JAR, rel)
            print(json.dumps([rel, kind, dest, rule.id if rule else None]))
    else:
        sys.exit("unknown mode " + mode)


if __name__ == "__main__":
    main()
