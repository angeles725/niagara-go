"""Freeze the golden cases of internal/secaudit from the Python niagara-security-audit.py.

Usage (from anywhere), with the niagara-research Python reference (D39 d, D47):
    git -C ~/niagara-research worktree add --detach \
        ~/niagara-research-worktrees/pre-go-retire-niagara-research pre-go-retire
    python3 -I internal/secaudit/testdata/freeze_golden.py

The reference is $NIAGARA_RESEARCH_PY_REF, default
~/niagara-research-worktrees/pre-go-retire-niagara-research: a worktree of the tag pre-go-retire,
whose HEAD must be PY_REF_REV (anything else is refused; the live checkout holds only an exec shim).
At that tag tools/niagara-security-audit.py execs the Go port when run as a script (D35), so each
case runs the frozen Python below that switch: the script is loaded with runpy under another
module name and its main() is called with sys.argv[0] = the script path.

For every case in testdata/golden/ it runs that Python with the case's cmd.json args and env over
os.environ plus COLUMNS=100 NO_COLOR=1, in testdata/ (the golden runner's cwd), and writes
stdout, stderr and exit. intended.json is
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
PY_REF_ENV = "NIAGARA_RESEARCH_PY_REF"
PY_REF_REV = "c846ce1e5eb92ee6d938efbb3585473a545f7e50"  # niagara-research tag pre-go-retire
PY_REF_DEFAULT = os.path.join("~", "niagara-research-worktrees", "pre-go-retire-niagara-research")
# Run the frozen Python of the script given as argv[1], not its D35 exec switch to the Go port: its
# own directory first on sys.path (as `python3 script.py` would put it, which -I and -c do not), and
# main()'s return value as the exit status (as sys.exit(main()) would).
RUNNER = ("import os, runpy, sys; sys.argv = sys.argv[1:]; "
          "sys.path.insert(0, os.path.dirname(os.path.abspath(sys.argv[0]))); "
          "sys.exit(runpy.run_path(sys.argv[0], run_name='frozen')['main']())")


def freeze(tool, golden, cwd):
    """Run every case under golden with cwd and write its stdout, stderr and exit."""
    for case in sorted(os.listdir(golden)):
        d = os.path.join(golden, case)
        with open(os.path.join(d, "cmd.json")) as fh:
            cmd = json.load(fh)
        env = dict(os.environ, COLUMNS="100", NO_COLOR="1", **cmd.get("env", {}))
        r = subprocess.run([sys.executable, "-I", "-c", RUNNER, tool] + cmd["args"], cwd=cwd, env=env,
                           capture_output=True)
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


def python_reference():
    """The pre-go-retire worktree of niagara-research; exits when it is missing or not at the tag."""
    ref = os.path.expanduser(os.environ.get(PY_REF_ENV) or PY_REF_DEFAULT)
    head = subprocess.run(["git", "-C", ref, "rev-parse", "HEAD"], capture_output=True, text=True)
    if head.returncode != 0:
        sys.exit(f"freeze_golden.py: {ref} is not a git worktree (set {PY_REF_ENV} to a worktree of "
                 f"niagara-research tag pre-go-retire): {head.stderr.strip()}")
    if head.stdout.strip() != PY_REF_REV:
        sys.exit(f"freeze_golden.py: {ref} is at {head.stdout.strip()}, not pre-go-retire {PY_REF_REV}")
    return ref


def main():
    tool = os.path.join(python_reference(), "tools", "niagara-security-audit.py")
    freeze(tool, os.path.join(HERE, "golden"), HERE)
    if os.name == "posix":
        freeze_unreadable_dir(tool)
        freeze_dangling_links(tool)


if __name__ == "__main__":
    main()
