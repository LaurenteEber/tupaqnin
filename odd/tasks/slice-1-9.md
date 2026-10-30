# Piece 1.9 · On-demand reading, conversation and a clear understanding

Objective: backlog-r1 v0.15, piece 1.9, with `read_source` and `search_sources` (the tools micro-test passed, `44f0b11`). One local commit `feat: slice-1.9` on `main`, no trailers, never push; then stop for Eber's test with Meta. Runner: `make test` · `make vet` · `make web`.

## Specs

- **S1 · Schemas:** «Start by syncing the embedded schemas with docs/diseno so make test is green again.» (`project_title`, `criteria` `minItems: 0`, `reply`, role `conclusion` · `other`.)
- **S2 · B-23:** «`criteria` puede venir vacío (B-23)». Listo cuando: «Prueba: un pedido sin criterios produce un «Entendí» aceptado».
- **S3 · B-24:** «`stop_reason` en `step_ended` (B-24)». Listo cuando: «prueba: `stop_reason` se lee al recargar».
- **S4 · R-5:** «`reply` en la salida del principal (esquema ya actualizado) y en la conversación». Listo cuando: «prueba: un mensaje que pregunta trae `reply`, no escribe en el documento y no cuenta como trabajo».
- **S5 · R-3 tools:** «`read_source(fuente, tramo)` y `search_sources(texto)` sobre las fuentes del proyecto, con `delivery` (`via = read`) · ficha de cada fuente en el índice (título, tipo, largo, primeras líneas)». Listo cuando: «prueba de las herramientas con salidas grabadas».
- **S6 · R-3 literal rule:** «una cifra o cita literal debe estar en una fuente que el principal leyó (`delivery` con `via = read`)». Listo cuando: «prueba: una cifra fuera de un pasaje leído deja «cifra sin decisión»».
- **S7 · Prompts (R-4, R-7, P-44):** «instrucciones del principal y del «Entendí»: leer el material en el «Entendí» y reportar lo leído; sin material y sin búsqueda, no escribir una síntesis de memoria y ofrecer en `understood` «subir material» o «esperar la búsqueda» (R-4 precisada; «de memoria, marcado» llega con R-1); decisiones de verdad (≤ 2 nuevas por paso, «elegiría la otra si» = cuándo gana la alternativa); triaje en palabras».
- **S8 · U-1:** «`project_title` en la salida del «Entendí» (esquema ya actualizado), editable en la cabecera».
- **S9 · R-8:** «rol `conclusion` u `other` (R-8)».
- **S10 · UI U-1 a U-6:** «interfaz U-1 a U-6 (el botón del «Entendí» es «Confirmar» y no condiciona el arranque, B-17)».
- **S11 · P-46:** «párrafos insertados separados (P-46)».
- **S12 · P-41 corrected:** «en la firma, «Ver la historia de ese punto» vuelve a aparecer después de una vuelta con «falta» (flujos §3), nunca antes de tu primera explicación (corrige P-41)».
- **No hacer:** «Procedencia por párrafo y «afirmación sin fuente» (R-1, R-2: rebanada 2). Plan acordado (R-6: rebanada 2). Búsqueda fuera de tu material (R-9: rebanada 3)».

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 S2 S8(Go) S9 | inline | done | schemas copied; 3 understanding fixtures get `project_title`, roles to two values; `TestB23_RequestWithoutCriteriaGetsAnAcceptedUnderstanding` (passed once synced), `TestU1_UnderstandingProposesTheProjectTitleWithoutReplacingYours` (RED seen) |
| T2 | S3 S4 S12(Go) (step_ended, conversation API, model info, rename) | inline | done | `TestB24_StopReasonIsStoredInTheStepWhereTheChainStops`, `TestR5_AQuestionGetsAReplyThatWritesNothingAndIsNotWork` (RED seen); `TestP41_HistoryOfThatPointShowsAgainAfterWonderNamesWhatIsMissing` (RED seen); server `TestB24_StopReasonIsReadAfterReload`, `TestU1_PersonRenamesTheProject`, `TestU2_SummaryShowsTheModel` (written after the code) |
| T3 | S5 S6 | inline | done | `TestR3_ToolsReadTheMaterialBeforeCitingWithRecordedOutputs` (recorded micro-test calls), `TestR3_FigureFromASourceNotReadLeavesFigureWithoutDecision` (RED seen); memory `TestR3_ReadSource…`, `TestR3_SearchSources…`, `TestR3_IndexShowsACardOfEachSource` (written after the code); old A2/A3/E1/B15/B16 tests seed the read |
| T4 | S7 | inline | done | prompts only, no runnable RED (Eber checks with Meta) |
| T5 | S11 | inline | done | `TestP46_InsertedTextIsASeparateParagraph` (RED seen); first rule broke inline inserts (A3, D7, B10), narrowed to glued text |
| T6 | S8(web) S9(web) S10 S12(web) + S3/S4 on screen | delegated (writer trigger a: parallel with T3, disjoint `web/src`) | done | writer report: 9 items, `pnpm run build` green; parent spot check: diff inside `web/src`, `make web` green; not run in a browser |

## Log

- **L1 (2026-10-30, user, verbatim):** «The tools micro-test passed: 5/5 runs used tools, 5/5 valid outputs, 5/5 read and quoted «1,5 veces» literally. R-3 stands; B-22 is discarded. First, commit the micro-test on its own (tools_test.go, the Makefile target, testdata/tools/, and docs/casos-prueba/micro-prueba-herramientas/), without the docs/diseno or docs/construccion changes and without signing (no Co-Authored-By, Signed-off-by, Claude-Session or «Generated with» lines). Then build piece 1.9 with read_source and search_sources, as in backlog v0.15. Start by syncing the embedded schemas with docs/diseno so make test is green again. Stop after 1.9 for Eber's test with Meta.»
- **L2 (2026-10-30, user, earlier turn, verbatim):** «Note: P-41 was my error — after Wonder names what is missing, «See the history of that point» must show again (flow §3); never before the first explanation.»
- **L3:** micro-test committed alone as `44f0b11`.
- **L4 (2026-10-30):** T1–T6 done; `make test`, `make vet`, `make web` green. Commit `feat: slice-1.9` without Eber's `docs/diseno` and backlog/supervision edits; its schema-copy tests need those docs committed. Next: Eber's test with Meta.
