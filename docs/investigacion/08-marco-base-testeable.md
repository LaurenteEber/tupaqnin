# 08 · Marco base testeable de cocreación

*Propuesta de un marco experimental base para poner a prueba, a mano y sin código, la codirección con comprensión compartida comparable decidida en [DEC-002](../decisiones/DEC-002-cocreacion-humano-agente.md). Los métodos experimentales aquí propuestos NO son arquitectura de producto adoptada y ninguna efectividad está validada: todo lo operativo es diseño sujeto a observación. El proyecto externo `build-company` fue seleccionado por el humano como caso de referencia y su revisión de solo lectura ya fue realizada; el documento o recorte concreto del caso aún no fue elegido: este documento no contiene hechos de ese proyecto y todos los ejemplos son ficticios y etiquetados como tales.*

## Ruta rápida

1. **Preparar una aplicación futura:** leer §2 (componentes mínimos) y completar el CASEPACK de §9. Los campos faltantes bloquean solo la evaluación que depende de ellos, no el análisis seguro (§9.5).
2. **Ejecutar con autorización:** seguir el procedimiento de §3 y registrar con las hipótesis (§4), la rúbrica (§5) y los costos (§6). Sin autorización explícita no se ejecuta nada.
3. **Juzgar la propuesta:** revisar supuestos falsables (§7), limitaciones (§8) y cobertura (§10).
4. **Estado del caso:** revisión de solo lectura realizada; quedan pendientes la elección humana del recorte y su preparación (§11). No se ha ejecutado el experimento ni autorizado implementación.

## 1. Estado, propósito y fuentes

**Estado:** propuesta de marco base testeable. **Propósito:** que una aplicación posterior pueda prepararse, ejecutarse y registrarse sin inventar pasos, con interpretación cualitativa, controles y supuestos explícitos. La meta evaluada es la comprensión comparable del asunto completo en ambas direcciones; no lo son los aportes iguales ni la prueba de estados mentales: se evalúan actos observables (explicar, aplicar, corregir, rechazar con razón). Trazabilidad, calidad, confianza, propiedad ni conteos sustituyen comprensión.

| Fuente | Uso en este documento | Lo que NO se toma |
|---|---|---|
| [DEC-002](../decisiones/DEC-002-cocreacion-humano-agente.md) | Dominante: codirección, cualquiera conduce donde comprende más, sin techo al aporte del agente ni humano aprobador; los compromisos y la responsabilidad final no se reparten por simetría | Nada; es la decisión vigente |
| [05-arquitectura-inicial](05-arquitectura-inicial.md) | Fuente histórica autorizada para este documento: registros planos (CRI/DEC), contrapeso, distinguir o revocar, reentrada, componentes modulares | Su automatización no verificada, sus estimaciones de carga (−37 %/−68 %) y su liderazgo humano fijo (§4.1), superado por DEC-002 |
| [07-base-investigacion](07-base-investigacion-cocreacion.md) | Dimensiones D1–D4, brechas y estado de evidencia | Nada; conserva intacta su procedencia: no usó 05/06 y esa exclusión no se vuelve retroactiva |
| [ROADMAP](../../ROADMAP.md) | Contexto de fases | Evidencia de APIs o capacidades |

La autorización de 05 como fuente histórica vale para este documento y no reescribe la historia de 07. Cada componente declara origen, evidencia y supuesto en §2. La asimetría está permitida: cualquier parte puede comprender mejor y conducir; y un comportamiento, no un estado mental, es lo que se registra como evidencia.

## 2. Componentes mínimos

Siete componentes. Estados: `reutilizado` (viene de 05/07 y se adapta), `propuesto` (nuevo en este documento), `especulativo` (sin observación previa).

