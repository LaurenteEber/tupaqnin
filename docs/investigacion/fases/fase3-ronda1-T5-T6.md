# Ronda 1: T5 (verificación sin tests) y T6 (cambios en documentos interdependientes)

*7 oct 2026 · Lluvia de ideas fundamentada para hamutay. Evidencia: **F** fuerte · **M** moderada · **D** débil · **E** especulativa. "No verificado" = no lo abrí en esta ronda.*

---

## T5. Verificación de documentos sin tests

### Diagnóstico en 4 hechos

1. **Más jueces no es más independencia.** Un panel de 9 LLM de 7 familias equivale a unos 2 votos independientes; agregar jueces o mejores agregadores no cierra la brecha ([Apple ML Research, *Nine Judges, Two Effective Votes*](https://machinelearning.apple.com/research/correlated-llm-evaluation-panels)). Los errores se correlacionan incluso entre proveedores y arquitecturas distintas, más en los modelos grandes ([Kim, Garg et al. 2025](https://arxiv.org/abs/2506.07962); [Goel et al., ICML 2025](https://arxiv.org/abs/2502.04313)).
2. **El juez prefiere su propio texto.** La autopreferencia crece con la capacidad de reconocer lo propio ([Panickssery et al., NeurIPS 2024](https://papers.nips.cc/paper_files/paper/2024/hash/7f1f0218e45f5414c79c0679633e47bc-Abstract-Conference.html)). El redactor no debe juzgarse.
3. **Los humanos tampoco detectan mucho.** En un ensayo con 607 revisores del BMJ, con 9 errores mayores sembrados por artículo, se encontraron en promedio entre 2,6 y 3; la capacitación apenas ayudó ([Schroter et al., LSHTM](https://researchonline.lshtm.ac.uk/id/eprint/6900)). "Sin hallazgos" no equivale a "verificado", ni en humanos ni en jueces.
4. **Qué señales contrarias funcionan.** Las explicaciones suben la confianza tanto en lo correcto como en lo incorrecto; las fuentes y las inconsistencias visibles sí la calibran ([Kim et al., CHI 2025](https://arxiv.org/abs/2502.08554)). Fuera de la IA, el disenso auténtico estimula más que el abogado del diablo, porque este solo actúa un papel (Nemeth, vía [Behavioral Scientist](https://behavioralscientist.org/stop-playing-devils-advocate-and-other-advice-for-better-decision-making/); experimentos originales no verificados).

**Consecuencia de diseño:** hay que diversificar **la información** que recibe cada lente, no el número de modelos. Además, las señales contrarias deben mostrarse como **evidencia cruda** (desacuerdo, fuente, hueco), no como prosa persuasiva.

### Ideas

**T5.1 Hoja de afirmaciones con marcas (tickmarks)**
- **Cómo:** `hamutay-construir` genera `verif/<seccion>.yaml`. Contiene una fila por afirmación que importa (cifra, cita, conclusión o recomendación) con:
  - **marca:** `✓fuente`, `≈inferencia`, `?sin verificar` o `✗contradicha`;
  - **quién** verificó (agente, lector o humano) y la **fecha**;
  - el **ítem revisado**: la página o celda de la fuente.

  El presupuesto de lectura sale de esta hoja: el humano solo lee `?`, `✗` y los `≈` críticos. `hm check` impide cerrar una sección que tenga `?` en una afirmación crítica.
- **Fundamento:**
  - AS 1215 de la PCAOB: la documentación debe permitir que un auditor experimentado sin relación con el encargo entienda qué se hizo; también debe registrar quién lo hizo y quién lo revisó, con fechas, y qué ítems se inspeccionaron ([PCAOB AS 1215](https://pcaobus.org/oversight/standards/auditing-standards/details/AS1215)).
  - Fact-checking periodístico: se marca cada hecho, se enlaza a su respaldo, se quita la marca al confirmarlo y se prefieren fuentes primarias ([AHCJ](https://healthjournalism.org/?p=13200)).
- **Evidencia:** F como práctica profesional; M como transferencia a documentos con IA.
- **Costo humano:** negativo, porque reemplaza la lectura completa. **Implementación:** baja a media. **Encaje:** **MVP**.
- **Bonus C6/C12:** la hoja es el expediente de qué se verificó, cómo y por quién, que el acervo puntuó con 0,25.

**T5.2 Lentes con información distinta, no jueces repetidos**
- **Cómo:** se reemplaza el "jurado" por 2 o 3 lectores, cada uno con un **insumo distinto**:
  - `hm-lector` **ciego**: solo ve la sección y detecta lo incomprensible o lo no sustentado para un externo. Ya existe.
  - **lector de fuentes**: ve las afirmaciones y las fuentes, pero no la prosa argumentativa. Llena las marcas de T5.1.
  - **lector de premisas**: ve los CRI y DEC vigentes y la sección, y detecta dónde el texto contradice o ignora una decisión.

  Ninguno ve la conversación. Lo que no comparten no se descarta: va a la cola, que es lo que ya fija la v1.1.
- **Fundamento:**
  - El debate solo gana con asimetría de información (Khan et al., citado en el informe).
  - La correlación de errores hace que los votos redundantes valgan poco (Apple; Kim et al.).
- **Evidencia:** M. **Costo humano:** 0 adicional. **Implementación:** media, porque son prompts y particiones de contexto. **Encaje:** **MVP**: dos lectores (ciego y premisas); el de fuentes, cuando haya fuentes.

**T5.3 Separación redactor–juez y diversidad de proveedor como bonus**
- **Cómo:**
  - Regla dura en un hook: el modelo y la sesión que redactan no emiten el veredicto final de su propia sección.
  - Si OpenCode u otro proveedor está disponible, `hm-lector` usa otra familia de modelos, pero el sistema **no lo presenta como independencia**. El reporte dice "lector: modelo X, mismo proveedor que el redactor: sí o no".
- **Fundamento:** Panickssery 2024 (autopreferencia); Kim et al. 2025 (la diversidad de proveedor ayuda poco en modelos grandes).
- **Evidencia:** M. **Costo humano:** 0. **Implementación:** baja. **Encaje:** **MVP** (resuelve el abierto "diversidad del lector": sí, pero sin sobrevenderla).

**T5.4 Errores sembrados para calibrar a los lectores**
- **Cómo:** en el piloto, `hm` siembra 2 o 3 errores conocidos en una **copia** de la sección que solo ven los lectores: una cifra alterada, una conclusión que contradice una DEC y una cita inexistente. La tasa de recuperación se guarda en `calibracion.yaml`. Si un lector recupera menos de la mitad, su "sin hallazgos" se muestra como "**cobertura baja**" y no como "OK". Al humano nunca se le siembran errores: sería un examen.
- **Fundamento:**
  - Los ensayos de errores sembrados en revisión por pares (BMJ) miden el recall real del revisor.
  - En auditoría se aplica la misma lógica de probar el control, no confiar en él.
- **Evidencia:** M (método); E (umbral). **Costo humano:** 0. **Implementación:** media. **Encaje:** MVP-ligero en el piloto (una corrida) y **v2** como rutina.

**T5.5 Disenso anclado en los descartes del propio humano**
- **Cómo:** para cada DEC crítica, el contrapeso no es un abogado del diablo genérico. Es **el mejor caso de la alternativa que el humano mismo descartó**, tomado de `descartes` y de la razón que dio, más la evidencia nueva que la favorezca desde entonces. Si no hay evidencia nueva, se muestra una línea: "nada nuevo a favor de B". Cuando dos lectores discrepan, se muestran **las dos lecturas lado a lado**, sin síntesis.
- **Fundamento:**
  - Disenso auténtico frente a disenso actuado (Nemeth).
  - Las inconsistencias calibran y las explicaciones no (Kim, CHI 2025).
  - El razonamiento propio sirve de ancla contra la complacencia (M12, en el acervo).
- **Evidencia:** M (Kim); D (transferencia de Nemeth). **Costo humano:** unos 30 a 60 s por DEC crítica, dentro del tope G3. **Implementación:** baja. **Encaje:** **MVP**.

**T5.6 Ficha de certeza por conclusión (GRADE-lite)**
- **Cómo:** cada conclusión o recomendación lleva `certeza: alta|moderada|baja|muy_baja` y, si baja, **en qué dominio**:
  - sesgo de la fuente;
  - inconsistencia entre fuentes;
  - indirectez: la evidencia es de otro país o nivel;
  - imprecisión.

  Una regla mecánica la calcula a partir de T5.1, por ejemplo: "solo `≈` → máximo moderada". No hay prosa.
- **Fundamento:** GRADE califica la certeza en 4 niveles y la baja por riesgo de sesgo, inconsistencia, imprecisión e indirectez ([SickKids/PRISMA ítem 20](https://lab.research.sickkids.ca/enrich/wp-content/uploads/sites/76/2024/07/Item-20_Certainty-assessment.pdf)).
- **Evidencia:** F como práctica en medicina; M como transferencia. **Costo humano:** una línea por conclusión. **Implementación:** baja. **Encaje:** v2, o MVP solo para recomendaciones.

**T5.7 Pre-mortem del lector**
- **Cómo:** se agrega al prompt de `hm-lector`: "Han pasado 6 meses y un revisor externo refutó esta sección. ¿Por qué?". La respuesta produce entradas para la cola, no prosa para el humano.
- **Fundamento:** la retrospección prospectiva mejora en torno a un 30 % la identificación de razones (Mitchell, Russo y Pennington 1989, citado por Klein; **el original no está verificado**, se conoce [vía fuente secundaria](https://www.processexcellencenetwork.com/business-process-management-bpm/articles/pre-mortem-analysis-anticipating-pitfalls-to-incre)).
- **Evidencia:** D en humanos; E en LLM. **Costo humano:** 0. **Implementación:** muy baja. **Encaje:** **MVP**.

**T5.8 Matriz de evidencia diagnóstica (ACH-lite)**
- **Cómo:** para una DEC crítica con alternativas, se arma una matriz de evidencia por opción (elegida y descartadas). El humano solo ve la evidencia **diagnóstica**, la que discrimina entre opciones; la que es compatible con todas se oculta.
- **Fundamento:** el *Analysis of Competing Hypotheses* de Heuer trabaja "a lo ancho" de la matriz y busca refutar ([Wikipedia](https://en.wikipedia.org/wiki/Analysis_of_competing_hypotheses)). No tiene evidencia empírica fuerte de eficacia.
- **Evidencia:** D. **Costo humano:** reduce la lectura de evidencia. **Implementación:** media. **Encaje:** **v2**.

### Regla transversal contra la complacencia

- Ningún lector puede devolver "OK" sin **cobertura**: qué afirmaciones miró y cuáles no.
- La interfaz nunca muestra un indicador verde agregado. Muestra conteos: "14 ✓, 3 ≈, 1 ?".

### Top 3 MVP (T5)

1. **T5.1 Hoja de afirmaciones.** Convierte la verificación en el insumo del presupuesto de lectura: baja la carga en vez de sumarla, y además cubre C6 y C12.
2. **T5.2 Lentes con información distinta.** Es la única respuesta con fundamento a los errores correlacionados: diversifica la información, no los votos.
3. **T5.5 Disenso anclado en los descartes.** Produce señales contrarias del tipo que sí calibra (inconsistencias y alternativas reales) a un costo acotado por G3.

---

## T6. Cambios en documentos interdependientes

### Diagnóstico

`hm impacto` sigue las dependencias **declaradas** a nivel de **sección**. Le faltan tres cosas:
1. **dependencias no declaradas**: un párrafo usa una premisa sin citarla;
2. **granularidad**: un cambio de DEC marca secciones enteras, lo que genera falsos positivos y fatiga;
3. **diff legible**: el diff textual no dice qué cambió en el argumento.

Las herramientas de comparación por carácter no detectan paráfrasis ni cambios de sentido. Ya hay benchmarks de comparación semántica de documentos legales con LLM ([LegDiff, ACL SRW 2026](https://aclanthology.org/2026.acl-srw.86/); no vi sus resultados).

### Ideas

**T6.1 Enlaces sospechosos (*suspect links*)**
- **Cómo:** cuando un CRI o DEC sube de versión, cada sección con `depende_de` sobre él pasa a `estado: sospechosa@DEC-012v3` en su frontmatter. El hook `Stop` (`hm check`) no deja cerrar mientras haya sospechosas. Una sospecha se limpia de una de dos formas:
  - el agente reescribe la sección;
  - el agente declara "sigue válida" con una razón de una línea.

  El humano solo interviene si la DEC es crítica (regla mecánica ya existente).
- **Fundamento:** en ingeniería de requisitos (DOORS, PTC Modeler vía OSLC), el enlace se marca sospechoso de forma automática cuando cambia el elemento enlazado; se revisa y se limpia o se borra el enlace ([PTC](https://support.ptc.com/help/modeler/r10.1/en/Modeler/rtsme/managing_changes_to_remote_oslc_data.html)).
- **Evidencia:** F como práctica industrial (sistemas críticos). **Costo humano:** proporcional solo a lo crítico. **Implementación:** baja, porque extiende `hm check`. **Encaje:** **MVP**.

**T6.2 Banderas de tratamiento al estilo citator (Shepard's/KeyCite)**
- **Cómo:** se agrega la vista inversa a `hm impacto <DEC>`. Para cada sección que la usa, el agente etiqueta el **tratamiento**: `aplica`, `distingue` (la usa con salvedad) o `contradice`. `contradice` es bandera roja: entra a G3 o a la alerta en caliente.
- **Fundamento:** los citadores legales marcan con banderas el tratamiento negativo posterior para confirmar que una autoridad "sigue siendo buena ley" ([UF Law](https://guides.law.ufl.edu/floridacaselaw/validating)).
- **Evidencia:** F como práctica legal; M como transferencia. **Costo humano:** escanear una lista corta. **Implementación:** baja a media. **Encaje:** **MVP**, como formato de salida de `hm impacto`.

**T6.3 Diff de esqueleto argumental (Toulmin-lite)**
- **Cómo:** en cada versión, el agente extrae a `esqueleto/<seccion>.yaml`:
  - tesis;
  - razones;
  - supuesto o garantía;
  - refutaciones reconocidas;
  - cifras clave.

  `hm diff <seccion> vA vB` compara esqueletos, no texto. Un ejemplo de salida: "tesis igual · razón 2 sustituida · supuesto nuevo S-4 · cifra brecha 12 %→15 %". Sustituye al diff textual en el presupuesto de lectura.
- **Fundamento:**
  - El modelo de Toulmin (afirmación, datos, garantía, respaldo, refutación) es un estándar de enseñanza de la argumentación ([UTSA](https://www.utsa.edu/twc/documents/Toulmin_Model_of_Argumentation.pdf)).
  - El mapeo de argumentos mejoraría el pensamiento crítico más del triple que otros métodos ([van Gelder](https://web-archive.southampton.ac.uk/cogprints.org/3008/); solo leí el resumen, el diseño no es controlado).
  - LegDiff (arriba) muestra que el problema se estudia.
- **Evidencia:** D a M. **Costo humano:** 5 a 10 líneas en vez de un diff completo. **Implementación:** media. **Encaje:** **MVP**: es el núcleo del "diff semántico" de C2.
- **Riesgo:** el esqueleto lo extrae un LLM y puede omitir cambios. Mitigación: el lector de premisas (T5.2) compara el esqueleto con el texto.

**T6.4 Triage de propagación en tres carriles**
- **Cómo:** después de `hm impacto`, cada sección afectada cae en uno de tres carriles:
  1. **El esqueleto no cambia:** se limpia sola, y queda registro.
  2. **El esqueleto cambia en una sección no crítica:** el agente reescribe y actualiza la hoja de T5.1.
  3. **Toca una cifra, una conclusión o un CRI:** pasa a G3.

  El desborde va a la cola.
- **Fundamento:** triage por riesgo (M33 en el acervo: reduce la fatiga a costa de algunos falsos negativos).
- **Evidencia:** M. **Costo humano:** acotado por los topes actuales. **Implementación:** baja. **Encaje:** **MVP**.

**T6.5 Detector de dependencias no declaradas**
- **Cómo:** `hm check` busca en cada sección los términos, cifras y alias de cada DEC y CRI vigentes que no estén en su `depende_de`. El agente propone añadirlos y, para evitar ceremonia, se aceptan solos salvo que la DEC sea crítica.
- **Fundamento:** la trazabilidad completa es requisito en ingeniería de requisitos; recuperar enlaces de trazabilidad con LLM o grafos es emergente (M26, en el acervo).
- **Evidencia:** D. **Costo humano:** casi 0. **Implementación:** baja para la versión léxica y media para la semántica. **Encaje:** MVP (léxico); v2 (semántico).

**T6.6 Mantenimiento de razones (TMS)**
- **Cómo:** distinguir "depende de" y "**solo** se sostiene en". Si un supuesto se retracta, cae (OUT) lo que no tiene otra justificación; lo que tiene un respaldo alternativo queda en pie, pero se marca "respaldo reducido".
- **Fundamento:** el sistema de mantenimiento de la verdad de Doyle (*Artificial Intelligence*, 1979) y la revisión de creencias ([Wikipedia, *Reason maintenance*](https://en.wikipedia.org/wiki/Reason_maintenance)).
- **Evidencia:** F en teoría; D en documentos. **Costo humano:** 0. **Implementación:** media a alta. **Encaje:** **v2**, porque exige registrar `SUP` por separado, y eso ya está en v2.

**T6.7 Cruce de consistencia entre pares de secciones**
- **Cómo:** después de propagar, el lector de premisas recibe **pares** de secciones que comparten una DEC y responde: ¿afirman lo mismo? Así ataca H6, las contradicciones de fondo entre piezas.
- **Fundamento:** el fact-checking manda revisar todas las versiones para confirmar que las correcciones se trasladaron (AHCJ).
- **Evidencia:** M (práctica). **Costo humano:** solo ve las contradicciones. **Implementación:** media; el costo de tokens es cuadrático, así que se limita a los pares que comparten una DEC crítica. **Encaje:** **v2** (MVP si el piloto tiene más de 5 secciones).

### Top 3 MVP (T6)

1. **T6.1 Enlaces sospechosos.** Es una práctica madura, barata y mecánica, que convierte "propagación" en un estado verificable por un hook y no deja el cambio de premisa a la memoria del agente.
2. **T6.3 Diff de esqueleto.** Es lo único que responde a "¿qué cambió en el argumento?", reemplaza la lectura del diff y alimenta la reentrada (T3).
3. **T6.4 Triage en tres carriles.** Evita que T6.1 genere fatiga: solo llega al humano lo que cambia el argumento en algo crítico.

---

## Analogías fuera de la IA (resumen)

| Dominio | Práctica | Uso en hamutay |
|---|---|---|
| Auditoría | Papeles de trabajo, *tickmarks*, quién y cuándo, ítems inspeccionados (AS 1215) | T5.1 |
| Periodismo | Anotar cada hecho con su respaldo; revisar que las correcciones se trasladen | T5.1, T6.7 |
| Revisión por pares | Errores sembrados para medir el recall | T5.4 |
| Psicología de grupos | Disenso auténtico frente a abogado del diablo | T5.5 |
| Medicina basada en evidencia | GRADE: certeza por dominios | T5.6 |
| Inteligencia | ACH: evidencia diagnóstica, refutar | T5.8 |
| Gestión de proyectos | Pre-mortem | T5.7 |
| Ingeniería de requisitos | *Suspect links* | T6.1 |
| Derecho | Citadores: tratamiento y banderas | T6.2 |
| Retórica | Toulmin, mapas de argumentos | T6.3 |
| IA clásica | Mantenimiento de la verdad (Doyle) | T6.6 |

## Sinergia clave

T5.1 (hoja de afirmaciones) y T6.3 (esqueleto) son **el mismo objeto visto dos veces**: unidades de argumento con estado. Si se construyen juntos, sirven a la vez para el presupuesto de lectura, la propagación (dependencias a nivel de afirmación, no de sección), el expediente para quien firma (C6) y la reentrada (T3).

## No verificado en esta ronda

- Experimentos originales de Nemeth.
- Mitchell et al. 1989 (cifra del 30 %).
- Resultados numéricos de LegDiff.
- Diseño del estudio de van Gelder.
- Khan et al. (tomado del informe previo).
