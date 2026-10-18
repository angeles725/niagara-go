"""Freeze the golden cases of internal/secaudit from the Python niagara-security-audit.py.

Usage (from anywhere):
    NIAGARA_RESEARCH=~/niagara-research python3 -I internal/secaudit/testdata/freeze_golden.py

For every case in testdata/golden/ it runs $NIAGARA_RESEARCH/tools/niagara-security-audit.py with
the case's cmd.json args and env over os.environ plus COLUMNS=100 NO_COLOR=1, in testdata/ (the
golden runner's cwd), and writes stdout, stderr and exit. intended.json is
never written: an intended difference is declared by hand. The output holds no path of the
script (argparse and the KeytoolStartError line print "niagara-security-audit.py"), so any
checkout gives the same bytes. No case runs a real keytool: the homes with a truststore run with
PATH=/nonexistent. The unreadable-dir suite (POSIX only) runs in a temp copy of
testdata/unreadable-dir holding the mode-000 directory TestGoldenUnreadableDir creates
(inputs/logs-locked/locked with an IOC in station.log); its outputs are written back to the
checked-in suite, which is never chmod'ed. The dangling-links suite (POSIX only) likewise runs
in a temp copy of testdata/dangling-links holding the two symlinks to a missing gone.log that
TestGoldenDanglingLinks creates (inputs/logs-dangling/nre.log, inputs/logs-partial/station.log).
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))


def freeze(tool, golden, cwd):
    """Run every case under golden with cwd and write its stdout, stderr and exit."""
    for case in sorted(os.listdir(golden)):
        d = os.path.join(golden, case)
        with open(os.path.join(d, "cmd.json")) as fh:
            cmd = json.load(fh)
        env = dict(os.environ, COLUMNS="100", NO_COLOR="1", **cmd.get("env", {}))
        r = subprocess.run([sys.executable, tool] + cmd["args"], cwd=cwd, env=env, capture_output=True)
        with open(os.path.join(d, "stdout"), "wb") as fh:
            fh.write(r.stdout)
        with open(os.path.join(d, "stderr"), "wb") as fh:
            fh.write(r.stderr)
        with open(os.path.join(d, "exit"), "w") as fh:
            fh.write(f"{r.returncode}\n")
        print(f"{case}: exit {r.returncode}")


def freeze_unreadable_dir(tool):
    """The unreadable-dir suite, run in a temp copy holding the locked directory (as
    TestGoldenUnreadableDir builds it)."""
    with tempfile.TemporaryDirectory() as tmp:
        suite = os.path.join(tmp, "suite")
        shutil.copytree(os.path.join(HERE, "unreadable-dir"), suite)
        locked = os.path.join(suite, "inputs", "logs-locked", "locked")
        os.mkdir(locked)
        with open(os.path.join(locked, "station.log"), "w") as fh:
            fh.write("Module validation has been DISABLED\n")
        os.chmod(locked, 0)
        try:
            try:
                os.listdir(locked)
            except PermissionError:
                pass
            else:
                sys.exit("freeze_golden.py: mode 000 does not stop this user listing a directory (root?)")
            freeze(tool, os.path.join(HERE, "unreadable-dir", "golden"), suite)
        finally:
            os.chmod(locked, 0o755)


def freeze_dangling_links(tool):
    """The dangling-links suite, run in a temp copy holding the links (as TestGoldenDanglingLinks
    builds them)."""
    with tempfile.TemporaryDirectory() as tmp:
        suite = os.path.join(tmp, "suite")
        shutil.copytree(os.path.join(HERE, "dangling-links"), suite)
        for link in ("logs-dangling/nre.log", "logs-partial/station.log"):
            path = os.path.join(suite, "inputs", link)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            os.symlink("gone.log", path)
        freeze(tool, os.path.join(HERE, "dangling-links", "golden"), suite)


def main():
    research = os.environ.get("NIAGARA_RESEARCH")
    if not research:
        sys.exit("freeze_golden.py: set NIAGARA_RESEARCH to a niagara-research checkout")
    tool = os.path.join(research, "tools", "niagara-security-audit.py")
    freeze(tool, os.path.join(HERE, "golden"), HERE)
    if os.name == "posix":
        freeze_unreadable_dir(tool)
        freeze_dangling_links(tool)


if __name__ == "__main__":
    main()
