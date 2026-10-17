#!/usr/bin/env python3
"""Fake module-navigator/tools/module_nav.py: the commands the nsearch front door runs.

  token <W> -n N            TOKEN line + 'Class line module' rows for a known identifier; a miss exits 1
                            (BSlowToken sleeps past the CODE budget)
  rank -n N -- <query>      'Rank ...' header + tab rows; rankdown exits 2 (the DEGRADED grep path), zz.. is a miss
  grep <rx> -n N            '[module]' groups of 'Class:line text'; grepdown exits 2, zz.. is a miss
  hierarchy <C> --depth D   a class tree; BNope is not found, BBig has 65 children, BSlowTree sleeps
  implementors <I> -n N     'CLASS MODULE PACKAGE' table; BNope is not found
  drivers [--help] [<mod>]  a driver table
  modules                   the module table (baja, bacnet-rt, alarm-rt, control-rt)
  license-features --module <m> / license-feature <f>
FAKE_MODULE_NAV_OLD=1 plays an older module_nav: no rank, no drivers, no modules (exit 2), and a
not-found class exits 0.
"""
import os
import re
import sys
import time

OLD = os.environ.get("FAKE_MODULE_NAV_OLD") == "1"
TOKENS = {
    "BStatusNumeric": [("BStatusNumeric", 12, "baja"), ("BStatusNumeric", 40, "docSource-doc"),
                       ("BNumericPoint", 7, "control-rt"), ("BNumericPoint", 9, "control-rt"),
                       ("BNumericPoint", 11, "bacnet-rt"), ("BNumericPoint", 15, "alarm-rt"),
                       ("BNumericPoint", 18, "modbus-rt"), ("BAlarmCounter", 3, "alarm-rt")],
    "BDeviceNetwork": [("BDeviceNetwork", 20, "driver-rt"), ("BBacnetNetwork", 5, "bacnet-rt")],
    "getFooBar": [("BFooUser", 31, "foo-rt")],
}
MODULES = [("baja", "rt", "1,234", "567"), ("bacnet-rt", "rt", "880", "301"),
           ("alarm-rt", "rt", "210", "88"), ("control-rt", "rt", "95", "40")]


def miss(text):
    print(text)
    sys.exit(1)


def fail(text):
    print(f"module_nav: {text}", file=sys.stderr)
    sys.exit(2)


def cls(word):
    return "B" + re.sub(r"\W", "", word).title().replace("_", "")


def opt(args, name, default):
    return args[args.index(name) + 1] if name in args else default


args = sys.argv[1:]
cmd, rest = (args[0], args[1:]) if args else ("", [])
if cmd == "token":
    word = rest[0]
    if word == "BSlowToken":
        time.sleep(10)
    rows = TOKENS.get(word)
    if not rows:
        miss(f"0 matches for token {word!r} in the token index")
    print(f"TOKEN: {word!r} ({len(rows)} occurrences in {len({r[0] for r in rows})} classes)")
    for name, line, module in rows:
        print(f"  {name}  {line}  {module}")
elif cmd == "rank":
    query = rest[rest.index("--") + 1]
    words = query.lower().split()
    if OLD:
        fail("argument command: invalid choice: 'rank'")
    if "rankdown" in words:
        fail("no class index (run module_nav index first)")
    if any(w.startswith("zz") for w in words):
        miss(f"0 matches for rank {query!r} (searched 120 classes)")
    names = [cls(w) + suffix for w in words for suffix in ("Service", "Ext")]
    if "BStatusNumeric" in query.split():
        names.insert(1, "BStatusNumeric")
    print(f"Rank {query!r}: {len(names)} matching classes (terms: {', '.join(words)})")
    for i, name in enumerate(names[:int(opt(rest, '-n', '20'))], 1):
        print(f"{i}\t{9.5 - i / 4:.3f}\t{name}\tdemo-rt\tcom/demo/{name}.java")
elif cmd == "grep":
    rx = rest[0]
    terms = [re.sub(r"\\(.)", r"\1", t) for t in rx.removeprefix("(?i)").split("|")]
    if "grepdown" in terms:
        fail("grep index unreadable")
    if any(t.startswith("zz") for t in terms):
        miss(f"0 matches for /{rx}/ in 40 .java files")
    print(f"Grep /{rx}/: matching lines in 40 .java files")
    for module in ("alarm-rt", "history-rt"):
        print(f"  [{module}]")
        for j, term in enumerate(terms):
            print(f"    com.demo.{cls(term)}Helper:{10 + j} uses {term} here")
            print(f"    com.demo.BShared:{30 + j} shared {term} and {terms[0]}")
elif cmd == "hierarchy":
    root = rest[0]
    if root == "BSlowTree":
        time.sleep(10)
    if root == "BNope":
        print("Loading class index ...")
        print(f"Class {root!r} not found.")
        sys.exit(0 if OLD else 1)
    kids = [f"BChild{i:02d}" for i in range(65)] if root == "BBig" else ["BChildA", "BChildB"]
    print(f"Hierarchy: {root} ({len(kids)} direct children, {len(kids) + 5} total descendants)")
    print(f"Depth limit: {opt(rest, '--depth', '3')}")
    print(f"{root} (driver-rt)")
    for k in kids:
        print(f"|-- {k} (demo-rt)")
    print("|   `-- BGrandChild (demo-rt)")
    print("    ... 4 descendants")
elif cmd == "implementors":
    iface = rest[0]
    if iface == "BNope":
        miss(f"Interface {iface!r} not found.")
    print(f"Implementors of {iface}: 3 classes")
    print("CLASS            MODULE     PACKAGE")
    print("-" * 40)
    for name, module in (("BPollScheduler", "driver-rt"), ("BBacnetPoller", "bacnet-rt"), ("BModbusPoller", "modbus-rt")):
        print(f"  {name}  {module}  com.demo.{module.split('-')[0]}")
elif cmd == "drivers":
    if OLD:
        fail("argument command: invalid choice: 'drivers'")
    if "--help" in rest:
        print("usage: module_nav drivers [module]")
        sys.exit(0)
    print(f"Drivers ({rest[0] if rest else 'all modules'}):")
    print("  bacnet-rt   BBacnetNetwork  BBacnetDevice  BBacnetPointDeviceExt")
    print("  modbus-rt   BModbusNetwork  BModbusDevice  BModbusPointDeviceExt")
elif cmd == "modules":
    if OLD:
        fail("argument command: invalid choice: 'modules'")
    print("NAME        TYPE  JAVA   CLASSES")
    for row in MODULES:
        print("  {}  {}  {}  {}".format(*row))
elif cmd == "license-features":
    module = opt(rest, "--module", "")
    if module not in ("bacnet-rt", "baja"):
        miss(f"0 matches: module {module!r} checks no license feature")
    print(f"License features required by {module}: 2")
    print("FEATURE  CHECKS")
    print("  bacnet  4")
    print("  mstp    1")
elif cmd == "license-feature":
    feature = rest[0]
    if feature not in ("bacnet", "mstp"):
        miss(f"0 matches for license feature {feature!r}")
    print(f"Call sites of license feature {feature!r}: 2")
    print("CLASS           MODULE     CHECK")
    print("-" * 40)
    print("  BBacnetNetwork  bacnet-rt  checkFeature")
    print("  BMstpNetwork    bacnet-rt  getFeature")
    print("Try: module_nav license-features --module bacnet-rt")
else:
    fail(f"unknown command {cmd!r}")
