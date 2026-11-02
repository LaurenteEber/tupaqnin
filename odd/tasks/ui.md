# Three-panel interface (backlog-ui U.1 → U.2 → U.3)

Objective: implement `docs/construccion/backlog-ui.md` v0.1, only `web/` and the served bundle. Branch `main`; one local commit per piece (`feat: ui.U.n`), explicit paths, no trailers, never push; delivery `single-pr` is not applicable: Eber reviews and pushes.
Runner: `pnpm test` (inside `make web`) · `make test` · `make vet` · headless-Chrome smoke with `TQ_PROVIDER=fake`, reported with a screenshot.

## Specs

Quoted verbatim from `docs/construccion/backlog-ui.md` and from Eber's request (L1).

- **S1 · U.1 Qué:** «La página pasa de `.columns` a tres paneles: izquierda (por ahora con lo que había en el menú «Tus proyectos ▾»), centro con «Conversación · Documento», derecha con «El marco · Historia». Paneles laterales plegables, divisor arrastrable y fondo de cuadros en el centro»
- **S2 · U.1 Listo cuando:** «`pnpm test`: pruebas de `layout.ts` (preferencias con `localStorage` que lanza error; límites del ancho; flechas; `routeTarget` para cada destino de hoy, incluido «Firmar» desde el informe → Documento, tarjeta → Conversación, historia → Historia con el panel abierto) · `make web`, `make test` y `make vet` en verde · **las pruebas de recorrido de Go no cambian** (no se tocan rutas) · prueba de humo con Chrome sin cabeza y el proveedor falso, que el agente corre y reporta: abrir el caso 5, plegar y desplegar los dos paneles, arrastrar el divisor, ir del esqueleto al documento y de una tarjeta del marco a la conversación, sin errores en la consola»
- **S3 · U.1 No hacer:** «Cambiar lo que muestra o hace cada componente existente (solo se mueven). Tocar `internal/` salvo el paquete servido. Sumar dependencias. Agregar el selector de secciones de la maqueta (U-3) o la serifa (U-4). Mostrar el marco durante la firma (P7)»
- **S4 · U.2 Qué:** «La izquierda queda como en la maqueta: «Nuevo proyecto» y «Probar con el caso de demo» arriba; «Tus proyectos» como historial; abajo el aviso sin cuenta, el idioma y la cuenta»
- **S5 · U.2 Listo cuando:** «`pnpm test`: pruebas en `home.ts` o un módulo nuevo para el texto de cada proyecto de la lista (sin nombre, fecha, contadores) y para qué muestra la parte de abajo con y sin cuenta · humo en Chrome sin cabeza: crear el caso de demo desde la izquierda, volver a la lista, abrirlo de nuevo, cambiar el idioma»
- **S6 · U.2 No hacer:** «Cambiar las reglas de cuentas, de proyectos por navegador o de topes (E.1, E.5, B-89)»
- **S7 · U.3 Qué:** «La derecha suma «Markdown»: el documento como se descarga, en «Vista» o «Fuente», con «Copiar» y «Descargar .md»»
- **S8 · U.3 Listo cuando:** «`pnpm test` de `markdown.ts`: encabezados, párrafos, listas, énfasis y código; un `<script>` o `<img onerror>` en el texto sale escapado; un texto vacío no rompe · humo en Chrome sin cabeza: editar una sección y ver el cambio en «Vista» y en «Fuente»; descargar el `.md`»
- **S9 · U.3 No hacer:** «Una dependencia de markdown. Cambiar lo que exporta Go»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1, S2, S3 | inline (no Writer trigger: pieces depend on each other) | done | RED `layout.test.ts` (module missing), GREEN; smoke `u1` |
| T2 | S4, S5, S6 | inline | done | RED `home.test.ts` (exports missing), GREEN; smoke `u2` |
| T3 | S7, S8, S9 | inline | done | RED `markdown.test.ts` (module missing), GREEN; smoke `u3` |

## Log

- **L1 (2026-11-02):** Eber: «b39f69f approved. Your question: keep each save as one edit, no grouping. Then, in this order, one local commit each, explicit paths, no trailers, no push; make test, make vet and make web green before each commit; do not call Meta or Fly: 0. «fix: P-116 keepalive on hide»: the save fired on visibilitychange/pagehide uses fetch with keepalive: true so closing the tab does not cancel it; web test. 1. The three-panel interface: docs/construccion/backlog-ui.md v0.1, pieces U.1 → U.2 → U.3 («feat: ui.U.n»). Read pantallas.md «Disposición general (2-nov)» and its correspondence table, and use docs/diseno/bloque-3/maqueta-tres-paneles.html as the visual reference for layout, controls, shortcuts and the grid background — but keep U-3 (no section selector: the frame's skeleton is the only navigation), U-4 (one font family, no serif) and P7 (signoff stays full screen and hides the frame). Only web/ and the served bundle; no server change, no new dependency (markdown is rendered by our own minimal escaped renderer, B-97). Move existing components, do not change what they do. Create odd/tasks/ui.md as for the slices. For each piece run the headless-Chrome smoke test the backlog describes with TQ_PROVIDER=fake and attach a screenshot to the report. If something is not in the design, mark [EL DISEÑO NO LO DICE], take the smallest choice, note it in elecciones-agente.md and keep going. Final report as before.»
- **L2:** Step 0 was already committed as `2d52a92` with its web test, before `0b67759`; not repeated.
- **L3:** Base-commit differential (`0b67759`, scratch copy, same walk): the CSP log with hash `sha256-PlumsSlv…` and the 404 on `history?block=1` of an empty paragraph appear there too; not caused by this work, reported to Eber.
- **L4:** `edge.test.ts` (B3-22 row 8) forbids `innerHTML` in `web/src`, which B-97's `renderMarkdown(src): string` would need to display. U.3 keeps the security rule and renders to React nodes.
- **L5:** `0df07c1` (U.2) was committed in a chained command that did not stop on a failed smoke: a read of `stop-report` got a 401 during sign-out, with no visible effect. History is not rewritten; `1bb40a2` fixes the order, 3 of 3 clean runs. Later commits are gated on the smoke.
- **L6:** The second CSP hash, `sha256-5sxII…`, is the origin marks' `<style>` in `SectionEditor`; it also appears at `0b67759` in a local run. Reported to Eber, not changed.
