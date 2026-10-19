package checkcoverage

// helpText is check-coverage.py __doc__, printed by -h/--help and when no name is given.
const helpText = `check-coverage.py — the prior-coverage GATE for opening a new research focus.

Before a new focus (frontier-auto-opened or operator-opened) declares scope or seeds a
single gap, it MUST run this on the target's real MODULE / PACKAGE names — NOT on the
abstract topic, and NEVER trusting the blocks recalled from the parent terminal.

For each name it scans every corpus block file found by the shared discovery rule
(niagara-go internal/blockfiles: the main corpus incl. the consolidated B1-3 file and suffixed
bloque<N>-<topic>.md names, the reflow *-block<N>.md sub-corpus,
examinacion-optimizer-4.13/os413-bloque<N>.md and notes/bloque<N>-*.md; organized/, _backups/,
tools/ and dot-dirs are skipped) and reports:
  - SUBJECT  : the name appears in the block's H1 title  -> that block MAPS it. Strong signal.
  - body     : the name appears only in the block body   -> mentioned; read before scoping.
  - (clear)  : no hits                                   -> genuinely unmapped.

Exit code:
  0  every name is clear (no prior coverage)             -> safe to open as net-new.
  1  usage error.
  3  zero block files found: the gate could not run, so "clear" would be a lie.
  2  at least one name has SUBJECT or body coverage      -> DO NOT claim "net-new" blindly;
     read the cited blocks, and in the bootstrap declare per module: net-new /
     deepening of B<n> (cite it) / already covered -> out of scope.

This is the executable form of retros/2026-09-14-frontier-bootstrap-breadth-check.md and
the FOCUSES.md bootstrap gate. It exists because the spyder-drivers focus (2026-09-14)
cited only B77/B96 and re-derived B106/B117 (+ B242/B105) that a search would have found.

Usage:
  python3 tools/check-coverage.py honeywellSpyderTool XL10NextGen honIrmConfig
  python3 tools/check-coverage.py --json honeywellVenomTools
A niagara-go binary (tools/check-coverage.py and ncov exec it); run from the repo root
(or anywhere — paths resolve to the niagara-research checkout).
`
