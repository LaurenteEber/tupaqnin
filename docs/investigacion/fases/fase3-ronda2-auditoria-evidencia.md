# Ronda 2 · Auditoría de evidencia de las ideas top (T1–T6)

*7-oct-2026 · ~40 consultas WebSearch/WebFetch. Estados: **VER** (verificada), **PAR** (parcial, con matiz), **NOV** (no verificada), **FAL** (falsa). Sin citas textuales largas.*

## 1. Afirmaciones verificadas una por una

| # | Afirmación (ronda 1) | Estado | Fuente primaria | Matiz y juicio sobre la inferencia |
|---|---|---|---|---|
| 1 | Apple: 9 jueces LLM ≈ 2 votos independientes | **VER** | [Kohli, Apple ML Research, jun-2026](https://machinelearning.apple.com/research/correlated-llm-evaluation-panels) | 9 modelos de 7 familias; ~¾ de la independencia se pierde; los agregadores cierran ≤11 % de la brecha y el mejor juez solo iguala o supera al panel. **Pero** las tareas son NLI y preferencias por pares (RewardBench), con respuesta correcta. Extrapolar a criticar prosa de política pública es un **salto moderado**; la dirección (no sumar jueces) sí está justificada. Autor único y preprint. |
| 2 | Kim & Garg 2025: errores correlacionados entre proveedores | **VER** | [Kim, Garg, Peng, Garg, ICML 2025](https://proceedings.mlr.press/v267/kim25e.html) | >350 LLM; ~60 % de acuerdo cuando ambos fallan; proveedor y arquitectura comparten error; los modelos grandes se correlacionan **aunque** difieran en ambos. Benchmarks de opción múltiple y cribado de CV. Justifica T5.3 ("no venderla como independencia"). |
| 3 | Panickssery et al. NeurIPS 2024, autopreferencia | **VER** | [NeurIPS 2024](https://papers.nips.cc/paper_files/paper/2024/hash/7f1f0218e45f5414c79c0679633e47bc-Abstract-Conference.html) | Correlación lineal autorreconocimiento–autopreferencia tras *fine-tuning* (GPT-4, Llama 2); tareas de resumen según literatura secundaria (no confirmado en el abstract). Apoya la separación redactor–juez; no cuantifica cuánto sesgo hay en textos largos ni si cambiar de sesión (mismo modelo) lo elimina: **eso es un salto** (el sesgo es del modelo, no de la sesión). |
| 4 | BMJ: ~3 de 9 errores sembrados | **VER** | [Schroter et al., *J R Soc Med* 2008](https://researchonline.lshtm.ac.uk/id/eprint/6900) | 607 revisores; 2,58 / 2,71 / 3,0 de 9 errores mayores; la capacitación breve apenas ayudó. Es de 2008 y de JRSM, no un "ensayo BMJ" reciente. El método (sembrar para medir recall) se transfiere bien a T5.4; la cifra humana no dice nada del recall de un LLM. |
| 5 | I-PASS: −23 % eventos adversos prevenibles | **PAR (cifra mal atribuida)** | [Starmer et al., NEJM 2014 vía HMS](https://hms.harvard.edu/news/safer-patient-handoffs); [MDedge](https://www.mdedge.com/content/inpatient-handoff-program-reduced-errors) | −23 % es **errores médicos** (24,5→18,8/100 ingresos); eventos adversos prevenibles bajaron **−30 %** (4,7→3,3). PSNet resume mal y la ronda 1 heredó el error. Diseño pre-post escalonado en 9 programas pediátricos (no aleatorizado); sin cambio significativo en el flujo de trabajo. Corregir la cifra. |
| 6 | Bisra et al. 2018, autoexplicación g≈0,55 | **VER** | [BPS Research Digest](https://bps.org.uk/research-digest/self-explanation-powerful-learning-technique-according-meta-analysis-64-studies) (*Educ Psychol Rev* 2018) | 64 estudios, ~6 000 estudiantes; opción múltiple = formato menos eficaz. Mide **aprendizaje de contenido** escolar. Usarlo para "defender un documento propio" es un **salto** (la ronda 1 ya lo admitió). |
| 7 | Rozenblit y Keil / "justificar polariza" | **PAR** | [Fernbach et al., *Psych Sci* 2013](https://www.hks.harvard.edu/publications/political-extremism-supported-illusion-understanding) | Explicar mecanismos redujo la confianza y moderó actitudes; **enumerar razones no moderó**. No hay evidencia en esa fuente de que "justificar polarice". Reformular: "justificar no ayuda". |
| 8 | Trafton et al. 2003, reanudación | **PAR** | [IJHCS 58 (2003)](https://interruptions.net/literature/Trafton-IJHCS03.pdf) | n=17; aviso con 8 s de margen; el beneficio (~4 s) apareció solo en la sesión 1 y el grupo sin aviso lo alcanzó en la 3; la codificación de metas fue espontánea, no instruida. Base débil para T3.1: interrupciones de **segundos** vs. pausas de **días**. **Salto grande.** |
| 9 | Gollwitzer & Sheeran 2006, d≈0,65 | **VER** | [*Adv Exp Soc Psychol* 38, KOPS](https://kops.uni-konstanz.de/entities/publication/2e749bfb-8533-437c-8203-7e788c910c5f) | 94 tests, d=0,65 en logro de metas. La actualización de 2024 (642 tests) da d entre 0,27 y 0,66 según moderadores ([resumen secundario](https://goalsandprogress.com/implementation-intentions-research/)). Mide **iniciar conductas**, no reanudar trabajo cognitivo. Analogía plausible, no evidencia directa. |
| 10 | Back brief: 11 de 32 (Canadian Army Journal) | **VER (secundaria)** | [Canadian Army Journal 2025](https://www.canada.ca/en/army/services/canadian-army-journal/articles/2025/short-bursts-rolls-commanders-intent-redux.html), cita a Shattuck en *Military Review* 2000 | Escenario del NTC; 34 % de comandantes de compañía actuó según la intención tras el back brief y un cambio de situación. Original no leído; sin grupo control (no muestra que proyectar "discrimine" mejor que repetir, solo que repetir no basta). |
| 11 | Apache lazy consensus (~72 h) | **VER** | [Apache Community](https://community.apache.org/committers/lazyConsensus.html) | Anuncio público, ~72 h, silencio = consentimiento; supone **muchos lectores** de una lista. Para una sola persona, el silencio puede ser desatención: **salto alto** (ya marcado en ronda 1). |
| 12 | Timing Matters, CHI 2025 | **VER** | [Qin et al., arXiv 2502.06197 / DOI 10.1145/3706598.3713146](https://arxiv.org/abs/2502.06197) | N=60, tarea de escritura; usar LLM desde el inicio redujo ideas originales, autoeficacia y autoría percibida. Un solo experimento corto: la etiqueta "fuerte en conjunto" de T4.1 se apoya en grupos nominales, que son otra situación (grupos humanos). Ajustar a **moderada**. |
| 13 | Gero et al. CHI 2024 | **PAR** | [CHI 2024, art. 838 (dblp)](https://dblp.uni-trier.de/pid/264/7387.html); [arXiv 2401.13726](https://arxiv.org/html/2401.13726v1) | N=24. **Sin diferencias** al comparar 9 reescrituras de email; ventaja solo con 50 respuestas (comparar modelos). La interfaz resalta similitudes **sobre el texto completo**; no resume en celdas. T4.4 (3–7 opciones, celdas ≤8 palabras generadas por LLM) **no** está respaldado por este estudio y reintroduce el riesgo de omisión que T2.5 critica. |
| 14 | Kim et al. CHI 2025: inconsistencias calibran | **VER (con matiz)** | [Kim, Vaughan, Liao, Lombrozo, Russakovsky, arXiv 2502.08554](https://arxiv.org/abs/2502.08554) | N=308, preregistrado, preguntas objetivas. Explicaciones ↑ dependencia en aciertos y errores; fuentes e **inconsistencias dentro de la explicación** ↓ dependencia en respuestas incorrectas. "Mostrar dos lecturas lado a lado" (T5.5) es una extensión razonable pero no probada. |
| 15 | EvalGen: criteria drift | **VER** | [Shankar et al., UIST 2024](https://arxiv.org/abs/2404.12272) | 9 profesionales, estudio cualitativo; deriva = añadir criterios y reinterpretarlos según los outputs vistos. Respalda que la deriva existe y es esperable; **no** respalda el umbral "≥2 objeciones" de T1.5 (diseño propio). |
| 16 | Enlaces sospechosos DOORS/PTC | **VER** | [PTC Modeler 10.1](https://support.ptc.com/help/modeler/r10.1/en/Modeler/rtsme/managing_changes_to_remote_oslc_data.html) | Sospecha automática al modificarse el ítem enlazado; se limpia tras revisar o se borra el enlace. Práctica industrial documentada, sin datos de eficacia. Transferencia **directa** (es el mismo problema). Diferencia: en DOORS limpia una persona; en T6.1 puede limpiar el agente. |
| 17 | Read-back selectivo (ICAO) | **VER** | [SKYbrary](https://skybrary.aero/articles/read-back-or-hear-back) | Solo ítems de seguridad; "Roger" no sustituye; sesgo de expectativa. |
| 18 | AS 1215 PCAOB | **VER** | [PCAOB AS 1215 .06A, .10](https://pcaobus.org/oversight/standards/auditing-standards/details/AS1215) | Comprensión por auditor externo, quién/cuándo, ítems inspeccionados. |
| 19 | Khan et al., debate con asimetría | **VER (otra cosa)** | [arXiv 2402.06782](https://arxiv.org/abs/2402.06782) | La asimetría es entre debatientes expertos y juez sin texto (76 %/88 % vs 48 %/60 %). No prueba que **partir el contexto entre lectores** descorrelacione errores (T5.2). |
| 20 | Muestreo de auditoría (ISA 530) | **NOV** | — | Práctica conocida; no se leyó la norma. Además, 1 párrafo por sección tiene poder bajísimo: con 5 % de párrafos erróneos, detecta ~1 de cada 20 secciones con error. |
| 21 | Cepeda 2006 | **VER** | [Psych Bull 2006](https://www.evullab.org/pdf/CepedaPashlerVulWixtedRohrer-PB-2006.pdf) | 317 experimentos, memoria verbal; justifica graduar (T3.4) en dirección, no los umbrales 3/14 días. |
| 22 | Anthropic N=52 | **VER** | [Anthropic 2026](https://www.anthropic.com/research/AI-assistance-coding-skills) | 50 % vs 67 % (d=0,74). "Generar y luego comprender" n=2: anecdótico. |

No se detectaron afirmaciones **FALSAS**; sí una cifra mal atribuida (I-PASS) y una sobrelectura (Fernbach).

## 2. Las 18 ideas top con evidencia ajustada

| Idea | Ronda 1 | Ajustada | Razón principal |
|---|---|---|---|
| T1.2 Lazy consensus | moderada/especulativa | **Especulativa** | Mecanismo de grupos con muchos lectores; con una persona, silencio ≠ acuerdo. |
| T1.4 Distinguir vs. revocar | fuerte/especulativa | **Débil** | Analogía institucional; ningún dato de uso en criterios personales. |
| T1.5 Deriva contada | moderada | **Moderada (fenómeno) / especulativa (umbral)** | EvalGen VER, n=9 cualitativo. |
| T4.1 Ideación humana primero | fuerte | **Moderada** | Un experimento N=60, tarea corta. |
| T4.4 Vista de contraste | moderada | **Débil** | Gero sin efecto con pocas respuestas; celdas resumidas no probadas. |
| T4.5 Dimensiones del humano | moderada | **Débil** | Extensión de T4.1; Kepner-Tregoe NOV. |
| T2.3 Read-back selectivo (+T2.4) | moderada | **Moderada (T2.3) / débil (T2.4)** | Práctica regulada VER; muestreo de 1 párrafo sin poder estadístico. |
| T2.1 Defensa por proyección | moderada | **Débil** | Shattuck secundaria, sin control; SAGAT no evaluado aquí. |
| T2.2 Explicar antes de ver | fuerte/débil | **Moderada** | Bisra + Fernbach VER, pero para aprendizaje, no defensa documental. |
| T3.1 Nota de reanudación | moderada-fuerte | **Débil** | Trafton: segundos, n=17, efecto transitorio; Gollwitzer: otra conducta. |
| T3.2 Recordar primero | fuerte/especulativa | **Moderada (recuerdo) / especulativa (cruce LLM)** | Testing effect sólido; cruce automático sin probar. |
| T3.3 I-PASS (+T3.4) | fuerte en dominio | **Moderada** | Pre-post, handoff entre personas distintas; cifra a corregir. |
| T5.1 Hoja de afirmaciones | fuerte/moderada | **Moderada** | AS 1215 VER como norma; sin datos de eficacia con LLM. |
| T5.2 Lentes con información distinta | moderada | **Moderada (problema) / especulativa (remedio)** | Correlación VER (Apple, Kim); partición de contexto no probada. |
| T5.5 Disenso anclado | moderada | **Moderada-débil** | Kim CHI 2025 VER; Nemeth NOV. |
| T6.1 Enlaces sospechosos | fuerte | **Moderada** | Práctica VER, sin estudios de eficacia; limpieza por agente es nueva. |
| T6.3 Diff de esqueleto | débil-moderada | **Débil** | Toulmin descriptivo; van Gelder no controlado; LegDiff NOV. |
| T6.4 Triage tres carriles | moderada | **Débil** | Diseño interno (M33); depende de T6.3. |

**Recuento:** fuerte 0 · moderada 7 · débil 8 · especulativa 1 (+ 2 mixtas). Ninguna idea top tiene evidencia directa en documentos largos coescritos con IA.

## 3. Saltos a vigilar en el piloto

1. **Silencio = consentimiento (T1.2).** Registrar si el humano abrió/leyó la sección antes de contar "sin objeción"; comparar CRI tácitos vs. explícitos revocados después.
2. **Segundos → días (T3.1).** Medir tiempo hasta la primera acción útil con y sin nota, en pausas >3 días.
3. **Aprendizaje escolar → defensa de documento (T2.1, T2.2).** No asumir g≈0,55; medir si la explicación detecta supuestos omitidos.
4. **Tabla resumida ≠ interfaz de Gero (T4.4).** Auditar si las celdas omiten diferencias reales; con ≤5 opciones puede no aportar.
5. **Correlación ≠ remedio (T5.2, T5.3).** Con errores sembrados (T5.4), comprobar si lectores con contexto partido recuperan errores **distintos**; cambiar de sesión no quita la autopreferencia del mismo modelo.
6. **El agente limpia sus propias sospechas (T6.1).** Muestrear limpiezas "sigue válida" y contar las erróneas.
7. **Esqueleto extraído por LLM (T6.3/T6.4).** Si omite un cambio, el triage lo manda al carril 1 y nadie lo ve: medir falsos negativos contra diff textual.
8. **Muestra de 1 párrafo (T2.4).** Insuficiente para calibrar: usar muestras acumuladas entre secciones o aumentar el tamaño.
9. **Cifras a corregir en documentos derivados:** I-PASS (−23 % errores, −30 % eventos prevenibles); BMJ = Schroter 2008 JRSM; "justificar polariza" → "justificar no modera".