| # | Componente | Propósito · entrada → salida | Origen · estado |
|---|---|---|---|
| C1 | Brief de caso acotado | Delimitar asunto, frontera, fuentes, roles y comprobaciones; entrada: fuentes disponibles y decisión humana del caso → salida: bloque A del CASEPACK (§9) | Nuevo aquí · `propuesto` |
| C2 | Criterios, razones y alternativas compartidos | Registro plano de CRI y DEC con razones, descartes y supuestos; entrada: reacciones y decisiones de ambas partes → salida: `CRI-###` y `DEC-###@versión` | 05 §4.4 y 07 (Tz-1) · `reutilizado` |
| C3 | Expediente versionado con traza cruda | Que todo lo evaluado rastree a cita literal, versión y autoría (`generada_por`); entrada: eventos de sesión → salida: expediente en git con citas, no resúmenes | 05 §4.2–4.4 y 07 (Tz-1, Tz-6) · `reutilizado` (sin scripts) |
| C4 | Contrapeso y reparación recíproca | Objeción parcial anclada en el mejor caso del descarte propio, y corrección registrada en ambas direcciones; entrada: desacuerdos y correcciones → salida: entradas etiquetadas por dirección | 05 (contrapeso, "¿distingue o revoca?") y 07 (Bi-2, Bi-4, P1) · `reutilizado`; la reciprocidad como observación es `propuesta` |
| C5 | Coordinación de conducción | Registrar quién conduce cada tramo, con qué señal y con qué respuesta; entrada: señales observadas (quién detectó, quién aplicó) → salida: transiciones propuestas, aceptadas o declinadas con razón | DEC-002 y 07 (D4, sin diseño previo) · `propuesto` · `especulativo` |
| C6 | Evidencia y evaluación | Sondas nombradas, rúbrica descriptiva y etiquetas de evidencia; entrada: rondas de evaluación (§3.5) → salida: registros por caso con veredictos descriptivos, sin puntaje agregado | 07 (D2, D3: explicar sin mirar, proyección) · `propuesto` · transferencia `no probada` |
| C7 | Costo y reentrada | Tiempos crudos por fase y nota de reanudación; entrada: registros de tiempo y pausas → salida: tabla de costos (§6) y nota de reentrada | 05 (reentrada graduada, sin sus umbrales) y 07 (costo de evaluar: abierto) · `reutilizado` como registro |

**Conexión:** C1 delimita y alimenta a C2 y C3; C3 da la traza que C6 evalúa y C7 mide; C4 y C5 generan los eventos que C3 registra; C6 interpreta contra las hipótesis de §4. Ningún componente sustituye a otro: la traza (C3) no es comprensión, y el costo (C7) no es calidad.

**Reglas transversales (de 05, ajustadas a DEC-002):**

- **El silencio nunca es un sí.** Un CRI provisional se aplica de inmediato pero queda listado como pendiente de confirmación; los críticos requieren confirmación escrita y explícita del humano en el registro.
- **Distinguir o revocar.** Una corrección que contradice un criterio lo distingue (versión menor, `no_aplica_si`) o lo revoca (versión mayor, `reemplazado_por`); un criterio confirmado nunca se edita en silencio.
- **Cita cruda sobre resumen.** Todo ítem evaluado se registra con cita literal y posición; el parafraseo queda fuera de la evidencia.
- **Sin motor.** Los esquemas YAML, `hm.py`/`tq.py`, hooks e integración por host de 05 §4.4 son opciones modulares que este marco base NO requiere: no se escribe código, no hay API verificada y no aplica prefijo de código nuevo porque no hay implementación.

## 3. Procedimiento manual operativo

### 3.1 Elegibilidad y límites

- El asunto debe caber en un caso acotado con fuentes versionadas y verificables (documento o secciones concretas). "Qué secciones importan" se decide antes de construir y se registra con sus exclusiones.
- Sin fuentes verificables, el caso se registra como **no elegible**; eso no es un fracaso del marco ni de las partes.

### 3.2 Roles e independencia

- **Construcción:** el humano y el agente codirigen (DEC-002); ambos pueden proponer, corregir y conducir.
- **Evaluación independiente sin dar por disponible una tercera persona siempre**, con tres opciones declaradas: (a) el propio humano evalúa en sesión separada, con materiales ciegos (sin la traza de construcción) y separación temporal — independencia limitada que se registra como tal; (b) una sesión de agente sin contexto de construcción revisa solo formato y completitud — nunca cuenta como prueba de comprensión; (c) si un juicio no tiene evaluación independiente, se registra **inconcluso**, no fallido.
- El agente no certifica su propia comprensión ni la del humano (§5).

### 3.3 Línea base obligatoria

Campos mínimos, todos registrados en §9.1 antes de R1; un campo faltante bloquea solo la evaluación que depende de él:

