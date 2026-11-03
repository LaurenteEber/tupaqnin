---
titulo: Backlog de construcción · cierre (P0 de las pruebas con usuarios)
estado: v0.1 · 3-nov · los 8 P0 de pruebas-usuarios-2nov.md (sin el 9: el caso 5 sigue en español) con las decisiones D-1 a D-3 de Eber
relacion: implementa pruebas-usuarios-2nov.md §2 (hallazgos 1 a 8) y las decisiones B-99 a B-102. Mismo formato y mismo «Retorno» que backlog-r1.md. Va antes del despliegue público
---

# Backlog · cierre

**Respuesta:** 7 piezas, C.1 → C.7, en este orden. Al terminar, **nada de lo que haces se pierde ni queda sin efecto, y nada cambia sin que lo veas**: recargar no deja la pestaña en solo lectura ni te saca del proyecto; el gasto y lo que hace el agente están siempre a la vista; tus mensajes se encolan; tus decisiones llegan solas al texto; las correcciones del juez se muestran con lo que corrigen y, si son drásticas o rompen el texto, llegan como propuesta; y no hay ids ni jerga a la vista. **Horas: 9–13 `[inferencia]`.** Cada pieza en su commit; la que no quede en verde no se integra.

**Regla que guía esta rebanada (Eber, 3-nov, de las pruebas):** «Nada cambia sin que lo veas, y nada tuyo se pierde ni queda sin efecto». Sirve a la deuda cognitiva (siempre sabes qué cambió y por qué) y a un proceso intuitivo (no hay callejones: el siguiente paso siempre se ve o lo da la app).

## Decisiones

| # | Propuesta | En vez de | Elegiría la otra si… | Decisión de Eber |
|---|---|---|---|---|
| B-99 | **Correcciones del juez, siempre a la vista; drásticas o rotas, como propuesta** (precisa B-67). Tras el lote de corrección, la conversación muestra una tarjeta «El juez encontró n fallas · el agente corrigió m en ‹sección› · Ver cambios», y cada cambio con «Sale / Entra» junto a la falla que corrige, en las palabras del juez; lo mismo queda en la historia del párrafo. **Pasa a propuesta** (B-73, con su contraste) en vez de aplicarse, por regla de la app y sin modelo, cuando el cambio: (a) quita más de la mitad del texto de la sección `[inicial]`; (b) borra un párrafo que sostiene una decisión tuya o confirmada, o una cita vinculada; (c) deja una oración sin cerrar o inserta texto a mitad de oración. Además, una inserción cuya ancla no termina en un final de oración se rechaza y el agente reintenta (como B-11) | Aplicar todo en silencio (hoy) · toda corrección como propuesta (muchos clics; la persona apurada acepta sin leer) | Las pruebas muestran demasiadas propuestas | **Eber, 3-nov:** opción A |
| B-100 | **Tus decisiones llegan solas al texto** (sigue el criterio de B-64). Al responder una tarjeta con «la otra», un «por aclarar» que bloquea secciones, o resolver una objeción a la otra opción, la app lanza **un** paso del agente para aplicarlo **cuando la sección afectada ya no tiene otras tarjetas ni «por aclarar» tuyos abiertos** (una tanda, no un paso por respuesta). Con «mantener» no se lanza nada. La tarjeta respondida dice «El agente actualizará ‹sección› con tu decisión»; el paso dice qué decisión aplicó; lo cambiado se ve como en B-99. En lo firmado llega como propuesta (B-73). Si no terminas de responder, no se lanza; el informe de parada lo lista («Tus decisiones aún no están en el texto de ‹sección›») con «Aplicarlas». Respeta los topes (B-89) y el modo normal: un paso, no una cadena | Esperar a que le escribas al agente (hoy: callejón) · un botón «Aplicar mis decisiones» (otro clic que descubrir) | Lanzarlo solo gasta de más en las pruebas | **Eber, 3-nov:** opción A |
| B-101 | **Tus mensajes se encolan siempre** (extiende B-83 al modo normal). Con un turno en curso, el mensaje entra a la conversación al instante con «En cola · entra al siguiente paso del agente», editable y quitable mientras espera; el agente lo recibe en literal al empezar su siguiente paso (ya lee los mensajes sin respuesta); si la cadena iba a terminar, tu mensaje abre un turno apenas acaba el actual. La respuesta del agente dice «Respondo a tu mensaje: «…»» | El 409 `turn_running` que pierde el mensaje (hoy) · interrumpir el paso en curso | — | **Eber, 3-nov:** opción A |
| B-102 | **La pestaña que recargas sigue editando.** `TAB_ID` en `sessionStorage` (sobrevive a la recarga de esa pestaña; una pestaña nueva tiene el suyo; si se duplica una pestaña, la copia lleva el mismo y la primera que renueva gana, como hoy) · `DELETE /api/projects/{id}/editor` `{tab_id}` libera la edición, y la página lo llama en `pagehide` con `navigator.sendBeacon` · el aviso de solo lectura suma **«Editar aquí»**, que toma la edición si la otra pestaña no renovó en 15 s `[inicial]`; si sí renovó, explica que otra pestaña está editando | Bajar los 45 s del lease (seguiría fallando) | — | **Se infiere:** hallazgo 1, causa confirmada (`client.ts:428`, `sections.go:20`) |

