# Slice 2 · Protections

Objective: backlog-r2 v0.7, pieces 2.1 → 2.9, 2.11, 2.12 (stop before 2.10). One local commit per piece `feat: slice-2.N` on `main`, explicit paths, no trailers, never push. Runner: `make test` · `make vet` · `make web`. Route: one bounded writer per piece, in order (context backstop: parent session already long); parent reviews the diff, runs the checks, commits.

## Specs

Each piece's spec is its row in `docs/construccion/backlog-r2.md` v0.7, read verbatim there: «Qué», «Implementa», «Leer», «Interfaz», «Listo cuando», «No hacer», «Si algo no está». Decisions B-36 to B-54 are all answered or inferred (same file). Where each view goes: `docs/diseno/bloque-3/pantallas.md` «Dónde va cada vista de la rebanada 2».

- **S1 · 2.1** Reglas de la app, triaje y vía directa. First step: «sync the embedded copies of salida-principal and salida-entendi; both schemas changed».
- **S2 · 2.2** Lista de cambios, avisos y «escrita con lo anterior».
- **S3 · 2.3** Comprobador y sondas. «Write checker.en.md … as drafts for me to review.» Hand-made fake checker outputs.
- **S4 · 2.11** Procedencia de cada afirmación.
- **S5 · 2.12** Plan acordado antes de escribir.
- **S6 · 2.4** «Por aclarar».
- **S7 · 2.5** Parada con informe.
- **S8 · 2.6** Preparación de la defensa. «Write … judge-defense.en.md as drafts for me to review.» Hand-made fake judge outputs.
- **S9 · 2.7** Reentrada y marco completo.
- **S10 · 2.8** Acciones de H-3.
- **S11 · 2.9** Avisos en el texto.
- **Constraints:** «You never call Meta: build tests with hand-made fake outputs». AGENTS.md rules 1–13; stop only in its 5 cases, and before 2.10.
- **Return per piece:** «what it does (5 lines), files, tests, what you did not do, [EL DISEÑO NO LO DICE], up to 3 questions with contrast».

## Tasks

| ID | Specs | Route | Status | Commit |
|---|---|---|---|---|
| T1 | S1 (2.1) | delegated (context backstop) | done · make test/vet/web green · assess medium, under_budget (no review due) | `8432c14` |
| T2 | S2 (2.2) | delegated (context backstop) | done · make test/vet/web green · migration 0004 only ADD COLUMN | see L5 |
| T3 | S3 (2.3) | delegated | pending | |
| T4 | S4 (2.11) | delegated | pending | |
| T5 | S5 (2.12) | delegated | pending | |
| T6 | S6 (2.4) | delegated | pending | |
| T7 | S7 (2.5) | delegated | pending | |
| T8 | S8 (2.6) | delegated | pending | |
| T9 | S9 (2.7) | delegated | pending | |
| T10 | S10 (2.8) | delegated | pending | |
| T11 | S11 (2.9) | delegated | pending | |

## Log

- **L1 (30-oct, Eber, verbatim):** «Start slice 2. Read docs/construccion/backlog-r2.md (v0.7) whole: its decisions B-36 to B-54 are all answered or inferred, and pantallas.md «Dónde va cada vista de la rebanada 2» says where each view goes. Work autonomously within the slice under AGENTS.md: one local commit per piece («feat: slice-2.N»), explicit paths only, no signature lines, no push. Stop only in the 5 cases of AGENTS.md, and before 2.10 (its brake table is not ready yet). Order: 2.1 first (its first step syncs the embedded copies of salida-principal and salida-entendi; both schemas changed), then by the dependency graph in the backlog: 2.2, 2.3, 2.11, 2.12, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9. You never call Meta: build tests with hand-made fake outputs; I record the real checker and judge outputs later. Write checker.en.md and judge-defense.en.md as drafts for me to review. After each piece, the usual return: what it does (5 lines), files, tests, what you did not do, [EL DISEÑO NO LO DICE], up to 3 questions with contrast.»
- **L2:** slice 1 closed at `1c1a313` (fix: P-64). Eber's uncommitted edits under `docs/` stay out of every commit.
- **L3 (2.1):** done at `8432c14`. Eber committed `3a3dc45 docs: update diseno and r1 and r2` meanwhile; review boundary = `3a3dc45`. Embedded schemas equal the working-tree design schemas (Eber has newer uncommitted edits there). Untracked `docs/casos-prueba/` is Eber's. Open questions to Eber: R4 only from figures (citations have no decision edge); mark button in the history panel; plan `focus`/`evidence_from` kept in the understanding `step_ended` until 2.12.
- **L4 (30-oct, Eber, verbatim):** «ok, paramos  cuando termine 2.2 para continuar mañana». After the 2.2 commit, stop; resume with 2.3 on 31-oct.
- **L5 (2.2):** committed as `feat: slice-2.2`. Risk: items 1 (migration `0004`, `ALTER TABLE edge ADD COLUMN data`, drops nothing) and 3 (`AddEdge`/`ChangeEdge` gain `data`; `POST /sources` gains optional `replaces`; two new routes). Open questions to Eber: card answered «other» does not warn (B-16 pending covers it); «Actualizado» requires your edit after the change; startup gives only `chg-n` labels. Next: T3 (2.3) on 31-oct.
