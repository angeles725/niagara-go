"""Print str(OSError) for Windows error codes as CPython raises them, for winerror_windows_test.go.

Usage: <Windows python.exe> -I pyerr.py OUT.json CODE... ; writes UTF-8 JSON:
  "codes":     {code: str(e)} for the OSError PyErr_SetFromWindowsErr(code) raises (the C path
               the socket module and the os functions use; ctypes.FormatError/WinError keep
               the trailing dot and are not that path)
  "refused":   str(e) of socket.create_connection(("127.0.0.1", 1))
  "addrinuse": str(e) of binding a second socket to a bound 127.0.0.1 port
"""
import ctypes
import json
import socket
import sys

if sys.platform != "win32":
    sys.exit("pyerr.py: run it with a Windows Python")

set_from = ctypes.pythonapi.PyErr_SetFromWindowsErr
set_from.argtypes = [ctypes.c_int]
set_from.restype = ctypes.py_object


def raised(f):
    try:
        f()
    except OSError as e:
        return str(e)
    return None


out = {"codes": {}}
for arg in sys.argv[2:]:
    code = int(arg)
    out["codes"][arg] = raised(lambda: set_from(ctypes.c_int32(code & 0xFFFFFFFF).value))
out["refused"] = raised(lambda: socket.create_connection(("127.0.0.1", 1), timeout=10))
with socket.socket() as first:
    first.bind(("127.0.0.1", 0))
    out["addrinuse"] = raised(lambda: socket.socket().bind(first.getsockname()))
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(out, f, ensure_ascii=False, indent=1)
