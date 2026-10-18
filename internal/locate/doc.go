// Package locate finds the checkout a tool works on (decision D44; owner
// Integrante1): the one walk-up the checkout-discovery copies of help,
// nav/core, slotomatic, course and searchdoor each wrote for themselves.
//
// A Python tool found its checkout from its own __file__; a Go binary has no
// source path, so each port searches for it (decision D24). The tools do not
// agree on where to look first, which variables name the checkout, or which
// file marks it, and those differences are deliberate (each keeps its Python
// tool's behaviour), so Find imposes none of them. The caller passes:
//
//	Env     variables naming the checkout, first non-empty wins. Its value is
//	        the answer as given: not made absolute, not checked against Marker
//	        (a caller that rejects a wrong value calls FirstEnv itself, checks it,
//	        and leaves Env empty).
//	Starts  where to search, in order: FromCwd and FromExecutable (the
//	        executable's directory, symlinks resolved; the path as given when it
//	        cannot be resolved). A start that cannot be read (Getwd or Executable
//	        fails) is skipped.
//	Marker  what identifies the checkout (HasFile, HasDir, Exists, All, or the
//	        caller's own predicate). Each start is walked up, itself first, to the
//	        file-system root; the first directory the marker accepts is the answer.
//
// Getenv, Getwd and Executable default to the process (os.Getenv, os.Getwd,
// os.Executable); tests inject them, so a test needs neither os.Setenv nor
// t.Chdir and can run in parallel. Find never prints and never fails: a
// checkout that is not found is ("", false) and the caller decides the message
// or the fallback.
package locate
