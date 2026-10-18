// Package licensador ports tools/licensador/licensador.py: the REF1 signed
// license format (REF1.<b64url canonical JSON>.<b64url RSA-PSS-SHA256>), its
// all-or-nothing verification pipeline, issue/revoke/genkeys/inspect/hostid
// and the A1..A10 selftest. Output is byte-identical to the Python tool under
// Python 3.15 for every deterministic path; PSS signatures are randomised in
// both, so signing parity is proven by cross-verification.
//
// Main parses argv with internal/argparse (prog "licensador", the installed command) and calls the
// Run* functions, which take already-parsed options.
package licensador
