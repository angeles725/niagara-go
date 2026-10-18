"""Freeze the station-modules golden cases from the Python original (the specification).

    python3 -I internal/stationmodules/testdata/capture.py

Builds synthetic Niagara install trees and stations under fixtures/ (module jars are ZIPs with
fixed timestamps holding only META-INF/module.xml), then runs `python3 -I
tools/station-modules.py <args>` in fixtures/ with COLUMNS=100 and NO_COLOR=1 and stores
golden/<case>/{cmd.json,stdout,stderr,exit}. No case lets --fix copy a jar into the fixtures:
the copy path is frozen in copy/ from a run on a scratch copy of fixtures/install-copy, and
the Go test replays it the same way. Autodetection (no --install) reads the real install
paths, so it is unit-tested only.
"""
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


ROOT = py_ref("niagara-research", "NIAGARA_RESEARCH_PY_REF", "c846ce1e5eb92ee6d938efbb3585473a545f7e50")
TOOL = os.path.join(ROOT, "tools", "station-modules.py")

# At the tag, running tools/<x>.py execs the Go port (its D35 switch block); the Python under that block
# is the frozen specification. `python3 [-I] -c FROZEN <tool> <args>` runs it like `python3 [-I] <tool>
# <args>` did before the switch: the block is cut out (giving back the exact e21fe8825^ source, line
# numbers included), the code runs as __main__ under the tool's own path, and a traceback skips this frame.
FROZEN = r"""
import linecache, os, sys, types
del sys.argv[0]
tool = sys.argv[0]
with open(tool, encoding="utf-8") as fh:
    lines = fh.read().splitlines(True)
i = next(n for n, line in enumerate(lines) if line.startswith('if __name__ == "__main__":  # Go switch (D35)'))
j = i + 1
while j < len(lines) and lines[j][:1] in (" ", "\t"):
    j += 1
if j < len(lines) and not lines[j].strip():
    j += 1
elif i and not lines[i - 1].strip():
    i -= 1
del lines[i:j]
linecache.cache[tool] = (None, None, lines, tool)
if not getattr(sys.flags, "safe_path", False):  # -P / -I (Python 3.11+)
    sys.path[0] = os.path.dirname(tool)
mod = sys.modules["__main__"] = types.ModuleType("__main__")
mod.__file__ = tool
try:
    exec(compile("".join(lines), tool, "exec"), mod.__dict__)
except SystemExit:
    raise
except BaseException as exc:
    exc = exc.with_traceback(exc.__traceback__.tb_next)
    sys.excepthook(type(exc), exc, exc.__traceback__)
    sys.exit(1)
"""
FIX = os.path.join(HERE, "fixtures")
DATE = (2020, 1, 1, 0, 0, 0)
V = "4.14.0.162"


def module_xml(part, module, profile, version, types=(), deps=(), vendor="Tridium", symbol=""):
    t = "".join("<type name='%s' class='x.B%s'/>" % (n, n) for n in types)
    d = "".join("<dependency name='%s' vendor='Tridium' vendorVersion='4.14'/>" % n for n in deps)
    return ("<?xml version='1.0' encoding='UTF-8'?>\n<module name='%s' moduleName='%s' runtimeProfile='%s' "
            "preferredSymbol='%s' vendor='%s' vendorVersion='%s'>\n<dependencies>%s</dependencies>\n"
            "<defs/><types>%s</types></module>\n" % (part, module, profile, symbol, vendor, version, d, t))


def zipped(members):
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        for name, data in members:
            z.writestr(zipfile.ZipInfo(name, DATE), data)
    return buf.getvalue()


def jar(xml):
    return zipped([("META-INF/MANIFEST.MF", "Manifest-Version: 1.0\n"), ("META-INF/module.xml", xml)])


BAJA = jar(module_xml("baja-rt", "baja", "rt", V, ["Folder", "Station", "ServiceContainer"], symbol="b"))
CONTROL = jar(module_xml("control-rt", "control", "rt", V, ["NumericWritable", "BooleanWritable"], ["baja-rt"], symbol="c"))
CONTROL_WB = jar(module_xml("control-wb", "control", "wb", V, ["NumericWritableEditor"], ["control-rt", "baja-rt"]))
DASH_WB = jar(module_xml("dashboard-wb", "dashboard", "wb", V, ["DashboardView"], ["dashboard-rt", "baja-rt"]))
KIT = jar(module_xml("kitControl-rt", "kitControl", "rt", V, ["LoopPoint", "BooleanSelect"], ["kitPid-rt", "baja-rt"]))
VENDOR = jar(module_xml("acme-rt", "acme", "rt", "1.2.3", ["Widget"], ["acmeLib-rt"], vendor="Acme"))

