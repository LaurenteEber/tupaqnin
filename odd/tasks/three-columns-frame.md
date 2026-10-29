# Three columns and minimal frame (backlog-r1 piece 1.5b)

Objective: implement piece 1.5b of `docs/construccion/backlog-r1.md`. Branch `main`; one local commit `feat: slice-1.5b` by the parent when green, no trailers, never push. Runner: `make test` · `make vet` · `make web`. Then 1.7 and the review stop.

## Specs

Quoted verbatim from `docs/construccion/backlog-r1.md` §1.5b.

- **S1 · Qué:** «La app pasa de una página apilada a tres columnas: el marco a la izquierda, la conversación al centro y el documento a la derecha. El marco te deja ver para qué es el documento, cómo se juzga, su esqueleto con las decisiones críticas y lo que te toca»
- **S2 · Implementa:** «B3-9 (precisión del 29-oct) · espec. §9 (el marco), sin la línea de vuelta ni el «sin ver» exacto · `pantallas.md` («Disposición general») · P-28 si la 1.3 no lo cerró»
- **S3 · Leer:** «`docs/diseno/bloque-3/pantallas.md` («Disposición general» y P4, P5) · `docs/diseno/bloque-3/boceto-ui.html` (disposición, textos y comportamiento de referencia; sus datos son de ejemplo) · espec. §9 · fichas §2 («El marco»)»
- **S4 · Frame:** «`internal/graph`: `Frame(ctx, projectID) (Frame, error)`, sin modelo y de solo lectura; reutiliza el estado de trabajo, `Critical` (1.4) y los pendientes (1.5); suma el «porque» de cada tarjeta respondida y la explicación de firma vigente; excluye lo reemplazado y lo revocado · `GET /api/projects/{projectID}/frame` · desplegar una decisión registra `decision_seen` con `seen_via: frame`»
- **S5 · Web:** «web: tres columnas; selector de proyectos arriba del marco; el pedido con «Qué quieres» y las fuentes, y «Afinar» plegado (para quién, criterios, tu idea, exploratorio); el «Entendí» y las tarjetas como tarjetas dentro de la conversación; el documento aparece cuando hay secciones; «Firmar sección» junto al documento; la traza plegada; «Te toca» del marco lleva a la tarjeta o a la firma · todo texto de pantalla desde `en.json`/`es.json` · en pantallas angostas, columnas apiladas»
- **S6 · Listo cuando:** «Prueba: `Frame` del caso 5 trae el pedido literal, los criterios con sus casos, las secciones con su primera línea y la decisión crítica bajo «Síntesis» y «Conclusiones» con el mismo id · prueba: una decisión reemplazada no aparece · prueba: desplegarla registra `decision_seen` (`frame`) · prueba: ≤ 300 ms con el caso 5 · en el navegador (lo prueba Eber): el recorrido del boceto, etapas 1 a 4, con el proveedor falso»
- **S7 · No hacer:** «La línea de vuelta y el «sin ver» exacto (rebanada 2). Que un modelo resuma el marco. Mostrar ids internos. Abrir el marco desde la firma»
- **S8 · Si algo no está:** «`[EL DISEÑO NO LO DICE]` y una línea en `elecciones-agente.md`»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S4, S6 (Frame in Go, route, decision_seen frame, ≤ 300 ms) | delegated (context backstop, Writer trigger b) | done | `TestM1_*` (5, RED seen), `TestM4_FirstLine…`; parent spot check `make test`/`make vet` green |
| T2 | S1, S2, S3, S5, S7 (three-column web) | delegated, same writer (one unit: the frame and its page) | done | `make web` green; browser walk-through is Eber's |

## Log

- **L1 (2026-10-29):** AGENTS rule 13 — pieces in order without waiting; order 1.5 → 1.6 → 1.5b → 1.7 (1.5b depends on 1.6).
- **L2:** Delegated to one bounded writer because the parent session passed the long-context backstop; the parent verifies, records choices and commits. «Firmar sección» is 1.7: in 1.5b the button exists only as placement (no signoff flow).
- **L3 (2026-10-29):** Writer returned; S1–S7 covered except the browser walk-through. 8 choices logged by the writer (`ComputeFrame` name and signature: memory imports graph). The writer created one case-5 project on Eber's :8080 server by mistake; that server runs the fake provider (its last triage is fixture 04), so no real spend. Request card stays editable after submit (no submitted flag in `RequestView`).
