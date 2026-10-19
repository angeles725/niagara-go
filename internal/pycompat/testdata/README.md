# pycompat testdata

The goldens and fixtures of this package (unicode.txt, errno_linux.txt, errno_windows.txt, fixtures.json, golden/) are frozen CPython output: their generators gen.py and gen_windows.py retired with the Python reference (D47), so they are regenerated from Go only; winerror/ keeps the two scripts the Windows tests run under PYCOMPAT_WINPY.
