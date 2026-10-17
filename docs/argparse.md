# internal/argparse: Python argparse parity

`internal/argparse` ports the part of Python 3.15 `argparse` the maintained tools use, so a
ported tool prints the same help, usage and errors, with the same exit codes, as the
Python original.

## Feature survey

Scanned with a Python AST walk on 2026-10-17: 75 files in niagara-research `tools/`,
`module-navigator/tools/` and `niagara-help/tools/` (64 of them call `add_argument`).

| Feature | Uses |
|---|---|
| `ArgumentParser(...)` | 38 |
| `description=` / `prog=` / `formatter_class=RawDescriptionHelpFormatter` | 34 / 13 / 11 |
| `parents=` (parser / `add_parser`) | 2 / 26 |
| `usage=`, `epilog=` (plus module-nav assigns `parser.epilog`), `add_help=False` | 1 each |
| `allow_abbrev=False` and an `error()` override exiting 3 (searchdoor `_Parser`) | 1 |
| `add_argument` (long / short / positional names) | 680 (398 / 153 / 211) |
| `help=` / `default=` / `dest=` | 589 / 195 / 150 |
| `action="store_true"` / `"store_const"` / `"append"` | 134 / 2 / 1 |
| custom `argparse.Action` (tools/lib/version.py `VersionAction`, on every tool) | 1 class |
| `type=int` / `float` / custom (`positive_int` x8, `_regex` x4) | 126 / 2 / 12 |
| `choices=` / `required=True` / `metavar=` / `const=` | 16 / 20 / 13 / 3 |
| `nargs="?"` / `"+"` / `"*"` | 23 / 5 / 2 |
| `default=argparse.SUPPRESS` (cliflags subcommand copies) | 1 helper |
| `add_mutually_exclusive_group()` | 5 |
| `add_subparsers` (`dest` / `required=True` / `metavar` / `help`, 1 `SUPPRESS`) | 13 (13 / 8 / 3 / 3) |
| `add_parser` (`help` / `description` / `epilog` / `aliases`) | 210 (197 / 7 / 1 / 1) |
| nested subparsers (niagara-help `versions add/list`) | 1 |
| `set_defaults` / `parse_args` / `parser.error` / `print_help` / `print_usage` | 182 / 43 / 12 / 5 / 1 |

Not used by any consumer: `parse_known_args`, `parse_intermixed_args`,
`add_argument_group`, `RawTextHelpFormatter`, `ArgumentDefaultsHelpFormatter`,
`store_false`, `count`, `extend`, `append_const`, `action="version"`, `nargs=N`,
`%(default)s` in help, `prefix_chars`, `fromfile_prefix_chars`, `conflict_handler`,
`exit_on_error`, `argument_default`, `BooleanOptionalAction`, `FileType`.
The cheap ones among them are implemented anyway (argument groups, the four formatter
classes, every store/append/count/extend/version action, `nargs=N`, help `%(...)s`,
`ParseKnown`) and are covered by the fixtures.

## How parity is proven

`internal/argparse/testdata/matrix.py` declares one parser per feature family plus
replicas of bog-nav, module-nav, niagara-help, corpus-nav, searchdoor and qnx6read, and
the argv cases: help at COLUMNS 100 and 80 (and 14 to 200 for wrapping), success
parses, and every error class (required, invalid choice with and without a suggestion,
invalid type, unrecognized, ambiguous abbreviation, not allowed with, expected N
arguments, ignored explicit argument), `--`, negative numbers, version and help exits.
`gen_fixtures.py` runs each case through Python and writes `fixtures.json`;
`TestFixtures` rebuilds the same parser through the Go API and compares stdout, stderr,
exit status and `repr()` of the namespace.

## Known gaps

| Gap | Consumer affected |
|---|---|
| No colour: Python 3.15 colours help on a TTY without `NO_COLOR`; Go output equals Python's piped output | all, interactive use only |
| Without `$COLUMNS`, the terminal width is read on Linux only (elsewhere 80) | Windows interactive help width |
| `""` means Python `None` for `Prog`, `Usage`, `Description`, `Epilog`, `Help`, `Metavar`, group titles; use `SubparserConfig.Listed` for `add_parser(help="")` | none |
| `int` values beyond int64 are rejected as invalid; `float` accepts ASCII digits only | none |
| The `action.default is namespace value` identity test is approximated by "still the placed default" | none |
| `parse_intermixed_args`, `REMAINDER`, `BooleanOptionalAction`, `fromfile_prefix_chars`, other `prefix_chars`, `conflict_handler="resolve"`, `exit_on_error=False`, `argument_default`, `deprecated` | none |
| Custom `type=` callables are the porter's job (module-find `_regex` compiles a Python regex) | module-find |
