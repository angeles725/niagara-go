# Fixture — review assignments

> **Status of every code listing: unverified — not compiled.**

## 1. Review Assignment 1 — BWithId

### 1.1 Requirements (from `sources/n4-dev-course/text/Review Assignment 1.txt`)

| ID | Requirement | Source line |
|---|---|---|
| RA1.M1 | Folder `Review` | :2-3 |
| RA1.M2 | Module name `<First>_<Last>_Review` | :3 |
| RA1.R1 | `BWithId extends BComponent` | :4 |
| RA1.R3a | `id` flags `SUMMARY \| READONLY` | :5, :7 |
| RA1.V9–V11 | Station steps | :8-9 |

### 1.2 Design

| # | Decision | Why (rule and evidence) |
|---|---|---|
| D1.1 | Runtime only | Lab 1 shows it. |

### 1.3 Source — unverified, not compiled

```properties
# a comment, not a heading
## not a heading either
```

### 1.4 Traceability

| Requirement | Satisfied by | Verified by |
|---|---|---|
| RA1.M1–M2 | wizard output | build (§3) |
| RA1.R1 | `extends BComponent` | compiles |
| RA1.V9–V11 | station steps | §3 |
| (robustness) | null guard | a test |

### 1.5 Known traps

| Trap | What goes wrong | This solution |
|---|---|---|
| T1.1 TRANSIENT on `id` | Id is not saved | `id` is persisted |

## 2. Review Assignment 2 — BSummary

### 2.1 Requirements (from `sources/n4-dev-course/text/Review Assignment 2.txt`)

| ID | Requirement | Source line |
|---|---|---|
| RA2.R1 | Add `control-rt` | :2 |

## 3. Needs a run (requires execution)

| ID | What is unconfirmed | Cheapest check |
|---|---|---|
| X1 | The listings compile | `gradlew jar` |
