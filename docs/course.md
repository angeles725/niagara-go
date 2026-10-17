# course: the N4 developer-course answer keys

`course` looks up the written answers of the N4 Developer Certification course by item ID
and counts them by verification status. It reads three Markdown files of a niagara-research
checkout and one or two course text files. It is new (no Python original), so its golden cases
run over the synthetic tree `internal/course/testdata/root/`.

## Sources

| File (relative to the root) | Items |
|---|---|
| `docs/n4-dev-course-solutions/labs.md` | lab items `L<lab>.<n>`, `LB.<n>` (Bonus lab), `LA.<n>` (Appendix A) |
| `docs/n4-dev-course-solutions/review-assignments.md` | requirements, design decisions, traps, needs-a-run checks |
| `docs/n4-dev-course-solutions/certification-exam.md` | exam problems `P<n>` |
| `sources/n4-dev-course/text/Lab Instructions.txt` | read only by `course text` (lab items, labs) |
| `sources/n4-dev-course/text/Review Assignment <k>.txt` | read only by `course text` (requirements) |

All three Markdown files are required: a missing one is exit 2, never an empty answer.
The text files are needed only by `course text`.

## Root discovery

1. `--root DIR`.
2. `NIAGARA_RESEARCH_DIR`, trimmed (an empty or blank value counts as unset).
3. The first directory at or above the working directory that holds
   `docs/n4-dev-course-solutions/labs.md`.
4. The same walk from the directory of the executable (symlinks resolved).

`--root` or the variable naming a directory without `labs.md` is exit 2 (no fallback to the
walk). Nothing found is exit 2 with one line naming the three options.

## Groups

| Group | Heading | `--lab` / `text` spellings (any case) |
|---|---|---|
| `L1`..`L11` | `## Lab <n> — <title> (`LI:a-b`)` | `3`, `L3` (leading zeros ignored: `03`, `L03`) |
| `LB` | `## Bonus lab — <title> (`LI:a-b`)` | `B`, `LB` |
| `LA` | `## Appendix A — <title> (`LI:a-b`)` | `A`, `LA` |
| `RA1`, `RA2` | `## <n>. Review Assignment <k> — <title>` | `RA1`, `RA2` |
| `RAX` | `## <n>. Needs a run ...` | `RAX` |
| `EXAM` | the whole `certification-exam.md` | `EXAM` |

`--lab RA` selects RA1, RA2 and RAX together. Groups are discovered from the headings, never
from a fixed list. The `LI:a-b` range of a lab heading is the lab's text range.

## labs.md items

An item is a bullet at column 0 inside a lab section:

    - **<ID>** · <step> · <answer>

- ID grammar: `L<lab>.<n>`, `LB.<n>`, `LA.<n>` (`<lab>`, `<n>` decimal), or a range of them
  in one group, `L2.2–L2.4` (en dash or hyphen): one item that `L2.3` also finds. The prefix
  must name the enclosing group (`L2.5` under Lab 1 is an error).
- The item continues over the following lines indented by at least two spaces and over blank
  lines followed by such a line; it ends at the next line at column 0. Continuation lines lose
  two spaces of indent, so code blocks are kept as written.
- The head is the first paragraph (up to a blank line or a code fence), joined with single
  spaces. The step and the answer are split at the **last** ` · ` of the head that is outside
  backticks (one item, L3.3, has `step · question · answer`).
- A head without a separator is either a question and its answer (`*Why ...?* The registry
  ...`): the leading `*...?*` question is the step and the rest is the answer; or a step with no answer
  (`` `gradlew jar` (`LI:474`). ``), reported as `(step only)`.
- The answer is the rest of the head plus every later line of the item.
- `LI:a-b` (or `LI:a`) means lines `a..b` of `Lab Instructions.txt`. The item's ranges are the
  `LI:` references of the step; when the step has none, those of the whole head.
- A bold line at column 0 (`**Customising the Device Manager (automanager)**`) inside a lab is
  a subsection; `**Goal.**` and `**Pitfalls.**` paragraphs are not items.
- `## Answers that need a live run` lists IDs and ranges (`L9.9–L9.13`); each one marks its
  item `live run: needed`. An ID there that is not an item is an error.

## review-assignments.md items

Tables are recognised by their header row; cells are split on `|` except `\|`.