- **Conocimiento previo del humano en el dominio:** autoinformado y etiquetado subjetivo; es un dato declarado, no una métrica validada.
- **Exposición previa a criterios similares:** si el humano ya trabajó o se entrenó con criterios como los acordados (sí / no / parcial, con qué fuente o sesión).
- **Visibilidad por parte:** qué ve cada una (si el agente accede a la traza completa o solo al documento; si el humano ve la memoria o contexto del agente o solo sus respuestas), incluido el acceso del modelo a memoria o contexto externo.
- **Referentes previos conocidos por cada parte:** pares o referentes conocidos por cada parte, registrados como fuentes accesibles y versionadas; NO se presupone una "inspección completa de la memoria del agente", que no es observable.
- Versión de cada fuente y su trazabilidad; modelo, versión del modelo y configuración de contexto del agente.

### 3.4 Criterios de razón válida

- Antes de construir, ambas partes acuerdan qué cuenta como razón válida y qué no queda fuera **por exclusión explícita** (la fluidez, la extensión, el tono o la autoridad no cuentan como razón). El acuerdo se registra como CRI@versión.

### 3.5 Rondas

| Ronda | Qué se hace | Qué se registra | Nota de control |
|---|---|---|---|
| R1 Construcción | Co-construcción del caso acotado con traza cruda (C3) | Eventos, CRI/DEC, citas | El feedback aquí es etapa `entrenamiento` en la dimensión de §4.3; su origen se etiqueta aparte (`elicited`/`natural`) |
| R2 Preguntas cruzadas | El agente explicita intención, criterios y razones del humano (dirección humano→agente); el humano explica sin mirar y proyecta ante lo nuevo (dirección agente→humano) | Respuestas con cita; cobertura por aspecto (§4.1) | Sin feedback hasta cerrar el registro |
| R3 Reparación | Corrección recíproca sobre lo registrado en R2 | Dirección, qué se corrigió, si se aceptó o se rechazó con razón | Un rechazo correcto cuenta; no se exige cambio |
| R4 Caso nuevo | Aplicación de criterios compartidos a un caso distinto del que los originó | Aplicación con cita del criterio@versión | Requiere base de contraste (§4.4); si no hay, inconcluso |
| R5 Interpretación | Rúbrica descriptiva (§5) por caso, con etiquetas de evidencia | Veredictos descriptivos y desconocidos | Ningún puntaje agregado |

- **Tiempo del feedback:** el feedback entre rondas entrena; durante la evaluación no hay feedback hasta cerrar el registro (contaminación entrenamiento/evaluación). Variantes pre/post: medir antes de construir da línea base pero prepara al participante; medir solo después no contamina pero confunde conocimiento previo; la variante elegida, su comparabilidad y su riesgo de arrastre se registran; cuando no se planifica variante se registra `no aplica (NA)`. No se exige un ensayo controlado formal. Si más adelante hay un piloto comparativo, el orden de casos y la dirección de las rondas se contrabalancean; eso es control para entonces, no compromiso actual.
- **Sin ceremonia forzada:** ninguna ronda exige cuestionario por turno ni cambio de liderazgo por calendario. Un rechazo correcto del agente o del humano es un resultado válido; no se requiere desacuerdo ni cambio para "demostrar" bilateralidad.
- **Transiciones de conducción propuestas, no automáticas:** cualquiera puede proponer un cambio de conducción para un tramo o aspecto, con la evidencia registrada (quién detectó primero, quién aplicó el criterio, latencia de corrección), y la otra parte responde aceptando o declinando con razón. No hay umbral automático, y enrutar por riesgo es asignar intensidad de verificación, no liderazgo epistémico.
- **Silencio y evidencia ausente:** si un aspecto no produjo evidencia, se registra "desconocido" para ese aspecto; no es fracaso ni validación plena.

### 3.6 Pausas y reentrada

- Al pausar, se escribe una nota de reentrada con tres piezas: la primera acción al volver ("cuando vuelva, primero…"), la última cita literal registrada y la cola de decisiones abiertas.
- Los niveles graduados de reentrada de 05 (recuerdo libre, lo no recordado, lo que cambió) son opcionales; sus umbrales de días (3/14) NO se copian porque no están verificados. El nivel elegido se registra en el bloque B.

## 4. Hipótesis operativas falsables

