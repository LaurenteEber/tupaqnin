# Section status, pendings and fingerprint (backlog-r1 piece 1.5)

Objective: implement piece 1.5 of `docs/construccion/backlog-r1.md`. Branch `main`; one local commit `feat: slice-1.5` when green, no trailers, never push (AGENTS.md 12–13). Next pieces in dependency order: 1.6, then 1.5b (it depends on 1.6), then 1.7 and the review stop. Runner: `make test` · `make vet` · `make web`.

## Specs

Quoted verbatim from `docs/construccion/backlog-r1.md` §1.5.

- **S1 · Qué:** «Cada sección dice en qué estado está y qué le falta, con una sola función que usan todas las vistas. La huella de la sección reemplaza la comparación de B-3. Solo una pestaña edita»
- **S2 · Huella:** «`internal/fingerprint`: `Section(markdown string) string` (`tq-md/1`)»
- **S3 · Estado:** «`internal/graph`: `SectionStatus(ctx, projectID, label) (Status, []Pending, error)`: `draft`, `ready` (calculado), `signed` y `changed_since_signoff`; los pendientes de la 1 son tarjetas abiertas y pendientes del agente, calculados (B-15)»
- **S4 · Edición firmada:** «tu edición sobre una sección firmada la pasa a `changed_since_signoff` (estados §1)»
- **S5 · Rutas:** «`GET /sections/{label}` y `GET /summary` (lo gastado frente al tope, B3-20)»
- **S6 · Pestañas:** «la segunda pestaña abre en solo lectura · el editor guarda antes de enviar un mensaje o pedir firmar»
- **S7 · Listo cuando:** «Prueba: dos Markdown con distinto formato y el mismo contenido dan la misma huella; uno con contenido invisible da otra · prueba: abrir y guardar sin editar no crea versión · prueba: con una tarjeta abierta la sección está en `draft`; al responderla, en `ready` · prueba: una segunda pestaña no puede guardar»
- **S8 · No hacer:** ««Escrita con lo anterior» y la lista de cambios (rebanada 2, E-4 y A-1). Volver a «firmada» al deshacer (rebanada 4, G-1)»
- **S9 · Si algo no está:** «Cómo se reconoce la pestaña editora: el mínimo, con `[EL DISEÑO NO LO DICE]`»
- **S10 · B-15 (pendientes del agente):** «Cifra sin decisión» · «Falta en el texto» (1.7) · «Retorno sin la forma fija»: «el último `output_retry` del turno; se cierra con el siguiente fin de paso válido» · «Fuente que no sostiene» · «Texto con una decisión reemplazada» (B-16): «se cierra con el siguiente paso del agente que toca esa sección y declara tu decisión entre las que usa»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S2, S7 (fingerprint) | inline | done | `TestS4_SameContentWithOtherFormattingGivesTheSameFingerprint` (RED seen: padding), `TestS4_InvisibleContentChangesTheFingerprint`, `TestS4_ContentChangesChangeTheFingerprint` |
| T2 | S3, S10, S7 (draft/ready) | inline | done | `TestE1_OpenCardLeavesTheSectionInDraftAndAnsweringMakesItReady`, `TestB15_*` (2), `TestB16_SupersededDecisionInTextClosesWithTheNextStepThatDeclaresIt` |
| T3 | S4, S7 (no version on unchanged save) | inline | done | `TestS4_SavingWithoutEditingMakesNoVersion`, `TestE1_YourEditOnASignedSectionChangesIt` |
| T4 | S5 | inline | done | `TestP3_SectionDetailAndSummary` |
| T5 | S6, S9, S7 (second tab) | inline | done | `TestS2_ASecondTabCannotSave` (mutation fails it); page read-only + 15 s renewal |

## Log

- **L1 (2026-10-29, user, verbatim):** «1.3 and 1.4 accepted, including all your choices except one, and both doubts […] Then continue with 1.5.»
- **L2:** Document created before any 1.5 write. Piece 1.5b appeared in Eber's `45d305e`; it depends on 1.6, so the order is 1.5 → 1.6 → 1.5b → 1.7.
- **L3 (2026-10-29):** goldmark v1.8.6 and jcs v1.0.2 added at the versions Eber adopted in `versiones-seguras.md` (spec §7, AGENTS rule 11). 8 choices in `elecciones-agente.md`. Not checked in a browser.
