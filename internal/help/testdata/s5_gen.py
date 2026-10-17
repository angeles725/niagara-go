"""Regenerate the s5 fixture outputs from the Python niagara-help.

Run: python3 -I internal/help/testdata/s5_gen.py   (NIAGARA_RESEARCH overrides the checkout)

s5/ is a niagara-help checkout for the commands ported after S6 (source, method, freshness,
class --jdk, selftest --recall): a javadoc HTML tree, .bajadoc XML under devguide/, guides/ and
jdk/ (one unparseable file, one needing the &#x0; sanitizer), Java sources (CRLF, invalid UTF-8,
trailing whitespace), guide and devguide pages, a stamped manifest and a fake install with doc
jars. For each case the script copies s5/ to a fresh temporary folder (method makes Python build
a full-text index there), runs tools/niagara_help.py on each case with that copy as the working directory and
writes s5-cli/<case>.txt (stdout, <ROOT> for the checkout) and s5-cli/cases.json (arguments,
exit status, stderr; null stderr where Python reports building its full-text index, which the
Go port never builds).
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
NR = os.environ.get("NIAGARA_RESEARCH", "/home/cristian/niagara-research")
TOOLS = os.path.join(NR, "niagara-help", "tools")
ROOT = "<ROOT>"

CLI = [
    ("source", ["source", "BAlarmService", "--code"]),
    ("source-fold", ["source", "bfoo"]),
    ("source-crlf", ["source", "BCrlf", "--code"]),
    ("source-missing-file", ["source", "BMissing", "--code"]),
    ("source-devguide-rc", ["source", "Example"]),
    ("source-glob", ["source", "BUser*"]),
    ("source-glob-fold", ["source", "buser0*"]),
    ("source-miss", ["source", "BZzz"]),
    ("method", ["method", "ackAlarm"]),
    ("method-fold", ["method", "ACKALARM"]),
    ("method-glob", ["method", "*a*"]),
    ("method-glob-fold", ["method", "ACK*"]),
    ("method-section-miss", ["method", "make", "--section", "fields"]),
    ("method-miss", ["method", "zzqq"]),
    ("freshness", ["freshness"]),
    ("freshness-all", ["freshness", "--all"]),
    ("class-jdk", ["class", "String", "--jdk"]),
    ("class-jdk-sanitized", ["class", "Bad", "--jdk"]),
    ("class-jdk-interface", ["class", "Other", "--jdk"]),
    ("class-jdk-not-in-file", ["class", "Ghost", "--jdk"]),
    ("selftest-recall", ["selftest", "--recall", "--no-samples"]),
    # fuzzy is retired in Go; selftest --recall samples it, so its output
    # is recorded here and checked through the internal function.
    ("fuzzy-alarm", ["fuzzy", "alarm"]),
    ("fuzzy-top1", ["fuzzy", "ack rec", "-n", "1"]),
    ("fuzzy-typo", ["fuzzy", "alarmrecrod"]),
    ("fuzzy-partial", ["fuzzy", "xxalarxx"]),
    ("fuzzy-miss", ["fuzzy", "zzqq"]),
    ("fuzzy-no-word", ["fuzzy", "a b"]),
]


def main():
    out = os.path.join(HERE, "s5-cli")
    os.makedirs(out, exist_ok=True)
    cases = []
    for name, args in CLI:
        # A fresh copy per case, so no case sees an index an earlier one built.
        with tempfile.TemporaryDirectory() as tmp:
            checkout = os.path.join(tmp, "checkout")
            shutil.copytree(os.path.join(HERE, "s5"), checkout)
            env = {"PATH": os.environ.get("PATH", ""), "COLUMNS": "100", "NO_COLOR": "1",
                   "PYTHONHASHSEED": "0", "NIAGARA_HELP_DIR": checkout,
                   "XDG_CACHE_HOME": os.path.join(tmp, "cache")}
            argv = ["--niagara-version", "4.14.0.162"] + args
            p = subprocess.run([sys.executable, "-I", os.path.join(TOOLS, "niagara_help.py")] + argv,
                               cwd=checkout, env=env, capture_output=True, text=True)
            with open(os.path.join(out, name + ".txt"), "w", encoding="utf-8") as f:
                f.write(p.stdout.replace(checkout, ROOT))
            stderr = p.stderr.replace(checkout, ROOT)
            cases.append({"name": name, "args": argv, "exit": p.returncode,
                          "stderr": None if "full-text" in stderr else stderr})
    with open(os.path.join(out, "cases.json"), "w", encoding="utf-8") as f:
        f.write("[\n" + ",\n".join(json.dumps(c) for c in cases) + "\n]\n")


if __name__ == "__main__":
    main()