| Header | Kind | ID column | Step | Answer |
|---|---|---|---|---|
| `\| ID \| Requirement \| Source line \|` | requirement | `RA<k>.<X><n>[a-z]`, a range `RA1.V9–V14`, or `RA<k>.S` | requirement | its traceability row |
| `\| # \| Decision \| Why ... \|` | decision | `D<k>.<n>` | decision | why |
| `\| Trap \| What goes wrong \| This solution \|` | trap | first word of the cell, `T<k>.<n>` | rest of the cell | what goes wrong + this solution |
| `\| ID \| What is unconfirmed \| Cheapest check \|` | check | `X<n>` | what is unconfirmed | cheapest check |
| `\| Requirement \| Satisfied by \| Verified by \|` | traceability, joined onto requirements | a requirement ID or range | | |

- Requirements read their text file from the requirements heading
  (``(from `sources/n4-dev-course/text/Review Assignment 1.txt`)``); the `Source line` cell
  (`:2-3`, `:2, :5`) gives the ranges.
- A range ID (`RA1.V9–V14`, en dash or hyphen) is one item; `RA1.V12` finds it. A traceability
  range (`RA1.M1–M6`) applies to every requirement it covers. A traceability row whose ID is
  not a requirement is an error; a parenthesised label cell such as `(robustness)` is skipped, any
  other first cell that is not an `RA` ID is an error.
- A data row with fewer than three cells in a recognised table is an error.
- Headings and tables inside code fences are ignored.

## certification-exam.md items

`## <n>. Problem <m> — <title>` is item `P<m>`: the step is `Problem <m> — <title>`, the
answer is the section body up to the next `## ` heading. Other `## ` sections are not items.

## Status vocabulary

An item carries every marker found in its own text; markers overlap (L1.9 is identical and
unverified). `written` means none of the others.

| Status | Marker |
|---|---|
| `predicted` | `*(predicted)*` |
| `unverified` | `*(unverified — not compiled)*`; a heading containing `unverified`; the `Needs a run` section |
| `corrected` | `**Corrected**` (any case) |
| `identical` | `**Identical**` (any case) |
| `verified` | `*(verified live <YYYY-MM-DD>)*` (an item confirmed on a live station); a file-level `**Status: verified ...**` line (certification-exam.md) |
| `written` | none of the above |

A file-level note such as `Status of every code listing: unverified` describes code listings,
not items, and is not applied to the items.

## Errors (exit 2)

Duplicate IDs (within or across files), malformed IDs in a lab section (`- **L1.x** · ...`),
an ID under the wrong group, unknown IDs in the live-run list or a traceability table, a
reversed or cross-group live-run range (`L2.4–L2.2`), a table row with fewer than three cells
and a traceability row that names no requirement are parse errors. Every error is printed as `course: <file>:<line>: <reason>` and the command
exits 2: an answer key that cannot be read without ambiguity is not searched.

## Commands

- `course show ID [ID ...] [--json]`: kind, ID, section, step, text ranges, status, live run,
  `file:line`, full answer. IDs are case-insensitive. An unknown ID prints
  `no item <ID> (searched N items in <files>)` to stderr; if any ID is unknown the exit is 1,
  even when others were printed.
- `course status [--lab G] [--status S] [--json]`: items per group and status (columns
  overlap), then with `--status` the matching IDs and steps (truncated to 80 characters).
  No match is exit 1 with the scope and counts.
- `course text ID|GROUP`: the text-file lines of the item's (or the lab's) ranges, numbered,
  with no trailing whitespace (a blank line prints only its number).
  No range is exit 1; a missing text file or a range past its end is exit 2. Requirements
  print their `Review Assignment <k>.txt` lines.

## Tests

- `internal/course/testdata/golden/`: hand-written expectations over the fixture roots
  (`root`, `dup-root`, `partial-root`, `norelated`); the help and usage cases were captured
  from the first green build and reviewed.
- `root_test.go`: the discovery order (flag, variable, cwd walk, executable walk) and the
  not-found error, run from a temporary directory with the executable stubbed, so no `labs.md`
  above the real checkout can leak in.
- `oracle_test.go`: the recall oracle (plain regexes over the raw files, its own range
  expansion) must equal the parsed ID set on the fixture, and on the real checkout when
  `NIAGARA_RESEARCH_DIR` is set, where the counts per group and per status are pinned.

Paths are printed relative to the root with `/`, on every OS.
