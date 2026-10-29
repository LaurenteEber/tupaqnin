---
titulo: Cómo orquesta gentle-pi y qué implica para tupaqnin
estado: análisis de referencia (25-oct-2026), insumo de la decisión transversal del bloque 1 y del módulo 6
fecha: 2026-10-25
relacion: verifica las citas de gentle-pi del doc 12; propone la variante B que adopta provisionalmente bloque-1-nucleo.md
---

# 13 · Cómo orquesta gentle-pi y qué implica para tupaqnin

*Referente: gentle-pi (gentle-shell), HEAD 64b2fe4 (25-oct-2026). Las citas del doc 12 eran contra 51b766d (24-oct). Entre ambos hay 26 commits. Ninguno toca `assets/`, `skills/`, `README.md`, `lib/agents-config.ts` ni los `odd/tasks` citados (`git diff --quiet 51b766d HEAD`). Sí cambian `lib/context-backstop.ts` (muestreo cada 5 s), `docs/delegated-verification.md` (+4 líneas al inicio) y `docs/readme-reference.md` (+2 líneas, después de la línea 1250).*

*Etiquetas: `[fuerte]` `[moderada]` `[débil]` `[práctica]` (observado en gentle-pi o en los casos) `[inferencia]`.*

---

## 1. Cómo orquesta gentle-pi

### 1.1 Respuesta corta

**El padre es mixto.** Su identidad dice «coordinador», pero sus reglas y su runtime lo hacen ejecutor en línea por defecto. Pasa a orquestador solo cuando una razón nombrada se cumple. La razón más frecuente en trabajo largo es el umbral de contexto medido. `[práctica]`

| Capa | Qué dice | Cita |
|---|---|---|
| Identidad | «You are a COORDINATOR, not the default executor for substantial work. Maintain one thin conversation thread, delegate real phase work…» | `assets/orchestrator.md:13` (texto presente desde 94b55fe0d, 11-may-2026) |
| Arnés siempre activo | «Use subagents when available for exploration, planning, implementation, and review, while keeping one parent session responsible for orchestration» | `extensions/gentle-ai.ts:1471` (`buildGentlePrompt`, 1426-1477) |
| Reglas de decisión | «no trigger holds by default, and when in doubt work inline» | `assets/orchestrator.md:53`; `assets/orchestrator-delegation.md:18`; paso 6 de `contracts/…/v1.4.0/bundle/routing/pi.md:12` |
| Runtime | Rechaza lanzar un escritor si no hay documento de tarea o umbral de contexto; rechaza el verificador si no hay ítem de riesgo | `lib/bounded-writer-admission.ts:92,121-129`; `lib/verifier-admission.ts:14-28` |

La retórica de «coordinador» convive con una mecánica de «en línea primero». Lo que decide es la mecánica: el runtime **impide** delegar sin razón. `[práctica]`

### 1.2 Qué hace en línea y qué delega

| Acción | Quién | Cita |
|---|---|---|
| Leer para decidir, dentro de un lote de ≤ 3 llamadas y ~10k tokens | Padre | `orchestrator-delegation-reference.md:48` |
| Leer más que eso, o > ~5 búsquedas seguidas | Un explorador de solo lectura; traspaso ≤ ~2k tokens con `path:line` | `orchestrator-delegation.md:21`; `agents/gentle-ai-explore.md:30` |
| Leer como preparación de una escritura | Quien escribe (nunca se explora lo que el padre leerá igual) | `orchestrator-delegation.md:32`; `routing/pi.md:8` |
| Escribir una tarea chica (cualquier número de archivos) | Padre | `orchestrator.md:39`; `orchestrator-delegation-reference.md:51` |
| Escribir una tarea grande **sin razón** | Padre, siguiendo la bitácora | `orchestrator-delegation-reference.md:52`; `orchestrator.md:59` |
| Escribir una unidad **con razón** | Un escritor acotado por unidad | `orchestrator-delegation-reference.md:53` |
| Prueba enfocada y suite del cambio | Quien hizo el cambio, en línea | `orchestrator-delegation.md:22` |
| Verificación independiente | Solo con ítem de riesgo alto nombrado (`Risk: item N`) | `orchestrator-verification.md:16`; `lib/verifier-admission.ts:14` |
| Hablar con el humano, decisiones de producto, git, revisión | Siempre el padre | `agents/gentle-ai-worker.md:16`; `orchestrator-tracking.md:19` |

