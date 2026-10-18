"""Python twin of internal/helpbuild/parity_versions_test.go (versions add).

Run with `python3 -I -B driver.py TOOLS add REPO INSTALL [--label L]
[--default] [--force] [--id ID]`, where TOOLS is the niagara-help tools/
directory (put on sys.path explicitly: -I leaves the script directory and
PYTHONPATH out). It is cli/cmd_versions.py's `versions add` with REPO as
the repository root: version_setup.add_version(...) printing on stdout,
its return value the exit status. The rebuild-all builders run as child
processes (sys.executable tools/<script> ROOT); PYTHONDONTWRITEBYTECODE
keeps them from writing __pycache__ into the checkout.

build_call_index.py is taken out of maintenance.BUILDERS first (decision
I3-5: call-index.json is retired in Go, so it is neither built nor
written empty), and stdout is line-buffered (testdata/rebuild/driver.py
does the same).
"""

import os
import sys

TOOLS = sys.argv[1]
sys.path.insert(0, TOOLS)
os.environ["PYTHONDONTWRITEBYTECODE"] = "1"

from niagara_help_lib import maintenance, version_setup  # noqa: E402

maintenance.BUILDERS[:] = [b for b in maintenance.BUILDERS
                           if b[0] != "build_call_index.py"]
sys.stdout.reconfigure(line_buffering=True)


def main():
    mode, args = sys.argv[2], sys.argv[3:]
    if mode != "add":
        sys.exit("unknown mode " + mode)
    repo, install, rest = args[0], args[1], args[2:]

    def opt(name):
        return rest[rest.index(name) + 1] if name in rest else None

    sys.exit(version_setup.add_version(
        repo, install, label=opt("--label"), make_default="--default" in rest,
        force="--force" in rest, version_id=opt("--id")))


main()
