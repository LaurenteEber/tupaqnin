# Apply the agent step (backlog-r1 piece 1.3)

Objective: implement piece 1.3 of `docs/construccion/backlog-r1.md` (validate and apply the principal's step). Branch: `main`. AGENTS.md rules 12–13 (29-oct): one local commit `feat: slice-1.3` when the piece is green, no trailers, never push; autonomous 1.3 → 1.7, review stops after 1.4 and 1.7; `[EL DISEÑO NO LO DICE]` → smallest choice, marked in code, one line in `docs/construccion/elecciones-agente.md`. Delivery: ask-on-risk. Test runner: `make test` · `make vet` (Go, temporary SQLite per test, fake provider).

## Specs

Quoted verbatim from `docs/construccion/backlog-r1.md` §1.3; the design files it lists are the source of truth (`AGENTS.md` rule 2).

- **S1 · Qué:** «Lo que el principal entrega se valida y se aplica: su texto, sus decisiones con contraste, las cifras vinculadas a una decisión y los datos copiados, vinculados a su fuente»
- **S2 · B-8 (aristas):** «migración `0002` con la clave de `edge` en `(project_id, id, version)`; `ChangeEdge` y `EndEdge` reemplazan a `SupersedeEdge` de la 0.4 (P-6), cada una con su evento `edge`»
- **S3 · EndStep / ApplyStep:** «`internal/cycle`: `ApplyStep(ctx, projectID, turnID string, output json.RawMessage) (ApplyResult, error)`, después de `EndStep` (0.7) · **`EndStep` cambia:** solo valida la forma (y pide el reintento); `ApplyStep` escribe `step_ended` (con las etiquetas asignadas a cada `new-n` y lo que no se aplica, B-17) en la misma transacción que el paso»
- **S4 · Reglas 2 a 6:** «las reglas 2 a 6 rechazan con un mensaje para el agente y cuentan como el reintento · aplica `replace` e `insert_after` de atrás hacia adelante; asigna etiquetas a los `new-n` antes de crear aristas»
- **S5 · Objetos y aristas:** «crea decisiones (`agent`), cifras, datos y citas, y las aristas `covers`, `used_in`, `applies`, `depends_on_assumption`, `comes_from` y `uses`»
- **S6 · Cifra sin decisión:** «un número del texto nuevo sin vínculo, que no aparece literal (con separadores normalizados) en una fuente usada en el paso ni en tu material, es un pendiente del agente «cifra sin decisión» (B-15); qué cuenta como número lo fija B-20 (b)». B-20 (b): «una secuencia con al menos un dígito; no cuentan la numeración de listas ni las etiquetas (`sec-3`); un año cuenta, pero si aparece literal en una fuente usada no deja pendiente»
- **S7 · Fuente que no sostiene:** «una cita cuyo pasaje no aparece literal en su fuente se crea sin la arista `comes_from` y deja el pendiente del agente «fuente que no sostiene» (fichas §3, glosario §9, B-15); no rechaza la salida»
- **S8 · Lo no aplicado:** «lo que la 1 no aplica se guarda sin aplicarse (B-17)» — B-17: «Lo que la 1 no aplica (`proposed_open_questions`, `handled_changes`) se guarda en el fin de paso, sin aplicarse»
- **S9 · Encadenar (B-17):** «el runner de la 0.10 vuelve a llamar al principal, paso tras paso, hasta 5 `[inicial]`, y se detiene antes si su paso no deja trabajo; cada paso pasa por `EndStep` y `ApplyStep`». Precisión B-17: «la cadena arranca sola cuando termina el «Entendí» (no espera a que confirmes los casos) y con cada mensaje tuyo. Un paso encadenado sin mensaje tuyo recibe un mensaje fijo de la app, en inglés y sin contenido tuyo (`Continue with the next step of the task.`). La cadena para cuando un paso no trae `new_text` ni decisiones nuevas, cuando un paso queda como pendiente del agente o al llegar a 5»
- **S10 · B-11:** «Hasta la rebanada 4, si la sección cambió desde la versión que leyó el agente, su paso se rechaza y se reintenta una vez con el texto vigente.»
- **S11 · Listo cuando:** «Pruebas con salidas grabadas: un ancla que no aparece, o que se repite, se rechaza diciendo cuántas veces aparece · un `new-n` en el texto se rechaza · «reemplaza» que apunta a una decisión tuya se rechaza · una cifra con vínculo crea `covers`; un dato literal de una fuente crea `comes_from`; un número suelto deja el pendiente · todo el paso va en una sola transacción»
- **S12 · No hacer:** «La regla 7 (A-1) y las demás comprobaciones de fichas §3, como los códigos internos en el texto: rebanada 2. Propuestas (rebanada 4)». Si algo no está: «Un caso de anclas que §3 no cubre: rechazar y marcar `[EL DISEÑO NO LO DICE]`»

- **S13 · P-28 (Eber, 2026-10-29):** «In 1.3, P-28: the stream must show the parsed text, triage line and note, never the raw JSON.»

Design reading list (from the item): espec. §2 y §3 completos · `docs/diseno/bloque-3/salida-principal.schema.json` · fichas §3 y §9 · estados §2 · glosario §3 y §8 · defaults T-2 a T-4.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S2 | inline · Risk item 1 (migration, keeps every row) | done | `TestB8_*` (4), migration test from a pre-0002 database |
| T2 | S3, S4, S10, S11 | inline | done | `TestS3R2_*`, `TestS3R3_*`, `TestS3R4_*`, `TestS3R5_*`, `TestS3R6_*`, `TestB11_*`, `TestS2_TheWholeStepGoesInOneTransaction` |
| T3 | S1, S5, S6, S7, S8, S11 | inline | done | `TestA3_*`, `TestA2_*`, `TestT4_*`, `TestB18_*`, `TestB20b_*` |
| T4 | S9 | inline | done | `TestB17_*` (3) |
| T5 | S13 | inline | done | `TestP28_*`; curl end-to-end on :8091 |

Dependencies: 0.7, 1.2, B-8, B-11, B-15, B-17 — 0.7 to 1.2 are committed (`481de50`, `9a00b4a`).

## Log

- **L1 (2026-10-29, user, verbatim):** «vamos a continuar la implementación de este proyecto que estuvimos trabajando con muse code, revisa si tienes acceso a la memoria de lo que avanzó, luego inicia los preparativos para inciiar con engram y odd»
- **L2 (2026-10-29):** Engram holds only the research phase (up to 2026-10-12); the Muse work (pieces 0.1–1.2) was reconstructed from git. Last commits: `481de50 feat: slice 1.2`, `9a00b4a feat: slice 1.2 uncommit files`. Working tree clean. Baseline: `make test` all packages ok, `make vet` clean. Next piece per dependency graph: 1.3. Sized large (migration, long requirements). Commits are Eber's (AGENTS.md rule 12 overrides ODD work-unit commits). Next step: read the design list, then T1.
- **L3 (2026-10-29, user, verbatim):** «1.2 accepted, including your three choices. Eber’s browser test (fake provider) found, please fix before 1.3: (1) P-26 … (5) P-31 … In 1.3, P-28: the stream must show the parsed text, triage line and note, never the raw JSON.» Muse memory read at `~/.local/share/muse/memory/projects/Users-Eber-Documents-SideProjects-tupaqnin-62362f280d8e6f33/continuacion-rebanada-1.md`. P-26, P-27, P-29, P-30, P-31 are fixed before 1.3 (small path, outside this document); P-28 added as S13/T5. 1.3 blocked until those fixes land.
- **L4 (2026-10-29):** P-26/27/29/30/31 implemented, uncommitted, awaiting Eber. Root cause of P-26 = P-29 (reproduced with curl: the understanding turn got fixtures 01/02 → `agent_pending`). `make test`, `make vet`, `make web` green; curl end-to-end: understanding `accepted`, role change v1→v2 then save 200, message turn gets the step fixture. Browser not checked by the agent. 1.3 starts after Eber accepts.
- **L5 (2026-10-29, user, verbatim):** «Fixes accepted, including both choices. From now on, run 1.3 → 1.7 in order without waiting for review between pieces. After each piece, when make test and make vet are green: make one local commit (no push) titled feat: slice-1.N, with no Co-Authored-By or Claude-Session trailer, and continue. For any [EL DISEÑO NO LO DICE], take the smallest choice, mark it in code, and append one line to docs/construccion/elecciones-agente.md (piece · choice · where · alternative). Stop and ask Eber only if: something contradicts the design or an adopted decision; a new dependency or major version is needed; the data model changes beyond the piece, or a migration would drop data; anything touches security (B3-22) or file handling; tests stay red after 2 attempts. Never call Meta: Eber runs make demo-check. Stop after 1.4 for a short review, then after 1.7. Keep P-28 in 1.3.» Also: «ok entonces por ahora es solo para el caso demo» (fake = case 5 only; option A: understanding panel shows pending). Committed `58698f8 fix: P-26, P-27, P-29, P-30, P-31`.
- **L6 (2026-10-29):** 1.3 done and committed `e10ca0f feat: slice-1.3`. `make test`, `make vet`, `make web` green. RED was observed for B-8 and P-31; the apply tests were written after the code, so two mutations were run instead (anchor count, datum link): both made their tests fail. 13 `[EL DISEÑO NO LO DICE]` choices recorded in `docs/construccion/elecciones-agente.md`. Next: 1.4 (criticality and cards), then stop for Eber's review.
