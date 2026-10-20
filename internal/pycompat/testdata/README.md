# pycompat testdata

The goldens and fixtures of this package (unicode.txt, errno_linux.txt, errno_windows.txt, fixtures.json, golden/) are frozen CPython output: their generators gen.py and gen_windows.py retired with the Python reference (D47), so they are regenerated from Go only; winerror/ holds the CPython 3.13 texts (Windows UI language 0x080A) the Windows tests compare with, frozen once from the retired pyerr.py and pyoserr.py.
