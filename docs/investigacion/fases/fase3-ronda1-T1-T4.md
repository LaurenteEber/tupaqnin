# Ronda 1 · T1 (criterio vivo) y T4 (co-generación y exploración)

*7-oct-2026 · 15 consultas web (búsquedas + lecturas). "No verificado" = no abierto en esta ronda.*

**Principio rector:** cada idea debe *sustituir* trabajo humano (leer, redactar reglas, repetir correcciones), no añadir un control. Si no reduce carga neta, va a v2 o se descarta.

---

## T1 · Criterio vivo (H1, H3, H4)

Hoy en v1.1: `hamutay-explorar` destila CRI desde reacciones +/–/? con cita literal; el hook bloquea `estado: confirmado` sin cita en transcript; las secciones declaran `CRI-@v`. Falta: **cómo se confirma sin ceremonia, qué dispara una versión nueva y cómo se ve la deriva**.

### T1.1 · Destilación por gesto, no por redacción
- **Cómo:** el humano nunca escribe un CRI. Reacciona (+/–/? o reescribe un párrafo); `hamutay-explorar`/`construir` propone el CRI con `cita_humana` = la reacción literal, `ejemplo` = el fragmento aprobado y `contraejemplo` = el rechazado. Una reescritura del humano genera el par antes/después automáticamente.
- **Fundamento:** ConstitutionMaker convierte elogio, crítica y reescritura en principios; con N=14, los usuarios redactaron principios con menor demanda mental que escribiéndolos ellos ([arXiv 2310.15428](https://arxiv.org/abs/2310.15428)). PRELUDE/CIPHER infiere preferencias legibles y editables a partir de ediciones del usuario ([Microsoft Research, NeurIPS 2024](https://www.microsoft.com/en-us/research/publication/aligning-llm-agents-by-learning-latent-preference-from-user-edits/)).
- **Evidencia:** moderada (laboratorio, textos cortos; CIPHER con usuario simulado).
- **Costo humano:** 0 interacciones extra; la reacción ya ocurría.
- **Implementación:** baja (prompt en la skill + plantilla CRI). **MVP.**

### T1.2 · Confirmación por consentimiento tácito con plazo ("lazy consensus")
- **Cómo:** un CRI propuesto nace `provisional` y **se aplica de inmediato**. Pasa a `confirmado` si el humano no lo objeta en los siguientes N contactos (p. ej., dos lotes G2) **y** fue aplicado al menos una vez a una sección que el humano leyó en su presupuesto de lectura. Solo los CRI que cumplen la regla mecánica de "crítico" (contradicen otro CRI, tocan cifra/conclusión) exigen un "sí" explícito. El lote G2 deja de ser "confirma estos 5" y pasa a ser "objeta si algo no es así".
- **Fundamento:** Apache usa *lazy consensus*: se anuncia la intención, se espera ~72 h y el silencio se toma como consentimiento; para asuntos que requieren discusión se abre un hilo aparte ([Apache Community](https://community.apache.org/committers/lazyConsensus.html)).
- **Evidencia:** moderada como práctica de gobernanza; especulativa aplicada a una sola persona (el silencio puede ser falta de atención, no acuerdo).
- **Salvaguarda:** el hook ya existente solo permite `confirmado` con cita **o** con el contador de "aplicado y leído sin objeción" en `sesiones/`; nunca por decisión libre del agente.
- **Costo humano:** baja de ≤5 confirmaciones por lote a ≤1–2 (solo críticos). **Implementación:** baja. **MVP.**

### T1.3 · Tarjeta de criterio como "regla + ejemplos + preguntas" (Example Mapping)
- **Cómo:** el CRI se expresa como una regla de una línea, 1–2 ejemplos verdes (fragmentos reales que cumplen), 1 contraejemplo y, si existe, una **pregunta roja** abierta ("¿aplica también a anexos?"). Las preguntas rojas van a la cola de desborde, no interrumpen.
- **Fundamento:** Example Mapping (Cucumber/BDD) organiza historias con tarjetas de reglas, ejemplos concretos y preguntas no resueltas para no detener la conversación ([cucumber.io](https://www.cucumber.io/docs/bdd/example-mapping)). Los ejemplos concretos vuelven la regla verificable.
- **Evidencia:** moderada (práctica difundida en ingeniería de requisitos; sin estudios controlados).
- **Costo humano:** ninguno extra; el humano juzga ejemplos, no abstracciones (más rápido).
- **Implementación:** baja (renombrar campos existentes: ya hay `ejemplo`/`contraejemplo`; añadir `preguntas`). **MVP.**

### T1.4 · Versionado por precedente: aplicar, distinguir, revocar
- **Cómo:** cuando el humano corrige algo que contradice un CRI vigente, el agente pregunta **una sola cosa**: "¿esto **distingue** (excepción con condición nueva → `CRI-012@v2` añade `no_aplica_si`) o **revoca** (→ `CRI-012` pasa a `reemplazado_por: CRI-019`)?". Versión menor = distinción; mayor = revocación. `hm impacto CRI-012` lista secciones atadas a la versión vieja.
- **Fundamento:** en derecho común, un precedente se sigue, se **distingue** por hechos materialmente distintos o se **revoca** ([referencia general sobre precedente y ratio decidendi](https://ipsaloquitur.com/2020/01/12/ratio-decidendi-obiter-dicta/)); los ADR usan estados como `superseded by` en vez de borrar ([ejemplo de índice ADR](https://paxman.readthedocs.io/en/latest/adr/)).
- **Evidencia:** fuerte como práctica institucional; especulativa como diseño de software para criterios personales.
- **Costo humano:** 1 pregunta binaria solo cuando hay contradicción (es la alerta en caliente ya presupuestada).
- **Implementación:** baja–media (dos campos y una regla en `hm check`). **MVP.**

### T1.5 · Deriva como contradicción observada, no como métrica
- **Cómo:** `hm check` cuenta, por CRI, las **objeciones o reescrituras humanas sobre fragmentos que lo citan**. Dos o más en una ventana → el CRI aparece marcado "en deriva" en la reentrada, con los fragmentos lado a lado. No hay puntaje de calidad ni juez.
- **Fundamento:** EvalGen definió la *deriva de criterios*: se necesitan criterios para calificar, pero calificar ayuda a definirlos; algunos criterios dependían de los outputs ya vistos ([Shankar et al., UIST 2024](https://arxiv.org/abs/2404.12272)). La deriva es esperable, no un error; hay que hacerla visible.
- **Evidencia:** moderada (estudio cualitativo de laboratorio).
- **Costo humano:** 0 si no hay deriva; 1 decisión T1.4 si la hay.
- **Implementación:** baja (contadores en el frontmatter). **MVP.**

### T1.6 · "Guiones ancla" para detectar deriva del agente (moderación de exámenes)
- **Cómo:** cada CRI confirmado congela 1–2 fragmentos como **anclas** (el ejemplo y el contraejemplo). En v2, `hm-lector` o un juez reclasifica periódicamente las anclas con el CRI vigente; si su veredicto cambia, el criterio o su aplicación se movió sin que el humano decidiera.
- **Fundamento:** en evaluación a gran escala, los examinadores se estandarizan con guiones ejemplares calificados por el examinador principal y se mide la exactitud como distancia a esas notas ([Cambridge Assessment, fiabilidad en GCSE](https://www.cambridgeassessment.org.uk/Images/109701-tools-for-the-trade-what-makes-gcse-marking-reliable-.pdf)). El uso operativo de "seed items" para vigilar deriva: **no verificado** en la fuente consultada.
- **Evidencia:** moderada (práctica de evaluación educativa).
- **Costo humano:** 0 salvo cuando hay discrepancia. **Implementación:** media. **v2.**

### T1.7 · Retroalimentar el encargo desde los CRI (H4)
- **Cómo:** al confirmar un CRI con `aplica_a: encargo`, se agrega como línea `S#` derivada en `encargo.md` con referencia `← CRI-###@v`. El encargo deja de congelarse al inicio.
- **Fundamento:** H4 del acervo; Fowler critica que en SDD la spec se fija al principio ([martinfowler.com](https://www.martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html), no reabierto en esta ronda).
- **Evidencia:** especulativa. **Costo humano:** 0. **Implementación:** baja. **MVP** (es solo una línea en `hamutay-construir`).

### Analogías fuera de IA (T1)
| Dominio | Práctica | Qué aporta |
|---|---|---|
| Derecho | Precedente: seguir, distinguir, revocar | Versionado semántico de criterios (T1.4) |
| Gobernanza OSS | Lazy consensus | Confirmar sin pedir "sí" explícito (T1.2) |
| Ingeniería de requisitos | Example Mapping | Regla anclada en ejemplos + preguntas en cola (T1.3) |
| Evaluación educativa | Estandarización con guiones ejemplares | Deriva medida contra anclas fijas (T1.6) |
| Investigación cualitativa | Libro de códigos iterativo con memos | Cada código tiene definición, inclusión/exclusión y ejemplo; se revisa por iteración ([Lumivero](https://lumivero.com/resources/blog/codebook-in-qualitative-research/), consultado solo en resultados de búsqueda: **no verificado** en detalle) |

### Top 3 T1 para el MVP
1. **T1.2 Lazy consensus con plazo.** Es la única idea que *quita* contactos: reduce G2 de confirmar 5 a objetar si hace falta, y conserva la regla de que el agente no decide (el hook exige cita o contador).
2. **T1.4 Distinguir vs. revocar.** Resuelve el versionado con una pregunta binaria y da sentido a `@v` en `depende_de`; sin esto, `hm impacto` no sabe qué es un cambio mayor.
3. **T1.5 Deriva como contradicciones contadas.** Cero costo cuando todo va bien, sin juez circular; usa señales que el humano ya emitió. (T1.1 y T1.3 se dan por incluidos: son plantillas, no mecanismos nuevos.)

---

## T4 · Co-generación y exploración (C1, M12)

Hoy en v1.1: G1 pide encuadre e ideas a `ideas/*-humano.md` (inmutable); `hm-divergente` genera tarjetas en `opciones.yaml` sin ver la recomendación. Falta: **cómo pedir ideas sin que pese, cómo divergir sin homogeneizar y cómo comparar sin leer todas las tarjetas**.

### T4.1 · Ideación humana primero, en formato mínimo y cronometrado
- **Cómo:** G1 pide **viñetas, no prosa**: 3–7 ideas en ≤10 minutos, incluidas las malas, en `ideas/*-humano.md`. El agente no muestra nada hasta que el archivo existe (hook `PreToolUse` que bloquea escribir `opciones.yaml` si falta el archivo humano del trabajo).
- **Fundamento:** con N=60, usar el LLM al inicio redujo ideas originales, autoeficacia creativa y autoría percibida respecto de usarlo tras idear solo ([Timing Matters, CHI 2025](https://arxiv.org/abs/2502.06197)). En brainstorming, los grupos nominales (cada uno genera sin interactuar) superan a los grupos interactivos por *production blocking* ([resumen](https://en.wikipedia.org/wiki/Production_blocking)).
- **Evidencia:** fuerte en conjunto (laboratorio + décadas de literatura de grupos nominales).
- **Costo humano:** 5–10 min una vez por trabajo (ya presupuestado en G1).
- **Implementación:** baja. **MVP.**

### T4.2 · Expansión "desde tu idea", no "en lugar de"
- **Cómo:** `hm-divergente` produce primero, por cada idea humana, una variante que la **lleva al extremo** y otra que la **combina** con otra idea humana; recién después agrega opciones propias, marcadas `origen: agente`. Las tarjetas indican linaje (`deriva_de: H3`).
- **Fundamento:** Pennanen et al. (2026, n=273) reportan que editar y hacer prompts iterativos recuperan parte de la autoría (citado en el acervo; **no reabierto** en esta ronda). Timing Matters sugiere preservar autonomía y autoría.
- **Evidencia:** moderada–débil. **Costo humano:** 0. **Implementación:** baja. **MVP.**

### T4.3 · Divergencia forzada contra la homogeneización
- **Cómo:** `hm-divergente` recibe un **eje de contraste obligatorio por tarjeta** (p. ej., actor que decide, escala temporal, instrumento, supuesto que se invierte) y la instrucción de que dos tarjetas no compartan valor en el eje. Opcional: invocarlo 2–3 veces con "personas" distintas o con otro proveedor. El orquestador descarta tarjetas casi duplicadas antes de mostrarlas.
- **Fundamento:** ideas generadas con ChatGPT fueron menos diversas a nivel de grupo que con el mazo Oblique Strategies, sin diferencia a nivel individual (N=33); el efecto homogeneizador aparece en modelos con *instruction tuning* ([Anderson et al., arXiv 2402.01536](https://arxiv.org/html/2402.01536v2)). Hay trabajos que proponen personas de IA diversas para mitigarlo ([Open University of Israel](https://cris.openu.ac.il/en/publications/diverse-ai-personas-can-mitigate-the-homogenization-effect-in-hum/), solo título: **no verificado**).
- **Evidencia:** moderada sobre el problema; débil sobre el remedio.
- **Costo humano:** 0. **Implementación:** baja (prompt + filtro). **MVP.**

### T4.4 · Comparar por diferencias, no por tarjetas (vista de contraste)
- **Cómo:** el humano no lee las N tarjetas. Recibe **una tabla**: filas = opciones, columnas = ejes de contraste y CRI vigentes, celdas de ≤8 palabras; lo que comparten todas se colapsa en una línea "común a todas". Solo se expande una tarjeta si el humano la toca.
- **Fundamento:** alinear y contrastar muchas respuestas hizo viables tareas de sensemaking que los participantes consideraban demasiado difíciles (N=24, [Gero et al., CHI 2024](https://arxiv.org/abs/2401.13726)). Luminate estructura primero dimensiones y luego variantes (14 escritores, [CHI 2024](https://arxiv.org/abs/2310.12953)).
- **Evidencia:** moderada. **Costo humano:** reduce lectura de N tarjetas a una tabla. **Implementación:** baja. **MVP.**

### T4.5 · Las dimensiones las nombra el humano primero
- **Cómo:** antes de que el agente proponga ejes de contraste (T4.3/T4.4), G1 incluye una pregunta: "¿en qué se diferenciaría una buena opción de una mala?". Las respuestas son los primeros ejes y, a la vez, **CRI semilla** provisionales (puente con T1.2). El agente solo añade ejes faltantes, marcados.
- **Fundamento:** el acervo advierte que las dimensiones propuestas por el modelo anclan el pensamiento (límite de M10/M11) y que la única mitigación con evidencia es idear primero (M12). Kepner-Tregoe separa criterios obligatorios y deseables antes de evaluar alternativas ([MindTools](https://www.mindtools.com/?p=60097), solo resultado de búsqueda: **no verificado**).
- **Evidencia:** moderada (por extensión de M12). **Costo humano:** 1–2 min dentro de G1. **Implementación:** baja. **MVP.**

### T4.6 · Pre-mortem de la opción elegida
- **Cómo:** al elegir, el agente no pide confirmación; pide **una línea**: "Si dentro de seis meses esta opción fracasó, ¿por qué fue?". La respuesta se guarda como `supuestos` de la DEC y su Contrapeso. El agente agrega 2 causas de fracaso que el humano no mencionó.
- **Fundamento:** el pre-mortem de Klein (HBR, 2007) supone que el proyecto ya fracasó y trabaja hacia atrás; formaliza la "retrospección prospectiva" (Mitchell, Russo y Pennington, 1989) ([resumen](https://en.wikipedia.org/wiki/Pre-mortem)). La cifra de +30 % atribuida a ese estudio: **no verificada**.
- **Evidencia:** moderada. **Costo humano:** 1 interacción breve por DEC crítica (sustituye "¿confirmas?"). **Implementación:** baja. **MVP o v2** según calibración.

### T4.7 · Mantener el conjunto vivo y converger por intersección
- **Cómo:** en vez de elegir una opción al final de la exploración, `opciones.yaml` mantiene 2–3 vigentes mientras se redactan las secciones que no dependen de la elección; cada CRI nuevo **descarta** opciones (con razón) hasta que queda una. El descarte por criterio ya existe; lo nuevo es no forzar la elección temprana.
- **Fundamento:** la ingeniería concurrente basada en conjuntos de Toyota explora alternativas, busca la intersección de opciones viables y estrecha el conjunto a medida que crece el conocimiento ([Sobek, Ward y Liker, MIT SMR](https://sloanreview.mit.edu/article/toyotas-principles-of-setbased-concurrent-engineering)).
- **Evidencia:** moderada (casos industriales). **Costo humano:** baja (decisiones más informadas, menos retrabajo). **Implementación:** media (dependencias condicionales). **v2.**

### T4.8 · Comparación por pares cuando la tabla no basta
- **Cómo:** si hay >5 opciones vigentes, se muestran pares ("¿A o B, y por qué en una frase?"); el "por qué" alimenta T1.1. Con 4–6 juicios se obtiene un orden.
- **Fundamento:** el juicio comparativo se usa en evaluación educativa como alternativa a la calificación absoluta, con literatura sobre su fiabilidad y eficiencia ([Frontiers in Education, 2021](https://www.frontiersin.org/journals/education/articles/10.3389/feduc.2021.785919/pdf), solo resultado de búsqueda: **no verificado** en detalle).
- **Evidencia:** moderada en educación; especulativa aquí. **Costo humano:** 4–6 clics. **Implementación:** baja. **v2.**

### Analogías fuera de IA (T4)
| Dominio | Práctica | Qué aporta |
|---|---|---|
| Psicología de grupos | Grupos nominales / brainwriting | Generar solo antes de exponerse a otros (T4.1) |
| Prospectiva | Delphi: estimaciones independientes antes de ver las de otros ([resumen](https://en.wikipedia.org/wiki/Delphi_method), no leído) | Mismo principio antianclaje |
| Ingeniería | Set-based design (Toyota) | Postergar la elección y converger por criterios (T4.7) |
| Gestión de riesgos | Pre-mortem | Supuestos explícitos con una sola pregunta (T4.6) |
| Evaluación educativa | Juicio comparativo | Ordenar sin rúbrica previa (T4.8) |
| Decisión gerencial | Kepner-Tregoe (obligatorios/deseables) | Criterios antes que opciones (T4.5) |

### Top 3 T4 para el MVP
1. **T4.1 Ideación humana primero, mínima y bloqueada por hook.** Mejor evidencia de todo el tema; cuesta 10 minutos una vez y protege autoría y originalidad.
2. **T4.4 Vista de contraste.** Es lo que más lectura ahorra: sustituye leer N tarjetas por una tabla de diferencias.
3. **T4.5 Dimensiones nombradas por el humano.** Ataca el anclaje que el acervo marca como límite principal de M10/M11 y siembra los CRI provisionales, conectando T4 con T1 sin contacto extra. (T4.3 se incluye como instrucción de `hm-divergente`; no requiere decisión.)

---

## Cruce T1 ↔ T4
El flujo mínimo queda: **G1** (ideas + "qué distingue lo bueno") → ejes y CRI semilla `provisional` → tarjetas divergentes → **tabla de contraste** → reacciones +/–/? → CRI destilados → confirmación tácita salvo críticos → versionado por distinguir/revocar → deriva contada. Contactos netos: G2 baja de ~5 confirmaciones a ~1–2 por lote.

## Riesgos y por probar
- **Silencio ≠ acuerdo** (T1.2): calibrar N en el piloto; si el humano no lee las secciones, el contador no avanza.
- La evidencia de T4 es de tareas creativas cortas; en documentos de política pública (GEE v6→v7) no está probada.
- Medir en el piloto: número de CRI revocados vs. distinguidos y diversidad de las tarjetas (similitud de embeddings, como en arXiv 2402.01536).
