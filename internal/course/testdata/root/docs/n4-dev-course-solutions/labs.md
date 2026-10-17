# Fixture — lab answer key

> **Conventions.** `LI:n` = line `n` of `Lab Instructions.txt`.

---

## Lab 1 — First component (`LI:1-12`)

**Goal.** Build one component.

- **L1.1** · Start the console (`LI:2-3`) · *Why the console?* It keeps the log.
- **L1.2** · Create the home folder · Setup only.
- **L1.3** · Write `BFirst` (`LI:4-6`) · **Identical** to shipped `LAB/BFirst.java:1-9`
  *(unverified — not compiled)*:

  ```java
  public class BFirst extends BComponent
  {
  }
  ```
  The setter error before Slot-o-Matic is expected.
- **L1.4** · *Observe* the jar (`LI:7`) · *(predicted)* one jar per part,
  `first-rt.jar` and `first-wb.jar`.
- **L1.5** · *Why restart?* The registry is rebuilt at start-up (`LI:8-9`, `LI:11-12`).
- **L1.6** · `gradlew jar` (`LI:10`).

**Pitfalls.** Editing generated code.

---

## Lab 2 — Executors (`LI:13-20`)

- **L2.1** · `gradlew jar`, restart · *Why reuse it?* (`LI:13-14`) · Instances are stored by `a · b` type spec.
- **L2.2–L2.4** · Three properties (`LI:15-16`) · Flags `SUMMARY | READONLY`.
- **L2.5** · Write the executors (`LI:17-18`) · Three are **identical** to shipped. One is **Corrected**
  *(unverified — not compiled)*: it names its thread.
- **L2.6** · *Observe* the second post (`LI:19-20`) · *(predicted)* merged.

## Bonus lab — Discovery (`LI:21-24`)

**Customising the manager**

- **LB.1** · Add a column (`LI:21-22`) · *(predicted)* a new column.

## Appendix A — Debugging (`LI:25-26`)

- **LA.1** · Attach the debugger (`LI:25-26`) · `suspend=n` lets it run.

---

## Answers that need a live run

Nothing above was executed: L1.4, L2.5–L2.6 and compiling every corrected block.
