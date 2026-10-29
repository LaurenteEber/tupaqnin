# Criticality and cards (backlog-r1 piece 1.4)

Objective: implement piece 1.4 of `docs/construccion/backlog-r1.md`. Branch: `main`; one local commit `feat: slice-1.4` when green, no trailers, never push (AGENTS.md 12–13). **Review stop after this piece.** Test runner: `make test` · `make vet` · `make web`.

## Specs

Quoted verbatim from `docs/construccion/backlog-r1.md` §1.4.

- **S1 · Qué:** «La app calcula qué decisión es crítica y le abre su tarjeta. Respondes con una opción y un «porque»; «ok» no cuenta»
- **S2 · Critical:** «`internal/graph`: `Critical(ctx, projectID, decisionLabel) (bool, []Reason, error)`, una sola función, llamada al leer y en cada transacción que cambia aristas o el rol (T-5)»
- **S3 · Abrir tarjeta:** «una decisión del agente que pasa a crítica abre su tarjeta (`open`)»
- **S4 · Responder:** «`POST /cards/{label}/answer`: «mantener» la deja `confirmed`; «la otra opción» la deja `superseded` y aplica B-16»
- **S5 · Eventos:** «una respuesta sin «porque» válido registra `answer_without_reason`, y cada respuesta, `act_timed` (K-11)»
- **S6 · Porque válido:** «el «porque» no puede estar vacío ni ser un asentimiento de la lista de tu idioma (T-1; la lista y cómo se compara, en B-20 (a), con la clave `assent_list` en `en.json` y `es.json`)». B-20 (a): «es: ok, okay, sí, si, dale, listo, de acuerdo, bien, vale, va; en: ok, okay, yes, yep, sure, fine, agreed, k, lgtm. Se comparan sin mayúsculas, sin tildes y sin signos finales; un «porque» de solo signos o emojis tampoco vale.»
- **S7 · Obsoleta:** «una tarjeta cuya decisión cambió antes de responder pasa a `obsolete`»
- **S8 · Firmada:** «si una decisión de una sección firmada pasa a crítica, la sección pasa a `changed_since_signoff` (T-5, K-6)»
- **S9 · Todo:** «`GET /todo` lista las tarjetas abiertas»
- **S10 · Listo cuando:** «Prueba: cada uno de los motivos de la 1 (cifra, rol conclusión o recomendación, 2 o más secciones) hace crítica una decisión · prueba: «ok», «sí», «yes», «sure» y el vacío dejan la tarjeta abierta · prueba: elegir la otra opción crea una decisión tuya y reemplaza la del agente · prueba: responder registra `decision_seen` con `card`»
- **S11 · No hacer:** «El cuarto motivo, tu marca (rebanada 2). La tarjeta de tu propia edición crítica (rebanada 2). Objetar y revocar (rebanada 4)»
- **S12 · B-16:** «Si eliges la otra opción en una tarjeta, nace una decisión tuya: su enunciado es el «en vez de» anterior; su «en vez de», el enunciado anterior; su «porque», el tuyo. Las aristas `covers` y `used_in` pasan a ella (B-8)… La decisión tuya que nace con `other` lleva como «elegiría si» el «elegiría si» de la decisión anterior, si lo tenía; si no, queda vacío y no deja pendiente»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S2 | inline · Risk item 3 (store read refactor shared by DB and Tx; memory moves to graph.Critical) | done | `TestT5_*` (6, incl. in-transaction read); full suite green after memory switch |
| T2 | S3, S7, S8 | inline | done | `TestT5_AgentDecisionThatBecomesCriticalOpensItsCard`, `TestT5_NonCriticalDecisionGetsNoCard`, `TestE3_*`, `TestK6_*`; mutation (no card) fails 2 tests |
| T3 | S4, S5, S6, S12 | inline | done | `TestE2_*`, `TestP2_*`, `TestB16_*`, `TestE3_AnsweringAClosedCardIsRefused`, `TestB20a_*` (2) |
| T4 | S9, P4 UI | inline | done | `TestP4_*`, `TestK13_*`; `make web` built (React not run in a browser) |

## Log

- **L1 (2026-10-29, user, verbatim):** «Fixes accepted, including both choices. From now on, run 1.3 → 1.7 in order without waiting for review between pieces. […] Stop after 1.4 for a short review, then after 1.7.»
- **L2 (2026-10-29):** Tracking created late: `internal/store/read.go` (Querier) and `internal/graph/critical.go` were written before this document; Eber asked «estas implementando con odd y engram, verdad?», which surfaced the gap. Design read: T-5, fichas §3, estados §3, spec §1.5, glosario §2, pantallas P4. Plan: memory's `isCritical` (its comment says it lives there until the checks need it) is replaced by `graph.Critical`; criticality runs inside each transaction through `store.Querier` (the read pool does not see uncommitted writes).
- **L3 (2026-10-29):** 1.4 committed `feat: slice-1.4`. `make test`, `make vet`, `make web` green. 9 choices appended to `docs/construccion/elecciones-agente.md`. Not checked in a browser. Not covered by the fake demo: the fake step recording registers no decision, so no card appears in the demo until 1.8 brings case-5 recordings. **Review stop for Eber.**
