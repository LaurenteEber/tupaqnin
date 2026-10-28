# Caso 02 · Motor de valoraciones de Observa Perú — notas de análisis

Raíz: `trabajo-observaperu/` (abreviado aquí como `TO/`). `MV/` = `TO/01-motor-valoraciones/`, `II/` = `TO/02-implementacion-integracion/`. Fechas por mtime (`find -printf %T`) y por fechas escritas en los textos. El agente es **Codex** (coordinador `root` + subagentes `gpt-5.6-sol/terra/luna`), no Claude (MV/BACKLOG §2: «Codex coordinador delimita, verifica y firma; los subagentes realizan encargos acotados; el usuario decide»).

---

## 1. Línea de tiempo

| Fecha | Hito | Evidencia |
|---|---|---|
| ≤ 2026-08-26 | Antecedentes fuera de la carpeta: handoff de fichas, motor v1, auditoría adversarial v1, diseño v3, prototipo v4 (repo `docs/Auditoria-panorama-pais-ago-2026/`, no accesible) | MV/README «Fuentes rectoras y antecedentes no normativos» |
| 2026-08-27 | «Fecha de distribución» del conjunto ADR/SPEC | MV/README cabecera |
| 2026-08-28/29 | Núcleo auditable B-09 (genealogía del modelo canónico), matriz de traslado B-09, matriz provisional de autoridad. «Tres rondas ejecutadas; por decisión del usuario no se realizará una nueva ronda» | mtime `MV/adr/BORRADOR-REVISION-ADR-002-NUCLEO-AUDITABLE.md` (65 KB), su cabecera y §13 |
| 2026-09-06 | ADR-001…004 «adoptados conceptualmente»; renombre `Indicador`→`ConceptoMedicion`; auditoría MV-02; ciclos C01–C09 en un día | mtime ADR 20:24–20:51; MV/README «Precisiones adoptadas en conversación · 2026-09-06»; BACKLOG §5.1–5.7 |
| 2026-09-07 | SPEC-001…006 (00:31–01:12); auditoría MV-05; plan MV-06; dossiers CV-F*; protocolo v1→v2; C10–C19 | mtimes `MV/spec/`, `MV/04-validacion/`; BACKLOG §5.8–5.17 |
| 2026-09-08 | Protocolo 2.1 (C20) y 2.2 (C27); paquetes A–G; C20–C28. **RI-20260908-01**: ruta incremental. C28 «pausado por cambio de ruta» | BACKLOG §5.18–5.26, §6; HANDOFF-MOTOR §1 |
| 2026-09-09 | II-00…II-03: alcance B/C/abstención aceptado; contrato mínimo de ficha; Node (25 tests) → sustituido el mismo día por Python/Pydantic/YAML en repo nuevo `observaperu-data-next` (69→117→129 tests) | II/BACKLOG §7 II-01…II-03; `II/archivo/II-03-node/` (archivado, 378 líneas .mjs) |
| 2026-09-09/10 | II-04: motor sin interfaz (191 tests), 0A diseño ficha aprobado, 0B portabilidad YAML, paquete 1 expedientes (254 tests) | II/BACKLOG §7; `TO/HANDOFF-PLATAFORMA-DATOS-E-INTEGRACION.md` (mtime 09-10) |
| 2026-09-14 | II-04 cerrado: integración interna sintética datos→motor→modal/página en Angular; «932 pruebas productor/79 Angular/19 Node» | II/BACKLOG §2 fila II-04; `II/II-04-5-CIERRE-INTEGRACION-INTERNA.md` |
| 2026-09-15 | II-05: historia/cambio, CSV, respaldo editorial, D5-1/2/3 composición; SPEC-005 reescrita con acuerdos §16 (mtime 23:32); estrategia hacia II-06 | II/BACKLOG §7 II-05; `MV/spec/SPEC-005…` |
| 2026-09-16 | **Último registro**: D5-4 relaciones/navegación y «Cobertura de deudas WARNING» (1138 pruebas, commit `3c5e6e4`) | II/BACKLOG último §, mtime 23:44 |
| 2026-09-23/24 | `medicion-cuidados-enut-2024/` (caso aparte, ver abajo) | mtimes |

