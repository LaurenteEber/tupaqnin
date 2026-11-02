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
| T1 | S1 · 4.1 | delegated (context backstop); verifier: items 1, 3, 4 | done | feat: slice-4.1 |
| T5 | S5 · 4.5 | delegated (context backstop) | pending | |
| T4 | S4 · 4.4 | delegated (context backstop) | pending | |
| T2 | S2 · 4.2 | delegated (context backstop) | pending | |
| T3 | S3 · 4.3 | delegated (context backstop) | pending | |
| T6 | S6 · 4.6 | delegated (context backstop) | pending | |

## Log

- **L1 (1-nov, Eber):** the request is quoted in full in `odd/tasks/slice-entrega.md` L1; this document is its item 2: "Slice 4: docs/construccion/backlog-r4.md v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6 (4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided)."
- **L2 (T1, 4.1):** writer delivered the piece; parent applied its reviewed patch to `internal/demo/walkthrough`, outside its surface, where the tests pinned the rejections B-73 replaces. Verifier on the real binary: 12 of 13 «Listo cuando» passed and 25 races ended consistently; 3 blockers fixed in one correction batch, RED then GREEN: the new-step notice was lost when accepting during a running step (now the proposal's own line in «lo que cambió», via `delivery`); the web dropped `turn_id`, so the new step ran unseen; the displaced contrast had no decisions (B-74). Open for Eber: rule 2 and the checker skip accepted text, as the row declares, so the same text gets `text_without_decision` as a step but not when accepted. Advisories: discarding a displaced proposal leaves the decision's `used_in` and its `open_card`, so the card blocks signing your text; an accepted figure comes back linked to a superseded decision; after an accepted `replace_all` a second displacement has empty triage and note; `ProposalLine` and the signoff list show only the first text; closed section proposals are never shown. For 4.2: after an undo, B-79 says the displaced proposal can be accepted and the code answers `section_changed`.
