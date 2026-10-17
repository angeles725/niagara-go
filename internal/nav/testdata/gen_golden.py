"""Regenerate the fixture-backed golden cases of the ported mnav commands.

    python3 -I internal/nav/testdata/gen_golden.py [--fixture]

--fixture first rebuilds fixture/ (a small class index cut from the real
module-navigator class index); then
every case below is run through Python module_nav.py from this directory
(COLUMNS=100, NO_COLOR=1) and written to golden/<case>/.
"""

import json
import os
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
NAV = os.path.expanduser("~/niagara-research/module-navigator")
CLI = os.path.join(NAV, "tools", "module_nav.py")
BASE = "fixture/nav"
NO_CORPUS = {"NAV_CORPUS_BASE": "fixture/no-corpus"}

CLASS_NAMES = ["BAlarmService", "BAlarmClass", "BAlarmRecord", "AlarmDbConnection", "BAckState",
               "AlarmSupport", "BAlarmArchive", "BAlarmablePointMarkerTag", "BAlarm", "Entry",
               "BAlarmConsole", "BComponent", "BAlarmSourceInfo", "BAlarmAcknowledger"]
CASES = [
    ("class", ["class", "BAlarmService"], NO_CORPUS),
    ("class-detail", ["class", "BAlarmConsole"], NO_CORPUS),
    ("class-fqn", ["class", "javax.baja.alarm.BAlarmService"], NO_CORPUS),
    ("class-case", ["class", "balarmservice"], NO_CORPUS),
    ("class-miss", ["class", "javax.baja.BNoSuchClassXyz"], NO_CORPUS),
    ("search", ["search", "BAlarm*", "-n", "4"], NO_CORPUS),
    ("search-miss", ["search", "*Nothing*"], NO_CORPUS),
    ("package", ["package", "javax.baja.alarm", "-n", "3"], NO_CORPUS),
    ("package-sub", ["package", "javax.baja"], NO_CORPUS),
    ("package-all", ["package", "--all"], NO_CORPUS),
    ("package-miss", ["package", "org.nothing"], NO_CORPUS),
    ("package-usage", ["package"], NO_CORPUS),
]


def build_fixture():
    fx = os.path.join(HERE, "fixture")
    shutil.rmtree(fx, ignore_errors=True)
    idx = os.path.join(fx, "nav", "indexes")
    os.makedirs(idx)
    with open(os.path.join(NAV, "indexes", "class-index.json"), encoding="utf-8") as f:
        ci = json.load(f)
    meta = {k: ci["_meta"][k] for k in ("description", "modules", "timestamp")}
    classes = {n: ci["classes"][n] for n in CLASS_NAMES}
    with open(os.path.join(idx, "class-index.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": meta, "classes": classes}, f)


def capture(cid, args, env):
    argv = args if args[0] == "--base-dir" else ["--base-dir", BASE] + args
    full_env = dict(os.environ, COLUMNS="100", NO_COLOR="1", **env)
    p = subprocess.run([sys.executable, CLI] + argv, cwd=HERE, env=full_env,
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    d = os.path.join(HERE, "golden", cid)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    with open(os.path.join(d, "cmd.json"), "w", encoding="utf-8") as f:
        json.dump({"args": argv, "env": env}, f)
        f.write("\n")
    for name, data in (("stdout", p.stdout), ("stderr", p.stderr)):
        if data or name == "stdout":
            with open(os.path.join(d, name), "wb") as f:
                f.write(data)
    with open(os.path.join(d, "exit"), "w") as f:
        f.write("%d\n" % p.returncode)


def main():
    if "--fixture" in sys.argv:
        build_fixture()
    for cid, args, env in CASES:
        capture("nav-" + cid, args, env)


if __name__ == "__main__":
    main()
