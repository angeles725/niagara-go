# RDD review driver (worker prompt)

Give this file to a fresh worker with REPO and BASE (and the allowed edit surfaces for a correction).
It drives one gentle-ai native review of committed work to a terminal outcome. Lessons from session 1
(2026-10-17) are folded in: BASE check, fixture/code slices, standing consent.

## Goal
Drive one gentle-ai 4.0.0 native review (receipt-driven development) of a committed candidate to a terminal outcome and report it compactly.

## Context
- Repo/worktree: given below as REPO. Candidate: the commits on the current branch since BASE (given below), `--committed-only`.
- Standing user consent: when START returns the consent/v3 envelope, choose the "granted" invocation (the user pre-authorized "Review this change"). Never pick declined.
- Shell is zsh: wrap loops in `bash -c`.
- BASE sanity check first: `git diff --stat BASE HEAD | tail -1` must show only the candidate's own lines. If the branch merged main after BASE, the diff includes already-reviewed main content and START fails with lens_context_budget_exceeded: use the latest main commit merged into the branch as BASE instead (report that you did).
- Fixture/code pairs: the reviewer only sees the slice's changed paths, so a code commit whose fixtures were added in the previous commit gets false "file missing" blockers (and the lineage then stops with corrupted_or_unverifiable_authority). When the candidate's parent is its `test(...)` fixture commit and the two together fit about 1,000 lines, review them as one slice (BASE = the fixture commit's parent).

## Task
1. Run `gentle-ai review assess --cwd REPO --agent claude-code --base-ref BASE --committed-only --json`. If `review_due` is false, report `review_due_reason` and stop.
2. Otherwise run its `next_transition.command` verbatim (preflight STATUS), then follow ONLY the returned `next_transition` each time: `execute` = run the exact operation + ordered argument tokens; `collect` = run each returned `review.capture-result` capture command exactly as returned (concurrently when several reviewer slots are offered: start all, then wait; do not add --input; never write reviewer prompts yourself); then re-run the bound STATUS exactly as returned; `stop` = stop.
3. On `correction_required`: run the provider-issued `review.status` continuation, then apply ONLY the corroborated frozen findings as one bounded correction in REPO (edit files, run `gofmt -l . && go vet ./... && go test ./...`, commit with a Conventional Commit message, NO AI attribution lines), then continue via the returned transitions (capture-correction-plan / targeted validation) exactly as issued.
4. On a final approved capture, run the exact `review.acknowledge-approved` invocation once and capture the `gentle-ai.review-acknowledged/v1` envelope.
5. On any `stop` reason code, report the code and the continuation; do not improvise recovery. If START fails with a dangerous_sink defect, report it (the parent will use an independent verifier).

## Constraints
- Never invent bindings, never edit lifecycle artifacts, never pass --binary, never change checkout. Do not push or merge.
- If the current state differs from this description, stop and report.

## Reply format
Exactly: `outcome: approved|corrected-approved|declined|stopped:<reason_code>|due:false:<reason>`; `risk:`; `lenses:` list; `findings:` one line each (severity, file:line, summary, blocking yes/no); `correction commit:` hash or none; `acknowledged:` envelope summary (lineage, target) or none; `follow-ups:` non-blocking advisories. Under 600 tokens. Then a `## Key Learnings` block of 1–5 numbered standalone factual sentences.