**Razones para un escritor** (`orchestrator-delegation.md:24`; `orchestrator.md:59`):
- **(a) Paralelismo.** Dos o más unidades independientes, con superficies disjuntas, cada una de varios minutos (varios archivos o ~100+ líneas), lanzadas juntas en segundo plano. Si no se pueden lanzar juntas, en línea.
- **(b) Contexto.** El runtime avisa que el contexto medido del padre cruzó el umbral (§2.2).
- **Nunca** por tamaño, número de archivos ni por tener un modelo más barato (`orchestrator-writer.md:3`; `delegate-for-reason.md` T24, línea 90).
- Unidades de ≤ ~100 líneas siempre en línea: «writing them costs less than starting workers» (`orchestrator-delegation.md:24`). Un subagente gasta 1–2 min leyendo antes de escribir (`orchestrator.md:59`).

### 1.3 ¿Existe un rol «writer»?

Sí: `gentle-ai-worker` (`assets/agents/gentle-ai-worker.md`) y su módulo de traspaso `assets/orchestrator-writer.md`, que el doc 12 no citaba.

| Rasgo | Detalle | Cita |
|---|---|---|
| Para qué | Trabajo acotado «too large for the parent to execute inline» | `gentle-ai-worker.md:16` |
| Quién es dueño | «The parent remains the orchestrator and owns user interaction, review, and terminal git actions» | `gentle-ai-worker.md:16` |
| Superficies | El padre deriva `## Allowed edit surfaces`; el humano nunca escribe rutas | `orchestrator-writer.md:9-24` |
| Invariantes | El prompt del escritor nombra los helpers que debe reusar | `orchestrator-writer.md:5` |
| Memoria | «Do not read persistent memory for context. The parent selects and forwards» | `gentle-ai-worker.md:33` |
| Cuando necesita al humano | Devuelve `interaction_required` con opciones cerradas; el padre las releva sin resumir | `gentle-ai-worker.md:87-95`; `orchestrator-prompts.md:11-16` |
| Retorno | Esquema fijo: `status`, `summary`, `files_changed`, `tdd_evidence`, `validation` (comando: resultado), `risks`, `review_focus` | `gentle-ai-worker.md:99-122` |
| Continuación | `subagent_continue` reanuda la **misma sesión** del hijo (`--session <ruta>`) y hereda sus superficies | `lib/agents-runner.ts:330`; `spec-by-reference-handoffs.md` S8 |

### 1.4 Quién escribe y quién explica

- **Escribe** el padre en el caso normal; el escritor cuando hay razón.
- **Explica al humano siempre el padre.** Sintetiza «decision, outcome, next action» (`orchestrator.md:15`) y cierra con «the verified outcome, every failed, skipped, or pending check, and the next step» (`routing/pi.md:13`).
- Cuando delegó, el padre **explica de segunda mano**, sobre el resumen del escritor. Lo compensa con evidencia determinista: re-ejecuta un comando reportado (*spot check*, `orchestrator-verification.md:16`) y corre `assess` sobre el diff real, no sobre el resumen (`orchestrator-verification.md:7`; `delegate-for-reason.md` S6).
- **Excepción que importa a tupaqnin:** lo que un subagente dirige al humano (un menú, una pregunta) se releva **sin resumir, reordenar ni reetiquetar** (`orchestrator-prompts.md:11-13`).

`[práctica]`

---

## 2. Cómo protege el contexto del padre

### 2.1 Mecanismos

