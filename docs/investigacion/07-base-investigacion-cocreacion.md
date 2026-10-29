# Base de investigación por dimensiones de cocreación

*Sistematiza los avances existentes del proyecto en cuatro dimensiones — trazabilidad, comprensión humana, comprensión bilateral y conducción dinámica — para continuar la investigación en comprensión comparable y codirección. El cuerpo de dimensiones usa la investigación interna ya hecha (01–04, DEC-002 y las fases) y además consolida, en [Aportes de la ronda externa](#aportes-de-la-ronda-externa), cuatro rondas de consulta externa realizadas antes de esta actualización — una primera de exploración por perspectivas, una segunda de profundización centrada en las preguntas abiertas de D3 y D4, una tercera de profundización en transferencia y reparación y una cuarta sobre aclaración de preguntas, costo de evaluación e interacción dirigida —: se incorporan como aportes a las preguntas, no como validación del encuadre; la segunda ronda sí amplió las consultas públicas respecto de la primera. No adopta arquitectura elegida ni propone implementación. `05-arquitectura-inicial.md` y `06-cocreacion-humano-agente.md` no se usaron ni se citan como fuentes (exclusión explícita; una exploración que los consultó quedó excluida con su contenido).*

## Propósito y relación entre dimensiones

El problema de partida es la deuda cognitiva: documentos que el humano firma sin comprender del todo ([01, §1](01-cuello-de-botella-humano-agente.md); [DEC-002](../decisiones/DEC-002-cocreacion-humano-agente.md)). DEC-002 eligió **codirección con comprensión compartida comparable**: humano y agente codirigen el razonamiento y la construcción, cualquiera conduce donde comprende más, y el humano no es tope ni mero aprobador. Este documento organiza lo investigado en cuatro dimensiones operativas:

| Dimensión | Pregunta núcleo | Estado dominante | Brecha dominante |
|---|---|---|---|
| **D1 Trazabilidad** | ¿Qué razones, criterios y decisiones sostienen cada parte del documento? | Traza real en uso + esquema sistemático solo propuesto | Eficacia no probada; dependencias desfasadas |
| **D2 Comprensión humana** | ¿El humano puede explicar, proyectar y defender lo construido? | Prácticas con evidencia moderada y transferencia incierta | Sin evidencia en documentos largos reales |
| **D3 Comprensión bilateral** | ¿Ambas partes explican, corrigen y aplican criterios compartidos? | Fundamentos internos parciales, mecanismos de un solo lado | No hay evidencia de bilateralidad operativa en el proyecto |
| **D4 Conducción dinámica** | ¿Quién conduce cada tramo y cómo cambia? | Fundamentos para cambiar criterios, no para transitar conducción | Sin diseño ni observación de transiciones en el proyecto |

Las cuatro no son ejes independientes ni etapas fijas; se conservan separadas por continuidad operativa. La comprensión humana (D2) es **componente** de la bilateral (D3), no su igual ni un hito previo obligatorio.

## D1 · Trazabilidad

**Definición operativa** (síntesis propia de este documento, no resultado empírico): hacer explícitos criterios, decisiones, descartes y supuestos como registros tipados y versionados, con cita de origen, versión, dependencias e impacto, de modo que se pueda ver qué sostiene a qué y dónde propaga un cambio.

**Avances concretos.** Conviene distinguir dos clases: la traza que **ya existe** en este repositorio como práctica de investigación (decisiones con razones y descartes en `docs/decisiones/`, el propio registro 01–04 y las fases) y el esquema sistemático (tipos, IDs, plantillas, scripts) que las fases **propusieron pero no está construido**:

- **Tz-1** · Registros tipados con ID, estados y regla de versión: redacción sin cambio de sentido edita en el mismo ID; cambio de sentido sube `version` con `cambio:`; cambio de objeto crea ID nuevo y `reemplazado_por` ([encuadre-5, §4.2–4.3](fases/fase2-encuadre-5-artefactos.md)).
- **Tz-2** · Enlaces en una sola dirección hacia lo que sostiene, impacto y diff semántico calculados desde el frontmatter, y convención de commits que lista lo que el humano avaló ([encuadre-5, §1, §4.5–4.6](fases/fase2-encuadre-5-artefactos.md)).
- **Tz-3** · Bloque `Contrapeso` obligatorio: incertidumbre en primera persona, conflicto con criterios, alternativa descartada y qué no está verificado ([encuadre-3, §2.5](fases/fase2-encuadre-3-humano.md)).
- **Tz-4** · Marcas de verificación por afirmación (`✓ ≈ ? ✗`, quién y cuándo marcó, qué ítem se revisó) como insumo del presupuesto de lectura ([ronda1-T5-T6, T5.1](fases/fase3-ronda1-T5-T6.md)).
- **Tz-5** · Tratamiento de dependencias: enlaces sospechosos al cambiar un criterio o decisión, banderas `aplica/distingue/contradice`, diff de esqueleto argumental, y versionado por precedente (distinguir vs. revocar) ([ronda1-T5-T6, T6.1–T6.3](fases/fase3-ronda1-T5-T6.md); [ronda1-T1-T4, T1.4](fases/fase3-ronda1-T1-T4.md)).
- **Tz-6** · Registro de corridas: qué se verificó, con qué criterio (`C#@vN`) y con qué modelo ([encuadre-4, §6](fases/fase2-encuadre-4-operacion.md)).

**Evidencia y estado:** según la clasificación de la auditoría interna del propio corpus (que las marca como práctica documentada, sin eficacia medida con LLM: [auditoría-evidencia, #16–18](fases/fase3-ronda2-auditoria-evidencia.md)), las prácticas de soporte — enlaces sospechosos en ingeniería de requisitos, read-back aeronáutico, documentación de auditoría AS 1215 — están bien establecidas en sus dominios de origen; esta tarea no hizo comprobación primaria de esas fuentes ni de su eficacia. Riesgo identificado: `depende_de` desactualizado produce falsos negativos y sensación de seguridad injustificada ([auditoría-A, §4 riesgo 4](fases/fase2-auditoria-A-ceremonia.md)); el costo de ceremonia es el riesgo principal ([auditoría-A, §4 riesgo 1](fases/fase2-auditoria-A-ceremonia.md)).

**Qué reutilizar:** el esquema mínimo de campos (origen, cita, versión, contrapeso), la regla de versión en tres casos y el lenguaje de marcas por afirmación.

**Brecha:** nada prueba que el registro reduzca la lectura neta en lugar de sumarse a ella; eso quedó abierto en la auditoría ([auditoría-A, §2](fases/fase2-auditoria-A-ceremonia.md)).

## D2 · Comprensión humana

**Definición operativa** (síntesis, sin métrica validada): contrastar la comprensión mediante actos, no por lecturas: explicar sin mirar, proyectar ante lo nuevo (no repetir), defender ante preguntas de un revisor externo. El humano es participante activo, no aprobador ni lector-contador.

**Avances concretos** (propuestas de diseño, con su base empírica separada):

- **Ch-1** · Tipología de contactos con presupuesto de atención y disparadores de alerta en caliente (contradicción con criterio confirmado, cambio de premisa, acción irreversible) ([encuadre-3, §2.1](fases/fase2-encuadre-3-humano.md)).
- **Ch-2** · Primitivas de comprensión: explicar sin mirar con contraste, defensa por proyección, recordar primero en la reentrada ([encuadre-3, §3](fases/fase2-encuadre-3-humano.md); [ronda1-T2-T3, T2.1–T2.2, T3.2](fases/fase3-ronda1-T2-T3.md)).
- **Ch-3** · Selección integrada: lectura presupuestada derivada de las marcas de D1 más una muestra al azar, y una sola primitiva "explicar sin mirar → contraste" ([auditoría-integración, §3 M2, M6](fases/fase3-ronda2-auditoria-integracion.md)).
- **Ch-4** · Base conceptual: la delegación erosiona comprensión y autoría; el humano genera en los momentos que definen el trabajo; participación activa mínima como condición de la tesis ([01, §4](01-cuello-de-botella-humano-agente.md); [04, §2](04-acervo-mecanismos.md); [03, §7](03-informe-investigacion.md)).

**Evidencia y estado** ([auditoría-evidencia, #5–7, #20–22 y §2](fases/fase3-ronda2-auditoria-evidencia.md)):

- Autoexplicación: metaanálisis fuerte pero de aprendizaje escolar; su transferencia a "defender un documento propio" es incierta (#6).
- "Generar y luego comprender" preservó aprendizaje en N=52, con la condición clave sustentada en n=2: anecdótico (#22). I-PASS: cifra de la ronda 1 mal atribuida, corregida (#5); "justificar polariza" era sobrelectura — lo sostenible es que justificar no modera (#7).
- Muestreo de un párrafo al azar: sin poder estadístico suficiente (#20); el espaciado justifica la dirección de la reentrada graduada, no los umbrales 3/14 días (#21).
- Las reducciones de lectura de la selección integrada (~37 %/68 %) son **estimaciones no medidas** ([auditoría-integración, §2](fases/fase3-ronda2-auditoria-integracion.md)).

**Qué reutilizar:** la distinción recuerdo vs. proyección, el contraste sin puntaje y el presupuesto de lectura con muestreo (calibración aún por hacer).

**Brecha:** ninguna idea top tiene evidencia directa en documentos largos coescritos con IA ([auditoría-evidencia, §2](fases/fase3-ronda2-auditoria-evidencia.md)); riesgo Goodhart y autoevaluación circular del agente ([auditoría-A, §3.3](fases/fase2-auditoria-A-ceremonia.md)).

## D3 · Comprensión bilateral

**Definición operativa** (síntesis): comprensión compartida comparable de todo el asunto — razones, premisas, criterios, alternativas y consecuencias — con corrección recíproca y reformulación conjunta de objetivos y encuadre. Aportes desiguales son válidos durante el trabajo; la bilateralidad no exige igualdad de contribución ni conocimiento idéntico. La comprensión humana (D2) es un componente, conservado como dimensión propia por continuidad operativa, no un eje independiente ni una etapa fija.

**Avances concretos:**

- **Bi-1** · La decisión de fondo: codirección, cualquiera conduce donde comprende más, el agente también explicita la intención y los criterios del humano, y ambos pueden cuestionar y reformular encuadre y criterios ([DEC-002](../decisiones/DEC-002-cocreacion-humano-agente.md)).
- **Bi-2** · `Contrapeso` como estructura mínima de objeción parcial con formato fijo ([encuadre-3, §2.5](fases/fase2-encuadre-3-humano.md)).
- **Bi-3** · Linaje de ideas del agente hacia las humanas (`deriva_de`): la expansión "desde tu idea", no en su lugar ([ronda1-T1-T4, T4.2](fases/fase3-ronda1-T1-T4.md)).
- **Bi-4** · Disenso anclado en los descartes del propio humano: el contrapeso es el mejor caso de la alternativa descartada, con evidencia nueva si la hay ([ronda1-T5-T6, T5.5](fases/fase3-ronda1-T5-T6.md)).

**Evidencia y estado:** dentro del corpus y de las propuestas de este repositorio, los mecanismos de comprensión existentes son de un solo lado: miden y ejercitan la comprensión del humano (encuadre-3 §3; ronda1-T2-T3). Separar redactor y juez (T5.3) **busca limitar** la autoevaluación — intención de diseño, no eficacia probada —, pero **no es demostración de comprensión mutua**: un agente que parafrasea con fluidez más un humano que explica no basta para afirmar bilateralidad. Los fundamentos están puestos; la operación bilateral no está completada ni observada en este proyecto. El aporte de las rondas externas a estas preguntas se resume en [Aportes de la ronda externa](#aportes-de-la-ronda-externa).

**Brecha y agenda prioritaria:**

- **P1 (priorizada).** ¿Qué observación distingue comprensión compartida real de repetición o desacuerdo ritual? Candidatos observables sin reclamos de estados mentales: aplicación de criterios y razones compartidas en casos nuevos y contraejemplos por ambas partes; correcciones recíprocas aceptadas o rechazadas con razón. Pregunta investigativa, no hallazgo: que un "contrapeso" nunca altere una decisión **no prueba** ritual — un rechazo razonado o una confirmación con justificación también pueden demostrar comprensión; solo daría una razón para investigar con la calidad de la aplicación y la justificación. Evidencia que ayudaría a distinguir alternativas: si un contraejemplo propuesto por una parte es recogido y aplicado por la otra en un caso distinto, la bilateralidad gana sustento. *Candidato de observación práctica: registrar en la traza de D1 quién propuso y quién aplicó cada criterio y corrección — no se declara experimento ejecutado ni meta.*
- **P2.** ¿Qué indicadores de bajo costo son honestos? Contar correcciones recíprocas y desacuerdos es barato, pero un conteo bajo es tan vulnerable a Goodhart como uno alto (auditoría-A §3.3); por sí solo no distingue ritual de comprensión. Necesita acompañarse cualitativamente — cómo se aplicó y se justificó el criterio — y sirve como señal para decidir dónde mirar, no como medida de comprensión.

## D4 · Conducción dinámica

**Definición operativa** (síntesis): la conducción del razonamiento y de la construcción puede moverse entre humano y agente según quién comprende más en cada tramo ([DEC-002](../decisiones/DEC-002-cocreacion-humano-agente.md)). No es una secuencia de etapas fijas ni un liderazgo humano por defecto; la generación humana primero es salvaguarda contextual de momentos clave, no límite permanente.

**Avances concretos:**

- **Cd-1** · Fundamentos para cambiar criterios y resolver conflictos: CRI provisional aplicado de inmediato y confirmación con cita solo si es crítico; la pregunta única "¿distingue o revoca?" ante contradicción ([ronda1-T1-T4, T1.2, T1.4](fases/fase3-ronda1-T1-T4.md); [auditoría-integración, §3 M4, M5](fases/fase3-ronda2-auditoria-integracion.md)).
- **Cd-2** · Enrutamiento operativo por costo de error y escalamiento (niveles R0–R3, topes de escalamiento): asigna intensidad de verificación, **no** es liderazgo cognitivo ([encuadre-4, §3.1–3.3](fases/fase2-encuadre-4-operacion.md)).
- **Cd-3** · Disparadores de alerta en caliente (contradicción con criterio confirmado, cambio de premisa, acción irreversible) como ocasiones preexistentes donde la conducción podría cambiar de mano ([encuadre-3, §2.1](fases/fase2-encuadre-3-humano.md)).

**Evidencia y estado:** los cimientos (cambiar criterios, distinguir/revocar, alertas) existen como propuestas de este repositorio; **las transiciones de conducción no están diseñadas ni observadas aquí**. Esta salvedad acota el alcance a las propuestas actuales del proyecto; las rondas externas consultadas tampoco muestran transiciones por comprensión ([Aportes de la ronda externa](#aportes-de-la-ronda-externa)), y una exploración sobre conmutación de roles quedó excluida por extracciones en conflicto — es una exclusión de evidencia dañada, no una prueba de ausencia. Los estados de evidencia de los mecanismos que las sustentarían son débiles a especulativos ([auditoría-evidencia, §2](fases/fase3-ronda2-auditoria-evidencia.md)).

**Brecha y agenda prioritaria:**

- **P1 (priorizada).** ¿Qué señal observable sugiere que la conducción debería cambiar de mano? La asimetría es local a la tarea. Candidatos observables sin umbral: quién detecta primero la contradicción, quién propone el contraejemplo que se sostiene, latencia de corrección en cada dirección, criterio compartido aplicado con éxito por una sola parte. Evidencia que ayudaría a distinguir alternativas: si las detecciones y correcciones se concentran en una parte para un tramo, hay asimetría local. Un riesgo a investigar — no un resultado — es la deriva hacia aprobador que DEC-002 marca en su contrapeso: que el agente conduzca siempre sin correcciones humanas **no la implica por sí mismo**; dejaría de preocupar si el humano demuestra comprensión aplicando o defendiendo lo construido en casos nuevos. Por eso el riesgo se leería junto con las señales de comprensión (D2/D3), no con la frecuencia de liderazgo sola. *Candidato de observación práctica: anotar en la traza existente el iniciador de propuestas y correcciones (`generada_por` ya existe en las plantillas; encuadre-3 §4) — sin declarar experimento ejecutado, punto final ni política.*
- **P2.** ¿Quién inicia o propone la transición y quién responde por el compromiso? Distinción por resolver: rol de conducción en un tramo vs. responsabilidad final sobre el documento, que DEC-002 declara no simétrica. Lo que la agenda **no** incluye: convertir decisiones críticas humanas en tope del razonamiento del agente, ni hacer humano-primero fijo y obligatorio, ni un motor, umbral o protocolo de transición antes de tener señales observadas.

## Dependencias entre dimensiones

- **D1 → D3/D4:** la trazabilidad da evidencia observable (quién propuso, quién aplicó, qué criterio@versión), pero **no es comprensión**: un expediente completo puede coexistir con comprensión nula; "leer el registro" puede ser solo releer con otro nombre ([auditoría-A, §2](fases/fase2-auditoria-A-ceremonia.md); [01, §4](01-cuello-de-botella-humano-agente.md): resumir más da ilusión de comprensión).
- **D2 → D3:** la comprensión humana es necesaria pero no suficiente para la bilateral. La bilateralidad exige que **ambas partes** demuestren comprensión comparable de todo el asunto — razones, premisas, criterios, alternativas, consecuencias — con corrección recíproca; la dirección que falta hoy es que el agente explicite la intención, los criterios y las razones del humano (no solo las de su propia producción), sin mecanismo investigado.
- **D3/D4 → retroalimentación:** la conducción depende de señales de comprensión, pero conducir no exime de seguir aprendiendo: quien conduce un tramo seguiría necesitando explicarse y corregirse. Un candidato que ayudaría — sin campos ni reglas obligatorias — es que las transiciones queden visibles en la traza (D1) para poder auditarlas después.

## Aportes de la ronda externa

Este apartado consolida tres rondas de consulta externa ya realizadas: una primera de cinco exploraciones independientes por perspectivas y un verificador selectivo de citas, una [segunda ronda de profundización](#segunda-ronda-preguntas-abiertas-de-d3-y-d4) sobre las preguntas abiertas de D3 y D4, y una [tercera ronda de profundización](#tercera-ronda-profundización-en-transferencia-y-reparación) sobre transferencia y reparación, más una [cuarta ronda](#cuarta-ronda-aclaración-de-preguntas-costo-de-evaluación-e-interacción-dirigida) sobre aclaración de preguntas, costo de evaluación e interacción dirigida y un [mapa transversal](#mapa-transversal-de-preguntas-cuatro-rondas) del estado de las brechas. Los aportes pueden relacionarse con varias dimensiones donde corresponda, y una dimensión puede recibir aportes de varias perspectivas; **cinco perspectivas no son cinco dimensiones**, y la correspondencia solo se traza donde resulta útil — en particular, ninguna de las cuatro rondas aporta soporte empírico específico nuevo para D1. Ningún aporte valida por sí mismo el encuadre ni fuerza un mecanismo.

**Método.** Cuatro rondas de consulta externa: exploraciones por perspectiva y por pregunta abierta, cada una con un contraste independiente de fuentes seleccionadas. No es una revisión sistemática ni hubo experimentos propios. La profundidad de lectura de cada fuente está en el [registro de fuentes](#registro-de-fuentes-profundidad-y-límites).

**Cómo leer los aportes.** Para cada perspectiva se separan: el resultado restringido que sí respalda la fuente, la implicación como hipótesis para tupaqnin (transferencia, no resultado), la cautela y lo que queda pendiente. Un estudio empírico consultado solo en parte (por ejemplo, por abstract) sigue siendo un estudio empírico con lectura limitada: no se reclasifica como especulación teórica, y la profundidad limitada se declara. Las teorías (Cooke, Horvitz) son de otro tipo que los estudios empíricos y no se presentan como evidencia de efecto. Ninguna fuente confirma la bilateralidad de todo el asunto en documentos largos humano-LLM, ni la transición de conducción por comprensión, ni aporta soporte específico nuevo para D1.

### Perspectiva de diálogo y anclaje (grounding)

- **Resultado restringido:** en corpus de diálogo, los LLM generan menos actos de anclaje conversacional que los humanos, y los datos de preferencia contemporáneos los reducen aún más ([Shaikh et al. 2024](https://aclanthology.org/2024.naacl-long.348/)); la lectura posterior del artículo principal (§1–10) con contraste independiente de §6–7.3 y su tabla 3 precisa: la frecuencia de actos sube con las incrustaciones de prompt, el κ de clarificación empeora mientras el de seguimiento y reconocimiento mejora — dirección mixta, no empeoramiento uniforme ni prueba de que la cantidad nunca ayude. Las etiquetas de acuerdo marcan actos de anclaje, no comprensión global; las reparaciones iniciadas por el modelo quedaron fuera del alcance del estudio. No se citan valores de κ: su escala admite ambigüedad de extracción. La teoría de cognición de equipo humano describe la actividad como contextual, no como estructuras idénticas estáticas (Cooke et al. 2013).
- **Implicación (hipótesis):** la corrección recíproca que D3 espera (Bi-1, Bi-4) no sería automática: un modelo que asume terreno común sin repararlo podría simular acuerdo. Los actos de reparación conjunta son candidatos de observación, no una medida.
- **Cautela:** el corpus es conversacional y en inglés, no coautoría bilateral de documentos largos humano-LLM; no prueba comprensión global en ninguna dirección, y la teoría de equipos humanos no se transfiere por analogía a una díada humano-LLM.

### Perspectiva de transferencia de conocimiento humano–IA

- **Resultado restringido:** en un estudio con N=118 (co-ideación con IA y luego implementación independiente sin IA), el desempeño del modelo en benchmarks se correlacionó con la transferencia al humano pero con valores atípicos: capacidad ≠ transferencia ([Shi et al. 2025, "KITE"](https://proceedings.neurips.cc/paper_files/paper/2025/file/975d11c51406cd10be48e47b36fb8698-Paper-Conference.pdf)). Su marco es conceptualmente bidireccional, pero la prueba empírica fue IA→humano.
- **Implicación (hipótesis):** el estudio probó transferencia mediante implementación humana independiente, sin IA ni transcripciones; para tupaqnin, contrastar transferencia en casos nuevos no exigiría copiar ese diseño — no se prescribe experimento ni replicación obligatoria —, y la meta de comprensión de todo el asunto no se reduce a implementación de código.
- **Cautela y pendientes:** KITE no midió empíricamente el aprendizaje del agente (humano→IA); esto no declara unidireccionalidad del marco ni ausencia de evidencia en toda la literatura. La generación propia seguida de consolidación y la corregulación vs. regulación conjunta/compartida siguen siendo pistas pedagógicas rivales, sin beneficio medido ni eficacia confirmada (Järvelä & Hadwin 2013, consultado solo en fragmentos de abstract/índice; pendiente de lectura completa).

### Perspectiva de iniciativa mixta y modos de conducción

- **Resultado restringido:** principios de iniciativa mixta — actuar, preguntar o esperar según incertidumbre, costos y utilidades — con prototipo ([Horvitz 1999](https://erichorvitz.com/chi99horvitz.pdf), leído completo): teoría de diseño de interfaz, sin validación de liderazgo cognitivo ni de quién comprende más. Un asistente opinante influyó las vistas escritas y las actitudes posteriores de usuarios que redactaron la mayoría de las oraciones (Jakesch et al. 2023). El abstract de Maier et al. 2025 reporta que el modo dirigido por el modelo sube la calidad y baja diversidad y propiedad, y el dirigido por el humano con reflexión las preserva; la lectura posterior de §4–7 (métodos, resultados, replicación y límites, sin apéndices completos; contraste independiente parcial: abstract y fragmento de resultados) confirma N=486 y 640, con calidad, diversidad y propiedad como variables medidas — no validación de comprensión — y modos fijos que no conmutan ni combinan dentro del diseño (lectura del primer explorador, no un experimento de transiciones).
- **Implicación (hipótesis):** para D4, la iniciativa (quién actúa) y la comprensión (quién explica y defiende) son variables distintas; para D2, que el humano redacte más no garantiza que defienda su posición. Calidad, propiedad, diversidad y desempeño no se sustituyen entre sí, y la comprensión es otra variable más.
- **Cautela:** Horvitz y Jakesch no validan transiciones por comprensión ni protocolos de iniciativa; Jakesch es un estudio de un tópico con GPT-3 y persistencia no medida, sin inmunidad ni determinismo personal. De Maier no se leyeron los apéndices completos y el contraste independiente fue parcial; la publicación en ACM sigue sin verificarse (el encabezado CHI'26 de la fuente es un metadato, no prueba de publicación). Nada de esto elige "el mejor modo".

### Perspectiva organizacional (contenido primario débil)

- **Resultado restringido:** limitado. De Carson et al. 2007 solo hay metadatos y la primera línea del abstract; de Austin 2003, el abstract vía índice (precisión compuesta, consenso y especialización, sin aislar la localización de pericia); de Anderson & Kilduff 2009, fragmentos; de Wu & Cormican 2021 (Frontiers), abstract, definiciones, hipótesis y método, con los coeficientes de resultados sin leer. No se rellenan los huecos.
- **Implicación (hipótesis):** la analogía con equipos humanos queda como fuente de hipótesis para D4, no de prueba; simultáneo-por-aspecto, rotación y autoridad negociada siguen **abiertas**, sin política mejor.
- **Cautela:** la confianza o influencia percibida pueden confundir la evaluación de comprensión; es una cautela a vigilar, no un resultado observado en operación.

### Perspectiva de evidencia agregada humano–IA y alineación

- **Resultado restringido:** metaanálisis de combinaciones humano–IA: en los experimentos revisados, el desempeño medio de humano+IA superó al del humano solo — en sus términos, augmentación: mejorar respecto al humano solo — pero quedó por debajo del mejor de los dos por separado, con heterogeneidad muy alta (I²=97,7 %); el subgrupo creativo tiene un punto estimado positivo pero no estadísticamente significativo (g=0,19, IC [−0,09; 0,48], p=0,18 — sin concluir efecto nulo) ([Vaccaro et al. 2024](https://doi.org/10.1038/s41562-024-02024-1)). Sharma et al. 2023, en cinco modelos y cuatro tareas de texto libre, distinguen dos cosas: asistentes que exhiben acuerdo con el usuario (sycophancy) y modelos de recompensa entrenados con valoraciones humanas, cuya preferencia por coincidir con las vistas del usuario puede conflictuar con la verdad en ciertos contextos; no se afirmó que todos los asistentes actuales lo hagan ni que todo acuerdo del usuario esté mal. Las explicaciones en cadena de pensamiento pueden tergiversar las razones reales del modelo (Turpin et al. 2023, solo abstract; cautela pendiente de lectura).
- **Implicación (hipótesis):** que el producto mejore no implica comprensión, ni sinergia siempre; y la deferencia documentada del modelo da una razón concreta para no leer "el agente acepta" como criterio validado: un rechazo o una reafirmación razonados son ambos legítimos, y contar desacuerdos o correcciones no mide comprensión.
- **Cautela:** que más del 95 % de los escenarios incluidos terminen con decisión final humana describe los escenarios revisados, no es una ley; tampoco se repite aquí la lectura antigua de "ganancias de creación probadas".

### Segunda ronda: preguntas abiertas de D3 y D4

La segunda ronda trató las preguntas abiertas de D3 y D4 con cuatro exploraciones temáticas y contraste independiente selectivo. Clasificación de lo obtenido:

**Resultados admitidos** (con la profundidad declarada y sin cifras ambiguas):

- **[Abuhaimed & Sen 2022 (HHAI)](https://www.hhai-conference.org/wp-content/uploads/2022/06/hhai-2022_paper_53.pdf):** N=130 (65 y 65), pericia simulada y rol de asignador fijo; la confianza de los agentes está mal calibrada y varía por tarea. Es evidencia de asignación de tareas, no de liderazgo cognitivo ni de comprensión; las ventajas del agente son reportes del primer explorador sin verificación independiente, y se omiten sus tamaños de efecto.
- **[MOSAAIC (Issak et al., ICCC 2025)](https://computationalcreativity.net/iccc25/wp-content/uploads/papers/iccc25-issak2025mosaaic.pdf):** revisión conceptual (pp. 1–6 de 11) que distingue autonomía, iniciativa y autoridad; no es un experimento, y su análisis de seis casos no se leyó ni se contrastó.
- **[Wu & Cormican 2021](https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2020.569198/full)**: admitido solo como observación de fuente, método y constructo (su constructo es liderazgo en redes de influencia, no comprensión); con los coeficientes de resultados sin leer, no se admite de este estudio ningún efecto empírico. Las lecturas ampliadas de **Shaikh** y **Maier** ya resumidas arriba sí sostienen resultados restringidos propios.

**Hipótesis para tupaqnin** (transferencia, no resultado):

- La aplicación de criterios en casos nuevos y la reparación recíproca siguen siendo candidatos de observación sin validar; que la clarificación empeore con más prompting (Shaikh) da una razón concreta para mirar las reparaciones recíprocas, no una medida.
- La confianza mal calibrada y variable por tarea (Abuhaimed & Sen) sugiere no leer "quién tiene más confianza" como sustituto de "quién comprende más" al decidir un tramo (D4): la asimetría local queda por contrastar contra desempeño, no contra confianza declarada.
- Rotación por tramo, simultáneo-por-aspecto y autoridad negociada siguen abiertas; el vocabulario autonomía/iniciativa/autoridad de MOSAAIC organiza la discusión sin elegir política.
- Producto, propiedad percibida, coordinación y comprensión quedan separadas: esta separación es una hipótesis propia del proyecto, no un constructo medido en esas fuentes. En Maier se midieron calidad, diversidad y propiedad (la comprensión no fue medida) y en Abuhaimed & Sen, asignación de tareas con confianza mal calibrada; ninguno midió "coordinación", y que la asignación de tareas funcione no valida la coordinación. Ninguna variable se sustituye por otra.

**Exclusiones registradas.** Una exploración temática (D3-P2) consultó `05`/`06` contra la exclusión explícita: su encuadre y sus cifras de snippets no se admiten como evidencia y este documento no incorpora su contenido. De una exploración sobre conmutación de roles ([arXiv 2509.20666](https://arxiv.org/pdf/2509.20666)) se excluyen todos los conteos, efectos y costos causales de transición: sus extracciones entraron en conflicto (número de decisiones y presencia de centipawn difieren entre lecturas) y la versión subyacente no está resuelta; no se atribuye causa sin diagnóstico.

**Fuentes pendientes, marcadas "no verificado"** (lectura no lograda, parcial o solo abstract; no se usan como evidencia):

| Fuente | Estado de lectura | Nota |
|---|---|---|
| [Li et al. 2024](https://arxiv.org/abs/2402.07632) | Solo abstract | Calibración de confianza: detección vs eficacia de decisión; sin métodos ni números verificados |
| [Mutual Theory of Mind 2024](https://arxiv.org/abs/2409.08811) | Solo abstract | Percepciones y desempeño de equipo podrían disociarse; sin N ni instrumentos verificados; no se etiqueta como comprensión efectiva |
| [Cleo 2026](https://arxiv.org/abs/2603.02050) | Parcial (abstract, introducción, estudio 1); ampliada con la v5 en la cuarta ronda | Profundidad histórica conservada; lectura v5 y límites en el [registro](#registro-de-fuentes-profundidad-y-límites), entrada 15 |
| Yang et al. 2022 ([PDF](https://metacog.bnu.edu.cn/pdf/articles/2022/YangZhaoYuanLuo2022.pdf)) | Extracción de PDF fallida | Pendiente |
| DeChurch & Mesmer-Magnus 2010 ([PDF](https://atlas.northwestern.edu/wp-content/uploads/2017/03/DeChurch-_-Mesmer-Magnus-2010-JAP.pdf)) | Sin texto leído | Pendiente |
| [Manheim & Garrabrant 2018](https://arxiv.org/abs/1803.04585) | Snippet de taxonomía teórica | Sin componente empírico |
| [Kosmyna et al. 2025](https://arxiv.org/abs/2506.08872) | Abstract de preprint | Sin verificación del texto completo; sin reclamos causales de EEG |
| SSRL ([Frontiers Educ. 2020](https://www.frontiersin.org/journals/education/articles/10.3389/feduc.2020.580543/full)) | Obtenida, no leída | Pendiente |
| [IJCSCL 2019 (Springer)](https://link.springer.com/article/10.1007/s11412-019-09313-2) | Sin leer | Pendiente |
| Alineación sintáctica ([PMC6677274](https://pmc.ncbi.nlm.nih.gov/articles/PMC6677274/)) | Obtención fallida, solo snippet | Pendiente |
| Pactos conceptuales ([PubMed 8921603](https://pubmed.ncbi.nlm.nih.gov/8921603/)) | Snippet; nueva extracción en la tercera ronda quedó incompleta | Pendiente |

Pan & Rickard 2018 salió de esta lista en la tercera ronda: la extracción fallida se resolvió con lectura real de métodos y resultados ([registro de fuentes](#registro-de-fuentes-profundidad-y-límites), entrada 16); se conserva su estado histórico. Este listado refleja las rondas de consulta y no es exhaustivo: quedan pendientes de rondas anteriores **Turpin et al. 2023** (solo abstract) y **Järvelä & Hadwin 2013** (fragmentos de abstract/índice), ya anotadas en el [registro de fuentes](#registro-de-fuentes-profundidad-y-límites) (entradas 9 y 11), sin repetir aquí sus URLs. También queda pendiente **Martijn 2026**: el intento de leer el PDF vía HTTP devolvió 500 y la URL quedó incompleta en el reporte, así que no se reconstruye el enlace. Dos pistas solo de búsqueda, sin peso extra: [un estudio de diálogo humano–humano (SIGDIAL 2024)](https://aclanthology.org/2024.sigdial-1.51/) y [arXiv 2305.16938](https://arxiv.org/abs/2305.16938), ambas registradas como pendientes opcionales; de la segunda (Mosbach et al.), la cuarta ronda leyó el abstract — sus métodos y criterios generales siguen sin leer, con la profundidad histórica actualizada desde snippet.

**Agenda de profundización al cierre de la segunda ronda:** métodos y resultados de las fuentes núcleo, costo humano de evaluar la comprensión, y uptake en contexto vs aprendizaje persistente. La tercera ronda ejecutó una parte del primero — métodos y resultados de [Pan & Rickard 2018](https://pdf.retrievalpractice.org/transfer/Pan_Rickard_2018.pdf), ver abajo —; el costo humano y el uptake en contexto siguen abiertos. Ninguna ausencia empírica declarada en esta ronda va más allá del material consultado.

### Tercera ronda: profundización en transferencia y reparación

La tercera ronda trató dos frentes — transferencia del aprendizaje con práctica y reparación asistida por retroalimentación — con dos exploraciones focalizadas y un contraste independiente de las tres fuentes, sin experimentos propios. Para cada fuente se separan resultado restringido, hipótesis y cautela; los números se limitan a los que ambas lecturas confirman.

**Transferencia del aprendizaje con práctica (D2).**

- **Resultado restringido:** [Pan & Rickard 2018](https://pdf.retrievalpractice.org/transfer/Pan_Rickard_2018.pdf), metaanálisis publicado en Psychological Bulletin; métodos, resultados y discusión leídos por el explorador de transferencia y el revisor independiente (pp. 4–7, 19–23 y 25–40; la tabla 1 completa y los forest plots no se inspeccionaron). 192 efectos de 122 experimentos en 67 artículos (N=10 382): d global ≈ 0,40, IC [0,31; 0,50] frente a reexposición o re-estudio, no frente a ausencia de entrenamiento. Aplicación e inferencia positivas; resolución de problemas con IC que cruza 0 y heterogeneidad — sin concluir efecto nulo ni transferencia lejana nula. Las categorías de transferencia cercana/lejana son preliminares y no lineales; la retención condiciona la transferencia (el desempeño inicial predice), sin ley universal del tipo "el recuerdo no predice". Sesgo de publicación al menos moderado, con ajustes dependientes del modelo; muestras sobre todo de estudiantes en laboratorio y transferencia lejana subrepresentada. Mide efectos de intervención de práctica con prueba: no es validez de instrumentos diagnósticos de comprensión ni prueba de un harness propio. La extracción fallida previa quedó resuelta con lectura real; el costo de tiempo no se cuantifica en las secciones leídas, lo que no es ausencia global.
- **Implicación (hipótesis):** para tupaqnin, la práctica de aplicar y explicar razones podría favorecer la proyección ante lo nuevo (D2). Esa transferencia desde los contextos estudiados aún no está comprobada: no convierte efectos de entrenamiento en validez diagnóstica ni en soporte del esquema de D1.
- **Cautela:** investigación de entrenamiento en aprendizaje escolar, mayormente de laboratorio y con sesgo de publicación; sin estadísticas propias de este documento.

**Desempeño asistido vs. aprendizaje independiente (D2/D3).**

- **Resultado restringido:** [Bastani et al.](https://atelierdesfuturs.org/wp-content/uploads/2025/07/ssrn-4895486.pdf), preprint SSRN 4895486 leído en esa versión (la publicación en PNAS 2025, DOI 10.1073/pnas.2422633122, no se leyó ni verificó); explorador en pp. 1–2, 6–10 y A4 de calificación, contraste independiente en pp. 1–14. Práctica escolar de matemática aleatorizada por clase con GPT Base, GPT Tutor y control, seguida de examen sin acceso con problemas conceptualmente similares: con GPT Base, la práctica es mejor y el examen peor; con GPT Tutor, la práctica es mejor y el examen es estadísticamente indistinguible del control — sin concluir efecto nulo ni garantizar ausencia de daño. Calificación independiente con rúbrica de docentes.
- **Implicación (hipótesis):** desempeño asistido ≠ aprendizaje independiente: que el agente ayude a escribir mejor no acredita que el humano aprenda, y el extremo adverso (GPT Base) muestra que puede incluso ir en contra.
- **Cautela:** matemática escolar, no lectura ni comprensión bilateral de documentos largos, sin reclamo de transferencia novedosa general; horas y costo sin leer (el número de calificadores no es métrica de costo).

**Reparación con memoria de retroalimentación (D3).**

- **Resultado restringido:** [Tandon et al. 2022, Learning to Repair](https://aclanthology.org/2022.findings-naacl.26.pdf), Findings de NAACL; artículo principal completo con límites de sección declarados y contraste independiente de arquitectura, datos, RQ1–RQ2 y límites §8.1–8.2. Corrector T5-XXL entrenado sobre ProScript congelada, más memoria externa de retroalimentación que crece durante el despliegue: reutilizar retroalimentación humana→sistema ayuda sobre 72 perturbaciones de ISET (~20 % lingüísticas, ~80 % analógicas), sin medir comprensión plena, y la retroalimentación engañosa daña. La métrica EM (edición estructural contra el gold) no mide comprensión y arrastra falsos negativos léxicos. Aplicar retroalimentación no es juzgar la calidad de los criterios; no es reparación recíproca en ambas direcciones ni uptake de criterios en contexto; la memoria creciente no son actualizaciones de pesos y no se verificó persistencia entre sesiones. El estudio declara costosa la retroalimentación general reutilizable, sin cuantificar el tiempo humano; los efectos del abstract y las tablas no se reconciliaron, así que se reporta cualitativamente.
- **Implicación (hipótesis):** la reparación de una sola dirección con memoria de retroalimentación es candidato de observación para D3-P1/P2: reutilizar correcciones humanas ayuda mecánicamente, pero no muestra corrección recíproca ni uso de criterios compartidos. Ninguna fuente de la ronda adopta esa arquitectura para el proyecto.
- **Cautela:** corrector entrenado de antemano, sin persistencia entre sesiones verificada; el efecto cuantitativo queda sin número hasta reconciliar abstract y tablas.

**Sin resolver en el material consultado:** el costo de evaluar la comprensión, la reparación iniciada por la máquina, el uptake de criterios generales en contexto, la bilateralidad de todo el asunto y la negociación de liderazgo.

### Cuarta ronda: aclaración de preguntas, costo de evaluación e interacción dirigida

La cuarta ronda trató tres frentes — aclaración de preguntas por el agente, costo de la interacción y de evaluar, e interacción humana dirigida en co-edición — con tres exploraciones, un contraste independiente de las cuatro fuentes y un seguimiento por fragmentos de dos de ellas, sin experimentos propios. Para cada fuente se separan resultado restringido, hipótesis y cautela; la clasificación incluye lo pendiente y lo excluido, no solo lo admitido.

**Aclaración de preguntas y detección de ambigüedad (D3/D4).**

- **Resultado restringido:** [CLAMBER](https://arxiv.org/pdf/2405.12063v2) (explorador: 21 pp completas; validador: pp. 1–12 con §1–7 de límites y apéndices A–C; D–F sin leer): clasificación estática de ambigüedad y generación de una pregunta de aclaración **provocada por prompt**, con cinco modelos (2024, inglés). La utilidad se midió como respuestas binarias sobre la pregunta, no como malentendido resuelto ni uptake de criterios. Las preguntas de referencia vinieron de GPT-4 con plantillas validadas por humanos; el apéndice de fiabilidad lo leyó solo el explorador, sin confirmación independiente. La "ignorancia epistémica" es inferencia de los autores, no prueba causal.
- **Implicación (hipótesis):** la cadena detectar→preguntar→incorporar la respuesta→aplicar el criterio en un caso nuevo es un candidato de observación para D3-P1/P2, con evidencia separada por eslabón: lo leído cubre detección y pregunta; incorporación y aplicación quedan sin evidencia.
- **Cautela:** iniciativa provocada, no natural ni reparación en vivo; sin reclamo para todos los modelos.

**Clarificación dirigida por etapas (D3).**

- **Resultado restringido:** [ClarifyWhenNecessary](https://aclanthology.org/2025.findings-naacl.306.pdf), fuente nueva (no estaba en el registro previo); el contraste independiente leyó §1–8 de métodos y resultados, sin apéndices: etapas de decidir aclarar, generar e incorporar la respuesta, con el diseño centrado en **cuándo** aclarar; el dataset oráculo (basado en GPT-3.5) provee pregunta y respuesta, y los modelos evaluados responden con la clarificación; ayuda de manera no uniforme y en algunas métricas de MT empeora. El presupuesto se define como porcentaje de ejemplos con aclaración permitida, no como minutos ni costo real de evaluar comprensión.
- **Implicación (hipótesis):** separar "decidir cuándo aclarar" de "generar" y de "incorporar" ordena la cadena de reparación; el presupuesto por porcentaje no responde cuánto cuesta evaluar.
- **Cautela:** sin criterios generales transferibles a casos nuevos ni prueba de que el aprendiz comprenda el asunto completo.

**Costo de interacción y de evaluación (D2/D3).**

- **Resultado restringido:** [estudio de costo de interacción](https://arxiv.org/html/2609.21117v1), abstract del explorador más fragmentos de seguimiento (§3.1 con la definición de Q y C, §3.3, validación humana y apéndices C/F): Q mide calidad del artefacto con un juez LLM (rúbricas 1–5); la validación humana de dos autores sobre un subconjunto no retuvo su correlación como métrica. C mide el trayecto por proxies — tokens y turnos, con el tiempo como alternativa gruesa —; el contraste dentro de la tarea usa z(Q)−z(C) como señal de productividad, si se necesita, sin complicar. Lo leído no mide tiempo humano de evaluar comprensión; esa ausencia vale solo para las secciones leídas.
- **Implicación (hipótesis):** costo de interacción ≠ costo de evaluación de la comprensión; los proxies de C son candidatos baratos para vigilar gasto, no para acreditar comprensión.
- **Cautela:** el resultado destacado del abstract no se verificó a nivel de método y no se cita aquí; el artículo no se leyó completo.

**Interacción humana dirigida en co-edición (D4).**

- **Resultado restringido:** [Cleo 2026, versión v5](https://arxiv.org/html/2603.02050v5), profundidad ampliada respecto de la ronda previa (explorador §1–5.1; seguimiento §2.1–2.3, §4.1.1–4.1.2, §4.3, §5.1 con DI1–DI2 y §5.5): el usuario interviene y redirige, y el agente infiere la retroalimentación — distinguiéndola del trabajo en paralelo y de la co-edición — y adapta su plan. Las condiciones son cualitativas, no predictores causales. §5.5 indica que los controles pueden reducir la negociación cambio por cambio; el costo social de humano↔agente no se comparó con el de humano↔humano. No hay traspaso bilateral de conducción razonado por comprensión. La v5 no se comparó con la versión citada antes: es profundidad nueva por versión leída, no contradicción.
- **Implicación (hipótesis):** distinguir retroalimentación de trabajo en paralelo y de co-edición es un candidato de observación para D4-P1; la adaptación del plan ante una intervención no equivale a conducción negociada.
- **Cautela:** condiciones cualitativas de un sistema concreto; sin conteos adoptados.

**Pendientes y exclusiones de la ronda** (enlaces compactos; sin efectos admitidos):

- [ChooseYourAgent](https://arxiv.org/html/2602.12089v3): explorador §1–3.3 sin resultados; los resultados numéricos no se admiten.
- [Mosbach et al.](https://arxiv.org/abs/2305.16938): el abstract quedó leído en esta ronda; métodos y criterios generales siguen sin leer (profundidad histórica actualizada desde snippet).
- [Ranaldi](https://arxiv.org/abs/2311.09410v4): solo abstract, pendiente opcional no verificado.
- [Mutual Theory of Mind](https://arxiv.org/abs/2409.08811): abstract repetido, sin profundidad añadida ni nueva marca de hallazgo.
- Shared mental models ([Taylor & Francis](https://www.tandfonline.com/doi/full/10.1080/1463922X.2022.2061080), [PMC](https://pmc.ncbi.nlm.nih.gov/articles/PMC8712333/)): HTTP 403 y bloqueo de JavaScript; sin efectos; los fragmentos de minutos no se verificaron y no se citan números.
- Negociación de autoridad (IEEE): solo abstract de un agregador; la URL primaria está indisponible y no se inventa enlace. [Negociación de planes (ACM)](https://dl.acm.org/doi/10.1145/3846181): HTTP 403; sin efectos.

**Sin resolver en el material consultado:** la bilateralidad de todo el asunto y ambas direcciones de comprensión siguen sin validación; el costo humano de evaluar la comprensión no se midió; la cadena completa de reparación — incorporar la respuesta y aplicar el criterio en un caso nuevo — no tiene evidencia observada.

### Mapa transversal de preguntas (cuatro rondas)

Estado de las cinco brechas que las rondas no cerraron; sin priorizar mecanismos — la elección de por dónde continuar se encarga aparte.

| Brecha | Avance acumulado | Límite | Pregunta que queda |
|---|---|---|---|
| Costo de evaluar la comprensión | Proxies de costo de interacción: calidad juzgada por LLM (Q) y tokens/turnos (C); los controles pueden reducir la negociación por cambio (Cleo §5.5) | Proxies ≠ tiempo humano evaluando comprensión; la correlación de la validación humana no se retuvo como métrica | ¿Cómo se mide el costo real de evaluar comprensión sin sustituirlo por proxies? |
| Reparación iniciada por la máquina | Aclaración provocada por prompt (CLAMBER) y adaptación del plan ante intervención (Cleo) | Iniciativa provocada, no natural; reparación en vivo sin observar; utilidad binaria de la pregunta, no uptake | ¿Cuándo surge la aclaración sin prompt y cómo se incorpora la respuesta? |
| Uptake de criterios generales en casos nuevos | Cadena detectar→preguntar→incorporar→aplicar como candidato de observación; clarificación por etapas (ClarifyWhenNecessary) | Ninguna fuente muestra transferencia de criterios generales a casos nuevos | ¿Se aplica un criterio compartido en un caso distinto del que lo originó? |
| Bilateralidad de todo el asunto | Condiciones cualitativas de interacción humano-agente (Cleo); advertencias sobre deferencia (rondas previas) | Condiciones cualitativas, no predictores causales; ninguna dirección de comprensión de todo el asunto queda validada | ¿Qué observación distingue comprensión bilateral de paralelismo bien coordinado? |
| Negociación de liderazgo | Vocabulario autonomía/iniciativa/autoridad (segunda ronda); iniciativa ≠ comprensión (primera ronda) | Control e intervención ≠ conducción negociada; sin comparación del costo social humano-agente frente al humano-humano | ¿Qué señal indicaría un traspaso de conducción razonado por comprensión? |

### Consecuencias para las preguntas de las dimensiones

Las siguientes son consecuencias para las preguntas abiertas, no campos ni reglas obligatorios:

- **Para D3-P1/P2:** las señales candidatas (quién propuso y quién aplicó, correcciones recíprocas) ganan una advertencia empírica: la sycophancy documentada en los modelos de Sharma y la influencia de asistentes opinantes (Jakesch) hacen que "el agente está de acuerdo" o "el humano escribió la mayoría" sean señales débiles por sí solas.
- **Para D4-P1/P2:** la iniciativa mixta (Horvitz) y la distinción entre interacción dirigida por el modelo y dirigida por el humano (Maier) aportan vocabulario de diseño, no un criterio de transición; ningún estudio de la ronda muestra transición de conducción por comprensión.
- **Para D1:** la ronda no aporta soporte empírico específico nuevo; su utilidad como soporte de observación (quién propuso, quién aplicó) sigue siendo candidata propia del proyecto.
- **Para D2:** la comprensión interactiva no reemplaza la defensa individual del humano de todo el asunto; siguen siendo complementarias.
- **Para D3-P1/P2 (segunda ronda):** la reparación conjunta gana una lectura empírica conversacional con dirección mixta (Shaikh) y la confianza mal calibrada (Abuhaimed & Sen) añade un confusor a vigilar; la reparación recíproca sigue siendo candidato de observación, no medida.
- **Para D4-P1/P2 (segunda ronda):** los modos fijos de Maier y el asignador fijo de Abuhaimed & Sen no muestran transiciones por comprensión; distinguir autonomía, iniciativa y autoridad (MOSAAIC) ordena el vocabulario sin elegir política.
- **Para D2 (tercera ronda):** la práctica con prueba transfiere mejor que el re-estudio (Pan & Rickard) y sustenta proyectar ante lo nuevo como investigación de entrenamiento; no es validez de diagnóstico de comprensión ni prueba de harness propio.
- **Para D3-P1/P2 (tercera ronda):** reutilizar retroalimentación humana ayuda en la reparación de una sola dirección (Tandon) y el desempeño asistido no es aprendizaje independiente (Bastani); ninguna de las dos muestra corrección recíproca ni uptake general de criterios.
- **Para D2 (cuarta ronda):** los proxies de costo (juez LLM, tokens/turnos) no miden tiempo humano evaluando comprensión; costo de interacción ≠ costo de evaluación.
- **Para D3-P1/P2 (cuarta ronda):** la cadena detectar→preguntar→incorporar→aplicar en caso nuevo separa eslabones con evidencia distinta (CLAMBER, ClarifyWhenNecessary); la utilidad binaria de la pregunta y la clarificación por etapas no acreditan uptake de criterios ni comprensión en ambas direcciones.
- **Para D4-P1/P2 (cuarta ronda):** la adaptación del plan ante intervención humana (Cleo) es observación candidata; control e intervención no son conducción negociada, y los controles pueden reducir la negociación por cambio sin comparación de costo social.

Observaciones discriminantes candidatas sugeridas por las fuentes (ninguna obligatoria): si el puntaje del modelo en benchmarks y la transferencia real se separan (los valores atípicos de KITE), la observación útil sería medir transferencia en casos del proyecto, no inferirla del rendimiento del modelo; y para distinguir comprensión de deferencia (Sharma, Jakesch), observar si una parte mantiene una posición razonada frente al desacuerdo de la otra, en lugar de cuántas veces coincide.

### Registro de fuentes, profundidad y límites

| # | Fuente | Tipo | Profundidad de lectura | Límite principal |
|---|---|---|---|---|
| 1 | [Shaikh et al. 2024, Grounding Gaps](https://aclanthology.org/2024.naacl-long.348/) | Estudio empírico (corpus) | Artículo principal §1–10; §6–7.3 y tabla 3 contrastadas de forma independiente | Diálogo conversacional en inglés, no coautoría bilateral de documentos; reparaciones iniciadas por el modelo fuera del alcance |
| 2 | [Cooke et al. 2013, Interactive Team Cognition](https://doi.org/10.1111/cogs.12009) | Teoría (equipos humanos) | Abstract | Analogía de equipos, no díadas humano-LLM |
| 3 | [Shi et al. 2025, KITE](https://proceedings.neurips.cc/paper_files/paper/2025/file/975d11c51406cd10be48e47b36fb8698-Paper-Conference.pdf) | Estudio empírico (N=118) | Artículo principal pp. 1–13, sin apéndice completo | Prueba empírica solo IA→humano |
| 4 | [Horvitz 1999, Mixed-Initiative UIs](https://erichorvitz.com/chi99horvitz.pdf) | Teoría / principios de diseño | Lectura completa | Sin validación de liderazgo cognitivo |
| 5 | [Jakesch et al. 2023](https://doi.org/10.1145/3544548.3581196) | Estudio empírico | §3, §4.1–4.5, §5.1–5.3 | Un tópico, GPT-3; persistencia no medida |
| 6 | [Maier et al. 2025](https://arxiv.org/html/2510.23324v3) | Estudio empírico, versión arXiv v3 consultada | §4–7 (métodos, resultados, replicación, límites), sin apéndices completos; contraste independiente parcial (abstract + fragmento de resultados) | Mide calidad/diversidad/propiedad, no comprensión; modos fijos sin conmutación; publicación en ACM no verificada |
| 7 | [Vaccaro et al. 2024](https://doi.org/10.1038/s41562-024-02024-1) | Metaanálisis | Abstract, resultados, discusión y límites; sin suplementos | Heterogeneidad I²=97,7 %; generalización a tupaqnin incierta |
| 8 | [Sharma et al. 2023, Sycophancy](https://arxiv.org/html/2310.13548v4) | Estudio empírico | §3, §4.1–4.3 | 5 modelos y 4 tareas (2023); no universal |
| 9 | [Turpin et al. 2023, CoT](https://arxiv.org/abs/2305.04388) | Estudio empírico | Solo abstract | Pendiente de lectura; cautela sin lectura completa |
| 10 | Carson et al. 2007 ([DOI](https://doi.org/10.5465/amj.2007.20159921)); Austin 2003 (10.1037/0021-9010.88.5.866); Anderson & Kilduff 2009 (10.1037/a0014201); [Wu & Cormican 2021](https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2020.569198/full) | Estudios de equipos humanos | Carson, Austin y Anderson & Kilduff: metadatos, abstracts o fragmentos; Wu & Cormican: abstract, definiciones, hipótesis y método (coeficientes de resultados sin leer) | Contenido primario débil; el constructo de Wu & Cormican es liderazgo en redes de influencia, no comprensión |
| 11 | Järvelä & Hadwin 2013 (10.1080/00461520.2012.748006) | Teoría (regulación del aprendizaje) | Fragmentos de abstract/índice | Pendiente; sin eficacia confirmada |
| 12 | [Abuhaimed & Sen 2022, HHAI](https://www.hhai-conference.org/wp-content/uploads/2022/06/hhai-2022_paper_53.pdf) | Estudio empírico (N=130) | Casi completa (explorador) + texto completo (contraste independiente) | Asignación de tareas, no liderazgo cognitivo ni comprensión; ventajas del agente sin verificación independiente |
| 13 | [MOSAAIC (Issak et al., ICCC 2025)](https://computationalcreativity.net/iccc25/wp-content/uploads/papers/iccc25-issak2025mosaaic.pdf) | Revisión conceptual | pp. 1–6 de 11 | Análisis de seis casos sin leer ni contrastar; vocabulario, no experimento |
| 14 | [Conmutación de roles (arXiv 2509.20666)](https://arxiv.org/pdf/2509.20666) | Estudio empírico (no admitido) | Lecturas con extracciones en conflicto | Conteos, efectos y costos causales excluidos hasta resolver versión/extracción |
| 15 | [Cleo 2026, v5](https://arxiv.org/html/2603.02050v5) | Estudio empírico | Lectura previa (abstract, introducción, estudio 1) ampliada en la cuarta ronda: explorador §1–5.1; seguimiento §2.1–2.3, §4.1.1–4.1.2, §4.3, §5.1 (DI1–DI2) y §5.5; v5 no comparada con la versión previa citada | Condiciones cualitativas, no predictores causales; §5.5 sin comparación de costo social humano-agente vs humano-humano; sin traspaso de conducción por comprensión |
| 16 | [Pan & Rickard 2018, transferencia de la práctica con prueba](https://pdf.retrievalpractice.org/transfer/Pan_Rickard_2018.pdf) | Metaanálisis (Psychological Bulletin) | Métodos, resultados y discusión (ambos exploradores); contraste independiente pp. 4–7, 19–23, 25–40; tabla 1 completa y forest plots sin inspeccionar | Frente a re-estudio, no frente a sin entrenamiento; sesgo de publicación al menos moderado; extracción fallida histórica resuelta en la tercera ronda |
| 17 | [Bastani et al., práctica escolar con GPT](https://atelierdesfuturs.org/wp-content/uploads/2025/07/ssrn-4895486.pdf) | Estudio empírico (preprint SSRN; PNAS 2025 no leído) | Explorador pp. 1–2, 6–10, A4; contraste independiente pp. 1–14 | Versión leída es preprint; matemática escolar, no documentos largos; horas y costo sin leer |
| 18 | [Tandon et al. 2022, Learning to Repair](https://aclanthology.org/2022.findings-naacl.26.pdf) | Estudio empírico (Findings NAACL) | Artículo principal completo; contraste independiente de arquitectura, datos, RQ1–RQ2 y §8.1–8.2 | Corrección humano→sistema, no recíproca; EM no mide comprensión; efectos abstract/tablas no reconciliados, se reporta cualitativamente |
| 19 | [CLAMBER](https://arxiv.org/pdf/2405.12063v2) | Estudio empírico (ambigüedad y aclaración) | Explorador: 21 pp completas; validador: pp. 1–12 con §1–7 y apéndices A–C (D–F sin leer) | Aclaración provocada por prompt, no iniciativa natural ni reparación en vivo; utilidad binaria de la pregunta, no malentendido resuelto ni uptake; fiabilidad del apéndice confirmada solo por el explorador |
| 20 | [ClarifyWhenNecessary](https://aclanthology.org/2025.findings-naacl.306.pdf) | Estudio empírico (Findings NAACL 2025) | §1–8 de métodos y resultados (contraste independiente), sin apéndices | Presupuesto como porcentaje de ejemplos con aclaración, no costo real de evaluar; sin criterios generales transferibles a casos nuevos |
| 21 | [Estudio de costo de interacción](https://arxiv.org/html/2609.21117v1) | Estudio empírico | Abstract (explorador) + fragmentos: §3.1 (definición de Q y C), §3.3, validación humana, apéndices C/F | Q y C son proxies (juez LLM; tokens/turnos), no tiempo humano de evaluar comprensión; resultado del abstract sin verificar a nivel de método y no citado |

Queda abierta toda la correspondencia entre estos aportes y las dimensiones: ninguno cierra una pregunta ni elige una alternativa.

## Formato para el próximo hallazgo de investigación

```text
Pregunta:    <la que se busca responder>
Fuente:      <ruta del proyecto §sección, o "externa" con su profundidad de lectura real (p. ej., abstract, secciones, lectura completa); su verificación se registra aparte>
Hallazgo:    <en una o dos frases>
Etapa:       propuesta documentada | observación | resultado de prueba
Evidencia:   <qué la sostiene: fuente, dominio, contexto y límites; verificar la fuente no equivale a eficacia probada>
Implicación: <qué cambia para D1–D4, o "ninguna por ahora">
```

La etapa dice en qué punto del trabajo está el hallazgo; la evidencia, qué tan fuerte es y de dónde viene. La verificación de la fuente (¿se abrió y se contrastó?) se declara aparte de la eficacia del hallazgo: una fuente verificada no convierte una analogía en resultado. Cuando algo no se comprobó, se escribe "no verificado" en Evidencia, sin más ceremonia.

## Límites y no metas

- Estas cuatro dimensiones **no son exhaustivas** ni congelan una arquitectura; las fases históricas son hallazgos de investigación, no ganchos ni marco obligatorio, y ninguna arquitectura previa se da por elegida.
- Los artefactos con prefijo `hm-` son propuestas históricas de las fases, no archivos implementados `tq-`; no se renombran aquí.
- Sin implementación, sin medidas validadas y sin requerimientos de producto adoptados; las recomendaciones son opcionales. Se incorporan cuatro rondas de consulta externa ya realizadas ([Aportes de la ronda externa](#aportes-de-la-ronda-externa)): sin nuevos experimentos propios, sin validación de producto y sin nuevas búsquedas en esta actualización documental; la segunda ronda amplió las consultas públicas respecto de la primera, la tercera profundizó la lectura real de una fuente antes pendiente y de dos estudios nuevos, y la cuarta añadió tres estudios de aclaración, costo e interacción con profundidad declarada por fuente y fragmento. Ninguna ausencia empírica declarada va más allá del material consultado, y una exploración que violó la exclusión de `05`/`06` quedó excluida con su contenido.
- Todo lo marcado **no verificado** en las fuentes conserva ese estado; la bibliografía académica citada en 01–04 y las fases no se revisó de nuevo en este documento. Las estimaciones numéricas citadas (auditoría-integración §2) son no medidas.