## Piezas

### C.1 · La pestaña que recargas sigue editando

| Campo | Contenido |
|---|---|
| **Qué** | Recargar no deja la pestaña en solo lectura; si de verdad hay otra pestaña editando, lo dice y te deja tomar la edición |
| **Implementa** | Hallazgo 1 · B-102 |
| **Leer** | `web/src/api/client.ts` (`TAB_ID`, `claimEditor`) · `web/src/App.tsx` (renovación del lease, aviso `tab.read_only`) · `internal/server/sections.go` (`editors`, `claim`, `editorLease`) |
| **Interfaz** | `TAB_ID` leído de `sessionStorage` (`tq.tabId`) con `try/catch`; si no hay, se crea y se guarda · `editors.release(projectID, tab)` y la ruta `DELETE …/editor` (en la tabla del control de dueño, B-87) · `pagehide` → `sendBeacon` · `editors.takeOver(projectID, tab, staleAfter)` y `POST …/editor` `{tab_id, take_over: true}` · el aviso con «Editar aquí» |
| **Listo cuando** | Go: liberar y volver a reclamar desde otra pestaña da editor; tomar la edición con el otro renovado hace 20 s da editor y con el otro renovado hace 5 s no · web: el `TAB_ID` se conserva entre dos lecturas con `sessionStorage` y se crea si falla · humo en Chrome sin cabeza: abrir un proyecto, recargar 3 veces, seguir editando; abrir una segunda pestaña: solo lectura con «Editar aquí» |
| **No hacer** | Guardar el lease en la base |

### C.2 · Lo que hace el agente y lo que gasta, siempre a la vista

| Campo | Contenido |
|---|---|
| **Qué** | El gasto de la cabecera está al día, y una línea fija dice qué hace el agente o qué te toca; la tarjeta de una ronda avanza sin recargar |
| **Implementa** | Hallazgos 2 y 3 |
| **Leer** | `App.tsx` (`getSummary`, `applySummary`, `watchTurn`, el stream) · `web/src/live.ts` (`roundStarted`, `ROUND_POLL_MS`) · `RoundCard.tsx` · `ModelLine.tsx` |
| **Interfaz** | `getSummary` también tras cada evento del stream (paso, ronda, juez, firma) y cada 10 s `[inicial]` mientras haya un turno · **línea de estado** en la cabecera del centro: «El agente trabaja: ‹escribiendo «Síntesis» · leyendo fuentes (ronda) · revisando (juez) · preparando la defensa›» o «Te toca: n cosas» o nada; sale de los eventos que ya llegan · **primero diagnosticar** por qué la tarjeta de una ronda lanzada por el principal (B-64) se queda en «waiting» (Paula): la consulta cada 2 s debe arrancar también cuando la ronda la abrió un paso, no solo cuando la abrió un clic; prueba que lo fije |
| **Listo cuando** | web: el texto de la línea de estado para cada evento (prueba de función pura) · prueba de la ronda lanzada por el principal que se actualiza sola · humo en Chrome sin cabeza con el falso: el gasto sube durante el turno sin recargar |
| **No hacer** | Una ruta nueva en el servidor |

