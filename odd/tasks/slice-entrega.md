# Slice Entrega · The jury enters with its link

Objective: backlog-entrega v0.2, pieces E.1 → E.6, then slice 4 (`odd/tasks/slice-4.md`). Branch: `main`, one local commit per piece `feat: slice-entrega.E.n`, explicit paths, no signature or trailer lines, never push. Delivery: single local series, Eber reviews and pushes. Runner: `go test -count=1 ./...` · `make vet` · `make web`; each commit gated on their exit codes. Never call Meta, never run fly.

## Specs

Each piece's spec is its row in `docs/construccion/backlog-entrega.md` v0.2, read verbatim there: «Qué», «Implementa», «Leer», «Interfaz», «Listo cuando», «No hacer», «Si algo no está». Decisions B-87 to B-94 in the same file.

- **S0 · P-112** «fix: P-112 walkthrough accepts the campo question on any section»: "in the Meta walkthrough (byGoal only), step 7 accepts an open «por aclarar» naming the pilot or «campo» that blocks any section; keep the fake-provider walkthrough as it is. Do not call Meta; I rerun the full demo-check when Entrega is done."
- **S1 · E.1** Acceso y aislamiento (B-87, B-88, B-92).
- **S2 · E.2** Seguridad del borde.
- **S3 · E.3** Topes por link, del día y total (B-89).
- **S4 · E.4** Copia, restauración y despliegue (B-93, B-94 only if Eber's H-5 confirms P-5).
- **S5 · E.5** Cuentas (B-90, B-91).
- **S6 · E.6** Inicio y tus proyectos.

Pre-authorized under AGENTS rule 13, for these backlogs only and exactly as specified there (L1): "the B3-22 security changes; the file handling they name (body caps and 413, VACUUM INTO backups and deleting old backups); migration 0005 and the E.1 data model; the hourly cleanup that deletes expired anonymous projects (this overrides AGENTS rule 6 for that cleanup only); the new dependency above." Stop and ask for: "anything not in the backlogs, any other dependency, a migration that drops data beyond that cleanup, a contradiction with the design, tests red after 2 attempts."

## Tasks

| ID | Spec | Route | Status | Commit |
|---|---|---|---|---|
| T0 | S0 | inline | done | 71d4dfb |
| T1 | S1 · E.1 | delegated (context backstop); verifier: item 2 security | done | feat: slice-entrega.E.1 |
| T2 | S2 · E.2 | delegated (context backstop); verifier: item 2 security | in progress | |
| T3 | S3 · E.3 | delegated (context backstop) | pending | |
| T4 | S4 · E.4 | delegated (context backstop); verifier: item 5 delivery | pending | |
| T5 | S5 · E.5 | delegated (context backstop); verifier: items 2, 5 | pending | |
| T6 | S6 · E.6 | delegated (context backstop) | pending | |

## Log

- **L1 (1-nov, Eber):** "Fourth make demo-check with Meta: TestDemoCheck passes up to Wonder's first round (27 calls, 1.27 USD) and TestDemoCheckRound passes again. TestDemoCheckSlice2 failed at step 7: the «por aclarar» does name the pilot and «campo», but Meta made it block «Síntesis» instead of «Estudios incluidos» (supervision P-112). Your B-42 fix is accepted as noted in elecciones-agente.md. Slice 3 is closed.

0. «fix: P-112 walkthrough accepts the campo question on any section»: in the Meta walkthrough (byGoal only), step 7 accepts an open «por aclarar» naming the pilot or «campo» that blocks any section; keep the fake-provider walkthrough as it is. Do not call Meta; I rerun the full demo-check when Entrega is done.

Then the next work, in this order, read from docs/ (synced and committed by me; AGENTS.md now points to them):
1) Slice «Entrega»: docs/construccion/backlog-entrega.md v0.2, pieces E.1 → E.6.
2) Slice 4: docs/construccion/backlog-r4.md v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6 (4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided).
Create odd/tasks/slice-entrega.md and odd/tasks/slice-4.md as you did for slice 3. One local commit per piece («feat: slice-entrega.E.n», «feat: slice-4.n»), explicit paths, no signature or trailer lines, never push; make test, make vet and make web green before each commit. A piece that is not green is not committed: that is the switch (backlog-r4 «Respuesta»).
Decisions already taken (do not ask again): B-90 approved — golang.org/x/crypto/bcrypt is the only new dependency, latest tagged version under docs/construccion/versiones-seguras.md; B-94 — the runtime image runs as root ONLY if my H-5 test deploy confirms P-5 (I will tell you); B-79, B-82, B-83 and all of B-73..B-94 as written.
Pre-authorization under AGENTS.md rule 13, for these two backlogs only and exactly as specified there: the B3-22 security changes; the file handling they name (body caps and 413, VACUUM INTO backups and deleting old backups); migration 0005 and the E.1 data model; the hourly cleanup that deletes expired anonymous projects (this overrides AGENTS rule 6 for that cleanup only); the new dependency above. Still stop and ask for: anything not in the backlogs, any other dependency, a migration that drops data beyond that cleanup, a contradiction with the design, tests red after 2 attempts.
Never call Meta and never run fly commands: list them for me (H-5 test deploy, restore drill, public deploy, invite links, demo-check runs). If something is not in the design, mark [EL DISEÑO NO LO DICE], take the smallest choice, note it in elecciones-agente.md and keep going.
At the end of each slice, one report: commits, what each piece does, tests, [EL DISEÑO NO LO DICE] items, Makefile targets and commands for me, questions."
- **L2 (T0):** the failing check lives in `TestDemoCheckSlice2`, in the meta-only `meta_test.go`; that test does not set `byGoal`, but the whole file only runs against Meta, so no flag was needed. `rushed_cases_test.go` (fake) still pins «Estudios incluidos». No runnable RED (needs Meta): checked with `go vet -tags=meta ./internal/demo/walkthrough/`, suite green. `AGENTS.md` carries an uncommitted line from Eber; left out of every commit.
- **L3:** review consent is not requested piece by piece during the slice (AGENTS rule 13, «without waiting for review»); each commit's `gentle-ai review assess` result goes here and the decision goes in the slice report.
- **L4 (T1, E.1):** writer delivered the piece; independent verifier found 2 blockers on the real binary. B1: `/api/pro%6Aects/{id}/…` skipped the owner check on all 56 routes, with or without a cookie, because the guard compared the escaped path while the mux decodes each segment. B2: `invite list --json` exited 0. Fixed RED→GREEN with `TestB3_22_6_EncodedPrefixMeetsTheOwnerCheck` (four spellings over the whole route table) and two more cases in `TestB92_InviteRefusesBadArguments`; the verifier's own probes pass after the fix. Open advisories: `invite create --budget Inf` accepted and `NaN` fails with a raw SQL error; `invite create` with an extra argument reports «--label is required»; `/I/<token>` (capital I, not a route) logs the token; a session lasts a fixed 14 days, so an active visitor without an account loses a live project on day 14 (in `elecciones-agente.md`, question for Eber). Review assessment: `unassessable` (untracked files, before commit), escalated item 2; decision goes in the slice report.
