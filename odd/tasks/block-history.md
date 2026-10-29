# History by block (backlog-r1 piece 1.6)

Objective: implement piece 1.6 of `docs/construccion/backlog-r1.md`. Branch `main`; one local commit `feat: slice-1.6` when green, no trailers, never push. Next: 1.5b, then 1.7 and the review stop. Runner: `make test` · `make vet` · `make web`.

## Specs

Quoted verbatim from `docs/construccion/backlog-r1.md` §1.6 and B-10.

- **S1 · Qué:** «Haces clic en un párrafo y ves cómo se llegó a él: quién, cuándo, qué decisión con su contraste, tu «porque» y lo descartado»
- **S2 · Interfaz:** «`internal/graph`: `BlockHistory(ctx, projectID, sectionLabel string, block int) ([]Entry, error)`, calculada desde la historia y el grafo, sin modelo · `GET /sections/{label}/history?block={n}` · abrirla registra `decision_seen` con `history` para cada decisión mostrada»
- **S3 · B-10:** «La historia es por bloque del texto vigente: un párrafo, un ítem de lista o una tabla, según goldmark. Su historia junta las decisiones vinculadas a sus cifras y citas, y los cambios de texto (pasos y ediciones) cuyo texto aparece en ese bloque». Precisión: «los bloques se numeran desde 1, en el orden de goldmark; una lista anidada cuenta como un solo ítem de su lista de arriba. Un paso del agente entra en la historia del bloque si su `new_text` o su ancla aparece en él; una edición tuya, si el bloque contiene, normalizado (NFC, espacios colapsados), un fragmento nuevo de 20 caracteres o más respecto de la versión anterior `[inicial]`»
- **S4 · Listo cuando:** «Prueba: tras un paso del agente y una edición tuya, el bloque muestra las dos entradas en orden · prueba: una cifra vinculada lleva a su decisión con su contraste · prueba: una decisión reemplazada aparece como descartada»
- **S5 · No hacer:** «Que un modelo narre la historia. Abrirla desde el marco (rebanada 2)»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S3 (blocks of the text) | inline | done | `TestB10_BlocksAreParagraphsListItemsAndTablesInOrder` |
| T2 | S1, S2, S3, S4 (BlockHistory) | inline | done | `TestB10_StepAndYourEditShowInTheBlockInOrder` (mutation fails it), `TestD7_*` (2), `TestB10_BlockOutOfRangeIsNotFound` |
| T3 | S2 (route, decision_seen), P5 UI | inline | done | `TestP5_OpeningTheHistoryRecordsDecisionSeen`; HistoryPanel; not checked in a browser |

## Log

- **L1 (2026-10-29, user, verbatim):** «Then continue with 1.5.» (and AGENTS rule 13: pieces in order without waiting).
- **L2:** Document created before any 1.6 write.
- **L3 (2026-10-29):** 1.6 done; 7 choices in `elecciones-agente.md`; tygo now also generates `internal/graph` types.