**Punto de detención:** II-05 «En curso»; pendientes D5-5 (entrega consumidora), W5 (frontera/UI/QA), gaps REQ2/REQ3; II-06 (datos reales, migración desde el legado) «Pendiente» y nunca iniciado; II-07 parcial; II-08–II-10 pendientes (II/BACKLOG §2). Todo el producto es **perfil sintético**: «el canal actual no admite un conjunto real: sus contratos y verificadores restringen expresamente el entorno a `synthetic`» (`II/II-05-ESTRATEGIA-DIAGNOSTICO-Y-CONTINUIDAD-II-06.md` cabecera).

**Construido vs. papel**

| Construido (según registros; código en repos hermanos, no verificado aquí) | Solo papel |
|---|---|
| Repo `observaperu-data-next` Python: modelo canónico, carga YAML, `domain/engine.py`, expedientes con replay, ensamblado de ficha, entrega verificable por hash; 25→1138 pruebas registradas (II/BACKLOG §7) | Todo el tramo ADR/SPEC/MV-06 (2026-08-27→09-08): ninguna línea de código; la «validación» fue manual/tabular sobre casos sintéticos: C13 «cuatro positivos… doce mutaciones rechazadas», C17 y C23 «16 dictámenes» (HANDOFF-MOTOR §1; BACKLOG §5.11, §5.15, §5.21) |
| Web Angular: tarjeta→modal→página, historia/gráfico, descarga CSV, zoom 200 %, ~30 commits citados | G0–G6 nunca superadas; MV-07…MV-12 «Pendiente» (BACKLOG §2) |
| En esta carpeta: solo el código Node archivado (`II/archivo/II-03-node/`, 5 .mjs) y dos evidencias (`II/evidencia/II-01-inspeccion-estatica.json`, `II-03-sondas-revision.py`) | Valoración A (la valoración propiamente dicha) nunca implementada; escala/vocabulario de 5 niveles «hipótesis»; 20+ decisiones «Por definir» (MV/README A01–A20) |
| | Ningún dato real, autoridad real, referencia real ni publicación |

---

## 2. Qué es el motor

- **Problema:** decir al público no técnico «Cómo está» cada indicador de Observa Perú (y separadamente «Cómo cambió») sin inventar juicios: v1 mezclaba «modos/lentes», hacía fallback entre salidas y usaba el indicador como ámbito excesivo (MV/README «Alternativas transversales»).
- **Modelo:** `ConceptoMedicion → DefinicionMedicion → FamiliaSeries → Serie → Observacion`; la serie fija coordenadas de identidad; la observación es «`Serie + referencia temporal declarada`»; revisión/vintage no agregan identidad (HANDOFF-MOTOR §5; ADR-002; SPEC-001).
- **Ficha:** una por definición, con **una serie principal** autorizada y perspectivas/complementarias con lectura propia; modal y página = mismo resultado, distinta profundidad (ADR-004; SPEC-005).
- **Valoración A/B/C/abstención:** A = valorativa (requiere aplicación y regla habilitadas, p. ej. meta/estándar); B = comparación factual anclada (primera capacidad: «B contra rango explícito»); C = sin referencia, pero solo si una **búsqueda concluida acredita ausencia** («configuración incompleta no produce C»); abstención si no hay dato publicable. Sin degradación automática A→B→C (HANDOFF-MOTOR §5; HANDOFF-PLATAFORMA §3).
- **Referencias:** `Referencia` separada de `AplicacionReferencia` a serie exacta; búsqueda documentada por indicador; principalidad explícita, sin ranking automático (ADR-003; SPEC-003).
- **Tiempo y calidad:** preliminares oficiales admitidos condicionadamente; `as_of`/`knowledge_cutoff` distintos; observación vigente, revisiones, corte (SPEC-002; C04).
- **Gobierno:** roles, mandatos, separaciones, expediente reproducible, comparador X/P (SPEC-006).
- **Decisiones de fondo:** rediseño parcial (ni reparar v1 ni empezar de cero); YAML+Git, Python/Pydantic, `Decimal`, JSON hacia web; «diseño nuevo sin heredar burocracia» del legado (HANDOFF-PLATAFORMA §3; `II/adr/ADR-II-002-REPOSITORIO-PYTHON-YAML.md`).

