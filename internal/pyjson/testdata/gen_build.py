"""Writes build_cases.json: the json.dumps output build_test.go compares FromAny + Dumps with.

Run from internal/pyjson: python3 -I testdata/gen_build.py > testdata/build_cases.json
Each case maps to [indent2, plain, canonical, unicode, indent0]:
json.dumps(v, indent=2), json.dumps(v), json.dumps(v, sort_keys=True,
separators=(",", ":")), json.dumps(v, ensure_ascii=False), json.dumps(v, indent=0).
The Go side builds the same values from Go data (see build_test.go).
"""
import json
import sys

CASES = {
    "scalars": [None, True, False, 0, -7, 12345678901234567890123, 0, 1.5, -0.0, 1e-7,
                1e22, float("inf"), float("-inf"), float("nan"), 2.5e-5,
                123456789012345678.0, 0.1, float("-inf"), None],
    "nested ordered dict": {"z": 1, "a": {"y": [1, "x", {"k": None}]}, "empty": {},
                            "list": [], "strs": ["A", "B"]},
    "repeated key keeps first position, last value": dict([("a", 1), ("b", 2), ("a", 3)]),
    "string escapes": "q\" b\\ \n\r\t\b\f \x01 \x7f ~ é — \U0001F600",
    "undecodable bytes read as U+FFFD": {"k�": "�abc���"},
    "dict and nil dict": [{"b": 1, "a": [True, "s"]}, {}],
    "value passes through with its lone surrogate": ["\ud800", 3],
}


def main():
    out = {}
    for name, v in CASES.items():
        out[name] = [
            json.dumps(v, indent=2),
            json.dumps(v),
            json.dumps(v, sort_keys=True, separators=(",", ":")),
            json.dumps(v, ensure_ascii=False),
            json.dumps(v, indent=0),
        ]
    json.dump(out, sys.stdout, indent=1, ensure_ascii=True)
    sys.stdout.write("\n")


main()