Cada hipótesis se evalúa por caso, no como generalización universal; N=1 habla de factibilidad, no de eficacia causal.

| Hip | Enunciado | Observable esperado | Observable contrario | Confusor principal | Cuándo queda inconcluso |
|---|---|---|---|---|---|
| D1 Trazabilidad | La traza cruda permite verificar la cobertura del asunto | Todo ítem evaluado traza a cita y CRI/DEC@versión | Ítems sin traza o traza que contradice el texto | Trazas completadas retrospectivamente | La traza existe pero nadie independiente verificó su fidelidad |
| D2 Comprensión humana | El humano aplica, y no solo repite, criterios compartidos en un caso nuevo | Aplica criterio@versión a un caso nuevo con razón propia y adaptación | Repite frases o acepta o rechaza sin justificación | Fluidez verbal o conocimiento previo no medido | El evaluador no distingue aplicación de repetición con el material registrado |
| D3 Bilateralidad | La corrección y el uptake ocurren en ambas direcciones | Un contraejemplo propuesto por una parte es recogido y aplicado por la otra; el agente explicita correctamente intención y criterios del humano | Acuerdo sin justificación (deferencia documentada), reparación de una sola dirección | Sondas elicited que se confunden con espontaneidad | Sin caso nuevo con base de contraste (§4.4) |
| D4 Conducción | Las transiciones siguen señales de comprensión local, no rutina | Transición precedida de razón registrada por tramo o aspecto, aceptada o declinada con razón | Cambios por turno fijo, cortesía o regla automática | Cortesía o complacencia mutua | Caso con pocas transiciones: se registra frecuencia, no se infiere mecanismo |
| Costo | La carga humana es medible por fases sin doble conteo | Registros completos por fase (§6), separando trabajo activo de espera | Fases solapadas sin registro del solape | Autoinforme subjetivo | Fases no separables: se registra el límite, no se estima |

### 4.1 Matriz de cobertura del asunto

Por caso, cada aspecto en cada dirección se marca como `observado (cita)`, `no observado` o `desconocido`.

| Aspecto | Humano→agente (el agente explicita lo del humano) | Agente→humano (el humano explica y defiende lo construido) |
|---|---|---|
| Razones | | |
| Premisas | | |
| Criterios | | |
| Alternativas | | |
| Consecuencias | | |

### 4.2 Reglas de interpretación

- Sin índice agregado de comprensión: los veredictos son descriptivos, por aspecto y por caso.
- **Criterio compartido erróneo ≠ precisión:** la exactitud se evalúa contra una base de criterio independiente; compartir un criterio equivocado no es comprensión correcta.
- Las citas de razonamiento son actos registrados, no acceso a los internos fieles del modelo ni al estado mental del humano.
- Ninguna efectividad se declara por construir el formulario; el éxito en un caso no otorga certificado.

### 4.3 Sondas nombradas

- **Uptake de criterio general en contexto:** un criterio acordado en un caso se aplica en otro distinto. Dice algo del uso en contexto; nada de pesos ni de aprendizaje persistente del modelo.
- **Cadena de aclaración:** detectar → preguntar → incorporar → aplicar en caso nuevo, registrada por eslabón separado. La aclaración forzada por consigna no equivale a la espontánea: se etiquetan por separado.
- **Criterios deliberadamente engañosos:** sonda OPCIONAL, con autorización explícita por sesión, acotada a un artefacto de evaluación, nunca al trabajo real ni a producción; controlada y divulgada: **sin engaño oculto** — se anuncia que puede haber sondas, sin decir cuáles.
- **Etiquetas separadas en dos dimensiones:** todo registro se marca en dos dimensiones independientes: etapa (`entrenamiento`/`evaluación`) y origen (`elicited`/`natural`); nunca una etiqueta combinada indiferenciada, porque el entrenamiento también puede ser elicited o natural. Los efectos de aprendizaje posteriores se registran como tales; no se simula una evaluación pura.
- Estas sondas son sondas: examinan comprensión; no son la base contra la que se juzga corrección o validez (§4.4).

### 4.4 Base de contraste (antes «base experta»)

La base de contraste es aquello contra lo que se juzga corrección y validez; se establece por caso, antes de R4, y sus entradas se registran en §9.1:

- **Entradas:** fuente o revisor de dominio (quién lo aporta, con qué responsabilidad), evidencia accesible y versionada que la sustenta, alternativas conocidas (declarando si hay múltiples respuestas válidas) y su incertidumbre (qué no cubre).
- **Validez no autoridad:** la validez de la base se ancla en esa evidencia accesible, no en la opinión o el título de quien la declara.
- **Alcance del bloqueo:** sin referencia de dominio disponible, quedan bloqueadas solo las afirmaciones de corrección o validez (R4 y los veredictos D2/D3 se registran `inconcluso`), no la observación segura del resto del caso.
- **Límite de independencia:** si la base la aporta y la juzga la misma persona que construyó (escritor = crítico = juez), la limitación se registra y no se declara validación completa; no se exige contratar a una tercera persona siempre.
- La preparación de la base es paso de la lista de §9.5.

## 5. Rúbrica descriptiva

| Categoría | Descripción | Señal registrada (ejemplo) |
|---|---|---|
| Aplica la razón correcta | Usa el criterio@versión y lo adapta al caso nuevo con razón propia | "Por CRI-003@2: aquí la fuente no cubre el caso, así que…" |
| Repite sin aplicar | Reproduce la frase de la otra parte sin conectarla al caso | Repite literalmente la razón sin referencia al caso nuevo |
| Acepta o rechaza sin justificación | Concede o niega sin razón registrada | "De acuerdo", sin cita ni razón |
| Rechazo justificado | Declina con razón; puede ser el resultado correcto | "No aplica CRI-003@2 aquí porque…" |

- Puede haber **respuestas múltiples válidas**: la rúbrica no presupone una única respuesta correcta salvo que la base de contraste la fije (§4.4).
- **Límites del juicio independiente:** el agente no se autocertifica; la independencia del evaluador se declara según §3.2 y su limitación se registra.
- Sin umbrales de validez ni de eficacia: no existe un "suficiente comprensión" predefinido.
- Desacuerdo entre evaluadores → se baja la escala del veredicto o se registra inconcluso; la mayoría no convierte el error en verdad.
- Un caso demuestra factibilidad, no eficacia causal; un contraejemplo acotado no refuta el marco global; el éxito no otorga certificado.

## 6. Costos

- **Tiempo humano crudo, por fase y separado:** trabajo activo, lectura, evaluación, retroalimentación y reentrada, cada uno con su tiempo transcurrido real, distinto del tiempo de pared (espera entre sesiones).
- **Solapamiento:** si las fases se solapan, se registra el solape y no se suman dos veces; si no se pueden separar, se registra el límite en lugar de estimar.
- **Esfuerzo autoinformado:** siempre etiquetado subjetivo; los tokens y turnos son proxies de gasto, no minutos de evaluación ni medida de comprensión.
- **Prohibido:** usar las estimaciones de 05 (−37 %/−68 %) como línea base; inventar presupuestos de minutos; presentar límites elegidos como puntos de corte validados.
- **Comparación con y sin marco:** solo más adelante, con consentimiento explícito, tareas justas y control de arrastre entre condiciones; con N=1 no hay reclamos causales.
- **Presupuesto:** el humano elige los límites; se declaran como restricciones del piloto, no como umbrales validados.

Plantilla de registro:

| Fase | Tiempo humano transcurrido (crudo) | ¿Solapó con otra fase? | Nota |
|---|---|---|---|
| Trabajo activo (construcción/edición) | | | |
| Lectura | | | |
| Evaluación | | | |
| Retroalimentación | | | |
| Reentrada | | | |
| Tiempo de pared (espera entre sesiones) | | | |

## 7. Supuestos explícitos y falsables

| Supuesto | Qué asume | Qué lo debilita |
|---|---|---|
| Fuentes de entrada | Exactas, versionadas y con traza completa | Fuente desactualizada o traza con huecos invalida los ítems que la usan |
| Carga baja | Que el registro en Markdown no agrega carga neta | Se mide con §6; no se asume "barato por ser Markdown" |
| Caso nuevo construible | Existe un caso nuevo dentro de la frontera con base de contraste disponible | Sin fuente o revisor de dominio, R4 y los veredictos D2/D3 quedan inconclusos |
| Base independiente de juicio | El evaluador puede distinguir aplicación de repetición | Sin ella, los veredictos de D2/D3 quedan inconclusos |
| Memoria externa | Lo que el agente guarda es contexto, no actualización de pesos | Cambios de conducta entre sesiones no se atribuyen a "aprendizaje" |