| Mecanismo | Cifra o umbral | Cita |
|---|---|---|
| Presupuesto de evidencia en línea | ≤ 3 llamadas, ~10k tokens; o ≤ ~5 búsquedas seguidas | `orchestrator.md:33,56`; `orchestrator-delegation.md:21` |
| Retorno acotado de lectores | Explorador y verificador: ≤ ~2k tokens | `gentle-ai-explore.md:30`; `gentle-ai-verify.md:44` |
| Informes grandes fuera del hilo | `outputMode: "file-only"`; el padre resume solo decisiones, bloqueos y rutas | `orchestrator-delegation-reference.md:142` |
| Traspaso por referencia | `Spec: odd/tasks/<f>.md (read until "## Log"). Do T#; S#.`; nunca traducir, condensar ni parafrasear los requisitos | `orchestrator-delegation.md:7`; `routing/pi.md:12` |
| No pasar contenidos | «pass paths, the spec reference, and what to change, not file contents» | `orchestrator-delegation.md:32` |
| Estado en un documento | Un documento por tarea grande: cabecera, `## Specs` (literal), `## Tasks`, `## Log` (L1 = pedido literal); edición dirigida, nunca reescritura | `orchestrator-memory.md:7-14` |
| Espejo en Engram | Copia completa del documento, refrescada por script para no reemitirlo como tokens de salida | `orchestrator-memory.md:14,18` |
| Retomar | `mem_context` → `mem_search` → `mem_get_observation` → archivo real; reconciliar | `orchestrator-memory.md:20`; `routing/pi.md:17` |
| Prompt siempre activo con tope de bytes | 8 192 → 8 400 → 9 100 → **9 250 B** (subido por decisión del autor con cada regla medida) | `tests/orchestrator-budget.test.ts:35-39` |
| Módulos perezosos | El delegador monolítico de ~49 KB se partió en módulos que solo se cargan cuando su mecanismo se activa; el camino chico no lee ninguno | `proportional-task-routing.md` S7, L18 |
| Contexto magro de los hijos | Se filtran del hijo los bloques solo-para-orquestador: prefijo de 86,8k → 56,9k (−30k, −34 %) por delegación | `lean-delegation-context.md` (Progress, «LIVE») |
| Umbral de contexto medido (*context backstop*) | §2.2 | `lib/context-backstop.ts` |
| Salida de comandos acotada | Conteos, `--stat`, `tail` | `orchestrator.md:61` |

### 2.2 El umbral de contexto (*context backstop*)

- **Fórmula:** `min(150 000, 0,15 × ventana del modelo)` (`lib/context-backstop.ts:9-12,38-42`).
  - Ventana de 272k → ~41k. Ventana de 1M o más → 150k (`docs/readme-reference.md:730`).
  - **Para Spark 1.3 (1M, doc 12 §9.1) daría 150k, no 64k.** `[práctica]` sobre la fórmula; aplicarla a Spark es `[inferencia]`.
- **Lo mide el runtime, no el modelo.** Antes era «~150k» en el prompt y el modelo tenía que estimarlo; una revisión lo marcó como imposible de observar (`lean-delegation-context.md` Progress T2, advertencia R3; `context-backstop.md` S1-S3).
- **Aviso:** se agrega una vez al siguiente resultado de herramienta: tokens medidos, % de la ventana, umbral e instrucción («delegate the next bounded unit that needs several steps… single small edits and state checks stay inline») (`lib/context-backstop.ts:51-73`).
- **Rearme:** cuando el contexto baja del umbral, por ejemplo tras compactar. Tokens desconocidos (justo después de compactar) no cuentan (`lib/context-backstop.ts:44-47,56-73`).
- **Muestreo:** cada 5 s por sesión, porque medir cuesta O(sesión). Es lo nuevo desde 51b766d (`lib/context-backstop.ts:15-17,82-106`; commits 11cff4bbd, 8fb244dd0).
- **Es también llave de admisión:** pasado el umbral, el runtime admite escritores y verificadores aunque no haya documento de tarea ni ítem de riesgo (`bounded-writer-admission.ts:129`; `verifier-admission.ts:28`).
- **De dónde sale 0,15:** comentario del código: «on the wide five-feature task, delegating at ~15% of a 272k window cost about 20-40% less and ran ~30% faster than never reaching the backstop, while small tasks never reached it» (`lib/context-backstop.ts:10-11`; commit 3facda23a, 15-oct). El n no está en el repo. `[débil]`

### 2.3 Lo que gentle-pi **no** hace

- **No reinicia el contexto en una frontera.** Usa la compactación de Pi más el documento de tarea y Engram para retomar. El umbral se rearma tras compactar. `[práctica]`
- **No mide la calidad del padre en sesiones largas.** La «fase 2» que lo mediría está planificada, no hecha: «long-horizon task with compactions… peak parent context» (`delegate-for-reason.md` L84). `[práctica]`

---

## 3. Evidencia medida frente a afirmaciones sin medir

### 3.1 Medido

