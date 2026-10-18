"""Python twin of internal/helpbuild/parity_rebuild_test.go (rebuild-all, S10a).

Run with `python3 -I -B driver.py TOOLS MODE ARGS...`, where TOOLS is the
niagara-help tools/ directory (put on sys.path explicitly: -I leaves the
script directory and PYTHONPATH out). The builders run as child processes
(sys.executable tools/<script> ROOT, as maintenance.cmd_rebuild_all starts
them); PYTHONDONTWRITEBYTECODE keeps them from writing __pycache__ into the
checkout.

build_call_index.py is taken out of maintenance.BUILDERS before anything
runs: call-index.json is retired in Go (decision I3-5), the one intended
difference of the plan (the `plan` mode shows what was taken out).

The parent's stdout is line-buffered, so its lines interleave with the
children's as they do on a terminal (to a pipe or a file Python would
hold its own lines back until it exits and print the children's first).

  plan                 one JSON line: {"removed": [scripts taken out],
                       "builders": [[script, produces, source, skip_if]...],
                       "optional": OPTIONAL_ARTIFACTS}
  run ROOT [--only X] [--dry-run] [--stamp-only]
                       maintenance.cmd_rebuild_all(ROOT, ...): stdout and
                       stderr are the process's own
  log ROOT             version_setup._rebuild(ROOT, out): rebuild-all with
                       its stdout in ROOT/build.log; then the out() lines
                       on stdout, one per line
"""

import json
import os
import sys

TOOLS = sys.argv[1]
sys.path.insert(0, TOOLS)
os.environ["PYTHONDONTWRITEBYTECODE"] = "1"

from niagara_help_lib import maintenance, version_setup  # noqa: E402

RETIRED = ("build_call_index.py",)
removed = [b[0] for b in maintenance.BUILDERS if b[0] in RETIRED]
maintenance.BUILDERS[:] = [b for b in maintenance.BUILDERS
                           if b[0] not in RETIRED]
sys.stdout.reconfigure(line_buffering=True)


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode == "plan":
        print(json.dumps({"removed": removed,
                          "builders": [list(b) for b in maintenance.BUILDERS],
                          "optional": maintenance.OPTIONAL_ARTIFACTS}))
    elif mode == "run":
        root, rest = args[0], args[1:]
        only = None
        if "--only" in rest:
            only = rest[rest.index("--only") + 1]
        maintenance.cmd_rebuild_all(root, only=only,
                                    dry_run="--dry-run" in rest,
                                    stamp_only="--stamp-only" in rest)
    elif mode == "log":
        lines = []
        version_setup._rebuild(args[0], lines.append)
        for line in lines:
            print(line)
    else:
        sys.exit("unknown mode " + mode)


main()