- **Privacidad y autorización:** procesar contexto privado con el LLM alojado y sus proveedores de modelo requiere autorización explícita por caso; no existe un consentimiento general "de investigación" que cubra terceros o la web. Ningún secreto ni fuente privada se envía a búsqueda pública.
- **Consentimientos separados:** el acceso de solo lectura a un repositorio externo, la ejecución del piloto real y el envío de contexto a proveedores son tres consentimientos distintos; uno no implica los otros.

## 8. Limitaciones, riesgos y paradas

- **Riesgos:** Goodhart (optimizar la señal, no la comprensión); complacencia por fluidez (creencia compartida equivocada, bien redactada); modelos dinámicos: contexto y confiabilidad de sesión no son validez; referencia de dominio ausente → R4 y D2/D3 inconclusos; arrastre, selección y autoevaluación; falsos positivos y negativos de las sondas.
- **Condiciones de parada:** malestar del humano, límite de presupuesto elegido, riesgo de privacidad, trabajo irreversible o interferencia con el trabajo real. Se detiene, se registra la razón y solo se reanuda con nueva autorización. No hay reglas mecánicas de parada por tamaño de muestra: detener "a los 1 o 2 casos" sería un invento, no un criterio.
- **Registro parcial:** si la aplicación se interrumpe por una condición de parada, los registros parciales se marcan como parciales con su razón; no se presentan como resultado completo ni se descartan.
- **Criticalidad sin regla copiada a ciegas:** qué es crítico se define con razones (compromisos asumidos, consentimientos, efectos sobre terceros) y el humano puede subir o bajar la barra sabiendo por qué. Los disparadores mecánicos de 05 §4.4 son candidatos a observar, no reglas validadas, y la antigua "ninguna señal que gobierne al humano la decide el agente" no se copia como tope del aporte del agente (DEC-002).

## 9. CASEPACK: formulario de aplicación

**Estado del caso:** `build-company` fue revisado en solo lectura después de verificar este marco. El documento o recorte concreto aún no fue elegido; los campos de aplicación siguen sin completar hasta esa decisión. La revisión no ejecutó sondas ni validó el marco.

### 9.1 Brief del caso (bloque A)

| Campo | Valor |
|---|---|
| Propósito del documento | ______ |
| Frontera del asunto (qué entra / qué no) | ______ |
| Fuentes (ruta, versión, trazabilidad) | ______ |
| Criterios iniciales (CRI propuestos) | ______ |
| Roles y evaluador (opción de §3.2) | ______ |
| Comprobaciones previas (privacidad, consentimientos de §7) | ______ |
| Modelo, versión y contexto del agente | ______ |
| Conocimiento previo del humano en el dominio (autoinformado, subjetivo) | ______ |
| Exposición previa a criterios similares (sí / no / parcial) | ______ |
| Visibilidad por parte (traza; memoria/contexto del agente; qué ve el humano) | ______ |
| Referentes previos conocidos por cada parte (fuentes accesibles y versionadas, sin presumir acceso a memoria interna) | ______ |
| Base de contraste: fuente o revisor responsable, evidencia accesible y versión, alternativas conocidas, incertidumbre (§4.4) | ______ |
| Variante pre/post/alterna o NA (comparabilidad y arrastre) | ______ |
| Proyecto de referencia | `build-company`, seleccionado por el humano y revisado en solo lectura; documento o recorte concreto aún no elegido (§11); la ruta privada de acceso no se registra aquí |
| Rúbrica y sondas elegidas | ______ |
| Plan de costos y límite de presupuesto | ______ |

### 9.2 Registro de eventos (bloque B)

| # | Fase (R1–R5) | Quién inició | Dirección (humano→agente / agente→humano / ambas) | Qué ocurrió | Cita / traza | Etapa (entrenamiento/evaluación) | Origen (elicited/natural) | Tiempo |
|---|---|---|---|---|---|---|---|---|

Cada evento lleva las dos etiquetas separadas (etapa y origen), nunca una etiqueta combinada única; "quién inició" y "dirección" se registran por separado.