| Qué | Resultado | n | Fuente |
|---|---|---|---|
| Ruteo proporcional (#1494): bug de 3 líneas | 335 s · 56 turnos · 0,339 USD → **77 s · 10 turnos · 0,105 USD**, en línea, sin subagentes | 1 por celda | `README.md` §Measured; `proportional-task-routing.md` L23, L25 |
| Bug en lógica compartida | 479 s · 0,361 → 276 s · 0,207, con un explorador | 1 | ídem |
| Migración de datos (riesgo alto) | 559 s · 0,403 (con ronda de corrección) → 539 s · 0,555, aceptada en la primera ronda, con un verificador | 1 | ídem |
| Spec de 73 líneas, 26 pruebas ocultas | Parafraseado: 13 fallando, 23/26 tras dos rondas. Por referencia: el README solo dice «Spec passed by reference (#1718)», sin cifra | 1 | `README.md` §Measured |
| Delegación forzada por pregunta | ~140k tokens ponderados; **+46–65 % de tokens y ~3× el tiempo** en preguntas puntuales, sin ganar calidad | no consta («controlled runs») | `lean-delegation-context.md:19-20`; estudio completo en gentle-ai#5139 (no leído) |
| Delegación selectiva (presupuesto de evidencia) | Costo neutro (±2 %) con ~27k tokens menos en el padre | no consta | `lean-delegation-context.md:21` |
| Prefijo de los hijos | −30k (−34 %) por delegación al filtrar bloques del orquestador | medición en vivo, 2 hijos | `lean-delegation-context.md` Progress |
| Delegar implementación a un modelo más barato (brazo R) | Nunca compensó: B 1,59 USD frente a R 3,12 en x1+x2+x5+x4; R-xh x4: 97 min, 3,65 USD. Se apagó la razón de costo | 1 por celda | `delegate-for-reason.md` T24 (línea 90), L17, L19, L49, L51 |
| Primera versión de «delegar por razón» | Empeoró todos los brazos (B 1,463 → 2,594 USD) por tres fallos: riesgo sin forzar, delegación por partes en serie, unidades «paralelas» lanzadas en serie | 1 | `delegate-for-reason.md` L14, L17 |
| Verificador v2 (sondas derivadas de la spec) | Recall 8/9 defectos conocidos, 0 falsos positivos altos en 4 controles limpios. El verificador previo halló 0 | 10 corridas ciegas | `delegate-for-reason.md` L30 |
| Frente a Codex con el mismo modelo, revisión ciega | Gentle por encima en chico, mediano y grande; tiempo ~1,3× en chico y mediano, **~3× en grande** | 1–3 | `delegate-for-reason.md` L78; `README.md` §Measured |
| Sin tope de correcciones | Una corrida hizo ~6 rondas verificar/corregir y agotó 60 min | 1 | `delegate-for-reason.md` L45 |
| Umbral a 0,15 | 20–40 % menos costo, ~30 % más rápido en la tarea ancha de cinco funciones | no consta | `lib/context-backstop.ts:10-11` |

Todas son corridas del propio autor sobre su banco privado (`~/work/gentle-shell-bench`). `[práctica]`, `[débil]` como generalización.

### 3.2 Lo que se retiró y por qué

| Fecha | Qué | Por qué | Cita |
|---|---|---|---|
| 19-sep | Puerta de runtime que **obligaba** a delegar al segundo archivo no trivial | Un modelo implementó varios archivos en línea pese al prompt | `odd/tasks/odd-runtime-delegation-gate.md` (commit e70c95423) |
| 22-sep | Se retiró esa puerta | «forces an expensive worker for work that does not need one»; el runtime no distingue una edición mecánica de una consecuente | `odd/tasks/odd-proportional-delegation.md` |
| 24-sep | Se quitó SDD; ODD queda como único flujo | Decisión del autor; alineado con el retiro en gentle-ai | `odd/tasks/remove-sdd-odd-only.md` |
| 30-sep | Disparadores por conteo («4-file rule», «~20 tool calls») → presupuesto de evidencia | Con las reglas viejas: 0 delegaciones en corridas controladas; forzar delegación costaba +46–65 % | `lean-delegation-context.md:19-22,46-48` |
| 01-oct | gentle-ai v4.0.0: se elimina el esquema de consentimiento de `sdd-integration` | — | `pin-gentle-ai-4.0.0-release.md` («Contract diff») |
| 03-oct | «Large task → one worker per task» → delegar solo por razón; tamaño por comprensión, riesgo y prueba de reanudación | Banco de #1494 y #1731 | `proportional-task-routing.md` S2-S4; `delegate-for-reason.md` S1, AC1 |
| 04-oct | Razón de costo (cociente de precios ≥ 3×) apagada | Nunca compensó (ver 3.1) | `delegate-for-reason.md` T24 |
| 15-oct | «~150k» en el prompt → umbral medido por el runtime | El modelo no ve su propio contexto | `context-backstop.md` |

**Sobre «COORDINATOR, not an executor» en gentle-ai v4.0.0:** no hay copia de gentle-ai en este entorno. `internal/assets/retired_workflow_ratchet_test.go:17-19` queda `[no verificado]`. Lo verificable en gentle-pi: se retiró SDD y el delegar por defecto; la etiqueta «COORDINATOR» sigue en `orchestrator.md:13`, matizada como «not the default executor for substantial work».

### 3.3 Afirmado sin medir

- Que el padre no se degrada porque relee el documento. Nada lo mide; el umbral de contexto existe justo porque el contexto sí pesa.
- Que un orquestador con varios frentes rinde mejor que agentes separados (fase 2, `delegate-for-reason.md` L84).
- Que el modelo cumple el protocolo: «Static prompt tests and scripted hook checks prove instruction delivery, not autonomous model adherence» (`docs/readme-reference.md:92`).
- Que el modo en segundo plano ayuda: «the difference is within noise» (`delegate-for-reason.md` L82); en tareas secuenciales «background did not help» (L74).

---

## 4. Verificación de las citas del doc 12 sobre gentle-pi

✓ se sostiene · ◐ en parte · ✗ no se sostiene · ? no verificable aquí.

| Cita del doc 12 | En 51b766d | En HEAD | Nota |
|---|---|---|---|
| §1: bug de 3 líneas 335 s → 77 s en línea (`README.md` §Measured, n=1) | ✓ | ✓ | Sin cambios |
| §1: spec parafraseada, 13 de 26 fallando; «pasada por referencia, las aprobó todas» (PR #1718) | ◐ | ◐ | El README da «13 failing, still 23/26»; para la versión por referencia no da cifra |
| §1: «en gentle-pi el padre no se degrada porque el estado vive en un documento que relee» (`orchestrator-memory.md`) | ✗ | ✗ | El archivo describe el documento y cómo retomar. No afirma ni mide que el padre no se degrade. El documento existe solo en tareas grandes. Es `[inferencia]`, no `[práctica]` |
| §2.1: vuelve a clasificar cuando algo cambia (`orchestrator.md:53`) | ✓ | ✓ | «When it resolves, re-evaluate task size» |
| §2.2: los conteos nunca clasifican (`orchestrator.md:37`) | ✓ | ✓ | |
| §2.3: prueba de reanudación (`orchestrator.md:31-37`) | ✓ | ✓ | |
| §2.3: ante la duda, se sube la consecuencia (`orchestrator.md:41`) | ✓ | ✓ | Adaptación legítima: «unclear» cuenta como alto solo si una mirada acotada no lo resuelve |
| §4.4: pausa solo lo que depende de lo que falta (`orchestrator-tracking.md:21`) | ◐ | ◐ | La línea habla de investigación declinada: «pause only unsafe decisions dependent on it». El doc 12 lo generaliza |
| §5.1: gentle-ai retiró «COORDINATOR, not an executor» en v4.0.0 (`retired_workflow_ratchet_test.go:17-19`) | ? | ? | gentle-ai no está en el entorno. En gentle-pi la etiqueta sigue (`orchestrator.md:13`, desde may-2026) |
| §5.1: lo retiró «tras medir» +46–65 % y ~3× (`lean-delegation-context.md:19-20`, n pequeño) | ◐ | ◐ | Las cifras están en esas líneas. El vínculo causal con el retiro no: ese trabajo (30-sep, #1587) es posterior al retiro de SDD en gentle-pi (24-sep) y trata los disparadores del prompt de gentle-pi. El n no consta |
| §5.1: «hoy los dos repos dicen “when in doubt the parent works inline”» (gentle-ai `README.md:154`) | ◐ | ◐ | En gentle-pi ✓ (`orchestrator.md:53`; `orchestrator-delegation.md:18`). gentle-ai `[no verificado]`. Omite que el mismo archivo dice «You are a COORDINATOR» (`:13`) |
| §5.2 D1: presupuesto de evidencia (`orchestrator-delegation.md:21`) | ✓ | ✓ | |
| §5.2: el tamaño nunca dispara una delegación (`delegate-for-reason.md:6`) | ✓ | ✓ | S1 |
| §5.2: un modelo más barato no es razón (`delegate-for-reason.md`, L51) | ✓ | ✓ | L49 (usuario) y L51 (T24) |
| §5.2: «llegar al presupuesto de contexto **no** es razón para delegar» | — | — | Sin cita, pero **contradice al referente**: en gentle-pi el umbral de contexto es una de las dos razones del escritor (`orchestrator.md:59,61`; `orchestrator-delegation.md:24(b),26`) y abre la admisión (`bounded-writer-admission.ts:129`) |
| §5.2: las comprobaciones de admisión eximen a los de solo lectura (`readme-reference.md:722-724`) | ✓ | ✓ | |
| §6.3: ~6 rondas, tope de 60 min (`delegate-for-reason.md`, L45) | ✓ | ✓ | |
| §6.3: Judgment Day (`skills/judgment-day/SKILL.md`) | ✓ existe | ✓ existe | Detalle del contenido no reverificado aquí |
| §7: seis ítems de riesgo (`orchestrator.md:41`) | ✓ | ✓ | |
| §8: *Allowed edit surfaces* = admisión, no guardia de escritura (`readme-reference.md:720`) | ✓ | ✓ | |
| §8: scratch de sesión que borra el runtime (`readme-reference.md:734-742`) | ✓ | ✓ | 734-738 describen el scratch |
| §9.6: gentle-pi dice que las pruebas no cubren la conducta del modelo (`readme-reference.md:92`) | ◐ | ◐ | La línea habla de ODD (crear, actualizar, retomar), que incluye el espejo de Engram; no de Engram en general |
| §10.1: no subir la verificación por un modelo pequeño (`delegated-verification.md:26`) | ✓ | ✓ en **:30** | Corrimiento de 4 líneas por la nueva sección «Background capability admission» |
| §10.2: resolución por capas (`lib/agents-config.ts:353-366`) | ✓ | ✓ | |

---

## 5. Variantes para tupaqnin y recomendación

### 5.1 La diferencia que importa

En gentle-pi, explicar de segunda mano es tolerable: el producto se comprueba con pruebas y diffs, y el padre re-ejecuta un comando. En tupaqnin el producto es prosa. La comprobación es la comprensión del humano. DEC-003 §2 exige que quien propone explique, con contraste. `[inferencia]`

Pero el bloque 3 ya ata la explicación al escritor: `edit_section` exige el contraste en la misma llamada, y el humano ve el diff literal (bloque 3 §8). **La explicación inicial la escribe siempre el autor de la escritura, sea cual sea su rol.** El riesgo DEC-002 que queda es otro: **la defensa en el turno siguiente**, cuando el humano pregunta «¿por qué A y no B?». `[inferencia]`

gentle-pi ofrece dos piezas para eso, ambas `[práctica]`:
- relevo literal de lo que el subagente dirige al humano (`orchestrator-prompts.md:11-13`);
- continuación que reanuda la sesión del mismo hijo (`lib/agents-runner.ts:330`).

### 5.2 Variantes

| | **V0 · Doc 12** | **A · Orquestador por umbral** | **B · Orquestador de sesión + conductor de tramo** | **C · Orquestador puro con redactor por turno** |
|---|---|---|---|---|
| Idea | El principal conduce y coescribe; delega lecturas y juicio (D1–D5); se reinicia en frontera | Como V0; al cruzar el umbral medido, la siguiente unidad de varios pasos va a un redactor con traspaso por referencia | El principal orquesta y nunca escribe el documento. Cada tramo lo conduce un agente que escribe, explica y defiende; muere al cerrar el tramo | El principal habla con el humano y coordina; cada escritura la hace un redactor nuevo |
| Escribe el texto | Principal | Principal; redactor tras el umbral | Conductor del tramo | Redactor de turno |
| Habla con el humano | Principal | Principal; el redactor por relevo literal y continuación | En construcción, el conductor. Entre tramos (triage, rondas, avance neto, «qué sigue»), el orquestador. Cada mensaje lleva el rol | Principal; la tarjeta del redactor, relevada |
| Defiende la propuesta | Su autor | Autor, por continuación | Su autor (el conductor sigue vivo todo el tramo) | Principal de segunda mano, o un redactor nuevo que no la escribió |
| Contexto del principal | Paquete 20–30k + sección + historial del tramo; objetivo ~64k (doc 12 §9.3–9.4) | Igual; el umbral lo acota | Orquestador: estado, índice, encuadre del tramo y digestos, ~10–15k, y crece solo con transiciones `[inferencia]`. Conductor: paquete del tramo 20–30k + sección + historial del tramo | Chico, pero recibe un resumen por turno |
| Costo y latencia | Base: ~1,7 USD/h en Spark `high` (doc 12 §10.1) `[inferencia]` | Base hasta el umbral; cada delegación suma un arranque (gentle-pi: 1–2 min leyendo, `orchestrator.md:59`) | ≈ base + 5–10 %: una llamada del orquestador por transición y un paquete por tramo, con caché de prefijo `[inferencia]` | Dos saltos en cada turno de construir. Lo medido en gentle-pi va en contra: +46–65 % de tokens y ~3× el tiempo al delegar por pregunta; brazo R a 2× el costo |
| Riesgo principal | El contexto crece entre tramos si no hay frontera; es la preocupación de Eber | El autor cambia a mitad de tramo y el humano pierde a su interlocutor; con 1M de ventana, la fórmula de gentle-pi da 150k | Dos voces; mensajes de borde que nadie sabe a quién mandar; el orquestador decide el triage, que la regla de proponer y explicar también le alcanza | Rompe «quien propone explica»; latencia en el turno más frecuente |
| Qué demuestra gentle-pi | Ejecutar en línea por defecto y delegar con razón medida | **Esto es gentle-pi**: en línea + umbral medido como razón y llave de admisión | Nada directo; no midió una división por frentes (fase 2 pendiente) | Lo midió y lo retiró |
| En el hackathon | Doc 12 §11 | Nada extra: el campo de rol en el registro (§6) | El conductor = el principal del doc 12 (mismo código), con el rol `conductor`. El orquestador es **código de la app**: elige el tramo, arma el paquete, reinicia al cerrar el tramo. El triage lo da el conductor al abrir el tramo, como en el doc 12. Sin LLM orquestador | No |
| Hoja de ruta | — | Umbral medido por la app; aviso al principal; redactor con tarjeta y continuación | Orquestador LLM para rondas, «continúa hasta terminar» y varios tramos; umbral medido (A) dentro del conductor como válvula | — |

### 5.3 Recomendación

**Recomiendo B, construido en el hackathon como V0 más el rol, y con la válvula de A en la hoja de ruta.**

Por qué:
1. **Cumple lo que pidió Eber sin romper DEC-002.** El agente que acumula conversación muere en cada frontera de tramo por construcción, no por un umbral. El que persiste, el orquestador, nunca carga texto del documento. Quien escribe sigue siendo quien explica y defiende. `[inferencia]`
2. **Es barato.** El turno más frecuente, construir, sigue siendo un solo salto. La orquestación ocurre en las transiciones, que son pocas. `[inferencia]`
3. **No cambia el núcleo del hackathon.** El memo de la demo es un solo tramo; ahí B y V0 son idénticos. Solo cambian los nombres de rol y lo que registra la app.

**Lo que gentle-pi demuestra** `[práctica]`:
- Ejecutar en línea por defecto y delegar solo con razón cuesta menos que delegar por tamaño o por pregunta, sin perder calidad (n=1 por celda).
- El traspaso por referencia supera a la paráfrasis.
- El umbral lo mide el runtime, no el modelo.
- Delegar se controla con admisión en código, no con el prompt.
- El padre releva sin resumir lo que un hijo dirige al humano.

**Lo que es inferencia mía** `[inferencia]`:
- Que dividir por tramo protege el contexto mejor que el reinicio en frontera del doc 12. En realidad son el mismo mecanismo con otro nombre. B gana sobre todo en claridad de roles y en que el agente persistente nunca carga texto, no en tokens.
- Que dos voces no confunden al humano. Hay que probarlo en la Fase 0.

**Lo que la evidencia de gentle-pi no apoya:** C. Un orquestador «que solo coordina» en cada turno es lo que gentle-pi abandonó. Si Eber entiende «orquestador» como C, la evidencia de su referente va en contra.

**Lo que obliga a corregir el doc 12:** §5.2 dice que llegar al presupuesto de contexto no es razón para delegar. En gentle-pi lo es. Con B se resuelve sin delegar la redacción: el corte natural es la frontera de tramo. Si un tramo solo supera el objetivo, se aplica A dentro del conductor (→ módulo 6).

---

## 6. Consecuencias para el módulo 1 (expediente)

Las reglas del expediente no deben depender de qué rol escribe. Dependen de **qué herramienta usó** y de **quién conversó con el humano sobre la propuesta**.

| Tema | Regla para el módulo 1 | Fallo que evita | ¿Depende del rol? |
|---|---|---|---|
| **Autor de una escritura** | `author_kind = agent` + **rol** (`orquestador`, `conductor`, `redactor`, `lector`, `juez`, `contraste`) + **id de la instancia** (la sesión de agente concreta) + modelo y esfuerzo | No saber quién debe defender una propuesta tras un reinicio; atribuir al «agente» lo que escribió una instancia ya cerrada | No: el rol es dato, no condición |
| **Quién puede proponer registros** (`propose_record`, `edit_section`) | Solo un rol que conversa con el humano sobre esa propuesta: hoy, el conductor (o el principal en V0) | Un lector o un juez no puede explicar ni defender (DEC-003 §2). En gentle-pi el verificador nunca escribe: devuelve pruebas para que el escritor las confirme (`gentle-ai-verify.md:30,38`) | Depende de la **herramienta habilitada**, no de ser «principal» |
| **Hallazgos de lectores o jueces** | Entran como proceso (digesto). Se vuelven propuesta solo si un rol que propone los adopta, con su propia autoría y contraste | Que una conclusión de un subagente llegue al expediente sin nadie que la defienda | No |
| **Propuesta aceptada** | Autor agente (rol + instancia) + humano que la aceptó + id de la llamada (bloque 3 §3, provisional) | Atribuir al humano un texto que no escribió (DEC-002) | No |
| **Defensa de una propuesta** | Una pregunta sobre una propuesta pendiente va a la instancia autora si sigue viva. Si murió (frontera, reinicio), la contesta una instancia nueva **con la tarjeta de la propuesta en literal**, y la app marca «defensa por relevo» | Explicar de segunda mano sin saberlo (doc 12 §9.3, estado en vuelo) | Marcado: en V0 el autor casi siempre es el principal; en B, el conductor |
| **Revocación** | Solo el humano, o lo que él adopta con un acto. El agente, de cualquier rol, solo propone una revocación (bloque 3 §3) | Reversión silenciosa del caso 01 | No |
| **Criticidad** | Cualquier rol que propone puede subirla, nunca bajarla (bloque 3 §8). En gentle-pi `escalate` también solo sube (`proportional-task-routing.md` S6) | Bajar el riesgo para saltarse el acto | No |
| **Estado en vuelo** (doc 12 §9.3: «último turno de propuesta») | Se guarda por propuesta y autor, no «del principal» | Perder al defensor en un cambio de rol | **Sí en el doc 12**: generalizar «principal» a «autor de la propuesta» |

**Conflictos con el bloque 3 (A.2, `CHANGESET`):**
- No tiene campos para rol, instancia, modelo ni esfuerzo. Hay `author_kind` y `tool_call_id`. La regla de «defensa» necesita al menos el id de instancia. Precedente: gentle-pi registra en runtime el modelo y el esfuerzo de cada mutación pendiente (`docs/delegated-verification.md:30`). → Pedido al bloque 3, sin rediseñar su almacén.
- **Choque de nombres:** `principal_id` en `CHANGESET` parece el identificador de la persona. «Principal» también es el agente. Conviene renombrar uno de los dos antes de cerrar el módulo 1.

**En el hackathon:** rol y id de instancia en cada escritura del agente; solo el conductor (o el principal) tiene `propose_record` y `edit_section`; lectores y jueces sin herramientas de escritura (ya lo dice el doc 12 §5.1). La «defensa por relevo» va a la hoja de ruta: en la demo no hay reinicios.

---

## 7. No verificado

- gentle-ai no está en el entorno: `README.md:154`, `internal/assets/retired_workflow_ratchet_test.go:17-19`, el estudio gentle-ai#5139 y el texto actual del orquestador en gentle-ai.
- El n de la medición +46–65 % / ~3× y del ±2 % (`lean-delegation-context.md:19-21`).
- El n y el detalle de la tarea que fijó la fracción 0,15 (solo un comentario de código y el commit 3facda23a, sin datos).
- Las cifras del banco: banco privado del autor; no se pudieron reproducir.
- El contenido de `skills/judgment-day/SKILL.md` (solo se comprobó que existe).
- Si Spark 1.3 se degrada a 150k: con la fórmula de gentle-pi, su umbral sería ese; el doc 12 propone 64k con base en la literatura general.
- La confusión del humano ante dos voces (variante B): sin evidencia; a probar en la Fase 0.
- Las estimaciones de costo de §5.2: `[inferencia]` sobre los supuestos del doc 12 §9.7 y §10.1.
