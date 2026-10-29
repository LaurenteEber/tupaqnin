# Ronda 1 · T2 (comprensión, defensa y presupuesto de lectura) y T3 (reentrada humana)

*Base: `arquitectura-v1.1.md` (skills `hamutay-construir`, `hamutay-retomar`, hooks `SessionStart`/`Stop`/`PreCompact`, regla mecánica de `critico`). Fuentes verificadas con WebSearch/WebFetch el 2026-10-07, salvo donde dice "no verificado".*

## Fuentes en una tabla

| Fuente | Hallazgo útil | Evidencia |
|---|---|---|
| [Bisra et al. 2018, metaanálisis de autoexplicación](https://bps.org.uk/research-digest/self-explanation-powerful-learning-technique-according-meta-analysis-64-studies) | 64 estudios, g≈0,55; el formato de opción múltiple fue el menos eficaz | Fuerte |
| [Rozenblit y Keil 2002, ilusión de profundidad explicativa](https://en.wikipedia.org/wiki/Illusion_of_explanatory_depth) | Explicar por escrito baja la autoevaluación de comprensión: revela huecos. Pedir "justificar" polariza; pedir "explicar" no | Moderada (n pequeño, muy replicado) |
| [Adesope et al. 2017, *practice testing*](https://www.frontiersin.org/journals/education/articles/10.3389/feduc.2019.00005/full) | La recuperación beneficia con o sin retroalimentación; el reconocimiento también sirve | Fuerte (cifras exactas no verificadas) |
| [Pretesting effect (PMC9839203)](https://pmc.ncbi.nlm.nih.gov/articles/PMC9839203) | Preguntar antes de leer mejora el recuerdo, también con textos; Pan y Sana 2021: d=0,30 frente a retrieval practice. Los aprendices lo subestiman | Moderada |
| [Cepeda et al. 2006](https://www.evullab.org/pdf/CepedaPashlerVulWixtedRohrer-PB-2006.pdf) | 317 experimentos; el intervalo óptimo crece con el tiempo de retención | Fuerte |
| [Anthropic 2026, N=52](https://www.anthropic.com/research/AI-assistance-coding-skills) | Delegar en IA: 50 % vs 67 % en el quiz. Preservaron aprendizaje la "indagación conceptual" y "generar y luego comprender" | Moderada (RCT pequeño) |
| [Talevski et al. 2020, teach-back](https://pmc.ncbi.nlm.nih.gov/articles/PMC7156054) | 20 estudios; 19 con resultado positivo; calidad moderada; casi todo medido inmediatamente | Moderada |
| [I-PASS, Starmer et al. NEJM 2014](https://psnet.ahrq.gov/issue/changes-medical-errors-after-implementation-handoff-program) | Handoff estructurado en 9 hospitales: −23 % eventos adversos prevenibles, sin afectar el flujo de trabajo; último paso: síntesis del receptor | Fuerte (dominio distinto) |
| [Read-back/hear-back, SKYbrary/ICAO](https://skybrary.aero/articles/read-back-or-hear-back) | Solo se repiten ítems de seguridad (autorizaciones, niveles, rumbos); lo demás basta con acuse. Falla típica: sesgo de expectativa | Práctica regulada |
| [SAGAT, Endsley (SKYbrary)](https://skybrary.aero/articles/situation-awareness-global-assessment-technique-sagat) | Congelar y preguntar en momentos aleatorios; niveles: percibir, comprender, **proyectar**. Crítica: intrusivo | Moderada |
| [Shattuck, citado en Canadian Army Journal 2025](https://www.canada.ca/en/army/services/canadian-army-journal/articles/2025/short-bursts-rolls-commanders-intent-redux.html) | Tras el *back brief*, ante un cambio de situación solo 11 de 32 comandantes actuaron según la intención | Moderada |
| [Trafton et al. 2003](https://interruptions.net/literature/Trafton-IJHCS03.pdf) | Codificar la meta futura antes de interrumpir acelera la reanudación; las claves del contexto ayudan | Moderada (n=17) |
| [Gollwitzer y Sheeran 2006](https://goalsandprogress.com/implementation-intentions-research/) | Intenciones de implementación "si X, haré Y": 94 tests, d=0,65 | Fuerte (vía fuente secundaria) |
| [Gu et al., GP-TSM, CHI 2024](https://arxiv.org/abs/2401.10873) | Atenuar en gris el detalle, sin borrarlo, hizo más eficiente responder preguntas de comprensión (n=18) | Débil-moderada |
| [Storey, arXiv 2603.22106](https://arxiv.org/pdf/2603.22106) | Plantea "cognitive debt" e "intent debt" | Conceptual |

**Lecciones transversales.** (1) Repetir no es comprender: el *back brief* de Shattuck se hizo y aun así falló al cambiar la situación. Lo que discrimina es **proyectar ante algo nuevo** (nivel 3 de Endsley). (2) Producir supera a reconocer (Bisra): nada de opción múltiple. (3) Lo que se pregunta guía la atención (crítica a SAGAT y efecto pretest): esto es un riesgo si se mide y una ventaja si se usa para guiar la lectura.

---

## T2. Comprensión, defensa y presupuesto de lectura

### T2.1 Defensa por proyección, no por recuerdo
- **Cómo:** `hamutay-retomar` y el cierre de sección generan 1 pregunta del tipo "Si [supuesto de DEC-012] resultara falso, ¿qué cambiarías y en qué sección?". La genera `hm-lector`, que ve DEC y CRI, pero no la conversación. El humano responde en 2–4 líneas. Después se muestra lo que dice `hm impacto DEC-012` y el humano marca: *coincide / no coincide / me sorprendió*.
- **Fundamento:** Shattuck (repetir no basta; reaccionar al cambio discrimina); nivel 3 de SAGAT; la evaluación de políticas usa el mismo truco al preguntar por el contrafactual.
- **Anti-Goodhart:** la pregunta es nueva cada vez, sale de un supuesto real y no admite respuesta memorizada. La "nota" no se agrega: el resultado solo alimenta la cola de relectura (T2.6).
- **Evidencia:** moderada. **Costo humano:** 2–3 min, 1 interacción. **Implementación:** baja (prompt + `hm impacto`). **Encaje:** MVP; reemplaza la "prueba de defensa" genérica.

### T2.2 Explicar antes de ver, con contraste en vez de nota
- **Cómo:** antes de aprobar una sección con DEC críticas, el humano escribe en 2–3 frases por qué la sección concluye lo que concluye, sin mirarla. El agente muestra una tabla de tres columnas: *lo que dijiste / lo que dice la DEC / lo que falta*. No hay puntaje. Si el humano omite un supuesto, ese párrafo pasa al presupuesto de lectura.
- **Fundamento:** autoexplicación (Bisra, g≈0,55, formato abierto); ilusión de profundidad explicativa (explicar revela el hueco; pedir "justifica" polariza, así que el prompt dice "explica"). Anthropic N=52: el patrón "generar y luego comprender" preservó el aprendizaje.
- **Evidencia:** fuerte para aprender; débil para "defender un documento". **Costo:** 1–2 min por sección crítica. **Implementación:** baja. **Encaje:** MVP, solo en secciones con DEC crítica.

### T2.3 Read-back selectivo: el presupuesto de lectura como lista de "autorizaciones"
- **Cómo:** se formaliza lo que ya marca v1.1. Hay dos clases de párrafo:
  - **Read-back:** depende de una DEC crítica, toca una cifra, una conclusión o una recomendación, o lleva `[no verificado]`. Se lee y se confirma con una frase propia, no con "ok".
  - **Acuse:** el resto. Basta con "visto".

  La clase la asignan las reglas mecánicas de `critico`, no el agente.
- **Fundamento:** la aviación solo exige read-back de los ítems que, si se entienden mal, causan daño, y prohíbe "Roger" como sustituto. El sesgo de expectativa es exactamente el riesgo de leer prosa generada que "suena bien".
- **Evidencia:** moderada (práctica regulada, sin RCT). **Costo:** neutro o negativo: reduce lectura. **Implementación:** baja. **Encaje:** MVP.

### T2.4 Muestreo de auditoría sobre lo no presupuestado
- **Cómo:** además de los ítems clave (T2.3), `hamutay-construir` elige **1 párrafo al azar por sección** fuera del presupuesto y lo marca como "muestra". Si el humano encuentra un error en la muestra, esa sección entera pasa a lectura obligatoria y se registra en `encargo.md` (Log). Así el presupuesto se calibra con evidencia y no con la confianza del agente.
- **Fundamento:** la auditoría combina ítems clave y muestreo del resto para inferir sobre lo no examinado (práctica estándar; **no verifiqué el texto de la ISA 530**).
- **Evidencia:** moderada como práctica profesional. **Costo:** +1 párrafo por sección. **Implementación:** baja. **Encaje:** MVP. Es la defensa directa contra un presupuesto mal calibrado.

### T2.5 Atenuar en lugar de ocultar
- **Cómo:** la vista de lectura (`hm vista` o un HTML estático) muestra los párrafos de acuse en gris y plegables, y los de read-back en negro. No se resume nada: el texto original sigue completo.
- **Fundamento:** GP-TSM (Gu et al. 2024). Los resúmenes pueden omitir o alucinar sin que el lector lo note; atenuar no introduce contenido nuevo.
- **Evidencia:** débil-moderada (n=18). **Costo:** ahorra lectura. **Implementación:** media (render). **Encaje:** v2; en el MVP basta una marca `▶ leer` en el margen.

### T2.6 La deuda de comprensión como cola, no como métrica
- **Cómo:** `comprension.yaml` (v2) solo registra, por DEC crítica, `ultima_defensa: fecha | nunca` y `cambio_desde_defensa: sí/no`. La "deuda" es el conteo de DEC críticas sin defensa desde su último cambio. Se muestra como lista para releer, nunca como porcentaje para mejorar.
- **Fundamento:** Goodhart; crítica a SAGAT (preguntar dirige la atención). Storey nombra la deuda, pero no la mide.
- **Evidencia:** especulativa. **Costo:** cero extra. **Implementación:** baja. **Encaje:** v2 (en el MVP, una línea en `reentrada.md`).

### T2.7 Una pregunta antes de leer
- **Cómo:** al abrir una sección presupuestada, se hace **una** pregunta de predicción ("¿qué crees que concluye esta sección sobre X?"). Se responde en una línea y luego se lee.
- **Fundamento:** efecto pretest, también con textos (Pan y Sana: d=0,30). Los aprendices lo subestiman, así que conviene decir en una línea por qué se hace.
- **Evidencia:** moderada. **Costo:** 20 s. **Implementación:** baja. **Encaje:** v2 u opcional; en reentrada se fusiona con T3.2.

**Analogías descartadas por ceremonia:** revisión por pares completa (cara y lenta); teach-back con el agente que "enseña" primero (ya hecho en la construcción); SAGAT con congelamientos aleatorios a mitad de sesión (intrusivo).

---

## T3. Reentrada humana

### T3.1 Nota de reanudación como intención de implementación
- **Cómo:** al cerrar la sesión (hook `Stop` o `PreCompact`), el agente propone una frase y el humano la corrige o acepta: *"Cuando vuelva, lo primero es decidir DEC-019 (A vs B); si veo que falta el dato de X, empiezo por la sección 4."* Se guarda en `sesiones/proxima.md` con la **cita literal** del humano.
- **Fundamento:** Trafton et al. 2003 (codificar la meta futura antes de interrumpir acelera la reanudación); intenciones de implementación (d=0,65).
- **Evidencia:** moderada-fuerte. **Costo:** 15–30 s. **Implementación:** baja. **Encaje:** MVP. Es lo más barato con más retorno.

### T3.2 Recordar primero; la reentrada muestra lo olvidado
- **Cómo:** `hamutay-retomar` pregunta: "Sin mirar: ¿qué decidiste y qué quedaba pendiente?". El humano responde en texto libre. `hm reentrada` cruza esa respuesta con las DEC y CRI vigentes y ordena `reentrada.md` así:
  1. lo que **no recordaste** (DEC críticas ausentes de tu respuesta);
  2. lo que **cambió** desde la última sesión;
  3. lo que recordaste bien, en una sola línea.

  Esto es "reentrada centrada en lo olvidado" (H5) sin medir nada aparte.
- **Fundamento:** testing effect (Adesope et al.); ilusión de profundidad explicativa; la autoexplicación revela huecos.
- **Evidencia:** fuerte para el mecanismo de recuerdo; especulativa para el cruce automático (riesgo: el LLM interpreta mal una paráfrasis; mitigación: mostrar el cruce y dejar que el humano lo corrija).
- **Costo:** 2–3 min, pero ahorra leer lo que ya sabe. **Implementación:** media. **Encaje:** MVP (ya está en v1.1; esto le da su función).

### T3.3 `reentrada.md` con estructura I-PASS
- **Cómo:** cinco bloques fijos, de ≤1 pantalla:

| I-PASS | hamutay |
|---|---|
| **I**llness severity | Estado del entregable: en curso / bloqueado / en riesgo, con 1 razón |
| **P**atient summary | Tesis del documento en 3 líneas, citando DEC |
| **A**ction list | La próxima decisión (de T3.1) y la cola |
| **S**ituation awareness & contingency | Qué cambió y **qué se rompería si un supuesto falla** (enlaza con T2.1) |
| **S**ynthesis by receiver | El humano dice en una frase qué hará primero: read-back |

- **Fundamento:** I-PASS (−23 % de eventos adversos prevenibles en 9 sitios, sin afectar el flujo de trabajo). El valor está en el formato fijo y en la síntesis del receptor; SBAR es la versión mínima.
- **Evidencia:** fuerte en su dominio; analogía moderada aquí (el "turno saliente" es el propio humano del pasado más el registro). **Costo:** lectura de 1 pantalla + 1 frase. **Implementación:** baja (plantilla). **Encaje:** MVP.

### T3.4 Reentrada graduada por la duración de la pausa
- **Cómo:** `SessionStart` calcula los días desde la última sesión:
  - **<3 días:** solo se muestra `proxima.md` (T3.1);
  - **3–14 días:** + recuerdo libre (T3.2) + I-PASS (T3.3);
  - **>14 días o cambio de premisa:** + 1 pregunta de proyección (T2.1) + lectura de la tesis completa.

  Los umbrales se calibran en el piloto.
- **Fundamento:** el olvido crece con el intervalo y el repaso óptimo también (Cepeda et al. 2006). Evita la ceremonia fija que advirtió la auditoría A.
- **Evidencia:** moderada. **Costo:** proporcional a la pausa. **Implementación:** baja. **Encaje:** MVP; reemplaza la "pausa de N días" binaria.

### T3.5 Claves contextuales: volver al mismo lugar
- **Cómo:** `sesiones/ultimo.yaml` guarda la sección abierta, la DEC en discusión y **la última reacción literal del humano** ("esto no me convence porque…"). Al retomar, se muestra esa cita antes que cualquier resumen: la voz propia es la clave de recuperación más fuerte.
- **Fundamento:** Trafton et al. (las claves del contexto preactivan la meta suspendida); modelo de memoria de metas de Altmann y Trafton (citado en ese artículo).
- **Evidencia:** moderada. **Costo:** cero. **Implementación:** baja (el hook `PreCompact` ya guarda el estado). **Encaje:** MVP.

### T3.6 Repaso espaciado de lo fallado
- **Cómo:** una DEC que no se recordó en T3.2 o que falló en T2.1 se vuelve a preguntar en la siguiente reentrada (caja de Leitner mínima, con 2 niveles). Si se acierta dos veces, sale.
- **Fundamento:** espaciado (Cepeda); teach-back con verificación repetida (Talevski: la mayoría midió solo en el momento, una debilidad).
- **Evidencia:** fuerte para el espaciado; especulativa aquí. **Costo:** +1 pregunta ocasional. **Implementación:** baja. **Encaje:** v2.

### T3.7 "Qué cambió sin ti" como diff de decisiones
- **Cómo:** si hubo trabajo del agente o de otros entre sesiones, el bloque S de I-PASS lista las DEC y CRI nuevas o modificadas, no el texto cambiado. El detalle textual queda a un clic.
- **Fundamento:** M21 ("catch me up") del acervo, pero centrado en decisiones. Se solapa con T6.
- **Evidencia:** débil. **Costo:** bajo. **Implementación:** media (depende del diff semántico de T6). **Encaje:** v2.

---

## Top 3 para el MVP

### T2
1. **T2.3 Read-back selectivo + T2.4 muestreo** (las cuento como una pieza: el presupuesto es la lista de read-back, y el muestreo lo calibra). Es lo único que **reduce** la lectura neta y, a la vez, controla el riesgo de que el presupuesto falle. Costo ≈ 0, con reglas mecánicas y no juicio del agente.
2. **T2.1 Defensa por proyección.** Responde a "defender sin examen y sin Goodhart": la pregunta es nueva, sale de un supuesto real y se autocorrige con `hm impacto`. Shattuck muestra que repetir no basta.
3. **T2.2 Explicar antes de ver, con contraste.** Es la técnica con mejor evidencia (g≈0,55). Sin puntaje, convierte la comprensión en un input del presupuesto de lectura, en vez de un control más.

### T3
1. **T3.1 Nota de reanudación.** 30 s al cerrar y la mayor ganancia al volver (Trafton; d=0,65).
2. **T3.2 Recordar primero → lo olvidado primero.** Es literalmente H5. Hace que la reentrada se salte lo que ya sabes.
3. **T3.3 I-PASS + T3.4 graduación.** Da un formato fijo de una pantalla con síntesis del receptor, y la graduación evita la ceremonia en pausas cortas.

## Medidas para el piloto (añadir a §3 de v1.1)
- En la defensa a la semana, separar **recuerdo** de **proyección** (T2.1); la segunda es la que importa.
- Tiempo hasta la **primera acción útil** tras la pausa, no el tiempo de lectura de `reentrada.md`.
- Tasa de errores hallados en la muestra aleatoria (T2.4): si es >0 en varias secciones, el presupuesto está mal calibrado.

## No verificado / abierto
- El texto exacto de ISA 530 sobre ítems clave.
- Las cifras exactas del metaanálisis de Adesope et al. (solo verifiqué los hallazgos cualitativos).
- Si un LLM puede cruzar con fiabilidad el recuerdo libre contra las DEC (T3.2); hay que probarlo en el piloto.