TREES = {
    "install/modules/baja-rt.jar": BAJA,
    "install/modules/control-rt.jar": CONTROL,
    "install/modules/control-wb.jar": CONTROL_WB,
    "install/modules/dashboard-wb.jar": DASH_WB,
    "install/modules/kitControl-rt.jar": KIT,
    "install/modules/acme-rt.jar": VENDOR,
    "install/modules/broken.jar": b"PK\x03\x04 not a zip",
    "install/modules/nometa.jar": zipped([("META-INF/MANIFEST.MF", "x")]),
    "install/modules/badxml.jar": zipped([("META-INF/module.xml", "<module name='bad-rt'")]),
    "install/modules/noname.jar": jar("<module moduleName='x'/>"),
    "install/modules/readme.txt": b"not a jar",
    "install/sw/%s/dashboard-rt.jar" % V: jar(module_xml("dashboard-rt", "dashboard", "rt", V, ["Dashboard"])),
    "install/sw/1.0/dashboard-rt.jar": jar(module_xml("dashboard-rt", "dashboard", "rt", "1.0", ["Dashboard"])),
    "install/sw/4.14/kitPid-rt.jar": jar(module_xml("kitPid-rt", "kitPid", "rt", "4.14.0.99", ["Pid"])),
    "install/sw/4.14/kitPid-rt.jar.sig": b"sig",
    "install/sw/stub/acmeLib-rt.jar": b"no manifest",
    "install-ok/modules/baja-rt.jar": BAJA,
    "install-ok/modules/control-rt.jar": CONTROL,
    "install-exists/modules/baja-rt.jar": BAJA,
    "install-exists/modules/kitControl-rt.jar": KIT,
    "install-exists/modules/kitPid-rt.jar": b"a corrupt copy already in modules/",
    "install-exists/sw/4.14/kitPid-rt.jar": jar(module_xml("kitPid-rt", "kitPid", "rt", "4.14.0.99", ["Pid"])),
    "install-nostaged/modules/baja-rt.jar": BAJA,
    "install-nostaged/modules/kitControl-rt.jar": KIT,
    "install-copy/modules/baja-rt.jar": BAJA,
    "install-copy/modules/kitControl-rt.jar": KIT,
    "install-copy/sw/%s/kitPid-rt.jar" % V: jar(module_xml("kitPid-rt", "kitPid", "rt", V, ["Pid"])),
    "no-modules/sw/x.jar": BAJA,
}

STATION = ("<?xml version='1.0' encoding='UTF-8'?>\n<bajaObjectGraph version='4.0'>\n"
           "<p h='1' m='b=baja c=control' t='b:Station'>\n"
           " <p n='Services' h='2' t='b:ServiceContainer'/>\n"
           " <p n='W' h='3' t='c:NumericWritable'/>\n"
           " <p n='D' h='4' m=\"dsb=dashboard\" t=\"dsb:Dashboard\"/>\n"
           " <p n='V' h='5' t='dsb:DashboardView'/>\n"
           " <p n='R' h='6' m='CRP=ColdRoomPan h=history' t='CRP:ColdRoom'/>\n"
           " <p n='L' h='7' m='kc=kitControl' t='kc:LoopPoint'/>\n"
           " <p n='X' h='8' t='zz:Unknown'/>\n"
           "</p>\n</bajaObjectGraph>\n")
OK = ("<?xml version='1.0'?><bajaObjectGraph version='4.0'><p h='1' m='b=baja c=control' t='b:Station'>"
      "<p n='W' h='2' t='c:BooleanWritable'/></p></bajaObjectGraph>")
KITSTATION = ("<?xml version='1.0'?><bajaObjectGraph version='4.0'><p h='1' m='b=baja kc=kitControl' t='b:Station'>"
              "<p n='L' h='2' t='kc:LoopPoint'/></p></bajaObjectGraph>")

FILES = {"station.xml": STATION, "ok.xml": OK, "kit.xml": KITSTATION, "empty.xml": "", "text.xml": "plain text"}