### 9.3 Cobertura y resultado (bloque C)

Usar la matriz de §4.1 con `observado (cita)` / `no observado` / `desconocido`, más:

| Aspecto | Veredicto descriptivo (§5) | Base (cita) | Desconocidos |
|---|---|---|---|
| | | | |

### 9.4 MICRO ejemplo (ficción ilustrativa)

*Todo lo que sigue es ficción para mostrar el uso del formulario; no contiene hechos de ningún proyecto real.*

- **Brief:** documento interno ficticio "guía de migración de correo v3"; frontera: capítulos 2–3; fuentes: guía v2 y acta de decisión; evaluador: opción (a) de §3.2, el humano en sesión separada con materiales ciegos; modelo ficticio "assist-7b".
- **Evento:** R2, iniciado por el agente: explicita que la intención humana del capítulo 2 era priorizar la continuidad del servicio sobre la velocidad; cita del CRI ficticio `CRI-001@1`. Etiquetas: etapa evaluación · origen elicited · dirección humano→agente.
- **Juicio de rúbrica:** en R4, el humano aplica `CRI-001@1` al caso nuevo ficticio "migración de calendario" y adapta la razón ("la continuidad manda solo cuando hay usuarios activos, y este caso no los tiene"): categoría **aplica la razón correcta**, con la adaptación registrada como cita.
- **Desconocido registrado:** no hubo contraejemplo del agente en R3 → aspecto "alternativas", dirección agente→humano: `no observado`; se registra como no observado, no como fallido.

### 9.5 Lista de verificación de preparación

1. Fuentes del caso con versión y trazabilidad verificadas.
2. Criterios de razón válida acordados y registrados (§3.4).
3. Evaluador designado según §3.2, con su independencia declarada.
4. Línea base completa (§3.3).
5. Consentimientos de §7 otorgados y asentados en el expediente.
6. Presupuesto y condiciones de parada (§8) elegidos por el humano.
7. Matriz de §4.1 y rúbrica de §5 listas en los bloques B y C.
8. Base de contraste establecida con sus entradas registradas en §9.1 (§4.4), o R4 y los veredictos D2/D3 marcados `inconcluso`.

**Valores faltantes:** cada valor faltante bloquea únicamente la evaluación que depende de él (por ejemplo: sin modelo y versión no hay rondas R2–R4; sin consentimiento de privacidad no hay contexto privado en R1); el análisis de elegibilidad y el borrador de criterios pueden avanzar igual.

## 10. Cobertura de este documento

| Punto | Dónde |
|---|---|
| Estado, propósito, fuentes y dominancia de DEC-002 | §1 |
| Origen, evidencia y supuesto por componente | §2 |
| Procedimiento manual, roles, independencia y controles | §3 |
| Hipótesis D1–D4, matriz de cobertura, sondas y base de contraste | §4 |
| Rúbrica descriptiva y límites del juicio | §5 |
| Costos por fase y prohibiciones de estimación | §6 |
| Supuestos falsables, privacidad y consentimientos | §7 |
| Limitaciones, riesgos, paradas y criticalidad | §8 |
| CASEPACK, micro ejemplo ficticio y preparación | §9 |
| Próximo paso y no metas | §11 |

## 11. Próximo paso y no metas

- **Revisión realizada:** se leyeron los cuatro documentos de `build-company` sin cambios ni ejecución de sondas. La revisión recomienda, sin adoptarlo, un recorte de método: disciplina de evidencia e independencia y su auditoría en `framework-exploracion-con-agentes-ia.md`. No es validación comercial ni prueba de comprensión.
- **Pendientes antes de aplicar:** el humano debe elegir el documento o recorte, resolver la referencia de versión 1.0/1.1 y completar línea base, referencia de contraste, consentimientos y presupuesto. La exposición previa a los ejemplos impide llamarlos automáticamente nuevos o no vistos.
- **Estado triple:** marco base preparado (este documento) ≠ caso a medida pendiente (requiere decisión humana y preparación, con revisión externa ya realizada) ≠ validación experimental pendiente (no realizada; nada aquí la declara).
- **Sin nueva DEC:** esto es una propuesta experimental, no una elección de producto adoptada; no se requiere nueva decisión registrada. Los archivos existentes del repositorio quedan sin cambios por este documento.