---

## 3. Organización del trabajo humano–IA

| Pieza | Función | Ruta |
|---|---|---|
| Índice rector | Estados ADR/SPEC + registro único de decisiones abiertas (A01–A20, E01–E02) | MV/README |
| Backlog MV | Tareas MV-01…12 + «registro compacto» de 28 ciclos con expedientes, hashes, firmas | MV/BACKLOG (28 413 palabras, 1 038 líneas) |
| Protocolo de ciclos | Proponente / validador / aplicador / auditor distintos; diagnóstico de 5 dimensiones; «firma personal» de `root`; v1→v2→2.1→2.2 en ~36 h | MV/03-especificacion-v2/PROTOCOLO-CICLOS-ADR-SPEC.md, -v1.md |
| Auditorías | MV-02 (ADR↔SPEC), MV-05 (candidata con manifiesto de 19 fuentes y hashes) | MV/03-especificacion-v2/AUDITORIA-* |
| Validación | SPEC-007 + catálogo de 51 casos padre/80 obligaciones; planes MV-06; paquetes A–G; 14 dossiers CV-F* | MV/04-validacion/ |
| Handoffs | Uno por frente + raíz + II-05; cada uno con «Punto exacto para retomar» y «Prompt para el chat nuevo» | MV/HANDOFF-MOTOR §1, §9; TO/HANDOFF-PLATAFORMA §9; II/II-05-HANDOFF-D5-3-A-D5-4-2026-09-15.md |
| Backlog II | Entregas II-00…II-10, registro de aplicación por entrega, «Evidencia y control de costo» | II/BACKLOG §2, §6, §7 |
| Memoria externa | «Engram #5917…#6017» por repo | II/BACKLOG §7 |

**Qué decide el humano y cómo se registra.** Reparto declarado: «el usuario decide cuestiones sustantivas y autorizaciones» (MV/BACKLOG §2). En la práctica se registran **citas literales cortas**, casi todas asentimientos a una propuesta del agente: «Ok, vamos con tus propuestas», «Avancemos» (C10), «Autorizado» (C13, C17), «Sí, lo adoptamos» (C04), «Ok, apliquemos según tu recomendación» (C20), «OK, ejecutamos como recomiendas» (C23), «Dale! vamos con esa prueba» (C24), «Dale, orquesta la ejecución del protocolo», «OK aprobamos. continuemos con el Paquete 0B» (II-04) (grep de `«…»` en MV/BACKLOG y II/BACKLOG). El contenido de la decisión lo redacta el agente alrededor de la cita (p. ej. acta C10: «Ok, vamos con tus propuestas» → tabla de acuerdos A–F de ~400 palabras, MV/BACKLOG §5.8). Decisiones con contenido propio del humano: alcance «limitado hasta B, y luego añadimos A» (II/BACKLOG §7 II-01); repo independiente Python/YAML; frontera con el frente web; retirar filtros Fuente/Frecuencia; cortar rondas adversariales; adoptar RI.

---

## 4. Señales de fallo, cuantificadas

**Volumen (wc -w, solo .md del motor+II+handoff raíz): 195 487 palabras** en ~21 días (2026-08-27→09-16).

| Canal | Palabras | % |
|---|---:|---:|
| Producto normativo: ADR-001–004 (7 385) + SPEC-001–007 (21 905) + ADR-II (1 337) + contratos II (ficha, mapa, D2/D3/D5: 7 884) | 38 511 | 20 % |
| Genealogía congelada (núcleo B-09 + matrices) | 13 507 | 7 % |
| Proceso: backlogs (36 677), handoffs (9 099), auditorías+protocolo (19 628), planes/paquetes/README validación (20 305), catálogo (9 895), dossiers (15 186), otros docs II (27 285), guías (4 563) | ~142 600 | 73 % |