### C.3 · Recargar te deja donde estabas

| Campo | Contenido |
|---|---|
| **Qué** | Recargar (o compartir la dirección) vuelve al mismo proyecto y al mismo idioma |
| **Implementa** | Hallazgo 6 |
| **Leer** | `App.tsx` (`openProject`, `closeProject`, `lang`) |
| **Interfaz** | El proyecto en `#/p/{projectID}` (sin cambio del servidor); abrir y cerrar actualizan el hash; al cargar, si el hash nombra un proyecto tuyo se abre, si no (404) se vuelve al inicio sin error · el idioma en `localStorage` (`tq.lang`) con `try/catch` |
| **Listo cuando** | web: el hash se lee y se escribe (función pura) · humo: abrir un proyecto, recargar, seguir en él; cambiar a español, recargar, seguir en español |

### C.4 · Tus mensajes nunca se pierden

| Campo | Contenido |
|---|---|
| **Qué** | Escribir mientras el agente trabaja deja tu mensaje en cola, a la vista, y el agente lo atiende en su siguiente paso |
| **Implementa** | Hallazgo 8 · B-101 (extiende B-83) |
| **Leer** | `internal/server/conversation.go` (el 409 `turn_running`) · lo que ya hace B-83 con el modo (`cycle/mode.go`, `runner.go`) · `App.tsx` (`send`, `messageQueued`) · `MessageInput.tsx` |
| **Interfaz** | Con un turno en curso, `POST …/messages` responde 202 `{queued: true}` también en modo normal · el mensaje en cola se ve en la conversación con su marca, y se puede editar o quitar (`PUT`/`DELETE …/messages/{n}` mientras siga en cola) · si la cadena termina con mensajes en cola, se abre un turno con ellos · la respuesta del agente lleva «Respondo a tu mensaje: «…»» (en las instrucciones del principal, `principal.en.md`) |
| **Listo cuando** | Go: un mensaje con turno en curso en modo normal queda en cola y entra al paso siguiente; al terminar la cadena con uno en cola se abre un turno; quitar uno en cola no lo envía · web: la marca y quitar · las pruebas de B-83 siguen en verde |

### C.5 · Tus decisiones llegan solas al texto

| Campo | Contenido |
|---|---|
| **Qué** | Al decidir algo que cambia el texto, la app le pide al agente que lo aplique, cuando terminaste de responder lo de esa sección, y te dice que lo hará |
| **Implementa** | Hallazgo 5 · B-100 |
| **Leer** | `internal/cycle/defense.go` (`afterYourAnswer`, hoy solo prepara defensas) · `internal/cycle/card.go` y `openquestion.go` · B-64 en `backlog-r3.md` (cómo se lanza un turno sin clic) · estados §1 («escrita con lo anterior») |
| **Interfaz** | `afterYourAnswer` suma: si la respuesta cambia el texto (B-100) y la sección ya no tiene pendientes tuyos abiertos, toma el turno y corre **un** paso con la tarea «aplica tu decisión ‹dec› en ‹sección›»; si el turno está ocupado, queda en cola como un mensaje (C.4) · la tarjeta respondida muestra «El agente actualizará ‹sección› con tu decisión» · el informe de parada lista las decisiones aún no aplicadas con «Aplicarlas» |
| **Listo cuando** | Go: «la otra» con la sección sin más pendientes lanza un paso y la sección deja de estar «escrita con lo anterior»; con otra tarjeta abierta en la sección no lanza nada; «mantener» no lanza; en una sección firmada el paso deja una propuesta · recorrido con el falso del caso 5: responder la tarjeta del piloto con «excluirlo» y ver el texto actualizado |
| **No hacer** | Un paso por respuesta. Abrir una cadena |

