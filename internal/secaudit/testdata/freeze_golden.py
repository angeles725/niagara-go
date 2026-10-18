"""Freeze the golden cases of internal/secaudit from the Python niagara-security-audit.py.

Usage (from anywhere):
    NIAGARA_RESEARCH=~/niagara-research python3 -I internal/secaudit/testdata/freeze_golden.py

For every case in testdata/golden/ it runs $NIAGARA_RESEARCH/tools/niagara-security-audit.py with
the case's cmd.json args and env over os.environ plus COLUMNS=100 NO_COLOR=1, in testdata/ (the
golden runner's cwd), and writes stdout, stderr and exit. intended.json is
never written: an intended difference is declared by hand. The output holds no path of the
script (argparse and the KeytoolStartError line print "niagara-security-audit.py"), so any
checkout gives the same bytes. No case runs a real keytool: the homes with a truststore run with
PATH=/nonexistent. The unreadable-dir suite needs a mode-000 directory and is not regenerated here.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    research = os.environ.get("NIAGARA_RESEARCH")
    if not research:
        sys.exit("freeze_golden.py: set NIAGARA_RESEARCH to a niagara-research checkout")
    tool = os.path.join(research, "tools", "niagara-security-audit.py")
    golden = os.path.join(HERE, "golden")
    for case in sorted(os.listdir(golden)):
        d = os.path.join(golden, case)
        with open(os.path.join(d, "cmd.json")) as fh:
            cmd = json.load(fh)
        env = dict(os.environ, COLUMNS="100", NO_COLOR="1", **cmd.get("env", {}))
        r = subprocess.run([sys.executable, tool] + cmd["args"], cwd=HERE, env=env, capture_output=True)
        with open(os.path.join(d, "stdout"), "wb") as fh:
            fh.write(r.stdout)
        with open(os.path.join(d, "stderr"), "wb") as fh:
            fh.write(r.stderr)
        with open(os.path.join(d, "exit"), "w") as fh:
            fh.write(f"{r.returncode}\n")
        print(f"{case}: exit {r.returncode}")


if __name__ == "__main__":
    main()
