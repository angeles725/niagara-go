"""Regenerate terms.json: class_rank's term rules on sample identifiers and questions.

    python3 -I internal/nav/lib/classrank/testdata/gen_terms.py [<module-navigator>/tools]

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
from module_nav_lib import class_rank  # noqa: E402

TEXTS = [
    "BWebServlet", "BACnetNetworkTool", "BAckState", "HTTPServerX", "XMLHttpRequest", "a", "Ab",
    "AB", "Bx", "B", "MAX_VALUE", "__init__", "javax.baja.alarm", "alarm-rt", "BIAlarmSource",
    "IPv6Address", "Utf8Decoder", "value2String", "BÄrger", "configuración", "ÉtatService",
    "histeresis", "chasis", "class", "glass", "alarms", "Address", "the", "TheAndOf", "x9y",
    "BWebServletWebServlet", "ﬁleStore", "ß", "über-Driver", "_x_", "a_b", "Ⅻ", "١٢٣", "",
]

QUESTIONS = [
    "como funciona el ack de alarmas", "que es un BWebServlet", "zzzqqq xxyyzz",
    "who calls ackAlarm", "configuración de la estación", "Histéresis del termostato",
    "the a of", "", "BAlarmService and HistoryService", "chasis", "alarmas alarmas alarm",
    "reconocimiento de alarmas", "¿Cómo se guardan los históricos?", "IPv6 BACnet/IP",
]

out = {
    "texts": [[t, list(class_rank.identifier_terms(t)), list(class_rank._text_terms(t)),
               class_rank._field_counts(t), class_rank._part_count(t), class_rank._squash(t)]
              for t in TEXTS],
    "questions": [[q, class_rank.query_terms(q)] for q in QUESTIONS],
}
with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "terms.json"), "w",
          encoding="utf-8") as f:
    f.write("{\n")
    for i, key in enumerate(("texts", "questions")):
        f.write('"{}": [\n'.format(key))
        f.write(",\n".join(json.dumps(row, ensure_ascii=False) for row in out[key]))
        f.write("\n]{}\n".format("," if i == 0 else ""))
    f.write("}\n")
