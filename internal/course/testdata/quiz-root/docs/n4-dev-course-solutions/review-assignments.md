# Fixture — review assignments of the quiz pool

## 1. Review Assignment 1 — BWithId

### 1.2 Design

| # | Decision | Why (rule and evidence) |
|---|---|---|
| D1.1 | Runtime only | Lab 1 shows a runtime-only module (`LI:1`). |

### 1.5 Known traps

| Trap | What goes wrong | This solution |
|---|---|---|
| T1.1 TRANSIENT on `id` | Id is not saved | `id` is persisted |
