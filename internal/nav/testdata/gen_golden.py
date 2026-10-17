"""Regenerate the fixture-backed golden cases of the ported mnav commands.

    python3 -I internal/nav/testdata/gen_golden.py [--fixture]

--fixture first rebuilds fixture/ (a small class index and module inventory cut
from the real module-navigator indexes, plus a synthetic module.xml); then
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
CORPUS = {"NAV_CORPUS_BASE": "fixture/organized"}

CLASS_NAMES = ["BAlarmService", "BAlarmClass", "BAlarmRecord", "AlarmDbConnection", "BAckState",
               "AlarmSupport", "BAlarmArchive", "BAlarmablePointMarkerTag", "BAlarm", "Entry",
               "BAlarmConsole", "BComponent", "BAlarmSourceInfo", "BAlarmAcknowledger"]
MODULES = ["alarm-rt", "alarm-ux", "alarm-wb", "alarm-se", "alarmOrion-rt", "apachePoi-rt",
           "docSource-doc", "baja", "backup-rt", "opcUaCore-rt", "obix-se", "niagaraTest-se", "lonNico-rt"]

# Python fails on these with a traceback; the port prints what the case's
# intended.json holds instead.
INTENDED = {
    "module-null-bytecode": {
        "reason": "Python raises TypeError comparing a null bytecode with 45 after printing half "
                  "the detail block; the port prints the whole block with 'vNone (Java ?)'",
        "stdout": "\n  Module:      lonNico-rt\n  Parent:      lonNico\n  Type:        rt\n"
                  "  JAR:         lonNico-rt.jar\n  Has code:    no\n  Class count: 0\n"
                  "  Java files:  0\n  ZKM:         YES\n  Bytecode:    vNone (Java ?)\n"
                  "  Vineflower:  no\n  CFR:         yes\n\n",
        "stderr": "",
        "exit": 0,
    },
}

MODULE_XML = """<?xml version="1.0" encoding="UTF-8"?>
<module name="alarm-rt" vendor="Tridium" preferredSymbol="alarm" moduleName="alarm">
  <dependencies>
    <dependency name="baja" vendor="Tridium" vendorVersion="4.14.0"/>
    <dependency name="aVeryLongDependencyModuleNameThatIsCut-rt" vendor="TridiumVendorLong" vendorVersion="4.14.0"/>
    <dependency name="control-rt" vendor="Tridium"/>
  </dependencies>
  <permissions>
    <niagara-permission-groups type="station"/>
    <java-permissions type="workbench">
      <java-permission class="java.lang.RuntimePermission" name="loadLibrary.alarmDialog"/>
      <java-permission class="com.tridium.nre.security.NiagaraSocketPermission" name="*:1-100000" action="connect,resolve"/>
    </java-permissions>
    <java-permissions type="station">
      <java-permission class="java.io.FilePermission" name="${protected.station.home}/-" action="read"/>
    </java-permissions>
  </permissions>
</module>
"""

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
    ("inventory", ["inventory"], CORPUS),
    ("module", ["module", "alarm-rt"], CORPUS),
    ("module-multiple", ["module", "alarm"], CORPUS),
    ("module-miss", ["module", "alarmz"], CORPUS),
    ("module-deps", ["module", "alarm-rt", "--dependencies"], CORPUS),
    ("module-perms", ["module", "alarm-rt", "--permissions", "--dependencies"], CORPUS),
    ("module-xml-missing", ["module", "backup-rt", "--permissions"], CORPUS),
    ("modules-type", ["modules", "--type", "rt"], CORPUS),
    ("modules-zkm-top", ["modules", "--zkm", "--top", "2"], CORPUS),
    ("modules-bytecode", ["modules", "-b", "52", "--has-code"], CORPUS),
    ("module-null-bytecode", ["module", "lonNico-rt"], CORPUS),
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
    with open(os.path.join(NAV, "indexes", "module-inventory.json"), encoding="utf-8") as f:
        inv = json.load(f)
    mods = {n: inv["modules"][n] for n in MODULES}
    with open(os.path.join(idx, "module-inventory.json"), "w", encoding="utf-8") as f:
        json.dump({"_meta": inv["_meta"], "modules": mods}, f)
    xml = os.path.join(fx, "organized", "alarm", "alarm-rt", "extracted", "META-INF")
    os.makedirs(xml)
    with open(os.path.join(xml, "module.xml"), "w", encoding="utf-8") as f:
        f.write(MODULE_XML)


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
    if cid[len("nav-"):] in INTENDED:
        with open(os.path.join(d, "intended.json"), "w", encoding="utf-8") as f:
            json.dump(INTENDED[cid[len("nav-"):]], f, indent=1)
            f.write("\n")


def main():
    if "--fixture" in sys.argv:
        build_fixture()
    for cid, args, env in CASES:
        capture("nav-" + cid, args, env)


if __name__ == "__main__":
    main()
