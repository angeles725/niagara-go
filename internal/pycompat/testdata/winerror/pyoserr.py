"""Print str(OSError) and its class for real Windows failures, for winerror_os_windows_test.go.

Usage: <Windows python.exe> -I pyoserr.py OUT.json DIR ; DIR is an existing empty directory the
Go test also uses. Writes UTF-8 JSON {case: {"text": str(e), "class": type(e).__name__}}:
  refused    socket.create_connection(("127.0.0.1", 1))
  addrinuse  binding a second socket to a bound 127.0.0.1 port
  stat       os.stat(DIR\\no\\such)          statfile  os.stat(DIR\\missing.txt)
  listdir    os.listdir(DIR\\no)             listfile  os.listdir(DIR\\file.txt) (a file)
  mkdir      os.mkdir(DIR\\no\\sub)          rename    os.rename(DIR\\missing.txt, DIR\\dst.txt)
  open       open(DIR\\missing.txt) (the C runtime's errno, not a WinError)
"""
import json
import os
import socket
import sys

if sys.platform != "win32":
    sys.exit("pyoserr.py: run it with a Windows Python")

out_path, d = sys.argv[1], sys.argv[2]
p = lambda *parts: os.path.join(d, *parts)
with open(p("file.txt"), "w") as f:
    f.write("x")


def raised(f):
    try:
        f()
    except OSError as e:
        return {"text": str(e), "class": type(e).__name__}
    return None


out = {
    "refused": raised(lambda: socket.create_connection(("127.0.0.1", 1), timeout=10)),
    "stat": raised(lambda: os.stat(p("no", "such"))),
    "statfile": raised(lambda: os.stat(p("missing.txt"))),
    "listdir": raised(lambda: os.listdir(p("no"))),
    "listfile": raised(lambda: os.listdir(p("file.txt"))),
    "mkdir": raised(lambda: os.mkdir(p("no", "sub"))),
    "rename": raised(lambda: os.rename(p("missing.txt"), p("dst.txt"))),
    "open": raised(lambda: open(p("missing.txt"))),
}
with socket.socket() as first:
    first.bind(("127.0.0.1", 0))
    out["addrinuse"] = raised(lambda: socket.socket().bind(first.getsockname()))
os.remove(p("file.txt"))
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(out, f, ensure_ascii=False, indent=1)
