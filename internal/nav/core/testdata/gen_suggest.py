"""Regenerate suggest.json: suggest.close_names on sample names and queries.

    python3 -I internal/nav/core/testdata/gen_suggest.py [<module-navigator>/tools]

<module-navigator>/tools defaults to the pre-go-retire worktree ($MODULE_NAVIGATOR_PY_REF, internal/pyref).
"""

import json
import os
import subprocess
import sys


def py_ref(repo, env, rev):  # D47: a clean worktree of <repo> tag pre-go-retire (docs/goldens.md)
    root = os.environ.get(env) or os.path.expanduser("~/niagara-research-worktrees/pre-go-retire-" + repo)
    git = lambda *a: subprocess.run(["git", "-C", root, *a], capture_output=True, text=True).stdout.strip()  # noqa: E731
    if git("rev-parse", "HEAD") != rev or git("status", "--porcelain"):
        sys.exit(f"{env}={root}: not a clean worktree of {repo} tag pre-go-retire ({rev[:9]}); see docs/goldens.md")
    return root


os.environ["MODULE_NAV_PYTHON"] = "1"  # a tag shim runs its Python, never Go
TOOLS = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
    py_ref("module-navigator", "MODULE_NAVIGATOR_PY_REF", "f897268e900138e08a82bf1b389a1715c1be26e1"), "tools")
sys.path.insert(0, TOOLS)
from module_nav_lib import suggest  # noqa: E402

NAMES = [
    "BDeviceNetwork", "BDevicenetwork", "bdevicenetwork", "BProxyConversion", "BLinearConversion",
    "ConversionUtils", "BConversionLink", "BConversions", "Conversion", "BReverseConversion",
    "BAlarmService", "BAlarmClass", "BAlarmConsole", "AlarmSupport", "BAlarmArchive",
    "alarm-rt", "alarm-ux", "alarm-wb", "alarm-se", "alarmOrion-rt", "alarm", "alarms-rt",
    "backup-rt", "baja", "bql-rt", "BComponent", "BComponentEvent", "Component", "BCompositeTable",
    "BLinkPad", "BLink", "BLinkCheck", "BWebServlet", "BServlet", "WebServlet", "BHttpServlet",
    "Zeta", "zeta", "ZETA", "ZetaB", "Bx", "BX", "B", "BÉtat", "BEtat", "étatService",
    "driver-rt", "driver-wb", "driver", "BDriverNetwork", "BDeviceFolder", "BDeviceExt",
]

QUERIES = [
    "BConversion", "bdevicenetwork", "alarm", "BAlarmServce", "BCompnent", "Conversion", "zeta",
    "javax.baja.BLinkPad", "alarm-rt.jar", "", "pkg.", "B", "BX", "Bx", "BÉtat", "état",
    "driver", "BServlet", "servlet", "BDevice", "BDeviceNet", "nothinglikeit", "BLinkPads",
]


def main():
    out = [[q, suggest.close_names(q, NAMES)] for q in QUERIES]
    out.append(["BConversion limit 2", suggest.close_names("BConversion", NAMES, limit=2)])
    here = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(here, "suggest.json"), "w", encoding="utf-8") as f:
        f.write('{"names": %s,\n "cases": [\n  ' % json.dumps(NAMES, ensure_ascii=False))
        f.write(",\n  ".join(json.dumps(c, ensure_ascii=False) for c in out))  # one case per line
        f.write("]}\n")


if __name__ == "__main__":
    main()
