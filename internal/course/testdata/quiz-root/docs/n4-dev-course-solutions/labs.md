# Fixture — the quiz pool: every kind, and the items it leaves out

> **Conventions.** `LI:n` = line `n` of `Lab Instructions.txt`.

---

## Lab 1 — Investigating ORDs (`LI:1-12`)

- **L1.1** · Read the log (`LI:2-3`) · The console shows it. *Why the console?* It keeps every station
  message, even after Workbench closes.
- **L1.2** · My Host = `local:` (`LI:4`) · One query, empty body = this machine.
- **L1.3** · Alarm = `local:|alarm:` (`LI:5`).
- **L1.4** · *Observe* two jars (`LI:6`) · *(predicted)* `a-rt.jar` and `a-wb.jar`, one per module part.
- **L1.5** · *Observe* the palette opens (`LI:7`).
- **L1.6** · *Observe* the banner (`LI:8`) · as stated.
- **L1.7** · *Observe* the console · The console prints one line per started service.
- **L1.8** · *Why no thread?* The engine already runs it on its own thread.
- **L1.9** · *Why sign it?* (`LI:9`) · *(unverified — not compiled)* The module asks for a privileged
  permission group.
- **L1.10** · *Observe* the station log (`LI:10`) · *(predicted)* one start-up line per service.
  *(verified live 2026-10-18)* evidence: `C1.1 found 3`
