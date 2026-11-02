# Slice 4 · Hard cases

Objective: backlog-r4 v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6, after Entrega (`odd/tasks/slice-entrega.md`). Branch: `main`, one local commit per piece `feat: slice-4.n`, explicit paths, no signature or trailer lines, never push. Runner: `go test -count=1 ./...` · `make vet` · `make web`; each commit gated on their exit codes. A piece that is not green is not committed: that is the switch. Never call Meta, never run fly.

## Specs

Each piece's spec is its row in `docs/construccion/backlog-r4.md` v0.7, read verbatim there. Decisions B-73 to B-86 in the same file, all adopted. 4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided.

- **S1 · 4.1** Propuestas (B-73 to B-77).
- **S5 · 4.5** Umbral y limpieza del contexto (B-86).
- **S4 · 4.4** «Continúa hasta terminar» (B-83, B-84, B-85).
- **S2 · 4.2** Deshacer y edición sospechosa (B-78, B-79, B-80).
- **S3 · 4.3** Objetar y revocar (B-81, B-82).
- **S6 · 4.6** Los casos 7 y 9, y los tramos cortados.

## Tasks

| ID | Spec | Route | Status | Commit |
|---|---|---|---|---|
| T1 | S1 · 4.1 | delegated (context backstop) | pending | |
| T5 | S5 · 4.5 | delegated (context backstop) | pending | |
| T4 | S4 · 4.4 | delegated (context backstop) | pending | |
| T2 | S2 · 4.2 | delegated (context backstop) | pending | |
| T3 | S3 · 4.3 | delegated (context backstop) | pending | |
| T6 | S6 · 4.6 | delegated (context backstop) | pending | |

## Log

- **L1 (1-nov, Eber):** the request is quoted in full in `odd/tasks/slice-entrega.md` L1; this document is its item 2: "Slice 4: docs/construccion/backlog-r4.md v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6 (4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided)."
