# Closing slice: the P0s of the user tests (backlog-cierre C.1 → C.7)

Objective: implement `docs/construccion/backlog-cierre.md` v0.1, pieces C.1 → C.7 in that order. Branch `main`; one local commit per piece (`feat: cierre.C.n`), explicit paths, no trailers, never push: Eber reviews and pushes.
Runner: `make test` · `make vet` · `make web` · headless-Chrome smoke with `TQ_PROVIDER=fake` per piece, reported with a screenshot. Delivery: not applicable (Eber pushes).

## Specs

Quoted verbatim from `docs/construccion/backlog-cierre.md` and from Eber's request (L1). B-99 to B-102 are read from the backlog's table.

- **S0 · Rule:** «nothing changes without you seeing it, and nothing of yours is lost or left without effect»
- **S1 · C.1 Interfaz:** «`TAB_ID` leído de `sessionStorage` (`tq.tabId`) con `try/catch`; si no hay, se crea y se guarda · `editors.release(projectID, tab)` y la ruta `DELETE …/editor` (en la tabla del control de dueño, B-87) · `pagehide` → `sendBeacon` · `editors.takeOver(projectID, tab, staleAfter)` y `POST …/editor` `{tab_id, take_over: true}` · el aviso con «Editar aquí»»
- **S2 · C.1 Listo cuando:** «Go: liberar y volver a reclamar desde otra pestaña da editor; tomar la edición con el otro renovado hace 20 s da editor y con el otro renovado hace 5 s no · web: el `TAB_ID` se conserva entre dos lecturas con `sessionStorage` y se crea si falla · humo en Chrome sin cabeza: abrir un proyecto, recargar 3 veces, seguir editando; abrir una segunda pestaña: solo lectura con «Editar aquí»»
- **S3 · C.1 No hacer:** «Guardar el lease en la base»
- **S4 · C.2 Interfaz:** «`getSummary` también tras cada evento del stream (paso, ronda, juez, firma) y cada 10 s `[inicial]` mientras haya un turno · **línea de estado** en la cabecera del centro: «El agente trabaja: ‹escribiendo «Síntesis» · leyendo fuentes (ronda) · revisando (juez) · preparando la defensa›» o «Te toca: n cosas» o nada; sale de los eventos que ya llegan · **primero diagnosticar** por qué la tarjeta de una ronda lanzada por el principal (B-64) se queda en «waiting» (Paula): la consulta cada 2 s debe arrancar también cuando la ronda la abrió un paso, no solo cuando la abrió un clic; prueba que lo fije»
- **S5 · C.2 Listo cuando:** «web: el texto de la línea de estado para cada evento (prueba de función pura) · prueba de la ronda lanzada por el principal que se actualiza sola · humo en Chrome sin cabeza con el falso: el gasto sube durante el turno sin recargar»
- **S6 · C.2 No hacer:** «Una ruta nueva en el servidor»
- **S7 · C.3 Interfaz:** «El proyecto en `#/p/{projectID}` (sin cambio del servidor); abrir y cerrar actualizan el hash; al cargar, si el hash nombra un proyecto tuyo se abre, si no (404) se vuelve al inicio sin error · el idioma en `localStorage` (`tq.lang`) con `try/catch`»
- **S8 · C.3 Listo cuando:** «web: el hash se lee y se escribe (función pura) · humo: abrir un proyecto, recargar, seguir en él; cambiar a español, recargar, seguir en español»
- **S9 · C.4 Interfaz:** «Con un turno en curso, `POST …/messages` responde 202 `{queued: true}` también en modo normal · el mensaje en cola se ve en la conversación con su marca, y se puede editar o quitar (`PUT`/`DELETE …/messages/{n}` mientras siga en cola) · si la cadena termina con mensajes en cola, se abre un turno con ellos · la respuesta del agente lleva «Respondo a tu mensaje: «…»» (en las instrucciones del principal, `principal.en.md`)»
- **S10 · C.4 Listo cuando:** «Go: un mensaje con turno en curso en modo normal queda en cola y entra al paso siguiente; al terminar la cadena con uno en cola se abre un turno; quitar uno en cola no lo envía · web: la marca y quitar · las pruebas de B-83 siguen en verde»
- **S11 · C.5 Interfaz:** «`afterYourAnswer` suma: si la respuesta cambia el texto (B-100) y la sección ya no tiene pendientes tuyos abiertos, toma el turno y corre **un** paso con la tarea «aplica tu decisión ‹dec› en ‹sección›»; si el turno está ocupado, queda en cola como un mensaje (C.4) · la tarjeta respondida muestra «El agente actualizará ‹sección› con tu decisión» · el informe de parada lista las decisiones aún no aplicadas con «Aplicarlas»»
- **S12 · C.5 Listo cuando:** «Go: «la otra» con la sección sin más pendientes lanza un paso y la sección deja de estar «escrita con lo anterior»; con otra tarjeta abierta en la sección no lanza nada; «mantener» no lanza; en una sección firmada el paso deja una propuesta · recorrido con el falso del caso 5: responder la tarjeta del piloto con «excluirlo» y ver el texto actualizado»
- **S13 · C.5 No hacer:** «Un paso por respuesta. Abrir una cadena»
- **S14 · C.6 Interfaz:** «**Primero diagnosticar**, con `data/tupaqnin.db` abierta en solo lectura (`sqlite3 -readonly`; no se modifica): (1) el paso del lote de corrección que dejó «Scope and influent microplastics» en una frase (proyecto de `ux-p1`); (2) la propuesta aceptada que insertó «…cálculo previo de 60 veces que no es comparable…» a mitad de oración (proyecto sin cuenta del jurado, caso 5, «Síntesis»); y reportar la causa antes de cambiar · **después:** la regla de B-99 en `apply.go` al aplicar el lote (sin modelo): (a) más de la mitad, (b) párrafo con decisión tuya o confirmada o cita vinculada, (c) oración sin cerrar o inserción a mitad de oración → propuesta · el rechazo del ancla que no termina en final de oración · la tarjeta «El juez encontró n fallas · corrigió m · Ver cambios» con «Sale / Entra» por falla»
- **S15 · C.6 Listo cuando:** «Go: un lote que quita el 60 % queda como propuesta; uno que inserta a mitad de oración se rechaza y reintenta; uno pequeño se aplica y deja su «Sale / Entra» con la falla · web: la tarjeta de cambios · la tabla de frenos de la 4.6 sigue pasando (6B usa el lote)»
- **S16 · C.6 No hacer:** «Pedir tu sí para toda corrección. Un modelo que juzgue qué es drástico»
- **S17 · C.7 Interfaz:** «Reescribir cada texto de la lista de §3 en lenguaje llano (en y es); quitar todo id a la vista; las notas del agente sin monoespaciado; «Evidence from: falta material» y demás mezclas, en el idioma de la interfaz · el pie «Decisions are made in their cards…» pasa a «Las decisiones críticas se toman en sus tarjetas»»
- **S18 · C.7 Listo cuando:** «web: prueba que falle si un texto de la página tiene `oq-n`, `rnd-n`, `dec-n`, `card-n`, `sec-n` o `cit-n`; prueba de que cada texto de §3 cambió · humo: recorrer el caso 5 en inglés y en español sin ids ni mezclas»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1, S2, S3 | inline (pieces build on each other, no Writer trigger) | done | RED `tab.test.ts` (module missing), GREEN; `TestB102_*`; smoke `c1` (reload x3, real second tab) |
| T2 | S4, S5, S6 | inline | pending | |
| T3 | S7, S8 | inline | pending | |
| T4 | S9, S10 | inline | pending | |
| T5 | S11, S12, S13 | inline | pending | |
| T6 | S14, S15, S16 | inline | pending | |
| T7 | S17, S18 | inline | pending | |

