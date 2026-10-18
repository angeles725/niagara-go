# Fixture — lab answer key for course verify-predictions

> Synthetic: the IDs are the ones the check spec links (checkspec.go Keys); the answers are
> stand-ins, not the real answer key. L4.13 and L11.25 are marked predicted here (not in the
> real key) because their checks observe the whole answer: they exercise CONFIRMED.

---

## Lab 1 — First component (`LI:1-9`)

- **L1.4** · *Observe* the Gradle files (`LI:2`) · *(predicted)* one build file per part.
- **L1.14** · *Observe* the module types (`LI:3`) · *(predicted)* a `<type>` for `FirstComponent`.
- **L1.25** · Invoke Execute (`LI:4`) · *(predicted)* Time Executed set to now.

---

## Lab 3 — Add part 2 (`LI:10-19`)

- **L3.9** · Change the points (`LI:11`) · *(predicted)* `Output` holds the sum.

---

## Lab 4 — Tstat (`LI:20-29`)

- **L4.13** · Set diff to 2 (`LI:21`) · *(predicted)* `diff` = 2.
- **L4.15–L4.16** · Action = Reverse; confirm heating (`LI:22-23`) · *(predicted)* both hold `false` in
  band. A range item: C4.6 reaches it through its covered ID L4.15, whose note makes it PARTIAL.

---

## Lab 10 — Jobs (`LI:30-39`)

- **L10.6–L10.7** · Depend on history-rt and file-rt (`LI:30`) · *(predicted)* both in module.xml. C10.7
  reaches it through two of its keys: one link, listed once.
- **L10.23** · Files: ten CSV files (`LI:31`) · *(verified live 2026-10-15)* already verified: not reported.
- **L10.24** · Open one CSV (`LI:32`) · *(predicted)* the history line, the header, rows. Decided by the
  three file-text checks C10.8-C10.10.

---

## Lab 11 — NDriver (`LI:40-49`)

- **L11.7** · Drop the server network (`LI:41`) · *(predicted)* its palette entry sets `127.0.0.1:23`.
- **L11.25** · Tcp Config `localhost`, port `23` (`LI:42`) · *(predicted)* the driver is set to `localhost:23`.
- **L11.26** · Ping the network (`LI:43`) · *(predicted)* Down = false, Last Ok Time = now.
- **L11.32** · Ping the device (`LI:44`) · *(predicted)* as stated.