CASES = [
    ("help", ["--help"]), ("check-help", ["check", "--help"]), ("doctor-help", ["doctor", "--help"]),
    ("no-command", []), ("bad-command", ["fix"]), ("check-no-target", ["check", "--install", "install"]),
    ("check", ["check", "station.xml", "--install", "install"]),
    ("check-json", ["check", "station.xml", "--install", "install", "--json"]),
    ("check-bog", ["check", "config.bog", "--install", "install"]),
    ("check-dist-json", ["check", "backup.dist", "--install", "install", "--json"]),
    ("check-ok", ["check", "ok.xml", "--install", "install-ok"]),
    ("check-ok-json", ["check", "ok.xml", "--install", "install-ok", "--json"]),
    ("check-exists-fix", ["check", "kit.xml", "--install", "install-exists", "--fix"]),
    ("check-exists-fix-json", ["check", "kit.xml", "--install", "install-exists", "--fix", "--json"]),
    ("check-nostaged", ["check", "kit.xml", "--install", "install-nostaged"]),
    ("check-missing-station", ["check", "nope.bog", "--install", "install"]),
    ("check-empty-station", ["check", "empty.xml", "--install", "install"]),
    ("check-text-station", ["check", "text.xml", "--install", "install"]),
    ("check-missing-install", ["check", "station.xml", "--install", "nope"]),
    ("check-no-modules", ["check", "station.xml", "--install", "no-modules"]),
    ("doctor", ["doctor", "--install", "install"]), ("doctor-json", ["doctor", "--install", "install", "--json"]),
    ("doctor-ok", ["doctor", "--install", "install-ok"]),
    ("doctor-ok-json", ["doctor", "--install", "install-ok", "--json"]),
    ("doctor-exists-fix", ["doctor", "--install", "install-exists", "--fix"]),
    ("doctor-trailing-slash", ["doctor", "--install", "install-ok/"]),
    ("doctor-missing-install", ["doctor", "--install", "nope"]),
]
COPY = ["check", "kit.xml", "--install", "install-copy", "--fix"]
ENV = dict(os.environ, COLUMNS="100", NO_COLOR="1")
# Cases whose output holds os.path.join paths as Linux CPython prints them (Windows CPython prints
# backslashes): requires.goos binds them to Linux (docs/goldens.md).
LINUX_PATHS = {"check", "check-json", "check-bog", "check-dist-json", "check-exists-fix", "check-exists-fix-json",
               "doctor", "doctor-json", "doctor-exists-fix"}


def run(args, cwd):
    return subprocess.run([sys.executable, "-I", "-c", FROZEN, TOOL, *args], cwd=cwd, env=ENV, capture_output=True, timeout=120)


def store(d, args, res):
    os.makedirs(d)
    with open(os.path.join(d, "cmd.json"), "w") as fh:
        spec = {"args": args, "cwd_fixture": "fixtures"}
        if os.path.basename(d) in LINUX_PATHS:
            spec["requires"] = {"goos": ["linux"]}
        json.dump(spec, fh)
        fh.write("\n")
    for part, data in (("stdout", res.stdout), ("stderr", res.stderr), ("exit", b"%d\n" % res.returncode)):
        with open(os.path.join(d, part), "wb") as fh:
            fh.write(data)


def main():
    if LINUX_PATHS - {case for case, _ in CASES}:
        sys.exit(f"LINUX_PATHS names no case: {sorted(LINUX_PATHS - {case for case, _ in CASES})}")
    shutil.rmtree(FIX, ignore_errors=True)
    for rel, data in {**TREES, **FILES}.items():
        p = os.path.join(FIX, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "wb") as fh:
            fh.write(data.encode("utf-8") if isinstance(data, str) else data)
    bog = zipped([("file.xml", STATION)])
    with open(os.path.join(FIX, "config.bog"), "wb") as fh:
        fh.write(bog)
    with open(os.path.join(FIX, "backup.dist"), "wb") as fh:
        fh.write(zipped([("stations/S/config.bog", bog)]))
    for d in ("golden", "copy"):
        shutil.rmtree(os.path.join(HERE, d), ignore_errors=True)
    for case, args in CASES:
        store(os.path.join(HERE, "golden", case), args, run(args, FIX))
    with tempfile.TemporaryDirectory() as tmp:
        shutil.copytree(os.path.join(FIX, "install-copy"), os.path.join(tmp, "install-copy"))
        shutil.copy(os.path.join(FIX, "kit.xml"), tmp)
        store(os.path.join(HERE, "copy"), COPY, run(COPY, tmp))


if __name__ == "__main__":
    main()