## Log

- **L1 (2026-11-03):** Eber: «Next work: docs/construccion/backlog-cierre.md v0.1 (synced in docs/), pieces C.1 → C.7 in that order, from the user tests in docs/construccion/pruebas-usuarios-2nov.md. The guiding rule: «nothing changes without you seeing it, and nothing of yours is lost or left without effect». Decisions B-99 to B-102 are adopted; do not ask again. One local commit per piece («feat: cierre.C.n»), explicit paths, no trailers, no push; make test, make vet and make web green before each commit; a headless-Chrome smoke test with TQ_PROVIDER=fake per piece, with a screenshot in the report. Do not call Meta or Fly. Create odd/tasks/cierre.md as for the slices. C.2 and C.6 start with a diagnosis you report before fixing. For C.6, open data/tupaqnin.db strictly read-only (sqlite3 -readonly) and never modify it: it is my live data, with the test users' projects. Pre-authorization under AGENTS.md rule 13, for this backlog only and exactly as specified: the new DELETE …/editor route and the take-over in C.1 (both inside the owner control of B-87); queued messages in normal mode in C.4; the automatic step after your answer in C.5; the proposal rule for judge corrections in C.6. Still stop and ask for anything not in the backlog, any new dependency or migration, or a contradiction with the design. If something is not in the design, mark [EL DISEÑO NO LO DICE], take the smallest choice, note it in elecciones-agente.md and keep going. Final report as before.»