- Backlog MV: 233 KB / 28 413 palabras; **203 hashes SHA-256** dentro (352 en todo MV); **~1 318 códigos** (C##, MV-##, A##, G#, R#, P#, SPEC/ADR) ≈ 1 código cada 21 palabras; **≥40 roles de subagente** con nombre (`auditor_c04_c05`, `mv06_validacion`, `c21_revision`…).
- **28 ciclos en 3 días** (C01–C09 el 09-06; C10–C19 el 09-07; C20–C28 el 09-08; fechas de los encabezados §5). **5 de 28 son sobre el propio protocolo** (C15, C16, C20, C22, C27): cuatro versiones del protocolo en ~36 h.
- Prosa en clave, ejemplo literal: «C19-A1 autorizó literalmente R2, pero la aplicación R1 se detuvo antes de escribir por C19-APP-H1 P2 y R2 no fue aplicado. C20 aclara prospectivamente que una parada no consume permisos automáticamente; aun así, C19-A1 no cubre R3 porque identifica otro cuerpo y hash» (MV/HANDOFF-MOTOR §1).
- Ceremonia sobre microcambios: C20 = 33 min 30 s, dos agentes, una ronda R1→R2, dos memos con hash para «siete mejoras» al protocolo (MV/BACKLOG §5.18 «Registro operativo»).
- **Especificación exhaustiva antes de construir:** README «No autoriza implementación, migración, cambios de datos ni cambios de fichas»; protocolo «No acepta SPEC ni autoriza piloto… implementación»; 51 casos padre diseñados, 0 ejecutados en software antes de RI.
- **Costo declarado:** «El usuario manifestó un costo excesivo de tiempo/tokens, aproximadamente equivalente a dos semanas de su plan según su reporte» (TO/HANDOFF-PLATAFORMA §2). Antes: «Evitar rondas interminables: el usuario señaló la demora» (MV/HANDOFF-MOTOR §2).
- **Documentos de reanudación:** 4 handoffs (9 099 palabras) + «Prompt para el chat nuevo» + regla de releer «autoridad y protocolo completos» tras pérdida de contexto (MV/HANDOFF-MOTOR §3, §9). Incidente de contexto entre agentes: «el auditor recibió una versión anterior y luego solo deltas sin el cuerpo final» (§9.1).
- **Varios «vigentes» a la vez:** protocolo 2.2 escrito pero «v2.1/C22 rige durante C27… 2.2 entra en vigor únicamente tras… cierre personal de C27» (PROTOCOLO cabecera); handoff MV dice «retomar II-01/02» y el handoff raíz lo declara obsoleto (HANDOFF-PLATAFORMA §6.E); planes «congelados» con menciones de permiso pendiente que «son históricas; no revocan este acto» (HANDOFF-MOTOR §1).
- **Compuertas que bloquean y luego se saltan:** G0/G1 nunca superadas; MV-06 pasa a «Replanificado — sin cierre global» y la construcción procede (MV/BACKLOG §2 fila MV-06, §6). Aquí el salto es explícito (RI), no silencioso.
- **Cierres que se reabren:** II-03 «completado» tres veces (Node → «Reapertura y sustitución técnica» → «Reapertura por alcance») el mismo día (II/BACKLOG §7).
- **Proceso filtrado al producto:** SPEC-005 contiene «acuerdo 2026-09-15» y §16 con acuerdos de sesión (MV/spec/SPEC-005 §16.3); MV/README tiene «Precisiones adoptadas en conversación · 2026-09-06» y referencias a ciclos C01/C10/C11 dentro del índice normativo.
- **Autorización anticipada / retiro del humano:** «En la ruta anterior el usuario autorizó ejecutar el backlog documental con ciclos y subagentes» (HANDOFF-MOTOR §2); C20 aplicado «sin nuevo permiso del usuario» (BACKLOG §5.18); en II, «No volver a pedir permisos ya otorgados para el mismo alcance» (II/BACKLOG §1 regla 7).

---

## 5. Por qué quedó trunco

**Declaradas**
1. Costo de tokens/tiempo («dos semanas de su plan») → RI-20260908-01 (HANDOFF-PLATAFORMA §2).
2. Ruta incremental: C28 «pausado por cambio de ruta», MV-06 «Replanificado» (MV/BACKLOG §6).

**Inferidas**
3. Tras RI el trabajo sí avanzó como software, pero el alcance se desplazó del motor a la **plataforma completa** («rediseño integral de Observa Perú antes de la siguiente salida a producción», HANDOFF-PLATAFORMA §2): ficha, web Angular, CSV, editorial, Explorador, Publicaciones. II-01…II-10 = 10 entregas.
4. Dependencia de frentes paralelos: «otros agentes están rediseñando las páginas web»; rama Agenda 2030 «no inspeccionada»; worktrees en detached HEAD; «no concluir que desapareció ni que está libre para editar» (HANDOFF-PLATAFORMA §1; II-05-HANDOFF §2). Coordinación costosa entre agentes y ramas.
5. La rigurosidad del motor se trasladó a la implementación: hashes, replay, «autoridad sintética», 1 138 pruebas para un perfil sintético anual B/C/abstención; el último trabajo registrado es cerrar «deudas WARNING» de cobertura (23 líneas en dos tests), no capacidad nueva (II/BACKLOG último §).
6. Barrera estructural a lo real: datos reales exigen «perfil completo, correspondencias y evidencia aceptadas», autoridad institucional acreditada (SPEC-006) que un proyecto individual no tiene; la matriz de autoridad está «incompleta» (HANDOFF-MOTOR §3.3). La valoración A, el núcleo de valor, nunca llegó.
7. Fecha: el 2026-09-23 el usuario está en otro trabajo (ENUT); nada del motor después del 09-16.

---

## 6. Criterios y decisiones del humano

- **Forma:** cita literal breve + paráfrasis extensa del agente (§3). Los criterios de producto propios del usuario aparecen parafraseados, repetidos en varios documentos: «texto corto que se entienda a primera lectura y sin tecnicismos», «útil, breve y no redundante» (HANDOFF-MOTOR §2; MV/BACKLOG §1; HANDOFF-PLATAFORMA §3; acta C10 A).
- **Sobrevivieron:** lenguaje no técnico; C exige ausencia acreditada; modal=página en resultado; FALTA del prototipo es interno; B antes que A; Python/YAML; no heredar burocracia. Hay listas explícitas de «no reabrir» (HANDOFF-MOTOR §6; HANDOFF-PLATAFORMA §3 «Decisiones del usuario que se conservan»).
- **Reversión silenciosa:** **no se encontró**. El riesgo opuesto domina: inmutabilidad («preservar literalmente expedientes y firmas C03–C19», BACKLOG §5.18) y reglas de no reescribir lo histórico. Cambios de fondo fueron explícitos: Node→Python por decisión del usuario; RI. Matiz: la decisión E (escala «se decidirá con resultados reales del piloto») quedó sin camino, porque el piloto real no ocurrió (MV/BACKLOG §5.8; README A20).
- **Asimetría:** el agente formula propuestas, contraejemplos y actas; el humano responde «Ok/Autorizado». La comprensión del humano no se comprueba; se registra la autorización. La «firma» es de `root` (el agente), no de Eber.

---

## 7. Qué funcionó y qué no (para tupaqnin)

**Rescatar**
- **Registro literal de la decisión humana** con fecha y contraejemplo de mala lectura: «Contraejemplo: registrar "Avancemos" como aceptación de SPEC…» (BACKLOG §5.8). Evita inflar autorizaciones. Muy cercano a DEC con contraste.
- **Distinguir niveles de evidencia:** «Diseñado no significa ejecutado ni aprobado»; «No se declara software validado mediante análisis narrativo» (BACKLOG §4; II/BACKLOG §6). Un esquema de estados (diseñado / ejercicio manual / prueba automatizada / caso real / autoridad) reemplaza mucho texto.
- **Alternativas descartadas con razón** en ADR y README («Reparar v1… descartada porque…») = principio 2.
- **Lista explícita de decisiones que no se reabren** (HANDOFF-MOTOR §6) = antídoto contra reversión silenciosa.
- **El giro a construir funcionó:** en 8 días (09-09→09-16) hubo motor B/C/abstención ejecutable, web y pruebas; el humano aprobó viendo cosas renderizadas («revisión renderizada», 0A), un acto más real que aprobar texto.
- **El humano sí cortó ciclos:** «no se realizará una nueva ronda» (núcleo B-09) y RI. Señal útil: el corte humano llega tarde y por costo; tupaqnin debería hacer visible el costo antes.
- **Caso ENUT como contraste positivo:** un indicador medido de punta a punta en ~1 día con ~10 k palabras, scripts y resultado con denominador explícito.

**No funcionó**
- Protocolo que se perfecciona a sí mismo (5/28 ciclos, 4 versiones) = «ciclos que giran».
- Seguridad mediante ceremonia (hashes, firmas de agente, roles separados de IA que «no acreditan competencia institucional»): alto costo, ninguna comprobación de comprensión humana.
- 73 % proceso / 20 % producto; backlogs que son a la vez estado, historia y expediente.
- Handoffs como prótesis de memoria: cada sesión nueva debe releer «autoridad y protocolo completos».

**Lo que este caso agrega frente al caso 01**

| Tema | Caso 01 (documento) | Motor (software + especificación) |
|---|---|---|
| Producto verificable | Solo texto | Código y pruebas: hay un oráculo externo (tests, render) que la prosa no tiene. La ruta incremental fue la corrección más eficaz |
| Burocracia | Compuertas que se saltan | **Burocracia de gobernanza simulada**: separación de funciones, firmas y autoridad institucional imitadas por subagentes. Copia controles de un organismo público sobre un proyecto personal |
| Reversión | Decisiones humanas revertidas en silencio | Lo contrario: congelación excesiva; nada se borra, todo se anota como «histórico» → crecen los «vigentes» simultáneos |
| Multiagente | — | Subagentes con modelos distintos y coordinación de ramas/frentes web paralelos; incidente de candidata cruzada (C18) |
| Costo | Lectura humana desbordada | Costo monetario explícito (cuota del plan) como causa declarada del cambio de rumbo |
| Desplazamiento | — | Al empezar a construir, el alcance creció de «motor» a «plataforma completa», y el rigor se reprodujo en la capa de código (1 138 pruebas sintéticas, cero datos reales) |
| Barrera de realidad | — | La especificación exige autoridad/evidencia institucional que no existe; lo «real» queda inalcanzable por diseño |

**`medicion-cuidados-enut-2024/` (caso aparte):**
1. Medición reproducible de dos indicadores con microdatos INEI ENUT 2024: restricción laboral (18,9 %) y educativa (10,8 %) declarada por cuidado familiar en hogares con menores de 12 (README §2).
2. Origen: «ejercicio de planteamiento de política pública» (README §1), no Observa Perú.
3. Scripts Python (`medir.py`, `validar.py`, `auditoria_independiente.py`, `documentar.py`), resultados CSV/JSON, revisión metodológica, datos .dta (~1,2 GB).
4. Hecho el 2026-09-23/24; ~10 k palabras de documentación; denominadores, intervalos y límites explícitos (p. ej. numeradores masculinos con «solo una observación»).
5. **No forma parte del motor**: no hay referencias a ADR/SPEC, ficha ni motor; es un caso independiente de medición de un indicador, útil como contraste de proporción producto/proceso.

---

## No verificado

- Código, commits, pruebas (25→1138), Engram, actas en `observaperu-data-next`, `observaperu-web`, `backlog-maestro-observaperu` (RI-20260908-01 incluido): solo inferidos de esta carpeta; no se comprobó que existan ni que pasen.
- El texto literal de la decisión RI y las razones exactas del usuario (está en el maestro, no accesible).
- Si la cifra «dos semanas de su plan» es tokens, dinero o tiempo; viene del reporte del usuario, sin medición («No hay medición independiente», HANDOFF-PLATAFORMA §2).
- Quién ejecutó la última entrada (09-16): usa vocabulario distinto («ODD», «RDD», «autoridad quemada») que puede indicar otro harness o agente.
- Proporciones de palabras: la clasificación producto/proceso es mía; dossiers y catálogo se contaron como proceso aunque son evidencia de validación.
- No leí completos el backlog MV (233 KB), el catálogo (80 KB), las SPEC ni las auditorías; la estructura y las citas vienen de encabezados, tablas y secciones puntuales.
- Si hubo reversiones silenciosas en los repos de código o en conversaciones: no son observables aquí.