### C.6 · Las correcciones del juez, visibles y seguras

| Campo | Contenido |
|---|---|
| **Qué** | Cada corrección se muestra con lo que corrige; la drástica o la que rompe el texto llega como propuesta |
| **Implementa** | Hallazgo 4 · B-99 (precisa B-67) |
| **Leer** | `internal/cycle/judge.go` (lote de corrección) · `internal/cycle/apply.go` y `proposal.go` (anclas, `insert_after`, aceptar) · `JudgeCard.tsx`, `ProposalPanel.tsx`, `HistoryPanel.tsx` |
| **Interfaz** | **Primero diagnosticar**, con `data/tupaqnin.db` abierta en solo lectura (`sqlite3 -readonly`; no se modifica): (1) el paso del lote de corrección que dejó «Scope and influent microplastics» en una frase (proyecto de `ux-p1`); (2) la propuesta aceptada que insertó «…cálculo previo de 60 veces que no es comparable…» a mitad de oración (proyecto sin cuenta del jurado, caso 5, «Síntesis»); y reportar la causa antes de cambiar · **después:** la regla de B-99 en `apply.go` al aplicar el lote (sin modelo): (a) más de la mitad, (b) párrafo con decisión tuya o confirmada o cita vinculada, (c) oración sin cerrar o inserción a mitad de oración → propuesta · el rechazo del ancla que no termina en final de oración · la tarjeta «El juez encontró n fallas · corrigió m · Ver cambios» con «Sale / Entra» por falla |
| **Listo cuando** | Go: un lote que quita el 60 % queda como propuesta; uno que inserta a mitad de oración se rechaza y reintenta; uno pequeño se aplica y deja su «Sale / Entra» con la falla · web: la tarjeta de cambios · la tabla de frenos de la 4.6 sigue pasando (6B usa el lote) |
| **No hacer** | Pedir tu sí para toda corrección. Un modelo que juzgue qué es drástico |

### C.7 · Sin ids ni jerga a la vista

| Campo | Contenido |
|---|---|
| **Qué** | La interfaz habla en lenguaje llano, en un solo idioma, sin códigos internos |
| **Implementa** | Hallazgo 7 · regla 7 · la lista de `pruebas-usuarios-2nov.md` §3 |
| **Leer** | `web/src/i18n/en.json` y `es.json` · los componentes que muestran `oq-`, `rnd-`, `dec-` · `p116.test.ts` (claves en los dos catálogos) |
| **Interfaz** | Reescribir cada texto de la lista de §3 en lenguaje llano (en y es); quitar todo id a la vista; las notas del agente sin monoespaciado; «Evidence from: falta material» y demás mezclas, en el idioma de la interfaz · el pie «Decisions are made in their cards…» pasa a «Las decisiones críticas se toman en sus tarjetas» |
| **Listo cuando** | web: prueba que falle si un texto de la página tiene `oq-n`, `rnd-n`, `dec-n`, `card-n`, `sec-n` o `cit-n`; prueba de que cada texto de §3 cambió · humo: recorrer el caso 5 en inglés y en español sin ids ni mezclas |

## Horas `[inferencia]`

C.1: 1,5–2 · C.2: 1,5–2 · C.3: 0,5–1 · C.4: 1–1,5 · C.5: 1,5–2 · C.6: 2–3 · C.7: 1–1,5. **Total: 9–13 h.**

## Retorno de cada pieza

El de `backlog.md`, más una captura de la prueba de humo. C.2 y C.6 reportan su diagnóstico antes del arreglo.
