"""Regenerate suggest.json: suggest.close_names on sample names and queries.

    python3 -I internal/nav/core/testdata/gen_suggest.py [<module-navigator>/tools]
"""

import json
import os
import sys

TOOLS = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser(
    "~/niagara-research/module-navigator/tools")
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
